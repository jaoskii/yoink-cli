// Package hints explains why other devices might not reach this machine.
package hints

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// IsWSL reports whether we run inside Windows Subsystem for Linux.
func IsWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	b, err := os.ReadFile("/proc/sys/kernel/osrelease")
	return err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft")
}

// NoConnection returns advice for when no device has connected yet.
func NoConnection(goos string, port int, wsl bool) []string {
	lines := []string{"No device has connected yet. If your phone can't open the link:"}
	switch {
	case wsl:
		lines = append(lines, "• WSL2 sits behind its own NAT, so phones can't reach it. Run yoink.exe from Windows, or turn on mirrored networking in .wslconfig.")
	case goos == "windows":
		lines = append(lines, "• Windows Firewall may be blocking it. Allow yoink.exe when prompted, and set this Wi-Fi to a Private network (Settings → Network & internet → Wi-Fi → network properties).")
	case goos == "darwin":
		lines = append(lines, "• macOS may be blocking incoming connections. Allow yoink when prompted (System Settings → Network → Firewall).")
	case goos == "linux":
		lines = append(lines, fmt.Sprintf("• A firewall may drop the connection: sudo ufw allow %d/tcp  (or: sudo firewall-cmd --add-port=%d/tcp). Use --port to keep the same port between runs.", port, port))
	}
	return append(lines,
		"• Both devices must be on the same Wi-Fi. Guest and office networks often block devices from reaching each other.",
		"• If the phone is on mobile data, switch it to Wi-Fi.",
	)
}
