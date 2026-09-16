// SPDX-License-Identifier: MIT

package helpers

import (
	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
)

// classProp is the wire name for the CLASS property
// (RFC 5545 §3.8.1.3). Applies to VEVENT, VTODO and VJOURNAL.
const classProp = "CLASS"

// transpProp is the wire name for the TRANSP property
// (RFC 5545 §3.8.2.7). Applies to VEVENT only.
const transpProp = "TRANSP"

// validClass reports whether v is one of the three wire values
// RFC 5545 §3.8.1.3 enumerates for CLASS.
func validClass(v vstar.Class) bool {
	switch v {
	case vstar.ClassPublic, vstar.ClassPrivate, vstar.ClassConfidential:
		return true
	default:
		return false
	}
}

// classApplies reports whether ct is a component type that admits a
// CLASS property per RFC 5545 §3.8.1.3.
func classApplies(ct vstar.CompType) bool {
	switch ct {
	case vstar.CompEvent, vstar.CompTodo, vstar.CompJournal:
		return true
	default:
		return false
	}
}

// validTransp reports whether v is one of the two wire values
// RFC 5545 §3.8.2.7 enumerates for TRANSP.
func validTransp(v vstar.Transp) bool {
	switch v {
	case vstar.TranspOpaque, vstar.TranspTransparent:
		return true
	default:
		return false
	}
}

// Class returns the parsed CLASS value of c when present and valid.
// Returns ok=false when CLASS is absent or carries an unrecognized
// value.
//
// An absent CLASS reports ok=false rather than (ClassPublic, true),
// even though RFC 5545 §3.8.1.3 assigns PUBLIC by default. The
// getter reports what is on the wire; the default is a separate,
// opt-in question answered by ClassOrDefault. Two reasons:
//
//   - "(value, bool)" means "present and recognized" everywhere else
//     in this package (Status, EventStatus, Transp, …). Having one
//     getter alone return ok=true for an absent property would make
//     the flag untrustworthy.
//   - Callers that must distinguish "explicitly PUBLIC" from
//     "unset" — round-trip fidelity, diffing, supersession
//     projection — cannot recover that distinction once the getter
//     has folded it away, whereas callers that only want the
//     effective value get it from ClassOrDefault at zero cost.
//
// This mirrors how the package already treats the other defaulted
// property it models: RelatedTo applies the RFC's "PARENT" default
// for an omitted RELTYPE, but does so in a function that returns
// the effective value with no ok flag to contradict.
func Class(c vstar.Component) (vstar.Class, bool) {
	p, ok := c.Get(classProp)
	if !ok {
		return "", false
	}
	v := vstar.Class(p.Value)
	if !validClass(v) {
		return "", false
	}
	return v, true
}

// ClassOrDefault returns the effective CLASS of c, applying the
// RFC 5545 §3.8.1.3 default of PUBLIC when the property is absent
// or carries an unrecognized value.
//
// Unrecognized values fall back to the default rather than being
// surfaced: the RFC treats an unknown CLASS token as equivalent to
// PRIVATE only for IANA/X- registrations it cannot resolve, and V*
// does not model those. Flagging the value is the validate
// package's job (VS044's sibling rules), not this accessor's.
func ClassOrDefault(c vstar.Component) vstar.Class {
	if v, ok := Class(c); ok {
		return v
	}
	return vstar.ClassPublic
}

// SetClass writes the CLASS property on c and refreshes the
// X-VSTAR-HASH last. No-op when c is nil, c.Type does not admit
// CLASS (VEVENT, VTODO, VJOURNAL per RFC 5545 §3.8.1.3), or v is
// not one of the three valid Class constants.
func SetClass(c *vstar.Component, v vstar.Class) {
	if c == nil || !classApplies(c.Type) || !validClass(v) {
		return
	}
	c.Set(vstar.Property{Name: classProp, Value: string(v)})
	hashing.SetXVSTAR(c)
}

// Transp returns the parsed TRANSP value of c when present and
// valid. Returns ok=false when TRANSP is absent or carries an
// unrecognized value. See Class for why an absent property is not
// reported as the RFC default here; use TranspOrDefault for the
// effective value.
func Transp(c vstar.Component) (vstar.Transp, bool) {
	p, ok := c.Get(transpProp)
	if !ok {
		return "", false
	}
	v := vstar.Transp(p.Value)
	if !validTransp(v) {
		return "", false
	}
	return v, true
}

// TranspOrDefault returns the effective TRANSP of c, applying the
// RFC 5545 §3.8.2.7 default of OPAQUE when the property is absent
// or carries an unrecognized value. OPAQUE means the event
// consumes free/busy time.
func TranspOrDefault(c vstar.Component) vstar.Transp {
	if v, ok := Transp(c); ok {
		return v
	}
	return vstar.TranspOpaque
}

// SetTransp writes the TRANSP property on c and refreshes the
// X-VSTAR-HASH last. No-op when c is nil, c.Type is not CompEvent
// (TRANSP is VEVENT-only per RFC 5545 §3.8.2.7), or v is not one of
// the two valid Transp constants.
func SetTransp(c *vstar.Component, v vstar.Transp) {
	if c == nil || c.Type != vstar.CompEvent || !validTransp(v) {
		return
	}
	c.Set(vstar.Property{Name: transpProp, Value: string(v)})
	hashing.SetXVSTAR(c)
}
