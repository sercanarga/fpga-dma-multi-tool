package main

import (
	"os"
	"path/filepath"
	"testing"
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
