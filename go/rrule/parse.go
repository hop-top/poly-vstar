// SPDX-License-Identifier: MIT

package rrule

import (
	"fmt"
	"strconv"
	"strings"

	vstar "hop.top/vstar"
)

// ParseRRule decomposes s — the RRULE property value with no
// "RRULE:" prefix, e.g. "FREQ=DAILY;INTERVAL=2;COUNT=10" — into a
// Rule struct. Returns:
//
//   - A populated Rule and nil error on success. Defaults applied
//     when the corresponding rule-part is absent: Interval=1,
//     WeekStart=MO. Slice fields are nil when their rule-part is
//     absent.
//   - vstar.ErrMalformed (wrapped via %w with positional context)
//     for syntactic errors: empty input, missing FREQ, unknown
//     rule-part name, INTERVAL=0/negative, both UNTIL+COUNT,
//     COUNT<=0, BYMONTHDAY=0, BYDAY=<n>SU with n=0, UNTIL not in
//     RFC 5545 form #2, non-integer where integer expected,
//     out-of-range integer values, duplicate rule-part.
//   - ErrUnsupportedRRule (wrapped via %w with the offending
//     rule-part name) for syntactically valid features that
//     spec/03 §RRULE parsing scope defers from v0.2: FREQ=SECONDLY,
//     RSCALE.
//
// Both keys and values must be uppercase per RFC 5545 wire
// convention; the parser is strict (matches vstar.ParseTime's
// strict posture). The order of rule-parts is irrelevant.
func ParseRRule(s string) (Rule, error) {
	if s == "" {
		return Rule{}, fmt.Errorf("rrule: empty input: %w", vstar.ErrMalformed)
	}
	r := Rule{
		Interval:  1,
		WeekStart: MO,
	}
	seen := map[string]bool{}
	parts := strings.Split(s, ";")
	for _, part := range parts {
		key, value, ok := splitRulePart(part)
		if !ok {
			return Rule{}, fmt.Errorf("rrule: malformed rule-part %q: %w", part, vstar.ErrMalformed)
		}
		if seen[key] {
			return Rule{}, fmt.Errorf("rrule: duplicate rule-part %q: %w", key, vstar.ErrMalformed)
		}
		seen[key] = true
		if err := r.applyRulePart(key, value); err != nil {
			return Rule{}, err
		}
	}
	if err := r.validateAfterParse(); err != nil {
		return Rule{}, err
	}
	return r, nil
}

// splitRulePart breaks "KEY=VALUE" into its halves. Returns ok=false
// when the input has no '=' or has an empty key. An empty value is
// allowed at this layer; per-key handlers reject empty values for
// rule-parts that require one.
func splitRulePart(p string) (key, value string, ok bool) {
	idx := strings.IndexByte(p, '=')
	if idx <= 0 {
		return "", "", false
	}
	return p[:idx], p[idx+1:], true
}

// applyRulePart dispatches one KEY=VALUE pair to the right field
// handler. Unknown keys are ErrMalformed (per spec/03 §RRULE parsing
// scope — unknown rule-part name is malformed).
func (r *Rule) applyRulePart(key, value string) error {
	switch key {
	case "FREQ":
		return r.parseFreq(value)
	case "INTERVAL":
		return r.parseInterval(value)
	case "UNTIL":
		return r.parseUntil(value)
	case "COUNT":
		return r.parseCount(value)
	case "BYDAY":
		return r.parseByDay(value)
	case "BYMONTH":
		return r.parseByMonth(value)
	case "BYMONTHDAY":
		return r.parseByMonthDay(value)
	case "BYHOUR":
		return r.parseByHour(value)
	case "BYMINUTE":
		return r.parseByMinute(value)
	case "BYSECOND":
		return r.parseBySecond(value)
	case "BYYEARDAY":
		return r.parseByYearDay(value)
	case "BYWEEKNO":
		return r.parseByWeekNo(value)
	case "BYSETPOS":
		return r.parseBySetPos(value)
	case "WKST":
		return r.parseWKST(value)

	// ── Out of v0.2 scope (spec/03 §RRULE parsing scope) ──────
	case "RSCALE":
		return fmt.Errorf("rrule: rule-part %s: %w", key, ErrUnsupportedRRule)

	default:
		return fmt.Errorf("rrule: unknown rule-part %q: %w", key, vstar.ErrMalformed)
	}
}

func (r *Rule) parseFreq(v string) error {
	switch v {
	case freqMinutely:
		r.Freq = FreqMinutely
	case freqHourly:
		r.Freq = FreqHourly
	case freqDaily:
		r.Freq = FreqDaily
	case freqWeekly:
		r.Freq = FreqWeekly
	case freqMonthly:
		r.Freq = FreqMonthly
	case freqYearly:
		r.Freq = FreqYearly
	case freqSecondly:
		return fmt.Errorf("rrule: FREQ=%s: %w", v, ErrUnsupportedRRule)
	default:
		return fmt.Errorf("rrule: invalid FREQ value %q: %w", v, vstar.ErrMalformed)
	}
	return nil
}

func (r *Rule) parseInterval(v string) error {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("rrule: INTERVAL non-integer %q: %w", v, vstar.ErrMalformed)
	}
	if n < 1 {
		return fmt.Errorf("rrule: INTERVAL must be >= 1, got %d: %w", n, vstar.ErrMalformed)
	}
	r.Interval = n
	return nil
}

func (r *Rule) parseUntil(v string) error {
	t, ok := vstar.ParseTime(v)
	if !ok {
		return fmt.Errorf("rrule: UNTIL must be RFC 5545 form #2 (UTC, Z-suffixed), got %q: %w", v, vstar.ErrMalformed)
	}
	r.Until = t
	return nil
}

func (r *Rule) parseCount(v string) error {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("rrule: COUNT non-integer %q: %w", v, vstar.ErrMalformed)
	}
	if n < 1 {
		return fmt.Errorf("rrule: COUNT must be >= 1, got %d: %w", n, vstar.ErrMalformed)
	}
	r.Count = n
	return nil
}

// parseByDay parses a comma-separated BYDAY list. Each entry is
// "[<ordinal>]<weekday>" where ordinal is in -53..-1 or 1..53 (the
// explicit "0" prefix is rejected) and weekday is one of SU, MO,
// TU, WE, TH, FR, SA.
func (r *Rule) parseByDay(v string) error {
	if v == "" {
		return fmt.Errorf("rrule: BYDAY empty: %w", vstar.ErrMalformed)
	}
	out := []ByDay{}
	for _, entry := range strings.Split(v, ",") {
		bd, err := parseByDayEntry(entry)
		if err != nil {
			return err
		}
		out = append(out, bd)
	}
	r.ByDay = out
	return nil
}

func parseByDayEntry(s string) (ByDay, error) {
	// The weekday is the last 2 characters; everything before is
	// the optional ordinal (which may carry a leading sign).
	if len(s) < 2 {
		return ByDay{}, fmt.Errorf("rrule: BYDAY entry %q too short: %w", s, vstar.ErrMalformed)
	}
	wd := s[len(s)-2:]
	w, err := parseWeekday(wd)
	if err != nil {
		return ByDay{}, err
	}
	prefix := s[:len(s)-2]
	if prefix == "" {
		return ByDay{Weekday: w}, nil
	}
	n, err := strconv.Atoi(prefix)
	if err != nil {
		return ByDay{}, fmt.Errorf("rrule: BYDAY ordinal %q non-integer: %w", prefix, vstar.ErrMalformed)
	}
	if n == 0 {
		return ByDay{}, fmt.Errorf("rrule: BYDAY ordinal 0 invalid (RFC 5545 §3.3.10): %w", vstar.ErrMalformed)
	}
	if n < -53 || n > 53 {
		return ByDay{}, fmt.Errorf("rrule: BYDAY ordinal %d out of range -53..53: %w", n, vstar.ErrMalformed)
	}
	return ByDay{Ordinal: n, Weekday: w}, nil
}

func parseWeekday(s string) (Weekday, error) {
	switch s {
	case "SU":
		return SU, nil
	case "MO":
		return MO, nil
	case "TU":
		return TU, nil
	case "WE":
		return WE, nil
	case "TH":
		return TH, nil
	case "FR":
		return FR, nil
	case "SA":
		return SA, nil
	default:
		return 0, fmt.Errorf("rrule: invalid weekday %q: %w", s, vstar.ErrMalformed)
	}
}

func (r *Rule) parseByMonth(v string) error {
	out, err := parseIntList("BYMONTH", v, 1, 12, false)
	if err != nil {
		return err
	}
	r.ByMonth = out
	return nil
}

// parseByMonthDay accepts -31..-1 or 1..31. 0 is explicitly
// rejected per RFC 5545.
func (r *Rule) parseByMonthDay(v string) error {
	if v == "" {
		return fmt.Errorf("rrule: BYMONTHDAY empty: %w", vstar.ErrMalformed)
	}
	out := []int{}
	for _, raw := range strings.Split(v, ",") {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("rrule: BYMONTHDAY non-integer %q: %w", raw, vstar.ErrMalformed)
		}
		if n == 0 {
			return fmt.Errorf("rrule: BYMONTHDAY 0 invalid (RFC 5545 §3.3.10): %w", vstar.ErrMalformed)
		}
		if n < -31 || n > 31 {
			return fmt.Errorf("rrule: BYMONTHDAY %d out of range -31..31: %w", n, vstar.ErrMalformed)
		}
		out = append(out, n)
	}
	r.ByMonthDay = out
	return nil
}

func (r *Rule) parseByHour(v string) error {
	out, err := parseIntList("BYHOUR", v, 0, 23, false)
	if err != nil {
		return err
	}
	r.ByHour = out
	return nil
}

func (r *Rule) parseByMinute(v string) error {
	out, err := parseIntList("BYMINUTE", v, 0, 59, false)
	if err != nil {
		return err
	}
	r.ByMinute = out
	return nil
}

// parseBySecond accepts 0..60 — 60 is retained for leap seconds
// per RFC 5545 §3.3.10.
func (r *Rule) parseBySecond(v string) error {
	out, err := parseIntList("BYSECOND", v, 0, 60, false)
	if err != nil {
		return err
	}
	r.BySecond = out
	return nil
}

// parseByYearDay accepts -366..-1 or 1..366. 0 is explicitly
// rejected per RFC 5545. The FREQ=YEARLY constraint is enforced in
// validateAfterParse once every rule-part has been consumed
// (rule-part order is irrelevant per RFC).
func (r *Rule) parseByYearDay(v string) error {
	out, err := parseSignedIntList("BYYEARDAY", v, 1, 366)
	if err != nil {
		return err
	}
	r.ByYearDay = out
	return nil
}

// parseByWeekNo accepts -53..-1 or 1..53. 0 is explicitly
// rejected per RFC 5545. The FREQ=YEARLY constraint is enforced in
// validateAfterParse.
func (r *Rule) parseByWeekNo(v string) error {
	out, err := parseSignedIntList("BYWEEKNO", v, 1, 53)
	if err != nil {
		return err
	}
	r.ByWeekNo = out
	return nil
}

// parseBySetPos accepts -366..-1 or 1..366. 0 is explicitly
// rejected per RFC 5545. The "must accompany another BY-*"
// constraint is enforced in validateAfterParse (rule-part order
// is irrelevant per RFC).
func (r *Rule) parseBySetPos(v string) error {
	out, err := parseSignedIntList("BYSETPOS", v, 1, 366)
	if err != nil {
		return err
	}
	r.BySetPos = out
	return nil
}

// parseSignedIntList parses a comma-separated list of signed
// integers where each value n must satisfy lo <= |n| <= hi and
// n != 0. The two-sided range mirrors RFC 5545 §3.3.10's
// "from-start (positive) or from-end (negative)" pattern shared by
// BYYEARDAY, BYWEEKNO, and BYSETPOS.
func parseSignedIntList(name, v string, lo, hi int) ([]int, error) {
	if v == "" {
		return nil, fmt.Errorf("rrule: %s empty: %w", name, vstar.ErrMalformed)
	}
	out := []int{}
	for _, raw := range strings.Split(v, ",") {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("rrule: %s non-integer %q: %w", name, raw, vstar.ErrMalformed)
		}
		if n == 0 {
			return nil, fmt.Errorf("rrule: %s 0 invalid (RFC 5545 §3.3.10): %w", name, vstar.ErrMalformed)
		}
		abs := n
		if abs < 0 {
			abs = -abs
		}
		if abs < lo || abs > hi {
			return nil, fmt.Errorf("rrule: %s %d out of range -%d..-%d or %d..%d: %w", name, n, hi, lo, lo, hi, vstar.ErrMalformed)
		}
		out = append(out, n)
	}
	return out, nil
}

// parseIntList parses a comma-separated list of integers, asserting
// each falls in [lo, hi]. allowNegative is for callers that accept
// negative values inside the same range — currently unused (BYDAY
// and BYMONTHDAY have their own custom parsers because their range
// rules differ).
func parseIntList(name, v string, lo, hi int, _ bool) ([]int, error) {
	if v == "" {
		return nil, fmt.Errorf("rrule: %s empty: %w", name, vstar.ErrMalformed)
	}
	out := []int{}
	for _, raw := range strings.Split(v, ",") {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("rrule: %s non-integer %q: %w", name, raw, vstar.ErrMalformed)
		}
		if n < lo || n > hi {
			return nil, fmt.Errorf("rrule: %s %d out of range %d..%d: %w", name, n, lo, hi, vstar.ErrMalformed)
		}
		out = append(out, n)
	}
	return out, nil
}

func (r *Rule) parseWKST(v string) error {
	w, err := parseWeekday(v)
	if err != nil {
		return err
	}
	r.WeekStart = w
	return nil
}

// validateAfterParse runs cross-field invariants once every
// rule-part has been consumed: FREQ required, UNTIL/COUNT mutual
// exclusion, BYYEARDAY/BYWEEKNO require FREQ=YEARLY, BYSETPOS
// requires at least one other BY-* clause.
func (r *Rule) validateAfterParse() error {
	if r.Freq == FreqInvalid {
		return fmt.Errorf("rrule: FREQ is required: %w", vstar.ErrMalformed)
	}
	if !r.Until.IsZero() && r.Count > 0 {
		return fmt.Errorf("rrule: UNTIL and COUNT are mutually exclusive: %w", vstar.ErrMalformed)
	}
	if len(r.ByYearDay) > 0 && r.Freq != FreqYearly {
		return fmt.Errorf("rrule: BYYEARDAY requires FREQ=YEARLY (RFC 5545 §3.3.10), got FREQ=%s: %w", r.Freq, vstar.ErrMalformed)
	}
	if len(r.ByWeekNo) > 0 && r.Freq != FreqYearly {
		return fmt.Errorf("rrule: BYWEEKNO requires FREQ=YEARLY (RFC 5545 §3.3.10), got FREQ=%s: %w", r.Freq, vstar.ErrMalformed)
	}
	if len(r.BySetPos) > 0 && !r.hasOtherBy() {
		return fmt.Errorf("rrule: BYSETPOS requires at least one other BY-* rule-part (RFC 5545 §3.3.10): %w", vstar.ErrMalformed)
	}
	return nil
}

// hasOtherBy reports whether the rule has at least one BY-* clause
// other than BYSETPOS — the precondition RFC 5545 §3.3.10 imposes
// on BYSETPOS use ("MUST only be used in conjunction with another
// BYxxx rule part").
func (r *Rule) hasOtherBy() bool {
	return len(r.ByDay) > 0 ||
		len(r.ByMonth) > 0 ||
		len(r.ByMonthDay) > 0 ||
		len(r.ByHour) > 0 ||
		len(r.ByMinute) > 0 ||
		len(r.BySecond) > 0 ||
		len(r.ByYearDay) > 0 ||
		len(r.ByWeekNo) > 0
}
