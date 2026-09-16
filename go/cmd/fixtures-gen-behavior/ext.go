// SPDX-License-Identifier: MIT

package main

import (
	"path/filepath"
	"strings"

	"hop.top/vstar/ext"
)

// scopeFixture is one entry of ext/scopes.json: an extension name
// and the spec/04 scope it classifies into.
//
// System is the slug lifted out of a ScopeSystem name, and null for
// every other scope — it is the answer to "which system owns this
// property?", which only ScopeSystem names have.
type scopeFixture struct {
	Name   string  `json:"name"`
	Scope  string  `json:"scope"`
	System *string `json:"system"`
}

// writeExtFamily writes ext/scopes.json.
func writeExtFamily(dir string) (int, error) {
	if err := writeJSON(filepath.Join(dir, "scopes.json"), scopeFixtures()); err != nil {
		return 0, err
	}
	return 1, nil
}

// extNames is the classification table's input: every scope in
// spec/04, each with the edge cases that distinguish it from its
// neighbors.
var extNames = []string{
	// ScopeVStar — the cross-system tier.
	"X-VSTAR-HASH",
	"X-VSTAR-EFFECTIVE-STATUS",
	// Case-insensitivity: the classification does not depend on
	// the wire casing.
	"x-vstar-hash",

	// ScopeSystem — one owning system, slug extractable.
	"X-AGR-FOO",
	"X-AGR-INTENT",
	"X-ACME-TICKET-ID",
	// A system slug that merely starts with the reserved word is
	// still a system: the reservation is on the whole segment.
	"X-VSTARLIKE-FOO",

	// ScopeExperimental — the unstable tier.
	"X-EXP-DRAFT",
	"X-EXP-A",

	// ScopeNone — not an extension at all.
	"DTSTART",
	"SUMMARY",
	"X",
	"",

	// ScopeUnknown — has the prefix, matches no tier.
	"X-",
	"X-FOO",
	"X-VSTAR-",
	"X-EXP-",
}

// scopeFixtures runs the reference over extNames and records the
// classification. The scope token is lowercased: the reference's
// String() is capitalised for human display, and the fixtures use
// one flat convention so ports need no case mapping.
func scopeFixtures() []scopeFixture {
	out := make([]scopeFixture, 0, len(extNames))
	for _, name := range extNames {
		f := scopeFixture{
			Name:  name,
			Scope: strings.ToLower(ext.ScopeOf(name).String()),
		}
		if sys, ok := ext.SystemName(name); ok {
			f.System = strPtr(sys)
		}
		out = append(out, f)
	}
	return out
}
