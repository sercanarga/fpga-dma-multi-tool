package main

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func (state *guiState) buildSetupTab() fyne.CanvasObject {
	status, statusBar := newStatusBar("Checking components…")
	type driverRow struct {
		stateLabel    *canvas.Text
		detailLabel   *widget.Label
		installObject fyne.CanvasObject
		removeButton  *winUISecondaryButton
	}

	driverRows := make(map[string]*driverRow, 3)
	var setupButtons []*widget.Button
	setSetupBusy := func(busy bool) {
		for _, button := range setupButtons {
			if busy {
				button.Disable()
			} else {
				button.Enable()
			}
		}
	}

	var refresh func()
	refresh = func() {
		setSetupBusy(true)
		status.SetText("Checking installed drivers…")
		go func() {
			statuses := inspectSystemComponents("", "")
			fyne.Do(func() {
				installed := 0
				for _, component := range statuses {
					row := driverRows[component.Name]
					if row == nil {
						continue
					}
					if component.Installed {
						installed++
						row.stateLabel.Text = "Installed"
						row.stateLabel.Color = winUILightPalette.textPrimary
						row.installObject.Hide()
						row.removeButton.Show()
					} else {
						row.stateLabel.Text = "Not installed"
						row.stateLabel.Color = winUILightPalette.error
						row.removeButton.Hide()
						row.installObject.Show()
					}
					row.stateLabel.Refresh()
					row.detailLabel.SetText(component.Details)
				}
				setSetupBusy(false)
				if installed == len(statuses) {
					status.SetText("All drivers are installed.")
				} else {
					status.SetText(fmt.Sprintf("%d of %d drivers installed.", installed, len(statuses)))
				}
			})
		}()
	}

	runSetupAction := func(progressText string, action func(context.Context) error) {
		setSetupBusy(true)
		status.SetText(progressText)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			err := action(ctx)
			fyne.Do(func() {
				if err != nil {
					setSetupBusy(false)
					status.SetText("Driver setup did not complete.")
					showWinUIError(state.window, err)
					return
				}
				refresh()
			})
		}()
	}

	installWCH := widget.NewButton("Install", func() {
		runSetupAction(
			"Installing WCH CH341/CH347 driver…",
			func(ctx context.Context) error { return runBundledWCHSetup(ctx, false) },
		)
	})
	installWCH.Importance = widget.HighImportance
	removeWCH := newWinUISecondaryButton("Remove", 88, func() {
		showWinUIConfirm(
			state.window,
			"Remove WCH CH341/CH347 driver",
			"Remove the WCH parallel/JTAG driver installed by this package?",
			"Remove",
			true,
			func() {
				runSetupAction(
					"Removing WCH CH341/CH347 driver…",
					func(ctx context.Context) error { return runBundledWCHSetup(ctx, true) },
				)
			},
		)
	})
	installFTDI := widget.NewButton("Install", func() {
		runSetupAction("Installing FTDI D3XX driver…", installFTDID3XX)
	})
	installFTDI.Importance = widget.HighImportance
	removeFTDI := newWinUISecondaryButton("Remove", 88, func() {
		showWinUIConfirm(
			state.window,
			"Remove FTDI D3XX driver",
			"Remove the FTDI D3XX driver package? Other FTDI serial drivers are not changed.",
			"Remove",
			true,
			func() {
				runSetupAction("Removing FTDI D3XX driver…", uninstallFTDID3XX)
			},
		)
	})
	installRS232 := widget.NewButton("Install", func() {
		showWinUIConfirm(
			state.window,
			"Install RS232 writer driver",
			"Keep the RS232 writer connected. In Zadig, select only Quad RS232-HS/FT232H Interface 0 (0403:6011 or 0403:6014), choose WinUSB, then click Install/Replace Driver.",
			"Open installer",
			false,
			func() {
				runSetupAction("Installing RS232 writer WinUSB driver…", installRS232Driver)
			},
		)
	})
	installRS232.Importance = widget.HighImportance
	removeRS232 := newWinUISecondaryButton("Remove", 88, func() {
		showWinUIConfirm(
			state.window,
			"Remove RS232 writer driver",
			"Remove only the WinUSB packages for FTDI 0403:6011/6014 Interface 0? Other FTDI interfaces and the D3XX driver are not changed.",
			"Remove",
			true,
			func() {
				runSetupAction("Removing RS232 writer WinUSB driver…", uninstallRS232Driver)
			},
		)
	})
	refreshButton := newWinUISecondaryButton("Check again", 104, refresh)

	newDriverRow := func(
		name string,
		purpose string,
		installButton *widget.Button,
		removeButton *winUISecondaryButton,
	) fyne.CanvasObject {
		stateLabel := canvas.NewText("Checking…", winUILightPalette.textSecondary)
		stateLabel.TextSize = 14
		stateLabel.TextStyle = fyne.TextStyle{Bold: true}
		stateLabelObject := container.New(
			layout.NewCustomPaddedLayout(4, 4, 4, 4),
			stateLabel,
		)
		purposeLabel := widget.NewLabel(purpose)
		purposeLabel.Wrapping = fyne.TextWrapWord
		detailLabel := widget.NewLabel("")
		detailLabel.Wrapping = fyne.TextWrapWord
		installObject := container.New(
			&minimumWidthLayout{width: 88},
			installButton,
		)
		installObject.Hide()
		removeButton.Hide()
		actions := container.NewStack(installObject, removeButton.Object)
		driverRows[name] = &driverRow{
			stateLabel:    stateLabel,
			detailLabel:   detailLabel,
			installObject: installObject,
			removeButton:  removeButton,
		}
		header := container.NewBorder(
			nil, nil, nil, actions,
			newSectionTitle(name),
		)
		cardContent := container.NewVBox(header, stateLabelObject, purposeLabel, detailLabel)
		background := canvas.NewRectangle(winUILightPalette.surface)
		background.CornerRadius = 6
		return container.NewStack(
			background,
			container.New(
				layout.NewCustomPaddedLayout(10, 10, 12, 12),
				cardContent,
			),
		)
	}
	setupButtons = []*widget.Button{
		installWCH, removeWCH.Button,
		installFTDI, removeFTDI.Button,
		installRS232, removeRS232.Button,
		refreshButton.Button,
	}

	toolbar := container.NewBorder(
		nil, nil, newSectionTitle("Windows drivers"), refreshButton.Object, nil,
	)
	content := container.NewVBox(
		toolbar,
		newDriverRow(
			"CH347 driver",
			"Used for FPGA detection and programming over JTAG.",
			installWCH,
			removeWCH,
		),
		newDriverRow(
			"FTDI D3XX driver",
			"Used for DMA Speed Test with FT600/FT601 adapters.",
			installFTDI,
			removeFTDI,
		),
		newDriverRow(
			"RS232 writer driver",
			"Uses WinUSB on FTDI Interface A for FPGA detection and programming with RS232 writer boards.",
			installRS232,
			removeRS232,
		),
	)
	go func() {
		time.Sleep(100 * time.Millisecond)
		fyne.Do(refresh)
	}()
	return newPageFrame(
		"Drivers",
		"Check application components and manage the required Windows drivers.",
		content,
		statusBar,
	)
}
