//go:build !windows

package main

import "fyne.io/fyne/v2"

func loadWinUIFont(string) fyne.Resource {
	return nil
}
