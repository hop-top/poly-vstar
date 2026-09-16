// SPDX-License-Identifier: MIT

package vstar

import (
	"testing"
	"time"
)

// TestComponent_DTSTART_PlainUTC verifies a DTSTART carrying a
// form #2 UTC value parses without consulting the calendar.
func TestComponent_DTSTART_PlainUTC(t *testing.T) {
	c := Component{Type: CompEvent, Props: []Property{
		{Name: "DTSTART", Value: "20260504T183045Z"},
	}}
	got, ok := c.DTSTART(Calendar{})
	if !ok {
		t.Fatalf("DTSTART: ok=false")
	}
	want := time.Date(2026, 5, 4, 18, 30, 45, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("DTSTART = %v, want %v", got, want)
	}
}

// TestComponent_DTSTART_Missing verifies absent DTSTART yields
// (zero, false).
func TestComponent_DTSTART_Missing(t *testing.T) {
	c := Component{Type: CompEvent}
	got, ok := c.DTSTART(Calendar{})
	if ok {
		t.Errorf("DTSTART = (%v, true), want false", got)
	}
}

// TestComponent_DTSTART_Malformed verifies a malformed value
// yields (zero, false) — non-inferring.
func TestComponent_DTSTART_Malformed(t *testing.T) {
	c := Component{Type: CompEvent, Props: []Property{
		{Name: "DTSTART", Value: "not-a-date"},
	}}
	got, ok := c.DTSTART(Calendar{})
	if ok {
		t.Errorf("DTSTART = (%v, true), want false", got)
	}
}

// TestComponent_DTSTART_TZIDMissingZone verifies a DTSTART
// referencing an unknown TZID fails closed.
func TestComponent_DTSTART_TZIDMissingZone(t *testing.T) {
	c := Component{Type: CompEvent, Props: []Property{
		{
			Name:   "DTSTART",
			Params: []Param{{Name: "TZID", Value: "America/Mars"}},
			Value:  "20260104T133045",
		},
	}}
	got, ok := c.DTSTART(Calendar{})
	if ok {
		t.Errorf("DTSTART (unknown TZID) = (%v, true), want false", got)
	}
}

// TestComponent_DTSTAMP verifies the time-returning DTSTAMP accessor
// (the ergonomic primary; raw wire string available via DTSTAMPRaw).
func TestComponent_DTSTAMP(t *testing.T) {
	c := Component{Type: CompEvent, Props: []Property{
		{Name: "DTSTAMP", Value: "20260504T180000Z"},
	}}
	got, ok := c.DTSTAMP()
	if !ok {
		t.Fatalf("DTSTAMP: ok=false")
	}
	want := time.Date(2026, 5, 4, 18, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("DTSTAMP = %v, want %v", got, want)
	}
}

// TestComponent_DTSTAMP_RejectsTZID verifies DTSTAMP with a TZID
// parameter fails — RFC 5545 §3.8.7.2 mandates DTSTAMP MUST be UTC.
func TestComponent_DTSTAMP_RejectsTZID(t *testing.T) {
	c := Component{Type: CompEvent, Props: []Property{
		{
			Name:   "DTSTAMP",
			Params: []Param{{Name: "TZID", Value: "America/Montreal"}},
			Value:  "20260504T140000",
		},
	}}
	got, ok := c.DTSTAMP()
	if ok {
		t.Errorf("DTSTAMP (TZID) = (%v, true), want false", got)
	}
}

// TestComponent_DTSTAMP_Missing verifies absent DTSTAMP yields
// (zero, false).
func TestComponent_DTSTAMP_Missing(t *testing.T) {
	c := Component{Type: CompEvent}
	got, ok := c.DTSTAMP()
	if ok {
		t.Errorf("DTSTAMP = (%v, true), want false", got)
	}
}

// TestComponent_DTEND_PlainUTC verifies the DTEND accessor on a
// VEVENT with a form #2 value.
func TestComponent_DTEND_PlainUTC(t *testing.T) {
	c := Component{Type: CompEvent, Props: []Property{
		{Name: "DTEND", Value: "20260504T200000Z"},
	}}
	got, ok := c.DTEND(Calendar{})
	if !ok {
		t.Fatalf("DTEND: ok=false")
	}
	want := time.Date(2026, 5, 4, 20, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("DTEND = %v, want %v", got, want)
	}
}

// TestComponent_DUE_Missing verifies absent DUE yields (zero,
// false).
func TestComponent_DUE_Missing(t *testing.T) {
	c := Component{Type: CompTodo}
	got, ok := c.DUE(Calendar{})
	if ok {
		t.Errorf("DUE = (%v, true), want false", got)
	}
}

// TestComponent_COMPLETED_PlainUTC verifies the COMPLETED accessor.
// RFC 5545 §3.8.2.1 — COMPLETED MUST be UTC. We still allow the
// caller to pass a calendar (uniform signature) but a TZID-bearing
// value will fail closed since the value won't match form #2.
func TestComponent_COMPLETED_PlainUTC(t *testing.T) {
	c := Component{Type: CompTodo, Props: []Property{
		{Name: "COMPLETED", Value: "20260504T193000Z"},
	}}
	got, ok := c.COMPLETED(Calendar{})
	if !ok {
		t.Fatalf("COMPLETED: ok=false")
	}
	want := time.Date(2026, 5, 4, 19, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("COMPLETED = %v, want %v", got, want)
	}
}

// TestComponent_CaseInsensitiveLookup verifies property lookup
// honors the existing Component.Get case-insensitivity.
func TestComponent_CaseInsensitiveLookup(t *testing.T) {
	c := Component{Type: CompEvent, Props: []Property{
		{Name: "dtstart", Value: "20260504T183045Z"},
	}}
	got, ok := c.DTSTART(Calendar{})
	if !ok {
		t.Fatalf("DTSTART (lowercase): ok=false")
	}
	want := time.Date(2026, 5, 4, 18, 30, 45, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("DTSTART = %v, want %v", got, want)
	}
}

// TestComponent_SetDTSTART_RoundTrip verifies SetDTSTART writes a
// form #2 UTC value that DTSTART reads back as the same instant.
func TestComponent_SetDTSTART_RoundTrip(t *testing.T) {
	c := Component{Type: CompEvent}
	in := time.Date(2026, 5, 4, 18, 30, 45, 0, time.UTC)
	c.SetDTSTART(in)
	got, ok := c.DTSTART(Calendar{})
	if !ok {
		t.Fatalf("DTSTART: ok=false after SetDTSTART")
	}
	if !got.Equal(in) {
		t.Errorf("round-trip DTSTART = %v, want %v", got, in)
	}
	// Confirm the wire form is form #2 UTC.
	p, ok := c.Get("DTSTART")
	if !ok {
		t.Fatalf("DTSTART property missing after Set")
	}
	if p.Value != "20260504T183045Z" {
		t.Errorf("DTSTART wire = %q, want %q", p.Value, "20260504T183045Z")
	}
	if len(p.Params) != 0 {
		t.Errorf("DTSTART params = %+v, want none (UTC must not carry TZID)", p.Params)
	}
}

// TestComponent_SetDTSTART_NonUTCConverted verifies a non-UTC input
// is converted before write — the wire is always UTC.
func TestComponent_SetDTSTART_NonUTCConverted(t *testing.T) {
	c := Component{Type: CompEvent}
	mtl := time.FixedZone("EST", -5*3600)
	in := time.Date(2026, 5, 4, 13, 30, 45, 0, mtl)
	c.SetDTSTART(in)
	p, _ := c.Get("DTSTART")
	if p.Value != "20260504T183045Z" {
		t.Errorf("DTSTART wire = %q, want %q", p.Value, "20260504T183045Z")
	}
}

// TestComponent_SetDTSTART_ZeroClears verifies passing the zero
// time removes the property.
func TestComponent_SetDTSTART_ZeroClears(t *testing.T) {
	c := Component{Type: CompEvent, Props: []Property{
		{Name: "DTSTART", Value: "20260504T183045Z"},
	}}
	c.SetDTSTART(time.Time{})
	if _, ok := c.Get("DTSTART"); ok {
		t.Errorf("DTSTART still present after Set(zero)")
	}
}

// TestComponent_SetDTSTART_ReplacesExisting verifies a second Set
// replaces (not duplicates) the prior property.
func TestComponent_SetDTSTART_ReplacesExisting(t *testing.T) {
	c := Component{Type: CompEvent}
	c.SetDTSTART(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	c.SetDTSTART(time.Date(2026, 5, 4, 18, 30, 45, 0, time.UTC))
	all := c.GetAll("DTSTART")
	if len(all) != 1 {
		t.Errorf("got %d DTSTART props, want 1: %+v", len(all), all)
	}
	if all[0].Value != "20260504T183045Z" {
		t.Errorf("DTSTART wire = %q, want %q", all[0].Value, "20260504T183045Z")
	}
}

// TestComponent_SetDTSTART_StripsTZIDParam verifies a Set on a
// property that previously had a TZID param produces a clean UTC
// property (no stale params).
func TestComponent_SetDTSTART_StripsTZIDParam(t *testing.T) {
	c := Component{Type: CompEvent, Props: []Property{
		{
			Name:   "DTSTART",
			Params: []Param{{Name: "TZID", Value: "America/Montreal"}},
			Value:  "20260104T133045",
		},
	}}
	c.SetDTSTART(time.Date(2026, 5, 4, 18, 30, 45, 0, time.UTC))
	p, _ := c.Get("DTSTART")
	if len(p.Params) != 0 {
		t.Errorf("after Set: DTSTART still has params %+v, want none", p.Params)
	}
	if p.Value != "20260504T183045Z" {
		t.Errorf("DTSTART wire = %q, want %q", p.Value, "20260504T183045Z")
	}
}

// TestComponent_SetDTEND_RoundTrip mirrors SetDTSTART for DTEND.
func TestComponent_SetDTEND_RoundTrip(t *testing.T) {
	c := Component{Type: CompEvent}
	in := time.Date(2026, 5, 4, 20, 0, 0, 0, time.UTC)
	c.SetDTEND(in)
	got, ok := c.DTEND(Calendar{})
	if !ok {
		t.Fatalf("DTEND: ok=false")
	}
	if !got.Equal(in) {
		t.Errorf("round-trip DTEND = %v, want %v", got, in)
	}
}

// TestComponent_SetDUE_RoundTrip mirrors SetDTSTART for DUE.
func TestComponent_SetDUE_RoundTrip(t *testing.T) {
	c := Component{Type: CompTodo}
	in := time.Date(2026, 5, 5, 9, 0, 0, 0, time.UTC)
	c.SetDUE(in)
	got, ok := c.DUE(Calendar{})
	if !ok {
		t.Fatalf("DUE: ok=false")
	}
	if !got.Equal(in) {
		t.Errorf("round-trip DUE = %v, want %v", got, in)
	}
}

// TestComponent_SetCOMPLETED_RoundTrip mirrors SetDTSTART for
// COMPLETED.
func TestComponent_SetCOMPLETED_RoundTrip(t *testing.T) {
	c := Component{Type: CompTodo}
	in := time.Date(2026, 5, 4, 19, 30, 0, 0, time.UTC)
	c.SetCOMPLETED(in)
	got, ok := c.COMPLETED(Calendar{})
	if !ok {
		t.Fatalf("COMPLETED: ok=false")
	}
	if !got.Equal(in) {
		t.Errorf("round-trip COMPLETED = %v, want %v", got, in)
	}
}

// TestComponent_SetDUE_ZeroClears verifies the writer's clear
// behavior for DUE specifically.
func TestComponent_SetDUE_ZeroClears(t *testing.T) {
	c := Component{Type: CompTodo, Props: []Property{
		{Name: "DUE", Value: "20260505T090000Z"},
	}}
	c.SetDUE(time.Time{})
	if _, ok := c.Get("DUE"); ok {
		t.Errorf("DUE still present after Set(zero)")
	}
}
