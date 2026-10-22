package server

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testToken = "testtoken"

type fakeMon struct {
	mu    sync.Mutex
	bytes int64
	ended []bool
}

func (f *fakeMon) Seen(string) {}
func (f *fakeMon) Begin(string, int64) Transfer {
	return &fakeTr{f}
}

type fakeTr struct{ f *fakeMon }

func (t *fakeTr) Add(n int64) {
	t.f.mu.Lock()
	t.f.bytes += n
	t.f.mu.Unlock()
}
func (t *fakeTr) End(c bool) {
	t.f.mu.Lock()
	t.f.ended = append(t.f.ended, c)
	t.f.mu.Unlock()
}

type harness struct {
	srv       *httptest.Server
	base      string
	mon       *fakeMon
	completed atomic.Int32
}

func newHarness(t *testing.T, s *Share) *harness {
	t.Helper()
	h := &harness{mon: &fakeMon{}}
	h.srv = httptest.NewServer(NewHandler(s, testToken, Options{
		Monitor:    h.mon,
		OnComplete: func() { h.completed.Add(1) },
	}))
	t.Cleanup(h.srv.Close)
	h.base = h.srv.URL + "/" + testToken + "/"
	return h
}

// wait closes the server, which blocks until handlers return, so the
// completion counters are final.
func (h *harness) wait() { h.srv.Close() }

func get(t *testing.T, rawURL string, header map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, body
}

func tempFile(t *testing.T, name string, size int) (string, []byte) {
	t.Helper()
	data := make([]byte, size)
	rand.Read(data)
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path, data
}

func TestUnknownPathsAre404(t *testing.T) {
	path, _ := tempFile(t, "a.bin", 10)
	s, err := NewShare([]string{path}, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, s)
	for _, p := range []string{"/", "/nope/", "/nope/a.bin", "/" + testToken + "/other.bin", "/favicon.ico"} {
		if resp, _ := get(t, h.srv.URL+p, nil); resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, resp.StatusCode)
		}
	}
}

func TestRedirectAddsSlash(t *testing.T) {
	h := newHarness(t, &Share{Name: "x.txt"})
	resp, _ := get(t, h.srv.URL+"/"+testToken, nil)
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/"+testToken+"/" {
		t.Fatalf("got %d Location=%q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestLandingEscapesNameAndSetsHeaders(t *testing.T) {
	h := newHarness(t, &Share{Name: "<b>x</b>.txt", Size: 1200, FileCount: 1})
	resp, body := get(t, h.base, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if strings.Contains(string(body), "<b>x</b>") || !strings.Contains(string(body), "&lt;b&gt;x&lt;/b&gt;.txt") {
		t.Error("file name not HTML-escaped")
	}
	if !strings.Contains(string(body), "1.2 KB") {
		t.Error("size missing from landing page")
	}
	for k, v := range map[string]string{"X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer", "Cache-Control": "no-store"} {
		if got := resp.Header.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestSingleFileDownload(t *testing.T) {
	path, data := tempFile(t, "my photo.bin", 200_000)
	s, err := NewShare([]string{path}, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, s)
	resp, body := get(t, h.base+url.PathEscape(s.Name), nil)
	h.wait()

	if resp.StatusCode != http.StatusOK || !bytes.Equal(body, data) {
		t.Fatalf("status %d, body match %v", resp.StatusCode, bytes.Equal(body, data))
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment;") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if h.completed.Load() != 1 || h.mon.bytes != int64(len(data)) {
		t.Errorf("completed=%d bytes=%d", h.completed.Load(), h.mon.bytes)
	}
}

func TestRangeRequestsAndResume(t *testing.T) {
	path, data := tempFile(t, "big.bin", 1000)
	s, err := NewShare([]string{path}, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, s)

	resp, body := get(t, h.base+s.Name, map[string]string{"Range": "bytes=0-9"})
	if resp.StatusCode != http.StatusPartialContent || !bytes.Equal(body, data[:10]) {
		t.Fatalf("head range: status %d len %d", resp.StatusCode, len(body))
	}
	resp, body = get(t, h.base+s.Name, map[string]string{"Range": "bytes=10-"})
	if resp.StatusCode != http.StatusPartialContent || !bytes.Equal(body, data[10:]) {
		t.Fatalf("tail range: status %d len %d", resp.StatusCode, len(body))
	}
	h.wait()
	if got := h.completed.Load(); got != 1 {
		t.Errorf("completed = %d, want 1 (only the range reaching EOF counts)", got)
	}
}

func TestFolderDownloadIsZip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "site")
	if err := os.MkdirAll(filepath.Join(dir, "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>hi</h1>"), 0o644)
	os.WriteFile(filepath.Join(dir, "css", "a.css"), []byte("body{}"), 0o644)

	s, err := NewShare([]string{dir}, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !s.IsZip || s.Name != "site.zip" || s.FileCount != 2 {
		t.Fatalf("share = %+v", s)
	}
	h := newHarness(t, s)
	resp, body := get(t, h.base+s.Name, nil)
	h.wait()
	if resp.Header.Get("Content-Type") != "application/zip" {
		t.Errorf("Content-Type = %q", resp.Header.Get("Content-Type"))
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	if strings.Join(names, ",") != "site/,site/css/,site/css/a.css,site/index.html" {
		t.Errorf("zip entries = %v", names)
	}
	if h.completed.Load() != 1 {
		t.Errorf("completed = %d", h.completed.Load())
	}
}

func TestPostRejected(t *testing.T) {
	h := newHarness(t, &Share{Name: "x.txt"})
	resp, err := http.Post(h.base, "text/plain", strings.NewReader("hi"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST = %d, want 405", resp.StatusCode)
	}
}

func TestContentDisposition(t *testing.T) {
	got := contentDisposition(`naïve "file".txt`)
	want := `attachment; filename="na_ve _file_.txt"; filename*=UTF-8''na%C3%AFve%20%22file%22.txt`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestRangeEnd(t *testing.T) {
	cases := map[string]int64{"bytes 0-9/100": 9, "bytes 10-99/100": 99, "": -1, "bytes */100": -1}
	for in, want := range cases {
		if got := rangeEnd(in); got != want {
			t.Errorf("rangeEnd(%q) = %d, want %d", in, got, want)
		}
	}
}
