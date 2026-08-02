//go:build ignore

// Generates build/appicon.png (1024²) — icon D "Three Lights":
// three overlapping bulb hues on the app's near-black rounded square.
// Run: go run build/gen_icon.go
package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

const (
	super = 2048 // 2x supersample, downscaled for antialiasing
	out   = 1024
)

type circle struct {
	x, y, r float64
	c       [3]float64
	a       float64
}

func main() {
	scale := float64(super) / 120.0
	corner := 27.0 * scale
	circles := []circle{
		{48 * scale, 48 * scale, 26 * scale, [3]float64{255, 217, 160}, 0.55},
		{74 * scale, 56 * scale, 26 * scale, [3]float64{137, 180, 250}, 0.45},
		{58 * scale, 76 * scale, 26 * scale, [3]float64{203, 166, 247}, 0.45},
	}
	bg := [3]float64{10, 10, 15} // #0A0A0F

	img := image.NewRGBA(image.Rect(0, 0, out, out))
	for oy := 0; oy < out; oy++ {
		for ox := 0; ox < out; ox++ {
			var rs, gs, bs, as float64
			for sy := 0; sy < 2; sy++ {
				for sx := 0; sx < 2; sx++ {
					x := float64(ox*2+sx) + 0.5
					y := float64(oy*2+sy) + 0.5
					if !inRoundedRect(x, y, super, corner) {
						continue
					}
					r, g, b := bg[0], bg[1], bg[2]
					for _, c := range circles {
						dx, dy := x-c.x, y-c.y
						if dx*dx+dy*dy <= c.r*c.r {
							r = r*(1-c.a) + c.c[0]*c.a
							g = g*(1-c.a) + c.c[1]*c.a
							b = b*(1-c.a) + c.c[2]*c.a
						}
					}
					rs += r
					gs += g
					bs += b
					as += 255
				}
			}
			img.Set(ox, oy, color.RGBA{
				uint8(rs / 4), uint8(gs / 4), uint8(bs / 4), uint8(as / 4),
			})
		}
	}

	f, err := os.Create("build/appicon.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func inRoundedRect(x, y, size, r float64) bool {
	if x < 0 || y < 0 || x > size || y > size {
		return false
	}
	cx := clamp(x, r, size-r)
	cy := clamp(y, r, size-r)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
