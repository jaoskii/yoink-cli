package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jaoskii/yoink-cli/internal/archive"
)

// Share describes what is being served.
type Share struct {
	Name      string // download file name
	Size      int64  // file size, or total uncompressed bytes for a zip
	FileCount int
	IsZip     bool

	path    string        // single file
	modTime time.Time     // single file
	plan    *archive.Plan // zip
}

// NewShare prepares paths for serving. A single regular file is served as-is
// (resumable); folders or several paths are zipped on the fly.
func NewShare(paths []string, forceZip bool, now time.Time) (*Share, error) {
	if len(paths) == 0 {
		return nil, errors.New("nothing to share")
	}
	if len(paths) == 1 && !forceZip {
		info, err := os.Stat(paths[0])
		if err != nil {
			return nil, err
		}
		if info.Mode().IsRegular() {
			abs, err := filepath.Abs(paths[0])
			if err != nil {
				return nil, err
			}
			return &Share{
				Name:      filepath.Base(abs),
				Size:      info.Size(),
				FileCount: 1,
				path:      abs,
				modTime:   info.ModTime(),
			}, nil
		}
	}

	plan, err := archive.Build(paths)
	if err != nil {
		return nil, err
	}
	if plan.FileCount == 0 {
		return nil, errors.New("no files to share (folders are empty or contain only symlinks)")
	}
	name := fmt.Sprintf("yoink-%s.zip", now.Format("20060102-1504"))
	if len(paths) == 1 {
		abs, err := filepath.Abs(paths[0])
		if err != nil {
			return nil, err
		}
		name = filepath.Base(abs) + ".zip"
	}
	return &Share{Name: name, Size: plan.TotalBytes, FileCount: plan.FileCount, IsZip: true, plan: plan}, nil
}

// Skipped lists paths left out of the zip (symlinks, special files).
func (s *Share) Skipped() []string {
	if s.plan == nil {
		return nil
	}
	return s.plan.Skipped
}
