// Package qr renders QR codes in the terminal and as PNG files.
package qr

import (
	"io"
	"os"
	"strings"

	"rsc.io/qr"

	"github.com/jaoskii/yoink-cli/internal/ansi"
)

const quietZone = 2 // light modules around the code; scanners need a margin

// Terminal writes the code using half-block characters, two modules per
// character row. Colors are explicit (white on black) so the code scans the
// same on light and dark terminal themes.
func Terminal(w io.Writer, text, indent string) error {
	code, err := qr.Encode(text, qr.L)
	if err != nil {
		return err
	}
	ansi.EnableVT(w)

	size := code.Size + 2*quietZone
	light := func(x, y int) bool {
		x, y = x-quietZone, y-quietZone
		if x < 0 || y < 0 || x >= code.Size || y >= code.Size {
			return true
		}
		return !code.Black(x, y)
	}

	var b strings.Builder
	for y := 0; y < size; y += 2 {
		b.WriteString(indent)
		b.WriteString("\x1b[97;40m") // bright white foreground, black background
		for x := 0; x < size; x++ {
			top := light(x, y)
			bottom := y+1 >= size || light(x, y+1)
			switch {
			case top && bottom:
				b.WriteString("█")
			case top:
				b.WriteString("▀")
			case bottom:
				b.WriteString("▄")
			default:
				b.WriteString(" ")
			}
		}
		b.WriteString("\x1b[0m\n")
	}
	_, err = io.WriteString(w, b.String())
	return err
}

// WritePNG saves the code as a PNG image at path.
func WritePNG(path, text string) error {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return err
	}
	code.Scale = 10
	return os.WriteFile(path, code.PNG(), 0o644)
}
