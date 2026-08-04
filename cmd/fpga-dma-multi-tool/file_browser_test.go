package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	fyneTest "fyne.io/fyne/v2/test"
)

func TestProgrammingFilePickerItemsSortsFoldersAndFiltersFiles(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"zeta", "Alpha"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"firmware.BIT", "board.bin", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	items, err := programmingFilePickerItems(directory)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Alpha", "zeta", "board.bin", "firmware.BIT"}
	if len(items) != len(want) {
		t.Fatalf("items = %v, want %v", items, want)
	}
	for index, name := range want {
		if items[index].name != name {
			t.Fatalf("items[%d].name = %q, want %q", index, items[index].name, name)
		}
	}
	if !items[0].isDir || !items[1].isDir || items[2].isDir || items[3].isDir {
		t.Fatalf("directory ordering is incorrect: %v", items)
	}
}

func TestProgrammingFilePickerSortKeepsParentAndFoldersFirst(t *testing.T) {
	items := []filePickerItem{
		{name: "large.bin", size: 400},
		{name: "folder", isDir: true},
		{name: "..", isDir: true, isParent: true},
		{name: "small.bit", size: 100},
	}
	sortProgrammingPickerItems(items, 3, true)
	wantAscending := []string{"..", "folder", "small.bit", "large.bin"}
	for index, want := range wantAscending {
		if items[index].name != want {
			t.Fatalf("ascending items[%d] = %q, want %q", index, items[index].name, want)
		}
	}

	sortProgrammingPickerItems(items, 3, false)
	wantDescending := []string{"..", "folder", "large.bin", "small.bit"}
	for index, want := range wantDescending {
		if items[index].name != want {
			t.Fatalf("descending items[%d] = %q, want %q", index, items[index].name, want)
		}
	}
}

func TestProgrammingFilePickerSortsModifiedTime(t *testing.T) {
	older := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	items := []filePickerItem{
		{name: "new.bin", modified: newer},
		{name: "old.bin", modified: older},
	}
	sortProgrammingPickerItems(items, 1, true)
	if items[0].name != "old.bin" || items[1].name != "new.bin" {
		t.Fatalf("modified-time order = %v", items)
	}
}

func TestFilePickerHeaderTogglesSortDirection(t *testing.T) {
	application := fyneTest.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(newWinUITheme())

	state := newTableSortState(-1, true)
	called := false
	header := newFilePickerHeader(&state, func() { called = true })
	if header.labels[0].Text != "Name" {
		t.Fatalf("initial header = %q", header.labels[0].Text)
	}
	header.targets[0].Tapped(nil)
	if !called || !state.ascending || header.labels[0].Text != "Name  ^" {
		t.Fatalf(
			"toggled header = %q, state=%+v, called=%v",
			header.labels[0].Text,
			state,
			called,
		)
	}
	header.targets[0].Tapped(nil)
	if state.ascending || header.labels[0].Text != "Name  v" {
		t.Fatalf("second toggle header = %q, state=%+v", header.labels[0].Text, state)
	}
}

func TestResolveProgrammingPickerPathAcceptsDirectoryAndFirmware(t *testing.T) {
	directory := t.TempDir()
	firmware := filepath.Join(directory, "firmware.bin")
	if err := os.WriteFile(firmware, []byte{0xaa}, 0o600); err != nil {
		t.Fatal(err)
	}

	resolvedDirectory, isDir, err := resolveProgrammingPickerPath(directory)
	if err != nil || !isDir || resolvedDirectory != directory {
		t.Fatalf("directory resolved to %q, isDir=%v, err=%v", resolvedDirectory, isDir, err)
	}
	resolvedFile, isDir, err := resolveProgrammingPickerPath(`"` + firmware + `"`)
	if err != nil || isDir || resolvedFile != firmware {
		t.Fatalf("file resolved to %q, isDir=%v, err=%v", resolvedFile, isDir, err)
	}
}

func TestResolveProgrammingPickerPathRejectsUnsupportedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("not firmware"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolveProgrammingPickerPath(path); err == nil {
		t.Fatal("unsupported file was accepted")
	}
}

func TestProgrammingPickerStartDirectoryUsesSelectedFileParent(t *testing.T) {
	directory := t.TempDir()
	firmware := filepath.Join(directory, "firmware.bit")
	if err := os.WriteFile(firmware, []byte{0xaa}, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := programmingPickerStartDirectory(firmware); got != directory {
		t.Fatalf("start directory = %q, want %q", got, directory)
	}
}

func TestFilePickerRowSupportsSingleAndDoubleClick(t *testing.T) {
	row := newFilePickerRow()
	selected := 0
	opened := false
	row.tapped = func() { selected++ }
	row.opened = func() { opened = true }

	row.MouseDown(nil)
	if selected != 1 {
		t.Fatal("mouse down did not immediately select the row")
	}
	row.DoubleTapped(nil)
	if !opened {
		t.Fatal("double click did not activate the row")
	}
}

func TestFilePickerDetailsFormatting(t *testing.T) {
	folder := filePickerItem{isDir: true}
	if got := filePickerType(folder); got != "File folder" {
		t.Fatalf("folder type = %q", got)
	}
	if got := formatFilePickerSize(folder); got != "" {
		t.Fatalf("folder size = %q", got)
	}
	firmware := filePickerItem{name: "firmware.bin", size: 1536}
	if got := filePickerType(firmware); got != "BIN file" {
		t.Fatalf("firmware type = %q", got)
	}
	if got := formatFilePickerSize(firmware); got != "2 KB" {
		t.Fatalf("firmware size = %q", got)
	}
}

func TestAddProgrammingPickerParentAddsDotDotExceptAtRoot(t *testing.T) {
	directory := t.TempDir()
	items := []filePickerItem{{name: "firmware.bin"}}
	withParent := addProgrammingPickerParent(directory, items)
	if len(withParent) != 2 || withParent[0].name != ".." || !withParent[0].isParent {
		t.Fatalf("parent row was not added: %v", withParent)
	}
	if withParent[0].path != filepath.Dir(directory) {
		t.Fatalf("parent path = %q, want %q", withParent[0].path, filepath.Dir(directory))
	}

	root := filepath.Clean(string(filepath.Separator))
	atRoot := addProgrammingPickerParent(root, items)
	if len(atRoot) != len(items) || atRoot[0].name != items[0].name {
		t.Fatalf("root unexpectedly received a parent row: %v", atRoot)
	}
}
