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

func deviceHistoryStateText(entry deviceHistoryEntry) string {
	if entry.Present {
		return "Connected"
	}
	return "Disconnected"
}

func deviceHistoryClassText(entry deviceHistoryEntry) string {
	if class := strings.TrimSpace(entry.Class); class != "" {
		return class
	}
	return "Unknown"
}

func compareDeviceHistoryTableRows(
	left, right deviceHistoryEntry,
	column int,
) int {
	switch column {
	case 1:
		return compareTableText(deviceHistoryStateText(left), deviceHistoryStateText(right))
	case 2:
		return compareTableText(deviceHistoryClassText(left), deviceHistoryClassText(right))
	case 3:
		return compareTableText(left.Enumerator, right.Enumerator)
	case 4:
		return compareTableText(left.FriendlyName, right.FriendlyName)
	default:
		return compareTableText(left.DeviceID, right.DeviceID)
	}
}

func (state *guiState) buildDeviceHistoryTab() fyne.CanvasObject {
	status, statusBar := newStatusBar("Scan Windows device history to begin.")
	summary := widget.NewLabelWithStyle(
		"Not scanned",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	columns := []sortableTableColumn{
		{Width: 38},
		{Title: "State", Width: 88, Sortable: true},
		{Title: "Class", Width: 98, Sortable: true},
		{Title: "Source", Width: 78, Sortable: true},
		{Title: "Device", Width: 220, Sortable: true},
		{Title: "Device ID", Width: 198, Sortable: true},
	}
	var allEntries []deviceHistoryEntry
	var entries []deviceHistoryEntry
	selectedIDs := make(map[string]bool)
	sortState := newTableSortState(-1, true)
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
		} else if table != nil {
			table.Refresh()
		}
	}

	table = widget.NewTable(
		func() (int, int) {
			return len(entries), len(columns)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			check := widget.NewCheck("", nil)
			check.Hide()
			return newStandardTableCell(label, check)
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			cell := standardTableCellContent(object)
			label := cell.Objects[0].(*widget.Label)
			check := cell.Objects[1].(*widget.Check)
			if id.Col == 0 {
				label.Hide()
				check.Show()
				check.OnChanged = nil
				if id.Row < 0 || id.Row >= len(entries) {
					check.Disable()
					check.SetChecked(false)
					return
				}
				entry := entries[id.Row]
				instanceID := deviceHistoryInstanceID(entry)
				check.SetChecked(selectedIDs[instanceID])
				if entry.Present || busy {
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
			if id.Row < 0 || id.Row >= len(entries) {
				label.SetText("")
				return
			}
			entry := entries[id.Row]
			switch id.Col {
			case 1:
				label.SetText(deviceHistoryStateText(entry))
			case 2:
				label.SetText(deviceHistoryClassText(entry))
			case 3:
				label.SetText(entry.Enumerator)
			case 4:
				label.SetText(entry.FriendlyName)
			default:
				label.SetText(entry.DeviceID)
			}
		},
	)
	columns[0].ConfigureHeader = func(header *sortableTableHeader) {
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
		header.showCheck(
			selectable > 0 && selected == selectable,
			selectable > 0 && !busy,
			func(checked bool) {
				for _, entry := range entries {
					if !entry.Present {
						selectedIDs[deviceHistoryInstanceID(entry)] = checked
					}
				}
				refreshSelectionUI()
			},
		)
	}
	configureSortableTable(table, columns, &sortState, func() {
		sortTableRows(entries, sortState, compareDeviceHistoryTableRows)
	})
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row < 0 || id.Row >= len(entries) {
			return
		}
		entry := entries[id.Row]
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
		} else {
			if !busy {
				removeButton.Enable()
			}
		}
		table.Refresh()
	}

	showConnected := widget.NewCheck("Show connected devices", nil)
	search := widget.NewEntry()
	search.SetPlaceHolder("Filter by device, class, source, or ID")
	applyFilter := func() {
		table.UnselectAll()
		entries = filterDeviceHistoryEntries(allEntries, showConnected.Checked, search.Text)
		sortTableRows(entries, sortState, compareDeviceHistoryTableRows)
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
	)
	body := container.NewBorder(top, nil, nil, nil, newTableViewport(table))
	return newPageFrame(
		"Device History",
		"Review Windows Plug and Play history or remove disconnected entries.",
		body,
		statusBar,
	)
}
