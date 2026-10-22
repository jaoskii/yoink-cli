// Package units formats byte counts and rates for humans.
package units

import (
	"fmt"
	"time"
)

// Bytes formats n using decimal units (1 KB = 1000 B), matching macOS Finder.
func Bytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// Rate formats n bytes transferred over d as a per-second rate.
func Rate(n int64, d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	return Bytes(int64(float64(n)/d.Seconds())) + "/s"
}
