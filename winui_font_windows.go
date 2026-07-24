//go:build windows

package main

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
)

func loadWinUIFont(name string) fyne.Resource {
	windowsDirectory := os.Getenv("WINDIR")
	if windowsDirectory == "" {
		windowsDirectory = `C:\Windows`
	}
	data, err := os.ReadFile(filepath.Join(windowsDirectory, "Fonts", name))
	if err != nil {
		return nil
	}
	return fyne.NewStaticResource(name, data)
}
