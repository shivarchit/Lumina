// Package core is the platform-neutral engine shared by the Wails desktop app
// and the Fyne mobile app: config, discovery, stale-IP healing, and command
// fanout. No UI-toolkit imports allowed here.
package core

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shivarchit/Lumina-TUI/pkg/config"
	"github.com/shivarchit/Lumina-TUI/pkg/wiz"
)

// Target addresses one device for a command.
type Target struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
	Name string `json:"name"`
	Mac  string `json:"mac"`
}

// FanoutResult aggregates one command sent to N targets.
type FanoutResult struct {
	OK     int      `json:"ok"`
	Failed []string `json:"failed"`
	Ms     int64    `json:"ms"`
	Hint   string   `json:"hint,omitempty"`
	Healed bool     `json:"healed,omitempty"` // a stale IP was rewritten; frontend must refresh targets
}

// StateResult is a device's live state, or the error fetching it.
type StateResult struct {
	Power      bool   `json:"power"`
	Brightness int    `json:"brightness"`
	ColorHex   string `json:"colorHex"`
	Temp       int    `json:"temp"`
	Ms         int64  `json:"ms"`
	Err        string `json:"err"`
	Hint       string `json:"hint,omitempty"`
}

// NormMac is the one canonical MAC form used for every comparison and map key.
func NormMac(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// Unreachable reports whether err is the unreachable-class failure that both a
// stale DHCP lease and a macOS Local Network denial produce (EHOSTUNREACH).
// It gates the heal-by-MAC retry on every platform.
func Unreachable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no route to host") || strings.Contains(msg, "host is down")
}

// HintFor maps a low-level send error to an actionable user hint. The
// permission hint is macOS-only; other platforms have no such toggle.
func HintFor(err error) string {
	if !Unreachable(err) || runtime.GOOS != "darwin" {
		return ""
	}
	return "macOS is blocking Local Network access — System Settings → Privacy & Security → Local Network → enable Lumina Desktop, then relaunch"
}

// lnPrompt debounces re-raising the Local Network permission flow.
type lnPrompt struct {
	mu        sync.Mutex
	lastPrime time.Time
	lastPane  time.Time
}

// due reports which recovery actions may fire at now: prime every 10s,
// settings pane every 60s.
func (p *lnPrompt) due(now time.Time) (prime, pane bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if now.Sub(p.lastPrime) > 10*time.Second {
		p.lastPrime = now
		prime = true
	}
	if now.Sub(p.lastPane) > 60*time.Second {
		p.lastPane = now
		pane = true
	}
	return
}

var localNet lnPrompt

// healGate debounces per-MAC re-discovery: a dial drag on an offline bulb
// fires debounced sends every ~140ms, and each would otherwise block 3s in a
// discovery broadcast.
var healGate = struct {
	mu   sync.Mutex
	last map[string]time.Time
}{last: map[string]time.Time{}}

func healDue(mac string) bool {
	healGate.mu.Lock()
	defer healGate.mu.Unlock()
	if time.Since(healGate.last[mac]) < 10*time.Second {
		return false
	}
	healGate.last[mac] = time.Now()
	return true
}

// Core owns the shared config and all device I/O.
type Core struct {
	mu  sync.Mutex
	cfg config.Config
}

// New loads the shared config (same file the TUI uses) and primes a background
// discovery that heals saved-device IPs gone stale across DHCP lease changes.
// On macOS the broadcast also raises the Local Network permission prompt.
func New() *Core {
	c := &Core{}
	if cfg, err := config.Load(); err == nil {
		c.cfg = cfg
	}
	if c.cfg.Port == "" {
		c.cfg.Port = "38899"
	}
	go c.RefreshSavedIPs()
	return c
}

// RefreshSavedIPs broadcasts a discovery and rewrites any saved device whose
// IP changed since it was saved (DHCP reassignment while the app was closed).
func (c *Core) RefreshSavedIPs() {
	devices, err := wiz.DiscoverDevices()
	if err != nil {
		return
	}
	byMac := map[string]string{}
	for _, d := range devices {
		byMac[NormMac(d.Mac)] = d.IP
	}
	c.UpdateSavedIPs(byMac)
}

// UpdateSavedIPs applies mac→ip fixes to saved devices; persists if changed.
func (c *Core) UpdateSavedIPs(byMac map[string]string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	changed := false
	for i := range c.cfg.SavedDevices {
		ip := byMac[NormMac(c.cfg.SavedDevices[i].Mac)]
		if ip != "" && ip != c.cfg.SavedDevices[i].IP {
			c.cfg.SavedDevices[i].IP = ip
			changed = true
		}
	}
	if changed {
		c.persist()
	}
	return changed
}

// GetConfig returns the shared config.
func (c *Core) GetConfig() config.Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg
}

func (c *Core) persist() {
	_ = config.Save(c.cfg)
}

// SetTheme persists the selected theme name.
func (c *Core) SetTheme(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg.Theme = name
	c.persist()
}

// SetLastState persists last color/brightness/temp for boot restore.
func (c *Core) SetLastState(colorHex string, brightness, temp int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if colorHex != "" {
		c.cfg.LastColor = colorHex
	}
	if brightness > 0 {
		c.cfg.LastBrightness = brightness
	}
	if temp > 0 {
		c.cfg.LastColorTemp = temp
	}
	c.persist()
}

// SetPilot sends a setPilot command to every target concurrently.
func (c *Core) SetPilot(targets []Target, params map[string]interface{}) FanoutResult {
	return c.Fanout(targets, "setPilot", params)
}

// SetPower sends a setState command to every target concurrently.
func (c *Core) SetPower(targets []Target, on bool) FanoutResult {
	return c.Fanout(targets, "setState", map[string]interface{}{"state": on})
}

// send delivers one command; on an unreachable-class error (stale DHCP lease
// looks identical to a permission denial: "no route to host") it re-resolves
// the device's current IP by MAC, heals the config, and retries once.
func (c *Core) send(t Target, method string, params map[string]interface{}) (healed bool, err error) {
	err = wiz.SendCommand(t.IP, t.Port, method, params)
	if err == nil || t.Mac == "" || !Unreachable(err) || !healDue(t.Mac) {
		return false, err
	}
	// ponytail: blocks this send up to 3s; per-MAC 10s gate keeps it rare.
	// A true permission denial pays it too — indistinguishable from IP drift.
	d, derr := wiz.DiscoverDeviceByMAC(t.Mac, t.Port, 3*time.Second)
	if derr != nil || d.IP == "" || d.IP == t.IP {
		return false, err
	}
	c.UpdateSavedIPs(map[string]string{NormMac(t.Mac): d.IP})
	return true, wiz.SendCommand(d.IP, t.Port, method, params)
}

// Fanout sends one command to every target concurrently and aggregates.
func (c *Core) Fanout(targets []Target, method string, params map[string]interface{}) FanoutResult {
	start := time.Now()
	type result struct {
		idx    int
		healed bool
		err    error
	}
	results := make(chan result, len(targets))
	for i, t := range targets {
		go func(i int, t Target) {
			healed, err := c.send(t, method, params)
			results <- result{i, healed, err}
		}(i, t)
	}
	res := FanoutResult{Failed: []string{}}
	for range targets {
		r := <-results
		if r.healed {
			res.Healed = true
		}
		if r.err != nil {
			name := targets[r.idx].Name
			if name == "" {
				name = targets[r.idx].IP
			}
			res.Failed = append(res.Failed, name)
			if res.Hint == "" {
				res.Hint = HintFor(r.err)
			}
		} else {
			res.OK++
		}
	}
	sort.Strings(res.Failed)
	if res.Hint != "" {
		c.forceLocalNetworkPrompt()
	}
	res.Ms = time.Since(start).Milliseconds()
	return res
}

// forceLocalNetworkPrompt re-raises the macOS Local Network permission flow
// after a blocked send: broadcast traffic re-triggers the system prompt (when
// macOS is still willing to show it), and the privacy pane opens so the user
// can flip the toggle by hand. Debounced so a toggle spree doesn't spam.
// No-op outside darwin (HintFor never fires there).
func (c *Core) forceLocalNetworkPrompt() {
	prime, pane := localNet.due(time.Now())
	if prime {
		go c.RefreshSavedIPs()
	}
	if pane && runtime.GOOS == "darwin" {
		go func() {
			_ = exec.Command("open",
				"x-apple.systempreferences:com.apple.preference.security?Privacy_LocalNetwork").Run()
		}()
	}
}

// GetState fetches live state from one device.
func (c *Core) GetState(ip, port string) StateResult {
	start := time.Now()
	st, err := wiz.GetPilotState(ip, port)
	out := StateResult{Ms: time.Since(start).Milliseconds()}
	if err != nil {
		out.Err = err.Error()
		out.Hint = HintFor(err)
		return out
	}
	out.Power = st.Power
	out.Brightness = st.Brightness
	out.ColorHex = st.ColorHex
	out.Temp = st.Temp
	return out
}

// Discover scans the local network for WiZ devices.
func (c *Core) Discover() []wiz.Device {
	devices, err := wiz.DiscoverDevices()
	if err != nil {
		return []wiz.Device{}
	}
	return devices
}

// SaveDevice upserts a saved device keyed by MAC and persists.
func (c *Core) SaveDevice(d config.SavedDevice) error {
	mac := NormMac(d.Mac)
	if mac == "" {
		return fmt.Errorf("cannot save device without MAC")
	}
	d.Mac = mac
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.cfg.SavedDevices {
		if NormMac(c.cfg.SavedDevices[i].Mac) == mac {
			c.cfg.SavedDevices[i] = d
			c.persist()
			return nil
		}
	}
	c.cfg.SavedDevices = append(c.cfg.SavedDevices, d)
	c.persist()
	return nil
}

// DeleteDevice removes a saved device by MAC and from any groups.
func (c *Core) DeleteDevice(mac string) {
	mac = NormMac(mac)
	c.mu.Lock()
	defer c.mu.Unlock()
	kept := c.cfg.SavedDevices[:0]
	for _, d := range c.cfg.SavedDevices {
		if NormMac(d.Mac) != mac {
			kept = append(kept, d)
		}
	}
	c.cfg.SavedDevices = kept
	for gi := range c.cfg.Groups {
		macs := c.cfg.Groups[gi].Macs[:0]
		for _, m := range c.cfg.Groups[gi].Macs {
			if NormMac(m) != mac {
				macs = append(macs, m)
			}
		}
		c.cfg.Groups[gi].Macs = macs
	}
	c.persist()
}

// SaveGroup creates or replaces a group by name.
func (c *Core) SaveGroup(g config.Group) error {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return fmt.Errorf("group name cannot be empty")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.cfg.Groups {
		if c.cfg.Groups[i].Name == g.Name {
			c.cfg.Groups[i] = g
			c.persist()
			return nil
		}
	}
	c.cfg.Groups = append(c.cfg.Groups, g)
	c.persist()
	return nil
}

// DeleteGroup removes a group by name.
func (c *Core) DeleteGroup(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	kept := c.cfg.Groups[:0]
	for _, g := range c.cfg.Groups {
		if g.Name != name {
			kept = append(kept, g)
		}
	}
	c.cfg.Groups = kept
	c.persist()
}

// toTarget builds a Target from a saved device, applying the port fallback.
// The single place that rule lives — both frontends resolve through it.
func toTarget(d config.SavedDevice, defaultPort string) Target {
	port := d.Port
	if port == "" {
		port = defaultPort
	}
	return Target{IP: d.IP, Port: port, Name: d.Name, Mac: d.Mac}
}

// DeviceTargets resolves one saved device (by MAC) to a target list.
func (c *Core) DeviceTargets(mac string) []Target {
	mac = NormMac(mac)
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, d := range c.cfg.SavedDevices {
		if NormMac(d.Mac) == mac && d.IP != "" {
			return []Target{toTarget(d, c.cfg.Port)}
		}
	}
	return nil
}

// GroupTargets resolves a group's MACs to targets via saved devices.
func (c *Core) GroupTargets(name string) []Target {
	c.mu.Lock()
	defer c.mu.Unlock()
	byMac := map[string]config.SavedDevice{}
	for _, d := range c.cfg.SavedDevices {
		byMac[NormMac(d.Mac)] = d
	}
	var targets []Target
	for _, g := range c.cfg.Groups {
		if g.Name != name {
			continue
		}
		for _, mac := range g.Macs {
			d, ok := byMac[NormMac(mac)]
			if !ok || d.IP == "" {
				continue
			}
			targets = append(targets, toTarget(d, c.cfg.Port))
		}
	}
	return targets
}
