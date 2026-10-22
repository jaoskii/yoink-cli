//go:build windows

package ansi

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// EnableVT turns on ANSI escape handling for the Windows console, which the
// legacy console host leaves off by default. Windows Terminal ignores it.
func EnableVT(w io.Writer) {
	f, ok := w.(*os.File)
	if !ok {
		return
	}
	h := windows.Handle(f.Fd())
	var mode uint32
	if windows.GetConsoleMode(h, &mode) != nil {
		return
	}
	_ = windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
