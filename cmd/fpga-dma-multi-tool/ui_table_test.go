package main

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	fyneTest "fyne.io/fyne/v2/test"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestTableSortStateTogglesDirectionAndIndicator(t *testing.T) {
	state := newTableSortState(-1, true)
	if got := state.headerText("Device", 2, true); got != "Device" {
		t.Fatalf("inactive header = %q, want no indicator", got)
	}

	state.toggle(2)
	if !state.active() || state.column != 2 || !state.ascending {
		t.Fatalf("first toggle state = %+v, want column 2 ascending", state)
	}
	if got := state.headerText("Device", 2, true); got != "Device  ^" {
		t.Fatalf("ascending header = %q, want ascending indicator", got)
	}

	state.toggle(2)
	if state.ascending {
		t.Fatal("second toggle should select descending order")
	}
	if got := state.headerText("Device", 2, true); got != "Device  v" {
		t.Fatalf("descending header = %q, want descending indicator", got)
	}

	state.toggle(1)
	if state.column != 1 || !state.ascending {
		t.Fatalf("new column state = %+v, want column 1 ascending", state)
	}
}

func TestWinUITableThemeUsesCrispAdaptiveDividers(t *testing.T) {
	base := newWinUITheme()
	tableTheme := &winUITableTheme{base: base}
	if got := tableTheme.Size(fyneTheme.SizeNamePadding); got != 1 {
		t.Fatalf("table cell gap = %v, want 1", got)
	}
	if got := tableTheme.Size(fyneTheme.SizeNameSeparatorThickness); got != 1 {
		t.Fatalf("table separator thickness = %v, want 1", got)
	}
	if got := tableTheme.Size(fyneTheme.SizeNameSelectionRadius); got != 0 {
		t.Fatalf("table selection radius = %v, want 0", got)
	}
	for _, test := range []struct {
		variant fyne.ThemeVariant
		want    color.NRGBA
	}{
		{variant: fyneTheme.VariantLight, want: winUILightPalette.controlBorder},
		{variant: fyneTheme.VariantDark, want: winUIDarkPalette.controlBorder},
	} {
		got := color.NRGBAModel.Convert(
			tableTheme.Color(fyneTheme.ColorNameSeparator, test.variant),
		).(color.NRGBA)
		if got != test.want {
			t.Fatalf("separator for variant=%d = %#v, want %#v", test.variant, got, test.want)
		}
	}
}

func TestSortTableRowsIsStableAndReversible(t *testing.T) {
	type row struct {
		name     string
		position int
	}
	rows := []row{
		{name: "Beta", position: 0},
		{name: "alpha", position: 1},
		{name: "alpha", position: 2},
	}
	state := newTableSortState(0, true)
	sortTableRows(rows, state, func(left, right row, _ int) int {
		return compareTableText(left.name, right.name)
	})
	if rows[0].position != 1 || rows[1].position != 2 || rows[2].position != 0 {
		t.Fatalf("ascending order = %+v", rows)
	}

	state.toggle(0)
	sortTableRows(rows, state, func(left, right row, _ int) int {
		return compareTableText(left.name, right.name)
	})
	if rows[0].name != "Beta" || rows[1].position != 1 || rows[2].position != 2 {
		t.Fatalf("descending order = %+v", rows)
	}
}

func TestConfigureSortableTableUsesDedicatedHeaderRow(t *testing.T) {
	application := fyneTest.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(newWinUITheme())

	rows := []string{"B", "A"}
	state := newTableSortState(-1, true)
	sorted := false
	table := widget.NewTable(
		func() (int, int) { return len(rows), 1 },
		func() fyne.CanvasObject {
			return newStandardTableCell(widget.NewLabel(""))
		},
		func(widget.TableCellID, fyne.CanvasObject) {},
	)
	configureSortableTable(
		table,
		[]sortableTableColumn{{Title: "Name", Width: 180, Sortable: true}},
		&state,
		func() { sorted = true },
	)

	if !table.ShowHeaderRow {
		t.Fatal("sortable table should use the dedicated sticky header row")
	}
	header := table.CreateHeader().(*sortableTableHeader)
	table.UpdateHeader(widget.TableCellID{Row: -1, Col: 0}, header)
	if header.label.Text != "Name" {
		t.Fatalf("header text = %q, want no inactive sort indicator", header.label.Text)
	}
	header.target.Tapped(nil)
	if !sorted || state.column != 0 || !state.ascending {
		t.Fatalf("header tap state = %+v, sorted = %v", state, sorted)
	}
	if got := table.CreateCell().MinSize().Height; got < tableRowHeight {
		t.Fatalf("table row minimum height = %v, want at least %v", got, tableRowHeight)
	}
}
