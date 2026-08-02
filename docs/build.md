# Building from source

Lumina Desktop is a [Wails v2](https://wails.io) app: Go backend, vanilla JS frontend.

## Prerequisites (all platforms)

- Go 1.25+
- Node 18+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

Check your setup with `wails doctor`.

## macOS

Xcode Command Line Tools (`xcode-select --install`), then:

```bash
wails build                                      # native arch → build/bin/Lumina Desktop.app
wails build -platform darwin/universal -clean    # universal (what releases ship)
```

Dev quirk: `wails build` re-signs ad-hoc each build, and macOS may re-ask (or silently
reset) Local Network permission for the fresh signature. Launching the binary from a
terminal inherits the terminal's permission and always works:

```bash
"./build/bin/Lumina Desktop.app/Contents/MacOS/Lumina Desktop"
```

## Linux

```bash
sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev   # Debian/Ubuntu
wails build -tags webkit2_41
```

The `webkit2_41` tag is required on distros that ship WebKitGTK 4.1 (Ubuntu 24.04+).
On older distros with `libwebkit2gtk-4.0-dev`, drop the tag.

## Windows

No extra system deps (WebView2 SDK is bundled). For the installer, NSIS is needed:

```powershell
choco install nsis
wails build -nsis        # → installer + portable exe in build/bin/
```

## Development loop

```bash
wails dev        # live-reload app (Vite + Go)
go test ./...    # backend tests
go vet ./...
```

## Releases

Pushing a `v*` tag triggers `.github/workflows/release.yml`: a build matrix
(macOS universal, Windows amd64, Linux amd64 + arm64) packages per-OS artifacts,
combines checksums into `SHA256SUMS.txt`, and publishes a GitHub Release with
generated notes. It can also be run manually from the Actions tab against an
existing tag.

## App icon

`build/appicon.png` is generated — don't edit by hand:

```bash
go run build/gen_icon.go
```
