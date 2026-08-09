// Package guests owns guest groups, exact-name lookup, and per-guest RSVP.
// Guests have no accounts: identity is the exact match of a normalized full
// name, and the guest list is deliberately impossible to enumerate.
package guests

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalize is the single source of truth for guest-name normalization:
// trim, casefold, strip diacritics, collapse inner whitespace. The importer
// reuses it — the same input must always produce the same key, or lookups
// and reconciliation would disagree.
func Normalize(name string) string {
	decomposed := norm.NFD.String(name)
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue // combining mark: drops the accent, keeps the base rune
		}
		b.WriteRune(r)
	}
	recomposed := norm.NFC.String(b.String())
	lowered := strings.ToLower(recomposed)
	return strings.Join(strings.Fields(lowered), " ")
}
