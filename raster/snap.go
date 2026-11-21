package raster

import (
	"image"
	"math"
	"slices"

	"github.com/sasaron/labelraster"
)

// Snap supersamples the page, finds 1D barcodes and redraws them so that every
// element width is a whole number of dots and same-sized elements match.
// Areas outside barcodes are rendered as Supersample renders them.
//
// A barcode is left unchanged when its elements do not fit a few width
// classes, when its module would be smaller than MinModule, or when it has
// no quiet zone.
type Snap struct {
	Factor       int // supersampling factor, defaults to about 64 pixels per mm
	MinModule    int // minimum module in dots, defaults to 2
	QuietModules int // quiet zone kept when a barcode grows, in modules, defaults to 10
}

const (
	minBars       = 10
	minHeightMM   = 4.0
	maxElementMM  = 3.0
	maxClasses    = 4
	maxClassRatio = 4.5
)

// Rasterize implements labelraster.Rasterizer.
func (s Snap) Rasterize(src labelraster.Source, d labelraster.Density, width, height int) (labelraster.Bitmap, error) {
	return newSnapper(s, d).rasterize(src, width, height)
}

// snapper is Snap with defaults applied and sizes converted to dots.
type snapper struct {
	factor, minModule, quietModules int
	minHeight                       int // dots
	maxElement                      int // supersampled pixels
}

func newSnapper(s Snap, d labelraster.Density) snapper {
	sn := snapper{factor: s.Factor, minModule: s.MinModule, quietModules: s.QuietModules}
	if sn.factor < 1 {
		sn.factor = max(int(math.Ceil(64/float64(d))), 2)
	}
	if sn.minModule < 1 {
		sn.minModule = 2
	}
	if sn.quietModules < 1 {
		sn.quietModules = 10
	}
	sn.minHeight = int(math.Ceil(minHeightMM * float64(d)))
	sn.maxElement = int(math.Ceil(maxElementMM * float64(d) * float64(sn.factor)))
	return sn
}

// barcode is a 1D barcode found in the page, in output dots.
type barcode struct {
	rect   image.Rectangle // bounds
	start  float64         // left edge
	widths []float64       // element widths, bar first
}

func (s snapper) rasterize(src labelraster.Source, width, height int) (labelraster.Bitmap, error) {
	img, err := src.Render(width*s.factor, height*s.factor, labelraster.RenderOptions{})
	if err != nil {
		return labelraster.Bitmap{}, err
	}
	b := downsample(img, width, height, s.factor, defaultLevel)
	for _, bc := range s.find(img, width, height) {
		s.redraw(b, bc)
	}

	// Vertical barcodes are found and redrawn on the transposed page.
	vertical := s.find(transposeGray(img), height, width)
	if len(vertical) == 0 {
		return b, nil
	}
	tb := transposeBitmap(b)
	for _, bc := range vertical {
		s.redraw(tb, bc)
	}
	return transposeBitmap(tb), nil
}

type segment struct {
	x0, x1 int // supersampled pixels
	widths []int
}

type region struct {
	y0, y1 int     // dots, inclusive
	x0, x1 float64 // sums of segment edges
	sums   []float64
	rows   int
}

func (r *region) near(sg segment, f int) bool {
	n := float64(r.rows)
	return len(r.sums) == len(sg.widths) &&
		math.Abs(r.x0/n-float64(sg.x0)) <= float64(f) && math.Abs(r.x1/n-float64(sg.x1)) <= float64(f)
}

// find returns horizontal barcodes in img, which is width*f x height*f.
func (s snapper) find(img *image.Gray, width, height int) []barcode {
	f := s.factor
	var open, done []*region
	for y := range height {
		segs := s.segments(img, y*f+f/2, width*f)
		var next []*region
		for _, sg := range segs {
			var match *region
			for _, r := range open {
				if r.near(sg, f) {
					match = r
					break
				}
			}
			if match == nil {
				match = &region{y0: y, sums: make([]float64, len(sg.widths))}
			} else {
				open = slices.DeleteFunc(open, func(r *region) bool { return r == match })
			}
			match.y1 = y
			match.rows++
			match.x0 += float64(sg.x0)
			match.x1 += float64(sg.x1)
			for i, w := range sg.widths {
				match.sums[i] += float64(w)
			}
			next = append(next, match)
		}
		done = append(done, open...)
		open = next
	}
	done = append(done, open...)

	var out []barcode
	for _, r := range done {
		if r.y1-r.y0+1 < s.minHeight {
			continue
		}
		n := float64(r.rows)
		x0, x1 := r.x0/n/float64(f), r.x1/n/float64(f)
		bc := barcode{
			rect:   image.Rect(int(math.Floor(x0)), r.y0, int(math.Ceil(x1)), r.y1+1),
			start:  x0,
			widths: make([]float64, len(r.sums)),
		}
		for i, sum := range r.sums {
			bc.widths[i] = sum / n / float64(f)
		}
		out = append(out, bc)
	}
	return out
}

// segments returns runs of narrow alternating elements on one supersampled row.
func (s snapper) segments(img *image.Gray, yy, width int) []segment {
	r := img.Bounds()
	if yy >= r.Dy() {
		return nil
	}
	var runs []int // alternating, starting with white
	black, n := false, 0
	for x := range min(width, r.Dx()) {
		bl := img.GrayAt(r.Min.X+x, r.Min.Y+yy).Y < defaultLevel
		if bl != black {
			runs = append(runs, n)
			black, n = bl, 0
		}
		n++
	}
	runs = append(runs, n)

	var segs []segment
	var cur []int
	start, x := 0, 0
	flush := func() {
		if len(cur) > 0 && len(cur)%2 == 0 {
			cur = cur[:len(cur)-1] // drop trailing space
		}
		if (len(cur)+1)/2 >= minBars {
			w := 0
			for _, c := range cur {
				w += c
			}
			segs = append(segs, segment{x0: start, x1: start + w, widths: slices.Clone(cur)})
		}
		cur = nil
	}
	for i, w := range runs {
		isBlack := i%2 == 1
		if w > s.maxElement || w == 0 {
			flush()
		} else if isBlack || len(cur) > 0 {
			if len(cur) == 0 {
				start = x
			}
			cur = append(cur, w)
		}
		x += w
	}
	flush()
	return segs
}

// redraw draws bc into b with whole-dot element widths. It leaves b unchanged
// when bc is doubtful or cannot keep its quiet zone.
func (s snapper) redraw(b labelraster.Bitmap, bc barcode) {
	classes := classify(bc.widths)
	narrow := classes[0]
	if len(classes) > maxClasses || classes[len(classes)-1]/narrow > maxClassRatio {
		return
	}
	ratio := make([]float64, len(bc.widths))
	total := 0.0
	for i, w := range bc.widths {
		c := nearest(classes, w)
		if math.Abs(w-c) > 0.35*narrow {
			return
		}
		ratio[i] = c / narrow
		total += w
	}

	center := bc.start + total/2
	mid := (bc.rect.Min.Y + bc.rect.Max.Y) / 2
	leftGap := gap(b, mid, bc.rect.Min.X-1, -1)
	rightGap := gap(b, mid, bc.rect.Max.X, 1)
	if float64(min(leftGap, rightGap)) < 3*narrow {
		return
	}

	for m := int(math.Round(narrow)); m >= s.minModule; m-- {
		dots := make([]int, len(ratio))
		width := 0
		for i, r := range ratio {
			dots[i] = max(int(math.Round(r*float64(m))), 1)
			width += dots[i]
		}
		x0 := int(math.Round(center - float64(width)/2))
		quiet := s.quietModules * m
		left := leftGap - max(bc.rect.Min.X-x0, 0)
		right := rightGap - max(x0+width-bc.rect.Max.X, 0)
		if left < min(leftGap, quiet) || right < min(rightGap, quiet) {
			continue
		}
		area := image.Rect(min(bc.rect.Min.X, x0), bc.rect.Min.Y, max(bc.rect.Max.X, x0+width), bc.rect.Max.Y)
		drawBars(b, area, x0, dots)
		return
	}
}

// drawBars clears rect in b and draws elements of the given widths from x0, bar first.
func drawBars(b labelraster.Bitmap, rect image.Rectangle, x0 int, widths []int) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			b.Set(x, y, false)
		}
		x := x0
		for i, w := range widths {
			for range w {
				b.Set(x, y, i%2 == 0)
				x++
			}
		}
	}
}

// classify groups widths into classes and returns the class means, smallest first.
func classify(widths []float64) []float64 {
	sorted := slices.Sorted(slices.Values(widths))
	var classes []float64
	sum, n := sorted[0], 1.0
	for _, w := range sorted[1:] {
		if w > sum/n*1.25 {
			classes = append(classes, sum/n)
			sum, n = 0, 0
		}
		sum += w
		n++
	}
	return append(classes, sum/n)
}

func nearest(classes []float64, w float64) float64 {
	best := classes[0]
	for _, c := range classes {
		if math.Abs(c-w) < math.Abs(best-w) {
			best = c
		}
	}
	return best
}

// gap counts white dots on row y starting at x and moving by step.
func gap(b labelraster.Bitmap, y, x, step int) int {
	n := 0
	for ; x >= 0 && x < b.Width && !b.Black(x, y); x += step {
		n++
	}
	return n
}

func transposeGray(img *image.Gray) *image.Gray {
	r := img.Bounds()
	t := image.NewGray(image.Rect(0, 0, r.Dy(), r.Dx()))
	for y := range r.Dy() {
		row := img.Pix[img.PixOffset(r.Min.X, r.Min.Y+y):]
		for x := range r.Dx() {
			t.Pix[x*t.Stride+y] = row[x]
		}
	}
	return t
}

func transposeBitmap(b labelraster.Bitmap) labelraster.Bitmap {
	t := labelraster.NewBitmap(b.Height, b.Width)
	for y := range b.Height {
		for x := range b.Width {
			if b.Black(x, y) {
				t.Set(y, x, true)
			}
		}
	}
	return t
}
