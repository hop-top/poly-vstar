// SPDX-License-Identifier: MIT

package rrule

import (
	"errors"
	"testing"
	"time"
)

// nextCase is one row of the NextOccurrence golden table.
//
//   - rule + dtstart + after define the input.
//   - want is the expected next-fire t' (zero when wantOK=false).
//   - wantOK is true when the rule has an occurrence after `after`,
//     false when terminated.
//   - wantErr (when non-nil) is the sentinel the call must wrap;
//     when set, want and wantOK are ignored.
type nextCase struct {
	name    string
	rule    Rule
	dtstart time.Time
	after   time.Time
	want    time.Time
	wantOK  bool
	wantErr error
}

func mustParseUTC(s string) time.Time {
	t, err := time.Parse("20060102T150405Z", s)
	if err != nil {
		panic("nextCase: bad fixture: " + s + ": " + err.Error())
	}
	return t
}

// TestNextOccurrence covers FREQ-only iteration, INTERVAL,
// UNTIL/COUNT termination, BY* filters, and the documented edge
// cases (leap-day BYMONTHDAY=29 in non-leap February, DST
// boundaries).
func TestNextOccurrence(t *testing.T) {
	dtAprilNoon := mustParseUTC("20260401T120000Z")
	dtJan1 := mustParseUTC("20260101T000000Z")

	cases := []nextCase{
		// ── FREQ-only ─────────────────────────────────────────────
		{
			name:    "hourly_simple",
			rule:    Rule{Freq: FreqHourly, Interval: 1, WeekStart: MO},
			dtstart: dtAprilNoon,
			after:   dtAprilNoon,
			want:    mustParseUTC("20260401T130000Z"),
			wantOK:  true,
		},
		{
			name:    "daily_simple",
			rule:    Rule{Freq: FreqDaily, Interval: 1, WeekStart: MO},
			dtstart: dtAprilNoon,
			after:   dtAprilNoon,
			want:    mustParseUTC("20260402T120000Z"),
			wantOK:  true,
		},
		{
			name:    "weekly_simple",
			rule:    Rule{Freq: FreqWeekly, Interval: 1, WeekStart: MO},
			dtstart: dtAprilNoon,
			after:   dtAprilNoon,
			want:    mustParseUTC("20260408T120000Z"),
			wantOK:  true,
		},
		{
			name:    "monthly_simple",
			rule:    Rule{Freq: FreqMonthly, Interval: 1, WeekStart: MO},
			dtstart: dtAprilNoon,
			after:   dtAprilNoon,
			want:    mustParseUTC("20260501T120000Z"),
			wantOK:  true,
		},
		{
			name:    "yearly_simple",
			rule:    Rule{Freq: FreqYearly, Interval: 1, WeekStart: MO},
			dtstart: dtAprilNoon,
			after:   dtAprilNoon,
			want:    mustParseUTC("20270401T120000Z"),
			wantOK:  true,
		},

		// ── INTERVAL ──────────────────────────────────────────────
		{
			name:    "daily_interval_2",
			rule:    Rule{Freq: FreqDaily, Interval: 2, WeekStart: MO},
			dtstart: dtAprilNoon,
			after:   dtAprilNoon,
			want:    mustParseUTC("20260403T120000Z"),
			wantOK:  true,
		},
		{
			name:    "weekly_interval_2",
			rule:    Rule{Freq: FreqWeekly, Interval: 2, WeekStart: MO},
			dtstart: dtAprilNoon,
			after:   dtAprilNoon,
			want:    mustParseUTC("20260415T120000Z"),
			wantOK:  true,
		},
		{
			name:    "monthly_interval_3",
			rule:    Rule{Freq: FreqMonthly, Interval: 3, WeekStart: MO},
			dtstart: dtAprilNoon,
			after:   dtAprilNoon,
			want:    mustParseUTC("20260701T120000Z"),
			wantOK:  true,
		},

		// ── UNTIL termination ────────────────────────────────────
		{
			name: "daily_until_terminated",
			rule: Rule{
				Freq:      FreqDaily,
				Interval:  1,
				Until:     mustParseUTC("20260403T000000Z"),
				WeekStart: MO,
			},
			dtstart: dtAprilNoon,
			after:   mustParseUTC("20260402T120000Z"),
			wantOK:  false, // next would be 20260403T120000Z, past UNTIL midnight
		},
		{
			name: "daily_until_still_firing",
			rule: Rule{
				Freq:      FreqDaily,
				Interval:  1,
				Until:     mustParseUTC("20260410T120000Z"),
				WeekStart: MO,
			},
			dtstart: dtAprilNoon,
			after:   mustParseUTC("20260408T120000Z"),
			want:    mustParseUTC("20260409T120000Z"),
			wantOK:  true,
		},

		// ── COUNT termination ────────────────────────────────────
		{
			name: "daily_count_3_after_first",
			rule: Rule{
				Freq:      FreqDaily,
				Interval:  1,
				Count:     3,
				WeekStart: MO,
			},
			dtstart: dtAprilNoon,
			after:   dtAprilNoon, // dtstart counts as occurrence #1
			want:    mustParseUTC("20260402T120000Z"),
			wantOK:  true,
		},
		{
			name: "daily_count_3_after_last",
			rule: Rule{
				Freq:      FreqDaily,
				Interval:  1,
				Count:     3,
				WeekStart: MO,
			},
			dtstart: dtAprilNoon,
			after:   mustParseUTC("20260403T120000Z"), // last occurrence
			wantOK:  false,
		},

		// ── BYMONTH ──────────────────────────────────────────────
		{
			name: "yearly_bymonth_jun_dec",
			rule: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByMonth:   []int{6, 12},
				WeekStart: MO,
			},
			dtstart: dtJan1,
			after:   dtJan1,
			want:    mustParseUTC("20260601T000000Z"),
			wantOK:  true,
		},

		// ── BYMONTHDAY ──────────────────────────────────────────
		{
			name: "monthly_bymonthday_15",
			rule: Rule{
				Freq:       FreqMonthly,
				Interval:   1,
				ByMonthDay: []int{15},
				WeekStart:  MO,
			},
			dtstart: mustParseUTC("20260115T120000Z"),
			after:   mustParseUTC("20260115T120000Z"),
			want:    mustParseUTC("20260215T120000Z"),
			wantOK:  true,
		},
		{
			name: "monthly_bymonthday_minus_1_last_day",
			rule: Rule{
				Freq:       FreqMonthly,
				Interval:   1,
				ByMonthDay: []int{-1},
				WeekStart:  MO,
			},
			dtstart: mustParseUTC("20260131T120000Z"),
			after:   mustParseUTC("20260131T120000Z"),
			want:    mustParseUTC("20260228T120000Z"), // Feb 2026 has 28 days
			wantOK:  true,
		},

		// ── BYDAY ────────────────────────────────────────────────
		{
			name: "weekly_byday_mo_we_fr",
			// April 1, 2026 = Wednesday (verify: 2026-04-01 was a
			// Wednesday). Next MO/WE/FR after that is FR = Apr 3.
			rule: Rule{
				Freq:      FreqWeekly,
				Interval:  1,
				ByDay:     []ByDay{{0, MO}, {0, WE}, {0, FR}},
				WeekStart: MO,
			},
			dtstart: mustParseUTC("20260401T120000Z"), // Wed
			after:   mustParseUTC("20260401T120000Z"),
			want:    mustParseUTC("20260403T120000Z"), // Fri
			wantOK:  true,
		},
		{
			name: "monthly_byday_first_monday",
			rule: Rule{
				Freq:      FreqMonthly,
				Interval:  1,
				ByDay:     []ByDay{{1, MO}},
				WeekStart: MO,
			},
			dtstart: mustParseUTC("20260406T120000Z"), // 1st Mon of April 2026
			after:   mustParseUTC("20260406T120000Z"),
			want:    mustParseUTC("20260504T120000Z"), // 1st Mon of May 2026
			wantOK:  true,
		},
		{
			name: "monthly_byday_last_friday",
			rule: Rule{
				Freq:      FreqMonthly,
				Interval:  1,
				ByDay:     []ByDay{{-1, FR}},
				WeekStart: MO,
			},
			dtstart: mustParseUTC("20260424T120000Z"), // last Fri of Apr 2026
			after:   mustParseUTC("20260424T120000Z"),
			want:    mustParseUTC("20260529T120000Z"), // last Fri of May 2026
			wantOK:  true,
		},

		// ── BYHOUR / BYMINUTE / BYSECOND ────────────────────────
		{
			name: "daily_byhour_9_17",
			rule: Rule{
				Freq:      FreqDaily,
				Interval:  1,
				ByHour:    []int{9, 17},
				WeekStart: MO,
			},
			dtstart: mustParseUTC("20260401T090000Z"),
			after:   mustParseUTC("20260401T090000Z"),
			want:    mustParseUTC("20260401T170000Z"),
			wantOK:  true,
		},

		// ── Leap-day edge case ──────────────────────────────────
		{
			name: "monthly_bymonthday_29_skips_feb_non_leap",
			// 2026 is NOT a leap year. BYMONTHDAY=29 should skip
			// February and land on March 29.
			rule: Rule{
				Freq:       FreqMonthly,
				Interval:   1,
				ByMonthDay: []int{29},
				WeekStart:  MO,
			},
			dtstart: mustParseUTC("20260129T120000Z"),
			after:   mustParseUTC("20260129T120000Z"),
			want:    mustParseUTC("20260329T120000Z"),
			wantOK:  true,
		},

		// ── DST boundary ────────────────────────────────────────
		// Spring-forward in America/New_York: 2026-03-08, 02:30
		// local does not exist. Daily rule from Mar 7 02:30 EST
		// (07:30 UTC) → Mar 8 03:00 EDT (07:00 UTC) — Go's
		// time.Add normalizes through the gap. We verify that the
		// next occurrence is one calendar day later in local terms,
		// which lands at the same UTC offset minus one hour after
		// spring-forward.
		dstSpringForwardCase(),

		// ── INTERVAL with COUNT terminates correctly ───────────
		{
			name: "weekly_interval_2_count_3_terminates",
			rule: Rule{
				Freq:      FreqWeekly,
				Interval:  2,
				Count:     3,
				WeekStart: MO,
			},
			dtstart: mustParseUTC("20260101T000000Z"),
			after:   mustParseUTC("20260129T000000Z"), // 3rd occurrence
			wantOK:  false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok, err := NextOccurrence(c.rule, c.dtstart, c.after)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("err: want errors.Is(%v), got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if ok != c.wantOK {
				t.Fatalf("ok: want %v, got %v (got time %v)", c.wantOK, ok, got)
			}
			if !c.wantOK {
				if !got.IsZero() {
					t.Fatalf("expected zero time when ok=false, got %v", got)
				}
				return
			}
			if !got.Equal(c.want) {
				t.Fatalf("time: want %v, got %v", c.want, got)
			}
		})
	}
}

// dstSpringForwardCase exercises a DAILY recurrence across the
// US/Eastern spring-forward boundary. dtstart is in
// America/New_York; the evaluator must respect the zone
// (time.Time arithmetic does this natively).
func dstSpringForwardCase() nextCase {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic("dstSpringForwardCase: tzdata missing: " + err.Error())
	}
	dt := time.Date(2026, 3, 7, 14, 0, 0, 0, loc) // Sat 14:00 EST
	return nextCase{
		name:    "daily_dst_spring_forward",
		rule:    Rule{Freq: FreqDaily, Interval: 1, WeekStart: MO},
		dtstart: dt,
		after:   dt,
		// Sun 14:00 EDT (after clocks jumped forward at 02:00 →
		// 03:00). Same wall-clock 14:00, new offset.
		want:   time.Date(2026, 3, 8, 14, 0, 0, 0, loc),
		wantOK: true,
	}
}

// TestNextOccurrence_TerminatesOnPathological asserts an exhausted
// COUNT reports clean termination — (zero, false, nil) with no
// error.
//
// Despite the name this exercises normal completion, not the
// safety limit: a genuine cap hit is covered by
// TestNextOccurrence_IterationCapIsDistinguishable, which reaches
// it with the unsatisfiable FREQ=MONTHLY;BYMONTH=2;BYMONTHDAY=30
// (February has no 30th) and expects ErrIterationCap. Kept as the
// regression guard that termination is never misreported as a
// cap hit.
func TestNextOccurrence_TerminatesOnPathological(t *testing.T) {
	// COUNT=1 with `after` past dtstart: the single occurrence is
	// consumed, so the rule has legitimately terminated.
	r := Rule{Freq: FreqDaily, Interval: 1, Count: 1, WeekStart: MO}
	dt := mustParseUTC("20260401T120000Z")
	got, ok, err := NextOccurrence(r, dt, mustParseUTC("20260401T120001Z"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Fatalf("want terminated, got %v", got)
	}
}

// TestNextOccurrence_GuardRails covers the input-validation guards
// that ParseRRule would normally catch but a hand-built Rule
// literal can bypass.
func TestNextOccurrence_GuardRails(t *testing.T) {
	dt := mustParseUTC("20260401T120000Z")
	t.Run("freq_invalid", func(t *testing.T) {
		_, _, err := NextOccurrence(Rule{}, dt, dt)
		if !errors.Is(err, ErrUnsupportedRRule) {
			t.Fatalf("want ErrUnsupportedRRule, got %v", err)
		}
	})
	t.Run("interval_zero", func(t *testing.T) {
		_, _, err := NextOccurrence(Rule{Freq: FreqDaily, Interval: 0}, dt, dt)
		if !errors.Is(err, ErrUnsupportedRRule) {
			t.Fatalf("want ErrUnsupportedRRule, got %v", err)
		}
	})
}

// TestNextOccurrence_ByFiltersExclude exercises the rejection
// branches of the BY* matchers: BYMONTH excluding, BYMONTHDAY
// negative-resolution skipping, BYDAY missing the weekday.
func TestNextOccurrence_ByFiltersExclude(t *testing.T) {
	// YEARLY with BYMONTH=12: starting in Jan should land on Dec 1.
	r := Rule{
		Freq:      FreqYearly,
		Interval:  1,
		ByMonth:   []int{12},
		WeekStart: MO,
	}
	dt := mustParseUTC("20260101T000000Z")
	got, ok, err := NextOccurrence(r, dt, dt)
	if err != nil || !ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
	want := mustParseUTC("20261201T000000Z")
	if !got.Equal(want) {
		t.Fatalf("BYMONTH=12: want %v, got %v", want, got)
	}

	// MONTHLY with BYMONTHDAY=-2 (penultimate day): January 2026
	// has 31 days → day 30. After dtstart on Jan 30, next is Feb
	// 27 (Feb has 28 days, -2 = day 27).
	r2 := Rule{
		Freq:       FreqMonthly,
		Interval:   1,
		ByMonthDay: []int{-2},
		WeekStart:  MO,
	}
	dt2 := mustParseUTC("20260130T000000Z")
	got2, ok, err := NextOccurrence(r2, dt2, dt2)
	if err != nil || !ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
	want2 := mustParseUTC("20260227T000000Z")
	if !got2.Equal(want2) {
		t.Fatalf("BYMONTHDAY=-2: want %v, got %v", want2, got2)
	}
}

// TestWeekday_ToTime exercises the ToTime conversion across all
// weekdays.
func TestWeekday_ToTime(t *testing.T) {
	cases := []struct {
		w    Weekday
		want time.Weekday
	}{
		{SU, time.Sunday},
		{MO, time.Monday},
		{SA, time.Saturday},
	}
	for _, c := range cases {
		if got := c.w.ToTime(); got != c.want {
			t.Errorf("%v.ToTime() = %v, want %v", c.w, got, c.want)
		}
	}
}

// TestNextOccurrence_HourlyWithFilters drives the hourly path
// through every BY* branch.
func TestNextOccurrence_HourlyWithFilters(t *testing.T) {
	dt := mustParseUTC("20260401T100000Z")
	r := Rule{
		Freq:      FreqHourly,
		Interval:  1,
		ByMinute:  []int{15, 45},
		BySecond:  []int{30},
		WeekStart: MO,
	}
	got, ok, err := NextOccurrence(r, dt, dt)
	if err != nil || !ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
	want := mustParseUTC("20260401T101530Z")
	if !got.Equal(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

// TestNextOccurrence_HourlyByDayFilter exercises the BYDAY
// rejection branch in expandHourly (rule is constructed manually
// — ParseRRule wouldn't allow this combo logically but the
// matcher should still skip non-matching days).
func TestNextOccurrence_HourlyByDayFilter(t *testing.T) {
	dt := mustParseUTC("20260401T100000Z") // Wednesday
	// BYDAY=FR — should advance until Friday.
	r := Rule{
		Freq:      FreqHourly,
		Interval:  1,
		ByDay:     []ByDay{{0, FR}},
		WeekStart: MO,
	}
	got, ok, err := NextOccurrence(r, dt, dt)
	if err != nil || !ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
	// Friday Apr 3, 2026 at 00:00 (first hour after dtstart Wed
	// 10am where BYDAY matches and minute/second taken from dt).
	if got.Weekday() != time.Friday {
		t.Fatalf("expected Friday, got %v at %v", got.Weekday(), got)
	}
}

// TestNextOccurrence_DailyBYMONTHReject covers the matchesByMonth
// rejection branch within expandDaily.
func TestNextOccurrence_DailyBYMONTHReject(t *testing.T) {
	dt := mustParseUTC("20260101T120000Z") // January
	// BYMONTH=6 — daily should yield first day of June.
	r := Rule{
		Freq:      FreqDaily,
		Interval:  1,
		ByMonth:   []int{6},
		WeekStart: MO,
	}
	got, ok, err := NextOccurrence(r, dt, dt)
	if err != nil || !ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
	if got.Month() != time.June {
		t.Fatalf("expected June, got %v", got)
	}
}

// TestNextOccurrence_MonthlyJan31SkipsFeb covers the
// dtstartDay > daysInMonth branch in monthDayCandidates.
func TestNextOccurrence_MonthlyJan31SkipsFeb(t *testing.T) {
	dt := mustParseUTC("20260131T120000Z") // Jan 31
	r := Rule{Freq: FreqMonthly, Interval: 1, WeekStart: MO}
	got, ok, err := NextOccurrence(r, dt, dt)
	if err != nil || !ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
	// Should skip Feb (no Feb 31) and land on Mar 31.
	if got.Day() != 31 || got.Month() != time.March {
		t.Fatalf("want Mar 31, got %v", got)
	}
}

// TestParseRRule_ExtraEdgeCases nudges parser coverage on rare
// branches.
func TestParseRRule_ExtraEdgeCases(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"byday_empty_list", "FREQ=WEEKLY;BYDAY="},
		{"byday_too_short", "FREQ=WEEKLY;BYDAY=M"},
		{"byday_ordinal_too_large", "FREQ=MONTHLY;BYDAY=54MO"},
		{"byday_ordinal_too_small", "FREQ=MONTHLY;BYDAY=-54MO"},
		{"byday_ordinal_non_int", "FREQ=MONTHLY;BYDAY=ABMO"},
		{"bymonthday_non_int", "FREQ=MONTHLY;BYMONTHDAY=abc"},
		{"bymonthday_empty", "FREQ=MONTHLY;BYMONTHDAY="},
		{"bymonth_non_int", "FREQ=YEARLY;BYMONTH=abc"},
		{"count_non_int", "FREQ=DAILY;COUNT=abc"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := ParseRRule(c.in); err == nil {
				t.Fatalf("ParseRRule(%q): want error, got nil", c.in)
			}
		})
	}
}

// TestNextOccurrence_YearlyByYearDay covers BYYEARDAY expansion
// under FREQ=YEARLY: positive day-of-year, negative (counted from
// year-end), the leap-year boundary at day 366, and a list with
// multiple values.
func TestNextOccurrence_YearlyByYearDay(t *testing.T) {
	cases := []struct {
		name    string
		rule    Rule
		dtstart time.Time
		after   time.Time
		want    time.Time
	}{
		{
			name: "yearly_byyearday_100",
			rule: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByYearDay: []int{100},
				WeekStart: MO,
			},
			// Day 100 of 2026 is April 10 (2026 not leap).
			dtstart: mustParseUTC("20260410T000000Z"),
			after:   mustParseUTC("20260410T000000Z"),
			want:    mustParseUTC("20270410T000000Z"),
		},
		{
			name: "yearly_byyearday_minus_1_last_day",
			rule: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByYearDay: []int{-1},
				WeekStart: MO,
			},
			dtstart: mustParseUTC("20261231T000000Z"),
			after:   mustParseUTC("20261231T000000Z"),
			want:    mustParseUTC("20271231T000000Z"),
		},
		{
			name: "yearly_byyearday_366_skips_non_leap",
			rule: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByYearDay: []int{366},
				WeekStart: MO,
			},
			// Start in 2024 (leap year). Day 366 = Dec 31 2024.
			// 2025, 2026, 2027 are non-leap → no day 366. Next fire
			// is Dec 31 2028 (leap).
			dtstart: mustParseUTC("20241231T000000Z"),
			after:   mustParseUTC("20241231T000000Z"),
			want:    mustParseUTC("20281231T000000Z"),
		},
		{
			name: "yearly_byyearday_list_first_after",
			rule: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByYearDay: []int{1, 100, -1},
				WeekStart: MO,
			},
			// dtstart Jan 1 2026 → first occurrence Jan 1 2026 (day 1).
			// Next after is day 100 = Apr 10.
			dtstart: mustParseUTC("20260101T000000Z"),
			after:   mustParseUTC("20260101T000000Z"),
			want:    mustParseUTC("20260410T000000Z"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok, err := NextOccurrence(c.rule, c.dtstart, c.after)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !ok {
				t.Fatalf("want ok, got terminated")
			}
			if !got.Equal(c.want) {
				t.Fatalf("want %v, got %v", c.want, got)
			}
		})
	}
}

// TestNextOccurrence_YearlyByWeekNo covers BYWEEKNO expansion
// under FREQ=YEARLY: ISO 8601 week numbering with WKST=MO
// (default). Week 1 of any year contains the first Thursday
// (equivalently, contains Jan 4). Negative values count from
// year-end. Years with 53 ISO weeks (e.g. 2026 — 53 weeks
// because Jan 1 2026 is Thursday) honor week 53.
func TestNextOccurrence_YearlyByWeekNo(t *testing.T) {
	cases := []struct {
		name    string
		rule    Rule
		dtstart time.Time
		after   time.Time
		want    time.Time
	}{
		{
			name: "yearly_byweekno_1",
			rule: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByWeekNo:  []int{1},
				ByDay:     []ByDay{{0, MO}},
				WeekStart: MO,
			},
			// ISO week 1 of 2026 starts Mon Dec 29 2025 (week
			// containing Jan 4 2026). dtstart on that Monday.
			dtstart: mustParseUTC("20251229T000000Z"),
			after:   mustParseUTC("20251229T000000Z"),
			// Next ISO week 1 + Monday: Mon Jan 4 2027 (week 1 of
			// 2027 begins Jan 4 since Jan 1 2027 = Friday).
			want: mustParseUTC("20270104T000000Z"),
		},
		{
			name: "yearly_byweekno_minus_1_last_week",
			rule: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByWeekNo:  []int{-1},
				ByDay:     []ByDay{{0, MO}},
				WeekStart: MO,
			},
			// ISO week 53 of 2026 starts Mon Dec 28 2026.
			dtstart: mustParseUTC("20261228T000000Z"),
			after:   mustParseUTC("20261228T000000Z"),
			// Next "last week + Monday": ISO week 52 of 2027 begins
			// Mon Dec 27 2027 (2027 has 52 weeks).
			want: mustParseUTC("20271227T000000Z"),
		},
		{
			name: "yearly_byweekno_53_skips_short_year",
			rule: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByWeekNo:  []int{53},
				ByDay:     []ByDay{{0, MO}},
				WeekStart: MO,
			},
			// 2026 has 53 ISO weeks (Jan 1 2026 = Thursday). 2027,
			// 2028, 2029 have 52. 2030, 2031 have 52. 2032 has 53
			// (Jan 1 2032 = Thursday — leap year starting Thursday).
			dtstart: mustParseUTC("20261228T000000Z"),
			after:   mustParseUTC("20261228T000000Z"),
			// Next year with 53 weeks: 2032. Week 53 begins Mon
			// Dec 27 2032.
			want: mustParseUTC("20321227T000000Z"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok, err := NextOccurrence(c.rule, c.dtstart, c.after)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !ok {
				t.Fatalf("want ok, got terminated")
			}
			if !got.Equal(c.want) {
				t.Fatalf("want %v, got %v", c.want, got)
			}
		})
	}
}

// TestNextOccurrence_YearlyByWeekNo_WKSTSunday verifies WKST
// affects week-boundary semantics for BYWEEKNO. With WKST=SU,
// the week starts Sunday (per RFC 5545 §3.3.10).
//
// Year 2023 is the discriminating case: Jan 1 2023 is a Sunday.
//   - WKST=MO: week 1 of 2023 starts Mon Jan 2 (first MO-anchored
//     week containing 4+ days of 2023). The Sunday in week 1 is
//     Jan 8.
//   - WKST=SU: week 1 of 2023 starts Sun Jan 1 (the SU-anchored
//     week containing 4+ days of 2023). The Sunday in week 1 is
//     Jan 1.
//
// We exercise the WKST=SU branch by asking for "first occurrence
// after dtstart=Jan 1 2023" — under WKST=SU that returns the
// Sunday of week 1 in 2024, which (since Jan 1 2024 is Mon) is
// Dec 31 2023 (the Sunday in the SU-anchored week containing Jan
// 4 2024).
func TestNextOccurrence_YearlyByWeekNo_WKSTSunday(t *testing.T) {
	r := Rule{
		Freq:      FreqYearly,
		Interval:  1,
		ByWeekNo:  []int{1},
		ByDay:     []ByDay{{0, SU}},
		WeekStart: SU,
	}
	dt := mustParseUTC("20230101T000000Z") // Sun Jan 1 2023
	got, ok, err := NextOccurrence(r, dt, dt)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !ok {
		t.Fatalf("want ok, got terminated")
	}
	// Week 1 of 2024 (WKST=SU): Jan 4 2024 = Thursday → SU-anchored
	// week containing Jan 4 starts Sun Dec 31 2023.
	want := mustParseUTC("20231231T000000Z")
	if !got.Equal(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

// TestNextOccurrence_BySetPos covers BYSETPOS positional filtering
// applied AFTER all other BY-* expansion within the FREQ period
// per RFC 5545 §3.3.10. 1-based; negative counts from end. Out-of-
// range entries silently dropped.
func TestNextOccurrence_BySetPos(t *testing.T) {
	cases := []struct {
		name    string
		rule    Rule
		dtstart time.Time
		after   time.Time
		want    time.Time
	}{
		{
			// "Last weekday of month": BYDAY=MO,TU,WE,TH,FR yields
			// every weekday; BYSETPOS=-1 picks the last one.
			// Apr 2026: Apr 30 = Thursday → expected.
			name: "monthly_last_weekday",
			rule: Rule{
				Freq:      FreqMonthly,
				Interval:  1,
				ByDay:     []ByDay{{0, MO}, {0, TU}, {0, WE}, {0, TH}, {0, FR}},
				BySetPos:  []int{-1},
				WeekStart: MO,
			},
			dtstart: mustParseUTC("20260401T000000Z"),
			after:   mustParseUTC("20260401T000000Z"),
			want:    mustParseUTC("20260430T000000Z"),
		},
		{
			// "Second Tuesday of month": BYDAY=TU yields every TU;
			// BYSETPOS=2 picks the 2nd. Apr 2026 Tuesdays: 7, 14,
			// 21, 28 → 14.
			name: "monthly_second_tuesday",
			rule: Rule{
				Freq:      FreqMonthly,
				Interval:  1,
				ByDay:     []ByDay{{0, TU}},
				BySetPos:  []int{2},
				WeekStart: MO,
			},
			dtstart: mustParseUTC("20260401T000000Z"),
			after:   mustParseUTC("20260401T000000Z"),
			want:    mustParseUTC("20260414T000000Z"),
		},
		{
			// BYSETPOS list with mixed signs: 1 + -1 picks first
			// and last. After Apr 1 → first weekday Apr 1 itself
			// is Wed (a weekday) — but `after` is Apr 1 inclusive,
			// so first match strictly after = Apr 30 (the -1 in
			// April) — wait: occurrences in April are {Apr 1, Apr
			// 30}; Apr 1 == after (not strictly after); next is
			// Apr 30. Then May's first/last. So with after=Apr 1,
			// next is Apr 30.
			name: "monthly_first_and_last_weekday",
			rule: Rule{
				Freq:      FreqMonthly,
				Interval:  1,
				ByDay:     []ByDay{{0, MO}, {0, TU}, {0, WE}, {0, TH}, {0, FR}},
				BySetPos:  []int{1, -1},
				WeekStart: MO,
			},
			dtstart: mustParseUTC("20260401T000000Z"),
			after:   mustParseUTC("20260401T000000Z"),
			want:    mustParseUTC("20260430T000000Z"),
		},
		{
			// Out-of-range BYSETPOS entry silently dropped:
			// Apr 2026 has 22 weekdays; BYSETPOS=99 produces no
			// occurrence in April; we fall through to May (also
			// 21 weekdays — May 1 is Fri). Combined with
			// BYSETPOS=1 → first weekday of May = May 1.
			name: "monthly_out_of_range_dropped",
			rule: Rule{
				Freq:      FreqMonthly,
				Interval:  1,
				ByDay:     []ByDay{{0, MO}, {0, TU}, {0, WE}, {0, TH}, {0, FR}},
				BySetPos:  []int{99, 1},
				WeekStart: MO,
			},
			dtstart: mustParseUTC("20260401T000000Z"),
			after:   mustParseUTC("20260401T120000Z"),
			want:    mustParseUTC("20260501T000000Z"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok, err := NextOccurrence(c.rule, c.dtstart, c.after)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !ok {
				t.Fatalf("want ok, got terminated")
			}
			if !got.Equal(c.want) {
				t.Fatalf("want %v, got %v", c.want, got)
			}
		})
	}
}

// TestNextOccurrence_YearlyFromFeb29_SkipsNonLeap covers the
// RFC 5545 §3.3.10 "non-existent dates simply do not occur"
// rule for FREQ=YEARLY anchored on Feb 29. Go's time.AddDate
// would normalize 2024-02-29 + 1y to 2025-03-01; the evaluator
// must instead skip non-leap years and emit the next true Feb 29
// (2028, 2032, 2036).
func TestNextOccurrence_YearlyFromFeb29_SkipsNonLeap(t *testing.T) {
	dt := mustParseUTC("20240229T000000Z")
	r := Rule{Freq: FreqYearly, Interval: 1, WeekStart: MO}

	want := []time.Time{
		mustParseUTC("20280229T000000Z"),
		mustParseUTC("20320229T000000Z"),
		mustParseUTC("20360229T000000Z"),
	}

	after := dt
	for i, w := range want {
		got, ok, err := NextOccurrence(r, dt, after)
		if err != nil {
			t.Fatalf("iter %d: err=%v", i, err)
		}
		if !ok {
			t.Fatalf("iter %d: want ok, got terminated", i)
		}
		if !got.Equal(w) {
			t.Fatalf("iter %d: want %v, got %v", i, w, got)
		}
		after = got
	}
}

// TestNextOccurrence_WeeklyByDay_AcrossWeekBoundary covers
// expandWeekly with BYDAY when the next occurrence is in the
// following week (exercises startOfWeek + advance).
func TestNextOccurrence_WeeklyByDay_AcrossWeekBoundary(t *testing.T) {
	r := Rule{
		Freq:      FreqWeekly,
		Interval:  1,
		ByDay:     []ByDay{{0, MO}},
		WeekStart: MO,
	}
	// Start on a Friday — next Monday is in next week.
	dt := mustParseUTC("20260403T120000Z") // Fri Apr 3
	// Find first MO >= Apr 3: Apr 6.
	got, ok, err := NextOccurrence(r, dt, mustParseUTC("20260403T000000Z"))
	if err != nil || !ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
	want := mustParseUTC("20260406T120000Z")
	if !got.Equal(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

// TestNextOccurrence_IterationCapIsDistinguishable is the core
// guard for the two-outcomes-one-signal defect: a rule that has
// legitimately finished and a rule the evaluator gave up searching
// for must not look alike to the caller. Both return
// (zero, false, ...); only the error slot tells them apart.
//
// The cap-hit input is FREQ=MONTHLY;BYMONTH=2;BYMONTHDAY=30 —
// syntactically valid, accepted by ParseRRule, and unsatisfiable
// for all time because February never has a 30th day. The
// evaluator therefore steps through every one of its budgeted
// FREQ periods without ever producing a candidate.
func TestNextOccurrence_IterationCapIsDistinguishable(t *testing.T) {
	dt := mustParseUTC("20260101T090000Z")

	t.Run("cap_hit_reports_ErrIterationCap", func(t *testing.T) {
		r, err := ParseRRule("FREQ=MONTHLY;BYMONTH=2;BYMONTHDAY=30")
		if err != nil {
			t.Fatalf("ParseRRule: unexpected error: %v", err)
		}
		got, ok, err := NextOccurrence(r, dt, dt)
		if ok {
			t.Fatalf("want no occurrence, got %v", got)
		}
		if !got.IsZero() {
			t.Fatalf("want zero time alongside ok=false, got %v", got)
		}
		if !errors.Is(err, ErrIterationCap) {
			t.Fatalf("want ErrIterationCap, got %v", err)
		}
	})

	t.Run("until_termination_reports_no_error", func(t *testing.T) {
		r, err := ParseRRule("FREQ=DAILY;UNTIL=20260103T090000Z")
		if err != nil {
			t.Fatalf("ParseRRule: unexpected error: %v", err)
		}
		got, ok, err := NextOccurrence(r, dt, mustParseUTC("20260103T090000Z"))
		if err != nil {
			t.Fatalf("normal UNTIL termination must not report an error, got %v", err)
		}
		if errors.Is(err, ErrIterationCap) {
			t.Fatalf("UNTIL termination misreported as a cap hit")
		}
		if ok {
			t.Fatalf("want terminated, got %v", got)
		}
	})

	t.Run("count_termination_reports_no_error", func(t *testing.T) {
		r, err := ParseRRule("FREQ=DAILY;COUNT=3")
		if err != nil {
			t.Fatalf("ParseRRule: unexpected error: %v", err)
		}
		got, ok, err := NextOccurrence(r, dt, mustParseUTC("20260103T090000Z"))
		if err != nil {
			t.Fatalf("normal COUNT termination must not report an error, got %v", err)
		}
		if errors.Is(err, ErrIterationCap) {
			t.Fatalf("COUNT termination misreported as a cap hit")
		}
		if ok {
			t.Fatalf("want terminated, got %v", got)
		}
	})

	t.Run("cap_and_termination_are_not_equal", func(t *testing.T) {
		capped, err := ParseRRule("FREQ=MONTHLY;BYMONTH=2;BYMONTHDAY=30")
		if err != nil {
			t.Fatalf("ParseRRule: unexpected error: %v", err)
		}
		done, err := ParseRRule("FREQ=DAILY;COUNT=3")
		if err != nil {
			t.Fatalf("ParseRRule: unexpected error: %v", err)
		}
		_, _, capErr := NextOccurrence(capped, dt, dt)
		_, _, doneErr := NextOccurrence(done, dt, mustParseUTC("20260103T090000Z"))
		if errors.Is(capErr, ErrIterationCap) == errors.Is(doneErr, ErrIterationCap) {
			t.Fatalf("cap hit and normal termination are indistinguishable: capErr=%v doneErr=%v", capErr, doneErr)
		}
	})
}

// TestMaxIterations_IsPublished asserts the iteration budget is a
// documented part of the API surface rather than a private magic
// number, so a caller reading ErrIterationCap can reason about
// what bound was actually hit.
func TestMaxIterations_IsPublished(t *testing.T) {
	if MaxIterations <= 0 {
		t.Fatalf("MaxIterations must be positive, got %d", MaxIterations)
	}
	if MaxIterations != 100000 {
		t.Fatalf("MaxIterations changed unexpectedly: got %d, want 100000", MaxIterations)
	}
}

// TestErrIterationCap_WrapsNothingElse pins ErrIterationCap as a
// distinct sentinel: it must not be confused with the pre-existing
// ErrUnsupportedRRule, which reports an input the evaluator refuses
// rather than a search it abandoned.
func TestErrIterationCap_WrapsNothingElse(t *testing.T) {
	if errors.Is(ErrIterationCap, ErrUnsupportedRRule) {
		t.Fatal("ErrIterationCap must not satisfy errors.Is(ErrUnsupportedRRule)")
	}
	if errors.Is(ErrUnsupportedRRule, ErrIterationCap) {
		t.Fatal("ErrUnsupportedRRule must not satisfy errors.Is(ErrIterationCap)")
	}
}

// nextN walks NextOccurrence n times from dtstart (after = dtstart
// first, then each result) and renders the sequence in wire form.
// A termination or error before n results is a fixture bug.
func nextN(t *testing.T, rule Rule, dtstart time.Time, n int) []string {
	t.Helper()
	out := make([]time.Time, 0, n)
	after := dtstart
	for range n {
		got, ok, err := NextOccurrence(rule, dtstart, after)
		if err != nil || !ok {
			t.Fatalf("NextOccurrence after %v: ok=%v err=%v (have %v)", after, ok, err, utcs(out))
		}
		out = append(out, got)
		after = got
	}
	return utcs(out)
}

// TestNextOccurrence_HourlyByHourLimits pins BYHOUR as a LIMIT under
// FREQ=HOURLY per the RFC 5545 §3.3.10 table (BYHOUR is "Limit" in
// the HOURLY column). Each hourly base whose hour is not in BYHOUR
// yields nothing; BYMINUTE/BYSECOND still expand within a base that
// passes.
func TestNextOccurrence_HourlyByHourLimits(t *testing.T) {
	// 2026-04-01 is a Wednesday; dtstart 08:00Z.
	dt := mustParseUTC("20260401T080000Z")

	t.Run("byhour_limit", func(t *testing.T) {
		// FREQ=HOURLY;BYHOUR=9,17 from 08:00:
		//   base 08:00 — hour 8 not in {9,17} → nothing (dtstart is
		//   not occurrence #1);
		//   base 09:00 → 09:00:00 (minute/second from the base, 0/0);
		//   bases 10:00..16:00 → nothing;
		//   base 17:00 → 17:00:00;
		//   bases 18:00..08:00 next day → nothing;
		//   2026-04-02 09:00 and 17:00 follow.
		r := mustParse(t, "FREQ=HOURLY;BYHOUR=9,17")
		want := []string{
			"20260401T090000Z",
			"20260401T170000Z",
			"20260402T090000Z",
			"20260402T170000Z",
		}
		if got := nextN(t, r, dt, 4); !eqStrs(got, want) {
			t.Fatalf("sequence = %v, want %v", got, want)
		}
	})

	t.Run("byhour_limit_byminute_expand", func(t *testing.T) {
		// FREQ=HOURLY;BYHOUR=9;BYMINUTE=0,30 from 08:00:
		//   base 08:00 — hour 8 ≠ 9 → nothing (an evaluator that
		//   ignores BYHOUR emits 08:00 and 08:30 here and reports
		//   08:30 first);
		//   base 09:00 → BYMINUTE expands to 09:00:00 and 09:30:00;
		//   bases 10:00..08:00 next day → nothing;
		//   2026-04-02 09:00 → 09:00:00, 09:30:00.
		r := mustParse(t, "FREQ=HOURLY;BYHOUR=9;BYMINUTE=0,30")
		want := []string{
			"20260401T090000Z",
			"20260401T093000Z",
			"20260402T090000Z",
			"20260402T093000Z",
		}
		if got := nextN(t, r, dt, 4); !eqStrs(got, want) {
			t.Fatalf("sequence = %v, want %v", got, want)
		}
	})
}

// TestNextOccurrence_Minutely pins FREQ=MINUTELY against the RFC
// 5545 §3.3.10 table's MINUTELY column: BYMONTH, BYMONTHDAY, BYDAY,
// BYHOUR and BYMINUTE limit; BYSECOND expands; BYSETPOS limits the
// period's expanded set. Every expected value is derived by hand
// from the rule and dtstart in the subtest comment. 2026-04-01 is a
// Wednesday, 2026-04-07 a Tuesday, 2026-02-01 a Sunday.
func TestNextOccurrence_Minutely(t *testing.T) {
	dt10 := mustParseUTC("20260401T100000Z")

	t.Run("first_5", func(t *testing.T) {
		// FREQ=MINUTELY from 10:00: one occurrence per minute base at
		// dtstart's second; dtstart itself is #1 and is not after
		// `after`, so the sequence starts at 10:01.
		r := mustParse(t, "FREQ=MINUTELY")
		want := []string{
			"20260401T100100Z", "20260401T100200Z", "20260401T100300Z",
			"20260401T100400Z", "20260401T100500Z",
		}
		if got := nextN(t, r, dt10, 5); !eqStrs(got, want) {
			t.Fatalf("sequence = %v, want %v", got, want)
		}
	})

	t.Run("interval_15", func(t *testing.T) {
		// INTERVAL=15: bases 10:00 (dtstart), 10:15, 10:30, 10:45,
		// 11:00 — an absolute 15-minute step, not a calendar unit.
		r := mustParse(t, "FREQ=MINUTELY;INTERVAL=15")
		want := []string{
			"20260401T101500Z", "20260401T103000Z",
			"20260401T104500Z", "20260401T110000Z",
		}
		if got := nextN(t, r, dt10, 4); !eqStrs(got, want) {
			t.Fatalf("sequence = %v, want %v", got, want)
		}
	})

	t.Run("bysecond_expands", func(t *testing.T) {
		// BYSECOND=0,30 expands within each minute: base 10:00 →
		// 10:00:00 (= dtstart, not after) and 10:00:30; base 10:01 →
		// 10:01:00, 10:01:30; base 10:02 → 10:02:00. An evaluator
		// that applies BYSECOND as a limit on the base's second
		// never emits :30.
		r := mustParse(t, "FREQ=MINUTELY;BYSECOND=0,30")
		want := []string{
			"20260401T100030Z", "20260401T100100Z",
			"20260401T100130Z", "20260401T100200Z",
		}
		if got := nextN(t, r, dt10, 4); !eqStrs(got, want) {
			t.Fatalf("sequence = %v, want %v", got, want)
		}
	})

	t.Run("byhour_byminute_limit", func(t *testing.T) {
		// BYHOUR=9;BYMINUTE=0,30 from 08:00: bases 08:00..08:59 fail
		// BYHOUR (dtstart is not occurrence #1); 09:00 passes;
		// 09:01..09:29 fail BYMINUTE; 09:30 passes; 09:31..08:59 next
		// day fail; 2026-04-02 09:00 and 09:30 follow.
		r := mustParse(t, "FREQ=MINUTELY;BYHOUR=9;BYMINUTE=0,30")
		dt := mustParseUTC("20260401T080000Z")
		want := []string{
			"20260401T090000Z", "20260401T093000Z",
			"20260402T090000Z", "20260402T093000Z",
		}
		if got := nextN(t, r, dt, 4); !eqStrs(got, want) {
			t.Fatalf("sequence = %v, want %v", got, want)
		}
	})

	t.Run("count_terminates", func(t *testing.T) {
		// COUNT=3 from 12:00: dtstart is #1, 12:01 #2, 12:02 #3; the
		// call after 12:02 reports normal termination (zero, false,
		// nil) — no error, no fourth occurrence.
		r := mustParse(t, "FREQ=MINUTELY;COUNT=3")
		dt := mustParseUTC("20260401T120000Z")
		want := []string{"20260401T120100Z", "20260401T120200Z"}
		if got := nextN(t, r, dt, 2); !eqStrs(got, want) {
			t.Fatalf("sequence = %v, want %v", got, want)
		}
		got, ok, err := NextOccurrence(r, dt, mustParseUTC("20260401T120200Z"))
		if err != nil || ok || !got.IsZero() {
			t.Fatalf("after #3: want (zero, false, nil), got (%v, %v, %v)", got, ok, err)
		}
	})

	t.Run("until_terminates", func(t *testing.T) {
		// UNTIL=12:02:00 inclusive from 12:00: 12:01, 12:02; the base
		// 12:03 is after UNTIL → (zero, false, nil).
		r := mustParse(t, "FREQ=MINUTELY;UNTIL=20260401T120200Z")
		dt := mustParseUTC("20260401T120000Z")
		want := []string{"20260401T120100Z", "20260401T120200Z"}
		if got := nextN(t, r, dt, 2); !eqStrs(got, want) {
			t.Fatalf("sequence = %v, want %v", got, want)
		}
		got, ok, err := NextOccurrence(r, dt, mustParseUTC("20260401T120200Z"))
		if err != nil || ok || !got.IsZero() {
			t.Fatalf("after UNTIL: want (zero, false, nil), got (%v, %v, %v)", got, ok, err)
		}
	})

	t.Run("byday_across_midnight", func(t *testing.T) {
		// INTERVAL=30;BYDAY=TU from Tuesday 2026-04-07 23:00: base
		// 23:30 is still Tuesday → fires; 2026-04-08 00:00 is
		// Wednesday → BYDAY fails, as does every base through Monday
		// 2026-04-13 23:30; Tuesday 2026-04-14 00:00 and 00:30 fire.
		// 289 periods from 23:30 to the next Tuesday, well under the
		// cap.
		r := mustParse(t, "FREQ=MINUTELY;INTERVAL=30;BYDAY=TU")
		dt := mustParseUTC("20260407T230000Z")
		want := []string{
			"20260407T233000Z", "20260414T000000Z", "20260414T003000Z",
		}
		if got := nextN(t, r, dt, 3); !eqStrs(got, want) {
			t.Fatalf("sequence = %v, want %v", got, want)
		}
	})

	t.Run("bysetpos_last_second", func(t *testing.T) {
		// BYSECOND=0,20,40;BYSETPOS=-1 from 10:00:00: each minute's
		// expanded set is {:00, :20, :40}; -1 keeps :40. Base 10:00 →
		// 10:00:40 (after dtstart); 10:01:40; 10:02:40.
		r := mustParse(t, "FREQ=MINUTELY;BYSECOND=0,20,40;BYSETPOS=-1")
		want := []string{
			"20260401T100040Z", "20260401T100140Z", "20260401T100240Z",
		}
		if got := nextN(t, r, dt10, 3); !eqStrs(got, want) {
			t.Fatalf("sequence = %v, want %v", got, want)
		}
	})

	t.Run("sparse_limit_caps", func(t *testing.T) {
		// BYMONTH=1 from 2026-02-01 00:00: every minute base until
		// 2027-01-01 fails BYMONTH. MaxIterations (100 000) minute
		// periods span 69 d 10 h 40 min and end 2026-04-11 10:40;
		// reaching January needs 334 days × 1440 = 480 960 periods.
		// The evaluator reports ErrIterationCap, never 00:01 (which
		// an evaluator that ignores BYMONTH would emit).
		r := mustParse(t, "FREQ=MINUTELY;BYMONTH=1")
		dt := mustParseUTC("20260201T000000Z")
		got, ok, err := NextOccurrence(r, dt, dt)
		if ok || !got.IsZero() {
			t.Fatalf("want no occurrence, got %v", got)
		}
		if !errors.Is(err, ErrIterationCap) {
			t.Fatalf("want ErrIterationCap, got %v", err)
		}
	})
}
