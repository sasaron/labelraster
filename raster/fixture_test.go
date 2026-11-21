package raster

import (
	"encoding/json"
	"flag"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/codabar"
	"github.com/boombuler/barcode/code128"
	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/goregular"
)

var update = flag.Bool("update", false, "regenerate testdata")

const mm = 72 / 25.4

// fixture is a barcode drawn in a test PDF. Positions are in millimeters.
type fixture struct {
	Name     string  `json:"name"`
	Content  string  `json:"content"`
	Modules  []int   `json:"modules"`
	Module   float64 `json:"module_mm"`
	X        float64 `json:"x_mm"`
	Y        float64 `json:"y_mm"`
	Height   float64 `json:"height_mm"`
	Vertical bool    `json:"vertical"`
}

func modules(bc barcode.Barcode) []int {
	var out []int
	b := bc.Bounds()
	prev, n := true, 0
	for x := b.Min.X; x < b.Max.X; x++ {
		r, _, _, _ := bc.At(x, b.Min.Y).RGBA()
		black := r < 0x8000
		if x > b.Min.X && black != prev {
			out = append(out, n)
			n = 0
		}
		prev = black
		n++
	}
	return append(out, n)
}

func fixtures(t *testing.T) []fixture {
	t.Helper()
	c128, err := code128.Encode("LR-0123456789")
	if err != nil {
		t.Fatal(err)
	}
	cbar, err := codabar.Encode("A0123456789A")
	if err != nil {
		t.Fatal(err)
	}
	return []fixture{
		{Name: "code128", Content: "LR-0123456789", Modules: modules(c128), Module: 0.3, X: 10.17, Y: 20, Height: 15},
		{Name: "code128-vertical", Content: "LR-0123456789", Modules: modules(c128), Module: 0.3, X: 70, Y: 50.3, Height: 15, Vertical: true},
		{Name: "codabar", Content: "A0123456789A", Modules: modules(cbar), Module: 0.3, X: 10.17, Y: 110, Height: 15},
	}
}

// writeLabel draws the fixtures on a 4x6 inch page, as rectangles or as one embedded image.
func writeLabel(t *testing.T, path string, fs []fixture, asImage bool) {
	t.Helper()
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{Unit: gopdf.UnitPT, PageSize: gopdf.Rect{W: 288, H: 432}})
	pdf.AddPage()
	pdf.SetNoCompression()
	pdf.SetFillColor(0, 0, 0)
	for _, f := range fs {
		if asImage {
			drawImage(t, &pdf, f)
			continue
		}
		pos := f.X
		if f.Vertical {
			pos = f.Y
		}
		for i, m := range f.Modules {
			w := float64(m) * f.Module
			if i%2 == 0 {
				if f.Vertical {
					pdf.RectFromUpperLeftWithStyle(f.X*mm, pos*mm, f.Height*mm, w*mm, "F")
				} else {
					pdf.RectFromUpperLeftWithStyle(pos*mm, f.Y*mm, w*mm, f.Height*mm, "F")
				}
			}
			pos += w
		}
	}
	if err := pdf.AddTTFFontData("goregular", goregular.TTF); err != nil {
		t.Fatal(err)
	}
	if err := pdf.SetFont("goregular", "", 8); err != nil {
		t.Fatal(err)
	}
	pdf.SetXY(10*mm, 40*mm)
	if err := pdf.Text("IIIIIIIIIIII lllllllllll 1111111111 labelraster"); err != nil {
		t.Fatal(err)
	}
	data, err := pdf.GetBytesPdfReturnErr()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// drawImage embeds the barcode as a 1-pixel-per-module image.
func drawImage(t *testing.T, pdf *gopdf.GoPdf, f fixture) {
	t.Helper()
	total := 0
	for _, m := range f.Modules {
		total += m
	}
	img := image.NewGray(image.Rect(0, 0, total, 1))
	x := 0
	for i, m := range f.Modules {
		for range m {
			if i%2 == 0 {
				img.SetGray(x, 0, color.Gray{})
			} else {
				img.SetGray(x, 0, color.Gray{Y: 255})
			}
			x++
		}
	}
	length := float64(total) * f.Module
	var src image.Image = img
	rect := &gopdf.Rect{W: length * mm, H: f.Height * mm}
	if f.Vertical {
		v := image.NewGray(image.Rect(0, 0, 1, total))
		for i := range total {
			v.SetGray(0, i, img.GrayAt(i, 0))
		}
		src = v
		rect = &gopdf.Rect{W: f.Height * mm, H: length * mm}
	}
	if err := pdf.ImageFrom(src, f.X*mm, f.Y*mm, rect); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateFixtures(t *testing.T) {
	if !*update {
		t.Skip("run with -update to regenerate testdata")
	}
	fs := fixtures(t)
	writeLabel(t, filepath.Join("testdata", "label.pdf"), fs, false)
	writeLabel(t, filepath.Join("testdata", "label-image.pdf"), fs, true)
	data, err := json.MarshalIndent(fs, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("testdata", "label.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestFixturesUpToDate checks that testdata matches fixtures(); regenerate with
// go test ./raster -run TestGenerateFixtures -update
func TestFixtures_Testdata_MatchesGenerator(t *testing.T) {
	got, want := loadFixtures(t), fixtures(t)
	if !reflect.DeepEqual(got, want) {
		t.Error("testdata/label.json is out of date; run go test ./raster -run TestGenerateFixtures -update")
	}
}

func loadFixtures(t *testing.T) []fixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "label.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fs []fixture
	if err := json.Unmarshal(data, &fs); err != nil {
		t.Fatal(err)
	}
	return fs
}
