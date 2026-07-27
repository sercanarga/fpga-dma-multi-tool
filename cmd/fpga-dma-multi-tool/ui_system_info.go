package main

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type systemInfoTableRow struct {
	State   string
	Feature string
	Detail  string
}

func systemInfoRows(snapshot systemInfoSnapshot) []systemInfoTableRow {
	rows := make([]systemInfoTableRow, 0, len(snapshot.Features)+len(snapshot.PCIeLinks))
	for _, feature := range snapshot.Features {
		rows = append(rows, systemInfoTableRow{
			State:   string(feature.State),
			Feature: feature.Name,
			Detail:  feature.Detail,
		})
	}
	if len(snapshot.PCIeLinks) == 0 {
		rows = append(rows, systemInfoTableRow{
			State:   string(systemInfoUnavailable),
			Feature: "PCIe link width",
			Detail:  "Windows did not expose a negotiated PCIe link width for any present device.",
		})
		return rows
	}
	for _, link := range snapshot.PCIeLinks {
		detail := fmt.Sprintf("Negotiated x%d", link.CurrentWidth)
		if link.MaximumWidth > 0 {
			detail += fmt.Sprintf("  •  Maximum x%d", link.MaximumWidth)
		}
		if strings.TrimSpace(link.InstanceID) != "" {
			detail += "  •  " + link.InstanceID
		}
		rows = append(rows, systemInfoTableRow{
			State:   fmt.Sprintf("x%d", link.CurrentWidth),
			Feature: link.Name,
			Detail:  detail,
		})
	}
	return rows
}

func systemInfoStateColor(state string) color.Color {
	switch state {
	case string(systemInfoOn):
		return winUILightPalette.success
	case string(systemInfoOff):
		return winUILightPalette.error
	case string(systemInfoUnavailable):
		return winUILightPalette.textDisabled
	default:
		return winUILightPalette.accent
	}
}

func (state *guiState) buildSystemInfoTab() fyne.CanvasObject {
	status, statusBar := newStatusBar("Reading Windows security and PCIe information…")
	processor := widget.NewLabelWithStyle(
		"Processor: checking…",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	note := widget.NewLabel(
		"PCIe rows show the negotiated width only when the device and Windows PnP stack expose it.",
	)
	note.Wrapping = fyne.TextWrapWord

	headers := []string{"State", "Feature or device", "Details"}
	rows := []systemInfoTableRow{}
	table := widget.NewTable(
		func() (int, int) {
			return len(rows) + 1, len(headers)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			stateText := canvas.NewText("", winUILightPalette.textPrimary)
			stateText.TextSize = 14
			stateText.TextStyle = fyne.TextStyle{Bold: true}
			return container.NewStack(label, stateText)
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			cell := object.(*fyne.Container)
			label := cell.Objects[0].(*widget.Label)
			stateText := cell.Objects[1].(*canvas.Text)
			if id.Row == 0 {
				stateText.Hide()
				label.Show()
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.SetText(headers[id.Col])
				return
			}
			index := id.Row - 1
			if index < 0 || index >= len(rows) {
				label.SetText("")
				stateText.Text = ""
				stateText.Refresh()
				return
			}
			row := rows[index]
			if id.Col == 0 {
				label.Hide()
				stateText.Show()
				stateText.Text = row.State
				stateText.Color = systemInfoStateColor(row.State)
				stateText.Refresh()
				return
			}
			stateText.Hide()
			label.Show()
			label.TextStyle = fyne.TextStyle{}
			if id.Col == 1 {
				label.SetText(row.Feature)
			} else {
				label.SetText(row.Detail)
			}
		},
	)
	for column, width := range []float32{95, 250, 470} {
		table.SetColumnWidth(column, width)
	}
	table.SetRowHeight(0, tableHeaderHeight)

	var refreshRunning atomic.Bool
	var refreshButton *widget.Button
	refresh := func() {
		if !refreshRunning.CompareAndSwap(false, true) {
			return
		}
		refreshButton.Disable()
		refreshButton.SetText("Refreshing…")
		if len(rows) == 0 {
			processor.SetText("Processor: checking…")
		}
		status.SetText("Refreshing Windows security and PCIe information…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			snapshot, err := inspectSystemInfo(ctx)
			fyne.Do(func() {
				refreshRunning.Store(false)
				refreshButton.SetText("Refresh")
				refreshButton.Enable()
				if err != nil {
					if len(rows) == 0 {
						processor.SetText("Processor: unavailable")
					}
					status.SetText("Refresh stopped: Windows did not respond within 15 seconds.")
					showWinUIError(state.window, err)
					return
				}
				rows = systemInfoRows(snapshot)
				table.Refresh()
				processorName := snapshot.ProcessorName
				if processorName == "" {
					processorName = "Unavailable"
				}
				processor.SetText("Processor: " + processorName)
				status.SetText(fmt.Sprintf(
					"System information refreshed: %d security features, %d PCIe links.",
					len(snapshot.Features),
					len(snapshot.PCIeLinks),
				))
			})
		}()
	}
	refreshButton = widget.NewButton("Refresh", refresh)
	refreshButton.Importance = widget.HighImportance
	top := container.NewVBox(
		container.NewBorder(nil, nil, processor, refreshButton, nil),
		note,
	)
	body := container.NewBorder(top, nil, nil, nil, table)
	go func() {
		time.Sleep(100 * time.Millisecond)
		fyne.Do(refresh)
	}()
	return newPageFrame(
		"System Info",
		"Review hardware virtualization, Windows security, and negotiated PCIe link widths.",
		body,
		statusBar,
	)
}
