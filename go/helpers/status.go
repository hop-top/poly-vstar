// SPDX-License-Identifier: MIT

package helpers

import (
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
)

// statusProp is the wire name for the STATUS property
// (RFC 5545 §3.8.1.11). The RFC scopes a distinct value vocabulary
// to each of VEVENT, VTODO and VJOURNAL; the property name is
// shared.
const statusProp = "STATUS"

// percentProp is the wire name for the VTODO PERCENT-COMPLETE
// property (RFC 5545 §3.8.1.8). Always carries an unsigned-int 0-100.
const percentProp = "PERCENT-COMPLETE"

// validTodoStatus reports whether s is one of the four wire values
// the spec enumerates for VTODO STATUS. Anything else is rejected by
// SetStatus and reported as ok=false by Status.
func validTodoStatus(s vstar.TodoStatus) bool {
	switch s {
	case vstar.TodoNeedsAction, vstar.TodoInProcess, vstar.TodoCompleted, vstar.TodoCancelled:
		return true
	default:
		return false
	}
}

// Status returns the parsed STATUS value of c when present and valid.
// Returns ok=false when STATUS is absent or carries an unrecognized
// value. Status does not enforce that c is a VTODO — callers wanting
// VTODO-specific semantics should gate on c.Type themselves.
func Status(c vstar.Component) (vstar.TodoStatus, bool) {
	p, ok := c.Get(statusProp)
	if !ok {
		return "", false
	}
	s := vstar.TodoStatus(p.Value)
	if !validTodoStatus(s) {
		return "", false
	}
	return s, true
}

// SetStatus writes the STATUS property on c and refreshes the
// X-VSTAR-HASH last. SetStatus is a no-op when:
//
//   - c is nil
//   - c.Type is not CompTodo (the four TodoStatus values are VTODO-
//     specific; VEVENT and VJOURNAL carry their own vocabularies —
//     see SetEventStatus and SetJournalStatus)
//   - s is not one of the four valid TodoStatus constants
//
// The no-op-on-mismatch shape is the Go convention for optional
// mutators on type-mixed inputs: callers don't get a false sense of
// success but also don't have to set up error-handling for a benign
// mistake.
func SetStatus(c *vstar.Component, s vstar.TodoStatus) {
	if c == nil || c.Type != vstar.CompTodo || !validTodoStatus(s) {
		return
	}
	c.Set(vstar.Property{Name: statusProp, Value: string(s)})
	hashing.SetXVSTAR(c)
}

// Complete finalizes a VTODO atomically: STATUS=COMPLETED,
// COMPLETED=t (UTC form), PERCENT-COMPLETE=100, X-VSTAR-HASH
// refreshed last.
//
// No-op when c is nil or c.Type is not CompTodo. The semantic mirrors
// the crm/internal/vcal precedent: if you call Complete you get the
// full set of "done" markers in one call and the stored hash matches.
func Complete(c *vstar.Component, t time.Time) {
	if c == nil || c.Type != vstar.CompTodo {
		return
	}
	c.Set(vstar.Property{Name: statusProp, Value: string(vstar.TodoCompleted)})
	c.SetCOMPLETED(t)
	setPercentCompleteUnchecked(c, percentMax)
	hashing.SetXVSTAR(c)
}

// validEventStatus reports whether s is one of the three wire values
// RFC 5545 §3.8.1.11 enumerates for VEVENT STATUS.
func validEventStatus(s vstar.EventStatus) bool {
	switch s {
	case vstar.EventTentative, vstar.EventConfirmed, vstar.EventCancelled:
		return true
	default:
		return false
	}
}

// validJournalStatus reports whether s is one of the three wire
// values RFC 5545 §3.8.1.11 enumerates for VJOURNAL STATUS.
func validJournalStatus(s vstar.JournalStatus) bool {
	switch s {
	case vstar.JournalDraft, vstar.JournalFinal, vstar.JournalCancelled:
		return true
	default:
		return false
	}
}

// EventStatus returns the parsed STATUS value of c when present and
// valid *for VEVENT*. Returns ok=false when STATUS is absent or
// carries a value outside the VEVENT vocabulary — including values
// that are legal for another component type, so a VTODO's
// NEEDS-ACTION reports ok=false here.
//
// Like Status, EventStatus does not gate on c.Type: it answers
// "does this component carry a VEVENT-shaped STATUS". Callers
// wanting the type check too should test c.Type themselves.
func EventStatus(c vstar.Component) (vstar.EventStatus, bool) {
	p, ok := c.Get(statusProp)
	if !ok {
		return "", false
	}
	s := vstar.EventStatus(p.Value)
	if !validEventStatus(s) {
		return "", false
	}
	return s, true
}

// SetEventStatus writes the STATUS property on c and refreshes the
// X-VSTAR-HASH last. No-op when c is nil, c.Type is not CompEvent,
// or s is not one of the three valid EventStatus constants.
//
// The type guard is what keeps the three STATUS vocabularies from
// bleeding into each other: calling SetEventStatus on a VJOURNAL
// leaves the component untouched.
func SetEventStatus(c *vstar.Component, s vstar.EventStatus) {
	if c == nil || c.Type != vstar.CompEvent || !validEventStatus(s) {
		return
	}
	c.Set(vstar.Property{Name: statusProp, Value: string(s)})
	hashing.SetXVSTAR(c)
}

// JournalStatus returns the parsed STATUS value of c when present
// and valid *for VJOURNAL*. Returns ok=false when STATUS is absent
// or carries a value outside the VJOURNAL vocabulary.
//
// Like Status, JournalStatus does not gate on c.Type.
func JournalStatus(c vstar.Component) (vstar.JournalStatus, bool) {
	p, ok := c.Get(statusProp)
	if !ok {
		return "", false
	}
	s := vstar.JournalStatus(p.Value)
	if !validJournalStatus(s) {
		return "", false
	}
	return s, true
}

// SetJournalStatus writes the STATUS property on c and refreshes
// the X-VSTAR-HASH last. No-op when c is nil, c.Type is not
// CompJournal, or s is not one of the three valid JournalStatus
// constants.
func SetJournalStatus(c *vstar.Component, s vstar.JournalStatus) {
	if c == nil || c.Type != vstar.CompJournal || !validJournalStatus(s) {
		return
	}
	c.Set(vstar.Property{Name: statusProp, Value: string(s)})
	hashing.SetXVSTAR(c)
}
