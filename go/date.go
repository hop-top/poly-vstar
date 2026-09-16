// SPDX-License-Identifier: MIT

package vstar

import (
	"strconv"
	"time"
)

// dateFormat is RFC 5545 §3.3.4 DATE — `YYYYMMDD`, e.g. "20260515".
// Eight octets: four-digit year, two-digit month, two-digit day. No
// 'T', no time, no zone designator.
const dateFormat = "20060102"

// dateOctets is the exact wire width of an RFC 5545 §3.3.4 DATE.
const dateOctets = 8

// Date is an RFC 5545 §3.3.4 DATE value: a calendar date with no
// time and no time zone.
//
// # Why a distinct type
//
// DATE and DATE-TIME are semantically different, not two spellings
// of one thing. `DUE;VALUE=DATE:20260515` means "due on the 15th,
// as reckoned by whoever reads it"; `DUE:20260515T000000Z` means
// "due at one specific instant, the stroke of midnight UTC". A task
// due on the 15th is not late at 00:00:01Z; a task due at midnight
// UTC is.
//
// A time.Time cannot carry that distinction. Every time.Time has
// clock fields and a Location, so a date-only value stored in one is
// indistinguishable from a midnight instant — the caller is left
// consulting an out-of-band flag to know which meaning applies, and
// forgetting to is silent data corruption rather than a type error.
// Date has no clock and no Location fields at all, so the
// distinction cannot be lost by accident: it is structural.
//
// The cost is a second type in the API surface and a second set of
// accessors (DTSTARTDate, DUEDate, …) alongside the DATE-TIME ones.
// That cost is paid once, at the call site, where the caller already
// has to decide what an all-day value means for their domain. Use
// Component.IsDateOnly to branch when the wire form is unknown.
//
// The zero Date is the "no date" sentinel, mirroring the zero
// time.Time's role in FormatTime / the Set* writers: it formats as
// the empty string, and the date-typed setters treat it as "clear
// the property".
//
// Date is comparable: `a == b` is date equality.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns the calendar date of t as observed in t's own
// Location.
//
// The conversion is deliberately location-sensitive. 2026-05-15
// 20:00 in a UTC-05:00 zone is 2026-05-16 01:00 UTC; DateOf reports
// May 15, the date a person standing in that zone would name. Callers
// wanting the UTC date should pass t.UTC().
//
// DateOf(time.Time{}) returns the zero Date.
func DateOf(t time.Time) Date {
	if t.IsZero() {
		return Date{}
	}
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// IsZero reports whether d is the zero Date — the "no date"
// sentinel. Mirrors time.Time.IsZero.
func (d Date) IsZero() bool { return d == Date{} }

// Time returns d as midnight UTC, for callers that need to hand the
// value to time-based arithmetic (durations, comparisons against
// instants).
//
// The conversion is lossy by design and one-way: the returned
// time.Time no longer records that its source was date-only. Do not
// round-trip a Date through Time to store it — use the date-typed
// accessors, which preserve the DATE value type on the wire.
//
// The zero Date returns the zero time.Time.
func (d Date) Time() time.Time {
	if d.IsZero() {
		return time.Time{}
	}
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// String returns the RFC 5545 §3.3.4 wire form, or "" for the zero
// Date. Equivalent to FormatDate(d).
func (d Date) String() string { return FormatDate(d) }

// FormatDate renders d as an RFC 5545 §3.3.4 DATE string —
// `YYYYMMDD`, zero-padded to eight octets.
//
// The zero Date renders as the empty string, so the date-typed
// property writers can use it to mean "clear the property" — the
// same convention FormatTime uses for the zero time.Time.
//
// Out-of-range field values (month 13, day 32, a year outside
// 0000–9999) render as the empty string rather than an impossible
// wire form; the DATE production is a fixed-width four-digit year,
// and emitting torn data would defeat the strictness the reader
// enforces.
func FormatDate(d Date) string {
	if d.IsZero() {
		return ""
	}
	if d.Year < 0 || d.Year > 9999 ||
		d.Month < time.January || d.Month > time.December ||
		d.Day < 1 || d.Day > 31 {
		return ""
	}
	// time.Format is not used here: it would happily normalize
	// out-of-range fields (Feb 30 → Mar 2) via time.Date, hiding
	// producer bugs. Build the fixed-width form directly.
	buf := make([]byte, 0, dateOctets)
	buf = appendPadded(buf, d.Year, 4)
	buf = appendPadded(buf, int(d.Month), 2)
	buf = appendPadded(buf, d.Day, 2)
	return string(buf)
}

// appendPadded appends n to buf as exactly width zero-padded ASCII
// digits. Callers guarantee 0 <= n < 10^width.
func appendPadded(buf []byte, n, width int) []byte {
	s := strconv.Itoa(n)
	for i := len(s); i < width; i++ {
		buf = append(buf, '0')
	}
	return append(buf, s...)
}

// ParseDate parses an RFC 5545 §3.3.4 DATE string (`YYYYMMDD`) into
// a Date. Returns (zero, false) for any other input shape — strict
// by design, matching ParseTime's posture.
//
// Specifically rejected:
//
//   - DATE-TIME forms (`YYYYMMDDTHHMMSS`, `YYYYMMDDTHHMMSSZ`) —
//     those are ParseTime / ParseTimeWithTZID's business.
//   - ISO 8601 extended layouts (`2026-05-15`).
//   - Impossible calendar dates (Feb 30, month 13, day 0,
//     Feb 29 in a non-leap year) — no silent roll-over.
//   - Empty strings, leading/trailing whitespace or extra octets,
//     and any input not exactly 8 octets long.
//
// The bool IS the error signal — no sentinel returned, matching
// ParseTime.
func ParseDate(s string) (Date, bool) {
	if len(s) != dateOctets {
		return Date{}, false
	}
	// time.Parse validates field ranges and rejects impossible dates
	// (it does NOT roll Feb 30 into Mar 2 — that is time.Date's
	// behavior, not time.Parse's). It also rejects non-digit octets
	// and any layout mismatch, which covers "2026-05-" and friends.
	t, err := time.ParseInLocation(dateFormat, s, time.UTC)
	if err != nil {
		return Date{}, false
	}
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}, true
}
