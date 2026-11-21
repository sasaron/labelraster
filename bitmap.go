package labelraster

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// Bitmap is a 1-bit image. A set bit is a black dot.
// Each row is Stride bytes, most significant bit first.
// Copies of a Bitmap share Bits.
type Bitmap struct {
	Width, Height int
	Stride        int
	Bits          []byte
}

// NewBitmap returns a white Bitmap. It panics if width or height is negative.
func NewBitmap(width, height int) Bitmap {
	if width < 0 || height < 0 {
		panic(fmt.Sprintf("labelraster: negative size %dx%d", width, height))
	}
	stride := (width + 7) / 8
	return Bitmap{Width: width, Height: height, Stride: stride, Bits: make([]byte, stride*height)}
}

// Black reports whether (x, y) is black. Points outside the bitmap are white.
func (b Bitmap) Black(x, y int) bool {
	if x < 0 || y < 0 || x >= b.Width || y >= b.Height {
		return false
	}
	return b.Bits[y*b.Stride+x/8]&(0x80>>(x%8)) != 0
}

// Set sets (x, y) in Bits, which copies of b share. Points outside the bitmap are ignored.
func (b Bitmap) Set(x, y int, black bool) {
	if x < 0 || y < 0 || x >= b.Width || y >= b.Height {
		return
	}
	i := y*b.Stride + x/8
	m := byte(0x80 >> (x % 8))
	if black {
		b.Bits[i] |= m
	} else {
		b.Bits[i] &^= m
	}
}

// Image returns b as a grayscale image.
func (b Bitmap) Image() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, b.Width, b.Height))
	for y := range b.Height {
		for x := range b.Width {
			if !b.Black(x, y) {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	return img
}

// Density is the print head density in dots per millimeter.
type Density float64

// Common print head densities.
const (
	DPI203 Density = 8  // 8 dots/mm, sold as 203 dpi
	DPI305 Density = 12 // 12 dots/mm, sold as 300 or 305 dpi
	DPI609 Density = 24 // 24 dots/mm, sold as 600 or 609 dpi
)

// DotsForPoints converts PDF points to whole dots.
func (d Density) DotsForPoints(pt float64) int {
	return int(math.Round(pt / 72 * 25.4 * float64(d)))
}
