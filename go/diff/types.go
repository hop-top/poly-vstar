// SPDX-License-Identifier: MIT

// Package diff provides semantic equality and structural diff for V*
// values. "Semantic" means via canonical form: two values that yield
// identical canonical bytes are reported equal regardless of property
// order, parameter order, datetime form, or whitespace. Diff returns
// property-level structural changes suitable for human display or
// programmatic inspection.
//
// Public API (mirroring the canonical package's per-type pattern,
// with `Of`-prefixed diff entrypoints to dodge the function/type
// name collision Go disallows — see OfComponent's doc):
//
//   - Component(a, b vstar.Component) bool — semantic Equal.
//   - Card(a, b vstar.Card) bool          — semantic Equal.
//   - Calendar(a, b vstar.Calendar) bool  — semantic Equal.
//   - OfComponent(a, b vstar.Component) ComponentDiff
//   - OfCard(a, b vstar.Card) ComponentDiff
//   - OfCalendar(a, b vstar.Calendar) []ComponentDiff
//   - (ComponentDiff).String() string for unified-diff-ish rendering.
//
// Equality rules (delegated to canonical):
//
//   - X-VSTAR-HASH is excluded from comparison and from diff output
//     on both sides (mirrors canonical's spec/03 rule 7 exclusion).
//   - Property/parameter ordering is irrelevant.
//   - Datetime forms compare equal when canonical resolves them to
//     the same UTC representation (TZID-tagged ↔ Z-suffixed where
//     a matching VTIMEZONE is in scope).
//
// Diff matching heuristics (documented limitations for v0.1):
//
//   - Sub-components are paired by (Type, UID) when both carry a
//     UID. Sub-components without a UID (e.g. VALARM) are paired
//     positionally by index in Sub. Reordering UID-less Subs will
//     therefore surface as Added+Removed rather than Changed.
//   - PropertyDiff entries within ComponentDiff.Properties are
//     emitted sorted by Property.Name (alphabetical, case-
//     insensitive). This matches canonical's property ordering rule
//     in practice but is specified independently for the diff layer.
//
// Performance: v0.1 prioritizes correctness. Equal currently routes
// through a full canonical-byte comparison; per-property short-
// circuiting is out of scope.
package diff

import (
	vstar "hop.top/vstar"
)

// DiffOp identifies the kind of property change recorded in a
// PropertyDiff entry.
type DiffOp int

const (
	// OpAdded marks a property present in b but missing from a.
	OpAdded DiffOp = iota + 1
	// OpRemoved marks a property present in a but missing from b.
	OpRemoved
	// OpChanged marks a property present in both with differing
	// Value or Params.
	OpChanged
)

// diffOpUnknown is the rendering used for any DiffOp value outside
// the defined enum range. Promoted to a constant so all unknown
// branches share one source of truth.
const diffOpUnknown = "Unknown"

// String renders the DiffOp as a short human-readable token. Used by
// (ComponentDiff).String().
func (o DiffOp) String() string {
	switch o {
	case OpAdded:
		return "Added"
	case OpRemoved:
		return "Removed"
	case OpChanged:
		return "Changed"
	default:
		return diffOpUnknown
	}
}

// PropertyDiff describes a single property-level change between two
// V* values.
//
// For OpAdded, Property is the new property (the b-side value) and
// Old is the zero Property. For OpRemoved, Property is the original
// property (the a-side value) and Old is the zero Property. For
// OpChanged, Property is the new (b-side) property and Old is the
// original (a-side) property.
type PropertyDiff struct {
	Op       DiffOp
	Property vstar.Property
	Old      vstar.Property
}

// ComponentDiff describes the structural changes between two
// components (or two cards, or a single calendar entry pair). Path
// identifies the diff site for human display:
//
//   - "" for a top-level Component or Card diff.
//   - "VCALENDAR.VEVENT[uid=…]" for entries returned by
//     CalendarDiff.
//   - "<parent path>.<SubType>[uid=…]" or
//     "<parent path>.<SubType>[#index]" for nested SubDiffs.
//
// Properties holds property-level changes at this level, sorted by
// Property.Name (alphabetical, case-insensitive). SubDiffs holds
// recursive diffs for sub-components that themselves changed.
type ComponentDiff struct {
	Path       string
	Properties []PropertyDiff
	SubDiffs   []ComponentDiff
}

// Empty reports whether the diff records no changes at this level
// nor in any nested sub-component. Useful for callers wanting to
// short-circuit on identical inputs.
func (d ComponentDiff) Empty() bool {
	if len(d.Properties) > 0 {
		return false
	}
	for _, s := range d.SubDiffs {
		if !s.Empty() {
			return false
		}
	}
	return true
}
