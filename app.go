package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"Lumina/core"

	"github.com/shivarchit/Lumina-TUI/pkg/config"
	"github.com/shivarchit/Lumina-TUI/pkg/wiz"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound backend: a thin shell over core.Core, which holds all
// engine logic shared with the mobile app. Method names and signatures are the
// frontend's contract — keep them stable.
type App struct {
	ctx  context.Context
	core *core.Core
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.core = core.New()
}

func (a *App) GetConfig() config.Config      { return a.core.GetConfig() }
func (a *App) SetTheme(name string)          { a.core.SetTheme(name) }
func (a *App) Discover() []wiz.Device        { return a.core.Discover() }
func (a *App) SaveDevice(d config.SavedDevice) error { return a.core.SaveDevice(d) }
func (a *App) DeleteDevice(mac string)       { a.core.DeleteDevice(mac) }
func (a *App) SaveGroup(g config.Group) error { return a.core.SaveGroup(g) }
func (a *App) DeleteGroup(name string)       { a.core.DeleteGroup(name) }

func (a *App) SetLastState(colorHex string, brightness, temp int) {
	a.core.SetLastState(colorHex, brightness, temp)
}

func (a *App) SetPilot(targets []core.Target, params map[string]interface{}) core.FanoutResult {
	return a.core.SetPilot(targets, params)
}

func (a *App) SetPower(targets []core.Target, on bool) core.FanoutResult {
	return a.core.SetPower(targets, on)
}

func (a *App) GetState(ip, port string) core.StateResult { return a.core.GetState(ip, port) }

func (a *App) GroupTargets(name string) []core.Target { return a.core.GroupTargets(name) }

// appVersion is stamped by release builds via -ldflags "-X main.appVersion=vX.Y.Z".
var appVersion = "dev"

// CheckUpdate returns the latest release tag when it differs from this build,
// or "" for up-to-date, dev builds, and network failures.
func (a *App) CheckUpdate() string {
	if !strings.HasPrefix(appVersion, "v") {
		return ""
	}
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/shivarchit/Lumina/releases/latest")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if json.NewDecoder(resp.Body).Decode(&rel) != nil || rel.Tag == "" || rel.Tag == appVersion {
		return ""
	}
	return rel.Tag
}

// OpenReleases opens the GitHub releases page in the default browser.
func (a *App) OpenReleases() {
	wruntime.BrowserOpenURL(a.ctx, "https://github.com/shivarchit/Lumina/releases/latest")
}
