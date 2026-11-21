package raster

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/sasaron/labelraster"
)

// shared is the Renderer for the PDF tests, since starting PDFium takes about 2 seconds.
var shared *Renderer

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Short() {
		r, err := NewRenderer()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		shared = r
	}
	code := m.Run()
	if shared != nil {
		if err := shared.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	os.Exit(code)
}

func openLabel(t *testing.T, file string) *Document {
	t.Helper()
	if shared == nil {
		t.Skip("skipping PDFium test in short mode")
	}
	data, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := shared.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := doc.Close(); err != nil {
			t.Error(err)
		}
	})
	return doc
}

func rasterizePage(t *testing.T, doc *Document, d labelraster.Density, s Strategy) labelraster.Bitmap {
	t.Helper()
	b, err := doc.Rasterize(0, d, s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// elementWidths returns the runs across the middle of the fixture's barcode.
func elementWidths(b labelraster.Bitmap, f fixture, d labelraster.Density) []int {
	dots := func(mm float64) int { return int(math.Round(mm * float64(d))) }
	if f.Vertical {
		return columnRuns(b, dots(f.X+f.Height/2))
	}
	return rowRuns(b, dots(f.Y+f.Height/2))
}

// timesModules returns modules with each element multiplied by dots.
func timesModules(modules []int, dots int) []int {
	out := make([]int, len(modules))
	for i, m := range modules {
		out[i] = m * dots
	}
	return out
}

func TestTimesModules(t *testing.T) {
	got := timesModules([]int{1, 2, 4}, 3)

	if want := []int{3, 6, 12}; !slices.Equal(got, want) {
		t.Errorf("timesModules([1 2 4], 3) = %v, want %v", got, want)
	}
}

func TestSnap_LabelPDF_RedrawsBarcodesWithWholeDotModules(t *testing.T) {
	fs := loadFixtures(t)
	densities := []struct {
		d          labelraster.Density
		moduleDots int // 0.3 mm rounded to whole dots
	}{
		{labelraster.DPI203, 2},
		{labelraster.DPI305, 4},
		{labelraster.DPI609, 7},
	}
	for _, file := range []string{"label.pdf", "label-image.pdf"} {
		t.Run(file, func(t *testing.T) {
			doc := openLabel(t, file)
			for _, tt := range densities {
				t.Run(fmt.Sprint(float64(tt.d)), func(t *testing.T) {
					b := rasterizePage(t, doc, tt.d, Snap{})

					for _, f := range fs {
						got, want := elementWidths(b, f, tt.d), timesModules(f.Modules, tt.moduleDots)
						if !slices.Equal(got, want) {
							t.Errorf("Snap %s = %v, want %v", f.Name, got, want)
						}
					}
				})
			}
		})
	}
}

func TestSnap_LabelPDF_LeavesTextUnchanged(t *testing.T) {
	doc := openLabel(t, "label.pdf")
	want := rasterizePage(t, doc, labelraster.DPI203, Supersample{Factor: 8})

	got := rasterizePage(t, doc, labelraster.DPI203, Snap{Factor: 8})

	// The text line sits between 37 and 41 mm from the top, at 8 dots per mm.
	if !bytes.Equal(got.Bits[296*got.Stride:328*got.Stride], want.Bits[296*want.Stride:328*want.Stride]) {
		t.Error("Snap changed rows 296 to 328 (text) of label.pdf; want them equal to Supersample{Factor: 8}")
	}
}

func TestRenderer_Open_NotAPDF_ReturnsError(t *testing.T) {
	if shared == nil {
		t.Skip("skipping PDFium test in short mode")
	}

	_, err := shared.Open([]byte("not a pdf"))

	if err == nil {
		t.Error(`Open("not a pdf") error = nil, want an error`)
	}
}

func TestDocument_PageCount_OnePageLabel_ReturnsOne(t *testing.T) {
	doc := openLabel(t, "label.pdf")

	got, err := doc.PageCount()

	if err != nil || got != 1 {
		t.Errorf("PageCount() = %d, %v, want 1, nil", got, err)
	}
}

func TestDocument_Page_OutOfRange_ReturnsError(t *testing.T) {
	doc := openLabel(t, "label.pdf")

	_, err := doc.Page(1)

	if err == nil {
		t.Error("Page(1) error = nil, want an error")
	}
}

func TestDocument_Rasterize_OutOfRange_ReturnsError(t *testing.T) {
	doc := openLabel(t, "label.pdf")

	_, err := doc.Rasterize(1, labelraster.DPI203, Snap{})

	if err == nil {
		t.Error("Rasterize(1) error = nil, want an error")
	}
}

func TestDocument_Rasterize_4x6Label_Returns813x1219Dots(t *testing.T) {
	doc := openLabel(t, "label.pdf")

	b, err := doc.Rasterize(0, labelraster.DPI203, Threshold{})

	if err != nil || b.Width != 813 || b.Height != 1219 {
		t.Errorf("Rasterize(0, DPI203) = %dx%d, %v, want 813x1219, nil", b.Width, b.Height, err)
	}
}
