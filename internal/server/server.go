// Package server serves a Share over HTTP behind a secret token path.
package server

import (
	_ "embed"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/jaoskii/yoink-cli/internal/token"
	"github.com/jaoskii/yoink-cli/internal/units"
)

//go:embed web/landing.html
var landingHTML string

var landingTmpl = template.Must(template.New("landing").Parse(landingHTML))

// Monitor observes requests and transfers (implemented by the progress UI).
type Monitor interface {
	Seen(remote string)
	Begin(remote string, total int64) Transfer
}

// Transfer tracks one download.
type Transfer interface {
	Add(n int64)
	End(complete bool)
}

// Options configure the handler.
type Options struct {
	Monitor    Monitor // optional
	OnComplete func()  // optional; called after each fully completed download
}

type handler struct {
	share *Share
	token string
	opt   Options
}

// NewHandler returns an http.Handler that serves s under /<token>/.
// Any other path is a 404, so the URL itself is the access key.
func NewHandler(s *Share, tok string, opt Options) http.Handler {
	if opt.Monitor == nil {
		opt.Monitor = nopMonitor{}
	}
	if opt.OnComplete == nil {
		opt.OnComplete = func() {}
	}
	return &handler{share: s, token: tok, opt: opt}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	remote := clientIP(r)
	h.opt.Monitor.Seen(remote)

	hdr := w.Header()
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Referrer-Policy", "no-referrer") // keep the token out of Referer headers
	hdr.Set("Cache-Control", "no-store")
	hdr.Set("X-Robots-Tag", "noindex")

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		hdr.Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tok, rest, hasSlash := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if !token.Equal(tok, h.token) {
		http.NotFound(w, r)
		return
	}
	switch {
	case !hasSlash:
		http.Redirect(w, r, "/"+h.token+"/", http.StatusFound)
	case rest == "":
		h.landing(w)
	case rest == h.share.Name:
		h.download(w, r, remote)
	default:
		http.NotFound(w, r)
	}
}

func (h *handler) landing(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = landingTmpl.Execute(w, map[string]any{
		"Name":  h.share.Name,
		"Size":  units.Bytes(h.share.Size),
		"Files": h.share.FileCount,
		"IsZip": h.share.IsZip,
		// "./" keeps names like "javascript:x" a relative path
		"Href": "./" + url.PathEscape(h.share.Name),
	})
}

func (h *handler) download(w http.ResponseWriter, r *http.Request, remote string) {
	w.Header().Set("Content-Disposition", contentDisposition(h.share.Name))
	if h.share.IsZip {
		h.serveZip(w, r, remote)
		return
	}
	h.serveFile(w, r, remote)
}

func (h *handler) serveFile(w http.ResponseWriter, r *http.Request, remote string) {
	f, err := os.Open(h.share.path)
	if err != nil {
		http.Error(w, "file is no longer available", http.StatusGone)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "cannot read file", http.StatusInternalServerError)
		return
	}
	cw := &countingWriter{
		ResponseWriter: w,
		mon:            h.opt.Monitor,
		remote:         remote,
		get:            r.Method == http.MethodGet,
		size:           info.Size(),
	}
	// ServeContent handles Range requests, so interrupted downloads resume.
	http.ServeContent(cw, r, h.share.Name, info.ModTime(), f)
	if cw.finish() {
		h.opt.OnComplete()
	}
}

func (h *handler) serveZip(w http.ResponseWriter, r *http.Request, remote string) {
	w.Header().Set("Content-Type", "application/zip")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	tr := h.opt.Monitor.Begin(remote, h.share.Size)
	w.WriteHeader(http.StatusOK)
	err := h.share.plan.Write(w, tr.Add)
	tr.End(err == nil)
	if err == nil {
		h.opt.OnComplete()
	}
}

// countingWriter reports body bytes to a Transfer and records whether the
// response delivered the file through its last byte (full body or tail range).
type countingWriter struct {
	http.ResponseWriter
	mon    Monitor
	remote string
	get    bool
	size   int64

	tr     Transfer
	wrote  bool
	expect int64
	n      int64
	toEnd  bool
}

func (c *countingWriter) WriteHeader(code int) {
	if !c.wrote {
		c.wrote = true
		if c.get && (code == http.StatusOK || code == http.StatusPartialContent) {
			c.expect, _ = strconv.ParseInt(c.Header().Get("Content-Length"), 10, 64)
			c.toEnd = code == http.StatusOK || rangeEnd(c.Header().Get("Content-Range")) == c.size-1
			c.tr = c.mon.Begin(c.remote, c.expect)
		}
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c *countingWriter) Write(p []byte) (int, error) {
	if !c.wrote {
		c.WriteHeader(http.StatusOK)
	}
	n, err := c.ResponseWriter.Write(p)
	if c.tr != nil && n > 0 {
		c.n += int64(n)
		c.tr.Add(int64(n))
	}
	return n, err
}

func (c *countingWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }

func (c *countingWriter) finish() bool {
	if c.tr == nil {
		return false
	}
	complete := c.toEnd && c.n == c.expect
	c.tr.End(complete)
	return complete
}

// rangeEnd parses "bytes 100-199/1000" and returns 199, or -1.
func rangeEnd(v string) int64 {
	rest, ok := strings.CutPrefix(v, "bytes ")
	if !ok {
		return -1
	}
	rng, _, _ := strings.Cut(rest, "/")
	_, end, ok := strings.Cut(rng, "-")
	if !ok {
		return -1
	}
	n, err := strconv.ParseInt(end, 10, 64)
	if err != nil {
		return -1
	}
	return n
}

// contentDisposition builds an attachment header with an ASCII fallback and
// an RFC 5987 UTF-8 name, so non-ASCII names survive on every browser.
func contentDisposition(name string) string {
	var fallback, encoded strings.Builder
	for _, r := range name {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			fallback.WriteByte('_')
		} else {
			fallback.WriteRune(r)
		}
	}
	for _, b := range []byte(name) {
		if isAttrChar(b) {
			encoded.WriteByte(b)
		} else {
			fmt.Fprintf(&encoded, "%%%02X", b)
		}
	}
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, fallback.String(), encoded.String())
}

func isAttrChar(b byte) bool {
	switch {
	case 'a' <= b && b <= 'z', 'A' <= b && b <= 'Z', '0' <= b && b <= '9':
		return true
	}
	return strings.IndexByte("!#$&+-.^_`|~", b) >= 0
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type nopMonitor struct{}

func (nopMonitor) Seen(string)                  {}
func (nopMonitor) Begin(string, int64) Transfer { return nopTransfer{} }

type nopTransfer struct{}

func (nopTransfer) Add(int64) {}
func (nopTransfer) End(bool)  {}
