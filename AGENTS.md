# labelraster

Go library that rasterizes PDF labels to a thermal printer's dot grid and sends them as ZPL or SBPL. No cgo.

## Rules

- Subpackages import only the root `labelraster` package, the standard library and external modules. Never a sibling package.
- Tests and test data stay in each package (`_test.go`, `testdata/`). Cross-package tests go in the root `labelraster_test` package and own the PDF fixtures in `testdata/`.
- Only `pdfium` imports go-pdfium. Keep other packages free of it so they stay small.
- Keep the public API small; export only what users set or read.
- Tests follow https://github.com/mavka-ai/unit-tests-skills: one behavior per test, names `TestUnit_Condition_Outcome`, literal expected values, public API first, no reliance on helper defaults.
- Package names are short lowercase nouns. Identifiers do not repeat the package name (`zpl.Encode`, not `zpl.EncodeZPL`).
- Code and comments are in English. Comments are minimal.

## Do not commit

- SATO programming reference copies or screenshots of them.
- Brother's SBPL emulation guide (redistribution prohibited).
- Code from zebrafy (LGPL-3.0) or zplgrf (GPL-3.0).
