// Lumina mobile: Fyne shell over the shared core engine, mirroring the
// desktop app's visual language (ambient glow, mono pills, thin numerals).
package main

import (
	"fmt"
	"image/color"
	"os"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"Lumina/core"

	"github.com/shivarchit/Lumina-TUI/pkg/config"
)

// Scene names/IDs match desktop (frontend/src/main.js SCENES).
var scenes = []struct {
	Name string
	ID   int
}{
	{"OCEAN", 1}, {"ROMANCE", 2}, {"SUNSET", 3}, {"PARTY", 4},
	{"FIREPLACE", 5}, {"COZY", 6}, {"FOREST", 7}, {"PASTEL", 8},
	{"WAKE-UP", 9}, {"BEDTIME", 10}, {"DAYLIGHT", 12}, {"FOCUS", 15},
}

type ui struct {
	eng *core.Core
	win fyne.Window

	sel           string // selected target: device MAC or "g:"+group name
	mode          string // dim | temp | scenes
	status        *canvas.Text
	dial          *dial
	content       *fyne.Container // swapped between home and manage
	manageRebuild func()
}

func main() {
	a := app.NewWithID("com.shivarchit.lumina")
	a.Settings().SetTheme(newLuminaTheme())
	// Android has no usable $HOME; point the shared config at app storage.
	if runtime.GOOS == "android" {
		if root := a.Storage().RootURI().Path(); root != "" {
			os.Setenv("HOME", root)
		}
	}
	u := &ui{eng: core.New(), mode: "dim", win: a.NewWindow("Lumina")}
	u.status = canvas.NewText("", colDim)
	u.status.TextSize = 10
	u.status.TextStyle = fyne.TextStyle{Monospace: true}
	u.content = container.NewStack()

	cfg := u.eng.GetConfig()
	if len(cfg.SavedDevices) > 0 {
		u.sel = strings.ToLower(cfg.SavedDevices[0].Mac)
	}

	bg := canvas.NewLinearGradient(colBG, colBGDeep, 0)
	glow := canvas.NewRadialGradient(withAlpha(colAccent, 0x24), withAlpha(colAccent, 0x00))
	u.win.SetContent(container.NewStack(bg, container.New(&glowLayout{}, glow), u.content))
	u.showHome()
	u.win.ShowAndRun()
}

// glowLayout parks the ambient blob top-left, oversized, like desktop's #blob-a.
type glowLayout struct{}

func (g *glowLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }
func (g *glowLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	for _, o := range objs {
		o.Resize(fyne.NewSize(s.Width*1.4, s.Width*1.4))
		o.Move(fyne.NewPos(-s.Width*0.35, -s.Width*0.55))
	}
}

// ── target resolution ───────────────────────────────────────────────

func (u *ui) targets() []core.Target {
	cfg := u.eng.GetConfig()
	if strings.HasPrefix(u.sel, "g:") {
		return u.eng.GroupTargets(strings.TrimPrefix(u.sel, "g:"))
	}
	for _, d := range cfg.SavedDevices {
		if strings.ToLower(d.Mac) == u.sel {
			port := d.Port
			if port == "" {
				port = cfg.Port
			}
			return []core.Target{{IP: d.IP, Port: port, Name: d.Name, Mac: d.Mac}}
		}
	}
	return nil
}

func (u *ui) targetLabel() (name, sub string) {
	cfg := u.eng.GetConfig()
	if strings.HasPrefix(u.sel, "g:") {
		n := strings.TrimPrefix(u.sel, "g:")
		return n, fmt.Sprintf("group · %d devices", len(u.eng.GroupTargets(n)))
	}
	for _, d := range cfg.SavedDevices {
		if strings.ToLower(d.Mac) == u.sel {
			return d.Name, "wiz · " + d.IP
		}
	}
	return "no device", "tap ✚ to discover"
}

// ── sends ───────────────────────────────────────────────────────────

func (u *ui) report(action string, res core.FanoutResult) {
	fyne.Do(func() {
		if len(res.Failed) > 0 {
			u.status.Color = colErr
			u.status.Text = fmt.Sprintf("%s · %d ok · failed: %s", action, res.OK, strings.Join(res.Failed, ", "))
		} else {
			u.status.Color = colOK
			u.status.Text = fmt.Sprintf("%s · %d ok · %dms", action, res.OK, res.Ms)
		}
		u.status.Refresh()
	})
}

func (u *ui) sendPilot(action string, params map[string]interface{}) {
	ts := u.targets()
	go u.report(action, u.eng.SetPilot(ts, params))
}

// ── home screen ─────────────────────────────────────────────────────

func (u *ui) showHome() {
	if u.sel == "" {
		if cfg := u.eng.GetConfig(); len(cfg.SavedDevices) > 0 {
			u.sel = strings.ToLower(cfg.SavedDevices[0].Mac)
		}
	}
	name, sub := u.targetLabel()
	title := canvas.NewText(name, colText)
	title.TextSize = 17
	title.Alignment = fyne.TextAlignCenter
	subT := canvas.NewText(strings.ToUpper(sub), colDim)
	subT.TextSize = 9
	subT.TextStyle = fyne.TextStyle{Monospace: true}
	subT.Alignment = fyne.TextAlignCenter

	var center fyne.CanvasObject
	switch u.mode {
	case "temp":
		center = u.tempView()
	case "scenes":
		center = u.scenesView()
	default:
		center = u.dialView()
	}

	modes := container.NewHBox()
	for _, m := range []string{"dim", "temp", "scenes"} {
		m := m
		p := newPill(strings.ToUpper(m), func() { u.mode = m; u.showHome() })
		p.setOn(u.mode == m)
		modes.Add(p)
	}

	u.content.Objects = []fyne.CanvasObject{container.NewBorder(
		container.NewVBox(u.switcher(), title, subT),
		container.NewVBox(container.NewCenter(modes), container.NewCenter(u.status)),
		nil, nil,
		container.NewCenter(center),
	)}
	u.content.Refresh()
}

func (u *ui) dialView() fyne.CanvasObject {
	u.dial = newDial(
		func(v int) {
			u.sendPilot(fmt.Sprintf("dim %d%%", v), map[string]interface{}{"dimming": v, "state": true})
			u.eng.SetLastState("", v, 0)
		},
		func(on bool) {
			ts := u.targets()
			lab := "off"
			if on {
				lab = "on"
			}
			go u.report(lab, u.eng.SetPower(ts, on))
		},
	)
	if b := u.eng.GetConfig().LastBrightness; b > 0 {
		u.dial.set(float64(b), true)
	}
	u.syncState()
	return u.dial
}

// syncState pulls live device state into the dial (single-device targets).
func (u *ui) syncState() {
	ts := u.targets()
	if len(ts) != 1 {
		return
	}
	d := u.dial
	go func() {
		st := u.eng.GetState(ts[0].IP, ts[0].Port)
		if st.Err != "" {
			return
		}
		fyne.Do(func() {
			if u.dial == d { // still on the same dial view
				d.set(float64(st.Brightness), st.Power)
			}
		})
	}()
}

func (u *ui) tempView() fyne.CanvasObject {
	val := canvas.NewText("3200", colText)
	val.TextSize = 38
	val.TextStyle = fyne.TextStyle{Monospace: true}
	val.Alignment = fyne.TextAlignCenter
	unit := canvas.NewText("KELVIN", colDim)
	unit.TextSize = 9
	unit.TextStyle = fyne.TextStyle{Monospace: true}
	unit.Alignment = fyne.TextAlignCenter

	s := widget.NewSlider(2200, 6500)
	s.Step = 100
	if t := u.eng.GetConfig().LastColorTemp; t > 0 {
		s.SetValue(float64(t))
		val.Text = fmt.Sprintf("%d", t)
	} else {
		s.SetValue(3200)
	}
	s.OnChanged = func(v float64) {
		val.Text = fmt.Sprintf("%d", int(v))
		val.Refresh()
	}
	s.OnChangeEnded = func(v float64) {
		u.sendPilot(fmt.Sprintf("%dK", int(v)), map[string]interface{}{"temp": int(v), "state": true})
		u.eng.SetLastState("", 0, int(v))
	}

	marks := container.NewGridWithColumns(4)
	for _, m := range []string{"CANDLE", "WARM", "NEUTRAL", "DAY"} {
		t := canvas.NewText(m, colDim)
		t.TextSize = 8
		t.TextStyle = fyne.TextStyle{Monospace: true}
		t.Alignment = fyne.TextAlignCenter
		marks.Add(t)
	}
	box := container.NewVBox(val, unit, widget.NewLabel(""), s, marks)
	return container.NewGridWrap(fyne.NewSize(300, 220), box)
}

func (u *ui) scenesView() fyne.CanvasObject {
	grid := container.NewGridWithColumns(3)
	for _, sc := range scenes {
		sc := sc
		grid.Add(newPill(sc.Name, func() {
			u.sendPilot(strings.ToLower(sc.Name), map[string]interface{}{"sceneId": sc.ID, "state": true})
		}))
	}
	return container.NewGridWrap(fyne.NewSize(300, 260), container.NewVScroll(grid))
}

// ── switcher ────────────────────────────────────────────────────────

func (u *ui) switcher() fyne.CanvasObject {
	cfg := u.eng.GetConfig()
	row := container.NewHBox()
	for _, d := range cfg.SavedDevices {
		mac := strings.ToLower(d.Mac)
		p := newPill(strings.ToUpper(d.Name), func() { u.sel = mac; u.showHome() })
		p.setOn(u.sel == mac)
		row.Add(p)
	}
	for _, g := range cfg.Groups {
		key := "g:" + g.Name
		p := newPill("◇ "+strings.ToUpper(g.Name), func() { u.sel = key; u.showHome() })
		p.setOn(u.sel == key)
		row.Add(p)
	}
	plus := newPill("✚", func() { u.showManage() })
	return container.NewBorder(nil, nil, nil, plus, container.NewHScroll(row))
}

// ── manage screen ───────────────────────────────────────────────────

func (u *ui) showManage() {
	head := container.NewBorder(nil, nil,
		widget.NewLabel("Manage"),
		newPill("✕", func() { u.showHome() }),
	)

	body := container.NewVBox()
	rebuild := func() { u.buildManageBody(body) }
	u.manageRebuild = rebuild
	rebuild()

	u.content.Objects = []fyne.CanvasObject{container.NewBorder(
		head,
		container.NewCenter(u.status),
		nil, nil,
		container.NewVScroll(body),
	)}
	u.content.Refresh()
}

func (u *ui) sectionLabel(s string) fyne.CanvasObject {
	t := canvas.NewText(strings.ToUpper(s), colDim)
	t.TextSize = 9
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return container.NewPadded(t)
}

func (u *ui) buildManageBody(body *fyne.Container) {
	cfg := u.eng.GetConfig()
	body.RemoveAll()

	// discover
	body.Add(u.sectionLabel("discover"))
	body.Add(newPill("SCAN NETWORK", func() {
		u.setStatus("scanning…", colDim)
		go func() {
			found := u.eng.Discover()
			saved := map[string]bool{}
			for _, d := range u.eng.GetConfig().SavedDevices {
				saved[strings.ToLower(strings.TrimSpace(d.Mac))] = true
			}
			fresh := 0
			for _, d := range found {
				if !saved[strings.ToLower(strings.TrimSpace(d.Mac))] {
					name := d.Name
					if name == "" {
						name = d.Model
					}
					if u.eng.SaveDevice(config.SavedDevice{Name: name, IP: d.IP, Mac: d.Mac}) == nil {
						fresh++
					}
				}
			}
			fyne.Do(func() {
				u.setStatus(fmt.Sprintf("found %d · added %d", len(found), fresh), colOK)
				if u.manageRebuild != nil {
					u.manageRebuild()
				}
			})
		}()
	}))

	// saved devices: rename inline, two-tap delete
	body.Add(u.sectionLabel("saved devices"))
	for _, d := range cfg.SavedDevices {
		d := d
		lbl := widget.NewLabel(fmt.Sprintf("%s\n%s · %s", d.Name, d.IP, d.Mac))
		row := container.NewBorder(nil, nil, nil, nil, lbl)
		rename := newPill("RENAME", nil)
		var del *pill
		del = newPill("DELETE", func() {
			if del.text != "SURE?" {
				del.text = "SURE?"
				del.accent = colErr
				del.setOn(true)
				return
			}
			u.eng.DeleteDevice(d.Mac)
			if u.sel == strings.ToLower(d.Mac) {
				u.sel = ""
			}
			u.manageRebuild()
		})
		rename.onTapped = func() {
			entry := widget.NewEntry()
			entry.SetText(d.Name)
			save := newPill("SAVE", func() {
				nd := d
				nd.Name = strings.TrimSpace(entry.Text)
				if nd.Name != "" {
					_ = u.eng.SaveDevice(nd)
				}
				u.manageRebuild()
			})
			row.Objects = []fyne.CanvasObject{container.NewBorder(nil, nil, nil, save, entry)}
			row.Refresh()
		}
		body.Add(container.NewBorder(nil, nil, nil, container.NewHBox(rename, del), row))
	}

	// groups
	body.Add(u.sectionLabel("groups"))
	for _, g := range cfg.Groups {
		g := g
		lbl := widget.NewLabel(fmt.Sprintf("◇ %s · %d devices", g.Name, len(g.Macs)))
		var del *pill
		del = newPill("DELETE", func() {
			if del.text != "SURE?" {
				del.text = "SURE?"
				del.accent = colErr
				del.setOn(true)
				return
			}
			u.eng.DeleteGroup(g.Name)
			if u.sel == "g:"+g.Name {
				u.sel = ""
			}
			u.manageRebuild()
		})
		body.Add(container.NewBorder(nil, nil, nil, del, lbl))
	}

	// group composer
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("new group name")
	members := map[string]bool{}
	memberRow := container.NewHBox()
	for _, d := range cfg.SavedDevices {
		mac := strings.ToLower(d.Mac)
		var p *pill
		p = newPill(strings.ToUpper(d.Name), func() {
			members[mac] = !members[mac]
			p.setOn(members[mac])
		})
		memberRow.Add(p)
	}
	create := newPill("CREATE GROUP", func() {
		var macs []string
		for m, on := range members {
			if on {
				macs = append(macs, m)
			}
		}
		if err := u.eng.SaveGroup(config.Group{Name: nameEntry.Text, Macs: macs}); err != nil {
			u.setStatus(err.Error(), colErr)
			return
		}
		u.manageRebuild()
	})
	body.Add(nameEntry)
	body.Add(container.NewHScroll(memberRow))
	body.Add(container.NewCenter(create))
	body.Refresh()
}

func (u *ui) setStatus(s string, c color.NRGBA) {
	u.status.Text = s
	u.status.Color = c
	u.status.Refresh()
}
