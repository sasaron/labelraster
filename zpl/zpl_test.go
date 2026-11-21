package zpl

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/ingridhq/zebrash"
	"github.com/ingridhq/zebrash/drawers"

	"github.com/sasaron/labelraster"
)

func TestEncode_Options_WritesCommands(t *testing.T) {
	small := labelraster.NewBitmap(8, 2)
	for x := range 8 {
		small.Set(x, 0, true)
	}
	small.Set(0, 1, true)
	small.Set(7, 1, true)
	tests := []struct {
		name string
		opt  Options
		want string
	}{
		{"origin", Options{X: 10, Y: 20}, "^XA\n^FO10,20^GFA,2,2,1,FF81^FS\n^XZ\n"},
		{"one copy", Options{Copies: 1}, "^XA\n^FO0,0^GFA,2,2,1,FF81^FS\n^XZ\n"},
		{"copies", Options{Copies: 2}, "^XA\n^FO0,0^GFA,2,2,1,FF81^FS\n^PQ2\n^XZ\n"},
		{"label size", Options{X: 3, Y: 4, LabelSize: true}, "^XA\n^PW11\n^LL6\n^FO3,4^GFA,2,2,1,FF81^FS\n^XZ\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(Encode(small, tt.opt)); got != tt.want {
				t.Errorf("Encode(8x2, %+v) =\n%q\nwant\n%q", tt.opt, got, tt.want)
			}
		})
	}
}

func TestEncode_Over99999Bytes_SplitsIntoBands(t *testing.T) {
	// 4x6 in at 8 dots/mm: 102 bytes x 1219 rows = 124,338 bytes; 980 rows fit in one ^GF.
	out := string(Encode(labelraster.NewBitmap(813, 1219), Options{Y: 7}))
	if n := strings.Count(out, "^GFA"); n != 2 {
		t.Fatalf("Encode(813x1219) has %d ^GFA, want 2", n)
	}
	for _, header := range []string{"^FO0,7^GFA,99960,99960,102,", "^FO0,987^GFA,24378,24378,102,"} {
		if !strings.Contains(out, header) {
			t.Errorf("Encode(813x1219, Y=7) lacks %q", header)
		}
	}
}

func TestEncode_RowWiderThanFieldLimit_WritesOneRowPerField(t *testing.T) {
	b := labelraster.NewBitmap(800008, 2) // 100001 bytes per row

	got := Encode(b, Options{})

	if n := bytes.Count(got, []byte("^GFA,100001,100001,100001,")); n != 2 {
		t.Errorf("Encode(800008x2) has %d fields of 100001 bytes, want 2", n)
	}
}

func TestEncode_RenderedByZebrash_MatchesBitmap(t *testing.T) {
	for _, size := range []struct{ w, h int }{
		{406, 203},  // one band
		{813, 1219}, // two bands
		{13, 9},     // width not a multiple of 8
	} {
		t.Run(fmt.Sprintf("%dx%d", size.w, size.h), func(t *testing.T) {
			roundTrip(t, size.w, size.h)
		})
	}
}

func roundTrip(t *testing.T, w, h int) {
	b := labelraster.NewBitmap(w, h)
	r := rand.New(rand.NewPCG(1, 2))
	for y := range h {
		for x := range w {
			b.Set(x, y, r.IntN(2) == 0)
		}
	}
	zplData := Encode(b, Options{LabelSize: true})

	labels, err := zebrash.NewParser().Parse(zplData)
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 1 {
		t.Fatalf("zebrash parsed %d labels, want 1", len(labels))
	}
	var buf bytes.Buffer
	err = zebrash.NewDrawer().DrawLabelAsPng(labels[0], &buf, drawers.DrawerOptions{
		LabelWidthMm:  float64(w) / 8,
		LabelHeightMm: float64(h) / 8,
		Dpmm:          8,
	})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if n, x, y := diff(b, img); n != 0 {
		t.Errorf("zebrash render of Encode(%dx%d) differs at %d dots, first at (%d, %d)", w, h, n, x, y)
	}
}

// diff counts dots where img differs from b and returns the first one.
func diff(b labelraster.Bitmap, img image.Image) (n, fx, fy int) {
	bounds := img.Bounds()
	fx, fy = -1, -1
	for y := range b.Height {
		for x := range b.Width {
			r, g, bl, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			if ((r+g+bl)/3 < 0x8000) != b.Black(x, y) {
				if n == 0 {
					fx, fy = x, y
				}
				n++
			}
		}
	}
	return n, fx, fy
}
