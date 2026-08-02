# Lumina

A desktop app for WiZ smart lights, built on the [Lumina-TUI](https://github.com/shivarchit/Lumina-TUI) engine.

Cinematic Ambient design: near-black canvas, one oversized brightness dial, frosted glass — and light blobs that mirror what your bulbs are actually doing. Click the ring, the room changes. The lamp is the interface.

![Lumina Desktop — Night mode](docs/screenshots/night.png)

## Install (macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/shivarchit/Lumina/master/install.sh | bash
```

Downloads the latest release, installs to `/Applications`, and launches. On first run, **allow Local Network access** when macOS asks — the app talks to your bulbs directly over LAN UDP (no cloud).

## Features

- **One dial** — click anywhere on the arc or drag; an off bulb wakes at the clicked brightness
- **Targets** — pill switcher for saved devices and groups; group commands fan out concurrently with one aggregated result
- **Color / Temp / Scenes** — hue wheel with recent hexes, warm-to-cool kelvin rail, twelve WiZ scene presets with color-preview chips; each takes the dial's place (progressive disclosure), `← Back`/Esc returns
- **Sleep timer** — 15/30/60 presets or custom minutes, live countdown, cancel
- **Discover** — UDP broadcast scan; devices stream in as cards, save/rename inline
- **Groups** — create, delete, and tick members in a management overlay; targeting a group fans every command out
- **Night / Dusk** — ☾ near-black or ⛅ muted twilight; one toggle, same soul

  ![Lumina Desktop — Dusk mode](docs/screenshots/dusk.png)

- **Live** — 10-second heartbeat re-syncs state, so changes from the phone app, the TUI, or cron show up here
- **Shared brain** — reads and writes the same `~/.lumina-config.json` as Lumina-TUI: devices, groups, theme, last state

## Architecture

The WiZ protocol, discovery, and config layers live in [`Lumina-TUI/pkg`](https://github.com/shivarchit/Lumina-TUI/tree/master/pkg) and are imported as a Go dependency — one engine, two faces (TUI + desktop). Engine fixes land there and reach this app with a version bump.

- `app.go` — Wails bindings: state fetch, fan-out control, discovery, device/group CRUD
- `frontend/` — vanilla JS + CSS, no framework; `src/main.js` and `src/style.css` are the whole UI
- `build/gen_icon.go` — stdlib generator for the Three Lights app icon

## Development

Requires Go 1.25+, Node 18+, and the [Wails CLI](https://wails.io/docs/gettingstarted/installation):

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails dev      # live-reload development
wails build    # production build → build/bin/Lumina Desktop.app
go test ./...  # backend tests
```

Known dev quirk: `wails build` re-signs ad-hoc each build, and macOS may re-ask (or silently reset) Local Network permission for the fresh signature. Launching the binary from a terminal inherits the terminal's permission and always works.

## Related

- [Lumina-TUI](https://github.com/shivarchit/Lumina-TUI) — the terminal app and the engine this one is built on. Its CLI (`lumina on`, `lumina scene party`, …) pairs with cron for scheduling.

## License

MIT
