// Package raster renders PDF pages into 1-bit bitmaps on a printer's dot grid.
package raster

import (
	"image"

	"github.com/sasaron/labelraster"
)

var (
	_ Strategy = Threshold{}
	_ Strategy = Supersample{}
	_ Strategy = Snap{}
	_ Source   = Page{}
)

// RenderOptions disables anti-aliasing per object type.
type RenderOptions struct {
	NoSmoothPath  bool
	NoSmoothText  bool
	NoSmoothImage bool
}

// Source renders a page scaled to width x height pixels.
type Source interface {
	Render(width, height int, opt RenderOptions) (*image.Gray, error)
}

// Strategy turns a page into a bitmap of width x height dots.
// Density is for strategies whose options are physical sizes.
type Strategy interface {
	Rasterize(src Source, d labelraster.Density, width, height int) (labelraster.Bitmap, error)
}

// Threshold renders at the dot grid and marks pixels darker than Level as black.
type Threshold struct {
	Level   uint8 // defaults to 128
	Options RenderOptions
}

// Rasterize implements Strategy.
func (t Threshold) Rasterize(src Source, _ labelraster.Density, width, height int) (labelraster.Bitmap, error) {
	img, err := src.Render(width, height, t.Options)
	if err != nil {
		return labelraster.Bitmap{}, err
	}
	level := levelOr(t.Level)
	b := labelraster.NewBitmap(width, height)
	r := img.Bounds()
	for y := range min(height, r.Dy()) {
		for x := range min(width, r.Dx()) {
			if img.GrayAt(r.Min.X+x, r.Min.Y+y).Y < level {
				b.Set(x, y, true)
			}
		}
	}
	return b, nil
}

// Supersample renders at Factor times the dot grid and marks a dot black when
// the mean of its Factor x Factor pixels is darker than Level.
type Supersample struct {
	Factor  int   // defaults to 4
	Level   uint8 // defaults to 128
	Options RenderOptions
}

// Rasterize implements Strategy.
func (s Supersample) Rasterize(src Source, _ labelraster.Density, width, height int) (labelraster.Bitmap, error) {
	f := s.Factor
	if f < 1 {
		f = 4
	}
	img, err := src.Render(width*f, height*f, s.Options)
	if err != nil {
		return labelraster.Bitmap{}, err
	}
	return downsample(img, width, height, f, levelOr(s.Level)), nil
}

const defaultLevel = 128

func levelOr(l uint8) uint8 {
	if l == 0 {
		return defaultLevel
	}
	return l
}

func downsample(img *image.Gray, width, height, f int, level uint8) labelraster.Bitmap {
	b := labelraster.NewBitmap(width, height)
	r := img.Bounds()
	for y := range height {
		for x := range width {
			sum, n := 0, 0
			for yy := y * f; yy < (y+1)*f && yy < r.Dy(); yy++ {
				for xx := x * f; xx < (x+1)*f && xx < r.Dx(); xx++ {
					sum += int(img.GrayAt(r.Min.X+xx, r.Min.Y+yy).Y)
					n++
				}
			}
			if n > 0 && sum < int(level)*n {
				b.Set(x, y, true)
			}
		}
	}
	return b
}
