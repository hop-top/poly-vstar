// SPDX-License-Identifier: MIT

package vstar

import (
	"testing"
	"time"
)

// TestFormatTime_KnownInstant verifies a well-known UTC instant is
// formatted using RFC 5545 §3.3.5 form #2 ("date with UTC time").
func TestFormatTime_KnownInstant(t *testing.T) {
	in := time.Date(2026, 5, 4, 18, 30, 45, 0, time.UTC)
	got := FormatTime(in)
	want := "20260504T183045Z"
	if got != want {
		t.Errorf("FormatTime(%v) = %q, want %q", in, got, want)
	}
}

// TestFormatTime_NonUTCConverted verifies non-UTC inputs are
// converted to UTC before formatting (callers don't have to pre-
// normalise; FormatTime is the chokepoint for the UTC invariant).
func TestFormatTime_NonUTCConverted(t *testing.T) {
	mtl := time.FixedZone("EST", -5*60*60) // arbitrary fixed -5h zone.
	// 13:30:45 -05:00 → 18:30:45 UTC.
	in := time.Date(2026, 5, 4, 13, 30, 45, 0, mtl)
	got := FormatTime(in)
	want := "20260504T183045Z"
	if got != want {
		t.Errorf("FormatTime(%v) = %q, want %q", in, got, want)
	}
}

// TestFormatTime_ZeroEmpty verifies the zero time renders as empty
// string. Callers (Set* writers) treat empty as "clear the property".
func TestFormatTime_ZeroEmpty(t *testing.T) {
	got := FormatTime(time.Time{})
	if got != "" {
		t.Errorf("FormatTime(zero) = %q, want %q", got, "")
	}
}

// TestFormatTime_NanosTruncated verifies sub-second precision is
// dropped — RFC 5545 form #2 has only second resolution.
func TestFormatTime_NanosTruncated(t *testing.T) {
	in := time.Date(2026, 5, 4, 18, 30, 45, 123456789, time.UTC)
	got := FormatTime(in)
	want := "20260504T183045Z"
	if got != want {
		t.Errorf("FormatTime(%v) = %q, want %q", in, got, want)
	}
}

// TestParseTime_FormTwoUTC verifies the canonical form #2
// (`YYYYMMDDTHHMMSSZ`) parses to the equivalent UTC instant.
func TestParseTime_FormTwoUTC(t *testing.T) {
	got, ok := ParseTime("20260504T183045Z")
	if !ok {
		t.Fatalf("ParseTime: ok=false, want true")
	}
	want := time.Date(2026, 5, 4, 18, 30, 45, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ParseTime = %v, want %v", got, want)
	}
	if got.Location() != time.UTC {
		t.Errorf("ParseTime Location = %v, want UTC", got.Location())
	}
}

// TestParseTime_RejectsFormOneLocal verifies form #1 (no Z, local
// time) is rejected. Floating local times are not supported
// and silent coercion would mask torn data.
func TestParseTime_RejectsFormOneLocal(t *testing.T) {
	got, ok := ParseTime("20260504T183045")
	if ok {
		t.Errorf("ParseTime(form #1 local) = (%v, true), want (zero, false)", got)
	}
	if !got.IsZero() {
		t.Errorf("ParseTime returned non-zero on failure: %v", got)
	}
}

// TestParseTime_RejectsFormThreeTZID verifies a form #3 input (with
// TZID parameter, handled by ParseTimeWithTZID) is rejected by
// ParseTime — that form must go through the TZID-aware entrypoint.
// Note: form #3 wire shape on the value side looks identical to
// form #1; the TZID parameter lives in the property params, not in
// the value. So we cover that via the form #1 rejection above. This
// test additionally rejects an input that looks like a date-only
// (form #1 of the DATE value type) which must NOT silently parse.
func TestParseTime_RejectsDateOnly(t *testing.T) {
	got, ok := ParseTime("20260504")
	if ok {
		t.Errorf("ParseTime(date-only) = (%v, true), want (zero, false)", got)
	}
}

// TestParseTime_RejectsRFC3339 verifies RFC 3339 layouts are
// rejected. Strict by design — readers should see torn data, not
// silently coerce.
func TestParseTime_RejectsRFC3339(t *testing.T) {
	cases := []string{
		"2026-05-04T18:30:45Z",
		"2026-05-04T18:30:45+00:00",
		"2026-05-04T18:30:45.123Z",
	}
	for _, in := range cases {
		got, ok := ParseTime(in)
		if ok {
			t.Errorf("ParseTime(%q) = (%v, true), want (zero, false)", in, got)
		}
	}
}

// TestParseTime_RejectsEmpty verifies the empty string fails.
func TestParseTime_RejectsEmpty(t *testing.T) {
	got, ok := ParseTime("")
	if ok {
		t.Errorf("ParseTime(\"\") = (%v, true), want (zero, false)", got)
	}
}

// TestParseTime_RejectsLowerZ verifies a lower-case `z` suffix is
// rejected. RFC 5545 §3.3.5 specifies upper-case Z; case-insensitive
// parsing here would mask invalid producer output.
func TestParseTime_RejectsLowerZ(t *testing.T) {
	got, ok := ParseTime("20260504T183045z")
	if ok {
		t.Errorf("ParseTime(lower z) = (%v, true), want (zero, false)", got)
	}
}

// TestParseTime_RejectsTrailingJunk verifies extra trailing or
// leading characters fail.
func TestParseTime_RejectsTrailingJunk(t *testing.T) {
	cases := []string{
		"20260504T183045Z ",
		" 20260504T183045Z",
		"20260504T183045ZZ",
		"X20260504T183045Z",
	}
	for _, in := range cases {
		got, ok := ParseTime(in)
		if ok {
			t.Errorf("ParseTime(%q) = (%v, true), want (zero, false)", in, got)
		}
	}
}

// TestParseTime_RejectsImpossibleDate verifies obviously invalid
// dates (wrong month/day) fail rather than rolling over.
func TestParseTime_RejectsImpossibleDate(t *testing.T) {
	cases := []string{
		"20261301T000000Z", // month 13
		"20260230T000000Z", // Feb 30
		"20260504T256000Z", // hour 25
	}
	for _, in := range cases {
		got, ok := ParseTime(in)
		if ok {
			t.Errorf("ParseTime(%q) = (%v, true), want (zero, false)", in, got)
		}
	}
}

// TestParseTime_RoundTrip verifies FormatTime → ParseTime returns
// the original instant.
func TestParseTime_RoundTrip(t *testing.T) {
	in := time.Date(2026, 5, 4, 18, 30, 45, 0, time.UTC)
	s := FormatTime(in)
	got, ok := ParseTime(s)
	if !ok {
		t.Fatalf("ParseTime(%q) ok=false", s)
	}
	if !got.Equal(in) {
		t.Errorf("round-trip = %v, want %v", got, in)
	}
}
