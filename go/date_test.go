// SPDX-License-Identifier: MIT

package vstar

import (
	"testing"
	"time"
)

// TestFormatDate_KnownDate verifies a date renders as RFC 5545
// §3.3.4 DATE form — `YYYYMMDD`, no T, no Z, no zone.
func TestFormatDate_KnownDate(t *testing.T) {
	got := FormatDate(Date{Year: 2026, Month: time.May, Day: 15})
	want := "20260515"
	if got != want {
		t.Errorf("FormatDate = %q, want %q", got, want)
	}
}

// TestFormatDate_ZeroPadded verifies single-digit months and days
// are zero-padded to the fixed 8-octet DATE width.
func TestFormatDate_ZeroPadded(t *testing.T) {
	got := FormatDate(Date{Year: 2026, Month: time.January, Day: 2})
	want := "20260102"
	if got != want {
		t.Errorf("FormatDate = %q, want %q", got, want)
	}
}

// TestFormatDate_ZeroEmpty verifies the zero Date renders as the
// empty string, mirroring FormatTime's "clear the property" signal.
func TestFormatDate_ZeroEmpty(t *testing.T) {
	if got := FormatDate(Date{}); got != "" {
		t.Errorf("FormatDate(zero) = %q, want %q", got, "")
	}
}

// TestParseDate_RoundTrip verifies ParseDate ∘ FormatDate is the
// identity over valid dates.
func TestParseDate_RoundTrip(t *testing.T) {
	for _, wire := range []string{"20260515", "20000229", "19991231", "20260102"} {
		d, ok := ParseDate(wire)
		if !ok {
			t.Fatalf("ParseDate(%q) = _, false; want ok", wire)
		}
		if got := FormatDate(d); got != wire {
			t.Errorf("round trip %q -> %v -> %q", wire, d, got)
		}
	}
}

// TestParseDate_Fields verifies the parsed components.
func TestParseDate_Fields(t *testing.T) {
	d, ok := ParseDate("20260515")
	if !ok {
		t.Fatal("ParseDate returned !ok")
	}
	if d.Year != 2026 || d.Month != time.May || d.Day != 15 {
		t.Errorf("ParseDate = %+v, want 2026-May-15", d)
	}
}

// TestParseDate_Rejects verifies ParseDate is strict: it takes only
// the 8-octet DATE form and nothing else.
func TestParseDate_Rejects(t *testing.T) {
	bad := []string{
		"",                 // empty
		"2026051",          // too short
		"202605150",        // too long
		"20260515T000000Z", // DATE-TIME form #2
		"20260515T000000",  // DATE-TIME form #1
		"2026-05-15",       // ISO 8601 extended
		"2026051a",         // non-digit
		"20261315",         // month 13
		"20260230",         // Feb 30 — impossible
		"20260500",         // day 0
		"19000229",         // 1900 is not a leap year
		" 20260515",        // leading space
		"20260515 ",        // trailing space
	}
	for _, s := range bad {
		if d, ok := ParseDate(s); ok {
			t.Errorf("ParseDate(%q) = %+v, true; want false", s, d)
		}
	}
}

// TestParseTime_StillRejectsDateOnly pins ParseTime's documented
// contract: DATE-only input stays rejected. The DATE value type is
// served by ParseDate, not by loosening ParseTime.
func TestParseTime_StillRejectsDateOnly(t *testing.T) {
	if got, ok := ParseTime("20260515"); ok {
		t.Errorf("ParseTime(%q) = %v, true; want zero, false", "20260515", got)
	}
}

// TestDate_IsZero verifies the zero-value predicate.
func TestDate_IsZero(t *testing.T) {
	if !(Date{}).IsZero() {
		t.Error("Date{}.IsZero() = false, want true")
	}
	if (Date{Year: 2026, Month: time.May, Day: 15}).IsZero() {
		t.Error("non-zero Date reported IsZero")
	}
}

// TestDate_Time verifies conversion to a midnight-UTC time.Time for
// callers that need to interoperate with time-based arithmetic. The
// conversion is explicit and lossy by design: the resulting
// time.Time no longer records that the source was date-only.
func TestDate_Time(t *testing.T) {
	d := Date{Year: 2026, Month: time.May, Day: 15}
	got := d.Time()
	want := time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("Date.Time() = %v, want %v", got, want)
	}
}

// TestDateOf verifies constructing a Date from a time.Time takes the
// calendar date in that value's own location.
func TestDateOf(t *testing.T) {
	// 2026-05-15 20:00 in a -05:00 zone is 2026-05-16 01:00 UTC. DateOf
	// must report the LOCAL calendar date, not the UTC one.
	zone := time.FixedZone("X", -5*60*60)
	in := time.Date(2026, time.May, 15, 20, 0, 0, 0, zone)
	got := DateOf(in)
	want := Date{Year: 2026, Month: time.May, Day: 15}
	if got != want {
		t.Errorf("DateOf(%v) = %+v, want %+v", in, got, want)
	}
}

// TestDate_String verifies the fmt.Stringer form is the wire form.
func TestDate_String(t *testing.T) {
	d := Date{Year: 2026, Month: time.May, Day: 15}
	if got := d.String(); got != "20260515" {
		t.Errorf("Date.String() = %q, want %q", got, "20260515")
	}
}
