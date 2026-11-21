package labelraster_test

import (
	"context"
	"log"
	"os"

	"github.com/sasaron/labelraster"
	"github.com/sasaron/labelraster/raster"
	"github.com/sasaron/labelraster/rawprint"
	"github.com/sasaron/labelraster/zpl"
)

// Print the first page of a carrier label on a 203 dpi Zebra printer.
func Example() {
	pdf, err := os.ReadFile("label.pdf")
	if err != nil {
		log.Fatal(err)
	}

	r, err := raster.NewRenderer()
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	doc, err := r.Open(pdf)
	if err != nil {
		log.Fatal(err)
	}
	defer doc.Close()

	b, err := doc.Rasterize(0, labelraster.DPI203, raster.Snap{})
	if err != nil {
		log.Fatal(err)
	}

	if err := rawprint.Send(context.Background(), "192.0.2.10", zpl.Encode(b, zpl.Options{})); err != nil {
		log.Fatal(err)
	}
}
