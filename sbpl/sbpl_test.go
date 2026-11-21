package sbpl

import (
	"bytes"
	"errors"
	"testing"

	"github.com/sasaron/labelraster"
)

// box is the 8x8 outline used in the CL4NX Programming Reference example.
func box() labelraster.Bitmap {
	b := labelraster.NewBitmap(8, 8)
	for i := range 8 {
		b.Set(i, 0, true)
		b.Set(i, 7, true)
		b.Set(0, i, true)
		b.Set(7, i, true)
	}
	return b
}

func graphic(b labelraster.Bitmap, hexData bool) []byte {
	var buf bytes.Buffer
	writeGraphic(&buf, b, hexData)
	return buf.Bytes()
}

func TestWriteGraphic_Bitmap_WritesESCG(t *testing.T) {
	tests := []struct {
		name string
		b    labelraster.Bitmap
		hex  bool
		want []byte
	}{
		{"binary", box(), false, append([]byte("\x1bGB001001"), 0xFF, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0xFF)},
		{"hex", box(), true, []byte("\x1bGH001001FF818181818181FF")},
		{"pads height", labelraster.NewBitmap(9, 3), false, append([]byte("\x1bGB002001"), make([]byte, 16)...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := graphic(tt.b, tt.hex); !bytes.Equal(got, tt.want) {
				t.Errorf("writeGraphic(%dx%d, hex=%v) = %q, want %q", tt.b.Width, tt.b.Height, tt.hex, got, tt.want)
			}
		})
	}
}

func TestEncode_Options_WritesJob(t *testing.T) {
	data := "\xff\x81\x81\x81\x81\x81\x81\xff"
	tests := []struct {
		name string
		opt  Options
		want string
	}{
		{"framed", Options{X: 50, Y: 50, Frame: true}, "\x02\x1bA\x1bV50\x1bH50\x1bGB001001" + data + "\x1bQ1\x1bZ\x03"},
		{"unframed", Options{}, "\x1bA\x1bV0\x1bH0\x1bGB001001" + data + "\x1bQ1\x1bZ"},
		{"copies", Options{Copies: 3}, "\x1bA\x1bV0\x1bH0\x1bGB001001" + data + "\x1bQ3\x1bZ"},
		{"hex", Options{Hex: true}, "\x1bA\x1bV0\x1bH0\x1bGH001001FF818181818181FF\x1bQ1\x1bZ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encode(box(), CL4NX203, tt.opt)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("Encode(box, %+v) = %q, want %q", tt.opt, got, tt.want)
			}
		})
	}
}

func TestEncode_4x6LabelAt203dpi_FitsCL4NX203(t *testing.T) {
	_, err := Encode(labelraster.NewBitmap(813, 1219), CL4NX203, Options{})

	if err != nil {
		t.Errorf("Encode(813x1219, CL4NX203) error = %v, want nil", err)
	}
}

func TestEncode_LargerThanModel_ReturnsErrTooLarge(t *testing.T) {
	tests := []struct {
		name string
		w, h int
	}{
		{"105 bytes wide", 840, 8},
		{"401 units high", 8, 3208},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encode(labelraster.NewBitmap(tt.w, tt.h), CL4NX203, Options{})

			if !errors.Is(err, ErrTooLarge) {
				t.Errorf("Encode(%dx%d, CL4NX203) error = %v, want ErrTooLarge", tt.w, tt.h, err)
			}
		})
	}
}

func TestEncode_EmptyBitmap_ReturnsError(t *testing.T) {
	_, err := Encode(labelraster.NewBitmap(0, 0), CL4NX203, Options{})

	if err == nil || errors.Is(err, ErrTooLarge) {
		t.Errorf("Encode(0x0, CL4NX203) error = %v, want a non-ErrTooLarge error", err)
	}
}
