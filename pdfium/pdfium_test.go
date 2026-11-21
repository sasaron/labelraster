package pdfium

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/signintech/gopdf"

	"github.com/sasaron/labelraster"
)

// shared is the Renderer for these tests, since starting PDFium takes about 2 seconds.
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

// blank is a Rasterizer that returns a white bitmap of the requested size.
type blank struct{}

func (blank) Rasterize(_ labelraster.Source, _ labelraster.Density, w, h int) (labelraster.Bitmap, error) {
	return labelraster.NewBitmap(w, h), nil
}

// openBlank opens a one-page PDF of widthPt x heightPt points.
func openBlank(t *testing.T, widthPt, heightPt float64) *Document {
	t.Helper()
	if shared == nil {
		t.Skip("skipping PDFium test in short mode")
	}
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{Unit: gopdf.UnitPT, PageSize: gopdf.Rect{W: widthPt, H: heightPt}})
	pdf.AddPage()
	data, err := pdf.GetBytesPdfReturnErr()
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

func TestRenderer_Open_NotAPDF_ReturnsError(t *testing.T) {
	if shared == nil {
		t.Skip("skipping PDFium test in short mode")
	}

	_, err := shared.Open([]byte("not a pdf"))

	if err == nil {
		t.Error(`Open("not a pdf") error = nil, want an error`)
	}
}

func TestDocument_PageCount_OnePage_ReturnsOne(t *testing.T) {
	doc := openBlank(t, 288, 432)

	got, err := doc.PageCount()

	if err != nil || got != 1 {
		t.Errorf("PageCount() = %d, %v, want 1, nil", got, err)
	}
}

func TestDocument_Page_4x6Page_ReturnsSizeInPoints(t *testing.T) {
	doc := openBlank(t, 288, 432)

	p, err := doc.Page(0)

	if err != nil || p.Width != 288 || p.Height != 432 {
		t.Errorf("Page(0) = %vx%v pt, %v, want 288x432, nil", p.Width, p.Height, err)
	}
}

func TestDocument_Page_OutOfRange_ReturnsError(t *testing.T) {
	doc := openBlank(t, 288, 432)

	_, err := doc.Page(1)

	if err == nil {
		t.Error("Page(1) error = nil, want an error")
	}
}

func TestDocument_Rasterize_OutOfRange_ReturnsError(t *testing.T) {
	doc := openBlank(t, 288, 432)

	_, err := doc.Rasterize(1, labelraster.DPI203, blank{})

	if err == nil {
		t.Error("Rasterize(1) error = nil, want an error")
	}
}

func TestDocument_Rasterize_4x6PageAt203dpi_Returns813x1219Dots(t *testing.T) {
	doc := openBlank(t, 288, 432)

	b, err := doc.Rasterize(0, labelraster.DPI203, blank{})

	if err != nil || b.Width != 813 || b.Height != 1219 {
		t.Errorf("Rasterize(0, DPI203) = %dx%d, %v, want 813x1219, nil", b.Width, b.Height, err)
	}
}

func TestPage_Render_BlankPage_IsWhite(t *testing.T) {
	doc := openBlank(t, 72, 72)
	p, err := doc.Page(0)
	if err != nil {
		t.Fatal(err)
	}

	img, err := p.Render(10, 10, labelraster.RenderOptions{})

	if err != nil || img.GrayAt(5, 5).Y != 255 {
		t.Errorf("Render(10, 10) pixel (5, 5) = %v, %v, want 255, nil", img.GrayAt(5, 5).Y, err)
	}
}
