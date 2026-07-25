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
