package main

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	fyneTheme "fyne.io/fyne/v2/theme"
)

func TestWinUIThemeUsesSelectedPalette(t *testing.T) {
	current := newWinUITheme()
	for _, test := range []struct {
		variant fyne.ThemeVariant
		want    color.NRGBA
	}{
		{variant: fyneTheme.VariantLight, want: winUILightPalette.background},
		{variant: fyneTheme.VariantDark, want: winUIDarkPalette.background},
	} {
		got := color.NRGBAModel.Convert(
			current.Color(fyneTheme.ColorNameBackground, test.variant),
		).(color.NRGBA)
		if got != test.want {
			t.Fatalf("background for variant=%d = %#v, want %#v", test.variant, got, test.want)
		}
	}
}

func TestWinUIThemeMapsControlPalette(t *testing.T) {
	current := newWinUITheme()
	expected := map[fyne.ThemeColorName]color.NRGBA{
		fyneTheme.ColorNameBackground:       winUILightPalette.background,
		fyneTheme.ColorNameHeaderBackground: winUILightPalette.backgroundSecondary,
		fyneTheme.ColorNameButton:           winUILightPalette.control,
		fyneTheme.ColorNameDisabledButton:   winUILightPalette.controlDisabled,
		fyneTheme.ColorNameInputBackground:  winUILightPalette.surface,
		fyneTheme.ColorNameInputBorder:      winUILightPalette.controlBorder,
		fyneTheme.ColorNameForeground:       winUILightPalette.textPrimary,
		fyneTheme.ColorNamePlaceHolder:      winUILightPalette.textSecondary,
	}
	for name, want := range expected {
		got := color.NRGBAModel.Convert(
			current.Color(name, fyneTheme.VariantLight),
		).(color.NRGBA)
		if got != want {
			t.Fatalf("%s = %#v, want %#v", name, got, want)
		}
	}
}

func TestWinUIThemeUsesExpectedAccentAndGeometry(t *testing.T) {
	current := newWinUITheme()
	accent := color.NRGBAModel.Convert(
		current.Color(fyneTheme.ColorNamePrimary, fyneTheme.VariantDark),
	).(color.NRGBA)
	if want := winUIDarkPalette.accent; accent != want {
		t.Fatalf("accent = %#v, want %#v", accent, want)
	}
	if got := current.Size(fyneTheme.SizeNameInputRadius); got != 4 {
		t.Fatalf("input radius = %v, want 4", got)
	}
	if got := current.Size(fyneTheme.SizeNameInputBorder); got != 1 {
		t.Fatalf("input border = %v, want 1", got)
	}
	if got := current.Size(fyneTheme.SizeNameInnerPadding); got != 6 {
		t.Fatalf("inner padding = %v, want 6", got)
	}
	if got := current.Size(fyneTheme.SizeNameInlineIcon); got != 16 {
		t.Fatalf("inline icon size = %v, want 16", got)
	}
}

func TestWinUIThemeInteractionLayersStayTranslucent(t *testing.T) {
	current := newWinUITheme()
	layers := make(map[fyne.ThemeColorName]color.NRGBA)
	for _, name := range []fyne.ThemeColorName{
		fyneTheme.ColorNameHover,
		fyneTheme.ColorNamePressed,
		fyneTheme.ColorNameSelection,
	} {
		layer := color.NRGBAModel.Convert(
			current.Color(name, fyneTheme.VariantLight),
		).(color.NRGBA)
		if layer.A == 0 || layer.A == 0xff {
			t.Fatalf("%s alpha = %#x, want a translucent interaction layer", name, layer.A)
		}
		layers[name] = layer
	}
	if layers[fyneTheme.ColorNamePressed].A <= layers[fyneTheme.ColorNameHover].A {
		t.Fatal("pressed feedback must be stronger than hover feedback")
	}
}

func TestWinUIThemeControlsRemainVisibleOnWindowBackground(t *testing.T) {
	current := newWinUITheme()
	background := color.NRGBAModel.Convert(
		current.Color(fyneTheme.ColorNameBackground, fyneTheme.VariantLight),
	).(color.NRGBA)
	for _, name := range []fyne.ThemeColorName{
		fyneTheme.ColorNameButton,
		fyneTheme.ColorNameInputBackground,
	} {
		control := color.NRGBAModel.Convert(
			current.Color(name, fyneTheme.VariantLight),
		).(color.NRGBA)
		if control == background {
			t.Fatalf("%s must remain visible on the window background", name)
		}
	}
}

func TestWinUIDarkThemeUsesReadableStatusForegrounds(t *testing.T) {
	current := newWinUITheme()
	want := winUILightPalette.textPrimary
	for _, name := range []fyne.ThemeColorName{
		fyneTheme.ColorNameForegroundOnError,
		fyneTheme.ColorNameForegroundOnSuccess,
		fyneTheme.ColorNameForegroundOnWarning,
	} {
		got := color.NRGBAModel.Convert(
			current.Color(name, fyneTheme.VariantDark),
		).(color.NRGBA)
		if got != want {
			t.Fatalf("%s = %#v, want %#v", name, got, want)
		}
	}
}

func TestWinUISelectThemeUsesOpaqueInteractionSurfaces(t *testing.T) {
	base := newWinUITheme()
	selectTheme := &winUISelectTheme{base: base}
	for _, test := range []struct {
		variant fyne.ThemeVariant
		palette winUIColorPalette
	}{
		{variant: fyneTheme.VariantLight, palette: winUILightPalette},
		{variant: fyneTheme.VariantDark, palette: winUIDarkPalette},
	} {
		for name, want := range map[fyne.ThemeColorName]color.NRGBA{
			fyneTheme.ColorNameInputBackground: test.palette.surface,
			fyneTheme.ColorNameHover:           test.palette.controlHover,
		} {
			got := color.NRGBAModel.Convert(
				selectTheme.Color(name, test.variant),
			).(color.NRGBA)
			if got != want {
				t.Fatalf("%s for variant=%d = %#v, want %#v", name, test.variant, got, want)
			}
		}
	}
}

func TestWinUIThemeAlwaysProvidesFontsAndIcons(t *testing.T) {
	current := newWinUITheme()
	for _, style := range []fyne.TextStyle{
		{},
		{Bold: true},
		{Italic: true},
		{Bold: true, Italic: true},
		{Monospace: true},
		{Symbol: true},
	} {
		if current.Font(style) == nil {
			t.Fatalf("font for style %+v is nil", style)
		}
	}
	if current.Icon(fyneTheme.IconNameConfirm) == nil {
		t.Fatal("confirm icon is nil")
	}
}
