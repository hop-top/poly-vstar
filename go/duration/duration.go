// SPDX-License-Identifier: MIT

// Package duration implements the RFC 5545 §3.3.6 DURATION value
// type (Parse, Duration.String) and the §3.8.6.3 TRIGGER property
// on VALARM, including relative triggers and their RELATED anchor
// (ParseTrigger, Trigger.Resolve).
//
// # Why a struct and not time.Duration
//
// Duration preserves the units the producer authored — weeks, days,
// hours, minutes, seconds — rather than collapsing them to a
// nanosecond count. time.Duration cannot represent "one day"
// distinctly from "24 hours", but RFC 5545 draws that distinction
// deliberately: a calendar day is 23, 24, or 25 hours across a DST
// transition. Re-serializing a nanosecond count would silently
// rewrite "P1D" as "PT24H" and shift every alarm that crosses a
// transition by an hour.
//
// Both views are available:
//
//   - Duration.Signed reports the nominal time.Duration (days = 24h,
//     weeks = 7 days), which is exact for time-only values and for
//     any value in a fixed-offset zone such as UTC.
//   - Duration.AddTo anchors the value against a real time.Time,
//     advancing calendar days and weeks with time.Time.AddDate so
//     zone transitions are honored, and the time part with Add.
//
// Callers doing wall-clock arithmetic in a zone with DST MUST use
// AddTo. Signed is the right choice for display, sorting, and
// comparisons where nominal length is what is meant.
//
// The package depends only on the standard library plus the root
// vstar package (for ErrMalformed, ParseTime, and the component
// model).
package duration

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	vstar "hop.top/vstar"
)

// Duration is an RFC 5545 §3.3.6 DURATION value in the units its
// producer authored.
//
// The grammar admits either a week form (Weeks alone) or a
// day-and-time form (Days plus an optional Hours/Minutes/Seconds
// time part); the two never mix. Negative applies to the WHOLE
// duration, not to any single field — it is what makes "-PT15M"
// mean "15 minutes before" on a VALARM TRIGGER.
//
// The zero Duration is a valid, positive, zero-length value and
// formats as "PT0S".
type Duration struct {
	// Negative reports that the whole duration is subtractive.
	Negative bool
	// Weeks is the "nW" count. When non-zero every other unit
	// field is zero — RFC 5545 forbids mixing weeks with other
	// units.
	Weeks int
	// Days is the "nD" count.
	Days int
	// Hours is the "nH" count from the time part.
	Hours int
	// Minutes is the "nM" count from the time part.
	Minutes int
	// Seconds is the "nS" count from the time part.
	Seconds int
	// DayForm records that the value was authored in the day form
	// with a zero day count ("P0D"). Every unit field is then zero,
	// which is indistinguishable from the zero Duration, so this
	// flag is what lets String reproduce "P0D" instead of the
	// canonical zero spelling "PT0S". It affects formatting only —
	// Signed, IsNegative and AddTo ignore it, and the two values
	// are numerically identical.
	DayForm bool
}

// Parse decodes s — an RFC 5545 §3.3.6 DURATION value with no
// property name or parameters, e.g. "-PT15M" — into a Duration.
//
// The grammar accepted is exactly:
//
//	dur-value  = ["+" / "-"] "P" (dur-date / dur-time / dur-week)
//	dur-date   = dur-day [dur-time]
//	dur-time   = "T" (dur-hour / dur-minute / dur-second)
//	dur-week   = 1*DIGIT "W"
//	dur-hour   = 1*DIGIT "H" [dur-minute]
//	dur-minute = 1*DIGIT "M" [dur-second]
//	dur-second = 1*DIGIT "S"
//	dur-day    = 1*DIGIT "D"
//
// Parsing is strict, matching vstar.ParseTime's posture. Rejected
// with vstar.ErrMalformed (wrapped via %w): empty input, a missing
// "P", lowercase designators, weeks mixed with any other unit,
// time units outside the "T" part, units out of RFC order,
// repeated units, digits with no unit, an empty "T" part, ISO 8601
// years or months ("P1Y", "P1M" — not in RFC 5545), fractional
// values, per-component signs, and any leading, trailing, or
// internal whitespace.
func Parse(s string) (Duration, error) {
	rest, neg, err := parseSign(s)
	if err != nil {
		return Duration{}, err
	}
	d, err := parseBody(rest, s)
	if err != nil {
		return Duration{}, err
	}
	d.Negative = neg
	return d, nil
}

// parseSign strips the optional leading "+"/"-" and the mandatory
// "P" designator, returning the remainder of the value.
func parseSign(s string) (rest string, neg bool, err error) {
	if s == "" {
		return "", false, fmt.Errorf("duration: empty input: %w", vstar.ErrMalformed)
	}
	switch s[0] {
	case '+':
		s = s[1:]
	case '-':
		neg = true
		s = s[1:]
	}
	if s == "" || s[0] != 'P' {
		return "", false, fmt.Errorf("duration: missing %q designator: %w", "P", vstar.ErrMalformed)
	}
	return s[1:], neg, nil
}

// parseBody decodes the portion after "P": either a week value, or
// a day value with an optional time part, or a bare time part. orig
// is the full input, used only for error messages.
func parseBody(body, orig string) (Duration, error) {
	if body == "" {
		return Duration{}, fmt.Errorf("duration: %q has no value after %q: %w", orig, "P", vstar.ErrMalformed)
	}

	var d Duration

	// A bare time part: "PT...".
	if body[0] == 'T' {
		if err := parseTimePart(body[1:], &d, orig); err != nil {
			return Duration{}, err
		}
		return d, nil
	}

	// Otherwise a date part: weeks, or days with an optional time.
	datePart, timePart, hasTime := strings.Cut(body, "T")

	n, unit, remainder, err := nextField(datePart, orig)
	if err != nil {
		return Duration{}, err
	}
	switch unit {
	case 'W':
		if remainder != "" {
			return Duration{}, fmt.Errorf(
				"duration: %q mixes weeks with other units: %w", orig, vstar.ErrMalformed,
			)
		}
		if hasTime {
			return Duration{}, fmt.Errorf(
				"duration: %q mixes weeks with a time part: %w", orig, vstar.ErrMalformed,
			)
		}
		d.Weeks = n
	case 'D':
		if remainder != "" {
			return Duration{}, fmt.Errorf(
				"duration: %q has trailing input %q after the day value: %w",
				orig, remainder, vstar.ErrMalformed,
			)
		}
		d.Days = n
		d.DayForm = true
	default:
		return Duration{}, fmt.Errorf(
			"duration: %q uses unit %q outside a time part (RFC 5545 has no years or months): %w",
			orig, string(unit), vstar.ErrMalformed,
		)
	}

	if hasTime {
		if err := parseTimePart(timePart, &d, orig); err != nil {
			return Duration{}, err
		}
	}
	return d, nil
}

// parseTimePart decodes the segment after "T" into d's Hours,
// Minutes and Seconds. Units must appear at most once and in RFC
// order (H, then M, then S).
func parseTimePart(s string, d *Duration, orig string) error {
	if s == "" {
		return fmt.Errorf("duration: %q has an empty time part: %w", orig, vstar.ErrMalformed)
	}
	// order tracks how far through H→M→S we have advanced, so a
	// repeated or out-of-order unit is rejected.
	order := 0
	for s != "" {
		n, unit, remainder, err := nextField(s, orig)
		if err != nil {
			return err
		}
		var rank int
		switch unit {
		case 'H':
			rank, d.Hours = 1, n
		case 'M':
			rank, d.Minutes = 2, n
		case 'S':
			rank, d.Seconds = 3, n
		default:
			return fmt.Errorf(
				"duration: %q uses unknown time unit %q: %w", orig, string(unit), vstar.ErrMalformed,
			)
		}
		if rank <= order {
			return fmt.Errorf(
				"duration: %q repeats or misorders time unit %q: %w",
				orig, string(unit), vstar.ErrMalformed,
			)
		}
		order = rank
		s = remainder
	}
	return nil
}

// nextField consumes one "1*DIGIT UNIT" field from the front of s,
// returning the value, the unit byte, and the unconsumed
// remainder.
func nextField(s, orig string) (n int, unit byte, rest string, err error) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, 0, "", fmt.Errorf(
			"duration: %q has a unit with no digits: %w", orig, vstar.ErrMalformed,
		)
	}
	if i == len(s) {
		return 0, 0, "", fmt.Errorf(
			"duration: %q has digits with no unit: %w", orig, vstar.ErrMalformed,
		)
	}
	n, convErr := strconv.Atoi(s[:i])
	if convErr != nil {
		return 0, 0, "", fmt.Errorf(
			"duration: %q has an out-of-range value %q: %w", orig, s[:i], vstar.ErrMalformed,
		)
	}
	return n, s[i], s[i+1:], nil
}

// Valid reports whether s is a well-formed RFC 5545 §3.3.6
// DURATION value. Equivalent to discarding Parse's result, offered
// so callers testing a wire string need not allocate an unused
// Duration.
func Valid(s string) bool {
	_, err := Parse(s)
	return err == nil
}

// String renders d as an RFC 5545 §3.3.6 DURATION value, preserving
// the units the value carries. Parse and String round-trip byte for
// byte, with one intentional normalisation: an explicit "+" sign is
// dropped, since a positive duration is the default.
//
// A Duration with no non-zero unit renders as "PT0S" — the zero
// value is thus a valid wire value, never the empty string or a
// bare "P" (both of which Parse rejects).
func (d Duration) String() string {
	var b strings.Builder
	if d.Negative && !d.isZero() {
		b.WriteByte('-')
	}
	b.WriteByte('P')

	if d.Weeks != 0 {
		fmt.Fprintf(&b, "%dW", d.Weeks)
		return b.String()
	}

	hasTime := d.Hours != 0 || d.Minutes != 0 || d.Seconds != 0

	// A wholly zero Duration has no unit to render, so it takes the
	// canonical zero spelling "PT0S" rather than a bare "P" (which
	// Parse rejects).
	if d.isZero() && !d.DayForm {
		b.WriteString("T0S")
		return b.String()
	}

	if d.Days != 0 || d.DayForm {
		fmt.Fprintf(&b, "%dD", d.Days)
	}
	if !hasTime {
		return b.String()
	}

	b.WriteByte('T')
	if d.Hours != 0 {
		fmt.Fprintf(&b, "%dH", d.Hours)
	}
	if d.Minutes != 0 {
		fmt.Fprintf(&b, "%dM", d.Minutes)
	}
	if d.Seconds != 0 {
		fmt.Fprintf(&b, "%dS", d.Seconds)
	}
	return b.String()
}

// isZero reports whether every unit field is zero.
func (d Duration) isZero() bool {
	return d.Weeks == 0 && d.Days == 0 && d.Hours == 0 && d.Minutes == 0 && d.Seconds == 0
}

// Signed returns d as a nominal time.Duration, negated when d is
// negative. Days count as 24 hours and weeks as 7 days.
//
// This is exact for time-only values and for any anchor in a
// fixed-offset zone (UTC included). It is NOT exact for day or week
// values crossing a DST transition — use AddTo when a real anchor is
// available.
func (d Duration) Signed() time.Duration {
	total := time.Duration(d.Weeks) * 7 * 24 * time.Hour
	total += time.Duration(d.Days) * 24 * time.Hour
	total += time.Duration(d.Hours) * time.Hour
	total += time.Duration(d.Minutes) * time.Minute
	total += time.Duration(d.Seconds) * time.Second
	if d.Negative {
		return -total
	}
	return total
}

// IsNegative reports whether d is subtractive. A zero-length
// duration is never negative, however it was authored.
func (d Duration) IsNegative() bool {
	return d.Negative && !d.isZero()
}

// AddTo advances t by d, honoring calendar semantics: weeks and
// days move by calendar date via time.Time.AddDate, so a value
// crossing a DST transition lands on the same wall-clock time; the
// hour/minute/second part is added as elapsed time.
//
// This is the difference the authored-unit design exists to
// preserve. In America/New_York across the 2026-03-08 spring
// forward, "P1D" from 2026-03-07 12:00 yields 2026-03-08 12:00
// (23 elapsed hours) while "PT24H" yields 2026-03-08 13:00.
//
// A negative d subtracts, moving both the calendar and elapsed
// parts backwards.
func (d Duration) AddTo(t time.Time) time.Time {
	sign := 1
	if d.Negative {
		sign = -1
	}
	days := sign * (d.Weeks*7 + d.Days)
	clock := time.Duration(d.Hours)*time.Hour +
		time.Duration(d.Minutes)*time.Minute +
		time.Duration(d.Seconds)*time.Second
	return t.AddDate(0, 0, days).Add(time.Duration(sign) * clock)
}

// FromSigned converts a time.Duration into a Duration expressed in
// hours, minutes and seconds. Sub-second precision is truncated:
// RFC 5545 durations have second resolution.
//
// The result never uses the week or day units — a time.Duration
// carries no calendar information, so emitting "P1D" from 24 hours
// would invent a distinction the input never made. Callers that
// mean calendar days should build the Duration struct directly.
func FromSigned(td time.Duration) Duration {
	d := Duration{}
	if td < 0 {
		d.Negative = true
		td = -td
	}
	td = td.Truncate(time.Second)
	d.Hours = int(td / time.Hour)
	td -= time.Duration(d.Hours) * time.Hour
	d.Minutes = int(td / time.Minute)
	td -= time.Duration(d.Minutes) * time.Minute
	d.Seconds = int(td / time.Second)
	return d
}
