// SPDX-License-Identifier: MIT

package vstar

import (
	"strings"
	"time"
)

// timeProp parses a time-bearing property's value, consulting cal
// when the property carries a TZID parameter. Returns (zero, false)
// for a missing property, malformed value, or unresolvable TZID.
//
// A property declaring VALUE=DATE is refused outright: it holds a
// calendar date, not an instant, and the two are not
// interchangeable (see Date). Returning its midnight would make an
// all-day value indistinguishable from one due at 00:00:00Z. Read
// such properties through the date-typed accessors — DTSTARTDate,
// DTENDDate, DUEDate, COMPLETEDDate — or branch on IsDateOnly.
//
// All accessors below funnel through here so the strict / non-
// inferring semantics are encoded once.
func (c *Component) timeProp(name string, cal Calendar) (time.Time, bool) {
	p, ok := c.Get(name)
	if !ok {
		return time.Time{}, false
	}
	if hasValueDate(p) {
		return time.Time{}, false
	}
	if tzid, hasTZ := paramValue(p, "TZID"); hasTZ {
		return ParseTimeWithTZID(p.Value, tzid, cal)
	}
	return ParseTime(p.Value)
}

// timePropUTC parses a time-bearing property that MUST carry a UTC
// value (no TZID parameter permitted). Used for DTSTAMP and other
// RFC-mandated UTC-only properties. Returns (zero, false) when a
// TZID is present, or when the property declares VALUE=DATE (these
// properties are DATE-TIME by definition; a DATE value is a
// producer bug).
func (c *Component) timePropUTC(name string) (time.Time, bool) {
	p, ok := c.Get(name)
	if !ok {
		return time.Time{}, false
	}
	if hasValueDate(p) {
		return time.Time{}, false
	}
	if _, hasTZ := paramValue(p, "TZID"); hasTZ {
		return time.Time{}, false
	}
	return ParseTime(p.Value)
}

// paramValue returns the value of the first param matching name
// (case-insensitive per RFC 5545 §3.2). Returns ("", false) when
// absent.
func paramValue(p Property, name string) (string, bool) {
	for _, par := range p.Params {
		if strings.EqualFold(par.Name, name) {
			return par.Value, true
		}
	}
	return "", false
}

// DTSTART returns the parsed DTSTART instant. When the property
// carries a TZID parameter, the zone is resolved via cal's
// VTIMEZONE registry. Plain UTC values (form #2) parse without
// touching cal.
//
// Returns (zero, false) on missing/malformed/unresolvable input.
// See ParseTime / ParseTimeWithTZID for strictness rules.
func (c *Component) DTSTART(cal Calendar) (time.Time, bool) {
	return c.timeProp("DTSTART", cal)
}

// DTEND returns the parsed DTEND instant; semantics match DTSTART.
func (c *Component) DTEND(cal Calendar) (time.Time, bool) {
	return c.timeProp("DTEND", cal)
}

// DUE returns the parsed VTODO DUE instant; semantics match
// DTSTART.
func (c *Component) DUE(cal Calendar) (time.Time, bool) {
	return c.timeProp("DUE", cal)
}

// COMPLETED returns the parsed VTODO COMPLETED instant. Per
// RFC 5545 §3.8.2.1 this property MUST be UTC; we accept the cal
// argument for signature uniformity but a TZID-bearing value will
// fail since ParseTime strict-rejects form #1.
func (c *Component) COMPLETED(cal Calendar) (time.Time, bool) {
	return c.timeProp("COMPLETED", cal)
}

// DTSTAMP returns the parsed DTSTAMP instant. Per RFC 5545 §3.8.7.2
// DTSTAMP MUST be UTC; a TZID parameter on DTSTAMP is a producer bug
// and is rejected here.
//
// This is the ergonomic primary accessor (matches DTSTART, DUE,
// DTEND, COMPLETED). For the raw wire string used by codec-level
// callers, use DTSTAMPRaw.
func (c *Component) DTSTAMP() (time.Time, bool) {
	return c.timePropUTC("DTSTAMP")
}

// setOrClearTime writes a UTC form #2 value for the named property
// or removes the property entirely when t is the zero time.
//
// On write, any pre-existing parameters on a same-named property
// are dropped: the V* on-the-wire DATE-TIME form is always plain
// UTC, no params. This covers both a stale TZID from a prior
// local-time form and a stale VALUE=DATE from a prior all-day
// value — leaving the latter in place would produce a property
// claiming DATE while carrying a DATE-TIME.
func (c *Component) setOrClearTime(name string, t time.Time) {
	if t.IsZero() {
		c.Remove(name)
		return
	}
	c.Set(Property{Name: name, Value: FormatTime(t)})
}

// SetDTSTART writes a UTC form #2 DTSTART. Any non-UTC input is
// converted to UTC by FormatTime. Passing the zero time removes
// the property. Existing parameters — TZID, VALUE=DATE — are
// dropped; V* writes only the UTC form. For an all-day DTSTART use
// SetDTSTARTDate.
func (c *Component) SetDTSTART(t time.Time) {
	c.setOrClearTime("DTSTART", t)
}

// SetDTEND writes a UTC form #2 DTEND; semantics match SetDTSTART.
func (c *Component) SetDTEND(t time.Time) {
	c.setOrClearTime("DTEND", t)
}

// SetDUE writes a UTC form #2 DUE; semantics match SetDTSTART.
func (c *Component) SetDUE(t time.Time) {
	c.setOrClearTime("DUE", t)
}

// SetCOMPLETED writes a UTC form #2 COMPLETED; semantics match
// SetDTSTART. RFC 5545 §3.8.2.1 mandates UTC for this property.
func (c *Component) SetCOMPLETED(t time.Time) {
	c.setOrClearTime("COMPLETED", t)
}
