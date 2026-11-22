// Package idgen produces short, random, URL-safe identifiers.
//
// Stored documents (checklists and checklist items) need an identifier that survives export/import and, later, synchronisation between
// two databases. A random 64-bit value in hexadecimal is short enough to read
// and wide enough that a collision is not a practical concern.
package idgen

import (
	"crypto/rand"
	"encoding/hex"
)

// New returns prefix followed by 16 random hexadecimal characters.
func New(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand does not fail on supported platforms. Keep the contract
		// (a non-empty identifier) rather than returning an error to every
		// caller; the store enforces uniqueness separately.
		return prefix + "0000000000000000"
	}
	return prefix + hex.EncodeToString(buf)
}
