// SPDX-License-Identifier: MIT

package helpers

import (
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
)

// categoriesProp is the wire name for the RFC 5545 §3.8.1.2
// CATEGORIES property. Values are comma-separated text labels.
const categoriesProp = "CATEGORIES"

// Categories returns the comma-separated CATEGORIES values of c as
// a slice. Whitespace adjacent to commas is trimmed, and empty
// tokens (from leading/trailing commas or "a,,b" runs) are dropped.
//
// Returns nil when CATEGORIES is absent or contains no non-empty
// tokens.
//
// Comparison/dedupe semantics live in SetCategories and AddCategory:
// readers preserve user input as-is.
func Categories(c vstar.Component) []string {
	p, ok := c.Get(categoriesProp)
	if !ok {
		return nil
	}
	raw := strings.Split(p.Value, ",")
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SetCategories replaces CATEGORIES with the comma-joined version of
// values (no space after the comma — RFC 5545 §3.3.11 admits both
// forms but the no-space variant is the canonical wire form).
//
// Empty input removes the property. Duplicate values are dropped
// while preserving first-seen order; comparison is case-sensitive
// (CATEGORIES are user-facing labels per the RFC, not registry
// tokens — "Work" and "work" are distinct).
//
// Refreshes X-VSTAR-HASH last on success. No-op when c is nil.
func SetCategories(c *vstar.Component, values []string) {
	if c == nil {
		return
	}
	deduped := dedupePreserve(values)
	if len(deduped) == 0 {
		c.Remove(categoriesProp)
		hashing.SetXVSTAR(c)
		return
	}
	c.Set(vstar.Property{Name: categoriesProp, Value: strings.Join(deduped, ",")})
	hashing.SetXVSTAR(c)
}

// AddCategory appends one category to CATEGORIES if not already
// present (case-sensitive comparison). Empty value is a no-op (no
// hash refresh, since nothing changed).
//
// Refreshes X-VSTAR-HASH last on success. No-op when c is nil.
func AddCategory(c *vstar.Component, value string) {
	if c == nil || value == "" {
		return
	}
	current := Categories(*c)
	for _, existing := range current {
		if existing == value {
			return
		}
	}
	next := append(current, value) //nolint:gocritic // intentional copy then append; current is freshly allocated
	c.Set(vstar.Property{Name: categoriesProp, Value: strings.Join(next, ",")})
	hashing.SetXVSTAR(c)
}

// dedupePreserve returns values with empty strings dropped and
// duplicates removed while preserving first-seen order. Values are
// trimmed of leading/trailing whitespace before compare and emit.
func dedupePreserve(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
