// Package ansi detects whether a writer is a terminal that should get colors.
package ansi

import (
	"io"
	"os"

	"golang.org/x/term"
)

// Supported reports whether w is a color-capable terminal, enabling ANSI
// handling on Windows as a side effect. NO_COLOR (https://no-color.org) and
// TERM=dumb turn colors off.
func Supported(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return false
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	EnableVT(f)
	return true
}

// Width returns the column width of the terminal behind w, or 0 if unknown.
func Width(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 0
	}
	return width
}
