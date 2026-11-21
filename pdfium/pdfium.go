// Package pdfium renders PDF pages with PDFium compiled to WebAssembly (no cgo).
package pdfium

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"time"

	gopdfium "github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"

	"github.com/sasaron/labelraster"
)

var _ labelraster.Source = Page{}

// Renderer renders PDFs with PDFium compiled to WebAssembly.
// It runs one PDFium instance and is not safe for concurrent use.
type Renderer struct {
	pool     gopdfium.Pool
	instance gopdfium.Pdfium
}

// NewRenderer starts a PDFium instance. Call Close when done.
func NewRenderer() (*Renderer, error) {
	pool, err := webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1})
	if err != nil {
		return nil, fmt.Errorf("pdfium: init pdfium: %w", err)
	}
	instance, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("pdfium: get pdfium instance: %w", err)
	}
	return &Renderer{pool: pool, instance: instance}, nil
}

// Close stops the PDFium instance.
func (r *Renderer) Close() error {
	return errors.Join(r.instance.Close(), r.pool.Close())
}

// Document is an open PDF.
type Document struct {
	r   *Renderer
	doc references.FPDF_DOCUMENT
}

// Open opens a PDF. Call Close when done.
func (r *Renderer) Open(pdf []byte) (*Document, error) {
	res, err := r.instance.OpenDocument(&requests.OpenDocument{File: &pdf})
	if err != nil {
		return nil, fmt.Errorf("pdfium: open document: %w", err)
	}
	return &Document{r: r, doc: res.Document}, nil
}

// Close closes the document.
func (d *Document) Close() error {
	_, err := d.r.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: d.doc})
	if err != nil {
		return fmt.Errorf("pdfium: close document: %w", err)
	}
	return nil
}

// PageCount returns the number of pages.
func (d *Document) PageCount() (int, error) {
	res, err := d.r.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: d.doc})
	if err != nil {
		return 0, fmt.Errorf("pdfium: page count: %w", err)
	}
	return res.PageCount, nil
}

// Page returns page i (0-based). It is valid until the Document is closed.
func (d *Document) Page(i int) (Page, error) {
	p := requests.Page{ByIndex: &requests.PageByIndex{Document: d.doc, Index: i}}
	res, err := d.r.instance.GetPageSize(&requests.GetPageSize{Page: p})
	if err != nil {
		return Page{}, fmt.Errorf("pdfium: page %d: %w", i, err)
	}
	return Page{d: d, page: p, Width: res.Width, Height: res.Height}, nil
}

// Rasterize renders page i at density with s.
func (d *Document) Rasterize(i int, density labelraster.Density, s labelraster.Strategy) (labelraster.Bitmap, error) {
	p, err := d.Page(i)
	if err != nil {
		return labelraster.Bitmap{}, err
	}
	return s.Rasterize(p, density, density.DotsForPoints(p.Width), density.DotsForPoints(p.Height))
}

// Page is a page of a Document. Width and Height are in points.
type Page struct {
	d             *Document
	page          requests.Page
	Width, Height float64
}

// Render implements labelraster.Source.
func (p Page) Render(width, height int, opt labelraster.RenderOptions) (*image.Gray, error) {
	var flags enums.FPDF_RENDER_FLAG
	if opt.NoSmoothPath {
		flags |= enums.FPDF_RENDER_FLAG_RENDER_NO_SMOOTHPATH
	}
	if opt.NoSmoothText {
		flags |= enums.FPDF_RENDER_FLAG_RENDER_NO_SMOOTHTEXT
	}
	if opt.NoSmoothImage {
		flags |= enums.FPDF_RENDER_FLAG_RENDER_NO_SMOOTHIMAGE
	}
	res, err := p.d.r.instance.RenderPageInPixels(&requests.RenderPageInPixels{
		Page:        p.page,
		Width:       width,
		Height:      height,
		RenderFlags: flags,
		ImageFormat: requests.RenderImageFormatGrayscale,
	})
	if err != nil {
		return nil, fmt.Errorf("pdfium: render page: %w", err)
	}
	defer res.Cleanup()
	src, ok := res.Result.RenderedImage.(*image.Gray)
	if !ok {
		return nil, fmt.Errorf("pdfium: unexpected image type %T", res.Result.RenderedImage)
	}
	out := image.NewGray(image.Rect(0, 0, width, height))
	draw.Draw(out, out.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(out, src.Bounds().Sub(src.Bounds().Min), src, src.Bounds().Min, draw.Src)
	return out, nil
}
