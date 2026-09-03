package main

import (
	"cmp"
	"image/color"
	"math"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type tableSortState struct {
	column    int
	ascending bool
}

func newTableSortState(column int, ascending bool) tableSortState {
	return tableSortState{column: column, ascending: ascending}
}

func (state tableSortState) active() bool {
	return state.column >= 0
}

func (state *tableSortState) toggle(column int) {
	if state.column == column {
		state.ascending = !state.ascending
		return
	}
	state.column = column
	state.ascending = true
}

func (state tableSortState) headerText(title string, column int, sortable bool) string {
	if !sortable {
		return title
	}
	if state.column != column {
		return title
	}
	if state.ascending {
		return title + "  ^"
	}
	return title + "  v"
}

type sortableTableColumn struct {
	Title           string
	Width           float32
	Sortable        bool
	Alignment       fyne.TextAlign
	ConfigureHeader func(*sortableTableHeader)
}

type sortableTableHeader struct {
	widget.BaseWidget

	label   *widget.Label
	check   *widget.Check
	target  *tapTarget
	content *fyne.Container
}

func newSortableTableHeader() *sortableTableHeader {
	header := &sortableTableHeader{}
	header.label = widget.NewLabelWithStyle(
		"",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	header.label.Truncation = fyne.TextTruncateEllipsis
	header.check = widget.NewCheck("", nil)
	header.check.Hide()
	header.target = newTapTarget(nil)
	header.target.Hide()
	labelArea := container.New(
		layout.NewCustomPaddedLayout(0, 0, 6, 6),
		header.label,
	)
	checkArea := container.NewCenter(header.check)
	header.content = container.NewStack(labelArea, checkArea, header.target)
	header.ExtendBaseWidget(header)
	return header
}

func (header *sortableTableHeader) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(header.content)
}

func (header *sortableTableHeader) showLabel(text string, tapped func()) {
	header.check.OnChanged = nil
	header.check.Hide()
	header.label.SetText(text)
	header.label.Show()
	header.target.tapped = tapped
	if tapped == nil {
		header.target.Hide()
	} else {
		header.target.Show()
	}
}

func (header *sortableTableHeader) showCheck(
	checked, enabled bool,
	changed func(bool),
) {
	header.target.tapped = nil
	header.target.Hide()
	header.label.Hide()
	header.check.OnChanged = nil
	header.check.SetChecked(checked)
	if enabled {
		header.check.Enable()
	} else {
		header.check.Disable()
	}
	header.check.OnChanged = changed
	header.check.Show()
}

func configureSortableTable(
	table *widget.Table,
	columns []sortableTableColumn,
	sortState *tableSortState,
	onSort func(),
) {
	table.ShowHeaderRow = true
	table.CreateHeader = func() fyne.CanvasObject {
		return newSortableTableHeader()
	}
	table.UpdateHeader = func(id widget.TableCellID, object fyne.CanvasObject) {
		if id.Row >= 0 || id.Col < 0 || id.Col >= len(columns) {
			return
		}
		columnIndex := id.Col
		column := columns[columnIndex]
		header := object.(*sortableTableHeader)
		header.label.Alignment = column.Alignment
		if column.ConfigureHeader != nil {
			column.ConfigureHeader(header)
			return
		}
		var tapped func()
		if column.Sortable {
			tapped = func() {
				sortState.toggle(columnIndex)
				if onSort != nil {
					onSort()
				}
				table.UnselectAll()
				table.Refresh()
			}
		}
		header.showLabel(
			sortState.headerText(column.Title, columnIndex, column.Sortable),
			tapped,
		)
	}
	table.SetRowHeight(-1, tableHeaderHeight)
	for index, column := range columns {
		table.SetColumnWidth(index, column.Width)
	}
}

func sortTableRows[T any](
	rows []T,
	state tableSortState,
	compare func(left, right T, column int) int,
) {
	if !state.active() || len(rows) < 2 {
		return
	}
	slices.SortStableFunc(rows, func(left, right T) int {
		result := compare(left, right, state.column)
		switch {
		case result < 0:
			result = -1
		case result > 0:
			result = 1
		}
		if !state.ascending {
			result = -result
		}
		return result
	})
}

func compareTableText(left, right string) int {
	leftFolded := strings.ToLower(strings.TrimSpace(left))
	rightFolded := strings.ToLower(strings.TrimSpace(right))
	if result := cmp.Compare(leftFolded, rightFolded); result != 0 {
		return result
	}
	return cmp.Compare(left, right)
}

func compareTableInt(left, right int) int {
	return cmp.Compare(left, right)
}

func newStandardTableCell(objects ...fyne.CanvasObject) *fyne.Container {
	stack := container.NewStack(objects...)
	return container.New(
		&minimumHeightLayout{height: tableRowHeight, horizontalPadding: 6},
		stack,
	)
}

func standardTableCellContent(object fyne.CanvasObject) *fyne.Container {
	cell := object.(*fyne.Container)
	return cell.Objects[0].(*fyne.Container)
}

type winUITableTheme struct {
	base fyne.Theme
}

func (theme *winUITableTheme) Color(
	name fyne.ThemeColorName,
	variant fyne.ThemeVariant,
) color.Color {
	if name == fyneTheme.ColorNameSeparator {
		return theme.base.Color(fyneTheme.ColorNameInputBorder, variant)
	}
	return theme.base.Color(name, variant)
}

func (theme *winUITableTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.base.Font(style)
}

func (theme *winUITableTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.base.Icon(name)
}

func (theme *winUITableTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case fyneTheme.SizeNamePadding,
		fyneTheme.SizeNameSeparatorThickness:
		return 1
	case fyneTheme.SizeNameSelectionRadius:
		return 0
	default:
		return theme.base.Size(name)
	}
}

func newTableViewport(table *widget.Table) fyne.CanvasObject {
	baseTheme := fyne.CurrentApp().Settings().Theme()
	themedTable := container.NewThemeOverride(
		table,
		&winUITableTheme{base: baseTheme},
	)
	tableContent := container.New(
		layout.NewCustomPaddedLayout(1, 1, 1, 1),
		themedTable,
	)
	background := canvas.NewRectangle(currentWinUIThemeColor(winUIColorSurface))
	background.CornerRadius = 6
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = currentWinUIThemeColor(winUIColorControlBorder)
	border.StrokeWidth = 1
	border.CornerRadius = 6
	return container.NewStack(background, tableContent, border)
}

type fillTableColumnLayout struct {
	table        *widget.Table
	column       int
	fixedWidth   float32
	minimumWidth float32
	columnCount  int
	lastWidth    float32
}

func fillTableColumnWidth(viewportWidth, fixedWidth, minimumWidth float32, columnCount int) float32 {
	// The viewport uses one pixel of inner padding on each side and the
	// table theme uses one pixel between adjacent columns.
	width := viewportWidth - fixedWidth - 2 - float32(columnCount-1)
	if width < minimumWidth {
		return minimumWidth
	}
	return width
}

func (layout *fillTableColumnLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	width := fillTableColumnWidth(size.Width, layout.fixedWidth, layout.minimumWidth, layout.columnCount)
	if math.Abs(float64(width-layout.lastWidth)) >= 0.5 {
		layout.lastWidth = width
		layout.table.SetColumnWidth(layout.column, width)
	}
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(size)
}

func (layout *fillTableColumnLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	height := float32(0)
	if len(objects) > 0 {
		height = objects[0].MinSize().Height
	}
	return fyne.NewSize(
		layout.fixedWidth+layout.minimumWidth+2+float32(layout.columnCount-1),
		height,
	)
}

func newFillTableColumnViewport(table *widget.Table, columns []sortableTableColumn, column int) fyne.CanvasObject {
	minimumWidth := columns[column].Width
	fixedWidth := float32(0)
	for index, definition := range columns {
		if index != column {
			fixedWidth += definition.Width
		}
	}
	viewport := newTableViewport(table)
	return container.New(
		&fillTableColumnLayout{
			table:        table,
			column:       column,
			fixedWidth:   fixedWidth,
			minimumWidth: minimumWidth,
			columnCount:  len(columns),
			lastWidth:    minimumWidth,
		},
		viewport,
	)
}

type minimumHeightLayout struct {
	height            float32
	horizontalPadding float32
}

var _ fyne.Theme = (*winUITableTheme)(nil)

func (layout *minimumHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		width := size.Width - layout.horizontalPadding*2
		if width < 0 {
			width = 0
		}
		object.Move(fyne.NewPos(layout.horizontalPadding, 0))
		object.Resize(fyne.NewSize(width, size.Height))
	}
}

func (layout *minimumHeightLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, layout.height)
	for _, object := range objects {
		minimum := object.MinSize()
		minimum.Width += layout.horizontalPadding * 2
		if minimum.Width > size.Width {
			size.Width = minimum.Width
		}
		if minimum.Height > size.Height {
			size.Height = minimum.Height
		}
	}
	return size
}
