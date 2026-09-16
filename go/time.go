// SPDX-License-Identifier: MIT

package vstar

import "time"

// timeFormatUTC is RFC 5545 §3.3.5 form #2 ("date with UTC time"),
// e.g. "20260504T183045Z". All V* on-the-wire timestamps use this
// form; floating local times are not supported in v0.1.
const timeFormatUTC = "20060102T150405Z"

// timeFormatLocal is RFC 5545 §3.3.5 form #1 ("date with local
// time"), e.g. "20260504T133045". Used in combination with a TZID
// parameter to build form #3 ("date with local time and time zone
// reference"). FormatTime never emits this form (V* writes UTC
// only); ParseTimeWithTZID consumes it.
const timeFormatLocal = "20060102T150405"

// FormatTime renders t as an RFC 5545 §3.3.5 form #2 string —
// `YYYYMMDDTHHMMSSZ` in UTC. Callers may pass any time.Time; the
// value is converted to UTC before formatting, so non-UTC inputs
// produce the same wire output as their UTC equivalent.
//
// The zero time renders as the empty string. Property writers
// (Set*) use this to detect "clear the property" semantics.
//
// Sub-second precision is truncated: RFC 5545 form #2 has only
// second resolution, and writing nanoseconds would produce invalid
// wire output.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(timeFormatUTC)
}

// ParseTime parses an RFC 5545 §3.3.5 form #2 string
// (`YYYYMMDDTHHMMSSZ`) into a UTC time.Time. Returns (zero, false)
// for any other input shape — strict by design.
//
// Specifically rejected:
//
//   - form #1 (`YYYYMMDDTHHMMSS`, local) — handled by
//     ParseTimeWithTZID when accompanied by a TZID parameter; bare
//     local times have no zone and are unsupported in v0.1.
//   - RFC 3339 / ISO 8601 layouts (`2026-05-04T18:30:45Z` etc.) —
//     V* readers should see torn data instead of silently coercing.
//   - Date-only values (`YYYYMMDD`) — DATE value type is a separate
//     concern not handled here.
//   - Lower-case `z` suffix — RFC 5545 mandates upper-case Z.
//   - Empty strings, leading/trailing whitespace or extra octets,
//     and any input not exactly 16 octets long.
//
// The bool IS the error signal — no sentinel returned (per plan).
func ParseTime(s string) (time.Time, bool) {
	// Form #2 is exactly 16 octets: 8 date + T + 6 time + Z.
	if len(s) != 16 {
		return time.Time{}, false
	}
	if s[15] != 'Z' {
		return time.Time{}, false
	}
	// time.Parse already validates field ranges and rejects
	// impossible dates (it does NOT silently roll Feb 30 into Mar 2
	// — that's time.Date's behavior, not time.Parse's). We use
	// time.UTC explicitly so the returned Location is canonical UTC.
	got, err := time.ParseInLocation(timeFormatUTC, s, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return got, true
}
