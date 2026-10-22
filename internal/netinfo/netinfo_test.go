package netinfo

import (
	"net"
	"testing"
)

func TestIsVirtual(t *testing.T) {
	virtual := []string{"docker0", "br-1a2b", "veth9", "utun4", "awdl0", "bridge100", "vEthernet (WSL)", "tailscale0", "wg0"}
	real := []string{"en0", "en1", "wlan0", "wlp3s0", "eth0", "enp3s0", "Wi-Fi", "Ethernet"}
	for _, n := range virtual {
		if !IsVirtual(n) {
			t.Errorf("IsVirtual(%q) = false, want true", n)
		}
	}
	for _, n := range real {
		if IsVirtual(n) {
			t.Errorf("IsVirtual(%q) = true, want false", n)
		}
	}
}

func TestChoose(t *testing.T) {
	cands := []Addr{
		{Iface: "utun4", IP: net.ParseIP("10.8.0.2").To4()},
		{Iface: "en0", IP: net.ParseIP("192.168.1.23").To4()},
		{Iface: "docker0", IP: net.ParseIP("172.17.0.1").To4()},
	}

	tests := []struct {
		name      string
		preferred string
		want      string
		wantIface string
		wantErr   bool
	}{
		{name: "VPN route falls back to Wi-Fi", preferred: "10.8.0.2", wantIface: "en0"},
		{name: "preferred Wi-Fi", preferred: "192.168.1.23", wantIface: "en0"},
		{name: "no route info", wantIface: "en0"},
		{name: "forced interface", want: "docker0", wantIface: "docker0"},
		{name: "forced is case-insensitive", want: "EN0", wantIface: "en0"},
		{name: "unknown interface", want: "nope", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pref net.IP
			if tt.preferred != "" {
				pref = net.ParseIP(tt.preferred).To4()
			}
			got, err := choose(cands, pref, tt.want)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Iface != tt.wantIface {
				t.Errorf("got %s, want %s", got.Iface, tt.wantIface)
			}
		})
	}
}

func TestChooseNoLAN(t *testing.T) {
	cands := []Addr{
		{Iface: "eth0", IP: net.ParseIP("203.0.113.5").To4()},
		{Iface: "docker0", IP: net.ParseIP("172.17.0.1").To4()},
	}
	if _, err := choose(cands, nil, ""); err == nil {
		t.Fatal("expected error when only public/virtual addresses exist")
	}
}
