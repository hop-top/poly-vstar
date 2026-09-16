// SPDX-License-Identifier: MIT

package vstar

import (
	"strconv"
	"strings"
	"time"
)

// ParseTimeWithTZID parses an RFC 5545 §3.3.5 form #1 string
// (`YYYYMMDDTHHMMSS`, no Z) as a wall-clock time in the IANA zone
// identified by tzid, where the zone is reconstructed from a
// VTIMEZONE component inside cal.
//
// Returns (zero, false) when:
//
//   - tzid is empty;
//   - cal contains no VTIMEZONE with a matching TZID property;
//   - the matching VTIMEZONE has no STANDARD/DAYLIGHT children, or
//     either child carries a malformed TZOFFSETFROM/TZOFFSETTO;
//   - the value isn't form #1 (15 octets, exactly one 'T' at index
//     8, no Z suffix);
//   - the calendar carries an RRULE pattern this implementation
//     doesn't recognize. v0.1 supports STANDARD-only (no RRULE,
//     trivial fixed offset) and STANDARD+DAYLIGHT with
//     `FREQ=YEARLY` rules. RDATE-only zones, multiple STANDARD
//     entries, and any non-yearly RRULE return (zero, false).
//
// The returned time.Time carries a FixedZone reflecting the offset
// active for that specific wall-clock instant. Call .UTC() to get
// the canonical UTC instant.
//
// Form #2 (Z-suffixed UTC) inputs are rejected here even when
// a TZID parameter is present — form #2 + TZID is contradictory
// wire output (a producer bug), and V*'s canonical layer falls
// back to verbatim emit in that case rather than re-shaping (see
// spec/03 §Datetime resolution). Callers handling already-UTC
// datetimes should use vstar.ParseTime instead.
func ParseTimeWithTZID(s, tzid string, cal Calendar) (time.Time, bool) {
	if tzid == "" {
		return time.Time{}, false
	}
	if !isFormOne(s) {
		return time.Time{}, false
	}
	tz, ok := findVTimezone(tzid, cal)
	if !ok {
		return time.Time{}, false
	}
	rules, ok := loadTZRules(tz)
	if !ok {
		return time.Time{}, false
	}
	// Parse the wall-clock fields as if UTC; we'll re-attach the
	// active offset below. This is safe because we never expose
	// this UTC-shaped intermediate to callers — only its decomposed
	// fields, which are stable across zones.
	wall, err := time.ParseInLocation(timeFormatLocal, s, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	offset, name := selectActiveRule(wall, rules)
	loc := time.FixedZone(name, offset)
	return time.Date(wall.Year(), wall.Month(), wall.Day(),
		wall.Hour(), wall.Minute(), wall.Second(), 0, loc), true
}

// isFormOne reports whether s exactly matches RFC 5545 §3.3.5 form
// #1 wire shape (15 octets, "T" at index 8, no "Z" suffix). Field
// validity (month, day, hour ranges) is delegated to time.Parse.
func isFormOne(s string) bool {
	if len(s) != 15 {
		return false
	}
	if s[8] != 'T' {
		return false
	}
	if s[len(s)-1] == 'Z' || s[len(s)-1] == 'z' {
		return false
	}
	return true
}

// tzRuleSet is the std + optional dst pair extracted from a
// VTIMEZONE.
type tzRuleSet struct {
	std    tzRule
	dst    tzRule
	hasDST bool
}

// tzRule captures the subset of a STANDARD/DAYLIGHT child we need
// to compute transition instants and the offset to apply.
type tzRule struct {
	name       string       // TZNAME, e.g. "EST" / "EDT" — falls back to "" when absent.
	offsetTo   int          // TZOFFSETTO in seconds east of UTC.
	offsetFrom int          // TZOFFSETFROM in seconds east of UTC.
	yearly     bool         // RRULE FREQ=YEARLY. False when no RRULE.
	month      int          // BYMONTH (1-12). Zero when no RRULE.
	weekday    time.Weekday // BYDAY weekday code.
	week       int          // BYDAY ordinal: positive = nth from start, negative = nth from end.
	hour       int          // DTSTART hour
	minute     int          // DTSTART minute
	second     int          // DTSTART second
}

// loadTZRules extracts the STANDARD/DAYLIGHT rule pair from a
// VTIMEZONE component. Returns (zero, false) on any shape
// violation.
func loadTZRules(tz Component) (tzRuleSet, bool) {
	standards := subsByType(tz, "STANDARD")
	daylights := subsByType(tz, "DAYLIGHT")

	if len(standards) == 0 && len(daylights) == 0 {
		return tzRuleSet{}, false
	}
	if len(standards) > 1 || len(daylights) > 1 {
		// Multiple STANDARD/DAYLIGHT entries (historical timezones)
		// outside v0.1 scope.
		return tzRuleSet{}, false
	}

	rs := tzRuleSet{}
	if len(standards) == 1 {
		r, ok := parseTZRule(standards[0])
		if !ok {
			return tzRuleSet{}, false
		}
		rs.std = r
	} else {
		// DAYLIGHT-only — treat the daylight rule as the "std"
		// (single-offset) since there's no transition to compute.
		r, ok := parseTZRule(daylights[0])
		if !ok {
			return tzRuleSet{}, false
		}
		rs.std = r
		return rs, true
	}

	if len(daylights) == 1 {
		r, ok := parseTZRule(daylights[0])
		if !ok {
			return tzRuleSet{}, false
		}
		rs.dst = r
		rs.hasDST = true
		// When both children present we require both to carry a
		// FREQ=YEARLY rule — anything else is unsupported.
		if !rs.std.yearly || !rs.dst.yearly {
			return tzRuleSet{}, false
		}
	}

	return rs, true
}

// selectActiveRule returns the active offset (seconds east of UTC)
// and TZNAME for the wall-clock instant.
//
// For fixed-offset zones (no DAYLIGHT child) we return the std
// rule's offsetTo unconditionally.
//
// For DST zones we compute, for the year of `wall`, both transition
// instants (in wall-clock terms) and pick the rule whose interval
// `wall` falls into. The interval [dstStart, stdStart) uses the
// daylight offset; everything else uses the standard offset.
func selectActiveRule(wall time.Time, rs tzRuleSet) (int, string) {
	if !rs.hasDST {
		return rs.std.offsetTo, rs.std.name
	}
	year := wall.Year()
	dstStart := transitionAt(year, rs.dst)
	stdStart := transitionAt(year, rs.std)
	// Compare on the wall-clock fields directly. Both transition
	// instants are wall-clock times by construction (no zone math
	// here), so a naive Before/After comparison on the same UTC
	// timeline is correct for ordering wall events within one year.
	if !wall.Before(dstStart) && wall.Before(stdStart) {
		return rs.dst.offsetTo, rs.dst.name
	}
	return rs.std.offsetTo, rs.std.name
}

// transitionAt returns the wall-clock instant at which rule r
// becomes active in `year`, expressed as a UTC-shaped time.Time
// (the Location is time.UTC but the fields are wall-clock values
// in the zone). Caller compares two such instants from the same
// year on the same timeline; absolute UTC value is meaningless.
func transitionAt(year int, r tzRule) time.Time {
	month := time.Month(r.month)
	day := nthWeekdayOfMonth(year, month, r.weekday, r.week)
	return time.Date(year, month, day, r.hour, r.minute, r.second, 0, time.UTC)
}

// nthWeekdayOfMonth returns the day-of-month for the nth occurrence
// of weekday wd in (year, month). Positive n counts from the start
// (1 = first, 2 = second…); negative n counts from the end (-1 =
// last). For RFC 5545 BYDAY semantics, n=0 is invalid (rejected at
// parse time).
func nthWeekdayOfMonth(year int, month time.Month, wd time.Weekday, n int) int {
	if n > 0 {
		// Find first occurrence of wd, then add (n-1) weeks.
		first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		offset := (int(wd) - int(first.Weekday()) + 7) % 7
		return 1 + offset + (n-1)*7
	}
	// n < 0 — last occurrence is somewhere in the final week.
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC) // last day of month.
	offset := (int(last.Weekday()) - int(wd) + 7) % 7
	return last.Day() - offset + (n+1)*7
}

// findVTimezone returns the VTIMEZONE component in cal whose TZID
// property matches tzid (case-sensitive — TZIDs are opaque IANA-
// style identifiers per RFC 5545 §3.2.19).
func findVTimezone(tzid string, cal Calendar) (Component, bool) {
	for _, c := range cal.Filter(CompTimezone) {
		if id, ok := c.Get("TZID"); ok && id.Value == tzid {
			return c, true
		}
	}
	return Component{}, false
}

// subsByType returns sub-components of c whose Type matches name
// (case-sensitive — RFC 5545 component identifiers are uppercase).
func subsByType(c Component, name string) []Component {
	var out []Component
	for _, s := range c.Sub {
		if string(s.Type) == name {
			out = append(out, s)
		}
	}
	return out
}

// parseTZRule extracts a tzRule from a STANDARD or DAYLIGHT
// sub-component. Returns (zero, false) on any field shape it
// doesn't understand.
func parseTZRule(c Component) (tzRule, bool) {
	r := tzRule{}

	if p, ok := c.Get("TZNAME"); ok {
		r.name = p.Value
	}

	to, ok := getOffset(c, "TZOFFSETTO")
	if !ok {
		return tzRule{}, false
	}
	r.offsetTo = to

	from, ok := getOffset(c, "TZOFFSETFROM")
	if !ok {
		return tzRule{}, false
	}
	r.offsetFrom = from

	dt, ok := c.Get("DTSTART")
	if !ok {
		return tzRule{}, false
	}
	parsedDT, err := time.Parse(timeFormatLocal, dt.Value)
	if err != nil {
		return tzRule{}, false
	}
	r.hour, r.minute, r.second = parsedDT.Hour(), parsedDT.Minute(), parsedDT.Second()

	if p, ok := c.Get("RRULE"); ok {
		if !parseYearlyRRULE(p.Value, &r) {
			return tzRule{}, false
		}
		r.yearly = true
	}

	return r, true
}

// getOffset reads a UTC offset property (TZOFFSETFROM / TZOFFSETTO)
// in the wire form `±HHMM` or `±HHMMSS`, returning seconds east of
// UTC. Returns (0, false) on shape mismatch.
func getOffset(c Component, name string) (int, bool) {
	p, ok := c.Get(name)
	if !ok {
		return 0, false
	}
	v := p.Value
	if len(v) != 5 && len(v) != 7 {
		return 0, false
	}
	var sign int
	switch v[0] {
	case '+':
		sign = 1
	case '-':
		sign = -1
	default:
		return 0, false
	}
	hh, err := strconv.Atoi(v[1:3])
	if err != nil {
		return 0, false
	}
	mm, err := strconv.Atoi(v[3:5])
	if err != nil {
		return 0, false
	}
	ss := 0
	if len(v) == 7 {
		ss, err = strconv.Atoi(v[5:7])
		if err != nil {
			return 0, false
		}
	}
	return sign * (hh*3600 + mm*60 + ss), true
}

// parseYearlyRRULE accepts only `FREQ=YEARLY` rules with optional
// BYMONTH and BYDAY=<n><WEEKDAY> parts (n a small signed non-zero
// integer). Returns false for anything else, including UNTIL,
// COUNT, BYWEEKNO, BYSETPOS, or unrecognized keys.
func parseYearlyRRULE(s string, r *tzRule) bool {
	parts := strings.Split(s, ";")
	freqSeen := false
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			return false
		}
		key := strings.ToUpper(kv[0])
		val := kv[1]
		switch key {
		case "FREQ":
			if !strings.EqualFold(val, "YEARLY") {
				return false
			}
			freqSeen = true
		case "BYMONTH":
			m, err := strconv.Atoi(val)
			if err != nil || m < 1 || m > 12 {
				return false
			}
			r.month = m
		case "BYDAY":
			ord, wd, ok := parseBYDAY(val)
			if !ok {
				return false
			}
			r.week = ord
			r.weekday = wd
		case "INTERVAL":
			// Accept INTERVAL=1 silently; reject anything else.
			n, err := strconv.Atoi(val)
			if err != nil || n != 1 {
				return false
			}
		default:
			// Unknown RRULE part — fail closed rather than silently
			// applying a partial rule. A producer including UNTIL,
			// COUNT, BYWEEKNO, etc. is outside the v0.1 subset.
			return false
		}
	}
	return freqSeen
}

// parseBYDAY parses a single BYDAY entry like "2SU" or "-1SU" into
// (ordinal, weekday). The 0-ordinal forms ("SU", "0SU") are
// rejected — VTIMEZONE rules require an explicit nth.
func parseBYDAY(s string) (int, time.Weekday, bool) {
	if len(s) < 3 {
		return 0, 0, false
	}
	wdCode := s[len(s)-2:]
	wd, ok := weekdayFromCode(wdCode)
	if !ok {
		return 0, 0, false
	}
	prefix := s[:len(s)-2]
	if prefix == "" {
		return 0, 0, false
	}
	n, err := strconv.Atoi(prefix)
	if err != nil || n == 0 {
		return 0, 0, false
	}
	return n, wd, true
}

func weekdayFromCode(code string) (time.Weekday, bool) {
	switch strings.ToUpper(code) {
	case "SU":
		return time.Sunday, true
	case "MO":
		return time.Monday, true
	case "TU":
		return time.Tuesday, true
	case "WE":
		return time.Wednesday, true
	case "TH":
		return time.Thursday, true
	case "FR":
		return time.Friday, true
	case "SA":
		return time.Saturday, true
	}
	return 0, false
}
