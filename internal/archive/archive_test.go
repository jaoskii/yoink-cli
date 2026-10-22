package archive

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildAndWrite(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	writeFile(t, filepath.Join(proj, "a.txt"), "hello")
	writeFile(t, filepath.Join(proj, "sub", "b.jpg"), "fakejpeg")
	single := filepath.Join(root, "single.txt")
	writeFile(t, single, "solo")
	symlinked := os.Symlink(filepath.Join(proj, "a.txt"), filepath.Join(proj, "link")) == nil

	plan, err := Build([]string{proj, single})
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, f := range plan.Files {
		names = append(names, f.Name)
	}
	for _, want := range []string{"proj/", "proj/a.txt", "proj/sub/", "proj/sub/b.jpg", "single.txt"} {
		if !slices.Contains(names, want) {
			t.Errorf("missing entry %q in %v", want, names)
		}
	}
	if slices.Contains(names, "proj/link") {
		t.Error("symlink was included; should be skipped")
	}
	if symlinked && len(plan.Skipped) != 1 {
		t.Errorf("Skipped = %v, want the symlink", plan.Skipped)
	}
	if plan.FileCount != 3 || plan.TotalBytes != int64(len("hello")+len("fakejpeg")+len("solo")) {
		t.Errorf("FileCount=%d TotalBytes=%d", plan.FileCount, plan.TotalBytes)
	}

	var buf bytes.Buffer
	var read int64
	if err := plan.Write(&buf, func(n int64) { read += n }); err != nil {
		t.Fatal(err)
	}
	if read != plan.TotalBytes {
		t.Errorf("onRead total = %d, want %d", read, plan.TotalBytes)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, zf := range zr.File {
		switch zf.Name {
		case "proj/a.txt":
			if zf.Method != zip.Deflate {
				t.Errorf("a.txt method = %d, want Deflate", zf.Method)
			}
			rc, err := zf.Open()
			if err != nil {
				t.Fatal(err)
			}
			got, _ := io.ReadAll(rc)
			rc.Close()
			if string(got) != "hello" {
				t.Errorf("a.txt content = %q", got)
			}
		case "proj/sub/b.jpg":
			if zf.Method != zip.Store {
				t.Errorf("b.jpg method = %d, want Store", zf.Method)
			}
		}
	}
}

func TestBuildDedupesTopLevelNames(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "x", "dup")
	b := filepath.Join(root, "y", "dup")
	writeFile(t, filepath.Join(a, "1.txt"), "1")
	writeFile(t, filepath.Join(b, "2.txt"), "2")

	plan, err := Build([]string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range plan.Files {
		names = append(names, f.Name)
	}
	for _, want := range []string{"dup/1.txt", "dup (2)/2.txt"} {
		if !slices.Contains(names, want) {
			t.Errorf("missing %q in %v", want, names)
		}
	}
}

func TestBuildMissingPath(t *testing.T) {
	if _, err := Build([]string{filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Fatal("expected error for missing path")
	}
}
