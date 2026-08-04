package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type deviceTableRow struct {
	Device    deviceResult
	Index     int
	IndexText string
	FPGA      string
	IDCode    string
	DNAID     string
	Board     string
}

func deviceTableRows(devices []deviceResult) []deviceTableRow {
	rows := make([]deviceTableRow, 0, len(devices))
	for _, device := range devices {
		board := strings.Join(device.BoardMatches, ", ")
		if board == "" {
			board = "Unknown board"
		}
		rows = append(rows, deviceTableRow{
			Device:    device,
			Index:     device.Index,
			IndexText: fmt.Sprintf("%d", device.Index),
			FPGA:      strings.ToUpper(partFamily(device.Part)),
			IDCode:    device.IDCode,
			DNAID:     device.FuseDNA,
			Board:     board,
		})
	}
	return rows
}

func compareDeviceTableRows(left, right deviceTableRow, column int) int {
	switch column {
	case 0:
		return compareTableInt(left.Index, right.Index)
	case 1:
		return compareTableText(left.FPGA, right.FPGA)
	case 2:
		return compareTableText(left.IDCode, right.IDCode)
	case 3:
		return compareTableText(left.DNAID, right.DNAID)
	default:
		return compareTableText(left.Board, right.Board)
	}
}

func (state *guiState) buildDeviceTab() fyne.CanvasObject {
	summary := widget.NewLabelWithStyle("Ready to scan", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	status, statusBar := newStatusBar("Connect the adapter and power the FPGA board.")
	columns := []sortableTableColumn{
		{Title: "#", Width: 44, Sortable: true},
		{Title: "FPGA", Width: 78, Sortable: true},
		{Title: "IDCODE", Width: 106, Sortable: true},
		{Title: "DNA ID", Width: 160, Sortable: true},
		{Title: "Board", Width: 216, Sortable: true},
		{Width: 116},
	}
	rows := []deviceTableRow{}
	sortState := newTableSortState(-1, true)

	var table *widget.Table
	table = widget.NewTable(
		func() (int, int) {
			return len(rows), len(columns)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			button := newWinUISecondaryButton("Detailed Info", 112, nil)
			button.Hide()
			return newStandardTableCell(label, button.Object)
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			cell := standardTableCellContent(object)
			label := cell.Objects[0].(*widget.Label)
			buttonObject := cell.Objects[1]
			button := winUISecondaryButtonWidget(buttonObject)
			if id.Row < 0 || id.Row >= len(rows) {
				buttonObject.Hide()
				label.Show()
				label.SetText("")
				return
			}
			row := rows[id.Row]
			if id.Col == len(columns)-1 {
				label.Hide()
				buttonObject.Show()
				button.OnTapped = func() {
					status.SetText("Opening detailed FPGA information…")
					state.showFPGADeviceDetails(row.Device)
				}
				return
			}
			buttonObject.Hide()
			label.Show()
			switch id.Col {
			case 0:
				label.SetText(row.IndexText)
			case 1:
				label.SetText(row.FPGA)
			case 2:
				label.SetText(row.IDCode)
			case 3:
				label.SetText(row.DNAID)
			default:
				label.SetText(row.Board)
			}
		},
	)
	configureSortableTable(table, columns, &sortState, func() {
		sortTableRows(rows, sortState, compareDeviceTableRows)
	})
	table.OnSelected = func(widget.TableCellID) {
		table.UnselectAll()
	}

	var scanButton *widget.Button
	scanButton = widget.NewButton("Scan device", func() {
		scanButton.Disable()
		rows = nil
		state.resultMu.Lock()
		state.result = scanResult{}
		state.resultMu.Unlock()
		table.Refresh()
		summary.SetText("Scanning…")
		status.SetText("Reading the FPGA…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			opts := options{backend: "auto", device: -1, ch347Index: -1, timeout: 45 * time.Second}
			var diagnostics bytes.Buffer
			devices, err := scanAutomatic(ctx, opts, &diagnostics)
			fyne.Do(func() {
				scanButton.Enable()
				if err != nil {
					summary.SetText("No device found")
					status.SetText(deviceScanErrorMessage(err))
					return
				}
				state.resultMu.Lock()
				state.result = scanResult{ToolVersion: version, Devices: devices}
				state.resultMu.Unlock()
				rows = deviceTableRows(devices)
				sortTableRows(rows, sortState, compareDeviceTableRows)
				if state.programPart != nil && len(devices) > 0 {
					if detected := partFamily(devices[0].Part); detected != "" {
						state.programPart.SetSelected(strings.ToUpper(detected))
					}
				}
				table.Refresh()
				summary.SetText(deviceScanSummary(devices))
				status.SetText("Scan complete. Use Detailed Info for configuration and boot status.")
			})
		}()
	})
	scanButton.Importance = widget.HighImportance

	actionBar := container.NewBorder(
		nil,
		nil,
		summary,
		scanButton,
		nil,
	)
	body := container.NewBorder(
		actionBar,
		nil,
		nil,
		nil,
		newTableViewport(table),
	)
	return newPageFrame(
		"FPGA Devices",
		"Read the JTAG chain, FPGA model, IDCODE, and factory DNA ID without changing the board.",
		body,
		statusBar,
	)
}

func deviceScanSummary(devices []deviceResult) string {
	if len(devices) == 0 {
		return "No device found"
	}
	model := strings.ToUpper(partFamily(devices[0].Part))
	if len(devices) == 1 {
		return model + " • 1 device"
	}
	return fmt.Sprintf("%s • %d devices in the JTAG chain", model, len(devices))
}

func deviceScanErrorMessage(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "tdo remained high"):
		return "TDO stayed high. Check board power, TDO/GND, and cable orientation."
	case strings.Contains(message, "tdo remained low"):
		return "TDO stayed low. Check board power, TDO/GND, and cable orientation."
	case strings.Contains(message, "ch347") &&
		(strings.Contains(message, "not found") || strings.Contains(message, "open")):
		return "CH347 adapter not found. Reconnect it, then check the Drivers tab."
	case strings.Contains(message, "rs232") &&
		(strings.Contains(message, "winusb") || strings.Contains(message, "driver")):
		return "RS232 writer found, but Interface A is not ready. Check its WinUSB driver in the Drivers tab."
	default:
		return "Could not read the FPGA through CH347 or RS232. Check the USB/JTAG cable, board power, and Drivers tab."
	}
}
