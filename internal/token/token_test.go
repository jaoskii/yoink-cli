package token

import (
	"regexp"
	"testing"
)

func TestNew(t *testing.T) {
	valid := regexp.MustCompile(`^[a-z2-7]{26}$`)
	seen := map[string]bool{}
	for range 1000 {
		tok := New()
		if !valid.MatchString(tok) {
			t.Fatalf("token %q is not 26 lowercase base32 chars", tok)
		}
		if seen[tok] {
			t.Fatalf("duplicate token %q", tok)
		}
		seen[tok] = true
	}
}

func TestEqual(t *testing.T) {
	if !Equal("abc", "abc") {
		t.Error("equal tokens reported different")
	}
	for _, other := range []string{"abd", "ab", "abcd", ""} {
		if Equal("abc", other) {
			t.Errorf("Equal(abc, %q) = true", other)
		}
	}
}
