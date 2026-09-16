// SPDX-License-Identifier: MIT

package vstar

import (
	"testing"
	"time"
)

// TestDUEDate_ReadsValueDateProperty verifies a DUE property tagged
// VALUE=DATE is read back as a Date.
func TestDUEDate_ReadsValueDateProperty(t *testing.T) {
	c := Component{Type: CompTodo, Props: []Property{{
		Name:   "DUE",
		Params: []Param{{Name: "VALUE", Value: "DATE"}},
		Value:  "20260515",
	}}}
	got, ok := c.DUEDate()
	if !ok {
		t.Fatal("DUEDate() = _, false; want ok")
	}
	want := Date{Year: 2026, Month: time.May, Day: 15}
	if got != want {
		t.Errorf("DUEDate() = %+v, want %+v", got, want)
	}
}

// TestDUE_RejectsDateOnlyValue is the load-bearing distinction: a
// date-only DUE MUST NOT surface through the DATE-TIME accessor.
// Were it to return midnight UTC, callers could not tell an all-day
// task from one due at 00:00:00Z — two different assertions.
func TestDUE_RejectsDateOnlyValue(t *testing.T) {
	c := Component{Type: CompTodo, Props: []Property{{
		Name:   "DUE",
		Params: []Param{{Name: "VALUE", Value: "DATE"}},
		Value:  "20260515",
	}}}
	if got, ok := c.DUE(Calendar{}); ok {
		t.Errorf("DUE() = %v, true on a VALUE=DATE property; want zero, false", got)
	}
}

// TestDUEDate_RejectsDateTimeValue is the mirror image: a midnight
// DATE-TIME DUE MUST NOT surface through the DATE accessor.
func TestDUEDate_RejectsDateTimeValue(t *testing.T) {
	c := Component{Type: CompTodo, Props: []Property{{
		Name:  "DUE",
		Value: "20260515T000000Z",
	}}}
	if got, ok := c.DUEDate(); ok {
		t.Errorf("DUEDate() = %+v, true on a DATE-TIME property; want false", got)
	}
	// ...and the DATE-TIME accessor must still work on it.
	tm, ok := c.DUE(Calendar{})
	if !ok {
		t.Fatal("DUE() = _, false on a DATE-TIME property; want ok")
	}
	if !tm.Equal(time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("DUE() = %v, want midnight UTC", tm)
	}
}

// TestDateOnly_MidnightDistinguishable states the requirement end to
// end: the two encodings of "May 15" are distinguishable through the
// public API in both directions.
func TestDateOnly_MidnightDistinguishable(t *testing.T) {
	var allDay, atMidnight Component
	allDay.SetDUEDate(Date{Year: 2026, Month: time.May, Day: 15})
	atMidnight.SetDUE(time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC))

	if _, ok := allDay.DUE(Calendar{}); ok {
		t.Error("all-day DUE leaked through the DATE-TIME accessor")
	}
	if _, ok := allDay.DUEDate(); !ok {
		t.Error("all-day DUE not readable through the DATE accessor")
	}
	if _, ok := atMidnight.DUEDate(); ok {
		t.Error("midnight DATE-TIME DUE leaked through the DATE accessor")
	}
	if _, ok := atMidnight.DUE(Calendar{}); !ok {
		t.Error("midnight DATE-TIME DUE not readable through the DATE-TIME accessor")
	}

	// And the wire forms differ.
	ap, _ := allDay.Get("DUE")
	mp, _ := atMidnight.Get("DUE")
	if ap.Value == mp.Value {
		t.Errorf("wire values identical (%q) — encodings must differ", ap.Value)
	}
}

// TestSetDUEDate_EmitsValueDateParam verifies the VALUE=DATE
// parameter is emitted. It is REQUIRED: the default value type for
// DUE is DATE-TIME (RFC 5545 §3.8.2.3), so an untagged `20260515`
// is a malformed DATE-TIME, not a DATE.
func TestSetDUEDate_EmitsValueDateParam(t *testing.T) {
	var c Component
	c.SetDUEDate(Date{Year: 2026, Month: time.May, Day: 15})
	p, ok := c.Get("DUE")
	if !ok {
		t.Fatal("DUE property not set")
	}
	if p.Value != "20260515" {
		t.Errorf("DUE value = %q, want %q", p.Value, "20260515")
	}
	v, ok := paramValue(p, "VALUE")
	if !ok {
		t.Fatal("VALUE parameter missing; VALUE=DATE is required")
	}
	if v != "DATE" {
		t.Errorf("VALUE = %q, want %q", v, "DATE")
	}
	if len(p.Params) != 1 {
		t.Errorf("params = %+v, want exactly VALUE=DATE", p.Params)
	}
}

// TestSetDUEDate_DropsStaleParams verifies the date setter follows
// the same param-dropping discipline as SetDUE: a stale TZID from a
// prior local-time form must not survive. A DATE value must never
// carry TZID (RFC 5545 §3.2.19 — TZID applies to DATE-TIME/TIME).
func TestSetDUEDate_DropsStaleParams(t *testing.T) {
	c := Component{Props: []Property{{
		Name:   "DUE",
		Params: []Param{{Name: "TZID", Value: "America/Montreal"}, {Name: "X-FOO", Value: "bar"}},
		Value:  "20260515T133045",
	}}}
	c.SetDUEDate(Date{Year: 2026, Month: time.May, Day: 15})
	p, _ := c.Get("DUE")
	if _, ok := paramValue(p, "TZID"); ok {
		t.Error("stale TZID survived SetDUEDate")
	}
	if _, ok := paramValue(p, "X-FOO"); ok {
		t.Error("stale X-FOO survived SetDUEDate")
	}
	if len(p.Params) != 1 {
		t.Errorf("params = %+v, want exactly VALUE=DATE", p.Params)
	}
}

// TestSetDUE_DropsValueDateParam is the converse: switching a
// date-only DUE back to a DATE-TIME must strip VALUE=DATE, or the
// property would claim to be a DATE while carrying a DATE-TIME.
func TestSetDUE_DropsValueDateParam(t *testing.T) {
	var c Component
	c.SetDUEDate(Date{Year: 2026, Month: time.May, Day: 15})
	c.SetDUE(time.Date(2026, time.May, 15, 9, 0, 0, 0, time.UTC))
	p, _ := c.Get("DUE")
	if _, ok := paramValue(p, "VALUE"); ok {
		t.Errorf("VALUE=DATE survived SetDUE; params = %+v", p.Params)
	}
	if p.Value != "20260515T090000Z" {
		t.Errorf("DUE value = %q, want %q", p.Value, "20260515T090000Z")
	}
}

// TestSetDUEDate_ZeroRemoves verifies the zero Date clears the
// property, mirroring SetDUE's zero-time semantics.
func TestSetDUEDate_ZeroRemoves(t *testing.T) {
	var c Component
	c.SetDUEDate(Date{Year: 2026, Month: time.May, Day: 15})
	c.SetDUEDate(Date{})
	if _, ok := c.Get("DUE"); ok {
		t.Error("zero Date did not remove the DUE property")
	}
}

// TestDateAccessors_AllProperties exercises DTSTART/DTEND/DUE/
// COMPLETED symmetry through the date-typed accessors and setters.
func TestDateAccessors_AllProperties(t *testing.T) {
	d := Date{Year: 2026, Month: time.May, Day: 15}
	cases := []struct {
		name string
		set  func(*Component, Date)
		get  func(*Component) (Date, bool)
	}{
		{"DTSTART", (*Component).SetDTSTARTDate, (*Component).DTSTARTDate},
		{"DTEND", (*Component).SetDTENDDate, (*Component).DTENDDate},
		{"DUE", (*Component).SetDUEDate, (*Component).DUEDate},
		{"COMPLETED", (*Component).SetCOMPLETEDDate, (*Component).COMPLETEDDate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var c Component
			tc.set(&c, d)
			p, ok := c.Get(tc.name)
			if !ok {
				t.Fatalf("%s not set", tc.name)
			}
			if p.Value != "20260515" {
				t.Errorf("%s value = %q, want %q", tc.name, p.Value, "20260515")
			}
			if v, ok := paramValue(p, "VALUE"); !ok || v != "DATE" {
				t.Errorf("%s VALUE param = %q,%v; want DATE,true", tc.name, v, ok)
			}
			got, ok := tc.get(&c)
			if !ok || got != d {
				t.Errorf("%s read back = %+v,%v; want %+v,true", tc.name, got, ok, d)
			}
		})
	}
}

// TestDateProp_IgnoresUntaggedDateValue verifies an untagged
// `YYYYMMDD` value is NOT silently promoted to a Date. Without
// VALUE=DATE the property declares itself DATE-TIME, so the value is
// simply malformed — torn data, per the library's strictness stance.
func TestDateProp_IgnoresUntaggedDateValue(t *testing.T) {
	c := Component{Props: []Property{{Name: "DUE", Value: "20260515"}}}
	if got, ok := c.DUEDate(); ok {
		t.Errorf("DUEDate() = %+v, true on an untagged value; want false", got)
	}
	if got, ok := c.DUE(Calendar{}); ok {
		t.Errorf("DUE() = %v, true on a malformed DATE-TIME; want false", got)
	}
}

// TestDateProp_ValueDateCaseInsensitive verifies the parameter
// comparison is case-insensitive per RFC 5545 §3.2 (param names) and
// §3.2.20 (the VALUE enum is a registered token, case-insensitive).
func TestDateProp_ValueDateCaseInsensitive(t *testing.T) {
	c := Component{Props: []Property{{
		Name:   "DTSTART",
		Params: []Param{{Name: "value", Value: "date"}},
		Value:  "20260515",
	}}}
	if _, ok := c.DTSTARTDate(); !ok {
		t.Error("lower-case value=date not recognized")
	}
}

// TestIsDateOnly verifies the predicate callers use to branch before
// choosing an accessor.
func TestIsDateOnly(t *testing.T) {
	var allDay, atMidnight, absent Component
	allDay.SetDUEDate(Date{Year: 2026, Month: time.May, Day: 15})
	atMidnight.SetDUE(time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC))

	if !allDay.IsDateOnly("DUE") {
		t.Error("IsDateOnly = false for a VALUE=DATE property")
	}
	if atMidnight.IsDateOnly("DUE") {
		t.Error("IsDateOnly = true for a DATE-TIME property")
	}
	if absent.IsDateOnly("DUE") {
		t.Error("IsDateOnly = true for an absent property")
	}
}

// TestDUE_RejectsDateOnlyWithTZID pins the timeProp VALUE=DATE guard
// directly. A DATE value must never carry TZID (RFC 5545 §3.2.19),
// but a buggy producer can emit one. Without the guard, timeProp
// routes such a property to ParseTimeWithTZID on the strength of the
// TZID parameter alone; this test exists so the guard is load-bearing
// rather than incidentally redundant with ParseTime's length check.
func TestDUE_RejectsDateOnlyWithTZID(t *testing.T) {
	cal := Calendar{Components: []Component{{
		Type:  CompTimezone,
		Props: []Property{{Name: "TZID", Value: "Etc/Fixed"}},
		Sub: []Component{{
			Type: "STANDARD",
			Props: []Property{
				{Name: "DTSTART", Value: "19700101T000000"},
				{Name: "TZOFFSETFROM", Value: "-0500"},
				{Name: "TZOFFSETTO", Value: "-0500"},
			},
		}},
	}}}
	c := Component{Type: CompTodo, Props: []Property{{
		Name: "DUE",
		Params: []Param{
			{Name: "VALUE", Value: "DATE"},
			{Name: "TZID", Value: "Etc/Fixed"},
		},
		// Deliberately a DATE-TIME-shaped value under a VALUE=DATE
		// declaration: contradictory wire output. The declared value
		// type wins, so this is a DATE that fails to parse as one,
		// never a resolvable instant.
		Value: "20260515T133045",
	}}}
	if got, ok := c.DUE(cal); ok {
		t.Errorf("DUE() = %v, true on a VALUE=DATE property; want zero, false", got)
	}
	if got, ok := c.DUEDate(); ok {
		t.Errorf("DUEDate() = %+v, true on a DATE-TIME-shaped value; want false", got)
	}
}

// TestDTSTAMP_RejectsDateOnly pins the same guard on the UTC-only
// path. DTSTAMP is DATE-TIME by definition (RFC 5545 §3.8.7.2).
func TestDTSTAMP_RejectsDateOnly(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		// A well-formed DATE: rejected on the declared value type.
		{"date value", "20260515"},
		// Contradictory wire output — VALUE=DATE declared over a
		// DATE-TIME-shaped value. The declaration wins; without the
		// guard this parses as an instant.
		{"datetime value under VALUE=DATE", "20260515T133045Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Component{Props: []Property{{
				Name:   "DTSTAMP",
				Params: []Param{{Name: "VALUE", Value: "DATE"}},
				Value:  tc.value,
			}}}
			if got, ok := c.DTSTAMP(); ok {
				t.Errorf("DTSTAMP() = %v, true on a VALUE=DATE property; want false", got)
			}
		})
	}
}
