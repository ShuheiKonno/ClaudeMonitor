package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"testing"
)

// DUMP_TRAY_PREVIEW=1 go test -run TestDumpTrayPreview で
// assets/tray-preview.png にトレイアイコンの主要状態を 3 状態 × 原寸 + 拡大で吐き出す。
func TestDumpTrayPreview(t *testing.T) {
	if os.Getenv("DUMP_TRAY_PREVIEW") == "" {
		t.Skip("DUMP_TRAY_PREVIEW=1 で有効化")
	}
	samples := []struct {
		pct5h, pct7d int
	}{
		{25, 30},
		{68, 65},
		{95, 92},
	}
	sz := trayIconSize
	scale := 8
	pad := 4
	cols := len(samples)
	w := cols*sz + (cols+1)*pad + cols*(sz*scale) + cols*pad
	h := sz*scale + pad*3
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	check1 := color.RGBA{R: 64, G: 64, B: 64, A: 255}
	check2 := color.RGBA{R: 90, G: 90, B: 90, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x/8+y/8)%2 == 0 {
				out.SetRGBA(x, y, check1)
			} else {
				out.SetRGBA(x, y, check2)
			}
		}
	}
	xpos := pad
	for _, s := range samples {
		img := drawTrayIconImage(s.pct5h, s.pct7d)
		draw.Draw(out, image.Rect(xpos, pad, xpos+sz, pad+sz), img, image.Point{}, draw.Over)
		scaled := image.NewRGBA(image.Rect(0, 0, sz*scale, sz*scale))
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				c := img.RGBAAt(x, y)
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						scaled.SetRGBA(x*scale+dx, y*scale+dy, c)
					}
				}
			}
		}
		draw.Draw(out,
			image.Rect(xpos+sz+pad, pad, xpos+sz+pad+sz*scale, pad+sz*scale),
			scaled, image.Point{}, draw.Over)
		xpos += sz + pad + sz*scale + pad
	}
	f, err := os.Create("assets/tray-preview.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, out); err != nil {
		t.Fatal(err)
	}
}
