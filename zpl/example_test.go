package zpl_test

import (
	"fmt"

	"github.com/sasaron/labelraster"
	"github.com/sasaron/labelraster/zpl"
)

func ExampleEncode() {
	b := labelraster.NewBitmap(8, 2)
	for x := range 8 {
		b.Set(x, 0, true)
	}
	fmt.Print(string(zpl.Encode(b, zpl.Options{})))
	// Output:
	// ^XA
	// ^FO0,0^GFA,2,2,1,FF00^FS
	// ^XZ
}
