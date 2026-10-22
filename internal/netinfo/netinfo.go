// Package netinfo finds the LAN address that other devices on the network can reach.
package netinfo

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// Addr is an IPv4 address on a named interface.
type Addr struct {
	Iface string
	IP    net.IP
}

// virtualPrefixes match interfaces a phone on the same Wi-Fi can't reach:
// containers, VMs, VPNs, Apple peer-to-peer links and hotspot bridges.
var virtualPrefixes = []string{
	"docker", "br-", "veth", "virbr", "vmnet", "vboxnet", "lxc", "lxd", "cni",
	"flannel", "cali", "kube", "podman", "utun", "tun", "tap", "wg", "zt",
	"tailscale", "awdl", "llw", "bridge", "anpi", "ap1", "gif", "stf",
	"vethernet", "vmware", "virtualbox", "hyper-v", "loopback",
}

// IsVirtual reports whether an interface name looks virtual or VPN.
func IsVirtual(name string) bool {
	n := strings.ToLower(name)
	for _, p := range virtualPrefixes {
		if strings.HasPrefix(n, p) {
			return true
		}
	}
	return false
}

// IsLAN reports whether ip is a private (RFC 1918) IPv4 address.
func IsLAN(ip net.IP) bool {
	ip4 := ip.To4()
	return ip4 != nil && ip4.IsPrivate()
}

// LANIP picks the address to advertise. want forces an interface by name.
func LANIP(want string) (Addr, error) {
	cands, err := candidates()
	if err != nil {
		return Addr{}, err
	}
	return choose(cands, outboundIP(), want)
}

// choose prefers the interface the OS routes outbound traffic through, unless
// that is a VPN or virtual link, then falls back to the first private address.
func choose(cands []Addr, preferred net.IP, want string) (Addr, error) {
	if want != "" {
		for _, c := range cands {
			if strings.EqualFold(c.Iface, want) {
				return c, nil
			}
		}
		return Addr{}, fmt.Errorf("interface %q has no IPv4 address (available: %s)", want, names(cands))
	}
	if preferred != nil {
		for _, c := range cands {
			if c.IP.Equal(preferred) && !IsVirtual(c.Iface) && IsLAN(c.IP) {
				return c, nil
			}
		}
	}
	for _, c := range cands {
		if !IsVirtual(c.Iface) && IsLAN(c.IP) {
			return c, nil
		}
	}
	return Addr{}, errors.New("no Wi-Fi or Ethernet address found; connect to a network or pass --interface")
}

func candidates() ([]Addr, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}
	var out []Addr
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if ip4 := ipn.IP.To4(); ip4 != nil && !ip4.IsLinkLocalUnicast() {
				out = append(out, Addr{Iface: ifc.Name, IP: ip4})
			}
		}
	}
	return out, nil
}

// outboundIP returns the source address the OS would use to reach the
// internet. Dialing UDP sends no packets; it only consults the routing table.
func outboundIP() net.IP {
	conn, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return nil
	}
	defer conn.Close()
	if ua, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return ua.IP.To4()
	}
	return nil
}

func names(cands []Addr) string {
	var out []string
	seen := map[string]bool{}
	for _, c := range cands {
		if !seen[c.Iface] {
			seen[c.Iface] = true
			out = append(out, c.Iface)
		}
	}
	if len(out) == 0 {
		return "none"
	}
	return strings.Join(out, ", ")
}
