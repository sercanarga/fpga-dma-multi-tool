package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (state *guiState) buildDeviceTab() fyne.CanvasObject {
	summary := widget.NewLabelWithStyle("Ready to scan", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	status, statusBar := newStatusBar("Connect the adapter and power the FPGA board.")
	headers := []string{"#", "FPGA", "IDCODE", "DNA ID", "Board", ""}

	var table *widget.Table
	table = widget.NewTable(
		func() (int, int) {
			state.resultMu.RLock()
			defer state.resultMu.RUnlock()
			return len(state.result.Devices) + 1, len(headers)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			button := newWinUISecondaryButton("Copy", 88, nil)
			button.Hide()
			return container.NewStack(label, button.Object)
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			cell := object.(*fyne.Container)
			label := cell.Objects[0].(*widget.Label)
			buttonObject := cell.Objects[1]
			button := winUISecondaryButtonWidget(buttonObject)
			if id.Row == 0 {
				buttonObject.Hide()
				label.Show()
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.SetText(headers[id.Col])
				return
			}
			if id.Col == len(headers)-1 {
				label.Hide()
				buttonObject.Show()
				row := id.Row - 1
				button.OnTapped = func() {
					state.resultMu.RLock()
					defer state.resultMu.RUnlock()
					if row >= len(state.result.Devices) {
						return
					}
					encoded, err := json.MarshalIndent(state.result.Devices[row], "", "  ")
					if err != nil {
						showWinUIError(state.window, err)
						return
					}
					state.window.Clipboard().SetContent(string(encoded))
					status.SetText("Device details copied.")
				}
				return
			}
			buttonObject.Hide()
			label.Show()
			label.TextStyle = fyne.TextStyle{}
			state.resultMu.RLock()
			defer state.resultMu.RUnlock()
			if id.Row-1 >= len(state.result.Devices) {
				label.SetText("")
				return
			}
			device := state.result.Devices[id.Row-1]
			board := strings.Join(device.BoardMatches, ", ")
			if board == "" {
				board = "Unknown board"
			}
			values := []string{
				fmt.Sprintf("%d", device.Index),
				strings.ToUpper(partFamily(device.Part)),
				device.IDCode,
				device.FuseDNA,
				board,
			}
			label.SetText(values[id.Col])
		},
	)
	for column, width := range []float32{48, 88, 118, 176, 280, 88} {
		table.SetColumnWidth(column, width)
	}
	table.SetRowHeight(0, tableHeaderHeight)

	var scanButton *widget.Button
	scanButton = widget.NewButton("Scan device", func() {
		scanButton.Disable()
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
				for row := 1; row <= len(devices); row++ {
					table.SetRowHeight(row, tableRowHeight)
				}
				if state.programPart != nil && len(devices) > 0 {
					if detected := partFamily(devices[0].Part); detected != "" {
						state.programPart.SetSelected(strings.ToUpper(detected))
					}
				}
				table.Refresh()
				summary.SetText(deviceScanSummary(devices))
				status.SetText("Scan complete. Use Copy to copy one device's details.")
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
		table,
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
