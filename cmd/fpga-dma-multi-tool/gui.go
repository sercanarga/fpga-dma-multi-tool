package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const applicationID = "com.sercanarga.fpgadmamultitool"
const repositoryURL = "https://github.com/sercanarga/fpga-dma-multi-tool"

const (
	tableHeaderHeight float32 = 34
	tableRowHeight    float32 = 34
)

var (
	statusBarBackground = winUILightPalette.backgroundSecondary
	statusBarBorder     = winUILightPalette.controlBorder
)

type guiState struct {
	window fyne.Window

	resultMu    sync.RWMutex
	result      scanResult
	programPart *winUIChoice
}

func launchGUI() {
	application := app.NewWithID(applicationID)
	application.Settings().SetTheme(newWinUITheme())
	window := application.NewWindow("FPGA DMA Multi Tool")
	state := &guiState{window: window}

	tabs := container.NewAppTabs(
		container.NewTabItem("Devices", state.buildDeviceTab()),
		container.NewTabItem("Flash", state.buildProgrammingTab()),
		container.NewTabItem("Speed Test", state.buildSpeedTestTab()),
		container.NewTabItem("Device History", state.buildDeviceHistoryTab()),
		container.NewTabItem("Drivers", state.buildSetupTab()),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	if setupRequired(inspectSystemComponents("", "")) {
		tabs.SelectIndex(4)
	}
	window.SetContent(tabs)
	window.Resize(fyne.NewSize(860, 590))
	window.CenterOnScreen()
	window.ShowAndRun()
}

func newPageTitle(text string) *widget.Label {
	title := widget.NewLabelWithStyle(
		text,
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	title.SizeName = fyneTheme.SizeNameSubHeadingText
	return title
}

func newSectionTitle(text string) *widget.Label {
	return widget.NewLabelWithStyle(
		text,
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
}

func outlinedSelect(selectControl *widget.Select) fyne.CanvasObject {
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = winUILightPalette.controlBorder
	border.StrokeWidth = 1
	border.CornerRadius = 4
	themedSelect := container.NewThemeOverride(
		selectControl,
		&winUISelectTheme{base: fyne.CurrentApp().Settings().Theme()},
	)
	return container.NewStack(
		border,
		container.New(
			layout.NewCustomPaddedLayout(1, 1, 1, 1),
			themedSelect,
		),
	)
}

type winUIChoice struct {
	Select  *widget.Select
	options []string
}

func newWinUIChoice(options []string) *winUIChoice {
	choice := &winUIChoice{options: append([]string(nil), options...)}
	choice.Select = widget.NewSelect(choice.options, func(string) {
		choice.hideSelectedOption()
	})
	return choice
}

func (choice *winUIChoice) SetSelected(value string) {
	choice.Select.SetOptions(choice.options)
	choice.Select.SetSelected(value)
	choice.hideSelectedOption()
}

func (choice *winUIChoice) Value() string {
	return choice.Select.Selected
}

func (choice *winUIChoice) hideSelectedOption() {
	options := make([]string, 0, len(choice.options))
	for _, option := range choice.options {
		if option != choice.Select.Selected {
			options = append(options, option)
		}
	}
	choice.Select.SetOptions(options)
}

type tapTarget struct {
	widget.BaseWidget
	tapped func()
}

func newTapTarget(tapped func()) *tapTarget {
	target := &tapTarget{tapped: tapped}
	target.ExtendBaseWidget(target)
	return target
}

func (target *tapTarget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (target *tapTarget) Tapped(*fyne.PointEvent) {
	if target.tapped != nil {
		target.tapped()
	}
}

func (*tapTarget) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

type winUISecondaryButton struct {
	Object *fyne.Container
	Button *widget.Button
}

func (control *winUISecondaryButton) Show() {
	control.Object.Show()
}

func (control *winUISecondaryButton) Hide() {
	control.Object.Hide()
}

func newWinUISecondaryButton(
	text string,
	minimumWidth float32,
	tapped func(),
) *winUISecondaryButton {
	return wrapWinUISecondaryButton(widget.NewButton(text, tapped), minimumWidth)
}

func wrapWinUISecondaryButton(
	button *widget.Button,
	minimumWidth float32,
) *winUISecondaryButton {
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = winUILightPalette.controlBorder
	border.StrokeWidth = 1
	border.CornerRadius = 4
	surface := container.NewStack(button, border)
	control := container.New(
		&minimumWidthLayout{width: minimumWidth},
		surface,
	)
	return &winUISecondaryButton{Object: control, Button: button}
}

func winUISecondaryButtonWidget(object fyne.CanvasObject) *widget.Button {
	control := object.(*fyne.Container)
	surface := control.Objects[0].(*fyne.Container)
	return surface.Objects[0].(*widget.Button)
}

type winUIDialogType int

const (
	winUIDialogInformation winUIDialogType = iota
	winUIDialogWarning
	winUIDialogError
)

func showWinUIDialog(
	parent fyne.Window,
	title, message string,
	dialogType winUIDialogType,
) {
	titleColor := winUILightPalette.textPrimary
	if dialogType == winUIDialogWarning {
		titleColor = winUILightPalette.warning
	} else if dialogType == winUIDialogError {
		titleColor = winUILightPalette.error
	}
	var popup *widget.PopUp
	closeButton := widget.NewButton("Close", func() { popup.Hide() })
	closeButton.Importance = widget.HighImportance
	popup = newWinUICustomPopup(
		parent,
		title,
		message,
		titleColor,
		container.NewHBox(closeButton),
	)
	popup.Show()
}

func showWinUIError(parent fyne.Window, err error) {
	showWinUIDialog(parent, "Something went wrong", err.Error(), winUIDialogError)
}

func showWinUIInformation(parent fyne.Window, title, message string) {
	showWinUIDialog(parent, title, message, winUIDialogInformation)
}

func showWinUIConfirm(
	parent fyne.Window,
	title, message, confirmText string,
	danger bool,
	confirmed func(),
) {
	var popup *widget.PopUp
	cancelButton := newWinUISecondaryButton("Cancel", 88, func() { popup.Hide() })
	confirmButton := widget.NewButton(confirmText, func() {
		popup.Hide()
		confirmed()
	})
	confirmButton.Importance = widget.HighImportance
	titleColor := winUILightPalette.textPrimary
	if danger {
		confirmButton.Importance = widget.DangerImportance
		titleColor = winUILightPalette.warning
	}
	popup = newWinUICustomPopup(
		parent,
		title,
		message,
		titleColor,
		container.NewHBox(cancelButton.Object, confirmButton),
	)
	popup.Show()
}

func newWinUICustomPopup(
	parent fyne.Window,
	title, message string,
	titleColor color.Color,
	actions fyne.CanvasObject,
) *widget.PopUp {
	titleText := canvas.NewText(title, titleColor)
	titleText.TextSize = 20
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleObject := container.New(
		layout.NewCustomPaddedLayout(4, 4, 4, 4),
		titleText,
	)
	messageLabel := widget.NewLabel(message)
	messageLabel.Alignment = fyne.TextAlignLeading
	messageLabel.Wrapping = fyne.TextWrapWord
	actionObject := container.New(
		layout.NewCustomPaddedLayout(4, 0, 4, 0),
		actions,
	)
	content := container.New(
		layout.NewCustomPaddedLayout(14, 14, 14, 14),
		container.NewVBox(
			titleObject,
			messageLabel,
			actionObject,
		),
	)
	background := canvas.NewRectangle(winUILightPalette.surface)
	background.CornerRadius = 8
	card := container.NewStack(background, content)
	return widget.NewModalPopUp(
		container.New(&minimumWidthLayout{width: 440}, card),
		parent.Canvas(),
	)
}

type minimumWidthLayout struct {
	width float32
}

func (layout *minimumWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		object.Move(fyne.NewPos(0, 0))
		object.Resize(size)
	}
}

func (layout *minimumWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(layout.width, 0)
	for _, object := range objects {
		minimum := object.MinSize()
		if minimum.Width > size.Width {
			size.Width = minimum.Width
		}
		if minimum.Height > size.Height {
			size.Height = minimum.Height
		}
	}
	return size
}

func newPageHeader(title, subtitle string) fyne.CanvasObject {
	description := widget.NewLabel(subtitle)
	description.Wrapping = fyne.TextWrapWord
	return container.NewVBox(
		newPageTitle(title),
		description,
	)
}

type statusBarText struct {
	text *canvas.Text
}

func (text *statusBarText) SetText(value string) {
	text.text.Text = value
	text.text.Refresh()
}

func newStatusBar(initial string) (*statusBarText, fyne.CanvasObject) {
	label := canvas.NewText(initial, winUILightPalette.textPrimary)
	label.Alignment = fyne.TextAlignLeading
	label.TextSize = 12
	centeredLabel := container.New(&statusLabelLayout{horizontalPadding: 8}, label)
	target, _ := url.Parse(repositoryURL)
	linkLabel := canvas.NewText("GitHub", winUILightPalette.accent)
	linkLabel.Alignment = fyne.TextAlignTrailing
	linkLabel.TextSize = 12
	linkTarget := newTapTarget(func() {
		_ = fyne.CurrentApp().OpenURL(target)
	})
	linkWidth := linkLabel.MinSize().Width + 16
	linkTextArea := container.New(
		&statusLabelLayout{horizontalPadding: 8, minimumWidth: linkWidth},
		linkLabel,
	)
	linkArea := container.NewStack(linkTextArea, linkTarget)
	background := canvas.NewRectangle(statusBarBackground)
	content := container.NewBorder(nil, nil, centeredLabel, linkArea, nil)
	bar := container.NewStack(background, content)
	topBorder := canvas.NewRectangle(statusBarBorder)
	framed := container.NewBorder(
		container.New(&fixedHeightLayout{height: 1}, topBorder),
		nil,
		nil,
		nil,
		bar,
	)
	return &statusBarText{text: label}, container.New(&fixedHeightLayout{height: 24}, framed)
}

type statusLabelLayout struct {
	horizontalPadding float32
	minimumWidth      float32
}

func (layout *statusLabelLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		height := object.MinSize().Height
		if height > size.Height {
			height = size.Height
		}
		width := size.Width - (layout.horizontalPadding * 2)
		if width < 0 {
			width = 0
		}
		object.Move(fyne.NewPos(layout.horizontalPadding, (size.Height-height)/2))
		object.Resize(fyne.NewSize(width, height))
	}
}

func (layout *statusLabelLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	width := layout.minimumWidth
	if width == 0 {
		width = 120
	}
	return fyne.NewSize(width, 24)
}

type fixedHeightLayout struct {
	height float32
}

func (layout *fixedHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		object.Move(fyne.NewPos(0, 0))
		object.Resize(fyne.NewSize(size.Width, layout.height))
	}
}

func (layout *fixedHeightLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(120, layout.height)
}

func newPageFrame(
	title, subtitle string,
	body, statusBar fyne.CanvasObject,
) fyne.CanvasObject {
	content := container.NewBorder(
		newPageHeader(title, subtitle),
		nil,
		nil,
		nil,
		body,
	)
	paddedContent := container.New(
		layout.NewCustomPaddedLayout(12, 12, 14, 14),
		content,
	)
	return container.NewBorder(
		nil,
		statusBar,
		nil,
		nil,
		paddedContent,
	)
}

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
			opts := options{backend: "ch347", device: -1, ch347Index: -1, timeout: 45 * time.Second}
			var diagnostics bytes.Buffer
			devices, err := scanCH347(ctx, opts, &diagnostics)
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
	default:
		return "Could not read the FPGA. Check the USB cable, JTAG cable, board power, and Drivers tab."
	}
}

func (state *guiState) buildProgrammingTab() fyne.CanvasObject {
	filePath := widget.NewEntry()
	filePath.SetPlaceHolder("Select a .bin file")
	mode := widget.NewRadioGroup([]string{"Flash (.bin, persistent)", "SRAM (.bit, temporary)"}, nil)
	mode.Horizontal = true
	mode.Required = true
	mode.SetSelected("Flash (.bin, persistent)")
	part := newWinUIChoice(
		[]string{"XC7A15T", "XC7A35T", "XC7A50T", "XC7A75T", "XC7A100T", "XC7A200T"},
	)
	part.SetSelected("XC7A100T")
	state.programPart = part
	status, statusBar := newStatusBar("Ready. Programming starts only after confirmation.")
	logEntry := widget.NewMultiLineEntry()
	logEntry.SetPlaceHolder("Progress will appear here.")
	logEntry.Wrapping = fyne.TextWrapWord
	logEntry.Disable()

	openFilePicker := func() {
		picker := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				showWinUIError(state.window, err)
				return
			}
			if reader == nil {
				return
			}
			path := reader.URI().Path()
			_ = reader.Close()
			filePath.SetText(path)
			switch strings.ToLower(fileExtension(path)) {
			case ".bin":
				mode.SetSelected("Flash (.bin, persistent)")
			case ".bit":
				mode.SetSelected("SRAM (.bit, temporary)")
			}
		}, state.window)
		picker.SetFilter(storage.NewExtensionFileFilter([]string{".bin", ".bit"}))
		picker.Resize(fyne.NewSize(760, 500))
		picker.Show()
	}
	chooseButton := newWinUISecondaryButton("Choose file", 104, openFilePicker)

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
		request := programRequest{
			FilePath: filePath.Text, Mode: selectedMode,
			Cable:    directCH347ProgrammingCable,
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
		message := fmt.Sprintf("This will %s.\n\nFile: %s\nFPGA: %s", action, request.FilePath, part.Value())
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
		widget.NewFormItem("File", container.NewBorder(nil, nil, nil, chooseButton.Object, filePath)),
		widget.NewFormItem("Target", outlinedSelect(part.Select)),
		widget.NewFormItem("Mode", mode),
	)
	controls := container.NewVBox(
		form,
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

func (state *guiState) buildSpeedTestTab() fyne.CanvasObject {
	duration := newWinUIChoice([]string{"3 seconds", "5 seconds", "10 seconds"})
	duration.SetSelected("5 seconds")
	status, statusBar := newStatusBar("Ready. Close memory-heavy applications for consistent results.")
	logEntry := widget.NewMultiLineEntry()
	logEntry.SetPlaceHolder("Test progress will appear here.")
	logEntry.Wrapping = fyne.TextWrapWord
	logEntry.Disable()

	var readButton, readWriteButton *widget.Button
	runTest := func(mode string) {
		seconds := 5
		_, _ = fmt.Sscanf(duration.Value(), "%d", &seconds)
		request := speedTestRequest{
			Mode: mode, Duration: seconds, Sizes: []int{4096, 8192, 16384, 32768},
		}
		start := func() {
			readButton.Disable()
			readWriteButton.Disable()
			duration.Select.Disable()
			logEntry.SetText("")
			status.SetText("Running the memory transfer test…")

			writer := newSynchronizedWriter(logEntry)
			go func() {
				timeout := time.Duration((seconds*len(request.Sizes)*2)+60) * time.Second
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				report, err := runSpeedTest(ctx, request, writer)
				fyne.Do(func() {
					readButton.Enable()
					readWriteButton.Enable()
					duration.Select.Enable()
					if err != nil {
						logEntry.SetText(err.Error())
						status.SetText("Speed test failed: " + err.Error())
						return
					}
					logEntry.SetText(formatSpeedTestReport(report))
					status.SetText(fmt.Sprintf("Completed %d measurements.", len(report.Passes)))
				})
			}()
		}
		if mode == "both" {
			showWinUIConfirm(
				state.window,
				"Confirm memory read/write test",
				"The test temporarily changes a small writable region in explorer.exe and restores the original bytes when it finishes. Continue?",
				"Run test",
				false,
				start,
			)
			return
		}
		start()
	}
	readButton = widget.NewButton("Memory Read", func() { runTest("read") })
	readWriteButton = widget.NewButton("Memory Read + Write", func() { runTest("both") })
	readButton.Importance = widget.HighImportance
	readWriteControl := wrapWinUISecondaryButton(readWriteButton, 152)

	options := container.NewHBox(
		widget.NewLabel("Duration per block"),
		outlinedSelect(duration.Select),
	)
	actions := container.NewHBox(readButton, readWriteControl.Object)
	toolbar := container.NewBorder(nil, nil, options, actions, nil)
	top := container.NewVBox(
		toolbar,
		newSectionTitle("Output"),
	)
	body := container.NewBorder(top, nil, nil, nil, logEntry)
	return newPageFrame(
		"DMA Speed Test",
		"Measure transfer speed, operation rate, and average latency.",
		body,
		statusBar,
	)
}

func (state *guiState) buildDeviceHistoryTab() fyne.CanvasObject {
	status, statusBar := newStatusBar("Scan Windows device history to begin.")
	summary := widget.NewLabelWithStyle(
		"Not scanned",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	selectedDetails := widget.NewLabel("Select a disconnected device-history entry to remove it.")
	selectedDetails.Wrapping = fyne.TextWrapWord
	headers := []string{"", "State", "Type", "Device", "Hardware ID"}
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
			values := []string{"", stateText, entry.Type, entry.FriendlyName, entry.HardwareID}
			label.SetText(values[id.Col])
		},
	)
	for column, width := range []float32{42, 105, 60, 300, 280} {
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
			"%s  •  %s\\%s\\%s",
			entry.FriendlyName,
			entry.Type,
			entry.HardwareID,
			entry.InstanceID,
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
	applyFilter := func() {
		table.UnselectAll()
		entries = filterDeviceHistoryEntries(allEntries, showConnected.Checked)
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

	var scan func()
	scan = func() {
		selectedIDs = make(map[string]bool)
		table.UnselectAll()
		setBusy(true)
		summary.SetText("Scanning…")
		selectedDetails.SetText("Refreshing Windows devices and reading PCI and USB history.")
		status.SetText("Refreshing devices and scanning history…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := rescanWindowsDevices(ctx); err != nil {
				fyne.Do(func() {
					allEntries = nil
					entries = nil
					table.Refresh()
					summary.SetText("Scan failed")
					selectedDetails.SetText("Windows devices could not be refreshed.")
					setBusy(false)
					status.SetText("Device scan failed.")
					showWinUIError(state.window, err)
				})
				return
			}
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
				status.SetText("Device history scan completed.")
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
						if _, err := removeDeviceHistory(ctx, entry); err != nil {
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
		showConnected,
		selectedDetails,
	)
	body := container.NewBorder(top, nil, nil, nil, table)
	return newPageFrame(
		"Device History",
		"Review PCI and USB history or remove disconnected entries.",
		body,
		statusBar,
	)
}

func (state *guiState) buildSetupTab() fyne.CanvasObject {
	status, statusBar := newStatusBar("Checking components…")
	type driverRow struct {
		stateLabel    *canvas.Text
		detailLabel   *widget.Label
		installObject fyne.CanvasObject
		removeButton  *winUISecondaryButton
	}

	driverRows := make(map[string]*driverRow, 2)
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

type synchronizedWriter struct {
	mutex  sync.Mutex
	entry  *widget.Entry
	buffer strings.Builder
}

func newSynchronizedWriter(entry *widget.Entry) *synchronizedWriter {
	return &synchronizedWriter{entry: entry}
}

func (writer *synchronizedWriter) Write(data []byte) (int, error) {
	writer.mutex.Lock()
	_, _ = writer.buffer.Write(data)
	text := writer.buffer.String()
	writer.mutex.Unlock()
	fyne.Do(func() {
		writer.entry.SetText(text)
		writer.entry.CursorRow = strings.Count(text, "\n")
		writer.entry.Refresh()
	})
	return len(data), nil
}

var _ io.Writer = (*synchronizedWriter)(nil)

func fileExtension(path string) string {
	lastDot := strings.LastIndex(path, ".")
	if lastDot < 0 {
		return ""
	}
	return path[lastDot:]
}
