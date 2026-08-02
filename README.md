# Lumina

A desktop app for WiZ smart lights, built on the [Lumina-TUI](https://github.com/shivarchit/Lumina-TUI) engine.

Cinematic Ambient design: near-black canvas, one oversized brightness dial, frosted glass panels — and light blobs that mirror what your bulbs are actually doing. Power off fades the window to black. The lamp is the interface.

## Features

- **One dial** — drag the arc to set brightness; 140ms-debounced UDP straight to the bulb
- **Targets** — pill switcher for saved devices and groups; group commands fan out concurrently with one aggregated result
- **Color / Temp / Scenes** — hue wheel with recent hexes, warm-to-cool kelvin slider, twelve WiZ scene presets; each expands from its pill (progressive disclosure)
- **Discover** — UDP broadcast scan, save bulbs inline
- **Themes** — the six shared palettes (Mocha, Macchiato, Frappé, Latte, Dracula, Gruvbox) recolor the accents
- **Live** — 10-second heartbeat re-syncs state, so changes from the phone app, the TUI, or cron show up here
- **Shared brain** — reads and writes the same `~/.lumina-config.json` as Lumina-TUI: devices, groups, theme, last state. Set a theme here, the TUI boots matching.

## Architecture

The WiZ protocol, discovery, and config layers live in [`Lumina-TUI/pkg`](https://github.com/shivarchit/Lumina-TUI/tree/master/pkg) and are imported as a Go dependency — one engine, two faces (TUI + desktop). Engine fixes land there and reach this app with a version bump.

- `app.go` — Wails bindings: state fetch, fan-out control, discovery, device/group CRUD
- `frontend/` — vanilla JS + CSS, no framework; `src/main.js` and `src/style.css` are the whole UI

## Development

Requires Go 1.25+, Node 18+, and the [Wails CLI](https://wails.io/docs/gettingstarted/installation):

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

```bash
wails dev      # live-reload development
wails build    # production build → "build/bin/Lumina Desktop.app"
go test ./...  # backend tests
```

## Related

- [Lumina-TUI](https://github.com/shivarchit/Lumina-TUI) — the terminal app and the engine this one is built on. Its CLI (`lumina on`, `lumina scene party`, …) pairs with cron for scheduling.

## License

MIT
