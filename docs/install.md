# Installing Lumina Desktop

Every release ships prebuilt artifacts for macOS, Windows, and Linux, plus a combined
`SHA256SUMS.txt`. Grab them from the [latest release](https://github.com/shivarchit/Lumina/releases/latest).

## macOS

One-liner (downloads the universal build, installs to `/Applications`, launches):

```bash
curl -fsSL https://raw.githubusercontent.com/shivarchit/Lumina/master/install.sh | bash
```

Or manually: download `Lumina-Desktop-mac-universal.zip`, unzip, drag `Lumina Desktop.app`
to `/Applications`. The app is ad-hoc signed; if Gatekeeper complains, right-click → Open,
or clear quarantine:

```bash
xattr -dr com.apple.quarantine "/Applications/Lumina Desktop.app"
```

**First launch: allow Local Network access when macOS asks.** The app talks to your bulbs
directly over LAN UDP — no cloud. If you denied the prompt (or toggles show *failed*), see
[troubleshooting](troubleshooting.md).

## Windows

One-liner (PowerShell — downloads the NSIS installer, runs it silently):

```powershell
irm https://raw.githubusercontent.com/shivarchit/Lumina/master/install.ps1 | iex
```

Or manually:

- `Lumina-Desktop-windows-amd64-installer.exe` — NSIS installer (Start menu + desktop shortcut)
- `Lumina-Desktop-windows-amd64.zip` — portable `Lumina Desktop.exe`, no install

Requires the WebView2 runtime; the installer fetches it automatically if missing
(preinstalled on Windows 11).

## Linux

Same one-liner as macOS (detects Linux, installs the tarball to `/usr/local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/shivarchit/Lumina/master/install.sh | bash
```

Or manually:

```bash
tar -xzf Lumina-Desktop-linux-amd64.tar.gz   # or linux-arm64
sudo install -m 755 lumina-desktop /usr/local/bin/
lumina-desktop
```

Runtime dependencies: GTK 3 and WebKitGTK 4.1 (`libgtk-3-0` and `libwebkit2gtk-4.1-0`
on Debian/Ubuntu).

## Verify a download

```bash
shasum -a 256 -c SHA256SUMS.txt --ignore-missing
```

## Configuration

Lumina reads and writes `~/.lumina-config.json` — the same file as
[Lumina-TUI](https://github.com/shivarchit/Lumina-TUI). Devices, groups, theme, and last
light state are shared between both apps. No separate setup: if you've used the TUI, your
devices are already there; otherwise hit **Discover** in the app.
