package main

import (
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// ambientLayer is the passive background: faint bulb silhouettes drifting
// slowly, like dust in lamplight. Pure decoration — pointer-transparent,
// low alpha, no interaction.
type bulbSpec struct {
	x, y   float32 // anchor as fraction of screen
	r      float32 // glass radius px
	period float64 // seconds per drift loop
	phase  float64
}

var bulbSpecs = []bulbSpec{
	{0.15, 0.16, 26, 26, 0},
	{0.82, 0.24, 18, 34, 1.7},
	{0.68, 0.62, 30, 30, 3.1},
	{0.22, 0.74, 16, 38, 4.4},
	{0.50, 0.42, 12, 24, 5.2},
	{0.88, 0.82, 22, 42, 2.3},
}

func newAmbient() fyne.CanvasObject {
	layer := container.NewWithoutLayout()
	var bulbs []*fyne.Container
	for _, s := range bulbSpecs {
		glass := canvas.NewCircle(colTransparent)
		glass.StrokeColor = withAlpha(colAccent, 0x1E)
		glass.StrokeWidth = 1.5
		glass.Resize(fyne.NewSize(s.r*2, s.r*2))

		glow := canvas.NewCircle(withAlpha(colAccent, 0x10))
		glow.Resize(fyne.NewSize(s.r*1.1, s.r*1.1))
		glow.Move(fyne.NewPos(s.r*0.45, s.r*0.45))

		base := canvas.NewRectangle(withAlpha(colAccent, 0x18))
		base.CornerRadius = 2
		base.Resize(fyne.NewSize(s.r*0.7, s.r*0.28))
		base.Move(fyne.NewPos(s.r*0.65, s.r*2+2))

		b := container.NewWithoutLayout(glass, glow, base)
		layer.Add(b)
		bulbs = append(bulbs, b)
	}

	// one shared animation drives every bulb on its own sine path
	start := time.Now()
	anim := fyne.NewAnimation(time.Hour, func(float32) {
		t := time.Since(start).Seconds()
		size := layer.Size()
		if size.Width == 0 {
			return
		}
		for i, s := range bulbSpecs {
			w := 2 * math.Pi / s.period
			dx := float32(10 * math.Sin(w*t+s.phase))
			dy := float32(16 * math.Sin(w*t*0.7+s.phase*1.3))
			bulbs[i].Move(fyne.NewPos(size.Width*s.x+dx, size.Height*s.y+dy))
		}
	})
	anim.RepeatCount = fyne.AnimationRepeatForever
	anim.Start()
	return layer
}
