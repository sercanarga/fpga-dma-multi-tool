package main

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func compareFPGADeviceDetailRows(
	left, right fpgaDetailRow,
	column int,
) int {
	switch column {
	case 0:
		return compareTableText(left.Section, right.Section)
	case 1:
		return compareTableText(left.Field, right.Field)
	default:
		return compareTableText(left.Value, right.Value)
	}
}

func (state *guiState) showFPGADeviceDetails(device deviceResult) {
	var snapshot *fpgaAdvancedSnapshot
	rows := fpgaDeviceDetailRows(device, nil)
	status := widget.NewLabel("Reading FPGA configuration registers…")
	status.Wrapping = fyne.TextWrapWord
	title := widget.NewLabelWithStyle(
		strings.ToUpper(partFamily(device.Part))+" Detailed Info",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	subtitle := widget.NewLabel(
		"Safe registers load automatically. Sensor and flash probes run only after confirmation.",
	)
	subtitle.Wrapping = fyne.TextWrapWord

	columns := []sortableTableColumn{
		{Title: "Section", Width: 120, Sortable: true},
		{Title: "Field", Width: 180, Sortable: true},
		{Title: "Value", Width: 402, Sortable: true},
	}
	sortState := newTableSortState(-1, true)
	table := widget.NewTable(
		func() (int, int) {
			return len(rows), len(columns)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			return newStandardTableCell(label)
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			cell := standardTableCellContent(object)
			label := cell.Objects[0].(*widget.Label)
			if id.Row < 0 || id.Row >= len(rows) {
				label.SetText("")
				return
			}
			row := rows[id.Row]
			switch id.Col {
			case 0:
				label.SetText(row.Section)
			case 1:
				label.SetText(row.Field)
			default:
				label.SetText(row.Value)
			}
		},
	)
	configureSortableTable(table, columns, &sortState, func() {
		sortTableRows(rows, sortState, compareFPGADeviceDetailRows)
	})
	table.OnSelected = func(widget.TableCellID) {
		table.UnselectAll()
	}

	var detailDialog dialog.Dialog
	var operationRunning atomic.Bool
	var refreshButton, sensorButton, flashButton *widget.Button
	setOperationRunning := func(running bool, active *widget.Button, activeText string) {
		for _, button := range []*widget.Button{refreshButton, sensorButton, flashButton} {
			if button == nil {
				continue
			}
			if running {
				button.Disable()
			} else {
				button.Enable()
			}
		}
		if active != nil {
			if running {
				active.SetText(activeText)
			} else {
				switch active {
				case refreshButton:
					active.SetText("Refresh registers")
				case sensorButton:
					active.SetText("Read sensors")
				case flashButton:
					active.SetText("Probe flash")
				}
			}
		}
	}
	updateRows := func() {
		rows = fpgaDeviceDetailRows(device, snapshot)
		sortTableRows(rows, sortState, compareFPGADeviceDetailRows)
		table.Refresh()
	}
	refresh := func() {
		if !operationRunning.CompareAndSwap(false, true) {
			return
		}
		setOperationRunning(true, refreshButton, "Reading…")
		status.SetText("Reading FPGA configuration registers…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			updated, err := inspectFPGAAdvancedInfo(ctx, device)
			fyne.Do(func() {
				operationRunning.Store(false)
				setOperationRunning(false, refreshButton, "")
				if err != nil {
					status.SetText("Detailed register read failed: " + err.Error())
					if len(updated.Warnings) == 0 {
						updated.Warnings = append(
							updated.Warnings,
							"Live configuration registers are unavailable.",
						)
					}
				} else if len(updated.Warnings) > 0 {
					status.SetText(fmt.Sprintf(
						"Detailed information loaded with %d warning(s).",
						len(updated.Warnings),
					))
				} else {
					status.SetText("Detailed information loaded.")
				}
				if snapshot != nil {
					updated.Sensors = snapshot.Sensors
					updated.Flash = snapshot.Flash
				}
				snapshot = &updated
				updateRows()
			})
		}()
	}
	refreshButton = widget.NewButton("Refresh registers", refresh)
	refreshButton.Importance = widget.HighImportance

	readSensors := func() {
		if !operationRunning.CompareAndSwap(false, true) {
			return
		}
		setOperationRunning(true, sensorButton, "Reading…")
		status.SetText("Reading XADC temperature and supply measurements…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			sensors, transport, err := inspectFPGAXADC(ctx, device)
			fyne.Do(func() {
				operationRunning.Store(false)
				setOperationRunning(false, sensorButton, "")
				if err != nil {
					status.SetText("XADC sensor read failed: " + err.Error())
					return
				}
				if snapshot == nil {
					snapshot = &fpgaAdvancedSnapshot{
						Registers: make(map[string]uint32),
					}
				}
				snapshot.Sensors = &sensors
				if strings.TrimSpace(transport) != "" {
					snapshot.Transport = transport
				}
				status.SetText(
					"XADC measurements loaded. Reconfigure the FPGA to restore its prior XADC sequencer.",
				)
				updateRows()
			})
		}()
	}
	sensorButton = widget.NewButton("Read sensors", func() {
		showWinUIConfirm(
			state.window,
			"Read XADC sensors",
			"This temporarily switches the FPGA XADC sequencer to single-channel mode. "+
				"The previous XADC configuration is restored when the FPGA image is reloaded.\n\n"+
				"Read temperature and supply voltages now?",
			"Read sensors",
			true,
			readSensors,
		)
	})

	probeFlash := func() {
		if !operationRunning.CompareAndSwap(false, true) {
			return
		}
		setOperationRunning(true, flashButton, "Probing…")
		status.SetText("Loading the temporary JTAG bridge and reading SPI flash identity…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			flash, transport, err := inspectFPGAFlash(ctx, device)
			fyne.Do(func() {
				operationRunning.Store(false)
				setOperationRunning(false, flashButton, "")
				if err != nil {
					status.SetText("SPI flash probe failed: " + err.Error())
					return
				}
				if snapshot == nil {
					snapshot = &fpgaAdvancedSnapshot{
						Registers: make(map[string]uint32),
					}
				}
				snapshot.Flash = &flash
				if strings.TrimSpace(transport) != "" {
					snapshot.Transport = transport
				}
				status.SetText("SPI flash information loaded; persistent contents were not written.")
				updateRows()
			})
		}()
	}
	flashButton = widget.NewButton("Probe flash", func() {
		showWinUIConfirm(
			state.window,
			"Probe SPI flash",
			"This temporarily replaces the running FPGA image with the SPI-over-JTAG bridge "+
				"and may reset the board. Persistent flash contents are not erased or written.\n\n"+
				"Probe the flash now?",
			"Probe flash",
			true,
			probeFlash,
		)
	})

	copyButton := newWinUISecondaryButton("Copy details", 112, func() {
		state.window.Clipboard().SetContent(formatFPGADeviceDetails(device, snapshot))
		status.SetText("Detailed information copied.")
	})
	closeButton := newWinUISecondaryButton("Close", 88, func() {
		if detailDialog != nil {
			detailDialog.Hide()
		}
	})
	actions := container.NewHBox(
		copyButton.Object,
		closeButton.Object,
	)
	probeActions := container.NewHBox(refreshButton, sensorButton, flashButton)
	header := container.NewVBox(title, subtitle, probeActions)
	footer := container.NewBorder(nil, nil, status, actions, nil)
	content := container.NewBorder(header, footer, nil, nil, newTableViewport(table))
	detailDialog = dialog.NewCustomWithoutButtons("FPGA Detailed Info", content, state.window)
	detailDialog.Resize(fyne.NewSize(790, 510))
	detailDialog.Show()
	refresh()
}
