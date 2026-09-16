// SPDX-License-Identifier: MIT

package vstar

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzComponent_PropertyRoundtrip exercises the case-insensitive
// Set / Get round-trip on a Component. For any (name, value) pair
// where name is a syntactically valid iCalendar/vCard property name
// (letters, digits, hyphen — RFC 5545 §3.2 / RFC 6350 §3.3), Set
// followed by Get under any case-variant of name MUST return the
// original value, and Get of an unrelated name MUST return ok=false.
//
// Seed corpus lives at testdata/fuzz/FuzzComponent_PropertyRoundtrip/
// and draws from RFC 5545 §3.7–§3.8 (UID, SUMMARY, DTSTART, …) and
// RFC 6350 §6 (FN, N, EMAIL, …).
func FuzzComponent_PropertyRoundtrip(f *testing.F) {
	f.Fuzz(func(t *testing.T, name, value string) {
		if !validPropertyName(name) {
			t.Skip()
		}
		if !utf8.ValidString(value) {
			t.Skip()
		}
		var c Component
		c.Set(Property{Name: name, Value: value})

		// Round-trip with the exact same case.
		got, ok := c.Get(name)
		if !ok {
			t.Fatalf("Get(%q) ok=false after Set", name)
		}
		if got.Value != value {
			t.Fatalf("Get(%q).Value = %q, want %q", name, got.Value, value)
		}

		// Case-insensitive lookup must also work.
		gotLower, okLower := c.Get(strings.ToLower(name))
		if !okLower || gotLower.Value != value {
			t.Fatalf("case-insensitive Get(%q) failed: %+v ok=%v", strings.ToLower(name), gotLower, okLower)
		}
		gotUpper, okUpper := c.Get(strings.ToUpper(name))
		if !okUpper || gotUpper.Value != value {
			t.Fatalf("case-insensitive Get(%q) failed: %+v ok=%v", strings.ToUpper(name), gotUpper, okUpper)
		}

		// A guaranteed-distinct probe must miss.
		probe := "X-VSTAR-FUZZ-NEVERMATCH-" + name
		if _, ok := c.Get(probe); ok {
			t.Fatalf("Get(%q) ok=true on never-set probe", probe)
		}
	})
}

// validPropertyName matches the iCalendar/vCard "name" production
// (RFC 5545 §3.2 iana-token / RFC 6350 §3.3): one or more ALPHA /
// DIGIT / "-", first char alphabetic. Conservative: rejects empty,
// vendor-prefix punctuation, and anything outside ASCII.
func validPropertyName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		case r == '-':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
