package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Theme palettes mirror desktop (frontend/src/main.js THEMES); store names
// stay TUI-compatible so all three apps share one config value.
type palette struct {
	Key    string
	Store  string
	Label  string
	BG     color.NRGBA
	BGDeep color.NRGBA
	Text   color.NRGBA
	Accent color.NRGBA
}

func hex(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xFF}
}

var palettes = []palette{
	{"night", "mocha", "☾ NIGHT", hex(0x0A0A0F), hex(0x020203), hex(0xF8F8F4), hex(0xFFD9A0)},
	{"dusk", "latte", "⛅ DUSK", hex(0x575072), hex(0x35304A), hex(0xF5F2FA), hex(0xFFC6A0)},
	{"macchiato", "macchiato", "MACCHIATO", hex(0x24273A), hex(0x181926), hex(0xCAD3F5), hex(0xC6A0F6)},
	{"frappe", "frappe", "FRAPPÉ", hex(0x303446), hex(0x232634), hex(0xC6D0F5), hex(0xCA9EE6)},
	{"dracula", "dracula", "DRACULA", hex(0x282A36), hex(0x1D1E26), hex(0xF8F8F2), hex(0xBD93F9)},
	{"gruvbox", "gruvbox", "GRUVBOX", hex(0x282828), hex(0x1D2021), hex(0xEBDBB2), hex(0xD3869B)},
	{"indigo", "indigo", "INDIGO", hex(0x0F172A), hex(0x020617), hex(0xE2E8F0), hex(0x818CF8)},
	{"ember", "ember", "EMBER", hex(0x1C120C), hex(0x0A0503), hex(0xF5E9DF), hex(0xFF9E64)},
}

// Active palette tokens — package globals read at render time; switching
// themes rewrites these then rebuilds the UI.
var (
	colBG     = palettes[0].BG
	colBGDeep = palettes[0].BGDeep
	colText   = palettes[0].Text
	colAccent = palettes[0].Accent

	colDim   = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x59}
	colFaint = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x14}
	colGlass = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x08}
	colOK    = color.NRGBA{R: 0xA6, G: 0xE3, B: 0xA1, A: 0xFF}
	colErr   = color.NRGBA{R: 0xF3, G: 0x8B, B: 0xA8, A: 0xFF}

	colTransparent = color.NRGBA{}
)

func paletteFor(store string) palette {
	for _, p := range palettes {
		if p.Store == store || p.Key == store {
			return p
		}
	}
	return palettes[0]
}

func applyPalette(p palette) {
	colBG, colBGDeep, colText, colAccent = p.BG, p.BGDeep, p.Text, p.Accent
}

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

// JetBrains Mono replaces Fyne's boxy default monospace everywhere.
func (t *luminaTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace {
		return resourceJetBrainsMonoRegularTtf
	}
	return t.Theme.Font(style)
}

// Softer corners on stock widgets (entries, sliders, buttons).
func (t *luminaTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInputRadius:
		return 14
	case theme.SizeNameSelectionRadius:
		return 8
	}
	return t.Theme.Size(name)
}
