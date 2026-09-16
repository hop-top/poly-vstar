// SPDX-License-Identifier: MIT

package helpers

import (
	"strconv"

	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
)

// sequenceProp is the wire name for the RFC 5545 §3.8.7.4 SEQUENCE
// property: the component's revision counter. Value is a
// non-negative integer; the RFC's default when the property is
// absent is 0.
const sequenceProp = "SEQUENCE"

// priorityProp is the wire name for the RFC 5545 §3.8.1.9 PRIORITY
// property. Value is an integer 0-9 where 0 means "undefined",
// 1 is the highest priority and 9 the lowest.
const priorityProp = "PRIORITY"

// priorityMin and priorityMax bound the RFC 5545 §3.8.1.9 value
// space. Note that priorityMin (0) is a legal *value* meaning
// "undefined" — it is not a sentinel for absence.
const (
	priorityMin = 0
	priorityMax = 9
)

// percentMin and percentMax bound the RFC 5545 §3.8.1.8
// PERCENT-COMPLETE value space.
const (
	percentMin = 0
	percentMax = 100
)

// parseUint reads s as a base-10 non-negative integer with no
// surrounding whitespace, sign, or other decoration. Returns
// ok=false for anything strconv.Atoi would reject as well as for
// negative values and leading/trailing space — the RFC 5545 value
// types here are "integer" with no permitted padding, so a sloppy
// wire value is treated as unparseable rather than coerced.
func parseUint(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, false
	}
	// strconv.Atoi accepts a leading "+" and "-"; reject any form
	// whose canonical rendering differs from the input so that
	// "+3", "03" and " 3" do not silently round-trip as 3.
	if strconv.Itoa(n) != s {
		return 0, false
	}
	return n, true
}

// carriesSequence reports whether SEQUENCE is applicable to t.
// RFC 5545 §3.8.7.4 scopes the property to VEVENT, VTODO and
// VJOURNAL.
func carriesSequence(t vstar.CompType) bool {
	switch t {
	case vstar.CompEvent, vstar.CompTodo, vstar.CompJournal:
		return true
	default:
		return false
	}
}

// carriesPriority reports whether PRIORITY is applicable to t.
// RFC 5545 §3.8.1.9 scopes the property to VEVENT and VTODO.
func carriesPriority(t vstar.CompType) bool {
	switch t {
	case vstar.CompEvent, vstar.CompTodo:
		return true
	default:
		return false
	}
}

// Sequence returns the parsed SEQUENCE revision counter of c.
//
// Returns ok=false when SEQUENCE is absent or carries a value that
// is not a canonical non-negative integer. Callers wanting the RFC
// default should read that as 0: RFC 5545 §3.8.7.4 says a component
// with no SEQUENCE is revision 0.
//
// Sequence does not gate on c.Type — a reader should surface what
// the wire actually holds. Type applicability is enforced by the
// mutators.
func Sequence(c vstar.Component) (int, bool) {
	p, ok := c.Get(sequenceProp)
	if !ok {
		return 0, false
	}
	return parseUint(p.Value)
}

// SetSequence writes the SEQUENCE revision counter on c and
// refreshes X-VSTAR-HASH last.
//
// SetSequence is a no-op when:
//
//   - c is nil
//   - c.Type does not carry SEQUENCE (RFC 5545 §3.8.7.4 scopes it
//     to VEVENT, VTODO and VJOURNAL)
//   - n is negative
//
// Out-of-range input is rejected rather than clamped: a negative
// revision counter is a caller bug, and silently rewriting it to 0
// would advertise a revision the caller never intended. The no-op
// shape matches SetStatus — no error return, no half-applied
// mutation, and no hash churn when nothing changed.
func SetSequence(c *vstar.Component, n int) {
	if c == nil || !carriesSequence(c.Type) || n < 0 {
		return
	}
	c.Set(vstar.Property{Name: sequenceProp, Value: strconv.Itoa(n)})
	hashing.SetXVSTAR(c)
}

// IncrementSequence bumps the SEQUENCE revision counter on c by one
// and refreshes X-VSTAR-HASH last.
//
// "Bump the revision" is the actual use case for SEQUENCE, and
// doing it as a read-then-write at call sites leaves a window in
// which a concurrent mutator can interleave. IncrementSequence
// closes that window for a single Component.
//
// An absent or unparseable SEQUENCE is treated as the RFC default
// of 0, so the first increment yields 1. No-op when c is nil or
// c.Type does not carry SEQUENCE.
func IncrementSequence(c *vstar.Component) {
	if c == nil || !carriesSequence(c.Type) {
		return
	}
	n, ok := Sequence(*c)
	if !ok {
		n = 0
	}
	c.Set(vstar.Property{Name: sequenceProp, Value: strconv.Itoa(n + 1)})
	hashing.SetXVSTAR(c)
}

// Priority returns the parsed PRIORITY of c.
//
// Returns ok=false when PRIORITY is absent or carries a value
// outside the RFC 5545 §3.8.1.9 range 0-9. The (value, ok) shape
// keeps "undefined" distinct from "not present": an explicit
// PRIORITY:0 returns (0, true) — the RFC's "undefined priority" —
// whereas a component with no PRIORITY at all returns (0, false).
// Use RemovePriority to move from the former to the latter.
func Priority(c vstar.Component) (int, bool) {
	p, ok := c.Get(priorityProp)
	if !ok {
		return 0, false
	}
	n, ok := parseUint(p.Value)
	if !ok || n < priorityMin || n > priorityMax {
		return 0, false
	}
	return n, true
}

// SetPriority writes PRIORITY on c and refreshes X-VSTAR-HASH last.
//
// SetPriority is a no-op when:
//
//   - c is nil
//   - c.Type does not carry PRIORITY (RFC 5545 §3.8.1.9 scopes it
//     to VEVENT and VTODO)
//   - n is outside 0-9
//
// Out-of-range input is rejected, not clamped. Clamping would turn
// a caller's off-by-one (say 10) into a legitimate-looking lowest
// priority of 9, hiding the bug in data that later round-trips
// cleanly. Rejection also leaves any existing PRIORITY untouched,
// so a bad call cannot corrupt a good value, and skips the hash
// refresh because nothing changed.
//
// Passing 0 is not rejection — it writes PRIORITY:0, the RFC's
// explicit "undefined" marker.
func SetPriority(c *vstar.Component, n int) {
	if c == nil || !carriesPriority(c.Type) || n < priorityMin || n > priorityMax {
		return
	}
	c.Set(vstar.Property{Name: priorityProp, Value: strconv.Itoa(n)})
	hashing.SetXVSTAR(c)
}

// RemovePriority deletes PRIORITY from c and refreshes
// X-VSTAR-HASH last. This is the counterpart to SetPriority(c, 0):
// removal means "no priority stated", whereas 0 means "priority
// explicitly undefined".
//
// No-op when c is nil. Unlike SetPriority this does not gate on
// c.Type — removing a property that should not be there is always
// safe.
func RemovePriority(c *vstar.Component) {
	if c == nil {
		return
	}
	c.Remove(priorityProp)
	hashing.SetXVSTAR(c)
}

// PercentComplete returns the parsed PERCENT-COMPLETE of c.
//
// Returns ok=false when the property is absent or carries a value
// outside the RFC 5545 §3.8.1.8 range 0-100. As with Priority, an
// explicit 0 ("started, nothing done") is distinct from absence and
// returns (0, true).
func PercentComplete(c vstar.Component) (int, bool) {
	p, ok := c.Get(percentProp)
	if !ok {
		return 0, false
	}
	n, ok := parseUint(p.Value)
	if !ok || n < percentMin || n > percentMax {
		return 0, false
	}
	return n, true
}

// SetPercentComplete writes PERCENT-COMPLETE on c and refreshes
// X-VSTAR-HASH last.
//
// SetPercentComplete is a no-op when:
//
//   - c is nil
//   - c.Type is not CompTodo (RFC 5545 §3.8.1.8 scopes the property
//     to VTODO)
//   - n is outside 0-100
//
// Out-of-range input is rejected rather than clamped, for the same
// reason as SetPriority: clamping 120 to 100 would silently assert
// the task is finished.
//
// Note that setting 100 does not by itself mark a VTODO done — use
// Complete for that, which also writes STATUS and COMPLETED.
func SetPercentComplete(c *vstar.Component, n int) {
	if c == nil || c.Type != vstar.CompTodo || n < percentMin || n > percentMax {
		return
	}
	c.Set(vstar.Property{Name: percentProp, Value: strconv.Itoa(n)})
	hashing.SetXVSTAR(c)
}

// RemovePercentComplete deletes PERCENT-COMPLETE from c and
// refreshes X-VSTAR-HASH last. No-op when c is nil; does not gate
// on c.Type (removal is always safe).
func RemovePercentComplete(c *vstar.Component) {
	if c == nil {
		return
	}
	c.Remove(percentProp)
	hashing.SetXVSTAR(c)
}

// setPercentCompleteUnchecked writes PERCENT-COMPLETE without the
// nil/type/range guards. It exists so Complete — which has already
// established c is a non-nil VTODO and passes a constant in range —
// can share the property-name and formatting choices with
// SetPercentComplete instead of duplicating a literal.
//
// It deliberately does not refresh the hash: its callers batch
// several mutations and refresh once, last.
func setPercentCompleteUnchecked(c *vstar.Component, n int) {
	c.Set(vstar.Property{Name: percentProp, Value: strconv.Itoa(n)})
}
