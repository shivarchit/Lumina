package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette mirrors the desktop app's tokens (frontend/src/style.css).
var (
	colBG     = color.NRGBA{R: 0x0A, G: 0x0A, B: 0x0F, A: 0xFF}
	colBGDeep = color.NRGBA{R: 0x02, G: 0x02, B: 0x03, A: 0xFF}
	colText   = color.NRGBA{R: 0xF8, G: 0xF8, B: 0xF4, A: 0xFF}
	colDim    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x59}
	colFaint  = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x14}
	colGlass  = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x08}
	colAccent = color.NRGBA{R: 0xFF, G: 0xD9, B: 0xA0, A: 0xFF}
	colOK     = color.NRGBA{R: 0xA6, G: 0xE3, B: 0xA1, A: 0xFF}
	colErr    = color.NRGBA{R: 0xF3, G: 0x8B, B: 0xA8, A: 0xFF}
)

func withAlpha(c color.NRGBA, a uint8) color.NRGBA { c.A = a; return c }

type luminaTheme struct{ fyne.Theme }

func newLuminaTheme() fyne.Theme { return &luminaTheme{theme.DefaultTheme()} }

func (t *luminaTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colBG
	case theme.ColorNameForeground:
		return colText
	case theme.ColorNamePrimary, theme.ColorNameFocus:
		return colAccent
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x0D}
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return colDim
	case theme.ColorNameButton:
		return colGlass
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return colFaint
	case theme.ColorNameScrollBar:
		return colFaint
	}
	return t.Theme.Color(name, theme.VariantDark)
}
