package labelraster

import "image"

// RenderOptions disables anti-aliasing per object type.
type RenderOptions struct {
	NoSmoothPath  bool
	NoSmoothText  bool
	NoSmoothImage bool
}

// Source renders a page scaled to width x height pixels.
type Source interface {
	Render(width, height int, opt RenderOptions) (*image.Gray, error)
}

// Strategy turns a page into a bitmap of width x height dots.
// Density is for strategies whose options are physical sizes.
type Strategy interface {
	Rasterize(src Source, d Density, width, height int) (Bitmap, error)
}
