#!/usr/bin/env bash
# Lumina Desktop installer for macOS.
#   curl -fsSL https://raw.githubusercontent.com/shivarchit/Lumina/master/install.sh | bash
set -euo pipefail

REPO="shivarchit/Lumina"
APP="Lumina Desktop.app"
ASSET="Lumina-Desktop-mac-universal.zip"
URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"

if [ "$(uname -s)" != "Darwin" ]; then
    echo "Lumina Desktop is macOS-only for now. On Linux, build from source: wails build" >&2
    exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading ${ASSET} ..."
curl -fSL --progress-bar "$URL" -o "$tmp/app.zip"

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
