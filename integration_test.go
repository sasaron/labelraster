package labelraster_test

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"

	"github.com/sasaron/labelraster"
	"github.com/sasaron/labelraster/raster"
	"github.com/sasaron/labelraster/sbpl"
	"github.com/sasaron/labelraster/zpl"
)

// shared is the Renderer for the PDF tests, since starting PDFium takes about 2 seconds.
var shared *raster.Renderer

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Short() {
		r, err := raster.NewRenderer()
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

// rasterizeLabel renders page 0 of raster/testdata/file.
func rasterizeLabel(t *testing.T, file string, d labelraster.Density, s raster.Strategy) labelraster.Bitmap {
	t.Helper()
	if shared == nil {
		t.Skip("skipping PDFium test in short mode")
	}
	data, err := os.ReadFile(filepath.Join("raster", "testdata", file))
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
		atMM     float64 // middle of the barcode, see raster/testdata/label.json
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
