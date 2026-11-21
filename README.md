# labelraster

Go library for printing PDF labels on thermal printers without breaking barcodes.

Carriers return shipping labels as PDFs. Most thermal printers cannot print PDFs, so the PDF is rasterized and sent as an image. At 203 dpi a barcode module rarely lands on the dot grid, and ordinary rasterization turns bars of the same width into 2 and 3 dots. labelraster finds 1D barcodes on the page and redraws them so every bar and space is a whole number of dots.

- Pure Go, no cgo. The `pdfium` package renders PDFs with [go-pdfium](https://github.com/klippa-app/go-pdfium) in WebAssembly mode. The other packages do not depend on it, so you can bring your own renderer.
- Output as ZPL (`^GFA`) or SBPL (`ESC+G`, experimental).
- Horizontal and vertical barcodes, embedded as vector paths or images.

## Install

```sh
go get github.com/sasaron/labelraster
```

Requires Go 1.26 or later.

## Example

```go
r, err := pdfium.NewRenderer()
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

err = rawprint.Send(ctx, "192.0.2.10", zpl.Encode(b, zpl.Options{}))
```

## Packages

| Package | Description |
| --- | --- |
| `labelraster` | `Bitmap`, `Density`, and the `Source` and `Rasterizer` interfaces |
| `pdfium` | Renders PDF pages with PDFium (WebAssembly). Adds about 12 MB to a binary |
| `raster` | Rasterizers that turn a `Source` into a `Bitmap`: `Threshold`, `Supersample`, `Snap` |
| `zpl` | Encodes a bitmap as ZPL |
| `sbpl` | Encodes a bitmap as SBPL |
| `rawprint` | Sends data to TCP port 9100 |

## Rasterizers

| Rasterizer | How | Barcode widths |
| --- | --- | --- |
| `Threshold` | Render at the dot grid and threshold | Uneven |
| `Supersample` | Render at N times the grid and average | Uneven |
| `Snap` | Supersample, detect 1D barcodes, redraw with whole-dot modules | Even |

`Snap` keeps the barcode centered. When rounding makes it wider, it keeps at least `QuietModules` modules of quiet zone, or falls back to a narrower module. Everything outside barcodes is rendered as `Supersample` renders it.

Measured on the test label in `testdata` (0.3 mm module, Code 128 and Codabar):

| Density | `Threshold` 1-module bars | `Snap` 1-module bars |
| --- | --- | --- |
| 8 dots/mm (203 dpi) | 1 to 3 dots | 2 dots |
| 12 dots/mm (305 dpi) | 2 to 4 dots | 4 dots |
| 24 dots/mm (609 dpi) | 6 to 8 dots | 7 dots |

`Snap` changes the printed width of a barcode slightly, because the module is rounded to whole dots. It leaves a barcode as `Supersample` renders it when the module would be smaller than `MinModule` dots, when element widths do not fit a few classes (for example bars drawn thinner than spaces), or when there is no quiet zone.

## Limitations

- Not yet tested on a printer, with a scanner, or with real carrier PDFs. ZPL output is checked by rendering it back with [zebrash](https://github.com/ingridhq/zebrash); SBPL output only against the examples in the SATO programming reference.
- Text in PDFs without embedded fonts may not render in WebAssembly mode (not verified).
- 2D codes are not modified.

## License

MIT.

The `pdfium` package embeds a WebAssembly build of [PDFium](https://pdfium.googlesource.com/pdfium/) through go-pdfium. PDFium has its own license (BSD-style and Apache-2.0); include its notices when you distribute a binary that imports `pdfium`. This project is not affiliated with Google or the PDFium authors.
