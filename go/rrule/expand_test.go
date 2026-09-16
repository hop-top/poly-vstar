// SPDX-License-Identifier: MIT

package rrule

import (
	"errors"
	"testing"
	"time"
)

// mustParse is the table-fixture helper for expansion tests: it
// parses an RRULE property value or panics (a bad fixture is a
// test-authoring bug, not a runtime condition).
func mustParse(t *testing.T, s string) Rule {
	t.Helper()
	r, err := ParseRRule(s)
	if err != nil {
		t.Fatalf("ParseRRule(%q) = %v, want nil", s, err)
	}
	return r
}

// utcs renders a []time.Time as wire-form UTC strings for readable
// table diffs.
func utcs(ts []time.Time) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.UTC().Format("20060102T150405Z")
	}
	return out
}

func eqStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ── Occurrences (bounded, count-limited) ──────────────────────────

type occCase struct {
	name    string
	rrule   string
	dtstart string
	limit   int
	want    []string
	// wantComplete asserts the Complete flag on the result: true
	// when the rule terminated on its own (UNTIL/COUNT) within the
	// limit, false when the limit truncated an ongoing series.
	wantComplete bool
}

// TestOccurrences pins the bounded expansion contract: the first
// occurrence is DTSTART itself (when it matches the BY* filters),
// results are chronological, and the limit truncates without
// error.
func TestOccurrences(t *testing.T) {
	cases := []occCase{
		{
			name:    "daily_count_terminates_before_limit",
			rrule:   "FREQ=DAILY;COUNT=3",
			dtstart: "20260401T120000Z",
			limit:   10,
			want: []string{
				"20260401T120000Z",
				"20260402T120000Z",
				"20260403T120000Z",
			},
			wantComplete: true,
		},
		{
			name:    "daily_unbounded_truncated_by_limit",
			rrule:   "FREQ=DAILY",
			dtstart: "20260401T120000Z",
			limit:   3,
			want: []string{
				"20260401T120000Z",
				"20260402T120000Z",
				"20260403T120000Z",
			},
			wantComplete: false,
		},
		{
			name:    "until_is_inclusive",
			rrule:   "FREQ=DAILY;UNTIL=20260403T120000Z",
			dtstart: "20260401T120000Z",
			limit:   10,
			want: []string{
				"20260401T120000Z",
				"20260402T120000Z",
				"20260403T120000Z",
			},
			wantComplete: true,
		},
		{
			name:    "interval_greater_than_one",
			rrule:   "FREQ=DAILY;INTERVAL=3;COUNT=3",
			dtstart: "20260401T120000Z",
			limit:   10,
			want: []string{
				"20260401T120000Z",
				"20260404T120000Z",
				"20260407T120000Z",
			},
			wantComplete: true,
		},
		{
			name:    "weekly_byday_multi",
			rrule:   "FREQ=WEEKLY;BYDAY=MO,WE;COUNT=4",
			dtstart: "20260406T090000Z", // Monday 2026-04-06
			limit:   10,
			want: []string{
				"20260406T090000Z", // Mon
				"20260408T090000Z", // Wed
				"20260413T090000Z", // Mon
				"20260415T090000Z", // Wed
			},
			wantComplete: true,
		},
		{
			name:    "monthly_byday_ordinal_second_tuesday",
			rrule:   "FREQ=MONTHLY;BYDAY=2TU;COUNT=3",
			dtstart: "20260414T100000Z", // 2nd Tuesday of April 2026
			limit:   10,
			want: []string{
				"20260414T100000Z",
				"20260512T100000Z",
				"20260609T100000Z",
			},
			wantComplete: true,
		},
		{
			name:    "monthly_bysetpos_last_weekday_of_month",
			rrule:   "FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1;COUNT=3",
			dtstart: "20260430T170000Z", // Thu 30 Apr 2026 = last weekday
			limit:   10,
			want: []string{
				"20260430T170000Z",
				"20260529T170000Z", // Fri 29 May 2026
				"20260630T170000Z", // Tue 30 Jun 2026
			},
			wantComplete: true,
		},
		{
			name:    "yearly_leap_day_skips_non_leap_years",
			rrule:   "FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=29;COUNT=2",
			dtstart: "20240229T080000Z",
			limit:   10,
			want: []string{
				"20240229T080000Z",
				"20280229T080000Z",
			},
			wantComplete: true,
		},
		{
			name:    "monthly_bymonthday_negative_last_day",
			rrule:   "FREQ=MONTHLY;BYMONTHDAY=-1;COUNT=3",
			dtstart: "20260131T235900Z",
			limit:   10,
			want: []string{
				"20260131T235900Z",
				"20260228T235900Z",
				"20260331T235900Z",
			},
			wantComplete: true,
		},
		{
			name:    "limit_zero_returns_nothing_and_is_incomplete",
			rrule:   "FREQ=DAILY;COUNT=3",
			dtstart: "20260401T120000Z",
			limit:   0,
			want:    nil,
			// A zero limit cannot prove the series ended.
			wantComplete: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := mustParse(t, tc.rrule)
			dtstart := mustParseUTC(tc.dtstart)
			got, complete, err := Occurrences(rule, dtstart, tc.limit)
			if err != nil {
				t.Fatalf("Occurrences: unexpected error %v", err)
			}
			if !eqStrs(utcs(got), tc.want) {
				t.Errorf("Occurrences = %v, want %v", utcs(got), tc.want)
			}
			if complete != tc.wantComplete {
				t.Errorf("complete = %v, want %v", complete, tc.wantComplete)
			}
		})
	}
}

// TestOccurrencesWKSTAffectsWeeklyExpansion pins the RFC 5545
// §3.3.10 WKST semantics: with INTERVAL>1 on FREQ=WEEKLY, the
// week-start choice moves which days fall in the same "week"
// period and therefore changes the expansion.
func TestOccurrencesWKSTAffectsWeeklyExpansion(t *testing.T) {
	// DTSTART Sunday 2026-04-05. With WKST=MO, Sunday closes the
	// week that began Mon 2026-03-30, so the following Monday
	// (2026-04-06) belongs to the NEXT week and is skipped by
	// INTERVAL=2. With WKST=SU, Sunday OPENS the week, so the
	// Monday right after is in the same week and fires.
	dtstart := mustParseUTC("20260405T090000Z")

	gotMO, _, err := Occurrences(mustParse(t, "FREQ=WEEKLY;INTERVAL=2;BYDAY=SU,MO;WKST=MO;COUNT=4"), dtstart, 10)
	if err != nil {
		t.Fatalf("WKST=MO: %v", err)
	}
	gotSU, _, err := Occurrences(mustParse(t, "FREQ=WEEKLY;INTERVAL=2;BYDAY=SU,MO;WKST=SU;COUNT=4"), dtstart, 10)
	if err != nil {
		t.Fatalf("WKST=SU: %v", err)
	}

	if eqStrs(utcs(gotMO), utcs(gotSU)) {
		t.Fatalf("WKST had no effect on biweekly expansion: both = %v", utcs(gotMO))
	}

	// WKST=SU: Sun 04-05 and Mon 04-06 share week 1; week 3 begins
	// Sun 04-19.
	wantSU := []string{
		"20260405T090000Z",
		"20260406T090000Z",
		"20260419T090000Z",
		"20260420T090000Z",
	}
	if !eqStrs(utcs(gotSU), wantSU) {
		t.Errorf("WKST=SU = %v, want %v", utcs(gotSU), wantSU)
	}
}

// ── Between ───────────────────────────────────────────────────────

// TestBetween pins the half-open [start, end) window contract.
func TestBetween(t *testing.T) {
	rule := mustParse(t, "FREQ=DAILY")
	dtstart := mustParseUTC("20260401T120000Z")

	got, err := Between(rule, dtstart, mustParseUTC("20260403T120000Z"), mustParseUTC("20260406T120000Z"))
	if err != nil {
		t.Fatalf("Between: %v", err)
	}
	// start inclusive, end exclusive.
	want := []string{
		"20260403T120000Z",
		"20260404T120000Z",
		"20260405T120000Z",
	}
	if !eqStrs(utcs(got), want) {
		t.Errorf("Between = %v, want %v", utcs(got), want)
	}
}

// TestBetweenStopsAtRuleTermination confirms Between honors
// COUNT/UNTIL even when the window extends past the series end.
func TestBetweenStopsAtRuleTermination(t *testing.T) {
	rule := mustParse(t, "FREQ=DAILY;COUNT=2")
	dtstart := mustParseUTC("20260401T120000Z")

	got, err := Between(rule, dtstart, dtstart, mustParseUTC("20270101T000000Z"))
	if err != nil {
		t.Fatalf("Between: %v", err)
	}
	want := []string{"20260401T120000Z", "20260402T120000Z"}
	if !eqStrs(utcs(got), want) {
		t.Errorf("Between = %v, want %v", utcs(got), want)
	}
}

// TestBetweenRejectsInvertedWindow: end <= start is a caller bug,
// not an empty result.
func TestBetweenRejectsInvertedWindow(t *testing.T) {
	rule := mustParse(t, "FREQ=DAILY")
	dtstart := mustParseUTC("20260401T120000Z")
	_, err := Between(rule, dtstart, mustParseUTC("20260405T120000Z"), mustParseUTC("20260403T120000Z"))
	if err == nil {
		t.Fatalf("Between with end < start: got nil error, want non-nil")
	}
}

// TestBetweenUnboundedWindowIsRejected: Between's termination
// contract requires a finite window. A zero `end` would be an
// unbounded expansion request.
func TestBetweenUnboundedWindowIsRejected(t *testing.T) {
	rule := mustParse(t, "FREQ=DAILY")
	dtstart := mustParseUTC("20260401T120000Z")
	if _, err := Between(rule, dtstart, dtstart, time.Time{}); err == nil {
		t.Fatalf("Between with zero end: got nil error, want non-nil")
	}
}

// ── All (iterator) ────────────────────────────────────────────────

// TestAllIsLazyAndStopsOnBreak proves All returns a lazy iterator:
// an infinite rule must be safe to range over as long as the
// consumer breaks.
func TestAllIsLazyAndStopsOnBreak(t *testing.T) {
	rule := mustParse(t, "FREQ=DAILY") // no UNTIL, no COUNT
	dtstart := mustParseUTC("20260401T120000Z")

	var got []time.Time
	for occ := range All(rule, dtstart) {
		got = append(got, occ)
		if len(got) == 4 {
			break
		}
	}
	want := []string{
		"20260401T120000Z",
		"20260402T120000Z",
		"20260403T120000Z",
		"20260404T120000Z",
	}
	if !eqStrs(utcs(got), want) {
		t.Errorf("All (first 4) = %v, want %v", utcs(got), want)
	}
}

// TestAllTerminatesOnCount: a bounded rule ends on its own without
// the consumer breaking.
func TestAllTerminatesOnCount(t *testing.T) {
	rule := mustParse(t, "FREQ=WEEKLY;COUNT=3")
	dtstart := mustParseUTC("20260406T090000Z")

	var got []time.Time
	for occ := range All(rule, dtstart) {
		got = append(got, occ)
		if len(got) > 10 {
			t.Fatalf("All did not terminate on COUNT=3")
		}
	}
	want := []string{
		"20260406T090000Z",
		"20260413T090000Z",
		"20260420T090000Z",
	}
	if !eqStrs(utcs(got), want) {
		t.Errorf("All = %v, want %v", utcs(got), want)
	}
}

// TestOccurrencesRejectsInvalidRule: a Rule literal that never went
// through ParseRRule (Interval 0, FREQ unset) is a caller error.
func TestOccurrencesRejectsInvalidRule(t *testing.T) {
	_, _, err := Occurrences(Rule{}, mustParseUTC("20260401T120000Z"), 5)
	if err == nil {
		t.Fatalf("Occurrences(zero Rule): got nil error, want non-nil")
	}
	if !errors.Is(err, ErrUnsupportedRRule) {
		t.Errorf("Occurrences(zero Rule): error %v, want ErrUnsupportedRRule", err)
	}
}

// TestOccurrencesDSTBoundary: with a zoned DTSTART, a daily rule
// keeps wall-clock time across a DST transition, so the UTC offset
// shifts by an hour.
func TestOccurrencesDSTBoundary(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("America/New_York unavailable: %v", err)
	}
	// US spring-forward 2026: Sunday 2026-03-08 02:00 local.
	dtstart := time.Date(2026, time.March, 6, 9, 0, 0, 0, ny)
	got, _, err := Occurrences(mustParse(t, "FREQ=DAILY;COUNT=4"), dtstart, 10)
	if err != nil {
		t.Fatalf("Occurrences: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d occurrences, want 4", len(got))
	}
	for i, occ := range got {
		if h := occ.In(ny).Hour(); h != 9 {
			t.Errorf("occurrence %d (%s): local hour %d, want 9 (wall-clock preserved across DST)", i, occ, h)
		}
	}
	// Before the transition the offset is EST (-5); after, EDT (-4).
	if got[0].UTC().Hour() != 14 {
		t.Errorf("pre-DST occurrence UTC hour = %d, want 14 (EST)", got[0].UTC().Hour())
	}
	if got[3].UTC().Hour() != 13 {
		t.Errorf("post-DST occurrence UTC hour = %d, want 13 (EDT)", got[3].UTC().Hour())
	}
}

// ── Iteration cap ─────────────────────────────────────────────────

// starvedRule parses cleanly — the parser stays permissive on
// unsatisfiable BY-* combinations — but never yields: February has
// no 30th, so every period is empty and only the evaluator's
// iteration bound stops the walk.
const starvedRule = "FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30"

// TestOccurrencesReportsIterationCap: an abandoned search is not a
// completed series. Occurrences must report ErrIterationCap, never
// (nil, complete=true, nil).
func TestOccurrencesReportsIterationCap(t *testing.T) {
	got, complete, err := Occurrences(mustParse(t, starvedRule), mustParseUTC("20260101T090000Z"), 5)
	if !errors.Is(err, ErrIterationCap) {
		t.Fatalf("Occurrences(starved) = (%v, %v, %v), want ErrIterationCap", utcs(got), complete, err)
	}
	if complete {
		t.Errorf("complete = true alongside ErrIterationCap; an abandoned search is not a completed series")
	}
}

// TestBetweenReportsIterationCap: the window bounds the result, not
// the search — a starved rule never reaches `end`, so the iteration
// bound is what stops it and must be reported.
func TestBetweenReportsIterationCap(t *testing.T) {
	_, err := Between(mustParse(t, starvedRule), mustParseUTC("20260101T090000Z"),
		mustParseUTC("20260101T090000Z"), mustParseUTC("20270101T090000Z"))
	if !errors.Is(err, ErrIterationCap) {
		t.Fatalf("Between(starved) error = %v, want ErrIterationCap", err)
	}
}

// TestAllEndsOnIterationCap: the lazy iterator has nowhere to put
// an error, so a starved rule ends the sequence — it must neither
// yield nor spin forever.
func TestAllEndsOnIterationCap(t *testing.T) {
	for occ := range All(mustParse(t, starvedRule), mustParseUTC("20260101T090000Z")) {
		t.Fatalf("All(starved) yielded %s, want nothing", occ)
	}
}
