package main

import (
	"context"
	"fmt"
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
}

// FanoutResult aggregates one command sent to N targets.
type FanoutResult struct {
	OK     int      `json:"ok"`
	Failed []string `json:"failed"`
	Ms     int64    `json:"ms"`
}

// StateResult is a device's live state, or the error fetching it.
type StateResult struct {
	Power      bool   `json:"power"`
	Brightness int    `json:"brightness"`
	ColorHex   string `json:"colorHex"`
	Temp       int    `json:"temp"`
	Ms         int64  `json:"ms"`
	Err        string `json:"err"`
}

// App is the Wails-bound backend.
type App struct {
	ctx context.Context
	mu  sync.Mutex
	cfg config.Config
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	cfg, err := config.Load()
	if err == nil {
		a.cfg = cfg
	}
	if a.cfg.Port == "" {
		a.cfg.Port = "38899"
	}
	// macOS raises the Local Network permission prompt on broadcast traffic,
	// not on connected unicast UDP (which is silently dropped when denied).
	// One background discovery primes the prompt on first launch.
	go func() { _, _ = wiz.DiscoverDevices() }()
}

// GetConfig returns the shared config (same file the TUI uses).
func (a *App) GetConfig() config.Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

func (a *App) persist() {
	_ = config.Save(a.cfg)
}

// SetTheme persists the selected theme name.
func (a *App) SetTheme(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Theme = name
	a.persist()
}

// SetLastState persists last color/brightness/temp for boot restore.
func (a *App) SetLastState(colorHex string, brightness, temp int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if colorHex != "" {
		a.cfg.LastColor = colorHex
	}
	if brightness > 0 {
		a.cfg.LastBrightness = brightness
	}
	if temp > 0 {
		a.cfg.LastColorTemp = temp
	}
	a.persist()
}

// SetPilot sends a setPilot command to every target concurrently.
func (a *App) SetPilot(targets []Target, params map[string]interface{}) FanoutResult {
	return fanout(targets, "setPilot", params)
}

// SetPower sends a setState command to every target concurrently.
func (a *App) SetPower(targets []Target, on bool) FanoutResult {
	return fanout(targets, "setState", map[string]interface{}{"state": on})
}

func fanout(targets []Target, method string, params map[string]interface{}) FanoutResult {
	start := time.Now()
	type result struct {
		idx int
		err error
	}
	results := make(chan result, len(targets))
	for i, t := range targets {
		go func(i int, ip, port string) {
			results <- result{i, wiz.SendCommand(ip, port, method, params)}
		}(i, t.IP, t.Port)
	}
	res := FanoutResult{Failed: []string{}}
	for range targets {
		r := <-results
		if r.err != nil {
			name := targets[r.idx].Name
			if name == "" {
				name = targets[r.idx].IP
			}
			res.Failed = append(res.Failed, name)
		} else {
			res.OK++
		}
	}
	sort.Strings(res.Failed)
	res.Ms = time.Since(start).Milliseconds()
	return res
}

// GetState fetches live state from one device.
func (a *App) GetState(ip, port string) StateResult {
	start := time.Now()
	st, err := wiz.GetPilotState(ip, port)
	out := StateResult{Ms: time.Since(start).Milliseconds()}
	if err != nil {
		out.Err = err.Error()
		return out
	}
	out.Power = st.Power
	out.Brightness = st.Brightness
	out.ColorHex = st.ColorHex
	out.Temp = st.Temp
	return out
}

// Discover scans the local network for WiZ devices.
func (a *App) Discover() []wiz.Device {
	devices, err := wiz.DiscoverDevices()
	if err != nil {
		return []wiz.Device{}
	}
	return devices
}

// SaveDevice upserts a saved device keyed by MAC and persists.
func (a *App) SaveDevice(d config.SavedDevice) error {
	mac := strings.ToLower(strings.TrimSpace(d.Mac))
	if mac == "" {
		return fmt.Errorf("cannot save device without MAC")
	}
	d.Mac = mac
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.cfg.SavedDevices {
		if strings.ToLower(strings.TrimSpace(a.cfg.SavedDevices[i].Mac)) == mac {
			a.cfg.SavedDevices[i] = d
			a.persist()
			return nil
		}
	}
	a.cfg.SavedDevices = append(a.cfg.SavedDevices, d)
	a.persist()
	return nil
}

// DeleteDevice removes a saved device by MAC and from any groups.
func (a *App) DeleteDevice(mac string) {
	mac = strings.ToLower(strings.TrimSpace(mac))
	a.mu.Lock()
	defer a.mu.Unlock()
	kept := a.cfg.SavedDevices[:0]
	for _, d := range a.cfg.SavedDevices {
		if strings.ToLower(strings.TrimSpace(d.Mac)) != mac {
			kept = append(kept, d)
		}
	}
	a.cfg.SavedDevices = kept
	for gi := range a.cfg.Groups {
		macs := a.cfg.Groups[gi].Macs[:0]
		for _, m := range a.cfg.Groups[gi].Macs {
			if strings.ToLower(strings.TrimSpace(m)) != mac {
				macs = append(macs, m)
			}
		}
		a.cfg.Groups[gi].Macs = macs
	}
	a.persist()
}

// SaveGroup creates or replaces a group by name.
func (a *App) SaveGroup(g config.Group) error {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return fmt.Errorf("group name cannot be empty")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.cfg.Groups {
		if a.cfg.Groups[i].Name == g.Name {
			a.cfg.Groups[i] = g
			a.persist()
			return nil
		}
	}
	a.cfg.Groups = append(a.cfg.Groups, g)
	a.persist()
	return nil
}

// DeleteGroup removes a group by name.
func (a *App) DeleteGroup(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	kept := a.cfg.Groups[:0]
	for _, g := range a.cfg.Groups {
		if g.Name != name {
			kept = append(kept, g)
		}
	}
	a.cfg.Groups = kept
	a.persist()
}

// GroupTargets resolves a group's MACs to targets via saved devices.
func (a *App) GroupTargets(name string) []Target {
	a.mu.Lock()
	defer a.mu.Unlock()
	byMac := map[string]config.SavedDevice{}
	for _, d := range a.cfg.SavedDevices {
		byMac[strings.ToLower(strings.TrimSpace(d.Mac))] = d
	}
	var targets []Target
	for _, g := range a.cfg.Groups {
		if g.Name != name {
			continue
		}
		for _, mac := range g.Macs {
			d, ok := byMac[strings.ToLower(strings.TrimSpace(mac))]
			if !ok || d.IP == "" {
				continue
			}
			port := d.Port
			if port == "" {
				port = a.cfg.Port
			}
			targets = append(targets, Target{IP: d.IP, Port: port, Name: d.Name})
		}
	}
	return targets
}
