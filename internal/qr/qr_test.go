package qr

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rsc.io/qr"
)

func TestTerminal(t *testing.T) {
	var buf bytes.Buffer
	if err := Terminal(&buf, "http://192.168.1.23:53421/abcdefghijklmnopqrstuvwxyz/", "  "); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) < 12 {
		t.Fatalf("got %d lines, expected a full code", len(lines))
	}
	for _, l := range lines {
		if !strings.HasPrefix(l, "  \x1b[97;40m") || !strings.HasSuffix(l, "\x1b[0m") {
			t.Fatalf("line missing indent or color codes: %q", l)
		}
	}
}

// TestTerminalMatchesEncoder decodes the half-block output back into modules
// and checks every one against the encoder, quiet zone included.
func TestTerminalMatchesEncoder(t *testing.T) {
	text := "http://10.0.0.5:8080/abcdefghijklmnopqrstuvwxyz/"
	var buf bytes.Buffer
	if err := Terminal(&buf, text, ""); err != nil {
		t.Fatal(err)
	}
	code, err := qr.Encode(text, qr.L)
	if err != nil {
		t.Fatal(err)
	}
	size := code.Size + 2*quietZone
	wantLight := func(x, y int) bool {
		x, y = x-quietZone, y-quietZone
		if x < 0 || y < 0 || x >= code.Size || y >= code.Size {
			return true
		}
		return !code.Black(x, y)
	}

	out := strings.NewReplacer("\x1b[97;40m", "", "\x1b[0m", "").Replace(buf.String())
	rows := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(rows) != (size+1)/2 {
		t.Fatalf("got %d rows, want %d", len(rows), (size+1)/2)
	}
	for r, row := range rows {
		cells := []rune(row)
		if len(cells) != size {
			t.Fatalf("row %d has %d cells, want %d", r, len(cells), size)
		}
		for x, c := range cells {
			top := c == '█' || c == '▀'
			bottom := c == '█' || c == '▄'
			y := r * 2
			if top != wantLight(x, y) {
				t.Fatalf("module (%d,%d) mismatch", x, y)
			}
			if y+1 < size && bottom != wantLight(x, y+1) {
				t.Fatalf("module (%d,%d) mismatch", x, y+1)
			}
		}
	}
}

func TestWritePNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qr.png")
	if err := WritePNG(path, "http://example.test/"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("not a PNG file")
	}
}
