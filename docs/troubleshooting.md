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
