// SPDX-License-Identifier: MIT

package rrule

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"
)

// MaxIterations is the evaluator's iteration bound: the hard cap on
// per-call NextOccurrence expansion steps, and the number of
// consecutive empty FREQ periods the expansion entry points (All,
// Occurrences, Between, Set) walk before giving up. A rule that
// fails to surface a candidate within this many FREQ-period steps
// causes the evaluator to abandon the search and report
// ErrIterationCap — it is far better to give up loudly than to
// loop forever on a pathological input.
//
// 100_000 is large enough for every realistic recurrence (e.g. a
// FREQ=YEARLY rule iterates ≤ 1 step per year, so the cap covers
// ~100k years of yearly rules and ~273 years of daily rules) and
// small enough to fail fast on rules that never yield (an
// unsatisfiable BY* combination such as
// FREQ=MONTHLY;BYMONTH=2;BYMONTHDAY=30).
//
// It is exported so a caller that receives ErrIterationCap can
// state the bound that was actually hit, and so the cost of a
// worst-case call is a documented property rather than a private
// magic number. The budget itself is deliberately NOT
// configurable: see ErrIterationCap for the rationale.
const MaxIterations = 100000

// ErrIterationCap signals that the evaluator — NextOccurrence, or
// an expansion entry point — exhausted MaxIterations FREQ-period
// steps without finding an occurrence, and gave up. It is NOT a
// statement that the rule has terminated — it is a statement that
// the evaluator stopped looking.
//
// Distinguishing the two matters: a terminated rule is normal
// completion, while a cap hit is a silent failure the caller
// almost certainly wants to surface (it means either an
// unsatisfiable BY* combination or a recurrence sparser than the
// evaluator budget). Before this sentinel existed both outcomes
// returned (zero, false, nil) and were indistinguishable.
//
// Consumers MUST match with errors.Is — every entry point wraps
// this with %w to add the offending rule context.
//
// The cap is a fixed constant rather than a per-call option: the
// only inputs that reach it are unsatisfiable rules, for which a
// larger budget merely costs more before failing identically. A
// caller who needs a tighter bound in a request path should apply
// its own deadline around the call rather than widen this API.
var ErrIterationCap = errors.New("rrule: iteration cap exceeded")

// NextOccurrence returns the next time t' strictly after `after`
// for which `rule` fires, computed relative to `dtstart` (the
// DTSTART of the component carrying the RRULE).
//
// Returns:
//
//   - (t, true, nil) — t is the next occurrence.
//   - (zero, false, nil) — the rule has terminated: UNTIL passed,
//     COUNT exhausted, or the calendar arithmetic ran out of
//     FREQ steps. This is normal completion, not a failure.
//   - (zero, false, ErrIterationCap) — the evaluator stepped
//     through MaxIterations FREQ periods without producing a
//     candidate and abandoned the search. The rule has NOT
//     necessarily terminated; the caller gave up looking. Callers
//     that treat this as ordinary completion will silently drop
//     occurrences, so match it with errors.Is and surface it.
//   - (zero, false, ErrUnsupportedRRule) — guard against future
//     parser/evaluator skew. v0.2 ships parser scope == evaluator
//     scope, so this branch is unreachable from valid inputs at
//     ship; it is kept as the published failure mode for any
//     future v0.2.x patch that surfaces an evaluator gap.
//
// Time-zone semantics: the evaluator operates on time.Time and
// uses its native arithmetic. The caller's `dtstart` zone carries
// through every candidate, so DST boundary normalization (e.g.
// "wall-clock 02:30 doesn't exist on spring-forward day") is
// handled by Go's time package. UTC dtstart in → UTC results out;
// zoned dtstart in → zoned results out.
//
// The first occurrence of a rule is `dtstart` itself per RFC 5545
// (when dtstart matches all BY* filters); pass `after = dtstart`
// to advance past it.
func NextOccurrence(rule Rule, dtstart, after time.Time) (time.Time, bool, error) {
	if rule.Freq == FreqInvalid {
		return time.Time{}, false, fmt.Errorf("rrule: NextOccurrence: FREQ is required: %w", ErrUnsupportedRRule)
	}
	if rule.Interval < 1 {
		// Defensive: ParseRRule guarantees Interval >= 1; but a
		// caller may construct a Rule literal without going through
		// the parser. Treat as INTERVAL=1 implicitly is risky;
		// terminate instead.
		return time.Time{}, false, fmt.Errorf("rrule: NextOccurrence: INTERVAL must be >= 1: %w", ErrUnsupportedRRule)
	}

	// Track count toward COUNT. Per RFC 5545 dtstart is occurrence
	// #1 when it matches the BY* filters, so we begin counting
	// from there.
	count := 0
	current := dtstart

	for range MaxIterations {
		// Expand the current FREQ-period base into one or more
		// concrete occurrences via the BY* filters. Sort to ensure
		// chronological order within the period.
		occurrences := expand(rule, current, dtstart)
		sort.Slice(occurrences, func(i, j int) bool {
			return occurrences[i].Before(occurrences[j])
		})
		// BYSETPOS is the positional filter applied AFTER all other
		// BY-* expansion within a FREQ period (RFC 5545 §3.3.10).
		if len(rule.BySetPos) > 0 {
			occurrences = applyBySetPos(occurrences, rule.BySetPos)
		}
		for _, occ := range occurrences {
			if occ.Before(dtstart) {
				continue
			}
			// UNTIL check (inclusive of UNTIL per RFC 5545).
			if !rule.Until.IsZero() && occ.After(rule.Until) {
				return time.Time{}, false, nil
			}
			count++
			if rule.Count > 0 && count > rule.Count {
				return time.Time{}, false, nil
			}
			if occ.After(after) {
				return occ, true, nil
			}
		}
		next, ok := advance(rule, current)
		if !ok {
			return time.Time{}, false, nil
		}
		current = next
	}
	// Falling out of the loop means the budget ran out, not that
	// the rule ended. Report it so the caller can tell an
	// abandoned search from a completed one.
	return time.Time{}, false, fmt.Errorf(
		"rrule: NextOccurrence: no occurrence after %s within %d %s periods: %w",
		after.Format(time.RFC3339), MaxIterations, rule.Freq, ErrIterationCap,
	)
}

// advance bumps `current` forward by one FREQ * INTERVAL step.
// Returns false only when the FREQ is not one this evaluator can
// step (guarded against upstream, so unreachable from a parsed
// Rule). That false is a termination signal, distinct from the
// iteration cap: NextOccurrence maps it to (zero, false, nil) —
// there are no more periods to walk — whereas exhausting
// MaxIterations means periods remained but the budget did not,
// and reports ErrIterationCap.
func advance(rule Rule, current time.Time) (time.Time, bool) {
	switch rule.Freq {
	case FreqHourly:
		return current.Add(time.Duration(rule.Interval) * time.Hour), true
	case FreqDaily:
		return current.AddDate(0, 0, rule.Interval), true
	case FreqWeekly:
		return current.AddDate(0, 0, 7*rule.Interval), true
	case FreqMonthly:
		return monthlyAdvance(current, rule.Interval), true
	case FreqYearly:
		return yearlyAdvance(current, rule.Interval), true
	default:
		return time.Time{}, false
	}
}

// monthlyAdvance steps `current` forward by `n` months while
// preserving the time-of-day. To avoid time.AddDate's day-overflow
// behavior (e.g. Jan 31 + 1 month → Mar 3), we land on day 1 of
// the target month and let the BY* expansion / dtstart-day fill in
// the day. This is sound because monthly recurrences with no
// BYMONTHDAY/BYDAY filter fire on dtstart's day-of-month, and the
// expand() function reconstructs that from dtstart.
func monthlyAdvance(current time.Time, n int) time.Time {
	year, month, _ := current.Date()
	hour, min, sec := current.Clock()
	loc := current.Location()
	// Use day=1 to avoid overflow; expand() will substitute the
	// correct day for the period.
	return time.Date(year, month+time.Month(n), 1, hour, min, sec, current.Nanosecond(), loc)
}

// yearlyAdvance steps `current` forward by `n` years while
// preserving the time-of-day. Mirrors monthlyAdvance: lands on
// Jan 1 of the target year so the dtstart-anchored month/day is
// not lost to time.AddDate's overflow normalization (which would
// turn 2024-02-29 + 1y into 2025-03-01, then leak March into
// expandYearly's BYMONTH-absent default — silently sliding a
// Feb 29 yearly recurrence into March on every non-leap year).
// expandYearly fills in the month from dtstart.Month() (the RFC
// anchor) and the day from BY-* clauses or dtstart.Day().
func yearlyAdvance(current time.Time, n int) time.Time {
	hour, min, sec := current.Clock()
	loc := current.Location()
	return time.Date(current.Year()+n, time.January, 1, hour, min, sec, current.Nanosecond(), loc)
}

// expand produces every concrete occurrence within the FREQ
// period anchored at `base`, applying BY* filters per RFC 5545
// §3.3.10. dtstart is the original component DTSTART, used as the
// fallback for fields not constrained by a BY* filter (e.g. when
// no BYMONTHDAY is set, MONTHLY fires on dtstart's day-of-month).
//
// The returned slice is unsorted; the caller sorts.
func expand(rule Rule, base, dtstart time.Time) []time.Time {
	switch rule.Freq {
	case FreqHourly:
		return expandHourly(rule, base, dtstart)
	case FreqDaily:
		return expandDaily(rule, base, dtstart)
	case FreqWeekly:
		return expandWeekly(rule, base, dtstart)
	case FreqMonthly:
		return expandMonthly(rule, base, dtstart)
	case FreqYearly:
		return expandYearly(rule, base, dtstart)
	}
	return nil
}

// expandHourly: a single occurrence per hour at base's
// minute/second (or BYMINUTE/BYSECOND if set). BYHOUR is a no-op
// for hourly (every hour fires); BYMONTH/BYMONTHDAY/BYDAY filter.
func expandHourly(rule Rule, base, dtstart time.Time) []time.Time {
	if !matchesByMonth(rule, base) || !matchesByMonthDay(rule, base) || !matchesByDay(rule, base) {
		return nil
	}
	minutes := byOrDefault(rule.ByMinute, base.Minute())
	seconds := byOrDefault(rule.BySecond, base.Second())
	out := make([]time.Time, 0, len(minutes)*len(seconds))
	for _, mn := range minutes {
		for _, sc := range seconds {
			out = append(out, time.Date(base.Year(), base.Month(), base.Day(), base.Hour(), mn, sc, dtstart.Nanosecond(), base.Location()))
		}
	}
	return out
}

// expandDaily: one base day; BYHOUR/BYMINUTE/BYSECOND control the
// time-of-day; BYMONTH/BYMONTHDAY/BYDAY filter.
func expandDaily(rule Rule, base, dtstart time.Time) []time.Time {
	if !matchesByMonth(rule, base) || !matchesByMonthDay(rule, base) || !matchesByDay(rule, base) {
		return nil
	}
	return crossTimeOfDay(rule, base, dtstart)
}

// expandWeekly: when BYDAY is set, enumerate the 7 days of the
// FREQ week (anchored on WKST) and emit one occurrence per matching
// weekday. When BYDAY is absent, fire only on dtstart's weekday
// within the period (RFC 5545 default).
func expandWeekly(rule Rule, base, dtstart time.Time) []time.Time {
	if len(rule.ByDay) == 0 {
		// Default: fire only on dtstart's weekday — i.e. the base
		// itself, since `advance` steps in 7×INTERVAL day units
		// preserving weekday.
		if !matchesByMonth(rule, base) || !matchesByMonthDay(rule, base) {
			return nil
		}
		return crossTimeOfDay(rule, base, dtstart)
	}
	weekStart := startOfWeek(base, rule.WeekStart)
	out := []time.Time{}
	for i := range 7 {
		day := weekStart.AddDate(0, 0, i)
		if !matchesByDay(rule, day) {
			continue
		}
		if !matchesByMonth(rule, day) {
			continue
		}
		if !matchesByMonthDay(rule, day) {
			continue
		}
		out = append(out, crossTimeOfDay(rule, day, dtstart)...)
	}
	return out
}

// expandMonthly: walk every day of the base month; emit
// occurrences for days satisfying BYMONTHDAY (or, when absent, the
// dtstart day) AND BYDAY (or the full set).
func expandMonthly(rule Rule, base, dtstart time.Time) []time.Time {
	if !matchesByMonth(rule, base) {
		return nil
	}
	year, month, _ := base.Date()
	loc := base.Location()
	dayCount := daysInMonth(year, month)

	// Day candidates: BYMONTHDAY if set, else dtstart's day.
	dayCandidates := monthDayCandidates(rule, dayCount, dtstart.Day())

	out := []time.Time{}
	for _, d := range dayCandidates {
		if d < 1 || d > dayCount {
			continue
		}
		day := time.Date(year, month, d, 0, 0, 0, 0, loc)
		if !matchesByDayMonthly(rule, day, year, month, dayCount) {
			continue
		}
		out = append(out, crossTimeOfDay(rule, day, dtstart)...)
	}
	return out
}

// expandYearly: walk the year. When BYYEARDAY is set, expand to
// those specific days-of-year (filtered by BYDAY/BYMONTH if set).
// When BYWEEKNO is set, expand to all 7 days of each named ISO
// week (filtered by BYDAY/BYMONTH). Otherwise: when BYMONTH is
// set, restrict to those months; otherwise use base.Month().
// Within each month, apply BYMONTHDAY/BYDAY/time-of-day expansion
// as for monthly.
func expandYearly(rule Rule, base, dtstart time.Time) []time.Time {
	year := base.Year()
	loc := base.Location()
	if len(rule.ByYearDay) > 0 {
		return expandYearlyByYearDay(rule, year, loc, dtstart)
	}
	if len(rule.ByWeekNo) > 0 {
		return expandYearlyByWeekNo(rule, year, loc, dtstart)
	}
	months := rule.ByMonth
	if len(months) == 0 {
		// Anchor on dtstart.Month() per RFC 5545: yearlyAdvance
		// resets `base` to January (to avoid AddDate's day-overflow
		// normalization), so base.Month() would be misleading here.
		months = []int{int(dtstart.Month())}
	}
	out := []time.Time{}
	for _, m := range months {
		if m < 1 || m > 12 {
			continue
		}
		month := time.Month(m)
		dayCount := daysInMonth(year, month)
		dayCandidates := monthDayCandidates(rule, dayCount, dtstart.Day())
		for _, d := range dayCandidates {
			if d < 1 || d > dayCount {
				continue
			}
			day := time.Date(year, month, d, 0, 0, 0, 0, loc)
			if !matchesByDayMonthly(rule, day, year, month, dayCount) {
				continue
			}
			out = append(out, crossTimeOfDay(rule, day, dtstart)...)
		}
	}
	return out
}

// expandYearlyByYearDay produces occurrences for each BYYEARDAY
// entry, applying BYMONTH and BYDAY filters as secondary
// constraints. Negative BYYEARDAY counts back from year-end.
// BYYEARDAY entries that resolve out-of-bounds for the year
// (e.g. day 366 in a non-leap year) are silently dropped.
func expandYearlyByYearDay(rule Rule, year int, loc *time.Location, dtstart time.Time) []time.Time {
	doys := daysInYear(year)
	out := []time.Time{}
	for _, yd := range rule.ByYearDay {
		var doy int
		if yd > 0 {
			doy = yd
		} else {
			doy = doys + yd + 1
		}
		if doy < 1 || doy > doys {
			continue
		}
		day := dayOfYearToDate(year, doy, loc)
		if !matchesByMonth(rule, day) {
			continue
		}
		if !matchesByDay(rule, day) {
			continue
		}
		out = append(out, crossTimeOfDay(rule, day, dtstart)...)
	}
	return out
}

// daysInYear returns 365 or 366 for the given Gregorian year.
func daysInYear(year int) int {
	// Day 0 of (year+1, January) normalizes back to (year, December
	// 31), whose YearDay is the year length.
	return time.Date(year+1, time.January, 0, 0, 0, 0, 0, time.UTC).YearDay()
}

// dayOfYearToDate converts a 1-based day-of-year into a time.Time
// at midnight in the given location. doy must be in 1..daysInYear.
func dayOfYearToDate(year, doy int, loc *time.Location) time.Time {
	return time.Date(year, time.January, doy, 0, 0, 0, 0, loc)
}

// expandYearlyByWeekNo produces occurrences for each BYWEEKNO
// entry, expanding to all 7 days of the named week (anchored at
// WKST). Negative entries count weeks from year-end. Entries that
// reference a week the year does not have (e.g. week 53 in a
// 52-week year) are silently dropped. BYDAY and BYMONTH filters
// narrow the expanded set.
func expandYearlyByWeekNo(rule Rule, year int, loc *time.Location, dtstart time.Time) []time.Time {
	totalWeeks := weeksInYear(year, rule.WeekStart)
	out := []time.Time{}
	for _, wn := range rule.ByWeekNo {
		var n int
		if wn > 0 {
			n = wn
		} else {
			n = totalWeeks + wn + 1
		}
		if n < 1 || n > totalWeeks {
			continue
		}
		weekStart := startOfWeekN(year, n, rule.WeekStart, loc)
		for i := range 7 {
			day := weekStart.AddDate(0, 0, i)
			if !matchesByMonth(rule, day) {
				continue
			}
			if !matchesByDay(rule, day) {
				continue
			}
			out = append(out, crossTimeOfDay(rule, day, dtstart)...)
		}
	}
	return out
}

// startOfWeekN returns the WKST-anchored start date of week N of
// the given year. Week 1 is the WKST-anchored week containing the
// first Thursday of the year (equivalently, containing Jan 4) —
// the ISO 8601 rule generalized to arbitrary WKST.
func startOfWeekN(year, n int, wkst Weekday, loc *time.Location) time.Time {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, loc)
	week1Start := startOfWeek(jan4, wkst)
	return week1Start.AddDate(0, 0, 7*(n-1))
}

// weeksInYear returns the number of WKST-anchored weeks the year
// contains under the ISO-8601-style "week 1 contains Jan 4" rule.
// Most years have 52; a 53-week year occurs when the last few days
// of December still belong to a week that contains a January 4 of
// the same year (i.e. the week containing Jan 4 of the NEXT year
// starts in the next year, leaving the trailing days of December
// in week 53 of the current year).
func weeksInYear(year int, wkst Weekday) int {
	week1Start := startOfWeek(time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC), wkst)
	nextYearWeek1Start := startOfWeek(time.Date(year+1, time.January, 4, 0, 0, 0, 0, time.UTC), wkst)
	days := int(nextYearWeek1Start.Sub(week1Start).Hours() / 24)
	return days / 7
}

// applyBySetPos filters a sorted occurrence slice down to the
// 1-based positions named in setpos. Positive entries index from
// the start; negative from the end (-1 = last). Out-of-range
// entries are silently dropped per RFC 5545 §3.3.10. The result
// is de-duplicated and re-sorted (positions may resolve to the
// same occurrence; entries may be given out of order).
func applyBySetPos(occs []time.Time, setpos []int) []time.Time {
	if len(occs) == 0 {
		return occs
	}
	picked := make(map[int]struct{}, len(setpos))
	for _, p := range setpos {
		var idx int
		if p > 0 {
			idx = p - 1
		} else {
			idx = len(occs) + p
		}
		if idx < 0 || idx >= len(occs) {
			continue
		}
		picked[idx] = struct{}{}
	}
	if len(picked) == 0 {
		return nil
	}
	out := make([]time.Time, 0, len(picked))
	for i := range occs {
		if _, ok := picked[i]; ok {
			out = append(out, occs[i])
		}
	}
	return out
}

// crossTimeOfDay produces the cartesian product of BYHOUR ×
// BYMINUTE × BYSECOND on `base`'s date. When a BY* time field is
// absent, the corresponding component is taken from dtstart.
func crossTimeOfDay(rule Rule, base, dtstart time.Time) []time.Time {
	hours := byOrDefault(rule.ByHour, dtstart.Hour())
	minutes := byOrDefault(rule.ByMinute, dtstart.Minute())
	seconds := byOrDefault(rule.BySecond, dtstart.Second())
	out := make([]time.Time, 0, len(hours)*len(minutes)*len(seconds))
	for _, h := range hours {
		for _, mn := range minutes {
			for _, sc := range seconds {
				out = append(out, time.Date(base.Year(), base.Month(), base.Day(), h, mn, sc, dtstart.Nanosecond(), base.Location()))
			}
		}
	}
	return out
}

// byOrDefault returns the filter list when non-empty, else a
// single-element slice with the default.
func byOrDefault(list []int, def int) []int {
	if len(list) == 0 {
		return []int{def}
	}
	out := make([]int, len(list))
	copy(out, list)
	return out
}

// matchesByMonth reports whether t.Month is in rule.ByMonth (or
// returns true when ByMonth is unset).
func matchesByMonth(rule Rule, t time.Time) bool {
	if len(rule.ByMonth) == 0 {
		return true
	}
	return slices.Contains(rule.ByMonth, int(t.Month()))
}

// matchesByMonthDay reports whether t.Day is in rule.ByMonthDay
// (resolving negative entries against the actual month length).
// Returns true when ByMonthDay is unset.
func matchesByMonthDay(rule Rule, t time.Time) bool {
	if len(rule.ByMonthDay) == 0 {
		return true
	}
	year, month, day := t.Date()
	dim := daysInMonth(year, month)
	for _, md := range rule.ByMonthDay {
		if md > 0 && md == day {
			return true
		}
		if md < 0 && (dim+md+1) == day {
			return true
		}
	}
	return false
}

// matchesByDay reports whether t.Weekday is in rule.ByDay
// (ignoring ordinals — those only apply in MONTHLY/YEARLY
// contexts; matchesByDayMonthly handles those).
func matchesByDay(rule Rule, t time.Time) bool {
	if len(rule.ByDay) == 0 {
		return true
	}
	wd := timeWeekdayToRRULE(t.Weekday())
	for _, bd := range rule.ByDay {
		if bd.Weekday == wd {
			return true
		}
	}
	return false
}

// matchesByDayMonthly is the BYDAY check for MONTHLY/YEARLY
// contexts where ordinals (e.g. 2MO = "second Monday of the
// month", -1FR = "last Friday") are honored.
func matchesByDayMonthly(rule Rule, t time.Time, year int, month time.Month, dim int) bool {
	if len(rule.ByDay) == 0 {
		return true
	}
	wd := timeWeekdayToRRULE(t.Weekday())
	for _, bd := range rule.ByDay {
		if bd.Weekday != wd {
			continue
		}
		if bd.Ordinal == 0 {
			return true
		}
		// Compute which-Nth-weekday-of-the-month t is.
		n := nthWeekdayOfMonth(t.Day())
		if bd.Ordinal > 0 && bd.Ordinal == n {
			return true
		}
		if bd.Ordinal < 0 {
			// Count from the end: how many weekdays of this kind
			// remain in the month?
			lastDayOfWeekday := lastDayOfWeekdayInMonth(year, month, dim, wd)
			daysFromEnd := (lastDayOfWeekday-t.Day())/7 + 1
			if -daysFromEnd == bd.Ordinal {
				return true
			}
		}
	}
	return false
}

// nthWeekdayOfMonth answers "this is the Nth occurrence of its
// weekday in the month" given the day-of-month. Day 1-7 = 1st;
// 8-14 = 2nd; etc.
func nthWeekdayOfMonth(day int) int {
	return (day-1)/7 + 1
}

// lastDayOfWeekdayInMonth returns the day-of-month of the LAST
// occurrence of weekday wd in the given month.
func lastDayOfWeekdayInMonth(year int, month time.Month, dim int, wd Weekday) int {
	for d := dim; d >= 1; d-- {
		t := time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
		if timeWeekdayToRRULE(t.Weekday()) == wd {
			return d
		}
	}
	return 0
}

// monthDayCandidates returns the days-of-month to expand for the
// given month. When BYMONTHDAY is set, resolve each entry (negative
// → from end). Otherwise, when BYDAY contains ordinal entries,
// every day must be considered (the BYDAY check filters); else
// dtstart.Day() is the single candidate.
func monthDayCandidates(rule Rule, dim, dtstartDay int) []int {
	if len(rule.ByMonthDay) > 0 {
		out := make([]int, 0, len(rule.ByMonthDay))
		for _, md := range rule.ByMonthDay {
			if md > 0 {
				out = append(out, md)
			} else if md < 0 {
				out = append(out, dim+md+1)
			}
		}
		return out
	}
	if len(rule.ByDay) > 0 {
		// Walk every day of the month; the BYDAY filter narrows.
		out := make([]int, dim)
		for i := range dim {
			out[i] = i + 1
		}
		return out
	}
	// Default: the dtstart day-of-month. If dtstart.Day() exceeds
	// the month length (e.g. Jan 31 stepping to Feb), the caller
	// skips silently — RFC 5545 §3.3.10 explicit behavior.
	if dtstartDay > dim {
		return nil
	}
	return []int{dtstartDay}
}

// startOfWeek returns the start-of-week date containing t,
// anchored on the given week-start weekday.
func startOfWeek(t time.Time, wkst Weekday) time.Time {
	day := timeWeekdayToRRULE(t.Weekday())
	// Number of days back to the wkst-anchor.
	delta := int(day) - int(wkst)
	if delta < 0 {
		delta += 7
	}
	return t.AddDate(0, 0, -delta)
}

// daysInMonth returns the number of days in the given calendar
// month, accounting for leap years.
func daysInMonth(year int, month time.Month) int {
	// time.Date with day=0 of (month+1) normalizes to the last day
	// of `month`.
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// timeWeekdayToRRULE converts time.Weekday to rrule.Weekday. They
// share the SU=0..SA=6 numbering so the conversion is a cast,
// exposed as a typed helper to keep the boundary explicit.
func timeWeekdayToRRULE(wd time.Weekday) Weekday {
	return Weekday(wd)
}
