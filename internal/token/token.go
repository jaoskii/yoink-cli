// Package token creates and compares the secret path segment of share URLs.
package token

import (
	"crypto/rand"
	"crypto/subtle"
	"strings"
)

// New returns a random URL-safe token with at least 128 bits of entropy.
func New() string {
	// rand.Text: 26 base32 chars (130 bits). Lowercase reads better in URLs.
	return strings.ToLower(rand.Text())
}

// Equal reports whether a and b match, in constant time.
func Equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
