package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type filePickerColumnsLayout struct{}

func (*filePickerColumnsLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 5 {
		return
	}
	const (
		padding   float32 = 6
		iconWidth float32 = 24
		dateWidth float32 = 150
		typeWidth float32 = 100
		sizeWidth float32 = 72
	)
	nameWidth := size.Width - iconWidth - dateWidth - typeWidth - sizeWidth - padding*5
	if nameWidth < 140 {
		nameWidth = 140
	}
	x := padding
	widths := []float32{iconWidth, nameWidth, dateWidth, typeWidth, sizeWidth}
	for index, object := range objects[:5] {
		object.Move(fyne.NewPos(x, 0))
		object.Resize(fyne.NewSize(widths[index], size.Height))
		x += widths[index] + padding
	}
}

func (*filePickerColumnsLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(560, 30)
}

type filePickerRow struct {
	widget.BaseWidget
	icon     *widget.Icon
	name     *widget.Label
	modified *widget.Label
	kind     *widget.Label
	size     *widget.Label
	tapped   func()
	opened   func()
}

func newFilePickerRow() *filePickerRow {
	row := &filePickerRow{
		icon:     widget.NewIcon(fyneTheme.DocumentIcon()),
		name:     widget.NewLabel("firmware.bin"),
		modified: widget.NewLabel("01/01/2000 12:00 PM"),
		kind:     widget.NewLabel("BIN file"),
		size:     widget.NewLabel("1 KB"),
	}
	row.ExtendBaseWidget(row)
	return row
}

func (row *filePickerRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.New(
		&filePickerColumnsLayout{},
		row.icon,
		row.name,
		row.modified,
		row.kind,
		row.size,
	))
}

func (row *filePickerRow) setItem(item filePickerItem) {
	icon := fyneTheme.DocumentIcon()
	if item.isDir {
		icon = fyneTheme.FolderIcon()
	}
	row.icon.SetResource(icon)
	row.name.SetText(item.name)
	row.modified.SetText(formatFilePickerModified(item.modified))
	row.kind.SetText(filePickerType(item))
	row.size.SetText(formatFilePickerSize(item))
}

func (row *filePickerRow) Tapped(*fyne.PointEvent) {
	if row.tapped != nil {
		row.tapped()
	}
}

func (row *filePickerRow) MouseDown(*desktop.MouseEvent) {
	if row.tapped != nil {
		row.tapped()
	}
}

func (*filePickerRow) MouseUp(*desktop.MouseEvent) {}

func (row *filePickerRow) DoubleTapped(*fyne.PointEvent) {
	if row.opened != nil {
		row.opened()
	}
}

func (*filePickerRow) Cursor() desktop.Cursor {
	return desktop.DefaultCursor
}

var filePickerColumnTitles = [...]string{"Name", "Date modified", "Type", "Size"}

type filePickerHeader struct {
	widget.BaseWidget

	sortState *tableSortState
	labels    []*widget.Label
	targets   []*tapTarget
	content   *fyne.Container
	onSort    func()
}

func newFilePickerHeader(
	sortState *tableSortState,
	onSort func(),
) *filePickerHeader {
	header := &filePickerHeader{sortState: sortState, onSort: onSort}
	objects := []fyne.CanvasObject{widget.NewIcon(fyneTheme.FileIcon())}
	for index, title := range filePickerColumnTitles {
		column := index
		label := widget.NewLabelWithStyle(
			title,
			fyne.TextAlignLeading,
			fyne.TextStyle{Bold: true},
		)
		label.Truncation = fyne.TextTruncateEllipsis
		header.labels = append(header.labels, label)
		target := newTapTarget(func() {
			header.sortState.toggle(column)
			header.refreshLabels()
			if header.onSort != nil {
				header.onSort()
			}
		})
		header.targets = append(header.targets, target)
		objects = append(objects, container.NewStack(label, target))
	}
	header.content = container.New(&filePickerColumnsLayout{}, objects...)
	header.ExtendBaseWidget(header)
	header.refreshLabels()
	return header
}

func (header *filePickerHeader) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(header.content)
}

func (header *filePickerHeader) refreshLabels() {
	for index, title := range filePickerColumnTitles {
		header.labels[index].SetText(header.sortState.headerText(title, index, true))
	}
}

func showProgrammingFilePicker(parent fyne.Window, initialPath string, onSelected func(string)) {
	locationEntry := widget.NewEntry()
	locationEntry.SetPlaceHolder("Type a folder or .bin/.bit file path")
	fileNameEntry := widget.NewEntry()
	fileNameEntry.SetPlaceHolder("File name")
	status := widget.NewLabel("")
	sortState := newTableSortState(-1, true)

	var (
		items          []filePickerItem
		selectedIndex  = -1
		currentDir     string
		history        []string
		historyIndex   = -1
		picker         *dialog.CustomDialog
		fileList       *widget.List
		setDirectory   func(string, bool)
		activateItem   func(int)
		chooseFilePath func(string)
	)

	backButton := widget.NewButtonWithIcon("", fyneTheme.NavigateBackIcon(), nil)
	forwardButton := widget.NewButtonWithIcon("", fyneTheme.NavigateNextIcon(), nil)
	upButton := widget.NewButtonWithIcon("", fyneTheme.MoveUpIcon(), nil)
	refreshButton := widget.NewButtonWithIcon("", fyneTheme.ViewRefreshIcon(), nil)
	backButton.Disable()
	forwardButton.Disable()

	openButton := widget.NewButton("Open", nil)
	openButton.Importance = widget.HighImportance
	openButton.Disable()

	updateHistoryButtons := func() {
		if historyIndex > 0 {
			backButton.Enable()
		} else {
			backButton.Disable()
		}
		if historyIndex >= 0 && historyIndex < len(history)-1 {
			forwardButton.Enable()
		} else {
			forwardButton.Disable()
		}
	}

	fileList = widget.NewList(
		func() int {
			return len(items)
		},
		func() fyne.CanvasObject {
			return newFilePickerRow()
		},
		func(id widget.ListItemID, object fyne.CanvasObject) {
			row := object.(*filePickerRow)
			row.setItem(items[id])
			row.tapped = func() {
				fileList.Select(id)
			}
			row.opened = func() {
				activateItem(id)
			}
		},
	)

	setDirectory = func(directory string, recordHistory bool) {
		loaded, err := programmingFilePickerItems(directory)
		if err != nil {
			status.SetText(err.Error())
			return
		}
		cleanDirectory := filepath.Clean(directory)
		items = addProgrammingPickerParent(cleanDirectory, loaded)
		sortProgrammingPickerItems(items, sortState.column, sortState.ascending)
		currentDir = cleanDirectory
		selectedIndex = -1
		fileList.UnselectAll()
		fileList.Refresh()
		locationEntry.SetText(cleanDirectory)
		fileNameEntry.SetText("")
		status.SetText(fmt.Sprintf("%d items", len(loaded)))
		openButton.SetText("Open")
		openButton.Disable()

		if recordHistory && (historyIndex < 0 || history[historyIndex] != cleanDirectory) {
			history = append(history[:historyIndex+1], cleanDirectory)
			historyIndex = len(history) - 1
		}
		updateHistoryButtons()
	}

	chooseFilePath = func(path string) {
		resolved, isDir, err := resolveProgrammingPickerPath(path)
		if err != nil {
			status.SetText(err.Error())
			return
		}
		if isDir {
			setDirectory(resolved, true)
			return
		}
		onSelected(resolved)
		picker.Hide()
	}

	activateItem = func(index int) {
		if index < 0 || index >= len(items) {
			return
		}
		item := items[index]
		if item.isDir {
			setDirectory(item.path, true)
			return
		}
		chooseFilePath(item.path)
	}

	fileList.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(items) {
			return
		}
		selectedIndex = id
		openButton.Enable()
		if items[id].isDir {
			fileNameEntry.SetText("")
			openButton.SetText("Go folder")
		} else {
			fileNameEntry.SetText(items[id].name)
			openButton.SetText("Open")
		}
	}

	navigateLocation := func(value string) {
		path, isDir, err := resolveProgrammingPickerPath(value)
		if err != nil {
			status.SetText(err.Error())
			return
		}
		if isDir {
			setDirectory(path, true)
			return
		}
		chooseFilePath(path)
	}
	locationEntry.OnSubmitted = navigateLocation

	goButton := widget.NewButtonWithIcon("", fyneTheme.NavigateNextIcon(), func() {
		navigateLocation(locationEntry.Text)
	})
	backButton.OnTapped = func() {
		if historyIndex <= 0 {
			return
		}
		historyIndex--
		setDirectory(history[historyIndex], false)
	}
	forwardButton.OnTapped = func() {
		if historyIndex < 0 || historyIndex >= len(history)-1 {
			return
		}
		historyIndex++
		setDirectory(history[historyIndex], false)
	}
	upButton.OnTapped = func() {
		if currentDir == "" {
			return
		}
		parentDirectory := filepath.Dir(currentDir)
		if parentDirectory != currentDir {
			setDirectory(parentDirectory, true)
		}
	}
	refreshButton.OnTapped = func() {
		if currentDir != "" {
			setDirectory(currentDir, false)
		}
	}

	openButton.OnTapped = func() {
		typedName := strings.Trim(strings.TrimSpace(fileNameEntry.Text), `"`)
		if typedName != "" {
			candidate := typedName
			if !filepath.IsAbs(candidate) {
				candidate = filepath.Join(currentDir, candidate)
			}
			chooseFilePath(candidate)
			return
		}
		activateItem(selectedIndex)
	}
	fileNameEntry.OnSubmitted = func(string) {
		openButton.OnTapped()
	}

	cancelButton := newWinUISecondaryButton("Cancel", 88, func() {
		picker.Hide()
	})

	locations := programmingPickerLocations()
	sidebar := widget.NewList(
		func() int {
			return len(locations)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewIcon(fyneTheme.FolderIcon()), widget.NewLabel("Location"))
		},
		func(id widget.ListItemID, object fyne.CanvasObject) {
			row := object.(*fyne.Container)
			row.Objects[0].(*widget.Icon).SetResource(locations[id].icon)
			row.Objects[1].(*widget.Label).SetText(locations[id].name)
		},
	)
	sidebar.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(locations) {
			setDirectory(locations[id].path, true)
			sidebar.UnselectAll()
		}
	}

	navigationButtons := container.NewHBox(backButton, forwardButton, upButton, refreshButton)
	addressBar := container.NewBorder(
		nil,
		nil,
		navigationButtons,
		goButton,
		locationEntry,
	)
	fileNameRow := container.NewBorder(
		nil,
		nil,
		widget.NewLabel("File name:"),
		container.NewHBox(cancelButton.Object, openButton),
		fileNameEntry,
	)
	fileHeader := newFilePickerHeader(&sortState, func() {
		selectedPath := ""
		if selectedIndex >= 0 && selectedIndex < len(items) {
			selectedPath = items[selectedIndex].path
		}
		sortProgrammingPickerItems(items, sortState.column, sortState.ascending)
		fileList.UnselectAll()
		selectedIndex = -1
		if selectedPath != "" {
			for index, item := range items {
				if strings.EqualFold(item.path, selectedPath) {
					fileList.Select(index)
					break
				}
			}
		}
		if selectedIndex < 0 {
			fileNameEntry.SetText("")
			openButton.SetText("Open")
			openButton.Disable()
		}
		fileList.Refresh()
	})
	fileArea := container.NewBorder(fileHeader, nil, nil, nil, fileList)
	mainArea := container.NewHSplit(sidebar, fileArea)
	mainArea.SetOffset(0.22)
	content := container.NewBorder(
		addressBar,
		container.NewVBox(status, fileNameRow),
		nil,
		nil,
		mainArea,
	)

	picker = dialog.NewCustomWithoutButtons("Choose firmware file", content, parent)
	picker.Resize(fyne.NewSize(820, 540))
	setDirectory(programmingPickerStartDirectory(initialPath), true)
	picker.Show()
	parent.Canvas().Focus(locationEntry)
}
