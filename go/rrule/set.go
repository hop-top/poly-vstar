// SPDX-License-Identifier: MIT

package rrule

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	vstar "hop.top/vstar"
)

// Set is a complete recurrence definition for one component: a
// DTSTART anchor, an optional RRULE, plus the explicit RDATE
// additions and EXDATE removals that RFC 5545 §3.8.5 layers on
// top.
//
// Expansion order per RFC 5545 §3.8.5.1/§3.8.5.2:
//
//  1. DTSTART is occurrence #1.
//  2. The RRULE (if any) expands from DTSTART.
//  3. RDATE values are merged in.
//  4. EXDATE values are removed — EXDATE is applied LAST, so an
//     excluded instant stays excluded even when an RDATE names it.
//  5. The result is sorted chronologically and de-duplicated.
//
// A Set with no RRule is legal: an RDATE-only recurrence is a
// finite, explicitly-enumerated series. A Set with neither RRule
// nor RDate is a single non-recurring occurrence at DTStart.
//
// Comparison for both dedupe and EXDATE matching is by instant
// (time.Time.Equal), not by wall-clock fields, so a UTC EXDATE
// correctly cancels a zoned occurrence at the same instant.
type Set struct {
	// DTStart is the component's DTSTART. It anchors the RRULE
	// expansion and is itself occurrence #1 (unless an EXDATE
	// removes it).
	DTStart time.Time
	// RRule is the recurrence rule, or nil for an RDATE-only /
	// non-recurring set.
	RRule *Rule
	// RDate holds explicit additional occurrences (RFC 5545
	// §3.8.5.2). Order is irrelevant; duplicates are harmless.
	RDate []time.Time
	// ExDate holds explicit exclusions (RFC 5545 §3.8.5.1). An
	// entry that matches no occurrence is silently ignored, per
	// the RFC.
	ExDate []time.Time
}

// Occurrences returns up to limit occurrences of the set in
// chronological order, with the same complete-flag contract as the
// package-level Occurrences: complete is true when the series
// genuinely ended within the limit, false when the limit truncated
// it.
//
// An RDATE-only set is always complete (it is finite by
// construction) provided the limit accommodates every value.
//
// EXDATE removals do NOT consume limit slots: the limit bounds
// returned occurrences, so a set whose first 100 RRULE occurrences
// are all excluded still yields the 101st.
//
// A rule that never yields is reported as a wrapped ErrIterationCap
// (see Occurrences); RDATE additions do not make such a series
// finite.
func (s Set) Occurrences(limit int) ([]time.Time, bool, error) {
	if limit < 0 {
		return nil, false, fmt.Errorf("rrule: Set.Occurrences: limit must be >= 0, got %d: %w", limit, ErrUnboundedExpansion)
	}
	if limit == 0 {
		return nil, false, nil
	}
	if s.RRule != nil {
		if err := checkExpandable(*s.RRule); err != nil {
			return nil, false, err
		}
	}

	excluded := s.ExDate
	// Explicit occurrences (DTSTART + RDATE) are finite; expand
	// them up front so they can be merged into the rule stream in
	// chronological order.
	explicit := s.explicitOccurrences()

	out := make([]time.Time, 0, limit)
	ei := 0

	// De-duplication is handled structurally rather than here:
	// `explicit` is already sorted and de-duped, and the merge loop
	// below advances past an explicit entry that coincides with a
	// rule occurrence. So emit never sees the same instant twice.
	//
	// emit reports false once the limit is full, which is the ONLY
	// way the merge stops early — so every early return below is a
	// truncation (complete=false), and falling through to the end
	// means the series genuinely ended (complete=true).
	emit := func(t time.Time) bool {
		if containsInstant(excluded, t) {
			return true
		}
		if len(out) == limit {
			return false
		}
		out = append(out, t)
		return true
	}

	if s.RRule != nil {
		truncated := false
		capped := walk(*s.RRule, s.DTStart, func(occ time.Time) bool {
			// Drain every explicit occurrence that sorts before
			// this rule occurrence, preserving global order.
			for ei < len(explicit) && explicit[ei].Before(occ) {
				if !emit(explicit[ei]) {
					truncated = true
					return false
				}
				ei++
			}
			if ei < len(explicit) && explicit[ei].Equal(occ) {
				ei++ // same instant; emit once below
			}
			if !emit(occ) {
				truncated = true
				return false
			}
			return true
		})
		if capped {
			return nil, false, iterationCapError("Set.Occurrences", *s.RRule)
		}
		if truncated {
			return out, false, nil
		}
	}
	for ; ei < len(explicit); ei++ {
		if !emit(explicit[ei]) {
			return out, false, nil
		}
	}

	if len(out) == 0 {
		return nil, true, nil
	}
	return out, true, nil
}

// Between returns every occurrence of the set in the half-open
// window [start, end), applying RDATE additions and EXDATE
// removals. Same window and iteration-bound rules as the
// package-level Between: end must be non-zero and strictly after
// start, and a rule that never yields is a wrapped ErrIterationCap.
func (s Set) Between(start, end time.Time) ([]time.Time, error) {
	if end.IsZero() {
		return nil, fmt.Errorf("rrule: Set.Between: end must be non-zero: %w", ErrUnboundedExpansion)
	}
	if !end.After(start) {
		return nil, fmt.Errorf("rrule: Set.Between: end %s must be after start %s: %w", end, start, ErrUnboundedExpansion)
	}

	merged := []time.Time{}
	if s.RRule != nil {
		if err := checkExpandable(*s.RRule); err != nil {
			return nil, err
		}
		capped := walk(*s.RRule, s.DTStart, func(occ time.Time) bool {
			if !occ.Before(end) {
				return false
			}
			merged = append(merged, occ)
			return true
		})
		if capped {
			return nil, iterationCapError("Set.Between", *s.RRule)
		}
	}
	merged = append(merged, s.explicitOccurrences()...)

	out := make([]time.Time, 0, len(merged))
	for _, t := range merged {
		if t.Before(start) || !t.Before(end) {
			continue
		}
		if containsInstant(s.ExDate, t) {
			continue
		}
		out = append(out, t)
	}
	return sortDedupe(out), nil
}

// explicitOccurrences returns DTSTART plus every RDATE, sorted and
// de-duplicated. These are the occurrences that exist independently
// of any RRULE.
func (s Set) explicitOccurrences() []time.Time {
	out := make([]time.Time, 0, len(s.RDate)+1)
	if !s.DTStart.IsZero() {
		out = append(out, s.DTStart)
	}
	out = append(out, s.RDate...)
	return sortDedupe(out)
}

// sortDedupe sorts by instant and removes duplicates, returning a
// new slice. Equality is time.Time.Equal (same instant), so values
// in different locations collapse correctly.
func sortDedupe(ts []time.Time) []time.Time {
	if len(ts) == 0 {
		return nil
	}
	out := make([]time.Time, len(ts))
	copy(out, ts)
	slices.SortFunc(out, func(a, b time.Time) int { return a.Compare(b) })
	return slices.CompactFunc(out, func(a, b time.Time) bool { return a.Equal(b) })
}

// containsInstant reports whether ts holds a time equal (by
// instant) to t.
func containsInstant(ts []time.Time, t time.Time) bool {
	return slices.ContainsFunc(ts, t.Equal)
}

// ── Property-level construction ───────────────────────────────────

// SetFromComponent builds a Set from a component's DTSTART, RRULE,
// RDATE, and EXDATE properties.
//
// EXDATE and RDATE may each appear multiple times and may each
// carry multiple comma-separated values; every value accumulates.
//
// Value-type support is limited to RFC 5545 form #2 (UTC,
// Z-suffixed) datetimes: spec/03 §Recurrence sets fixes
// recurrence sets as UTC-only, matching vstar.ParseTime's strict
// posture and the UTC-only rule for RRULE bounds. Returns a wrapped
// ErrUnsupportedRRule when an EXDATE/RDATE carries:
//
//   - VALUE=DATE — date-only values are outside the spec's
//     recurrence-set scope; resolving one to an instant
//     would mean guessing a time-of-day.
//   - TZID — EXDATE and RDATE are not on spec/03's datetime
//     resolution allow-list, so a zoned value reaches this
//     constructor unresolved and would need the calendar's
//     VTIMEZONE registry, which a Component-scoped constructor
//     cannot reach (compare vstar.ParseTimeWithTZID, which takes a
//     Calendar).
//
// Failing closed on both is deliberate: silently dropping an
// unparseable EXDATE would surface an occurrence the producer
// explicitly canceled.
func SetFromComponent(c vstar.Component) (Set, error) {
	var set Set

	if p, ok := c.Get("DTSTART"); ok {
		t, ok := vstar.ParseTime(p.Value)
		if !ok {
			return Set{}, fmt.Errorf("rrule: DTSTART %q is not RFC 5545 form #2: %w", p.Value, vstar.ErrMalformed)
		}
		set.DTStart = t
	}

	for _, p := range c.Props {
		switch strings.ToUpper(p.Name) {
		case "RRULE":
			r, err := ParseRRule(p.Value)
			if err != nil {
				return Set{}, err
			}
			set.RRule = &r
		case "RDATE":
			ts, err := parseDateListProperty(p)
			if err != nil {
				return Set{}, err
			}
			set.RDate = append(set.RDate, ts...)
		case "EXDATE":
			ts, err := parseDateListProperty(p)
			if err != nil {
				return Set{}, err
			}
			set.ExDate = append(set.ExDate, ts...)
		}
	}

	set.RDate = sortDedupe(set.RDate)
	set.ExDate = sortDedupe(set.ExDate)
	return set, nil
}

// parseDateListProperty validates an EXDATE/RDATE property's
// parameters, then parses its multi-valued datetime list.
func parseDateListProperty(p vstar.Property) ([]time.Time, error) {
	name := strings.ToUpper(p.Name)
	for _, param := range p.Params {
		switch strings.ToUpper(param.Name) {
		case "VALUE":
			if !strings.EqualFold(param.Value, "DATE-TIME") {
				return nil, fmt.Errorf("rrule: %s VALUE=%s is outside the supported value types (DATE-TIME only): %w", name, param.Value, ErrUnsupportedRRule)
			}
		case "TZID":
			return nil, fmt.Errorf("rrule: %s TZID=%s requires VTIMEZONE resolution unavailable at component scope: %w", name, param.Value, ErrUnsupportedRRule)
		}
	}
	ts, err := ParseDateTimeList(p.Value)
	if err != nil {
		return nil, fmt.Errorf("rrule: %s: %w", name, err)
	}
	return ts, nil
}

// ParseDateTimeList parses a comma-separated list of RFC 5545 form
// #2 (UTC, Z-suffixed) datetimes — the value form of a
// DATE-TIME-valued EXDATE or RDATE property.
//
// The result is sorted chronologically and de-duplicated, so
// callers get a canonical set regardless of producer ordering.
//
// Returns a wrapped vstar.ErrMalformed for an empty input or any
// value that is not form #2 — including date-only (VALUE=DATE)
// values, which the spec's recurrence sets exclude (see
// SetFromComponent).
func ParseDateTimeList(s string) ([]time.Time, error) {
	if s == "" {
		return nil, fmt.Errorf("rrule: empty date-time list: %w", vstar.ErrMalformed)
	}
	parts := strings.Split(s, ",")
	out := make([]time.Time, 0, len(parts))
	for _, raw := range parts {
		t, ok := vstar.ParseTime(raw)
		if !ok {
			return nil, fmt.Errorf("rrule: %q is not an RFC 5545 form #2 date-time: %w", raw, vstar.ErrMalformed)
		}
		out = append(out, t)
	}
	return sortDedupe(out), nil
}

// FormatDateTimeList renders times as an EXDATE/RDATE property
// value: comma-separated RFC 5545 form #2 UTC datetimes, sorted
// and de-duplicated so identical logical content yields identical
// bytes. Returns "" for an empty input.
func FormatDateTimeList(times []time.Time) string {
	sorted := sortDedupe(times)
	if len(sorted) == 0 {
		return ""
	}
	parts := make([]string, len(sorted))
	for i, t := range sorted {
		parts[i] = vstar.FormatTime(t)
	}
	return strings.Join(parts, ",")
}

// ── RECURRENCE-ID ─────────────────────────────────────────────────

// Range is the RECURRENCE-ID RANGE parameter (RFC 5545 §3.2.13).
type Range int

// Range values. The zero value is RangeThisInstance, matching the
// RFC default when the RANGE parameter is absent.
const (
	// RangeThisInstance is the default: the RECURRENCE-ID
	// identifies exactly one instance of the series.
	RangeThisInstance Range = iota
	// RangeThisAndFuture is RANGE=THISANDFUTURE: the override
	// applies to the identified instance and every later one.
	RangeThisAndFuture
)

// String renders the Range as its wire token. RangeThisInstance
// renders "" because the RFC default is expressed by omitting the
// RANGE parameter entirely.
func (r Range) String() string {
	switch r {
	case RangeThisInstance:
		return ""
	case RangeThisAndFuture:
		return "THISANDFUTURE"
	default:
		return "Range(" + strconv.Itoa(int(r)) + ")"
	}
}

// RecurrenceID is the typed form of an RFC 5545 §3.8.4.4
// RECURRENCE-ID property: the DTSTART-equivalent instant that
// identifies which instance of a recurring series a component
// overrides, plus the RANGE parameter.
//
// # What this type does and does not do
//
// This is parsing and typed access only. Applying overrides — i.e.
// taking a base component plus a sibling set of RECURRENCE-ID
// components and producing the effective series, with modified
// instances substituted and THISANDFUTURE ranges propagated —
// is NOT implemented. That operation needs component-level
// semantics (which properties an override replaces versus
// inherits, how SEQUENCE arbitrates conflicts, what happens when
// an override's DTSTART moves it outside the series) that belong
// above this package, alongside the supersession layer.
//
// Consumers that need override resolution today can build it on
// this type plus Set: expand the base Set, then replace instances
// whose start matches a RecurrenceID.Time.
type RecurrenceID struct {
	// Time is the identified instance's original start instant —
	// the value the base series' expansion produces for it, NOT
	// the overriding component's own (possibly moved) DTSTART.
	Time time.Time
	// Range is the RANGE parameter; RangeThisInstance when absent.
	Range Range
}

// ParseRecurrenceID extracts a RecurrenceID from a RECURRENCE-ID
// property.
//
// Returns a wrapped vstar.ErrMalformed when the property is not
// named RECURRENCE-ID, when its value is not an RFC 5545 form #2
// (UTC) datetime, or when RANGE carries a value other than
// THISANDFUTURE.
//
// A TZID-tagged or VALUE=DATE RECURRENCE-ID returns a wrapped
// ErrUnsupportedRRule for the same reasons as EXDATE/RDATE (see
// SetFromComponent).
func ParseRecurrenceID(p vstar.Property) (RecurrenceID, error) {
	if !strings.EqualFold(p.Name, "RECURRENCE-ID") {
		return RecurrenceID{}, fmt.Errorf("rrule: property %q is not RECURRENCE-ID: %w", p.Name, vstar.ErrMalformed)
	}
	var out RecurrenceID
	for _, param := range p.Params {
		switch strings.ToUpper(param.Name) {
		case "RANGE":
			if !strings.EqualFold(param.Value, "THISANDFUTURE") {
				return RecurrenceID{}, fmt.Errorf("rrule: RECURRENCE-ID RANGE=%q invalid (RFC 5545 §3.2.13 defines THISANDFUTURE only): %w", param.Value, vstar.ErrMalformed)
			}
			out.Range = RangeThisAndFuture
		case "VALUE":
			if !strings.EqualFold(param.Value, "DATE-TIME") {
				return RecurrenceID{}, fmt.Errorf("rrule: RECURRENCE-ID VALUE=%s is outside the supported value types (DATE-TIME only): %w", param.Value, ErrUnsupportedRRule)
			}
		case "TZID":
			return RecurrenceID{}, fmt.Errorf("rrule: RECURRENCE-ID TZID=%s requires VTIMEZONE resolution unavailable at property scope: %w", param.Value, ErrUnsupportedRRule)
		}
	}
	t, ok := vstar.ParseTime(p.Value)
	if !ok {
		return RecurrenceID{}, fmt.Errorf("rrule: RECURRENCE-ID %q is not an RFC 5545 form #2 date-time: %w", p.Value, vstar.ErrMalformed)
	}
	out.Time = t
	return out, nil
}

// Property renders the RecurrenceID back to wire form. The RANGE
// parameter is emitted only for RangeThisAndFuture — the default
// is expressed by omission.
func (r RecurrenceID) Property() vstar.Property {
	p := vstar.Property{Name: "RECURRENCE-ID", Value: vstar.FormatTime(r.Time)}
	if s := r.Range.String(); s != "" {
		p.Params = []vstar.Param{{Name: "RANGE", Value: s}}
	}
	return p
}
