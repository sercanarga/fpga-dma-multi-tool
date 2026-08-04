package main

import (
	"context"
	"fmt"
	"image/color"
	"strconv"
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
		return currentWinUIThemeColor(winUIColorSuccess)
	case string(systemInfoOff):
		return currentWinUIThemeColor(winUIColorError)
	case string(systemInfoUnavailable):
		return currentWinUIThemeColor(winUIColorTextDisabled)
	default:
		return currentWinUIThemeColor(winUIColorAccent)
	}
}

func compareSystemInfoTableRows(
	left, right systemInfoTableRow,
	column int,
) int {
	switch column {
	case 0:
		leftWidth, leftErr := strconv.Atoi(strings.TrimPrefix(strings.ToLower(left.State), "x"))
		rightWidth, rightErr := strconv.Atoi(strings.TrimPrefix(strings.ToLower(right.State), "x"))
		if leftErr == nil && rightErr == nil {
			return compareTableInt(leftWidth, rightWidth)
		}
		return compareTableText(left.State, right.State)
	case 1:
		return compareTableText(left.Feature, right.Feature)
	default:
		return compareTableText(left.Detail, right.Detail)
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

	columns := []sortableTableColumn{
		{Title: "State", Width: 80, Sortable: true},
		{Title: "Feature or device", Width: 220, Sortable: true},
		{Title: "Details", Width: 432, Sortable: true},
	}
	rows := []systemInfoTableRow{}
	sortState := newTableSortState(-1, true)
	table := widget.NewTable(
		func() (int, int) {
			return len(rows), len(columns)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			stateText := canvas.NewText("", currentWinUIThemeColor(winUIColorTextPrimary))
			stateText.TextSize = 14
			stateText.TextStyle = fyne.TextStyle{Bold: true}
			stateCell := container.New(
				&statusLabelLayout{horizontalPadding: 8, minimumWidth: 16},
				stateText,
			)
			return newStandardTableCell(label, stateCell)
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			cell := standardTableCellContent(object)
			label := cell.Objects[0].(*widget.Label)
			stateCell := cell.Objects[1].(*fyne.Container)
			stateText := stateCell.Objects[0].(*canvas.Text)
			if id.Row < 0 || id.Row >= len(rows) {
				label.SetText("")
				stateCell.Hide()
				stateText.Text = ""
				stateText.Refresh()
				return
			}
			row := rows[id.Row]
			if id.Col == 0 {
				label.Hide()
				stateCell.Show()
				stateText.Text = row.State
				stateText.Color = systemInfoStateColor(row.State)
				stateText.Refresh()
				return
			}
			stateCell.Hide()
			label.Show()
			if id.Col == 1 {
				label.SetText(row.Feature)
			} else {
				label.SetText(row.Detail)
			}
		},
	)
	configureSortableTable(table, columns, &sortState, func() {
		sortTableRows(rows, sortState, compareSystemInfoTableRows)
	})
	table.OnSelected = func(widget.TableCellID) {
		table.UnselectAll()
	}

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
				sortTableRows(rows, sortState, compareSystemInfoTableRows)
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
	body := container.NewBorder(top, nil, nil, nil, newTableViewport(table))
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
