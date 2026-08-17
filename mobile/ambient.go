package main

import (
	"image"
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// The aura: the screen is the bulb. The glow is baked into one small texture
// (regenerated only when the light's color changes — GPU scales it up), and
// brightness maps to cheap Translucency. The animation only Moves embers.

const glowTex = 256 // baked glow texture size; blurry by nature, scales free

type aura struct {
	img     *canvas.Image
	topFade *canvas.Image // smoothstep blends into the system-bar strips,
	botFade *canvas.Image // with opaque headroom past the content edge
	layer   *fyne.Container
	anim    *fyne.Animation

	embers []*canvas.RadialGradient

	col   color.NRGBA
	level float64
	on    bool
}

// glowAlpha is the color-independent radial falloff, computed once: a bright
// core easing into a wide soft corona; (1-d²)ⁿ keeps the tail smooth so the
// texture edge never reads as a boundary.
var glowAlpha = func() []uint8 {
	a := make([]uint8, glowTex*glowTex)
	c := float64(glowTex) / 2
	for y := 0; y < glowTex; y++ {
		for x := 0; x < glowTex; x++ {
			d := math.Hypot(float64(x)-c, float64(y)-c) / c // 0 center → 1 edge
			if d < 1 {
				a[y*glowTex+x] = uint8(math.Pow(1-d*d, 2.6) * 0.9 * 255)
			}
		}
	}
	return a
}()

// bakeGlow tints the precomputed falloff — cheap enough for per-drag recolor.
func bakeGlow(col color.NRGBA) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, glowTex, glowTex))
	for i, al := range glowAlpha {
		if al == 0 {
			continue
		}
		p := i * 4
		img.Pix[p], img.Pix[p+1], img.Pix[p+2], img.Pix[p+3] = col.R, col.G, col.B, al
	}
	return img
}

// set retints/rescales the glow to the light's live state.
func (a *aura) set(col color.NRGBA, level float64, on bool) {
	if col != a.col {
		a.img.Image = bakeGlow(col)
		for i, s := range emberSpecs {
			a.embers[i].StartColor = withAlpha(col, s.alpha)
			a.embers[i].Refresh()
		}
	}
	a.col, a.level, a.on = col, level, on
	strength := 0.10
	if on {
		strength = 0.22 + level*0.48
	}
	a.img.Translucency = 1 - strength
	a.place(a.layer.Size())
	a.img.Refresh()
}

// place centers the glow, rising slightly with brightness.
func (a *aura) place(s fyne.Size) {
	if s.Width == 0 {
		return
	}
	y := s.Height * float32(0.56-0.24*a.level)
	// spans the whole screen; the translucent activity theme lets it run
	// under the system bars, so the falloff never meets a hard edge
	w := s.Width * 2.6
	h := s.Height * 1.9
	a.img.Resize(fyne.NewSize(w, h))
	a.img.Move(fyne.NewPos(s.Width/2-w/2, y-h/2))
	// fades overhang the content edge: the glow bleeds under the translucent
	// system bars (outside content bounds), so each fade holds full strip
	// color across that overhang before easing off inside the content
	fadeSize := fyne.NewSize(s.Width, fadeOver+fadeRun)
	a.topFade.Resize(fadeSize)
	a.topFade.Move(fyne.NewPos(0, -fadeOver))
	a.botFade.Resize(fadeSize)
	a.botFade.Move(fyne.NewPos(0, s.Height-fadeRun))
}

type emberSpec struct {
	x      float32
	size   float32
	period float64
	phase  float64
	sway   float64
	alpha  uint8
}

var emberSpecs = []emberSpec{
	{0.08, 5, 24, 0.00, 22, 0x3C}, {0.18, 3, 31, 0.45, 14, 0x2E},
	{0.29, 6, 21, 0.80, 30, 0x46}, {0.38, 2, 36, 0.15, 12, 0x24},
	{0.47, 4, 27, 0.60, 20, 0x38}, {0.56, 3, 33, 0.30, 16, 0x2A},
	{0.66, 5, 23, 0.90, 26, 0x40}, {0.74, 2, 38, 0.05, 10, 0x20},
	{0.83, 4, 26, 0.55, 18, 0x34}, {0.91, 3, 30, 0.70, 24, 0x2C},
	{0.25, 4, 29, 0.25, 20, 0x30}, {0.62, 6, 20, 0.40, 28, 0x44},
}

// newAura builds glow + embers. The animation only moves embers — cheap.
const (
	// ponytail: fixed overhang instead of querying the real bar inset —
	// covers any portrait status/nav bar; switch to Canvas().InteractiveArea()
	// if a device ever ships a taller inset
	fadeOver float32 = 200 // opaque overhang past the content edge
	fadeRun  float32 = 300 // smoothstep run inside the content
	fadeTex          = 512 // 1px-wide baked ramp, GPU-stretched like the glow
)

// bakeFade renders the 1×N strip-color ramp: opaque across the overhang,
// then a smoothstep to transparent (zero slope both ends, so neither edge
// of the fade shows).
func bakeFade(bottom bool) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 1, fadeTex))
	over := float64(fadeOver / (fadeOver + fadeRun))
	for y := 0; y < fadeTex; y++ {
		t := float64(y) / float64(fadeTex-1)
		if bottom {
			t = 1 - t
		}
		t = (t - over) / (1 - over)
		f := 1.0
		if t > 0 {
			f = 1 - (3*t*t - 2*t*t*t)
		}
		img.SetNRGBA(0, y, withAlpha(colBGDeep, uint8(f*255)))
	}
	return img
}

// refade rebakes the edge fades (theme switch changes colBGDeep).
func (a *aura) refade() {
	a.topFade.Image = bakeFade(false)
	a.botFade.Image = bakeFade(true)
	a.topFade.Refresh()
	a.botFade.Refresh()
}

func newAura() *aura {
	a := &aura{img: canvas.NewImageFromImage(bakeGlow(colAccent))}
	a.img.ScaleMode = canvas.ImageScaleFastest
	a.topFade = &canvas.Image{ScaleMode: canvas.ImageScaleFastest}
	a.botFade = &canvas.Image{ScaleMode: canvas.ImageScaleFastest}
	a.refade()
	a.layer = container.NewWithoutLayout(a.img, a.topFade, a.botFade)

	for _, s := range emberSpecs {
		e := canvas.NewRadialGradient(withAlpha(colAccent, s.alpha), colTransparent)
		e.Resize(fyne.NewSize(s.size*4, s.size*4))
		a.layer.Add(e)
		a.embers = append(a.embers, e)
	}

	start := time.Now()
	a.anim = fyne.NewAnimation(time.Hour, func(float32) {
		t := time.Since(start).Seconds()
		size := a.layer.Size()
		if size.Height == 0 {
			return
		}
		show := int(math.Ceil(a.level * float64(len(a.embers))))
		if !a.on {
			show = 2
		}
		for i, s := range emberSpecs {
			if i >= show {
				a.embers[i].Hide()
				continue
			}
			a.embers[i].Show()
			p := t/s.period + s.phase
			p -= math.Floor(p)
			y := size.Height*1.05 - float32(p)*size.Height*1.15
			x := size.Width*s.x + float32(s.sway*math.Sin(2*math.Pi*p*2.2+s.phase*7))
			a.embers[i].Move(fyne.NewPos(x, y))
		}
	})
	a.anim.RepeatCount = fyne.AnimationRepeatForever
	a.anim.Start()
	return a
}
