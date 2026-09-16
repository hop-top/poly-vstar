// SPDX-License-Identifier: MIT

package rrule

// ValidateRRule reports whether s would parse cleanly via
// ParseRRule. Returns nil on success; on failure returns the same
// error ParseRRule would (wrapped vstar.ErrMalformed for syntactic
// problems, wrapped ErrUnsupportedRRule for v0.2-deferred features).
//
// ValidateRRule is a convenience wrapper for boundary checks
// where the caller only needs a yes/no answer (e.g. validate
// package's VS050/VS051 wiring). It calls ParseRRule and discards
// the structured Rule; allocations are equivalent. A future
// fast-path implementation could skip Rule construction if a
// real perf need surfaces.
func ValidateRRule(s string) error {
	_, err := ParseRRule(s)
	return err
}
