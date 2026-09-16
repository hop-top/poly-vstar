// SPDX-License-Identifier: MIT

package vstar

import "strings"

// ValueParam is the RFC 5545 §3.2.20 parameter name that declares a
// property's value type explicitly.
const ValueParam = "VALUE"

// ValueDate is the RFC 5545 §3.2.20 VALUE parameter value selecting
// the DATE value type (§3.3.4).
//
// The parameter is REQUIRED on any date-only DTSTART/DTEND/DUE/
// COMPLETED: the default value type for those properties is
// DATE-TIME (RFC 5545 §3.8.2.4, §3.8.2.2, §3.8.2.3, §3.8.2.1), so an
// untagged eight-octet value is a malformed DATE-TIME, not a DATE.
const ValueDate = "DATE"

// hasValueDate reports whether p carries VALUE=DATE.
//
// Both halves are compared case-insensitively: parameter names are
// case-insensitive per RFC 5545 §3.2, and the VALUE parameter's
// argument is a registered value-type token (§3.2.20), likewise
// case-insensitive.
func hasValueDate(p Property) bool {
	v, ok := paramValue(p, ValueParam)
	return ok && strings.EqualFold(v, ValueDate)
}

// IsDateOnly reports whether the named property is present and
// carries VALUE=DATE — i.e. whether its value is a calendar date
// rather than an instant.
//
// This is the branch point for callers that do not know the wire
// form up front:
//
//	if c.IsDateOnly("DUE") {
//	    d, _ := c.DUEDate()
//	    ...
//	} else {
//	    t, _ := c.DUE(cal)
//	    ...
//	}
//
// Returns false for an absent property.
func (c *Component) IsDateOnly(name string) bool {
	p, ok := c.Get(name)
	if !ok {
		return false
	}
	return hasValueDate(p)
}

// dateProp parses a date-bearing property's value. Returns
// (zero, false) for a missing property, a property that does not
// declare VALUE=DATE, or a malformed DATE value.
//
// The VALUE=DATE requirement is deliberate and not merely defensive.
// An untagged `20260515` declares itself DATE-TIME by default and is
// simply torn data; promoting it to a Date would be the silent
// coercion the parsers exist to prevent. A producer that means DATE
// must say so.
//
// All date-typed accessors below funnel through here so the rule is
// encoded once, mirroring how timeProp centralizes the DATE-TIME
// rules.
func (c *Component) dateProp(name string) (Date, bool) {
	p, ok := c.Get(name)
	if !ok {
		return Date{}, false
	}
	if !hasValueDate(p) {
		return Date{}, false
	}
	return ParseDate(p.Value)
}

// DTSTARTDate returns the DTSTART value as a calendar date when the
// property carries VALUE=DATE (an all-day event or task).
//
// Returns (zero, false) when DTSTART is absent, is a DATE-TIME, or
// carries a malformed DATE value. A midnight DATE-TIME does NOT
// surface here — see Date for why the two are not interchangeable.
func (c *Component) DTSTARTDate() (Date, bool) {
	return c.dateProp("DTSTART")
}

// DTENDDate returns the DTEND value as a calendar date; semantics
// match DTSTARTDate.
//
// Note RFC 5545 §3.6.1: for an all-day event DTEND is EXCLUSIVE —
// a one-day event on the 15th has DTEND;VALUE=DATE:20260516. This
// accessor reports the wire value as written and does not adjust it.
func (c *Component) DTENDDate() (Date, bool) {
	return c.dateProp("DTEND")
}

// DUEDate returns the VTODO DUE value as a calendar date — the
// all-day-task reader. Semantics match DTSTARTDate.
func (c *Component) DUEDate() (Date, bool) {
	return c.dateProp("DUE")
}

// COMPLETEDDate returns the VTODO COMPLETED value as a calendar
// date; semantics match DTSTARTDate.
//
// RFC 5545 §3.8.2.1 defines COMPLETED as DATE-TIME only, so a
// VALUE=DATE COMPLETED is non-conforming input. The accessor is
// provided for symmetry with the other three and to let readers
// recover such a value rather than lose it; producers should prefer
// SetCOMPLETED.
func (c *Component) COMPLETEDDate() (Date, bool) {
	return c.dateProp("COMPLETED")
}

// setOrClearDate writes a date-only value for the named property or
// removes the property entirely when d is the zero Date.
//
// The written property carries exactly one parameter, VALUE=DATE,
// and nothing else. This follows setOrClearTime's discipline of
// dropping pre-existing parameters — here it is not merely tidy but
// required: a stale TZID from a prior local-time form would be
// meaningless on a DATE (RFC 5545 §3.2.19 scopes TZID to DATE-TIME
// and TIME values), and a stale parameter set would make the
// canonical bytes depend on the property's edit history.
func (c *Component) setOrClearDate(name string, d Date) {
	if d.IsZero() {
		c.Remove(name)
		return
	}
	c.Set(Property{
		Name:   name,
		Params: []Param{{Name: ValueParam, Value: ValueDate}},
		Value:  FormatDate(d),
	})
}

// SetDTSTARTDate writes an all-day DTSTART: the value in RFC 5545
// §3.3.4 DATE form plus the required VALUE=DATE parameter. Passing
// the zero Date removes the property.
//
// Any pre-existing parameters on DTSTART are dropped, including a
// stale TZID — see setOrClearDate.
func (c *Component) SetDTSTARTDate(d Date) {
	c.setOrClearDate("DTSTART", d)
}

// SetDTENDDate writes an all-day DTEND; semantics match
// SetDTSTARTDate.
//
// Per RFC 5545 §3.6.1 the all-day DTEND is EXCLUSIVE: to express a
// one-day event on the 15th, pass the 16th. This setter writes what
// it is given and does not adjust.
func (c *Component) SetDTENDDate(d Date) {
	c.setOrClearDate("DTEND", d)
}

// SetDUEDate writes an all-day DUE — the all-day-task writer;
// semantics match SetDTSTARTDate.
func (c *Component) SetDUEDate(d Date) {
	c.setOrClearDate("DUE", d)
}

// SetCOMPLETEDDate writes a date-only COMPLETED; semantics match
// SetDTSTARTDate.
//
// RFC 5545 §3.8.2.1 mandates DATE-TIME for COMPLETED, so this emits
// non-conforming output. It exists for symmetry; prefer
// SetCOMPLETED.
func (c *Component) SetCOMPLETEDDate(d Date) {
	c.setOrClearDate("COMPLETED", d)
}
