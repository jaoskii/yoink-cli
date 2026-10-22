package units

import (
	"testing"
	"time"
)

func TestBytes(t *testing.T) {
	cases := map[int64]string{
		0:             "0 B",
		999:           "999 B",
		1000:          "1.0 KB",
		48_200_000:    "48.2 MB",
		1_500_000_000: "1.5 GB",
	}
	for in, want := range cases {
		if got := Bytes(in); got != want {
			t.Errorf("Bytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestRate(t *testing.T) {
	if got := Rate(10_000_000, 2*time.Second); got != "5.0 MB/s" {
		t.Errorf("Rate = %q, want 5.0 MB/s", got)
	}
	if got := Rate(10, 0); got != "—" {
		t.Errorf("Rate with zero duration = %q", got)
	}
}
