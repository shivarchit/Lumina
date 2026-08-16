// Lumina mobile: Fyne shell over the shared core engine, mirroring the
// desktop app's visual language (ambient glow, mono pills, thin numerals).
package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"Lumina/core"

	"github.com/shivarchit/Lumina-TUI/pkg/config"
)

// Scene names/IDs/colors match desktop (frontend/src/main.js SCENES).
var scenes = []struct {
	Name string
	ID   int
	Tint color.NRGBA
}{
	{"OCEAN", 1, hex(0x0891B2)}, {"ROMANCE", 2, hex(0xF472B6)},
	{"SUNSET", 3, hex(0xFB923C)}, {"PARTY", 4, hex(0xE879F9)},
	{"FIREPLACE", 5, hex(0xF97316)}, {"COZY", 6, hex(0xFDBA74)},
	{"FOREST", 7, hex(0x4ADE80)}, {"PASTEL", 8, hex(0xF9A8D4)},
	{"WAKE-UP", 9, hex(0xFDE68A)}, {"BEDTIME", 10, hex(0x818CF8)},
	{"DAYLIGHT", 12, hex(0xFCD34D)}, {"FOCUS", 15, hex(0x7DD3FC)},
}

type ui struct {
	app fyne.App
	eng *core.Core
	win fyne.Window

	sel           string // selected target: device MAC or "g:"+group name
	mode          string // bright | temp | scenes | timer
	status        *canvas.Text
	dial          *dial
	content       *fyne.Container // swapped between home and manage
	manageRebuild func()

	timerEnd time.Time
	timerT   *time.Timer
}

func main() {
	a := app.NewWithID("com.shivarchit.lumina")
	// Android has no usable $HOME; point the shared config at app storage.
	if runtime.GOOS == "android" {
		if root := a.Storage().RootURI().Path(); root != "" {
			os.Setenv("HOME", root)
		}
	}
	u := &ui{app: a, eng: core.New(), mode: "bright", win: a.NewWindow("Lumina")}
	applyPalette(paletteFor(u.eng.GetConfig().Theme))
	a.Settings().SetTheme(newLuminaTheme())
	u.status = canvas.NewText("", colDim)
	u.status.TextSize = 10
	u.status.TextStyle = fyne.TextStyle{Monospace: true}
	u.content = container.NewStack()

	cfg := u.eng.GetConfig()
	if len(cfg.SavedDevices) > 0 {
		u.sel = strings.ToLower(cfg.SavedDevices[0].Mac)
	}

	u.setChrome()
	u.showHome()
	u.win.ShowAndRun()
}

// setChrome (re)builds the themed backdrop: gradient, glow, ambient bulbs.
// Called at boot and again on every theme switch.
func (u *ui) setChrome() {
	bg := canvas.NewLinearGradient(colBG, colBGDeep, 0)
	glow := canvas.NewRadialGradient(withAlpha(colAccent, 0x24), withAlpha(colAccent, 0x00))
	u.win.SetContent(container.NewStack(bg, container.New(&glowLayout{}, glow), newAmbient(), u.content))
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
	// empty state: no devices yet — invite straight into discover
	if cfg := u.eng.GetConfig(); len(cfg.SavedDevices) == 0 {
		msg := monoText("NO DEVICES ADDED", 11, colDim)
		msg.Alignment = fyne.TextAlignCenter
		go2 := newPill("✚ DISCOVER DEVICES", func() { u.showManage(true) })
		go2.setOn(true)
		u.content.Objects = []fyne.CanvasObject{container.NewCenter(
			container.NewVBox(msg, widget.NewLabel(""), container.NewCenter(go2)),
		)}
		u.content.Refresh()
		return
	}

	name, sub := u.targetLabel()
	title := canvas.NewText(name, colText)
	title.TextSize = 19
	title.Alignment = fyne.TextAlignCenter
	subT := monoText(strings.ToUpper(sub), 9, colDim)
	subT.Alignment = fyne.TextAlignCenter

	var center fyne.CanvasObject
	switch u.mode {
	case "temp":
		center = u.tempView()
	case "scenes":
		center = u.scenesView()
	case "timer":
		center = u.timerView()
	default:
		center = u.dialView()
	}

	modes := container.NewHBox()
	for _, m := range []string{"bright", "temp", "scenes", "timer"} {
		m := m
		p := newPill(strings.ToUpper(m), func() { u.mode = m; u.showHome() })
		p.size = 11
		p.setOn(u.mode == m)
		modes.Add(p)
	}

	gap := func(h float32) fyne.CanvasObject {
		r := canvas.NewRectangle(colTransparent)
		r.SetMinSize(fyne.NewSize(1, h))
		return r
	}
	// one centered column — no dead voids between title, dial, and modes
	u.content.Objects = []fyne.CanvasObject{container.NewBorder(
		u.switcher(), nil, nil, nil,
		container.NewCenter(container.NewVBox(
			title, subT,
			gap(26),
			container.NewCenter(center),
			gap(26),
			container.NewCenter(modes),
			gap(8),
			container.NewCenter(u.status),
		)),
	)}
	u.content.Refresh()
}

func (u *ui) dialView() fyne.CanvasObject {
	u.dial = newDial(
		func(v int) {
			u.sendPilot(fmt.Sprintf("brightness %d%%", v), map[string]interface{}{"dimming": v, "state": true})
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

// kelvinColor approximates black-body light color for a kelvin value
// (Tanner Helland fit) — the slider strip and numeral preview real output.
func kelvinColor(k float64) color.NRGBA {
	t := k / 100
	clamp := func(v float64) uint8 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(v)
	}
	var r, g, b float64
	if t <= 66 {
		r = 255
		g = 99.4708025861*math.Log(t) - 161.1195681661
	} else {
		r = 329.698727446 * math.Pow(t-60, -0.1332047592)
		g = 288.1221695283 * math.Pow(t-60, -0.0755148492)
	}
	if t >= 66 {
		b = 255
	} else if t <= 19 {
		b = 0
	} else {
		b = 138.5177312231*math.Log(t-10) - 305.0447927307
	}
	return color.NRGBA{R: clamp(r), G: clamp(g), B: clamp(b), A: 0xFF}
}

func (u *ui) tempView() fyne.CanvasObject {
	val := canvas.NewText("3200", colText)
	val.TextSize = 38
	val.TextStyle = fyne.TextStyle{Monospace: true}
	val.Alignment = fyne.TextAlignCenter
	unit := monoText("KELVIN", 9, colDim)
	unit.Alignment = fyne.TextAlignCenter

	// live preview strip: actual kelvin ramp behind the slider
	strip := canvas.NewRaster(func(w, h int) image.Image {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		for x := 0; x < w; x++ {
			c := kelvinColor(2200 + (6500-2200)*float64(x)/float64(w))
			c.A = 0xCC
			for y := 0; y < h; y++ {
				img.SetNRGBA(x, y, c)
			}
		}
		return img
	})

	setPreview := func(k float64) {
		val.Text = fmt.Sprintf("%d", int(k))
		val.Color = kelvinColor(k)
		val.Refresh()
	}

	s := widget.NewSlider(2200, 6500)
	s.Step = 100
	start := 3200.0
	if t := u.eng.GetConfig().LastColorTemp; t > 0 {
		start = float64(t)
	}
	s.SetValue(start)
	setPreview(start)
	s.OnChanged = setPreview
	s.OnChangeEnded = func(v float64) {
		u.sendPilot(fmt.Sprintf("%dK", int(v)), map[string]interface{}{"temp": int(v), "state": true})
		u.eng.SetLastState("", 0, int(v))
	}

	marks := container.NewGridWithColumns(4)
	for _, m := range []string{"CANDLE", "WARM", "NEUTRAL", "DAY"} {
		t := monoText(m, 8, colDim)
		t.Alignment = fyne.TextAlignCenter
		marks.Add(t)
	}
	box := container.NewVBox(
		val, unit,
		widget.NewLabel(""),
		container.NewGridWrap(fyne.NewSize(300, 8), strip),
		s, marks,
	)
	return container.NewGridWrap(fyne.NewSize(300, 250), box)
}

func (u *ui) timerView() fyne.CanvasObject {
	disp := canvas.NewText("–", colText)
	disp.TextSize = 34
	disp.TextStyle = fyne.TextStyle{Monospace: true}
	disp.Alignment = fyne.TextAlignCenter
	sub := monoText("SLEEP TIMER · TURNS TARGET OFF", 9, colDim)
	sub.Alignment = fyne.TextAlignCenter

	render := func() {
		if u.timerEnd.IsZero() {
			disp.Text = "–"
		} else {
			left := time.Until(u.timerEnd)
			if left < 0 {
				left = 0
			}
			disp.Text = fmt.Sprintf("%d:%02d", int(left.Minutes()), int(left.Seconds())%60)
		}
		disp.Refresh()
	}
	render()
	// ticker dies with the view: stops once the user leaves timer mode
	go func() {
		for range time.Tick(time.Second) {
			if u.mode != "timer" {
				return
			}
			fyne.Do(render)
		}
	}()

	cancel := newTintPill("CANCEL", colErr, func() {
		if u.timerT != nil {
			u.timerT.Stop()
		}
		u.timerEnd = time.Time{}
		u.setStatus("timer cancelled", colDim)
		render()
	})
	start := func(mins int) {
		if u.timerT != nil {
			u.timerT.Stop()
		}
		ts := u.targets()
		u.timerEnd = time.Now().Add(time.Duration(mins) * time.Minute)
		u.timerT = time.AfterFunc(time.Duration(mins)*time.Minute, func() {
			u.report("sleep off", u.eng.SetPower(ts, false))
			u.timerEnd = time.Time{}
		})
		u.setStatus(fmt.Sprintf("sleep in %dm", mins), colOK)
		render()
	}

	presets := container.NewHBox()
	for _, m := range []int{15, 30, 60, 120} {
		m := m
		presets.Add(newPill(fmt.Sprintf("%dM", m), func() { start(m) }))
	}

	// ponytail: in-app timer only — dies if the app quits, same as desktop.
	box := container.NewVBox(
		disp, sub,
		widget.NewLabel(""),
		container.NewCenter(presets),
		container.NewCenter(cancel),
	)
	return container.NewGridWrap(fyne.NewSize(320, 240), box)
}

func (u *ui) scenesView() fyne.CanvasObject {
	grid := container.NewGridWithColumns(3)
	for _, sc := range scenes {
		sc := sc
		p := newTintPill(sc.Name, sc.Tint, func() {
			u.sendPilot(strings.ToLower(sc.Name), map[string]interface{}{"sceneId": sc.ID, "state": true})
		})
		p.size = 10
		grid.Add(container.NewPadded(p))
	}
	return container.NewGridWrap(fyne.NewSize(330, 330), grid)
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
	plus := newTintPill("✚", colAccent, func() { u.showManage(false) })
	return container.NewBorder(nil, nil, nil, plus, container.NewHScroll(row))
}

// ── manage screen ───────────────────────────────────────────────────

func (u *ui) showManage(autoScan bool) {
	title := canvas.NewText("Manage", colText)
	title.TextSize = 19
	head := container.NewPadded(container.NewBorder(nil, nil,
		title,
		newTintPill("✕ CLOSE", colAccent, func() { u.showHome() }),
	))

	body := container.NewVBox()
	rebuild := func() { u.buildManageBody(body) }
	u.manageRebuild = rebuild
	rebuild()

	u.content.Objects = []fyne.CanvasObject{container.NewBorder(
		head,
		container.NewCenter(u.status),
		nil, nil,
		container.NewVScroll(container.NewPadded(body)),
	)}
	u.content.Refresh()
	if autoScan {
		u.scan()
	}
}

// scan discovers, saves anything new (keeping user names), rebuilds manage.
func (u *ui) scan() {
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
}

func (u *ui) sectionLabel(s string) fyne.CanvasObject {
	return container.NewPadded(monoText(strings.ToUpper(s), 9, colDim))
}

// deviceTitle renders a card's name + mono metadata column.
func deviceTitle(name, meta string) fyne.CanvasObject {
	n := canvas.NewText(name, colText)
	n.TextSize = 14
	return container.NewVBox(n, monoText(meta, 9, colDim))
}

// confirmPill turns a DELETE pill into a two-tap confirm; red from the start.
func confirmPill(onConfirm func()) *pill {
	var del *pill
	del = newTintPill("DELETE", colErr, func() {
		if del.text != "SURE?" {
			del.text = "SURE?"
			del.setOn(true)
			return
		}
		onConfirm()
	})
	return del
}

func (u *ui) buildManageBody(body *fyne.Container) {
	cfg := u.eng.GetConfig()
	body.RemoveAll()
	gap := func(h float32) fyne.CanvasObject {
		r := canvas.NewRectangle(colTransparent)
		r.SetMinSize(fyne.NewSize(1, h))
		return r
	}

	// discover
	body.Add(u.sectionLabel("discover"))
	scanP := newTintPill("SCAN NETWORK", colAccent, u.scan)
	scanP.size = 13
	body.Add(container.NewCenter(scanP))
	body.Add(gap(16))

	// saved devices: rename inline, two-tap delete
	body.Add(u.sectionLabel("saved devices"))
	for _, d := range cfg.SavedDevices {
		d := d
		row := container.NewStack(deviceTitle(d.Name, d.IP+" · "+d.Mac))
		del := confirmPill(func() {
			u.eng.DeleteDevice(d.Mac)
			if u.sel == strings.ToLower(d.Mac) {
				u.sel = ""
			}
			u.manageRebuild()
		})
		rename := newPill("RENAME", nil)
		rename.onTapped = func() {
			entry := widget.NewEntry()
			entry.SetText(d.Name)
			save := newTintPill("SAVE", colOK, func() {
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
		body.Add(glassCard(container.NewBorder(nil, nil, nil, container.NewHBox(rename, del), row)))
	}
	body.Add(gap(16))

	// groups
	body.Add(u.sectionLabel("groups"))
	for _, g := range cfg.Groups {
		g := g
		del := confirmPill(func() {
			u.eng.DeleteGroup(g.Name)
			if u.sel == "g:"+g.Name {
				u.sel = ""
			}
			u.manageRebuild()
		})
		body.Add(glassCard(container.NewBorder(nil, nil, nil, del,
			deviceTitle("◇ "+g.Name, fmt.Sprintf("%d devices", len(g.Macs))))))
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
	create := newTintPill("CREATE GROUP", colOK, func() {
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
	body.Add(glassCard(container.NewVBox(
		nameEntry,
		container.NewHScroll(memberRow),
		container.NewBorder(nil, nil, nil, create),
	)))
	body.Add(gap(16))

	// themes — desktop palettes, shared store names
	body.Add(u.sectionLabel("themes"))
	cur := paletteFor(cfg.Theme)
	tgrid := container.NewGridWithColumns(2)
	for _, p := range palettes {
		p := p
		tp := newTintPill(p.Label, p.Accent, func() {
			u.eng.SetTheme(p.Store)
			applyPalette(p)
			u.app.Settings().SetTheme(newLuminaTheme()) // force widget re-theme
			u.setChrome()
			u.showManage(false)
		})
		tp.setOn(cur.Key == p.Key)
		tgrid.Add(container.NewPadded(tp))
	}
	body.Add(tgrid)
	body.Refresh()
}

func (u *ui) setStatus(s string, c color.NRGBA) {
	u.status.Text = s
	u.status.Color = c
	u.status.Refresh()
}
