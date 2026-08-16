package main

import (
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// ambientLayer: rising embers — tiny warm light motes floating up like dust
// in lamplight (option B from the ambient mockup). Pure light, no shapes,
// pointer-transparent, colors follow the active theme accent.
type emberSpec struct {
	x      float32 // horizontal anchor as fraction of width
	size   float32 // px
	period float64 // seconds for one bottom-to-top pass
	phase  float64 // 0..1 offset so they don't march in step
	sway   float64 // horizontal wander in px
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

func newAmbient() (fyne.CanvasObject, *fyne.Animation) {
	layer := container.NewWithoutLayout()
	var embers []*canvas.RadialGradient
	for _, s := range emberSpecs {
		e := canvas.NewRadialGradient(withAlpha(colAccent, s.alpha), colTransparent)
		e.Resize(fyne.NewSize(s.size*4, s.size*4)) // glow halo is 4x the core
		layer.Add(e)
		embers = append(embers, e)
	}

	start := time.Now()
	anim := fyne.NewAnimation(time.Hour, func(float32) {
		t := time.Since(start).Seconds()
		size := layer.Size()
		if size.Height == 0 {
			return
		}
		for i, s := range emberSpecs {
			p := t/s.period + s.phase
			p -= math.Floor(p) // 0..1, wraps forever
			y := size.Height*1.05 - float32(p)*size.Height*1.15
			x := size.Width*s.x + float32(s.sway*math.Sin(2*math.Pi*p*2.2+s.phase*7))
			embers[i].Move(fyne.NewPos(x, y))
		}
	})
	anim.RepeatCount = fyne.AnimationRepeatForever
	anim.Start()
	return layer, anim
}
