package main

import (
	"image/color"
	"io"
	"net/url"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

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

type winUISegmentedControl struct {
	Object   *fyne.Container
	options  []string
	buttons  map[string]*widget.Button
	selected string
}

func newWinUISegmentedControl(options []string) *winUISegmentedControl {
	control := &winUISegmentedControl{
		options: append([]string(nil), options...),
		buttons: make(map[string]*widget.Button, len(options)),
	}
	objects := make([]fyne.CanvasObject, 0, len(options))
	for _, option := range control.options {
		option := option
		button := widget.NewButton(option, func() {
			control.SetSelected(option)
		})
		control.buttons[option] = button
		objects = append(objects, button)
	}
	columns := len(objects)
	if columns == 0 {
		columns = 1
	}
	control.Object = container.NewGridWithColumns(columns, objects...)
	if len(control.options) > 0 {
		control.SetSelected(control.options[0])
	}
	return control
}

func (control *winUISegmentedControl) SetSelected(value string) {
	if _, exists := control.buttons[value]; !exists {
		return
	}
	control.selected = value
	for option, button := range control.buttons {
		if option == value {
			button.Importance = widget.HighImportance
		} else {
			button.Importance = widget.MediumImportance
		}
		button.Refresh()
	}
}

func (control *winUISegmentedControl) Value() string {
	return control.selected
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
