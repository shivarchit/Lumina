#!/usr/bin/env bash
# Lumina Desktop installer for macOS and Linux.
#   curl -fsSL https://raw.githubusercontent.com/shivarchit/Lumina/master/install.sh | bash
set -euo pipefail

REPO="shivarchit/Lumina"
BASE="https://github.com/${REPO}/releases/latest/download"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

case "$(uname -s)" in
Darwin)
    APP="Lumina Desktop.app"
    ASSET="Lumina-Desktop-mac-universal.zip"

    echo "Downloading ${ASSET} ..."
    curl -fSL --progress-bar "${BASE}/${ASSET}" -o "$tmp/app.zip"

    echo "Installing to /Applications ..."
    ditto -x -k "$tmp/app.zip" "$tmp/extract"
    rm -rf "/Applications/${APP}"
    mv "$tmp/extract/${APP}" /Applications/

    # Ad-hoc signed app downloaded from the internet: clear quarantine so
    # Gatekeeper doesn't refuse it outright.
    xattr -dr com.apple.quarantine "/Applications/${APP}" 2>/dev/null || true

    echo "Installed /Applications/${APP}"
    echo "First launch: allow Local Network access when macOS asks — the app talks to your bulbs over LAN UDP."
    open "/Applications/${APP}"
    ;;
Linux)
    case "$(uname -m)" in
    x86_64) arch="amd64" ;;
    aarch64 | arm64) arch="arm64" ;;
    *)
        echo "Unsupported architecture: $(uname -m). Build from source: wails build -tags webkit2_41" >&2
        exit 1
        ;;
    esac
    ASSET="Lumina-Desktop-linux-${arch}.tar.gz"

    echo "Downloading ${ASSET} ..."
    curl -fSL --progress-bar "${BASE}/${ASSET}" -o "$tmp/app.tar.gz"
    tar -xzf "$tmp/app.tar.gz" -C "$tmp"

    dest="/usr/local/bin"
    echo "Installing to ${dest}/lumina-desktop ..."
    if [ -w "$dest" ]; then
        install -m 755 "$tmp/lumina-desktop" "$dest/"
    else
        sudo install -m 755 "$tmp/lumina-desktop" "$dest/"
    fi

    echo "Installed ${dest}/lumina-desktop"
    echo "Runtime deps: GTK 3 + WebKitGTK 4.1 (Debian/Ubuntu: libgtk-3-0 libwebkit2gtk-4.1-0)."
    ;;
*)
    echo "Unsupported OS. On Windows, run in PowerShell:" >&2
    echo "  irm https://raw.githubusercontent.com/${REPO}/master/install.ps1 | iex" >&2
    exit 1
    ;;
esac
