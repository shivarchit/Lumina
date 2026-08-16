package main

import (
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// monoText is a mono canvas label in the app's type voice.
func monoText(s string, size float32, c color.Color) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

// gap is a fixed-height transparent spacer.
func gap(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(colTransparent)
	r.SetMinSize(fyne.NewSize(1, h))
	return r
}

// ── word: Aura's only button — a tappable mono word, color is meaning ──

type word struct {
	widget.BaseWidget
	text     string
	col      color.NRGBA
	size     float32
	onTapped func()

	lbl *canvas.Text
}

// size is part of construction — post-render size pokes would silently
// half-apply (renderer caches TextSize), so the field stays unexported.
func newWord(text string, size float32, col color.NRGBA, tapped func()) *word {
	w := &word{text: text, col: col, size: size, onTapped: tapped}
	w.ExtendBaseWidget(w)
	return w
}

func (w *word) set(text string, col color.NRGBA) {
	w.text = text
	w.col = col
	w.Refresh()
}

func (w *word) Tapped(*fyne.PointEvent) {
	if w.onTapped != nil {
		w.onTapped()
	}
}

func (w *word) CreateRenderer() fyne.WidgetRenderer {
	w.lbl = monoText(w.text, w.size, w.col)
	return &wordRenderer{w: w}
}

type wordRenderer struct{ w *word }

func (r *wordRenderer) MinSize() fyne.Size {
	ts := fyne.MeasureText(r.w.text, r.w.size, fyne.TextStyle{Monospace: true})
	return fyne.NewSize(ts.Width+12, ts.Height+14) // breathing room = touch target
}

func (r *wordRenderer) Layout(s fyne.Size) {
	ts := fyne.MeasureText(r.w.text, r.w.size, fyne.TextStyle{Monospace: true})
	r.w.lbl.Move(fyne.NewPos((s.Width-ts.Width)/2, (s.Height-ts.Height)/2))
	r.w.lbl.Resize(ts)
}

func (r *wordRenderer) Refresh() {
	r.w.lbl.Text = r.w.text
	r.w.lbl.Color = r.w.col
	r.w.lbl.Refresh()
	r.Layout(r.w.Size())
}

func (r *wordRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.w.lbl} }
func (r *wordRenderer) Destroy()                     {}

// ── dragField: vertical drag anywhere = brightness ──────────────────

type dragField struct {
	widget.BaseWidget
	value    float64 // 10..100, seeded from the real brightness
	onChange func(v float64)
	onEnd    func(v int)
	startV   float64
	startY   float32
	dragging bool
}

func newDragField(start float64, onChange func(float64), onEnd func(int)) *dragField {
	d := &dragField{value: start, onChange: onChange, onEnd: onEnd}
	d.ExtendBaseWidget(d)
	return d
}

func (d *dragField) Dragged(e *fyne.DragEvent) {
	if !d.dragging {
		d.dragging = true
		d.startV = d.value
		d.startY = e.Position.Y
	}
	d.value = math.Max(10, math.Min(100, d.startV+float64(d.startY-e.Position.Y)*0.5))
	if d.onChange != nil {
		d.onChange(d.value)
	}
}

func (d *dragField) DragEnd() {
	d.dragging = false
	if d.onEnd != nil {
		d.onEnd(int(math.Round(d.value)))
	}
}

func (d *dragField) CreateRenderer() fyne.WidgetRenderer {
	// invisible: the numeral/labels stack on top of it in a Stack container
	return widget.NewSimpleRenderer(canvas.NewRectangle(colTransparent))
}

// ── gslider: draggable gradient bar (temp kelvin ramp, color hue) ───

const (
	gsliderPad float32 = 14
	gsliderH   float32 = 44
)

type gslider struct {
	widget.BaseWidget
	min, max, value float64
	grad            func(v float64) color.NRGBA // value → color
	onChange, onEnd func(v float64)

	raster *canvas.Raster
	thumb  *canvas.Circle
}

func newGSlider(min, max, val float64, grad func(float64) color.NRGBA, onChange, onEnd func(float64)) *gslider {
	g := &gslider{min: min, max: max, value: val, grad: grad, onChange: onChange, onEnd: onEnd}
	g.ExtendBaseWidget(g)
	return g
}

func (g *gslider) frac() float64 { return (g.value - g.min) / (g.max - g.min) }

func (g *gslider) setFromX(x float32) {
	pad := gsliderPad
	f := float64((x - pad) / (g.Size().Width - 2*pad))
	f = math.Max(0, math.Min(1, f))
	g.value = g.min + f*(g.max-g.min)
	g.Refresh()
	if g.onChange != nil {
		g.onChange(g.value)
	}
}

func (g *gslider) Dragged(e *fyne.DragEvent) { g.setFromX(e.Position.X) }
func (g *gslider) DragEnd() {
	if g.onEnd != nil {
		g.onEnd(g.value)
	}
}
func (g *gslider) Tapped(e *fyne.PointEvent) {
	g.setFromX(e.Position.X)
	if g.onEnd != nil {
		g.onEnd(g.value)
	}
}

func (g *gslider) MinSize() fyne.Size { return fyne.NewSize(260, gsliderH) }

func (g *gslider) CreateRenderer() fyne.WidgetRenderer {
	g.raster = canvas.NewRaster(g.drawBar)
	g.thumb = canvas.NewCircle(colAccent)
	g.thumb.StrokeColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xE6}
	g.thumb.StrokeWidth = 2
	return &gsliderRenderer{g: g}
}

// drawBar renders a rounded gradient bar with feathered edges.
func (g *gslider) drawBar(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w == 0 || h == 0 {
		return img
	}
	barH := float64(h) * 0.22
	cy := float64(h) / 2
	pad := float64(h) * float64(gsliderPad/gsliderH)
	r := barH / 2
	x0, x1 := pad+r, float64(w)-pad-r
	feather := 1.5
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			fx, fy := float64(x), float64(y)
			cx := math.Max(x0, math.Min(x1, fx))
			b := r - math.Hypot(fx-cx, fy-cy)
			if b <= -feather {
				continue
			}
			frac := (cx - x0) / (x1 - x0)
			c := g.grad(g.min + frac*(g.max-g.min))
			c.A = uint8(float64(0xE0) * math.Max(0, math.Min(1, b/feather+1)))
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

type gsliderRenderer struct{ g *gslider }

func (r *gsliderRenderer) MinSize() fyne.Size { return r.g.MinSize() }
func (r *gsliderRenderer) Layout(s fyne.Size) {
	r.g.raster.Resize(s)
	r.g.raster.Refresh()
	r.Refresh()
}
func (r *gsliderRenderer) Refresh() {
	g := r.g
	s := g.Size()
	d := float32(20)
	x := gsliderPad + float32(g.frac())*(s.Width-2*gsliderPad) - d/2
	g.thumb.FillColor = g.grad(g.value)
	g.thumb.Resize(fyne.NewSize(d, d))
	g.thumb.Move(fyne.NewPos(x, s.Height/2-d/2))
	g.thumb.Refresh()
	// bar is value-independent — only Layout (resize) re-rasters it
}
func (r *gsliderRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.g.raster, r.g.thumb}
}
func (r *gsliderRenderer) Destroy() {}
