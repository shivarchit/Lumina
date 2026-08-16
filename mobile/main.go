// Lumina mobile: minimal Fyne shell over the shared core engine.
// Android-first; also runs as a plain desktop window for development.
package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"Lumina/core"

	"github.com/shivarchit/Lumina-TUI/pkg/config"
)

func main() {
	eng := core.New()
	a := app.New()
	w := a.NewWindow("Lumina")
	w.Resize(fyne.NewSize(380, 640))

	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	list := container.NewVBox()

	targets := func() []core.Target {
		cfg := eng.GetConfig()
		var ts []core.Target
		for _, d := range cfg.SavedDevices {
			port := d.Port
			if port == "" {
				port = cfg.Port
			}
			ts = append(ts, core.Target{IP: d.IP, Port: port, Name: d.Name, Mac: d.Mac})
		}
		return ts
	}

	report := func(action string, res core.FanoutResult) {
		fyne.Do(func() {
			if len(res.Failed) > 0 {
				status.SetText(fmt.Sprintf("%s: %d ok, failed: %s", action, res.OK, strings.Join(res.Failed, ", ")))
				return
			}
			status.SetText(fmt.Sprintf("%s: %d ok (%dms)", action, res.OK, res.Ms))
		})
	}

	rebuild := func() {
		list.RemoveAll()
		for _, t := range targets() {
			t := t
			name := t.Name
			if name == "" {
				name = t.IP
			}
			on := widget.NewButton("On", func() {
				go report(name, eng.SetPower([]core.Target{t}, true))
			})
			off := widget.NewButton("Off", func() {
				go report(name, eng.SetPower([]core.Target{t}, false))
			})
			list.Add(container.NewBorder(nil, nil, widget.NewLabel(name), container.NewHBox(on, off)))
		}
		if len(list.Objects) == 0 {
			list.Add(widget.NewLabel("No saved devices — tap Discover."))
		}
		list.Refresh()
	}

	brightness := widget.NewSlider(10, 100)
	if b := eng.GetConfig().LastBrightness; b > 0 {
		brightness.SetValue(float64(b))
	} else {
		brightness.SetValue(80)
	}
	brightness.OnChangeEnded = func(v float64) {
		go func() {
			res := eng.SetPilot(targets(), map[string]interface{}{"dimming": int(v)})
			eng.SetLastState("", int(v), 0)
			report(fmt.Sprintf("brightness %d%%", int(v)), res)
		}()
	}

	discover := widget.NewButton("Discover", func() {
		status.SetText("Scanning…")
		go func() {
			found := eng.Discover()
			saved := map[string]bool{}
			for _, d := range eng.GetConfig().SavedDevices {
				saved[strings.ToLower(strings.TrimSpace(d.Mac))] = true
			}
			added := 0
			for _, d := range found {
				if saved[strings.ToLower(strings.TrimSpace(d.Mac))] {
					continue // keep user-given names on already-saved devices
				}
				name := d.Name
				if name == "" {
					name = d.Model
				}
				if eng.SaveDevice(config.SavedDevice{Name: name, IP: d.IP, Mac: d.Mac}) == nil {
					added++
				}
			}
			fyne.Do(func() {
				status.SetText(fmt.Sprintf("Found %d, added %d", len(found), added))
				rebuild()
			})
		}()
	})

	rebuild()
	w.SetContent(container.NewBorder(
		discover,
		container.NewVBox(widget.NewLabel("Brightness (all devices)"), brightness, status),
		nil, nil,
		container.NewVScroll(list),
	))
	w.ShowAndRun()
}
