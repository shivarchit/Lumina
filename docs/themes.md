# Themes

![Themes overlay](screenshots/themes.png)

Eight moods behind the theme pill (bottom right). The chrome accent, text, and
background gradient all follow the theme; the dial always shows your light's
real color.

| Theme | Stored as | Notes |
|-------|-----------|-------|
| ☾ Night | `mocha` | Near-black default |
| ⛅ Dusk | `latte` | Muted twilight plum, the one light-ish mood |
| Macchiato | `macchiato` | Catppuccin Macchiato |
| Frappé | `frappe` | Catppuccin Frappé |
| Dracula | `dracula` | Classic Dracula |
| Gruvbox | `gruvbox` | Warm retro |
| Indigo | `indigo` | Deep blue slate |
| Ember | `ember` | Burnt orange dark |

| ☾ Night | ⛅ Dusk |
|---------|--------|
| ![Night](screenshots/night.png) | ![Dusk](screenshots/dusk.png) |

The choice persists in `~/.lumina-config.json` under names the TUI
understands, so both apps keep one look: `night` is stored as `mocha`, `dusk`
as `latte`, the Catppuccin/Dracula/Gruvbox names as-is. Indigo and Ember are
desktop-only — the TUI falls back to its default theme for those.
