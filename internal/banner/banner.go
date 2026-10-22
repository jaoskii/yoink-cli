// Package banner draws the yoink wordmark.
package banner

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/jaoskii/yoink-cli/internal/ansi"
)

const tagline = "share files over Wi-Fi with a QR code"

// "ANSI Shadow" style glyphs, only the ones the wordmark needs. Each glyph
// carries its own spacing, so glyphs are joined without a separator.
var font = map[rune][6]string{
	'Y': {"██╗   ██╗", "╚██╗ ██╔╝", " ╚████╔╝ ", "  ╚██╔╝  ", "   ██║   ", "   ╚═╝   "},
	'O': {" ██████╗ ", "██╔═══██╗", "██║   ██║", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
	'I': {"██╗", "██║", "██║", "██║", "██║", "╚═╝"},
	'N': {"███╗   ██╗", "████╗  ██║", "██╔██╗ ██║", "██║╚██╗██║", "██║ ╚████║", "╚═╝  ╚═══╝"},
	'K': {"██╗  ██╗", "██║ ██╔╝", "█████╔╝ ", "██╔═██╗ ", "██║  ██╗", "╚═╝  ╚═╝"},
}

// 256-color palette (not 24-bit) so macOS Terminal.app renders it too:
// lime → green → teal → cyan, left to right. Shadow strokes stay gray.
var palette = []int{154, 118, 84, 48, 49, 43, 44, 45}

const shadowColor = 242

// Print writes the wordmark and tagline to w. Colors only on a capable
// terminal; terminals too narrow for the wordmark get a one-line title.
func Print(w io.Writer, version string) {
	_, _ = io.WriteString(w, render(version, ansi.Supported(w), ansi.Width(w)))
}

func render(version string, color bool, width int) string {
	rows := wordmark("YOINK")
	cols := 0
	for _, r := range rows {
		cols = max(cols, utf8.RuneCountInString(r))
	}

	var b strings.Builder
	b.WriteString("\n")
	if width > 0 && width < cols+4 {
		b.WriteString("  " + paint("yoink", color, "1;38;5;48") + "\n")
	} else {
		for _, row := range rows {
			b.WriteString("  ")
			x := 0
			for _, ch := range row {
				switch {
				case !color || ch == ' ':
					b.WriteRune(ch)
				case ch == '█':
					fmt.Fprintf(&b, "\x1b[38;5;%dm%c", palette[x*len(palette)/cols], ch)
				default: // box-drawing shadow
					fmt.Fprintf(&b, "\x1b[38;5;%dm%c", shadowColor, ch)
				}
				x++
			}
			if color {
				b.WriteString("\x1b[0m")
			}
			b.WriteString("\n")
		}
	}

	tag := tagline
	if version != "" {
		tag += " · " + version
	}
	b.WriteString("\n  " + paint(tag, color, "2") + "\n")
	return b.String()
}

func wordmark(text string) []string {
	rows := make([]string, len(font['Y']))
	for _, ch := range text {
		for r, line := range font[ch] {
			rows[r] += line
		}
	}
	for r := range rows {
		rows[r] = strings.TrimRight(rows[r], " ")
	}
	return rows
}

func paint(s string, color bool, code string) string {
	if !color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}
