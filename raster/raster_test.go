package raster

import (
	"bytes"
	"errors"
	"image"
	"math"
	"slices"
	"testing"

	"github.com/sasaron/labelraster"
)

// barcodeSource draws an ideal barcode with exact area coverage.
// Coordinates are in output dots.
type barcodeSource struct {
	width, height int
	modules       []int   // element widths in modules, bar first
	module        float64 // dots per module
	x0, y0, y1    float64
	vertical      bool
	shrink        float64   // bar width reduction in dots, split across both edges
	jitter        []float64 // per-element width offsets in dots
	leftBar       float64   // a 30-dot black block this many dots left of x0, 0 for none
}

func (s barcodeSource) Render(w, h int, opt labelraster.RenderOptions) (*image.Gray, error) {
	if s.vertical {
		s.vertical = false
		hor, _ := s.Render(h, w, opt) // barcodeSource never fails
		img := image.NewGray(image.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				img.Pix[y*img.Stride+x] = hor.Pix[x*hor.Stride+y]
			}
		}
		return img, nil
	}
	scale := float64(w) / float64(s.width)
	var edges [][2]float64 // black intervals in pixels
	x := s.x0 * scale
	for i, m := range s.modules {
		w := float64(m) * s.module
		if i < len(s.jitter) {
			w += s.jitter[i]
		}
		next := x + w*scale
		if i%2 == 0 {
			d := s.shrink / 2 * scale
			edges = append(edges, [2]float64{x + d, next - d})
		}
		x = next
	}
	if s.leftBar > 0 {
		edges = append(edges, [2]float64{(s.x0 - s.leftBar - 30) * scale, (s.x0 - s.leftBar) * scale})
	}
	img := image.NewGray(image.Rect(0, 0, w, h))
	for px := range w {
		cover := 0.0
		for _, e := range edges {
			cover += max(0, min(e[1], float64(px+1))-max(e[0], float64(px)))
		}
		if opt.NoSmoothPath {
			cover = math.Round(cover)
		}
		v := uint8(math.Round(255 * (1 - cover)))
		for py := range h {
			y := (float64(py) + 0.5) / scale
			if y >= s.y0 && y < s.y1 {
				img.Pix[py*img.Stride+px] = v
			} else {
				img.Pix[py*img.Stride+px] = 255
			}
		}
	}
	return img, nil
}

// pattern is a Code 128 style element sequence in modules, bar first: 11 bars using 1 to 4 modules.
var pattern = []int{2, 1, 1, 2, 1, 4, 1, 1, 3, 2, 2, 2, 1, 2, 3, 1, 2, 2, 2, 1, 2}

// newBarcode returns pattern drawn on a 400x120-dot page from x = 60.3 and y = 20 to 100.
func newBarcode(module float64) barcodeSource {
	return barcodeSource{width: 400, height: 120, modules: pattern, module: module, x0: 60.3, y0: 20, y1: 100}
}

func rasterize(t *testing.T, s labelraster.Rasterizer, src barcodeSource) labelraster.Bitmap {
	t.Helper()
	w, h := src.width, src.height
	if src.vertical {
		w, h = h, w
	}
	b, err := s.Rasterize(src, labelraster.DPI203, w, h)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// rowRuns returns the widths of the runs on row y from the first black dot.
func rowRuns(b labelraster.Bitmap, y int) []int {
	var out []int
	black, n, started := false, 0, false
	for x := range b.Width {
		bl := b.Black(x, y)
		if !started {
			if !bl {
				continue
			}
			started, black = true, true
		}
		if bl != black {
			out = append(out, n)
			black, n = bl, 0
		}
		n++
	}
	if started && black {
		out = append(out, n)
	}
	return out
}

func columnRuns(b labelraster.Bitmap, x int) []int {
	col := labelraster.NewBitmap(b.Height, 1)
	for y := range b.Height {
		col.Set(y, 0, b.Black(x, y))
	}
	return rowRuns(col, 0)
}

func blackDots(b labelraster.Bitmap) int {
	n := 0
	for y := range b.Height {
		for x := range b.Width {
			if b.Black(x, y) {
				n++
			}
		}
	}
	return n
}

// patternAt2 is pattern with 2 dots per module.
var patternAt2 = []int{4, 2, 2, 4, 2, 8, 2, 2, 6, 4, 4, 4, 2, 4, 6, 2, 4, 4, 4, 2, 4}

func TestThreshold_OffGridModule_GivesUnevenBars(t *testing.T) {
	src := newBarcode(2.4)

	b := rasterize(t, Threshold{Options: labelraster.RenderOptions{NoSmoothPath: true}}, src)

	if got := rowRuns(b, 60); slices.Equal(got, patternAt2) {
		t.Errorf("Threshold row 60 = %v, want uneven widths for a 2.4-dot module", got)
	}
}

func TestSnap_OffGridModule_RedrawsWholeDotBars(t *testing.T) {
	src := newBarcode(2.4)

	b := rasterize(t, Snap{}, src)

	if got := rowRuns(b, 60); !slices.Equal(got, patternAt2) {
		t.Errorf("Snap row 60 = %v, want %v", got, patternAt2)
	}
}

func TestSnap_OffGridModule_RedrawsEveryRow(t *testing.T) {
	src := newBarcode(2.4)

	b := rasterize(t, Snap{}, src)

	if got := rowRuns(b, 21); !slices.Equal(got, patternAt2) {
		t.Errorf("Snap row 21 = %v, want %v", got, patternAt2)
	}
	if got := rowRuns(b, 98); !slices.Equal(got, patternAt2) {
		t.Errorf("Snap row 98 = %v, want %v", got, patternAt2)
	}
}

func TestSnap_VerticalBarcode_RedrawsWholeDotBars(t *testing.T) {
	src := newBarcode(2.4)
	src.vertical = true

	b := rasterize(t, Snap{}, src)

	if got := columnRuns(b, 60); !slices.Equal(got, patternAt2) {
		t.Errorf("Snap column 60 = %v, want %v", got, patternAt2)
	}
}

func TestSnap_JitteredWidths_RedrawsWholeDotBars(t *testing.T) {
	src := newBarcode(2.4)
	src.jitter = []float64{0.3, -0.3, 0.2, -0.1, 0.3, -0.2, 0.1, -0.3, 0.3, -0.3, 0.2, -0.1, 0.3, -0.2, 0.1, -0.3, 0.3, -0.3, 0.2, -0.1, 0.3}

	b := rasterize(t, Snap{}, src)

	if got := rowRuns(b, 60); !slices.Equal(got, patternAt2) {
		t.Errorf("Snap row 60 = %v, want %v", got, patternAt2)
	}
}

func TestSnap_DoubtfulBarcode_LeavesSupersampleOutput(t *testing.T) {
	tests := []struct {
		name string
		src  barcodeSource
	}{
		{"bars thinner than spaces", barcodeSource{width: 400, height: 120, modules: pattern, module: 2.4, x0: 60.3, y0: 20, y1: 100, shrink: 0.6}},
		{"module below MinModule", barcodeSource{width: 400, height: 120, modules: pattern, module: 1.2, x0: 60.3, y0: 20, y1: 100}},
		{"no quiet zone", barcodeSource{width: 400, height: 120, modules: pattern, module: 2.4, x0: 60.3, y0: 20, y1: 100, leftBar: 2}},
		{"shorter than 4 mm", barcodeSource{width: 400, height: 120, modules: pattern, module: 2.4, x0: 60.3, y0: 20, y1: 30}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := rasterize(t, Supersample{Factor: 8}, tt.src)

			got := rasterize(t, Snap{Factor: 8, MinModule: 2}, tt.src)

			if !bytes.Equal(got.Bits, want.Bits) {
				t.Errorf("Snap row 25 = %v, want Supersample row 25 = %v", rowRuns(got, 25), rowRuns(want, 25))
			}
		})
	}
}

func TestRasterizer_ZeroValue_UsesDocumentedDefaults(t *testing.T) {
	tests := []struct {
		name           string
		zero, explicit labelraster.Rasterizer
	}{
		{"Threshold", Threshold{}, Threshold{Level: 128}},
		{"Supersample", Supersample{}, Supersample{Factor: 4, Level: 128}},
		{"Snap", Snap{}, Snap{Factor: 8, MinModule: 2, QuietModules: 10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := newBarcode(2.4)
			want := rasterize(t, tt.explicit, src)

			got := rasterize(t, tt.zero, src)

			if !bytes.Equal(got.Bits, want.Bits) {
				t.Errorf("%s{} row 60 = %v, want %+v row 60 = %v", tt.name, rowRuns(got, 60), tt.explicit, rowRuns(want, 60))
			}
		})
	}
}

type errSource struct{}

var errRender = errors.New("render failed")

func (errSource) Render(int, int, labelraster.RenderOptions) (*image.Gray, error) {
	return nil, errRender
}

func TestRasterizer_SourceFails_ReturnsError(t *testing.T) {
	for _, s := range []labelraster.Rasterizer{Threshold{}, Supersample{}, Snap{}} {
		_, err := s.Rasterize(errSource{}, labelraster.DPI203, 400, 120)

		if !errors.Is(err, errRender) {
			t.Errorf("%T.Rasterize(failing source) error = %v, want %v", s, err, errRender)
		}
	}
}

func TestThreshold_HigherLevel_MarksMoreDotsBlack(t *testing.T) {
	src := newBarcode(2.4)

	dark := rasterize(t, Threshold{Level: 250}, src)
	light := rasterize(t, Threshold{Level: 5}, src)

	if d, l := blackDots(dark), blackDots(light); d <= l {
		t.Errorf("black dots with Level 250 = %d, Level 5 = %d, want more with 250", d, l)
	}
}
