package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	fyneTheme "fyne.io/fyne/v2/theme"
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
	sortProgrammingPickerItems(items, 0, true)
	return items, nil
}

func sortProgrammingPickerItems(
	items []filePickerItem,
	column int,
	ascending bool,
) {
	slices.SortStableFunc(items, func(left, right filePickerItem) int {
		if left.isParent != right.isParent {
			if left.isParent {
				return -1
			}
			return 1
		}
		if left.isDir != right.isDir {
			if left.isDir {
				return -1
			}
			return 1
		}

		result := 0
		switch column {
		case 1:
			result = left.modified.Compare(right.modified)
		case 2:
			result = compareFilePickerText(filePickerType(left), filePickerType(right))
		case 3:
			result = cmp.Compare(left.size, right.size)
		default:
			result = compareFilePickerText(left.name, right.name)
		}
		if result == 0 {
			result = compareFilePickerText(left.name, right.name)
		}
		if !ascending {
			result = -result
		}
		return result
	})
}

func compareFilePickerText(left, right string) int {
	leftFolded := strings.ToLower(strings.TrimSpace(left))
	rightFolded := strings.ToLower(strings.TrimSpace(right))
	if result := strings.Compare(leftFolded, rightFolded); result != 0 {
		return result
	}
	return strings.Compare(left, right)
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
