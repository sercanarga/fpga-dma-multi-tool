package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	fyneTheme "fyne.io/fyne/v2/theme"
)

type winUIColorPalette struct {
	background          color.NRGBA
	backgroundSecondary color.NRGBA
	surface             color.NRGBA
	control             color.NRGBA
	controlHover        color.NRGBA
	controlDisabled     color.NRGBA
	controlBorder       color.NRGBA
	textPrimary         color.NRGBA
	textSecondary       color.NRGBA
	textDisabled        color.NRGBA
	textOnAccent        color.NRGBA
	accent              color.NRGBA
	focusOverlay        color.NRGBA
	hoverOverlay        color.NRGBA
	pressedOverlay      color.NRGBA
	selectionOverlay    color.NRGBA
	selectFocus         color.NRGBA
	scrollBar           color.NRGBA
	shadow              color.NRGBA
	error               color.NRGBA
	success             color.NRGBA
	warning             color.NRGBA
}

var winUILightPalette = winUIColorPalette{
	background:          color.NRGBA{R: 0xf3, G: 0xf3, B: 0xf3, A: 0xff},
	backgroundSecondary: color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff},
	surface:             color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	control:             color.NRGBA{R: 0xfb, G: 0xfb, B: 0xfb, A: 0xff},
	controlHover:        color.NRGBA{R: 0xf6, G: 0xf6, B: 0xf6, A: 0xff},
	controlDisabled:     color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff},
	controlBorder:       color.NRGBA{R: 0xd9, G: 0xd9, B: 0xd9, A: 0xff},
	textPrimary:         color.NRGBA{R: 0x1b, G: 0x1b, B: 0x1b, A: 0xff},
	textSecondary:       color.NRGBA{R: 0x5e, G: 0x5e, B: 0x5e, A: 0xff},
	textDisabled:        color.NRGBA{R: 0x9a, G: 0x9a, B: 0x9a, A: 0xff},
	textOnAccent:        color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	accent:              color.NRGBA{R: 0x00, G: 0x67, B: 0xc0, A: 0xff},
	focusOverlay:        color.NRGBA{R: 0x00, G: 0x67, B: 0xc0, A: 0x26},
	hoverOverlay:        color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x05},
	pressedOverlay:      color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x0a},
	selectionOverlay:    color.NRGBA{R: 0x00, G: 0x67, B: 0xc0, A: 0x26},
	selectFocus:         color.NRGBA{R: 0xe5, G: 0xf3, B: 0xff, A: 0xff},
	scrollBar:           color.NRGBA{R: 0x8a, G: 0x8a, B: 0x8a, A: 0xb0},
	shadow:              color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x26},
	error:               color.NRGBA{R: 0xc4, G: 0x2b, B: 0x1c, A: 0xff},
	success:             color.NRGBA{R: 0x0f, G: 0x7b, B: 0x0f, A: 0xff},
	warning:             color.NRGBA{R: 0x9d, G: 0x5d, B: 0x00, A: 0xff},
}

var winUIDarkPalette = winUIColorPalette{
	background:          color.NRGBA{R: 0x20, G: 0x20, B: 0x20, A: 0xff},
	backgroundSecondary: color.NRGBA{R: 0x1c, G: 0x1c, B: 0x1c, A: 0xff},
	surface:             color.NRGBA{R: 0x2b, G: 0x2b, B: 0x2b, A: 0xff},
	control:             color.NRGBA{R: 0x32, G: 0x32, B: 0x32, A: 0xff},
	controlHover:        color.NRGBA{R: 0x3a, G: 0x3a, B: 0x3a, A: 0xff},
	controlDisabled:     color.NRGBA{R: 0x29, G: 0x29, B: 0x29, A: 0xff},
	controlBorder:       color.NRGBA{R: 0x45, G: 0x45, B: 0x45, A: 0xff},
	textPrimary:         color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff},
	textSecondary:       color.NRGBA{R: 0xc7, G: 0xc7, B: 0xc7, A: 0xff},
	textDisabled:        color.NRGBA{R: 0x7a, G: 0x7a, B: 0x7a, A: 0xff},
	textOnAccent:        color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	accent:              color.NRGBA{R: 0x00, G: 0x78, B: 0xd4, A: 0xff},
	focusOverlay:        color.NRGBA{R: 0x4c, G: 0xc2, B: 0xff, A: 0x55},
	hoverOverlay:        color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0a},
	pressedOverlay:      color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x18},
	selectionOverlay:    color.NRGBA{R: 0x4c, G: 0xc2, B: 0xff, A: 0x40},
	selectFocus:         color.NRGBA{R: 0x17, G: 0x4c, B: 0x66, A: 0xff},
	scrollBar:           color.NRGBA{R: 0xa0, G: 0xa0, B: 0xa0, A: 0xb0},
	shadow:              color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x66},
	error:               color.NRGBA{R: 0xff, G: 0x99, B: 0xa4, A: 0xff},
	success:             color.NRGBA{R: 0x6c, G: 0xcb, B: 0x5f, A: 0xff},
	warning:             color.NRGBA{R: 0xfc, G: 0xe1, B: 0x00, A: 0xff},
}

type winUIColorToken uint8

const (
	winUIColorBackgroundSecondary winUIColorToken = iota
	winUIColorSurface
	winUIColorControlBorder
	winUIColorTextPrimary
	winUIColorTextSecondary
	winUIColorTextDisabled
	winUIColorAccent
	winUIColorError
	winUIColorSuccess
	winUIColorWarning
)

type winUIAdaptiveColor struct {
	theme *winUITheme
	token winUIColorToken
}

func (adaptive winUIAdaptiveColor) RGBA() (r, g, b, a uint32) {
	variant := fyneTheme.VariantLight
	if application := fyne.CurrentApp(); application != nil {
		variant = application.Settings().ThemeVariant()
	}
	return adaptive.theme.palette(variant).tokenColor(adaptive.token).RGBA()
}

func (palette winUIColorPalette) tokenColor(token winUIColorToken) color.NRGBA {
	switch token {
	case winUIColorBackgroundSecondary:
		return palette.backgroundSecondary
	case winUIColorSurface:
		return palette.surface
	case winUIColorControlBorder:
		return palette.controlBorder
	case winUIColorTextPrimary:
		return palette.textPrimary
	case winUIColorTextSecondary:
		return palette.textSecondary
	case winUIColorTextDisabled:
		return palette.textDisabled
	case winUIColorAccent:
		return palette.accent
	case winUIColorError:
		return palette.error
	case winUIColorSuccess:
		return palette.success
	case winUIColorWarning:
		return palette.warning
	default:
		return palette.textPrimary
	}
}

type winUITheme struct {
	base fyne.Theme

	regular    fyne.Resource
	bold       fyne.Resource
	italic     fyne.Resource
	boldItalic fyne.Resource
	monospace  fyne.Resource
}

func newWinUITheme() *winUITheme {
	return &winUITheme{
		base:       fyneTheme.DefaultTheme(),
		regular:    loadWinUIFont("segoeui.ttf"),
		bold:       loadWinUIFont("seguisb.ttf"),
		italic:     loadWinUIFont("segoeuii.ttf"),
		boldItalic: loadWinUIFont("segoeuiz.ttf"),
		monospace:  loadWinUIFont("consola.ttf"),
	}
}

func (theme *winUITheme) palette(variant fyne.ThemeVariant) winUIColorPalette {
	if variant == fyneTheme.VariantDark {
		return winUIDarkPalette
	}
	return winUILightPalette
}

func (theme *winUITheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	palette := theme.palette(variant)
	switch name {
	case fyneTheme.ColorNameBackground:
		return palette.background
	case fyneTheme.ColorNameButton:
		return palette.control
	case fyneTheme.ColorNameDisabledButton:
		return palette.controlDisabled
	case fyneTheme.ColorNameDisabled:
		return palette.textDisabled
	case fyneTheme.ColorNameError:
		return palette.error
	case fyneTheme.ColorNameFocus:
		return palette.focusOverlay
	case fyneTheme.ColorNameForeground:
		return palette.textPrimary
	case fyneTheme.ColorNameForegroundOnPrimary:
		return palette.textOnAccent
	case fyneTheme.ColorNameForegroundOnError,
		fyneTheme.ColorNameForegroundOnSuccess:
		if variant == fyneTheme.VariantDark {
			return winUILightPalette.textPrimary
		}
		return palette.textOnAccent
	case fyneTheme.ColorNameForegroundOnWarning:
		if variant == fyneTheme.VariantDark {
			return winUILightPalette.textPrimary
		}
		return palette.textPrimary
	case fyneTheme.ColorNameHeaderBackground:
		return palette.backgroundSecondary
	case fyneTheme.ColorNameHover:
		return palette.hoverOverlay
	case fyneTheme.ColorNameHyperlink, fyneTheme.ColorNamePrimary:
		return palette.accent
	case fyneTheme.ColorNameInputBackground:
		return palette.surface
	case fyneTheme.ColorNameMenuBackground:
		return palette.surface
	case fyneTheme.ColorNameOverlayBackground:
		return palette.surface
	case fyneTheme.ColorNameInputBorder:
		return palette.controlBorder
	case fyneTheme.ColorNamePlaceHolder:
		return palette.textSecondary
	case fyneTheme.ColorNamePressed:
		return palette.pressedOverlay
	case fyneTheme.ColorNameScrollBar:
		return palette.scrollBar
	case fyneTheme.ColorNameScrollBarBackground:
		return color.Transparent
	case fyneTheme.ColorNameSelection:
		return palette.selectionOverlay
	case fyneTheme.ColorNameSeparator:
		return palette.controlBorder
	case fyneTheme.ColorNameShadow:
		return palette.shadow
	case fyneTheme.ColorNameSuccess:
		return palette.success
	case fyneTheme.ColorNameWarning:
		return palette.warning
	default:
		return theme.base.Color(name, variant)
	}
}

func currentWinUIThemeColor(token winUIColorToken) color.Color {
	application := fyne.CurrentApp()
	if application != nil {
		if current, ok := application.Settings().Theme().(*winUITheme); ok {
			return winUIAdaptiveColor{theme: current, token: token}
		}
	}
	return winUILightPalette.tokenColor(token)
}

type winUISelectTheme struct {
	base fyne.Theme
}

func (theme *winUISelectTheme) Color(
	name fyne.ThemeColorName,
	variant fyne.ThemeVariant,
) color.Color {
	palette := winUILightPalette
	if current, ok := theme.base.(*winUITheme); ok {
		palette = current.palette(variant)
	} else if variant == fyneTheme.VariantDark {
		palette = winUIDarkPalette
	}
	switch name {
	case fyneTheme.ColorNameInputBackground:
		return palette.surface
	case fyneTheme.ColorNameHover:
		return palette.controlHover
	case fyneTheme.ColorNameFocus:
		return palette.selectFocus
	default:
		return theme.base.Color(name, variant)
	}
}

func (theme *winUISelectTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.base.Font(style)
}

func (theme *winUISelectTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.base.Icon(name)
}

func (theme *winUISelectTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.base.Size(name)
}

func (theme *winUITheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Symbol {
		return theme.base.Font(style)
	}
	if style.Monospace {
		return firstResource(theme.monospace, theme.base.Font(style))
	}
	if style.Bold && style.Italic {
		return firstResource(theme.boldItalic, theme.base.Font(style))
	}
	if style.Bold {
		return firstResource(theme.bold, theme.base.Font(style))
	}
	if style.Italic {
		return firstResource(theme.italic, theme.base.Font(style))
	}
	return firstResource(theme.regular, theme.base.Font(style))
}

func (theme *winUITheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.base.Icon(name)
}

func (theme *winUITheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case fyneTheme.SizeNameCaptionText:
		return 12
	case fyneTheme.SizeNameText:
		return 14
	case fyneTheme.SizeNameHeadingText:
		return 28
	case fyneTheme.SizeNameSubHeadingText:
		return 20
	case fyneTheme.SizeNamePadding:
		return 4
	case fyneTheme.SizeNameInnerPadding:
		return 6
	case fyneTheme.SizeNameInlineIcon:
		return 16
	case fyneTheme.SizeNameLineSpacing:
		return 4
	case fyneTheme.SizeNameInputBorder,
		fyneTheme.SizeNameSeparatorThickness:
		return 1
	case fyneTheme.SizeNameInputRadius,
		fyneTheme.SizeNameSelectionRadius:
		return 4
	case fyneTheme.SizeNameScrollBar:
		return 10
	case fyneTheme.SizeNameScrollBarSmall:
		return 4
	case fyneTheme.SizeNameScrollBarRadius:
		return 5
	default:
		return theme.base.Size(name)
	}
}

func firstResource(preferred, fallback fyne.Resource) fyne.Resource {
	if preferred != nil {
		return preferred
	}
	return fallback
}

var (
	_ fyne.Theme = (*winUITheme)(nil)
	_ fyne.Theme = (*winUISelectTheme)(nil)
)
