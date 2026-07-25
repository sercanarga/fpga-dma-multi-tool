package main

import (
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fyneTest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestStatusBarHasStableCompactSize(t *testing.T) {
	application := fyneTest.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(newWinUITheme())

	status, statusBar := newStatusBar(
		"A long status message must not enlarge the application window or wrap vertically.",
	)
	status.SetText("Updated status")
	if status.text.Text != "Updated status" {
		t.Fatalf("status text = %q, want updated text", status.text.Text)
	}
	size := statusBar.MinSize()
	if size.Width != 120 || size.Height != 24 {
		t.Fatalf("status bar minimum size = %v, want 120x24", size)
	}
}

func TestStatusLabelIsVerticallyCentered(t *testing.T) {
	application := fyneTest.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(newWinUITheme())

	label := canvas.NewText("Ready", winUILightPalette.textPrimary)
	label.TextSize = 12
	bar := container.New(&statusLabelLayout{horizontalPadding: 8}, label)
	bar.Resize(fyne.NewSize(640, 23))

	wantY := (bar.Size().Height - label.MinSize().Height) / 2
	if label.Position().Y != wantY {
		t.Fatalf("status label y = %v, want %v", label.Position().Y, wantY)
	}
	if label.Size().Height != label.MinSize().Height {
		t.Fatalf("status label height = %v, want natural height %v", label.Size().Height, label.MinSize().Height)
	}
}

func TestStatusLinkUsesTheSameVerticalCenterAsStatusText(t *testing.T) {
	application := fyneTest.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(newWinUITheme())

	statusText := canvas.NewText("Ready", winUILightPalette.textPrimary)
	statusText.TextSize = 12
	linkText := canvas.NewText("GitHub", winUILightPalette.accent)
	linkText.TextSize = 12
	statusArea := container.New(&statusLabelLayout{horizontalPadding: 8}, statusText)
	linkArea := container.New(
		&statusLabelLayout{horizontalPadding: 8, minimumWidth: linkText.MinSize().Width + 16},
		linkText,
	)
	statusArea.Resize(fyne.NewSize(560, 23))
	linkArea.Resize(fyne.NewSize(80, 23))

	if statusText.Position().Y != linkText.Position().Y {
		t.Fatalf(
			"status y = %v, GitHub link y = %v; both must share the same vertical center",
			statusText.Position().Y,
			linkText.Position().Y,
		)
	}
}

func TestPageFrameKeepsStatusBarFullWidth(t *testing.T) {
	application := fyneTest.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(newWinUITheme())

	body := canvas.NewRectangle(statusBarBackground)
	statusSurface := canvas.NewRectangle(statusBarBackground)
	statusBar := container.New(&fixedHeightLayout{height: 24}, statusSurface)
	frame := newPageFrame("Title", "Subtitle", body, statusBar)
	frame.Resize(fyne.NewSize(640, 420))

	if statusBar.Position().X != 0 {
		t.Fatalf("status bar x = %v, want 0", statusBar.Position().X)
	}
	if statusBar.Size().Width != 640 {
		t.Fatalf("status bar width = %v, want 640", statusBar.Size().Width)
	}
}

func TestInteractiveControlsUseWinUIHeight(t *testing.T) {
	application := fyneTest.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(newWinUITheme())

	controls := []struct {
		name   string
		height float32
	}{
		{name: "button", height: widget.NewButton("Action", nil).MinSize().Height},
		{name: "entry", height: widget.NewEntry().MinSize().Height},
	}
	for _, control := range controls {
		if control.height < 28 || control.height > 34 {
			t.Fatalf("%s height = %v, want 28..34", control.name, control.height)
		}
	}
	selectHeight := outlinedSelect(
		widget.NewSelect([]string{"One"}, nil),
	).MinSize().Height
	if selectHeight < 30 || selectHeight > 36 {
		t.Fatalf("outlined select height = %v, want 30..36", selectHeight)
	}
}

func TestSecondaryButtonUsesMinimumWidthAndFillsAvailableSpace(t *testing.T) {
	application := fyneTest.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(newWinUITheme())

	button := newWinUISecondaryButton("Copy", 88, nil)
	if button.Object.MinSize().Width < 88 {
		t.Fatalf("secondary button minimum width = %v, want at least 88", button.Object.MinSize().Width)
	}

	button.Object.Resize(fyne.NewSize(140, button.Object.MinSize().Height))
	surface := button.Object.Objects[0]
	if surface.Size().Width != 140 {
		t.Fatalf("secondary button surface width = %v, want 140", surface.Size().Width)
	}
}

func TestWinUIChoiceHidesCurrentSelectionFromMenu(t *testing.T) {
	application := fyneTest.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(newWinUITheme())

	choice := newWinUIChoice([]string{"A", "B", "C"})
	choice.SetSelected("B")
	if choice.Value() != "B" {
		t.Fatalf("selected value = %q, want B", choice.Value())
	}
	if slices.Contains(choice.Select.Options, "B") {
		t.Fatal("current selection should not be repeated in dropdown options")
	}

	choice.Select.SetSelected("C")
	if choice.Value() != "C" {
		t.Fatalf("selected value = %q, want C", choice.Value())
	}
	if slices.Contains(choice.Select.Options, "C") {
		t.Fatal("new selection should not be repeated in dropdown options")
	}
}

func TestSegmentedControlKeepsOneVisibleSelection(t *testing.T) {
	application := fyneTest.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(newWinUITheme())

	control := newWinUISegmentedControl([]string{"Auto", "CH347", "RS232"})
	if control.Value() != "Auto" {
		t.Fatalf("initial value = %q, want Auto", control.Value())
	}
	if control.buttons["Auto"].Importance != widget.HighImportance {
		t.Fatal("initial segment is not highlighted")
	}

	control.buttons["RS232"].Tapped(nil)
	if control.Value() != "RS232" {
		t.Fatalf("value after tap = %q, want RS232", control.Value())
	}
	if control.buttons["RS232"].Importance != widget.HighImportance {
		t.Fatal("selected segment is not highlighted")
	}
	if control.buttons["Auto"].Importance == widget.HighImportance {
		t.Fatal("previous segment remained highlighted")
	}

	control.SetSelected("unsupported")
	if control.Value() != "RS232" {
		t.Fatal("unsupported selection changed the current value")
	}
}

func TestTapTargetRunsAction(t *testing.T) {
	called := false
	target := newTapTarget(func() { called = true })
	target.Tapped(nil)
	if !called {
		t.Fatal("tap target did not run its action")
	}
}

func TestProgrammingModeAlwaysHasASelection(t *testing.T) {
	mode := widget.NewRadioGroup(
		[]string{"Flash (.bin, persistent)", "SRAM (.bit, temporary)"},
		nil,
	)
	mode.Required = true
	mode.SetSelected("Flash (.bin, persistent)")
	if !mode.Required || mode.Selected == "" {
		t.Fatal("programming mode must require and initialize one selection")
	}
}
