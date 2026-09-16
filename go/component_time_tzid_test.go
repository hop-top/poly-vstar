// SPDX-License-Identifier: MIT

package vstar_test

import (
	"testing"
	"time"

	vstar "hop.top/vstar"
)

// The TZID-resolving half of the Component date/time accessors. It
// lives here rather than in component_time_test.go because its input
// is the parsed america_montreal.ics registry; see
// time_fixture_test.go for why that needs the external test package.

// TestComponent_DTSTART_TZID verifies a DTSTART with a TZID
// parameter is resolved against the calendar's VTIMEZONE registry.
func TestComponent_DTSTART_TZID(t *testing.T) {
	cal := americaMontrealCalendar(t)
	c := vstar.Component{Type: vstar.CompEvent, Props: []vstar.Property{
		{
			Name:   "DTSTART",
			Params: []vstar.Param{{Name: "TZID", Value: "America/Montreal"}},
			Value:  "20260104T133045",
		},
	}}
	got, ok := c.DTSTART(cal)
	if !ok {
		t.Fatalf("DTSTART: ok=false")
	}
	want := time.Date(2026, 1, 4, 18, 30, 45, 0, time.UTC)
	if !got.UTC().Equal(want) {
		t.Errorf("DTSTART (TZID) = %v (UTC %v), want %v", got, got.UTC(), want)
	}
}

// TestComponent_DUE_TZID verifies the DUE accessor resolves a
// TZID-bearing value against the calendar.
func TestComponent_DUE_TZID(t *testing.T) {
	cal := americaMontrealCalendar(t)
	c := vstar.Component{Type: vstar.CompTodo, Props: []vstar.Property{
		{
			Name:   "DUE",
			Params: []vstar.Param{{Name: "TZID", Value: "America/Montreal"}},
			Value:  "20260704T090000",
		},
	}}
	got, ok := c.DUE(cal)
	if !ok {
		t.Fatalf("DUE: ok=false")
	}
	want := time.Date(2026, 7, 4, 13, 0, 0, 0, time.UTC) // EDT = -0400
	if !got.UTC().Equal(want) {
		t.Errorf("DUE = %v (UTC %v), want %v", got, got.UTC(), want)
	}
}
