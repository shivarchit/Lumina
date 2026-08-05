# Troubleshooting

## Toggles show "failed" (macOS)

Symptom: every power/brightness change reports `failed: <device>` in the status line,
often right after a fresh install.

Cause: macOS is blocking the app's **Local Network** access. When the permission is
denied, the OS rejects UDP sends to LAN devices (`no route to host`) — the bulbs never
see the command. The app detects this and appends a hint to the status line.

Fix:

1. Open **System Settings → Privacy & Security → Local Network**.
2. Enable **Lumina Desktop** (add it with **+** if it's not listed).
3. Quit and relaunch the app.

Still failing? Known macOS quirks:

- The permission prompt only appears once. If it was dismissed or denied, macOS never
  asks again — use the settings pane above.
- On some macOS versions the toggle doesn't take effect until the app is relaunched,
  and occasionally not until the Mac is rebooted (an Apple TCC bug).
- Each app update is re-signed, which can silently reset the permission — re-check the
  settings pane after updating.
- The app itself retries: any blocked command re-fires a broadcast (which re-raises
  the prompt when macOS is willing) and opens the Local Network settings pane for you.

## macOS 27 beta: app never appears in Local Network at all

On macOS 27 beta seeds (e.g. build `26A5388g`), the Local Network registration
pipeline is broken for newly-seen apps: the prompt never fires, the app never
appears in the settings pane, and every LAN send fails silently. This hits other
apps too (Codex Desktop, LightBurn) — it is an OS bug, not something a reinstall,
permission reset, or re-sign can fix.

Workaround until Apple fixes the seed: launch the app as a **child of a terminal
that already has Local Network access** — it inherits the terminal's grant:

```bash
nohup "/Applications/Lumina Desktop.app/Contents/MacOS/Lumina Desktop" >/dev/null 2>&1 &
```

Add it as an alias (`alias lumina='pkill -x "Lumina Desktop" 2>/dev/null; nohup
"/Applications/Lumina Desktop.app/Contents/MacOS/Lumina Desktop" >/dev/null 2>&1 &
disown'`) and launch with one word. Note: the terminal app itself must have the
grant (check the settings pane) — a terminal that never got it can't lend it.

## Devices show "offline"

- Bulbs and computer must be on the same network/VLAN; UDP port 38899 must be reachable.
- Probe a bulb directly:

  ```bash
  echo '{"method":"getPilot","params":{}}' | nc -u -w1 <bulb-ip> 38899
  ```

  A JSON reply means the bulb is fine and the problem is app-side (usually the
  Local Network permission above). No reply means a network problem.

## Discover finds nothing

- Discovery uses UDP broadcast; some routers/mesh systems isolate wireless clients
  ("AP isolation") — disable that, or add devices via the TUI with a known IP.
- On macOS, broadcast is what triggers the Local Network prompt. If discovery has
  never worked, check the permission first.

## State changes from the phone app don't show up

The window re-syncs every 10 seconds while visible. Heartbeat is paused while the
window is hidden or while you're dragging a dial — give it a moment after switching
back to the app.

## Config

Everything lives in `~/.lumina-config.json`, shared with
[Lumina-TUI](https://github.com/shivarchit/Lumina-TUI). Deleting it resets the app
(devices, groups, theme, last state).
