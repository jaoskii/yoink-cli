package banner

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestRenderPlain(t *testing.T) {
	out := render("v1.2.3", false, 0)
	if strings.Contains(out, "\x1b[") {
		t.Error("plain render contains escape codes")
	}
	if got := strings.Count(out, "█"); got < 40 {
		t.Errorf("wordmark looks incomplete: %d blocks", got)
	}
	if !strings.Contains(out, tagline+" · v1.2.3") {
		t.Error("tagline or version missing")
	}
}

func TestRenderColor(t *testing.T) {
	out := render("dev", true, 120)
	if !strings.Contains(out, "\x1b[38;5;154m█") || !strings.Contains(out, "\x1b[0m") {
		t.Error("color render missing gradient codes")
	}
	if !strings.Contains(out, fmt.Sprintf("\x1b[38;5;%dm╗", shadowColor)) {
		t.Error("shadow strokes not gray")
	}
}

func TestRenderNarrow(t *testing.T) {
	out := render("dev", false, 20)
	if strings.Contains(out, "█") || !strings.Contains(out, "yoink") {
		t.Errorf("narrow render should be a text title, got:\n%s", out)
	}
}

func TestWordmarkRowsAligned(t *testing.T) {
	rows := wordmark("YOINK")
	if len(rows) != 6 {
		t.Fatalf("got %d rows, want 6", len(rows))
	}
	for ch, glyph := range font {
		w := len([]rune(glyph[0]))
		for i, line := range glyph {
			if len([]rune(line)) != w {
				t.Errorf("glyph %c row %d width %d, want %d", ch, i, len([]rune(line)), w)
			}
		}
	}
}

func TestPrintToBufferIsPlain(t *testing.T) {
	var buf bytes.Buffer
	Print(&buf, "dev")
	if strings.Contains(buf.String(), "\x1b[") {
		t.Error("non-terminal output should have no colors")
	}
}
