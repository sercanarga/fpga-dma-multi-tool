package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type filePickerItem struct {
	name     string
	path     string
	isDir    bool
	isParent bool
	size     int64
	modified time.Time
}

type filePickerLocation struct {
	name string
	path string
	icon fyne.Resource
}

func programmingFilePickerItems(directory string) ([]filePickerItem, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	items := make([]filePickerItem, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			extension := strings.ToLower(filepath.Ext(entry.Name()))
			if extension != ".bin" && extension != ".bit" {
				continue
			}
		}
		item := filePickerItem{
			name:  entry.Name(),
			path:  filepath.Join(directory, entry.Name()),
			isDir: entry.IsDir(),
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			item.size = info.Size()
			item.modified = info.ModTime()
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].isDir != items[j].isDir {
			return items[i].isDir
		}
		return strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
	})
	return items, nil
}

func addProgrammingPickerParent(directory string, items []filePickerItem) []filePickerItem {
	cleanDirectory := filepath.Clean(directory)
	parentDirectory := filepath.Dir(cleanDirectory)
	if parentDirectory == cleanDirectory {
		return items
	}
	withParent := make([]filePickerItem, 0, len(items)+1)
	withParent = append(withParent, filePickerItem{
		name:     "..",
		path:     parentDirectory,
		isDir:    true,
		isParent: true,
	})
	return append(withParent, items...)
}

func resolveProgrammingPickerPath(value string) (string, bool, error) {
	path := strings.Trim(strings.TrimSpace(value), `"`)
	if path == "" {
		return "", false, errors.New("Enter a folder or .bin/.bit file path.")
	}
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", false, err
	}
	if info.IsDir() {
		return absolute, true, nil
	}
	extension := strings.ToLower(filepath.Ext(absolute))
	if extension != ".bin" && extension != ".bit" {
		return "", false, errors.New("Select a .bin or .bit file.")
	}
	return absolute, false, nil
}

func programmingPickerStartDirectory(value string) string {
	if path, isDir, err := resolveProgrammingPickerPath(value); err == nil {
		if isDir {
			return path
		}
		return filepath.Dir(path)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	if workingDirectory, err := os.Getwd(); err == nil {
		return workingDirectory
	}
	return string(filepath.Separator)
}

func programmingPickerLocations() []filePickerLocation {
	locations := make([]filePickerLocation, 0, 12)
	seen := make(map[string]bool)
	add := func(name, path string, icon fyne.Resource) {
		if path == "" || seen[strings.ToLower(filepath.Clean(path))] {
			return
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return
		}
		seen[strings.ToLower(filepath.Clean(path))] = true
		locations = append(locations, filePickerLocation{name: name, path: path, icon: icon})
	}

	home, _ := os.UserHomeDir()
	add("Home", home, fyneTheme.HomeIcon())
	add("Desktop", filepath.Join(home, "Desktop"), fyneTheme.DesktopIcon())
	add("Documents", filepath.Join(home, "Documents"), fyneTheme.DocumentIcon())
	add("Downloads", filepath.Join(home, "Downloads"), fyneTheme.DownloadIcon())

	if runtime.GOOS == "windows" {
		for letter := 'A'; letter <= 'Z'; letter++ {
			volume := fmt.Sprintf("%c:\\", letter)
			add(fmt.Sprintf("Local Disk (%c:)", letter), volume, fyneTheme.StorageIcon())
		}
	} else {
		add("Computer", string(filepath.Separator), fyneTheme.ComputerIcon())
	}
	return locations
}

func formatFilePickerSize(item filePickerItem) string {
	if item.isDir {
		return ""
	}
	const (
		kilobyte = int64(1024)
		megabyte = kilobyte * 1024
		gigabyte = megabyte * 1024
	)
	switch {
	case item.size >= gigabyte:
		return fmt.Sprintf("%.1f GB", float64(item.size)/float64(gigabyte))
	case item.size >= megabyte:
		return fmt.Sprintf("%.1f MB", float64(item.size)/float64(megabyte))
	default:
		kilobytes := (item.size + kilobyte - 1) / kilobyte
		return fmt.Sprintf("%d KB", kilobytes)
	}
}

func formatFilePickerModified(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("01/02/2006 03:04 PM")
}

func filePickerType(item filePickerItem) string {
	if item.isDir {
		return "File folder"
	}
	switch strings.ToLower(filepath.Ext(item.name)) {
	case ".bin":
		return "BIN file"
	case ".bit":
		return "BIT file"
	default:
		return "File"
	}
}

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

func newFilePickerHeader() fyne.CanvasObject {
	label := func(text string) *widget.Label {
		return widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	return container.New(
		&filePickerColumnsLayout{},
		widget.NewIcon(fyneTheme.FileIcon()),
		label("Name"),
		label("Date modified"),
		label("Type"),
		label("Size"),
	)
}

func showProgrammingFilePicker(parent fyne.Window, initialPath string, onSelected func(string)) {
	locationEntry := widget.NewEntry()
	locationEntry.SetPlaceHolder("Type a folder or .bin/.bit file path")
	fileNameEntry := widget.NewEntry()
	fileNameEntry.SetPlaceHolder("File name")
	status := widget.NewLabel("")

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
	fileArea := container.NewBorder(newFilePickerHeader(), nil, nil, nil, fileList)
	mainArea := container.NewHSplit(sidebar, fileArea)
	mainArea.SetOffset(0.22)

	fileNameRow := container.NewBorder(
		nil,
		nil,
		widget.NewLabel("File name:"),
		container.NewHBox(cancelButton.Object, openButton),
		fileNameEntry,
	)
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
