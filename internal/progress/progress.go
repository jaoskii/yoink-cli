// Package progress renders a live one-line transfer status in the terminal.
package progress

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/jaoskii/yoink-cli/internal/units"
)

// Tracker keeps the set of active transfers and redraws a status line.
// When output is not a terminal it skips the live line and prints only
// per-transfer results.
type Tracker struct {
	mu          sync.Mutex
	out         io.Writer
	fd          int
	interactive bool
	quiet       bool
	stopped     bool
	active      []*Transfer
	lineLen     int
	seen        atomic.Bool
}

// New returns a Tracker writing to out. quiet suppresses all output.
func New(out io.Writer, quiet bool) *Tracker {
	t := &Tracker{out: out, quiet: quiet, fd: -1}
	if f, ok := out.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		t.fd = int(f.Fd())
		t.interactive = true
	}
	return t
}

// Seen records that some device reached the server.
func (t *Tracker) Seen(string) { t.seen.Store(true) }

// SeenAny reports whether any request has arrived.
func (t *Tracker) SeenAny() bool { return t.seen.Load() }

// Begin starts tracking a transfer of total bytes (0 if unknown).
func (t *Tracker) Begin(remote string, total int64) *Transfer {
	tr := &Transfer{t: t, remote: remote, total: total, start: time.Now()}
	t.mu.Lock()
	t.active = append(t.active, tr)
	t.mu.Unlock()
	return tr
}

// Println prints msg above the status line.
func (t *Tracker) Println(msg string) {
	if t.quiet {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.clearLocked()
	fmt.Fprintln(t.out, msg)
	t.drawLocked()
}

// Run redraws the status line until ctx is done or Stop is called.
func (t *Tracker) Run(ctx context.Context) {
	if t.quiet || !t.interactive {
		return
	}
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			t.mu.Lock()
			stopped := t.stopped
			t.drawLocked()
			t.mu.Unlock()
			if stopped {
				return
			}
		}
	}
}

// Stop clears the status line and stops further redraws.
func (t *Tracker) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.clearLocked()
	t.stopped = true
}

func (t *Tracker) drawLocked() {
	if t.quiet || !t.interactive || t.stopped {
		return
	}
	line := t.statusLocked()
	if w, _, err := term.GetSize(t.fd); err == nil && w > 1 && utf8.RuneCountInString(line) >= w {
		line = string([]rune(line)[:w-1]) // never wrap, or \r can't redraw in place
	}
	n := utf8.RuneCountInString(line)
	pad := ""
	if t.lineLen > n {
		pad = strings.Repeat(" ", t.lineLen-n)
	}
	fmt.Fprint(t.out, "\r"+line+pad)
	t.lineLen = n
}

func (t *Tracker) clearLocked() {
	if t.interactive && t.lineLen > 0 {
		fmt.Fprint(t.out, "\r"+strings.Repeat(" ", t.lineLen)+"\r")
	}
	t.lineLen = 0
}

func (t *Tracker) statusLocked() string {
	if len(t.active) == 0 {
		return "  Waiting for a device…  (Ctrl+C to stop)"
	}
	var done, total int64
	start := t.active[0].start
	for _, tr := range t.active {
		done += tr.done.Load()
		total += tr.total
		if tr.start.Before(start) {
			start = tr.start
		}
	}
	who := t.active[0].remote
	if len(t.active) > 1 {
		who = fmt.Sprintf("%d downloads", len(t.active))
	}
	rate := units.Rate(done, time.Since(start))
	if total <= 0 {
		return fmt.Sprintf("  ↓ %s  %s  %s", who, units.Bytes(done), rate)
	}
	pct := min(float64(done)/float64(total), 1)
	return fmt.Sprintf("  ↓ %s  %s %3.0f%%  %s / %s  %s",
		who, bar(pct, 20), pct*100, units.Bytes(done), units.Bytes(total), rate)
}

// roundDuration keeps two significant-ish digits: 37ms, 4.1s, 1m23s.
func roundDuration(d time.Duration) time.Duration {
	switch {
	case d < time.Second:
		return d.Round(time.Millisecond)
	case d < time.Minute:
		return d.Round(100 * time.Millisecond)
	default:
		return d.Round(time.Second)
	}
}

func bar(pct float64, width int) string {
	filled := int(pct * float64(width))
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// Transfer is one tracked download.
type Transfer struct {
	t      *Tracker
	remote string
	total  int64
	start  time.Time
	done   atomic.Int64
	ended  atomic.Bool
}

// Add records n more bytes sent.
func (tr *Transfer) Add(n int64) { tr.done.Add(n) }

// End removes the transfer and prints its result. Tiny incomplete requests
// (browser probes, small ranges) end silently.
func (tr *Transfer) End(complete bool) {
	if !tr.ended.CompareAndSwap(false, true) {
		return
	}
	t := tr.t
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, a := range t.active {
		if a == tr {
			t.active = append(t.active[:i], t.active[i+1:]...)
			break
		}
	}

	n, elapsed := tr.done.Load(), time.Since(tr.start)
	var msg string
	switch {
	case complete:
		msg = fmt.Sprintf("  ✓ %s  downloaded %s in %s (%s)",
			tr.remote, units.Bytes(n), roundDuration(elapsed), units.Rate(n, elapsed))
	case n >= 1<<20:
		msg = fmt.Sprintf("  ✗ %s  stopped at %s / %s", tr.remote, units.Bytes(n), units.Bytes(tr.total))
	}
	if msg == "" || t.quiet {
		t.drawLocked()
		return
	}
	t.clearLocked()
	fmt.Fprintln(t.out, msg)
	t.drawLocked()
}
