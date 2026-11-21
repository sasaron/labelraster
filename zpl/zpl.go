// Package zpl encodes bitmaps as Zebra ZPL graphic fields.
package zpl

import (
	"bytes"
	"fmt"

	"github.com/sasaron/labelraster"
)

// maxFieldBytes is the ^GF graphic field count limit; larger bitmaps are split into bands.
const maxFieldBytes = 99999

// Options configures Encode.
type Options struct {
	X, Y      int  // origin in dots
	Copies    int  // emits ^PQ when greater than 1
	LabelSize bool // emits ^PW and ^LL from the bitmap size
}

// Encode returns b as a single ZPL label.
func Encode(b labelraster.Bitmap, opt Options) []byte {
	var buf bytes.Buffer
	buf.WriteString("^XA\n")
	if opt.LabelSize {
		fmt.Fprintf(&buf, "^PW%d\n^LL%d\n", opt.X+b.Width, opt.Y+b.Height)
	}
	if b.Width > 0 && b.Height > 0 {
		rowsPerBand := max(maxFieldBytes/b.Stride, 1)
		for y0 := 0; y0 < b.Height; y0 += rowsPerBand {
			y1 := min(y0+rowsPerBand, b.Height)
			band := b.Bits[y0*b.Stride : y1*b.Stride]
			n := len(band)
			fmt.Fprintf(&buf, "^FO%d,%d^GFA,%d,%d,%d,%X^FS\n", opt.X, opt.Y+y0, n, n, b.Stride, band)
		}
	}
	if opt.Copies > 1 {
		fmt.Fprintf(&buf, "^PQ%d\n", opt.Copies)
	}
	buf.WriteString("^XZ\n")
	return buf.Bytes()
}
