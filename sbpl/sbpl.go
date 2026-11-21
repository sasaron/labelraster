// Package sbpl encodes bitmaps as SATO SBPL graphic commands (experimental).
package sbpl

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/sasaron/labelraster"
)

const (
	esc = 0x1b
	stx = 0x02
	etx = 0x03

	maxSize = 999 // ESC+G sizes are three digits
)

// Model holds the ESC+G size limits of a printer.
type Model struct {
	MaxCrossBytes  int // bytes per row; one byte is 8 dots
	MaxLengthUnits int // one unit is 8 rows
}

// Limits of SATO CL4NX printers at 203, 305 and 609 dpi. Do not modify.
var (
	CL4NX203 = Model{MaxCrossBytes: 104, MaxLengthUnits: 400}
	CL4NX305 = Model{MaxCrossBytes: 156, MaxLengthUnits: 600}
	CL4NX609 = Model{MaxCrossBytes: 312, MaxLengthUnits: 999}
)

// ErrTooLarge is returned when a bitmap exceeds the Model limits.
var ErrTooLarge = errors.New("sbpl: bitmap exceeds model limits")

// Options configures Encode.
type Options struct {
	X, Y   int  // origin in dots
	Copies int  // defaults to 1
	Hex    bool // send ESC+GH instead of ESC+GB
	Frame  bool // wrap the job in STX and ETX
}

// Encode returns b as a single SBPL job.
func Encode(b labelraster.Bitmap, m Model, opt Options) ([]byte, error) {
	units := (b.Height + 7) / 8
	if b.Stride == 0 || units == 0 {
		return nil, errors.New("sbpl: empty bitmap")
	}
	if b.Stride > min(m.MaxCrossBytes, maxSize) || units > min(m.MaxLengthUnits, maxSize) {
		return nil, fmt.Errorf("%w: %d bytes x %d units", ErrTooLarge, b.Stride, units)
	}

	var buf bytes.Buffer
	if opt.Frame {
		buf.WriteByte(stx)
	}
	command(&buf, "A")
	command(&buf, "V%d", opt.Y)
	command(&buf, "H%d", opt.X)
	writeGraphic(&buf, b, opt.Hex)
	command(&buf, "Q%d", max(opt.Copies, 1))
	command(&buf, "Z")
	if opt.Frame {
		buf.WriteByte(etx)
	}
	return buf.Bytes(), nil
}

// writeGraphic writes the ESC+G command for b, padding the height to a multiple of 8.
func writeGraphic(buf *bytes.Buffer, b labelraster.Bitmap, hexData bool) {
	units := (b.Height + 7) / 8
	data := make([]byte, b.Stride*units*8)
	copy(data, b.Bits)
	if hexData {
		command(buf, "GH%03d%03d%X", b.Stride, units, data)
		return
	}
	command(buf, "GB%03d%03d", b.Stride, units)
	buf.Write(data)
}

func command(buf *bytes.Buffer, format string, args ...any) {
	buf.WriteByte(esc)
	fmt.Fprintf(buf, format, args...)
}
