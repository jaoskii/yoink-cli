// Package archive streams files and folders as a zip, without temp files.
package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// File is one entry in the archive.
type File struct {
	Src     string // path on disk; empty for directories
	Name    string // slash-separated name inside the zip; directories end in "/"
	Dir     bool
	Size    int64
	Mode    fs.FileMode
	ModTime time.Time
}

// Plan lists the entries to write. It is built before streaming so the
// total size is known up front.
type Plan struct {
	Files      []File
	TotalBytes int64
	FileCount  int
	Skipped    []string // symlinks and special files left out
}

// Build walks paths and prepares archive entries. Top-level names are made
// unique; symlinks inside folders are skipped, never followed.
func Build(paths []string) (*Plan, error) {
	p := &Plan{}
	used := map[string]bool{}
	for _, arg := range paths {
		abs, err := filepath.Abs(arg)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		top := uniqueName(filepath.Base(abs), used)
		if !info.IsDir() {
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("%s: not a regular file", arg)
			}
			p.add(File{Src: abs, Name: top, Size: info.Size(), Mode: info.Mode(), ModTime: info.ModTime()})
			continue
		}
		err = filepath.WalkDir(abs, func(fp string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(abs, fp)
			if err != nil {
				return err
			}
			name := top
			if rel != "." {
				name = path.Join(top, filepath.ToSlash(rel))
			}
			switch {
			case d.IsDir():
				fi, err := d.Info()
				if err != nil {
					return err
				}
				p.add(File{Name: name + "/", Dir: true, Mode: fi.Mode(), ModTime: fi.ModTime()})
			case d.Type().IsRegular():
				fi, err := d.Info()
				if err != nil {
					return err
				}
				p.add(File{Src: fp, Name: name, Size: fi.Size(), Mode: fi.Mode(), ModTime: fi.ModTime()})
			default:
				p.Skipped = append(p.Skipped, fp)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (p *Plan) add(f File) {
	p.Files = append(p.Files, f)
	if !f.Dir {
		p.TotalBytes += f.Size
		p.FileCount++
	}
}

// Write streams the zip to w. onRead, if non-nil, receives the number of
// source bytes read as they are consumed.
func (p *Plan) Write(w io.Writer, onRead func(n int64)) error {
	zw := zip.NewWriter(w)
	for _, f := range p.Files {
		hdr := &zip.FileHeader{Name: f.Name, Modified: f.ModTime, Method: method(f)}
		hdr.SetMode(f.Mode)
		ew, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		if f.Dir {
			continue
		}
		if err := copyFile(ew, f.Src, onRead); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	return zw.Close()
}

func copyFile(w io.Writer, src string, onRead func(int64)) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, &countingReader{r: f, fn: onRead})
	return err
}

type countingReader struct {
	r  io.Reader
	fn func(int64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 && c.fn != nil {
		c.fn(int64(n))
	}
	return n, err
}

// storedExts are already compressed; deflating them again only burns CPU.
var storedExts = map[string]bool{
	".zip": true, ".gz": true, ".tgz": true, ".bz2": true, ".xz": true, ".zst": true,
	".7z": true, ".rar": true, ".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".webp": true, ".heic": true, ".heif": true, ".avif": true, ".mp4": true, ".mov": true,
	".m4v": true, ".mkv": true, ".webm": true, ".avi": true, ".mp3": true, ".m4a": true,
	".aac": true, ".ogg": true, ".opus": true, ".flac": true, ".docx": true, ".xlsx": true,
	".pptx": true, ".apk": true, ".ipa": true, ".dmg": true, ".jar": true,
}

func method(f File) uint16 {
	if f.Dir || storedExts[strings.ToLower(filepath.Ext(f.Name))] {
		return zip.Store
	}
	return zip.Deflate
}

// uniqueName returns name, or "name (2).ext", "name (3).ext"... if taken.
func uniqueName(name string, used map[string]bool) string {
	if !used[name] {
		used[name] = true
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if !used[cand] {
			used[cand] = true
			return cand
		}
	}
}
