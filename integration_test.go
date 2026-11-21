package labelraster_test

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"

	"github.com/sasaron/labelraster"
	"github.com/sasaron/labelraster/pdfium"
	"github.com/sasaron/labelraster/raster"
	"github.com/sasaron/labelraster/sbpl"
	"github.com/sasaron/labelraster/zpl"
)

// shared is the Renderer for the PDF tests, since starting PDFium takes about 2 seconds.
var shared *pdfium.Renderer

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Short() {
		r, err := pdfium.NewRenderer()
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

// rasterizeLabel renders page 0 of testdata/file.
func rasterizeLabel(t *testing.T, file string, d labelraster.Density, s labelraster.Rasterizer) labelraster.Bitmap {
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
	defer func() {
		if err := doc.Close(); err != nil {
			t.Error(err)
		}
	}()
	b, err := doc.Rasterize(0, d, s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// scanLine reads one barcode across the row, or column, at atMM millimeters.
func scanLine(b labelraster.Bitmap, d labelraster.Density, atMM float64, vertical bool, reader gozxing.Reader) (string, error) {
	at := int(math.Round(atMM * float64(d)))
	n := b.Width
	if vertical {
		n = b.Height
	}
	img := image.NewGray(image.Rect(0, 0, n, 20))
	for i := range n {
		black := b.Black(i, at)
		if vertical {
			black = b.Black(at, i)
		}
		if !black {
			for y := range 20 {
				img.Pix[y*img.Stride+i] = 255
			}
		}
	}
	bmp, err := gozxing.NewBinaryBitmap(gozxing.NewGlobalHistgramBinarizer(gozxing.NewLuminanceSourceFromImage(img)))
	if err != nil {
		return "", err
	}
	res, err := reader.Decode(bmp, nil)
	if err != nil {
		return "", err
	}
	return res.GetText(), nil
}

func TestSnap_LabelPDF_BarcodesStayReadable(t *testing.T) {
	barcodes := []struct {
		name     string
		atMM     float64 // middle of the barcode, see testdata/label.json
		vertical bool
		reader   func() gozxing.Reader
		want     string // Codabar readers drop the start and stop characters
	}{
		{"code128", 27.5, false, oned.NewCode128Reader, "LR-0123456789"},
		{"code128-vertical", 77.5, true, oned.NewCode128Reader, "LR-0123456789"},
		{"codabar", 117.5, false, oned.NewCodaBarReader, "0123456789"},
	}
	for _, file := range []string{"label.pdf", "label-image.pdf"} {
		for _, d := range []labelraster.Density{labelraster.DPI203, labelraster.DPI305, labelraster.DPI609} {
			t.Run(fmt.Sprintf("%s/%v", file, float64(d)), func(t *testing.T) {
				b := rasterizeLabel(t, file, d, raster.Snap{})

				for _, bc := range barcodes {
					got, err := scanLine(b, d, bc.atMM, bc.vertical, bc.reader())
					if err != nil || got != bc.want {
						t.Errorf("scan %s = %q, %v; want %q, nil", bc.name, got, err, bc.want)
					}
				}
			})
		}
	}
}

func TestSBPL_LabelPDF_FitsCL4NX(t *testing.T) {
	tests := []struct {
		d     labelraster.Density
		model sbpl.Model
	}{
		{labelraster.DPI203, sbpl.CL4NX203},
		{labelraster.DPI305, sbpl.CL4NX305},
		{labelraster.DPI609, sbpl.CL4NX609},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(float64(tt.d)), func(t *testing.T) {
			b := rasterizeLabel(t, "label.pdf", tt.d, raster.Snap{})

			_, err := sbpl.Encode(b, tt.model, sbpl.Options{})

			if err != nil {
				t.Errorf("sbpl.Encode(%dx%d, %+v) error = %v, want nil", b.Width, b.Height, tt.model, err)
			}
		})
	}
}

func TestZPL_LabelPDF_SplitsIntoTwoBands(t *testing.T) {
	b := rasterizeLabel(t, "label.pdf", labelraster.DPI203, raster.Snap{})

	got := zpl.Encode(b, zpl.Options{})

	if n := bytes.Count(got, []byte("^GFA")); n != 2 {
		t.Errorf("zpl.Encode(%dx%d) has %d ^GFA fields, want 2", b.Width, b.Height, n)
	}
}

// lineRuns returns the run widths across the row, or column, at atMM millimeters,
// starting at the first black dot.
func lineRuns(b labelraster.Bitmap, d labelraster.Density, atMM float64, vertical bool) []int {
	at := int(math.Round(atMM * float64(d)))
	n := b.Width
	if vertical {
		n = b.Height
	}
	var out []int
	for i := range n {
		black := b.Black(i, at)
		if vertical {
			black = b.Black(at, i)
		}
		last := len(out) - 1
		switch {
		case last < 0 && !black:
		case last >= 0 && black == (last%2 == 0):
			out[last]++
		default:
			out = append(out, 1)
		}
	}
	if len(out)%2 == 0 && len(out) > 0 {
		out = out[:len(out)-1] // trailing white
	}
	return out
}

func TestLineRuns_BarsAndSpaces_ReturnsWidthsFromFirstBar(t *testing.T) {
	b := labelraster.NewBitmap(10, 1)
	for _, x := range []int{2, 3, 5, 8} {
		b.Set(x, 0, true)
	}

	got := lineRuns(b, labelraster.DPI203, 0, false)

	if want := []int{2, 1, 1, 2, 1}; !slices.Equal(got, want) {
		t.Errorf("lineRuns(..XX.X..X.) = %v, want %v", got, want)
	}
}

// timesModules returns modules with each element multiplied by dots.
func timesModules(modules []int, dots int) []int {
	out := make([]int, len(modules))
	for i, m := range modules {
		out[i] = m * dots
	}
	return out
}

func TestTimesModules_ThreeDots_MultipliesEachElement(t *testing.T) {
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
		for _, tt := range densities {
			t.Run(fmt.Sprintf("%s/%v", file, float64(tt.d)), func(t *testing.T) {
				b := rasterizeLabel(t, file, tt.d, raster.Snap{})

				for _, f := range fs {
					at := f.Y + f.Height/2
					if f.Vertical {
						at = f.X + f.Height/2
					}
					got, want := lineRuns(b, tt.d, at, f.Vertical), timesModules(f.Modules, tt.moduleDots)
					if !slices.Equal(got, want) {
						t.Errorf("Snap %s = %v, want %v", f.Name, got, want)
					}
				}
			})
		}
	}
}

func TestSnap_LabelPDF_LeavesTextUnchanged(t *testing.T) {
	want := rasterizeLabel(t, "label.pdf", labelraster.DPI203, raster.Supersample{Factor: 8})

	got := rasterizeLabel(t, "label.pdf", labelraster.DPI203, raster.Snap{Factor: 8})

	// The text line sits between 37 and 41 mm from the top, at 8 dots per mm.
	if !bytes.Equal(got.Bits[296*got.Stride:328*got.Stride], want.Bits[296*want.Stride:328*want.Stride]) {
		t.Error("Snap changed rows 296 to 328 (text) of label.pdf; want them equal to Supersample{Factor: 8}")
	}
}
