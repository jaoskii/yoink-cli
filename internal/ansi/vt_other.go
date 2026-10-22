//go:build !windows

package ansi

import "io"

// EnableVT is a no-op: Unix terminals handle ANSI escapes natively.
func EnableVT(io.Writer) {}
