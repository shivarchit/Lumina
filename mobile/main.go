// Lumina mobile — Aura: the screen is the bulb. A glow field renders the
// light's real color and brightness; every control lives in a bottom sheet;
// chrome is text, not buttons.
package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fynemobile "fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"Lumina/core"

	"github.com/shivarchit/Lumina-TUI/pkg/config"
	"github.com/shivarchit/Lumina-TUI/pkg/wiz"
)

// Scene names/IDs/colors match desktop (frontend/src/main.js SCENES).
var scenes = []struct {
	Name string
	ID   int
	Tint color.NRGBA
}{
	{"OCEAN", 1, hex(0x22D3EE)}, {"ROMANCE", 2, hex(0xF472B6)},
	{"SUNSET", 3, hex(0xFB923C)}, {"PARTY", 4, hex(0xE879F9)},
	{"FIREPLACE", 5, hex(0xF97316)}, {"COZY", 6, hex(0xFDBA74)},
	{"FOREST", 7, hex(0x4ADE80)}, {"PASTEL", 8, hex(0xF9A8D4)},
	{"WAKE-UP", 9, hex(0xFDE68A)}, {"BEDTIME", 10, hex(0x818CF8)},
	{"DAYLIGHT", 12, hex(0xFCD34D)}, {"FOCUS", 15, hex(0x7DD3FC)},
}

// Preset hexes match desktop (frontend/src/main.js PRESET_HEXES).
var presetColors = []color.NRGBA{
	hex(0xFFD9A0), hex(0xCBA6F7), hex(0x89B4FA), hex(0xA6E3A1),
	hex(0xF38BA8), hex(0xFFD700), hex(0xFF8C00), hex(0x00FFFF),
}

type ui struct {
	app fyne.App
	eng *core.Core
	win fyne.Window

	sel        string // device MAC or "g:"+group name
	tab        string // light | scenes | timer | themes
	huePick    bool   // light tab: full-spectrum slider open
	bright     float64
	power      bool
	lastSynced string // target already state-synced; avoids UDP per tab switch

	aura          *aura
	dim           *canvas.Rectangle // full-window scrim behind manage
	status        *canvas.Text
	content       *fyne.Container
	numTxt        *canvas.Text
	descTxt       *canvas.Text
	pwWord        *word
	manageRebuild func()
	timerTick     func() // timer tab's countdown render, driven by hero ticker
	viewGen       int

	timerEnd time.Time
	timerT   *time.Timer
}

func main() {
	a := app.NewWithID("com.shivarchit.lumina")
	// Android has no usable $HOME; point the shared config at app storage.
	// ponytail: process-global env mutation to steer the TUI module's
	// os.UserHomeDir — replace with a config.SetDir API in Lumina-TUI when
	// one ships.
	if runtime.GOOS == "android" {
		if root := a.Storage().RootURI().Path(); root != "" {
			os.Setenv("HOME", root)
		}
	}
	u := &ui{app: a, eng: core.New(), tab: "light", bright: 80, power: true, win: a.NewWindow("Lumina")}
	cfg := u.eng.GetConfig()
	applyPalette(paletteFor(cfg.Theme))
	a.Settings().SetTheme(newLuminaTheme())
	if cfg.LastBrightness > 0 {
		u.bright = float64(cfg.LastBrightness)
	}
	u.status = monoText("", 9, colDim)
	u.content = container.NewStack()
	u.aura = newAura()

	// scrim sits in the window stack so it also covers the translucent
	// system-bar strip — a panel inside the inset content can't reach it;
	// the ground itself is the theme background (colBGDeep), no extra rect
	u.dim = canvas.NewRectangle(scrimCol())
	u.dim.Hide()
	u.win.SetPadded(false) // edge-to-edge: no frame around the app
	u.win.SetContent(container.NewStack(u.aura.layer, u.dim, u.content))
	// Android back inside manage returns home instead of minimizing the app;
	// the scrim is visible exactly while manage is open
	u.win.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if k.Name == fynemobile.KeyBack && u.dim.Visible() {
			u.showHome()
		}
	})
	u.refreshAura()
	u.showHome()
	u.win.ShowAndRun()
}

// temp is the persisted white temperature (single source: config).
func (u *ui) temp() int {
	if t := u.eng.GetConfig().LastColorTemp; t > 0 {
		return t
	}
	return 3200
}

// lightRGB is the current light output color for the aura.
func (u *ui) lightRGB() color.NRGBA {
	cfg := u.eng.GetConfig()
	if c, ok := lightColor(cfg.LastColor, cfg.LastColorTemp); ok {
		return c
	}
	return kelvinColor(3200)
}

func (u *ui) refreshAura() {
	u.aura.set(u.lightRGB(), u.bright/100, u.power)
}

// nrgbaHex formats a color the way config.LastColor stores it.
func nrgbaHex(c color.NRGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// paint is the one "make the room this color" ritual: preview the aura,
// send, persist (off the UI thread), update the caption.
func (u *ui) paint(c color.NRGBA, action string, params map[string]interface{}) {
	u.power = true
	u.showPower()
	u.aura.set(c, u.bright/100, true)
	u.sendPilot(action, params)
	go u.eng.SetLastState(nrgbaHex(c), 0, 0)
	u.refreshDesc()
}

func (u *ui) refreshDesc() {
	if u.descTxt != nil {
		u.descTxt.Text = u.describeLight()
		u.descTxt.Refresh()
	}
}

func (u *ui) showPower() {
	if u.pwWord == nil {
		return
	}
	if u.power {
		u.pwWord.set("ON", colOK)
	} else {
		u.pwWord.set("OFF", colErr)
	}
}

// lightColor resolves a hex string (preferred) or kelvin temp to a color.
func lightColor(hexStr string, temp int) (color.NRGBA, bool) {
	if r, g, b, err := wiz.HexToRGB(strings.TrimSpace(hexStr)); err == nil {
		return color.NRGBA{R: r, G: g, B: b, A: 0xFF}, true
	}
	if temp > 0 {
		return kelvinColor(float64(temp)), true
	}
	return color.NRGBA{}, false
}

// ── target resolution ───────────────────────────────────────────────

func (u *ui) targets() []core.Target {
	if strings.HasPrefix(u.sel, "g:") {
		return u.eng.GroupTargets(strings.TrimPrefix(u.sel, "g:"))
	}
	return u.eng.DeviceTargets(u.sel)
}

func (u *ui) targetLabel() (name, sub string) {
	if strings.HasPrefix(u.sel, "g:") {
		n := strings.TrimPrefix(u.sel, "g:")
		return n, fmt.Sprintf("group · %d devices", len(u.eng.GroupTargets(n)))
	}
	if ts := u.eng.DeviceTargets(u.sel); len(ts) == 1 {
		return ts[0].Name, "wiz · " + ts[0].IP
	}
	return "no device", ""
}

// selectable returns every switchable target key in order.
func (u *ui) selectable() []string {
	cfg := u.eng.GetConfig()
	var keys []string
	for _, d := range cfg.SavedDevices {
		keys = append(keys, core.NormMac(d.Mac))
	}
	for _, g := range cfg.Groups {
		keys = append(keys, "g:"+g.Name)
	}
	return keys
}

// ── sends ───────────────────────────────────────────────────────────

func (u *ui) report(action string, res core.FanoutResult) {
	msg, col := fmt.Sprintf("%s · %d ok · %dms", action, res.OK, res.Ms), colOK
	if len(res.Failed) > 0 {
		msg, col = fmt.Sprintf("%s · %d ok · failed: %s", action, res.OK, strings.Join(res.Failed, ", ")), colErr
	}
	fyne.Do(func() { u.setStatus(msg, col) })
}

func (u *ui) sendPilot(action string, params map[string]interface{}) {
	ts := u.targets()
	// closure keeps the UDP fanout off the UI thread — `go f(expensive())`
	// would evaluate the send synchronously here
	go func() { u.report(action, u.eng.SetPilot(ts, params)) }()
}

func (u *ui) setStatus(s string, c color.NRGBA) {
	u.status.Text = s
	u.status.Color = c
	u.status.Refresh()
}

// kelvinColor approximates black-body light color (Tanner Helland fit).
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

// ── home shell ──────────────────────────────────────────────────────

func (u *ui) showHome() {
	u.viewGen++
	u.dim.Hide()
	u.dim.Refresh()
	u.manageRebuild = nil // drop the manage widget tree
	u.timerTick = nil     // rebound below if the timer tab is open
	keys := u.selectable()
	if u.sel == "" && len(keys) > 0 {
		u.sel = keys[0]
	}

	// empty state: no devices — straight into discover
	if len(u.eng.GetConfig().SavedDevices) == 0 {
		msg := monoText("NO DEVICES ADDED", 11, colDim)
		msg.Alignment = fyne.TextAlignCenter
		go2 := newWord("✚ DISCOVER DEVICES", 13, colAccent, func() { u.showManage(true) })
		u.content.Objects = []fyne.CanvasObject{container.NewCenter(
			container.NewVBox(msg, gap(14), container.NewCenter(go2)),
		)}
		u.content.Refresh()
		return
	}

	name, sub := u.targetLabel()
	title := canvas.NewText(name, colText)
	title.TextSize = 20
	title.Alignment = fyne.TextAlignCenter
	subT := monoText(strings.ToUpper(sub), 9, colDim)
	subT.Alignment = fyne.TextAlignCenter

	// device dots: only when there is something to switch between
	dots := container.NewHBox(layout.NewSpacer())
	if len(keys) > 1 {
		for _, k := range keys {
			d := canvas.NewCircle(colFaint)
			if k == u.sel {
				d.FillColor = colAccent
			}
			dots.Add(container.NewGridWrap(fyne.NewSize(6, 6), d))
		}
	}
	dots.Add(layout.NewSpacer())
	cycle := newWord("", 11, colTransparent, func() {
		for i, k := range keys {
			if k == u.sel {
				u.sel = keys[(i+1)%len(keys)]
				break
			}
		}
		u.showHome() // sel changed → showHome re-syncs
	})

	head := container.NewVBox(gap(36), container.NewStack(container.NewVBox(title, subT), cycle), gap(8), dots)

	// center: numeral floats in the glow, drag anywhere = brightness
	u.numTxt = monoText(strconv.Itoa(int(u.bright)), 64, colText)
	u.numTxt.Alignment = fyne.TextAlignCenter
	u.descTxt = monoText(u.describeLight(), 9, colDim)
	u.descTxt.Alignment = fyne.TextAlignCenter
	u.pwWord = newWord("ON", 12, colOK, u.togglePower)
	u.showPower()
	pct := monoText("%", 16, colDim)
	numRow := container.NewHBox(layout.NewSpacer(), u.numTxt, pct, layout.NewSpacer())

	field := newDragField(u.bright,
		func(v float64) {
			u.bright = v
			u.numTxt.Text = strconv.Itoa(int(v))
			u.numTxt.Refresh()
			// preview with the aura's cached color — no config read per drag event
			u.aura.set(u.aura.col, v/100, u.power)
		},
		func(v int) {
			u.sendPilot(fmt.Sprintf("brightness %d%%", v), map[string]interface{}{"dimming": v, "state": true})
			go u.eng.SetLastState("", v, 0)
			u.power = true
			u.showPower()
			u.refreshAura()
		},
	)
	hint := monoText("SLIDE ↑ ↓ TO DIM", 8, withAlpha(colText, 0x4D))
	hint.Alignment = fyne.TextAlignCenter

	// active sleep timer surfaces on the hero, whatever tab is open; this is
	// the view's only countdown ticker — the timer tab hooks in via timerTick
	timerTxt := monoText("", 9, colDim)
	timerTxt.Alignment = fyne.TextAlignCenter
	renderTimer := func() {
		timerTxt.Text = ""
		if s := u.countdownStr(); s != "" {
			timerTxt.Text = "SLEEP · " + s
		}
		timerTxt.Refresh()
		if u.timerTick != nil {
			u.timerTick()
		}
	}
	renderTimer()
	gen := u.viewGen
	go func() {
		tk := time.NewTicker(time.Second)
		defer tk.Stop()
		last := ""
		for range tk.C {
			if u.viewGen != gen {
				return
			}
			s := u.countdownStr()
			if s == last {
				continue // idle: zero UI work; one last pass clears cancel/fire
			}
			last = s
			fyne.Do(renderTimer)
		}
	}()

	center := container.NewStack(field,
		container.NewCenter(container.NewVBox(numRow, u.descTxt, gap(4), hint, gap(8),
			container.NewCenter(u.pwWord), gap(2), timerTxt)))

	u.content.Objects = []fyne.CanvasObject{container.NewBorder(head, u.sheet(), nil, nil, center)}
	u.content.Refresh()
	// UDP state fetch only when the target actually changed, not per tab switch
	if u.lastSynced != u.sel {
		u.lastSynced = u.sel
		u.syncState()
	}
}

func (u *ui) describeLight() string {
	cfg := u.eng.GetConfig()
	if cfg.LastColor != "" {
		return strings.ToUpper(cfg.LastColor)
	}
	k := u.temp()
	warm := "WARM WHITE"
	switch {
	case k < 3000:
		warm = "CANDLELIGHT"
	case k >= 5200:
		warm = "DAYLIGHT"
	case k >= 4000:
		warm = "NEUTRAL"
	}
	return fmt.Sprintf("%dK · %s", k, warm)
}

func (u *ui) togglePower() {
	u.power = !u.power
	u.showPower()
	u.refreshAura()
	ts := u.targets()
	on := u.power
	lab := "off"
	if on {
		lab = "on"
	}
	go func() { u.report(lab, u.eng.SetPower(ts, on)) }()
}

// syncState pulls live device state into numeral + aura (single targets).
func (u *ui) syncState() {
	ts := u.targets()
	if len(ts) != 1 {
		return
	}
	gen := u.viewGen
	go func() {
		st := u.eng.GetState(ts[0].IP, ts[0].Port)
		if st.Err != "" {
			return
		}
		fyne.Do(func() {
			if u.viewGen != gen {
				return
			}
			if st.Brightness > 0 {
				u.bright = float64(st.Brightness)
				u.numTxt.Text = strconv.Itoa(st.Brightness)
				u.numTxt.Refresh()
			}
			u.power = st.Power
			u.showPower()
			if c, ok := lightColor(st.ColorHex, st.Temp); ok {
				u.aura.set(c, u.bright/100, u.power)
			} else {
				u.refreshAura()
			}
		})
	}()
}

// ── the sheet ───────────────────────────────────────────────────────

// sheet wraps the active tab's content in the rounded bottom surface.
func (u *ui) sheet() fyne.CanvasObject {
	var body fyne.CanvasObject
	switch u.tab {
	case "scenes":
		body = u.scenesSheet()
	case "timer":
		body = u.timerSheet()
	case "themes":
		body = u.themesSheet()
	default:
		body = u.lightSheet()
	}

	nav := container.NewHBox(layout.NewSpacer())
	for _, t := range []string{"light", "scenes", "timer", "themes", "manage"} {
		t := t
		c := colDim
		if u.tab == t {
			c = colAccent
		}
		nav.Add(newWord(strings.ToUpper(t), 11, c, func() {
			if t == "manage" {
				u.showManage(false)
				return
			}
			u.tab = t
			u.showHome()
		}))
	}
	nav.Add(layout.NewSpacer())

	inner := container.NewVBox(
		gap(14),
		body,
		nav,
		container.NewCenter(u.status),
		gap(14),
	)
	// the rect extends past the bottom so only its top corners show rounded
	return container.NewStack(sheetBG(0xCC), container.NewPadded(inner))
}

// sheetBG is the rounded sheet surface; negative bottom padding pushes the
// rect past the screen edge so only its top corners read rounded.
func sheetBG(alpha uint8) fyne.CanvasObject {
	r := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x08, B: 0x0C, A: alpha})
	r.CornerRadius = 24
	r.StrokeColor = colFaint
	r.StrokeWidth = 1
	return container.New(layout.NewCustomPaddedLayout(0, -30, 0, 0), r)
}

func sheetLabel(s string) fyne.CanvasObject {
	return container.NewPadded(monoText(strings.ToUpper(s), 8, colDim))
}

// light: temp bar + color dots
func (u *ui) lightSheet() fyne.CanvasObject {
	temp := newGSlider(2200, 6500, float64(u.temp()), kelvinColor,
		func(v float64) {
			u.aura.set(kelvinColor(v), u.bright/100, true)
		},
		func(v float64) {
			u.power = true
			u.showPower()
			u.sendPilot(fmt.Sprintf("%dK", int(v)), map[string]interface{}{"temp": int(v), "state": true})
			go func() { // persist off the UI thread: two JSON writes
				u.eng.ClearLastColor() // white mode: temp, not old RGB, restores
				u.eng.SetLastState("", 0, int(v))
				fyne.Do(u.refreshDesc)
			}()
		})

	dotRow := container.NewHBox(layout.NewSpacer())
	dot := func(obj fyne.CanvasObject, tapped func()) {
		tap := newWord(" ", 11, colTransparent, tapped)
		dotRow.Add(container.NewStack(container.NewCenter(container.NewGridWrap(fyne.NewSize(17, 17), obj)), tap))
		dotRow.Add(layout.NewSpacer())
	}
	for _, c := range presetColors {
		c := c
		dot(canvas.NewCircle(c), func() { u.paintRGB(c) })
	}
	// last dot: hue wheel — toggles the full-spectrum slider
	wheel := canvas.NewImageFromImage(hueDot)
	wheel.ScaleMode = canvas.ImageScaleFastest
	dot(wheel, func() {
		u.huePick = !u.huePick
		u.showHome()
	})

	box := container.NewVBox(temp, gap(4), dotRow, gap(4))
	if u.huePick {
		hue := newGSlider(0, 360, 180, hueColor,
			func(v float64) { u.aura.set(hueColor(v), u.bright/100, true) },
			func(v float64) { u.paintRGB(hueColor(v)) })
		box.Add(hue)
		box.Add(gap(4))
	}
	return box
}

// paintRGB sends a solid color, shared by preset dots and the hue slider.
func (u *ui) paintRGB(c color.NRGBA) {
	u.paint(c, strings.ToUpper(nrgbaHex(c)), map[string]interface{}{
		"r": int(c.R), "g": int(c.G), "b": int(c.B), "state": true,
	})
}

// hueColor maps 0..360 to the full-saturation RGB spectrum.
func hueColor(h float64) color.NRGBA {
	h = math.Mod(h, 360) / 60
	f := h - math.Floor(h)
	var r, g, b float64
	switch int(h) {
	case 0:
		r, g, b = 1, f, 0
	case 1:
		r, g, b = 1-f, 1, 0
	case 2:
		r, g, b = 0, 1, f
	case 3:
		r, g, b = 0, 1-f, 1
	case 4:
		r, g, b = f, 0, 1
	default:
		r, g, b = 1, 0, f
	}
	return color.NRGBA{R: uint8(r * 255), G: uint8(g * 255), B: uint8(b * 255), A: 0xFF}
}

// hueDot is the little color wheel that opens the hue slider, baked once.
var hueDot = func() image.Image {
	const n, r = 64, 30.0
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx, dy := float64(x)-n/2, float64(y)-n/2
			if math.Hypot(dx, dy) > r {
				continue
			}
			img.SetNRGBA(x, y, hueColor(math.Atan2(dy, dx)*180/math.Pi+180))
		}
	}
	return img
}()

// scenes: tinted words, tap = the room becomes it
func (u *ui) scenesSheet() fyne.CanvasObject {
	grid := container.NewGridWithColumns(2)
	for _, sc := range scenes {
		sc := sc
		// ponytail: persists the scene tint as LastColor so the aura restores;
		// desktop will read it as a plain RGB — add LastSceneID in core if that
		// ever matters.
		w := newWord(sc.Name, 12, sc.Tint, func() {
			u.paint(sc.Tint, strings.ToLower(sc.Name), map[string]interface{}{"sceneId": sc.ID, "state": true})
		})
		grid.Add(w)
	}
	return container.NewVBox(sheetLabel("scenes"), grid, gap(4))
}

// countdownStr is the live timer as M:SS, or "" when no timer is armed.
func (u *ui) countdownStr() string {
	if u.timerEnd.IsZero() {
		return ""
	}
	left := time.Until(u.timerEnd)
	if left < 0 {
		left = 0
	}
	return fmt.Sprintf("%d:%02d", int(left.Minutes()), int(left.Seconds())%60)
}

// timer: countdown + presets + manual entry
func (u *ui) timerSheet() fyne.CanvasObject {
	disp := monoText("–", 34, colText)
	disp.Alignment = fyne.TextAlignCenter
	sub := monoText("SLEEP TIMER · TURNS TARGET OFF", 8, colDim)
	sub.Alignment = fyne.TextAlignCenter

	render := func() {
		disp.Text = "–"
		if s := u.countdownStr(); s != "" {
			disp.Text = s
		}
		disp.Refresh()
	}
	render()
	u.timerTick = render // hero's per-second ticker drives this display too

	start := func(mins int) {
		if u.timerT != nil {
			u.timerT.Stop()
		}
		ts := u.targets()
		dur := time.Duration(mins) * time.Minute
		u.timerEnd = time.Now().Add(dur)
		u.timerT = time.AfterFunc(dur, func() {
			u.report("sleep off", u.eng.SetPower(ts, false))
			fyne.Do(func() { // hero must reflect the off state, not just the bulb
				u.timerEnd = time.Time{}
				u.power = false
				u.showPower()
				u.refreshAura()
			})
		})
		render() // countdown itself is the feedback — no status line
	}

	presets := container.NewHBox(layout.NewSpacer())
	for _, m := range []int{15, 30, 60, 120} {
		m := m
		presets.Add(newWord(strconv.Itoa(m), 11, colDim, func() { start(m) }))
	}
	presets.Add(layout.NewSpacer())

	mins := widget.NewEntry()
	mins.SetPlaceHolder("MINUTES")
	startW := newWord("START", 11, colOK, func() {
		m, err := strconv.Atoi(strings.TrimSpace(mins.Text))
		if err != nil || m < 1 || m > 720 {
			u.setStatus("minutes must be 1–720", colErr)
			return
		}
		start(m)
	})
	cancel := newWord("CANCEL", 11, colErr, func() {
		if u.timerT != nil {
			u.timerT.Stop()
		}
		u.timerEnd = time.Time{}
		render()
	})
	manual := container.NewHBox(layout.NewSpacer(),
		container.NewGridWrap(fyne.NewSize(110, 40), mins), startW, cancel, layout.NewSpacer())

	// ponytail: in-app timer only — dies if the app quits, same as desktop.
	return container.NewVBox(disp, sub, gap(8), presets, manual, gap(2))
}

// more: themes + manage entry
func (u *ui) themesSheet() fyne.CanvasObject {
	grid := container.NewGridWithColumns(2)
	cur := paletteFor(u.eng.GetConfig().Theme)
	for _, p := range palettes {
		p := p
		c := p.Accent
		if cur.Store == p.Store {
			c = colText // active theme reads white, others in their accent
		}
		grid.Add(newWord(p.Label, 12, c, func() {
			go u.eng.SetTheme(p.Store)
			applyPalette(p)
			u.app.Settings().SetTheme(newLuminaTheme())
			// repaint the chrome that captured the old palette
			u.dim.FillColor = scrimCol()
			u.aura.refade()
			u.showHome()
		}))
	}
	return container.NewVBox(sheetLabel("theme"), grid, gap(4))
}

// ── manage: full-height sheet ───────────────────────────────────────

func (u *ui) showManage(autoScan bool) {
	u.viewGen++
	title := canvas.NewText("Manage", colText)
	title.TextSize = 20
	head := container.NewPadded(container.NewBorder(nil, nil, title,
		newWord("✕ CLOSE", 11, colAccent, func() { u.showHome() })))

	body := container.NewVBox()
	composer := container.NewVBox()
	u.manageRebuild = func() {
		u.buildManageBody(body)
		u.buildComposer(composer)
	}
	u.manageRebuild()

	// window-layer scrim darkens everything (aura included) behind manage
	u.dim.Show()
	u.dim.Refresh()
	u.content.Objects = []fyne.CanvasObject{
		container.NewPadded(container.NewBorder(
			container.NewVBox(gap(24), head),
			container.NewVBox(composer, gap(10)),
			nil, nil,
			container.NewVScroll(container.NewPadded(body)),
		)),
	}
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
			saved[core.NormMac(d.Mac)] = true
		}
		fresh := 0
		for _, d := range found {
			if !saved[core.NormMac(d.Mac)] {
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

// hairRow is a manage list row with a hairline underneath.
func hairRow(left fyne.CanvasObject, actions ...fyne.CanvasObject) fyne.CanvasObject {
	line := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x0D})
	line.SetMinSize(fyne.NewSize(1, 1))
	return container.NewVBox(
		container.NewBorder(nil, nil, nil, container.NewHBox(actions...), left),
		line,
	)
}

func rowTitle(name, meta string) fyne.CanvasObject {
	n := canvas.NewText(name, colText)
	n.TextSize = 15
	return container.NewVBox(n, monoText(meta, 8, colDim))
}

// confirmWord turns DELETE into a two-tap confirm.
func confirmWord(onConfirm func()) *word {
	var del *word
	del = newWord("DELETE", 9, colErr, func() {
		if del.text != "SURE?" {
			del.set("SURE?", colErr)
			return
		}
		onConfirm()
	})
	return del
}

func (u *ui) buildManageBody(body *fyne.Container) {
	cfg := u.eng.GetConfig()
	body.RemoveAll()

	body.Add(sheetLabel("discover"))
	scanW := newWord("SCAN NETWORK", 12, colAccent, u.scan)
	// status lives right under the word that triggers it
	body.Add(container.NewVBox(container.NewCenter(scanW), container.NewCenter(u.status)))
	body.Add(gap(16))

	body.Add(sheetLabel("devices"))
	for _, d := range cfg.SavedDevices {
		d := d
		row := container.NewStack(rowTitle(d.Name, d.IP+" · "+d.Mac))
		del := confirmWord(func() {
			u.eng.DeleteDevice(d.Mac)
			if u.sel == core.NormMac(d.Mac) {
				u.sel = ""
			}
			u.manageRebuild()
		})
		rename := newWord("RENAME", 9, colDim, nil)
		rename.onTapped = func() {
			entry := widget.NewEntry()
			entry.SetText(d.Name)
			save := newWord("SAVE", 11, colOK, func() {
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
		body.Add(hairRow(row, rename, del))
	}
	body.Add(gap(16))

	body.Add(sheetLabel("groups"))
	for _, g := range cfg.Groups {
		g := g
		del := confirmWord(func() {
			u.eng.DeleteGroup(g.Name)
			if u.sel == "g:"+g.Name {
				u.sel = ""
			}
			u.manageRebuild()
		})
		body.Add(hairRow(rowTitle("◇ "+g.Name, fmt.Sprintf("%d devices", len(g.Macs))), del))
	}
	body.Refresh()
}

// buildComposer is the new-group builder, pinned to the screen bottom.
func (u *ui) buildComposer(box *fyne.Container) {
	cfg := u.eng.GetConfig()
	box.RemoveAll()

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("NEW GROUP NAME")
	members := map[string]bool{}
	memberRow := container.NewHBox()
	for _, d := range cfg.SavedDevices {
		mac := core.NormMac(d.Mac)
		name := strings.ToUpper(d.Name)
		var w *word
		w = newWord(name, 10, colDim, func() {
			members[mac] = !members[mac]
			if members[mac] {
				w.set("✓ "+name, colAccent)
			} else {
				w.set(name, colDim)
			}
		})
		memberRow.Add(w)
	}
	create := newWord("CREATE", 11, colOK, func() {
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

	box.Add(sheetLabel("new group · tap members to pick"))
	box.Add(container.NewHScroll(memberRow))
	box.Add(container.NewBorder(nil, nil, nil, container.NewPadded(create), nameEntry))
	box.Refresh()
}
