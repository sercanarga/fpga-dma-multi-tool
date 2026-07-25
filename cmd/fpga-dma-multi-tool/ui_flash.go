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

func (state *guiState) buildProgrammingTab() fyne.CanvasObject {
	selectedFilePath := ""
	filePath := widget.NewEntry()
	filePath.SetPlaceHolder("Select a .bin or .bit file")
	mode := widget.NewRadioGroup([]string{"Flash (.bin, persistent)", "SRAM (.bit, temporary)"}, nil)
	mode.Horizontal = true
	mode.Required = true
	mode.SetSelected("Flash (.bin, persistent)")
	part := newWinUIChoice(
		[]string{"XC7A15T", "XC7A35T", "XC7A50T", "XC7A75T", "XC7A100T", "XC7A200T"},
	)
	part.SetSelected("XC7A100T")
	state.programPart = part
	writerSelector := newWinUISegmentedControl([]string{"Auto", "CH347", "RS232"})
	status, statusBar := newStatusBar("Ready. Programming starts only after confirmation.")
	logEntry := widget.NewMultiLineEntry()
	logEntry.SetPlaceHolder("Progress will appear here.")
	logEntry.Wrapping = fyne.TextWrapWord
	logEntry.Disable()

	openFilePicker := func() {
		showProgrammingFilePicker(state.window, selectedFilePath, func(path string) {
			selectedFilePath = path
			filePath.SetText(path)
			switch strings.ToLower(fileExtension(path)) {
			case ".bin":
				mode.SetSelected("Flash (.bin, persistent)")
			case ".bit":
				mode.SetSelected("SRAM (.bit, temporary)")
			}
		})
	}
	chooseButton := newWinUISecondaryButton("Choose file", 104, openFilePicker)
	fileField := container.NewStack(filePath, newTapTarget(openFilePicker))

	var programButton *widget.Button
	programButton = widget.NewButton("Program FPGA", func() {
		selectedMode := programFlash
		if strings.HasPrefix(mode.Selected, "SRAM") {
			selectedMode = programSRAM
		}
		state.resultMu.RLock()
		chainIndex := 0
		if len(state.result.Devices) > 0 {
			for _, device := range state.result.Devices {
				if strings.EqualFold(partFamily(device.Part), part.Value()) {
					chainIndex = device.Index
					break
				}
			}
		}
		state.resultMu.RUnlock()
		cable := autoProgrammingCable
		switch writerSelector.Value() {
		case "CH347":
			cable = directCH347ProgrammingCable
		case "RS232":
			cable = rs232ProgrammingCable
		}
		request := programRequest{
			FilePath: selectedFilePath, Mode: selectedMode,
			Cable:    cable,
			FPGAPart: strings.ToLower(part.Value()), ChainIndex: chainIndex,
		}
		if err := request.validate(); err != nil {
			showWinUIError(state.window, err)
			return
		}
		action := "load this file into temporary SRAM"
		if selectedMode == programFlash {
			action = "erase and write the board's persistent flash"
		}
		message := fmt.Sprintf(
			"This will %s.\n\nFile: %s\nFPGA: %s\nWriter: %s",
			action, request.FilePath, part.Value(), writerSelector.Value(),
		)
		showWinUIConfirm(state.window, "Confirm programming", message, "Program", false, func() {
			programButton.Disable()
			logEntry.SetText("")
			status.SetText("Programming… Do not disconnect the board.")
			writer := newSynchronizedWriter(logEntry)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				defer cancel()
				err := runProgramming(ctx, request, writer)
				fyne.Do(func() {
					programButton.Enable()
					if err != nil {
						status.SetText("Programming failed: " + err.Error())
						showWinUIError(state.window, err)
						return
					}
					status.SetText("Programming completed successfully.")
				})
			}()
		})
	})
	programButton.Importance = widget.HighImportance

	form := widget.NewForm(
		widget.NewFormItem("File", container.NewBorder(nil, nil, nil, chooseButton.Object, fileField)),
		widget.NewFormItem("Target", outlinedSelect(part.Select)),
		widget.NewFormItem("Writer", writerSelector.Object),
		widget.NewFormItem("Mode", mode),
	)
	writerHint := widget.NewLabel("Auto uses CH347 first, then falls back to RS232.")
	writerHint.Wrapping = fyne.TextWrapWord
	controls := container.NewVBox(
		form,
		writerHint,
		container.NewBorder(
			nil,
			nil,
			nil,
			programButton,
			nil,
		),
		newSectionTitle("Activity"),
	)
	body := container.NewBorder(controls, nil, nil, nil, logEntry)
	return newPageFrame(
		"Flash",
		"Load a bitstream into temporary SRAM or write persistent flash.",
		body,
		statusBar,
	)
}
