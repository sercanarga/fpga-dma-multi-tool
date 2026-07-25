package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (state *guiState) buildDeviceHistoryTab() fyne.CanvasObject {
	status, statusBar := newStatusBar("Scan Windows device history to begin.")
	summary := widget.NewLabelWithStyle(
		"Not scanned",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	selectedDetails := widget.NewLabel("Select a disconnected device-history entry to remove it.")
	selectedDetails.Wrapping = fyne.TextWrapWord
	headers := []string{"", "State", "Class", "Source", "Device", "Device ID"}
	var allEntries []deviceHistoryEntry
	var entries []deviceHistoryEntry
	selectedIDs := make(map[string]bool)
	busy := false

	var table *widget.Table
	var scanButton, removeButton *widget.Button
	var refreshSelectionUI func()
	setBusy := func(value bool) {
		busy = value
		for _, button := range []*widget.Button{scanButton, removeButton} {
			if value {
				button.Disable()
			} else {
				button.Enable()
			}
		}
		if !value {
			refreshSelectionUI()
		}
	}

	table = widget.NewTable(
		func() (int, int) {
			return len(entries) + 1, len(headers)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			check := widget.NewCheck("", nil)
			check.Hide()
			return container.NewStack(label, check)
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			cell := object.(*fyne.Container)
			label := cell.Objects[0].(*widget.Label)
			check := cell.Objects[1].(*widget.Check)
			if id.Col == 0 {
				label.Hide()
				check.Show()
				check.OnChanged = nil
				if id.Row == 0 {
					selectable := 0
					selected := 0
					for _, entry := range entries {
						if entry.Present {
							continue
						}
						selectable++
						if selectedIDs[deviceHistoryInstanceID(entry)] {
							selected++
						}
					}
					check.Enable()
					check.SetChecked(selectable > 0 && selected == selectable)
					check.OnChanged = func(checked bool) {
						for _, entry := range entries {
							if !entry.Present {
								selectedIDs[deviceHistoryInstanceID(entry)] = checked
							}
						}
						refreshSelectionUI()
					}
					return
				}
				index := id.Row - 1
				if index < 0 || index >= len(entries) {
					check.Disable()
					check.SetChecked(false)
					return
				}
				entry := entries[index]
				instanceID := deviceHistoryInstanceID(entry)
				check.SetChecked(selectedIDs[instanceID])
				if entry.Present {
					check.Disable()
					return
				}
				check.Enable()
				check.OnChanged = func(checked bool) {
					selectedIDs[instanceID] = checked
					refreshSelectionUI()
				}
				return
			}
			check.Hide()
			label.Show()
			if id.Row == 0 {
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.SetText(headers[id.Col])
				return
			}
			label.TextStyle = fyne.TextStyle{}
			index := id.Row - 1
			if index < 0 || index >= len(entries) {
				label.SetText("")
				return
			}
			entry := entries[index]
			stateText := "Disconnected"
			if entry.Present {
				stateText = "Connected"
			}
			class := entry.Class
			if strings.TrimSpace(class) == "" {
				class = "Unknown"
			}
			values := []string{
				"", stateText, class, entry.Enumerator, entry.FriendlyName, entry.DeviceID,
			}
			label.SetText(values[id.Col])
		},
	)
	for column, width := range []float32{42, 100, 115, 90, 250, 230} {
		table.SetColumnWidth(column, width)
	}
	table.SetRowHeight(0, tableHeaderHeight)
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 {
			table.UnselectAll()
			return
		}
		index := id.Row - 1
		if index < 0 || index >= len(entries) {
			return
		}
		entry := entries[index]
		selectedDetails.SetText(fmt.Sprintf(
			"%s  •  %s  •  %s",
			entry.FriendlyName,
			entry.Class,
			deviceHistoryInstanceID(entry),
		))
		if entry.Present {
			status.SetText("Disconnect this device before removing its history.")
		} else {
			status.SetText("Use the checkbox to include this entry in removal.")
		}
	}

	selectedEntries := func() []deviceHistoryEntry {
		selected := make([]deviceHistoryEntry, 0, len(selectedIDs))
		for _, entry := range allEntries {
			if !entry.Present && selectedIDs[deviceHistoryInstanceID(entry)] {
				selected = append(selected, entry)
			}
		}
		return selected
	}
	refreshSelectionUI = func() {
		count := len(selectedEntries())
		if count == 0 {
			removeButton.Disable()
			selectedDetails.SetText("Select one or more disconnected entries to remove.")
		} else {
			if !busy {
				removeButton.Enable()
			}
			selectedDetails.SetText(fmt.Sprintf("%d disconnected entries selected.", count))
		}
		table.Refresh()
	}

	showConnected := widget.NewCheck("Show connected devices", nil)
	search := widget.NewEntry()
	search.SetPlaceHolder("Filter by device, class, source, or ID")
	applyFilter := func() {
		table.UnselectAll()
		entries = filterDeviceHistoryEntries(allEntries, showConnected.Checked, search.Text)
		refreshSelectionUI()
		if len(allEntries) == 0 {
			summary.SetText("No device-history entries found")
			return
		}
		disconnected := 0
		for _, entry := range allEntries {
			if !entry.Present {
				disconnected++
			}
		}
		if strings.TrimSpace(search.Text) != "" {
			summary.SetText(fmt.Sprintf(
				"%d matching entries  •  %d total  •  %d disconnected",
				len(entries),
				len(allEntries),
				disconnected,
			))
			return
		}
		if showConnected.Checked {
			summary.SetText(fmt.Sprintf(
				"%d entries  •  %d connected  •  %d disconnected",
				len(allEntries),
				len(allEntries)-disconnected,
				disconnected,
			))
			return
		}
		summary.SetText(fmt.Sprintf("%d disconnected entries", disconnected))
	}
	showConnected.OnChanged = func(bool) {
		applyFilter()
		status.SetText("Device history filter updated.")
	}
	search.OnChanged = func(string) {
		applyFilter()
		status.SetText("Device history filter updated.")
	}

	var scan func()
	scan = func() {
		selectedIDs = make(map[string]bool)
		table.UnselectAll()
		setBusy(true)
		summary.SetText("Scanning…")
		selectedDetails.SetText("Refreshing Windows devices and reading Plug and Play history.")
		status.SetText("Refreshing devices and scanning history…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			rescanErr := rescanWindowsDevices(ctx)
			scanned, err := scanDeviceHistory(ctx)
			fyne.Do(func() {
				if err != nil {
					allEntries = nil
					entries = nil
					table.Refresh()
					summary.SetText("Scan failed")
					selectedDetails.SetText("Windows device history could not be read.")
					setBusy(false)
					status.SetText("Device history scan failed.")
					showWinUIError(state.window, err)
					return
				}
				allEntries = scanned
				applyFilter()
				setBusy(false)
				if rescanErr != nil {
					status.SetText("Device history loaded. Hardware rescan was unavailable without administrator access.")
				} else {
					status.SetText("Device history scan completed.")
				}
			})
		}()
	}

	scanButton = widget.NewButton("Scan", scan)
	scanButton.Importance = widget.HighImportance
	removeButton = widget.NewButton("Remove selected", func() {
		selected := selectedEntries()
		if len(selected) == 0 {
			return
		}
		names := make([]string, 0, 5)
		for index, entry := range selected {
			if index == 4 {
				names = append(names, fmt.Sprintf("…and %d more", len(selected)-index))
				break
			}
			names = append(names, "• "+entry.FriendlyName)
		}
		message := fmt.Sprintf(
			"Remove %d disconnected Windows device-history entries?\n\n%s\n\nEach removal is verified against Windows PnP data and the active device registry.",
			len(selected),
			strings.Join(names, "\n"),
		)
		showWinUIConfirm(
			state.window,
			"Remove selected device history",
			message,
			"Remove",
			true,
			func() {
				setBusy(true)
				status.SetText("Removing selected device history…")
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer cancel()
					removed := 0
					var failures []string
					for _, entry := range selected {
						if err := removeDeviceHistory(ctx, entry); err != nil {
							failures = append(failures, fmt.Sprintf("%s: %v", entry.FriendlyName, err))
							continue
						}
						removed++
					}
					fyne.Do(func() {
						scan()
						if len(failures) > 0 {
							showWinUIDialog(
								state.window,
								"Device history cleanup incomplete",
								fmt.Sprintf(
									"Removed and verified %d of %d entries.\n\n%s",
									removed,
									len(selected),
									strings.Join(failures, "\n"),
								),
								winUIDialogError,
							)
							return
						}
						showWinUIInformation(
							state.window,
							"Device history removed",
							fmt.Sprintf("%d disconnected entries were removed and verified.", removed),
						)
					})
				}()
			},
		)
	})
	removeButton.Importance = widget.DangerImportance
	removeButton.Disable()
	actions := container.NewHBox(scanButton, removeButton)
	top := container.NewVBox(
		container.NewBorder(nil, nil, summary, actions, nil),
		container.NewBorder(nil, nil, showConnected, nil, search),
		selectedDetails,
	)
	body := container.NewBorder(top, nil, nil, nil, table)
	return newPageFrame(
		"Device History",
		"Review Windows Plug and Play history or remove disconnected entries.",
		body,
		statusBar,
	)
}
