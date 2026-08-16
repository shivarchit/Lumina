package main

import (
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// glassCard wraps content in the app's rounded glass surface.
func glassCard(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x06})
	bg.StrokeColor = colFaint
	bg.StrokeWidth = 1
	bg.CornerRadius = 18
	return container.NewStack(bg, container.NewPadded(container.NewPadded(content)))
}

// monoText is a mono canvas label in the app's type voice.
func monoText(s string, size float32, c color.Color) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

// ── pill: the app's mono rounded-border button ──────────────────────

type pill struct {
	widget.BaseWidget
	text     string
	on       bool
	accent   color.NRGBA // border/text when on
	tint     color.NRGBA // semantic color shown even when off (zero = none)
	size     float32
	onTapped func()

	bg  *canvas.Rectangle
	lbl *canvas.Text
}

func newPill(text string, tapped func()) *pill {
	p := &pill{text: text, accent: colAccent, size: 12, onTapped: tapped}
	p.ExtendBaseWidget(p)
	return p
}

// newTintPill is a pill whose semantic color is always visible (delete=red,
// save=green, close=amber) so intent reads before any interaction.
func newTintPill(text string, tint color.NRGBA, tapped func()) *pill {
	p := newPill(text, tapped)
	p.tint = tint
	p.accent = tint
	return p
}

func (p *pill) setOn(on bool) {
	p.on = on
	p.Refresh()
}

func (p *pill) Tapped(*fyne.PointEvent) {
	if p.onTapped != nil {
		p.onTapped()
	}
}

func (p *pill) CreateRenderer() fyne.WidgetRenderer {
	p.bg = canvas.NewRectangle(color.Transparent)
	p.bg.StrokeWidth = 1
	p.lbl = canvas.NewText(p.text, colDim)
	p.lbl.TextStyle = fyne.TextStyle{Monospace: true}
	p.lbl.TextSize = p.size
	return &pillRenderer{p: p}
}

type pillRenderer struct{ p *pill }

func (r *pillRenderer) MinSize() fyne.Size {
	ts := fyne.MeasureText(r.p.text, r.p.size, fyne.TextStyle{Monospace: true})
	return fyne.NewSize(ts.Width+30, ts.Height+18)
}

func (r *pillRenderer) Layout(s fyne.Size) {
	r.p.bg.Resize(s)
	r.p.bg.CornerRadius = s.Height / 2
	ts := fyne.MeasureText(r.p.text, r.p.size, fyne.TextStyle{Monospace: true})
	r.p.lbl.Move(fyne.NewPos((s.Width-ts.Width)/2, (s.Height-ts.Height)/2))
	r.p.lbl.Resize(ts)
}

func (r *pillRenderer) Refresh() {
	r.p.lbl.Text = r.p.text
	switch {
	case r.p.on:
		r.p.bg.StrokeColor = r.p.accent
		r.p.bg.FillColor = withAlpha(r.p.accent, 0x1A)
		r.p.lbl.Color = r.p.accent
	case r.p.tint.A > 0:
		r.p.bg.StrokeColor = withAlpha(r.p.tint, 0x66)
		r.p.bg.FillColor = withAlpha(r.p.tint, 0x0D)
		r.p.lbl.Color = r.p.tint
	default:
		r.p.bg.StrokeColor = colFaint
		r.p.bg.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x05}
		r.p.lbl.Color = colDim
	}
	r.p.bg.Refresh()
	r.p.lbl.Refresh()
}

func (r *pillRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.p.bg, r.p.lbl} }
func (r *pillRenderer) Destroy()                     {}

// ── dial: draggable brightness arc, tap center for power ────────────
//
// Same geometry as desktop: 300° sweep, 60° gap at the bottom,
// 0% at bottom-left rising clockwise to 100% at bottom-right.

const (
	dialGapFrom = 150.0 // degrees, 0 = top, clockwise
	dialGapTo   = 210.0
	dialSweep   = 300.0
)

type dial struct {
	widget.BaseWidget
	value   float64 // 10..100
	power   bool
	onEnd   func(v int) // fired on drag end
	onPower func(on bool)

	raster *canvas.Raster
	valTxt *canvas.Text
	pctTxt *canvas.Text
	labTxt *canvas.Text
	pwrTxt *canvas.Text
}

func newDial(onEnd func(int), onPower func(bool)) *dial {
	d := &dial{value: 80, power: true, onEnd: onEnd, onPower: onPower}
	d.ExtendBaseWidget(d)
	return d
}

func (d *dial) set(v float64, power bool) {
	d.value = math.Max(10, math.Min(100, v))
	d.power = power
	d.Refresh()
}

func (d *dial) angleToPct(dx, dy float64) (float64, bool) {
	a := math.Atan2(dx, -dy) * 180 / math.Pi
	if a < 0 {
		a += 360
	}
	if a > dialGapFrom && a < dialGapTo {
		return 0, false // dead zone at the bottom gap
	}
	pos := a - dialGapTo
	if pos < 0 {
		pos += 360
	}
	return math.Max(10, math.Min(100, pos/dialSweep*100)), true
}

func (d *dial) Dragged(e *fyne.DragEvent) {
	s := d.Size()
	if v, ok := d.angleToPct(float64(e.Position.X-s.Width/2), float64(e.Position.Y-s.Height/2)); ok {
		d.value = v
		d.Refresh()
	}
}

func (d *dial) DragEnd() {
	if d.onEnd != nil {
		d.onEnd(int(math.Round(d.value)))
	}
}

func (d *dial) Tapped(e *fyne.PointEvent) {
	s := d.Size()
	dx, dy := float64(e.Position.X-s.Width/2), float64(e.Position.Y-s.Height/2)
	rIn := float64(fyne.Min(s.Width, s.Height))/2 * 0.62
	if math.Hypot(dx, dy) < rIn {
		d.power = !d.power
		d.Refresh()
		if d.onPower != nil {
			d.onPower(d.power)
		}
	}
}

func (d *dial) MinSize() fyne.Size { return fyne.NewSize(240, 240) }

func (d *dial) CreateRenderer() fyne.WidgetRenderer {
	d.raster = canvas.NewRaster(d.drawRing)
	d.valTxt = canvas.NewText("80", colText)
	d.valTxt.TextSize = 44
	d.valTxt.TextStyle = fyne.TextStyle{Monospace: true}
	d.pctTxt = canvas.NewText("%", colDim)
	d.pctTxt.TextSize = 17
	d.pctTxt.TextStyle = fyne.TextStyle{Monospace: true}
	d.labTxt = canvas.NewText("B R I G H T N E S S", colDim)
	d.labTxt.TextSize = 9
	d.labTxt.TextStyle = fyne.TextStyle{Monospace: true}
	d.pwrTxt = canvas.NewText("ON", colOK)
	d.pwrTxt.TextSize = 11
	d.pwrTxt.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	return &dialRenderer{d: d}
}

// drawRing renders track + value arc with rounded end caps and a soft
// feathered edge (no more flat-chopped ends).
func (d *dial) drawRing(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	cx, cy := float64(w)/2, float64(h)/2
	rOut := math.Min(cx, cy) - 2
	thick := rOut * 0.19
	rMid := rOut - thick/2
	track := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x12}
	fill := colAccent
	if !d.power {
		fill = withAlpha(colAccent, 0x40)
	}
	filled := d.value / 100 * dialSweep

	// cap centers sit on the ring's midline at a given clock angle
	capAt := func(deg float64) (float64, float64) {
		rad := deg * math.Pi / 180
		return cx + rMid*math.Sin(rad), cy - rMid*math.Cos(rad)
	}
	sx, sy := capAt(dialGapTo)            // sweep start (bottom-left)
	ex, ey := capAt(dialGapFrom)          // track end (bottom-right)
	vx, vy := capAt(dialGapTo + filled - 360*math.Floor((dialGapTo+filled)/360)) // value end

	capR := thick / 2
	feather := 1.5
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			fx, fy := float64(x), float64(y)
			dx, dy := fx-cx, fy-cy

			// signed coverage of the band and each cap; the max wins
			band := -1e9
			var c color.NRGBA
			if b := capR - math.Abs(math.Hypot(dx, dy)-rMid); b > -feather {
				a := math.Atan2(dx, -dy) * 180 / math.Pi
				if a < 0 {
					a += 360
				}
				if !(a > dialGapFrom && a < dialGapTo) {
					pos := a - dialGapTo
					if pos < 0 {
						pos += 360
					}
					band = b
					c = track
					if pos <= filled {
						c = fill
					}
				}
			}
			for _, cap := range [][3]float64{{sx, sy, 1}, {vx, vy, 1}, {ex, ey, 0}} {
				if b := capR - math.Hypot(fx-cap[0], fy-cap[1]); b > band {
					band = b
					if cap[2] == 1 {
						c = fill
					} else {
						c = track
					}
				}
			}
			if band <= -feather {
				continue
			}
			edge := math.Max(0, math.Min(1, band/feather+1))
			c.A = uint8(float64(c.A) * edge)
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

type dialRenderer struct{ d *dial }

func (r *dialRenderer) MinSize() fyne.Size { return fyne.NewSize(240, 240) }

func (r *dialRenderer) Layout(s fyne.Size) {
	d := r.d
	d.raster.Resize(s)
	vs := fyne.MeasureText(d.valTxt.Text, d.valTxt.TextSize, d.valTxt.TextStyle)
	ps := fyne.MeasureText(d.pctTxt.Text, d.pctTxt.TextSize, d.pctTxt.TextStyle)
	total := vs.Width + ps.Width
	cy := s.Height / 2
	d.valTxt.Move(fyne.NewPos((s.Width-total)/2, cy-vs.Height/2-14))
	d.pctTxt.Move(fyne.NewPos((s.Width-total)/2+vs.Width, cy+vs.Height/2-ps.Height-16))
	ls := fyne.MeasureText(d.labTxt.Text, d.labTxt.TextSize, d.labTxt.TextStyle)
	d.labTxt.Move(fyne.NewPos((s.Width-ls.Width)/2, cy+vs.Height/2-8))
	ws := fyne.MeasureText(d.pwrTxt.Text, d.pwrTxt.TextSize, d.pwrTxt.TextStyle)
	d.pwrTxt.Move(fyne.NewPos((s.Width-ws.Width)/2, cy+vs.Height/2+ls.Height))
}

func (r *dialRenderer) Refresh() {
	d := r.d
	d.valTxt.Text = itoa(int(math.Round(d.value)))
	if d.power {
		d.pwrTxt.Text = "ON"
		d.pwrTxt.Color = colOK
	} else {
		d.pwrTxt.Text = "OFF"
		d.pwrTxt.Color = colErr
	}
	r.Layout(d.Size())
	d.raster.Refresh()
	d.valTxt.Refresh()
	d.pwrTxt.Refresh()
	d.labTxt.Refresh()
	d.pctTxt.Refresh()
}

func (r *dialRenderer) Objects() []fyne.CanvasObject {
	d := r.d
	return []fyne.CanvasObject{d.raster, d.valTxt, d.pctTxt, d.labTxt, d.pwrTxt}
}
func (r *dialRenderer) Destroy() {}

func itoa(v int) string {
	if v >= 100 {
		return "100"
	}
	return string([]byte{byte('0' + v/10), byte('0' + v%10)})
}
