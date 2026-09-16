// SPDX-License-Identifier: MIT

package duration_test

import (
	"errors"
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/duration"
)

// TestParse_Grammar walks the RFC 5545 §3.3.6 grammar table: the
// week form, the day form, the bare time form, and the combined
// day+time form, in both signs.
func TestParse_Grammar(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want duration.Duration
	}{
		{"days only", "P15D", duration.Duration{Days: 15, DayForm: true}},
		{"hours only", "PT1H", duration.Duration{Hours: 1}},
		{"negative minutes", "-PT15M", duration.Duration{Negative: true, Minutes: 15}},
		{"day plus time", "P1DT2H30M", duration.Duration{Days: 1, Hours: 2, Minutes: 30, DayForm: true}},
		{"weeks", "P2W", duration.Duration{Weeks: 2}},
		{"zero seconds", "PT0S", duration.Duration{Seconds: 0}},
		{"explicit plus", "+PT15M", duration.Duration{Minutes: 15}},
		{"negative weeks", "-P2W", duration.Duration{Negative: true, Weeks: 2}},
		{"full time part", "PT1H30M45S", duration.Duration{Hours: 1, Minutes: 30, Seconds: 45}},
		{"seconds only", "PT45S", duration.Duration{Seconds: 45}},
		{"minutes and seconds", "PT30M10S", duration.Duration{Minutes: 30, Seconds: 10}},
		{"day with seconds", "P7DT5S", duration.Duration{Days: 7, Seconds: 5, DayForm: true}},
		{"zero days", "P0D", duration.Duration{Days: 0, DayForm: true}},
		{"negative day+time", "-P1DT2H", duration.Duration{Negative: true, Days: 1, Hours: 2, DayForm: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := duration.Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("Parse(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

// TestParse_Malformed asserts the parser is strict: every input
// below violates RFC 5545 §3.3.6 and MUST report vstar.ErrMalformed
// rather than coercing to a best-effort value.
func TestParse_Malformed(t *testing.T) {
	bad := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"sign only", "-"},
		{"designator only", "P"},
		{"negative designator only", "-P"},
		{"empty time part", "PT"},
		{"day with empty time part", "P1DT"},
		{"missing P", "T1H"},
		{"missing P with days", "1D"},
		{"lowercase", "pt15m"},
		{"lowercase unit", "PT15m"},
		{"weeks combined with days", "P1W1D"},
		{"weeks combined with time", "P1WT1H"},
		{"time unit outside time part", "P1H"},
		{"minutes outside time part", "P30M"},
		{"seconds outside time part", "P10S"},
		{"unit out of order", "PT1M1H"},
		{"repeated unit", "PT1H1H"},
		{"date unit after time part", "PT1H1D"},
		{"no digits before unit", "PTH"},
		{"unknown unit", "PT1X"},
		{"trailing garbage", "PT1Hx"},
		{"leading whitespace", " PT1H"},
		{"trailing whitespace", "PT1H "},
		{"internal space", "P1D T1H"},
		{"double sign", "--PT1H"},
		{"plus minus", "+-PT1H"},
		{"digits with no unit", "PT1"},
		{"date digits with no unit", "P1"},
		{"rfc3339 style", "P1Y"},
		{"months unsupported", "P1M"},
		{"fractional seconds", "PT1.5S"},
		{"negative component", "PT-1H"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			got, err := duration.Parse(tc.in)
			if err == nil {
				t.Fatalf("Parse(%q) = %+v, want error", tc.in, got)
			}
			if !errors.Is(err, vstar.ErrMalformed) {
				t.Fatalf("Parse(%q) error = %v, want errors.Is(vstar.ErrMalformed)", tc.in, err)
			}
		})
	}
}

// TestFormat_RoundTrip asserts authored-unit fidelity: parsing a
// wire string and re-formatting it MUST reproduce the input byte
// for byte. This is the property that forbids collapsing P1D into
// PT24H internally.
func TestFormat_RoundTrip(t *testing.T) {
	inputs := []string{
		"P15D",
		"PT1H",
		"-PT15M",
		"P1DT2H30M",
		"P2W",
		"PT0S",
		"-P2W",
		"PT1H30M45S",
		"P7DT5S",
		"P0D",
		"-P1DT2H",
		"PT45S",
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			d, err := duration.Parse(in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", in, err)
			}
			if got := d.String(); got != in {
				t.Fatalf("Parse(%q).String() = %q, want %q", in, got, in)
			}
		})
	}
}

// TestFormat_NormalizesPlusSign documents the one intentional
// non-identity: RFC 5545 permits an explicit "+" sign, but the
// canonical output form omits it (a positive duration is the
// default). Re-parsing the output MUST yield an equal value.
func TestFormat_NormalizesPlusSign(t *testing.T) {
	d, err := duration.Parse("+PT15M")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	const want = "PT15M"
	if got := d.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	again, err := duration.Parse(d.String())
	if err != nil {
		t.Fatalf("re-Parse: %v", err)
	}
	if again != d {
		t.Fatalf("re-Parse = %+v, want %+v", again, d)
	}
}

// TestFormat_ZeroValue asserts the zero Duration formats as a
// valid RFC 5545 value rather than the empty string or a bare "P"
// (which the parser itself rejects).
func TestFormat_ZeroValue(t *testing.T) {
	var d duration.Duration
	const want = "PT0S"
	if got := d.String(); got != want {
		t.Fatalf("zero Duration.String() = %q, want %q", got, want)
	}
	back, err := duration.Parse(d.String())
	if err != nil {
		t.Fatalf("Parse(zero.String()): %v", err)
	}
	if back != d {
		t.Fatalf("round-trip zero = %+v, want %+v", back, d)
	}
}

// TestDuration_Signed asserts the sign applies to the WHOLE
// duration, which is what makes "-PT15M" mean "15 minutes before".
func TestDuration_Signed(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"PT15M", 15 * time.Minute},
		{"-PT15M", -15 * time.Minute},
		{"P1DT2H30M", 26*time.Hour + 30*time.Minute},
		{"-P1DT2H30M", -(26*time.Hour + 30*time.Minute)},
		{"P2W", 14 * 24 * time.Hour},
		{"-P2W", -14 * 24 * time.Hour},
		{"PT0S", 0},
		{"P15D", 15 * 24 * time.Hour},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			d, err := duration.Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.in, err)
			}
			if got := d.Signed(); got != tc.want {
				t.Fatalf("Parse(%q).Signed() = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestDuration_IsNegative distinguishes a negative duration from a
// positive one, including the zero case (which is not negative even
// when authored as "-PT0S").
func TestDuration_IsNegative(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"PT15M", false},
		{"-PT15M", true},
		{"P1D", false},
		{"-P1D", true},
		{"PT0S", false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			d, err := duration.Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.in, err)
			}
			if got := d.IsNegative(); got != tc.want {
				t.Fatalf("Parse(%q).IsNegative() = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestDuration_AddTo_PreservesCalendarDays is the DST test that
// justifies the authored-unit design. Across the America/New_York
// spring-forward boundary, "P1D" means the same wall-clock time the
// next calendar day (23 elapsed hours), whereas "PT24H" means 24
// elapsed hours. A nanosecond-only representation cannot tell them
// apart.
func TestDuration_AddTo_PreservesCalendarDays(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	// 2026-03-08 02:00 local is the spring-forward instant.
	start := time.Date(2026, 3, 7, 12, 0, 0, 0, loc)

	oneDay, err := duration.Parse("P1D")
	if err != nil {
		t.Fatalf("Parse(P1D): %v", err)
	}
	twentyFourHours, err := duration.Parse("PT24H")
	if err != nil {
		t.Fatalf("Parse(PT24H): %v", err)
	}

	gotDay := oneDay.AddTo(start)
	gotHours := twentyFourHours.AddTo(start)

	wantDay := time.Date(2026, 3, 8, 12, 0, 0, 0, loc)
	if !gotDay.Equal(wantDay) {
		t.Fatalf("P1D.AddTo(%v) = %v, want %v (same wall clock next day)", start, gotDay, wantDay)
	}
	wantHours := time.Date(2026, 3, 8, 13, 0, 0, 0, loc)
	if !gotHours.Equal(wantHours) {
		t.Fatalf("PT24H.AddTo(%v) = %v, want %v (24 elapsed hours)", start, gotHours, wantHours)
	}
	if gotDay.Equal(gotHours) {
		t.Fatal("P1D and PT24H resolved identically across a DST boundary; authored units were lost")
	}
}

// TestDuration_AddTo_Weeks asserts weeks advance by calendar days
// (7 per week), not by a fixed 168-hour span.
func TestDuration_AddTo_Weeks(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	start := time.Date(2026, 3, 7, 12, 0, 0, 0, loc)
	d, err := duration.Parse("P1W")
	if err != nil {
		t.Fatalf("Parse(P1W): %v", err)
	}
	want := time.Date(2026, 3, 14, 12, 0, 0, 0, loc)
	if got := d.AddTo(start); !got.Equal(want) {
		t.Fatalf("P1W.AddTo(%v) = %v, want %v", start, got, want)
	}
}

// TestDuration_AddTo_Negative asserts a negative duration subtracts
// from the anchor — the "-PT15M means 15 minutes before" contract.
func TestDuration_AddTo_Negative(t *testing.T) {
	start := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	d, err := duration.Parse("-PT15M")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := time.Date(2026, 6, 1, 8, 45, 0, 0, time.UTC)
	if got := d.AddTo(start); !got.Equal(want) {
		t.Fatalf("-PT15M.AddTo(%v) = %v, want %v", start, got, want)
	}
}

// TestDuration_AddTo_NegativeDays asserts the whole-duration sign
// also drives the calendar-day arithmetic backwards.
func TestDuration_AddTo_NegativeDays(t *testing.T) {
	start := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	d, err := duration.Parse("-P2DT1H")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	if got := d.AddTo(start); !got.Equal(want) {
		t.Fatalf("-P2DT1H.AddTo(%v) = %v, want %v", start, got, want)
	}
}

// TestFromSigned builds a Duration from a time.Duration, the
// inverse of Signed for the time-only forms.
func TestFromSigned(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{15 * time.Minute, "PT15M"},
		{-15 * time.Minute, "-PT15M"},
		{0, "PT0S"},
		{90 * time.Minute, "PT1H30M"},
		{26*time.Hour + 30*time.Minute, "PT26H30M"},
		{45 * time.Second, "PT45S"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := duration.FromSigned(tc.in)
			if s := got.String(); s != tc.want {
				t.Fatalf("FromSigned(%v).String() = %q, want %q", tc.in, s, tc.want)
			}
			if back := got.Signed(); back != tc.in.Truncate(time.Second) {
				t.Fatalf("FromSigned(%v).Signed() = %v, want %v", tc.in, back, tc.in)
			}
		})
	}
}

// TestValid reports whether a wire string is a well-formed
// RFC 5545 duration without forcing callers to discard a value.
func TestValid(t *testing.T) {
	if !duration.Valid("-PT15M") {
		t.Fatal(`Valid("-PT15M") = false, want true`)
	}
	if duration.Valid("P1Y") {
		t.Fatal(`Valid("P1Y") = true, want false`)
	}
}
