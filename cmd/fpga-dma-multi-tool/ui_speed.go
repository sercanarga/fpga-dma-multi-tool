package main

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (state *guiState) buildSpeedTestTab() fyne.CanvasObject {
	duration := newWinUIChoice([]string{"3 seconds", "5 seconds", "10 seconds"})
	duration.SetSelected("5 seconds")
	status, statusBar := newStatusBar("Ready. Close memory-heavy applications for consistent results.")
	logEntry := widget.NewMultiLineEntry()
	logEntry.SetPlaceHolder("Test progress will appear here.")
	logEntry.Wrapping = fyne.TextWrapWord
	logEntry.Disable()

	var readButton, readWriteButton *widget.Button
	runTest := func(mode string) {
		seconds := 5
		_, _ = fmt.Sscanf(duration.Value(), "%d", &seconds)
		request := speedTestRequest{
			Mode: mode, Duration: seconds, Sizes: []int{4096, 8192, 16384, 32768},
		}
		start := func() {
			readButton.Disable()
			readWriteButton.Disable()
			duration.Select.Disable()
			logEntry.SetText("")
			status.SetText("Running the memory transfer test…")

			writer := newSynchronizedWriter(logEntry)
			go func() {
				timeout := time.Duration((seconds*len(request.Sizes)*2)+60) * time.Second
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				report, err := runSpeedTest(ctx, request, writer)
				fyne.Do(func() {
					readButton.Enable()
					readWriteButton.Enable()
					duration.Select.Enable()
					if err != nil {
						logEntry.SetText(err.Error())
						status.SetText("Speed test failed: " + err.Error())
						return
					}
					logEntry.SetText(formatSpeedTestReport(report))
					status.SetText(fmt.Sprintf("Completed %d measurements.", len(report.Passes)))
				})
			}()
		}
		if mode == "both" {
			showWinUIConfirm(
				state.window,
				"Confirm memory read/write test",
				"The test temporarily changes a small writable region in explorer.exe and restores the original bytes when it finishes. Continue?",
				"Run test",
				false,
				start,
			)
			return
		}
		start()
	}
	readButton = widget.NewButton("Memory Read", func() { runTest("read") })
	readWriteButton = widget.NewButton("Memory Read + Write", func() { runTest("both") })
	readButton.Importance = widget.HighImportance
	readWriteControl := wrapWinUISecondaryButton(readWriteButton, 152)

	options := container.NewHBox(
		widget.NewLabel("Duration per block"),
		outlinedSelect(duration.Select),
	)
	actions := container.NewHBox(readButton, readWriteControl.Object)
	toolbar := container.NewBorder(nil, nil, options, actions, nil)
	top := container.NewVBox(
		toolbar,
		newSectionTitle("Output"),
	)
	body := container.NewBorder(top, nil, nil, nil, logEntry)
	return newPageFrame(
		"DMA Speed Test",
		"Measure transfer speed, operation rate, and average latency.",
		body,
		statusBar,
	)
}
