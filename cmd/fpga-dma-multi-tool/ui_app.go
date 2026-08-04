package main

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

const applicationID = "com.sercanarga.fpgadmamultitool"
const repositoryURL = "https://github.com/sercanarga/fpga-dma-multi-tool"

const (
	tableHeaderHeight float32 = 34
	tableRowHeight    float32 = 34
)

type guiState struct {
	window fyne.Window

	resultMu    sync.RWMutex
	result      scanResult
	programPart *winUIChoice
}

func launchGUI() {
	application := app.NewWithID(applicationID)
	application.Settings().SetTheme(newWinUITheme())
	window := application.NewWindow("FPGA DMA Multi Tool")
	state := &guiState{window: window}

	tabs := container.NewAppTabs(
		container.NewTabItem("Devices", state.buildDeviceTab()),
		container.NewTabItem("Flash", state.buildProgrammingTab()),
		container.NewTabItem("Speed Test", state.buildSpeedTestTab()),
		container.NewTabItem("Device History", state.buildDeviceHistoryTab()),
		container.NewTabItem("System Info", state.buildSystemInfoTab()),
		container.NewTabItem("Drivers", state.buildSetupTab()),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	if setupRequired(inspectSystemComponents("", "")) {
		tabs.SelectIndex(5)
	}
	window.SetContent(tabs)
	window.Resize(fyne.NewSize(860, 590))
	window.CenterOnScreen()
	window.ShowAndRun()
}
