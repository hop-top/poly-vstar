// SPDX-License-Identifier: MIT

package vstar_test

import (
	"testing"
	"time"

	vstar "hop.top/vstar"
)

// TestParseTimeWithTZID_MontrealStandard verifies that a winter
// (EST) instant in America/Montreal is interpreted with the -0500
// offset — so 13:30:45 local → 18:30:45 UTC.
func TestParseTimeWithTZID_MontrealStandard(t *testing.T) {
	cal := americaMontrealCalendar(t)
	got, ok := vstar.ParseTimeWithTZID("20260104T133045", "America/Montreal", cal)
	if !ok {
		t.Fatalf("ParseTimeWithTZID: ok=false, want true")
	}
	want := time.Date(2026, 1, 4, 18, 30, 45, 0, time.UTC)
	if !got.UTC().Equal(want) {
		t.Errorf("ParseTimeWithTZID winter = %v (UTC %v), want %v",
			got, got.UTC(), want)
	}
}

// TestParseTimeWithTZID_MontrealDaylight verifies that a summer
// (EDT) instant in America/Montreal is interpreted with -0400 —
// so 13:30:45 local → 17:30:45 UTC.
func TestParseTimeWithTZID_MontrealDaylight(t *testing.T) {
	cal := americaMontrealCalendar(t)
	got, ok := vstar.ParseTimeWithTZID("20260704T133045", "America/Montreal", cal)
	if !ok {
		t.Fatalf("ParseTimeWithTZID: ok=false, want true")
	}
	want := time.Date(2026, 7, 4, 17, 30, 45, 0, time.UTC)
	if !got.UTC().Equal(want) {
		t.Errorf("ParseTimeWithTZID summer = %v (UTC %v), want %v",
			got, got.UTC(), want)
	}
}

// TestParseTimeWithTZID_StandardOnly verifies the trivial single-
// STANDARD path (no DST) round-trips with a fixed offset.
func TestParseTimeWithTZID_StandardOnly(t *testing.T) {
	cal := utcOnlyCalendar(t)
	got, ok := vstar.ParseTimeWithTZID("20260504T183045", "UTC", cal)
	if !ok {
		t.Fatalf("ParseTimeWithTZID: ok=false, want true")
	}
	want := time.Date(2026, 5, 4, 18, 30, 45, 0, time.UTC)
	if !got.UTC().Equal(want) {
		t.Errorf("ParseTimeWithTZID = %v (UTC %v), want %v", got, got.UTC(), want)
	}
}

// TestParseTimeWithTZID_MissingVTIMEZONE verifies that an unknown
// TZID fails closed.
func TestParseTimeWithTZID_MissingVTIMEZONE(t *testing.T) {
	cal := americaMontrealCalendar(t)
	got, ok := vstar.ParseTimeWithTZID("20260104T133045", "America/Toronto", cal)
	if ok {
		t.Errorf("ParseTimeWithTZID(missing) = (%v, true), want (zero, false)", got)
	}
}

// TestParseTimeWithTZID_EmptyTZID verifies an empty tzid fails.
func TestParseTimeWithTZID_EmptyTZID(t *testing.T) {
	cal := americaMontrealCalendar(t)
	got, ok := vstar.ParseTimeWithTZID("20260104T133045", "", cal)
	if ok {
		t.Errorf("ParseTimeWithTZID(empty) = (%v, true), want (zero, false)", got)
	}
}

// TestParseTimeWithTZID_MalformedValue verifies a value that isn't
// form #1 fails (e.g. has a Z suffix — that's form #2 and belongs
// to vstar.ParseTime).
func TestParseTimeWithTZID_MalformedValue(t *testing.T) {
	cal := americaMontrealCalendar(t)
	cases := []string{
		"20260104T133045Z",
		"2026-01-04T13:30:45",
		"",
		"20260104",
	}
	for _, in := range cases {
		got, ok := vstar.ParseTimeWithTZID(in, "America/Montreal", cal)
		if ok {
			t.Errorf("ParseTimeWithTZID(%q) = (%v, true), want false", in, got)
		}
	}
}

// TestParseTimeWithTZID_MalformedVTIMEZONE verifies a VTIMEZONE
// with no STANDARD/DAYLIGHT children fails.
func TestParseTimeWithTZID_MalformedVTIMEZONE(t *testing.T) {
	cal := vstar.Calendar{
		Components: []vstar.Component{{
			Type:  vstar.CompTimezone,
			Props: []vstar.Property{{Name: "TZID", Value: "BrokenZone"}},
			// No Sub — no STANDARD/DAYLIGHT.
		}},
	}
	got, ok := vstar.ParseTimeWithTZID("20260104T133045", "BrokenZone", cal)
	if ok {
		t.Errorf("ParseTimeWithTZID(no rules) = (%v, true), want false", got)
	}
}

// TestParseTimeWithTZID_BadOffsetRejected verifies a malformed
// TZOFFSETTO/FROM (not ±HHMM) causes the lookup to fail.
func TestParseTimeWithTZID_BadOffsetRejected(t *testing.T) {
	cal := vstar.Calendar{
		Components: []vstar.Component{{
			Type:  vstar.CompTimezone,
			Props: []vstar.Property{{Name: "TZID", Value: "Bad"}},
			Sub: []vstar.Component{{
				Type: vstar.CompType("STANDARD"),
				Props: []vstar.Property{
					{Name: "DTSTART", Value: "19700101T000000"},
					{Name: "TZOFFSETFROM", Value: "garbage"},
					{Name: "TZOFFSETTO", Value: "garbage"},
				},
			}},
		}},
	}
	got, ok := vstar.ParseTimeWithTZID("20260104T133045", "Bad", cal)
	if ok {
		t.Errorf("ParseTimeWithTZID(bad offset) = (%v, true), want false", got)
	}
}

// TestParseTimeWithTZID_LastSundayBYDAY verifies the negative-
// ordinal BYDAY form (e.g. -1SU = "last Sunday of month") is
// honored. EU DST historically used "last Sunday of October" /
// "last Sunday of March".
func TestParseTimeWithTZID_LastSundayBYDAY(t *testing.T) {
	cal := vstar.Calendar{Components: []vstar.Component{{
		Type:  vstar.CompTimezone,
		Props: []vstar.Property{{Name: "TZID", Value: "Europe/Berlin"}},
		Sub: []vstar.Component{
			{
				Type: vstar.CompType("DAYLIGHT"),
				Props: []vstar.Property{
					{Name: "DTSTART", Value: "19960331T020000"},
					{Name: "TZOFFSETFROM", Value: "+0100"},
					{Name: "TZOFFSETTO", Value: "+0200"},
					{Name: "TZNAME", Value: "CEST"},
					{Name: "RRULE", Value: "FREQ=YEARLY;BYMONTH=3;BYDAY=-1SU"},
				},
			},
			{
				Type: vstar.CompType("STANDARD"),
				Props: []vstar.Property{
					{Name: "DTSTART", Value: "19961027T030000"},
					{Name: "TZOFFSETFROM", Value: "+0200"},
					{Name: "TZOFFSETTO", Value: "+0100"},
					{Name: "TZNAME", Value: "CET"},
					{Name: "RRULE", Value: "FREQ=YEARLY;BYMONTH=10;BYDAY=-1SU"},
				},
			},
		},
	}}}
	// July 4 2026 — DST active (CEST = +0200).
	got, ok := vstar.ParseTimeWithTZID("20260704T140000", "Europe/Berlin", cal)
	if !ok {
		t.Fatalf("ParseTimeWithTZID: ok=false")
	}
	want := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	if !got.UTC().Equal(want) {
		t.Errorf("ParseTimeWithTZID = %v (UTC %v), want %v", got, got.UTC(), want)
	}
	// January 4 2026 — STANDARD (CET = +0100).
	got2, ok := vstar.ParseTimeWithTZID("20260104T140000", "Europe/Berlin", cal)
	if !ok {
		t.Fatalf("ParseTimeWithTZID winter: ok=false")
	}
	want2 := time.Date(2026, 1, 4, 13, 0, 0, 0, time.UTC)
	if !got2.UTC().Equal(want2) {
		t.Errorf("ParseTimeWithTZID winter = %v (UTC %v), want %v", got2, got2.UTC(), want2)
	}
}

// TestParseTimeWithTZID_AllWeekdayCodes verifies every two-letter
// weekday code parses. Uses minimal STANDARD-only zones with the
// rule's BYDAY field exercised purely by the parser.
func TestParseTimeWithTZID_AllWeekdayCodes(t *testing.T) {
	codes := []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}
	for _, code := range codes {
		cal := vstar.Calendar{Components: []vstar.Component{{
			Type:  vstar.CompTimezone,
			Props: []vstar.Property{{Name: "TZID", Value: "X"}},
			Sub: []vstar.Component{
				{
					Type: vstar.CompType("STANDARD"),
					Props: []vstar.Property{
						{Name: "DTSTART", Value: "19700101T000000"},
						{Name: "TZOFFSETFROM", Value: "+0000"},
						{Name: "TZOFFSETTO", Value: "+0000"},
						{Name: "RRULE", Value: "FREQ=YEARLY;BYMONTH=1;BYDAY=1" + code},
					},
				},
				{
					Type: vstar.CompType("DAYLIGHT"),
					Props: []vstar.Property{
						{Name: "DTSTART", Value: "19700601T000000"},
						{Name: "TZOFFSETFROM", Value: "+0000"},
						{Name: "TZOFFSETTO", Value: "+0100"},
						{Name: "RRULE", Value: "FREQ=YEARLY;BYMONTH=6;BYDAY=1" + code},
					},
				},
			},
		}}}
		_, ok := vstar.ParseTimeWithTZID("20260315T120000", "X", cal)
		if !ok {
			t.Errorf("BYDAY=1%s: ok=false, want true", code)
		}
	}
	// Bogus weekday code rejected.
	cal := vstar.Calendar{Components: []vstar.Component{{
		Type:  vstar.CompTimezone,
		Props: []vstar.Property{{Name: "TZID", Value: "Y"}},
		Sub: []vstar.Component{{
			Type: vstar.CompType("STANDARD"),
			Props: []vstar.Property{
				{Name: "DTSTART", Value: "19700101T000000"},
				{Name: "TZOFFSETFROM", Value: "+0000"},
				{Name: "TZOFFSETTO", Value: "+0000"},
				{Name: "RRULE", Value: "FREQ=YEARLY;BYMONTH=1;BYDAY=1XX"},
			},
		}},
	}}}
	if _, ok := vstar.ParseTimeWithTZID("20260315T120000", "Y", cal); ok {
		t.Errorf("bogus weekday code: ok=true, want false")
	}
}

// TestParseTimeWithTZID_UnsupportedRRULE verifies any RRULE that
// isn't FREQ=YEARLY is rejected — we don't silently apply rules we
// don't understand.
func TestParseTimeWithTZID_UnsupportedRRULE(t *testing.T) {
	cal := vstar.Calendar{
		Components: []vstar.Component{{
			Type:  vstar.CompTimezone,
			Props: []vstar.Property{{Name: "TZID", Value: "Weird"}},
			Sub: []vstar.Component{{
				Type: vstar.CompType("STANDARD"),
				Props: []vstar.Property{
					{Name: "DTSTART", Value: "20070101T000000"},
					{Name: "TZOFFSETFROM", Value: "-0500"},
					{Name: "TZOFFSETTO", Value: "-0500"},
					{Name: "RRULE", Value: "FREQ=MONTHLY;BYDAY=1SU"},
				},
			}},
		}},
	}
	got, ok := vstar.ParseTimeWithTZID("20260104T133045", "Weird", cal)
	if ok {
		t.Errorf("ParseTimeWithTZID(non-yearly rrule) = (%v, true), want false", got)
	}
}
