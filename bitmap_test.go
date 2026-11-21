package labelraster

import (
	"bytes"
	"image/color"
	"testing"
)

func TestNewBitmap_WidthNotMultipleOf8_RoundsStrideUp(t *testing.T) {
	b := NewBitmap(10, 2)

	if b.Stride != 2 || len(b.Bits) != 4 {
		t.Errorf("NewBitmap(10, 2) Stride = %d, len(Bits) = %d, want 2, 4", b.Stride, len(b.Bits))
	}
}

func TestNewBitmap_NegativeSize_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewBitmap(-1, 1) did not panic")
		}
	}()

	NewBitmap(-1, 1)
}

func TestSet_Black_SetsMostSignificantBitFirst(t *testing.T) {
	b := NewBitmap(10, 2)

	b.Set(0, 0, true)
	b.Set(9, 1, true)

	if want := []byte{0x80, 0x00, 0x00, 0x40}; !bytes.Equal(b.Bits, want) {
		t.Errorf("Bits after Set(0,0) and Set(9,1) = %#v, want %#v", b.Bits, want)
	}
}

func TestSet_White_ClearsDot(t *testing.T) {
	b := NewBitmap(8, 1)
	b.Set(3, 0, true)

	b.Set(3, 0, false)

	if b.Black(3, 0) {
		t.Error("Black(3, 0) = true after Set(3, 0, false), want false")
	}
}

func TestSet_OutsideBitmap_IsIgnored(t *testing.T) {
	b := NewBitmap(8, 1)

	b.Set(8, 0, true)
	b.Set(-1, 0, true)

	if want := []byte{0x00}; !bytes.Equal(b.Bits, want) {
		t.Errorf("Bits after Set outside = %#v, want %#v", b.Bits, want)
	}
}

func TestBlack_OutsideBitmap_IsWhite(t *testing.T) {
	b := NewBitmap(2, 2)
	b.Bits[0], b.Bits[1] = 0xFF, 0xFF

	for _, p := range [][2]int{{-1, 0}, {2, 0}, {0, -1}, {0, 2}} {
		if b.Black(p[0], p[1]) {
			t.Errorf("Black(%d, %d) = true, want false", p[0], p[1])
		}
	}
}

func TestImage_BlackAndWhiteDots_MapTo0And255(t *testing.T) {
	b := NewBitmap(2, 1)
	b.Set(0, 0, true)

	img := b.Image()

	if got := img.GrayAt(0, 0); got != (color.Gray{Y: 0}) {
		t.Errorf("Image().GrayAt(0, 0) = %v, want {0}", got)
	}
	if got := img.GrayAt(1, 0); got != (color.Gray{Y: 255}) {
		t.Errorf("Image().GrayAt(1, 0) = %v, want {255}", got)
	}
}

func TestDotsForPoints_LabelSizes_RoundToWholeDots(t *testing.T) {
	tests := []struct {
		d    Density
		pt   float64
		want int
	}{
		{DPI203, 288, 813},  // 4 in = 812.8 dots
		{DPI203, 432, 1219}, // 6 in = 1219.2 dots
		{DPI305, 288, 1219}, // 4 in = 1219.2 dots
		{DPI609, 72, 610},   // 1 in = 609.6 dots
	}
	for _, tt := range tests {
		if got := tt.d.DotsForPoints(tt.pt); got != tt.want {
			t.Errorf("Density(%v).DotsForPoints(%v) = %d, want %d", float64(tt.d), tt.pt, got, tt.want)
		}
	}
}
