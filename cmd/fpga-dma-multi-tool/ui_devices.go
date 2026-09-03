package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fyneTheme "fyne.io/fyne/v2/theme"
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

type deviceTableCell struct {
	container      *fyne.Container
	label          *widget.Label
	dnaLabel       *widget.Label
	dnaContent     *fyne.Container
	copyButton     *widget.Button
	detailsContent *fyne.Container
	detailsButton  *widget.Button
}

const deviceTableDoubleClickWindow = 500 * time.Millisecond

type deviceTableDoubleClickTracker struct {
	row int
	at  time.Time
}

func (tracker *deviceTableDoubleClickTracker) activate(row int, now time.Time) bool {
	if row == tracker.row && !tracker.at.IsZero() && now.Sub(tracker.at) <= deviceTableDoubleClickWindow {
		tracker.row = -1
		tracker.at = time.Time{}
		return true
	}
	tracker.row = row
	tracker.at = now
	return false
}

func newDeviceTableCell() *fyne.Container {
	label := widget.NewLabel("")
	label.Truncation = fyne.TextTruncateEllipsis

	dnaLabel := widget.NewLabel("")
	dnaLabel.Truncation = fyne.TextTruncateEllipsis
	copyControl := wrapWinUISecondaryButton(
		widget.NewButtonWithIcon("", fyneTheme.ContentCopyIcon(), nil),
		40,
	)
	dnaContent := container.NewBorder(
		nil,
		nil,
		nil,
		container.NewCenter(copyControl.Object),
		dnaLabel,
	)
	dnaContent.Hide()

	detailsControl := newWinUISecondaryButton("Detailed Info", 116, nil)
	detailsContent := container.NewCenter(detailsControl.Object)
	detailsContent.Hide()

	return newStandardTableCell(label, dnaContent, detailsContent)
}

func deviceTableCellContent(object fyne.CanvasObject) *deviceTableCell {
	container := object.(*fyne.Container)
	content := standardTableCellContent(container)
	label := content.Objects[0].(*widget.Label)
	dnaContent := content.Objects[1].(*fyne.Container)
	dnaLabel := dnaContent.Objects[0].(*widget.Label)
	copyArea := dnaContent.Objects[1].(*fyne.Container)
	copyObject := copyArea.Objects[0]
	detailsContent := content.Objects[2].(*fyne.Container)
	detailsObject := detailsContent.Objects[0]
	return &deviceTableCell{
		container:      container,
		label:          label,
		dnaLabel:       dnaLabel,
		dnaContent:     dnaContent,
		copyButton:     winUISecondaryButtonWidget(copyObject),
		detailsContent: detailsContent,
		detailsButton:  winUISecondaryButtonWidget(detailsObject),
	}
}

func (cell *deviceTableCell) reset() {
	cell.copyButton.OnTapped = nil
	cell.detailsButton.OnTapped = nil
	cell.label.Hide()
	cell.dnaContent.Hide()
	cell.detailsContent.Hide()
}

func deviceTableRows(devices []deviceResult) []deviceTableRow {
	rows := make([]deviceTableRow, 0, len(devices))
	for _, device := range devices {
		board := strings.Join(device.BoardMatches, ", ")
		if board == "" {
			board = "Unknown board"
		}
		dnaID := device.DeviceDNA
		if dnaID == "" {
			dnaID = device.FuseDNA
		}
		rows = append(rows, deviceTableRow{
			Device:    device,
			Index:     device.Index,
			IndexText: fmt.Sprintf("%d", device.Index),
			FPGA:      strings.ToUpper(partFamily(device.Part)),
			IDCode:    device.IDCode,
			DNAID:     dnaID,
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
		{Title: "#", Width: 48, Sortable: true, Alignment: fyne.TextAlignCenter},
		{Title: "FPGA", Width: 92, Sortable: true, Alignment: fyne.TextAlignCenter},
		{Title: "IDCODE", Width: 126, Sortable: true, Alignment: fyne.TextAlignCenter},
		{Title: "DNA ID", Width: 270, Sortable: true},
		{Title: "Board", Width: 250, Sortable: true},
		{Width: 138, Alignment: fyne.TextAlignCenter},
	}
	rows := []deviceTableRow{}
	sortState := newTableSortState(-1, true)

	var table *widget.Table
	table = widget.NewTable(
		func() (int, int) {
			return len(rows), len(columns)
		},
		func() fyne.CanvasObject {
			return newDeviceTableCell()
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			cell := deviceTableCellContent(object)
			cell.reset()
			defer cell.container.Refresh()
			if id.Row < 0 || id.Row >= len(rows) {
				cell.label.Show()
				cell.label.SetText("")
				return
			}
			row := rows[id.Row]
			if id.Col == len(columns)-1 {
				cell.detailsContent.Show()
				cell.detailsButton.OnTapped = func() {
					status.SetText("Opening detailed FPGA information…")
					state.showFPGADeviceDetails(row.Device)
				}
				return
			}
			if id.Col == 3 {
				cell.dnaLabel.SetText(row.DNAID)
				cell.dnaContent.Show()
				cell.copyButton.OnTapped = func() {
					state.window.Clipboard().SetContent(row.DNAID)
					status.SetText("DNA ID copied to the clipboard: " + row.DNAID)
				}
				return
			}
			cell.label.Alignment = columns[id.Col].Alignment
			cell.label.Show()
			switch id.Col {
			case 0:
				cell.label.SetText(row.IndexText)
			case 1:
				cell.label.SetText(row.FPGA)
			case 2:
				cell.label.SetText(row.IDCode)
			default:
				cell.label.SetText(row.Board)
			}
		},
	)
	configureSortableTable(table, columns, &sortState, func() {
		sortTableRows(rows, sortState, compareDeviceTableRows)
	})
	doubleClick := deviceTableDoubleClickTracker{row: -1}
	table.OnSelected = func(id widget.TableCellID) {
		table.UnselectAll()
		if id.Row < 0 || id.Row >= len(rows) || !doubleClick.activate(id.Row, time.Now()) {
			return
		}
		status.SetText("Opening detailed FPGA information…")
		state.showFPGADeviceDetails(rows[id.Row].Device)
	}

	var scanButton *widget.Button
	scanButton = widget.NewButton("Scan device", func() {
		scanButton.Disable()
		doubleClick = deviceTableDoubleClickTracker{row: -1}
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
