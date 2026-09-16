// SPDX-License-Identifier: MIT

package rrule

import (
	"errors"
	"reflect"
	"testing"
	"time"

	vstar "hop.top/vstar"
)

// parseCase is one row of the ParseRRule table. wantErr is the
// sentinel the parser must wrap (vstar.ErrMalformed or
// ErrUnsupportedRRule); when nil, want is the expected Rule.
type parseCase struct {
	name    string
	in      string
	want    Rule
	wantErr error
}

// parseCases is the shared table for ParseRRule and ValidateRRule
// tests (T2 and T3). Cases cover every accepted rule-part, the
// defaults (INTERVAL absent → 1, WKST absent → MO), and every
// malformed and deferred category from spec/03 §RRULE parsing scope.
func parseCases() []parseCase {
	mustUntil := func(s string) time.Time {
		t, ok := vstar.ParseTime(s)
		if !ok {
			panic("parseCases: bad fixture UNTIL: " + s)
		}
		return t
	}
	return []parseCase{
		// ── Accepted: every FREQ value ───────────────────────────
		{
			name: "freq_minutely",
			in:   "FREQ=MINUTELY",
			want: Rule{Freq: FreqMinutely, Interval: 1, WeekStart: MO},
		},
		{
			name: "freq_hourly",
			in:   "FREQ=HOURLY",
			want: Rule{Freq: FreqHourly, Interval: 1, WeekStart: MO},
		},
		{
			name: "freq_daily",
			in:   "FREQ=DAILY",
			want: Rule{Freq: FreqDaily, Interval: 1, WeekStart: MO},
		},
		{
			name: "freq_weekly",
			in:   "FREQ=WEEKLY",
			want: Rule{Freq: FreqWeekly, Interval: 1, WeekStart: MO},
		},
		{
			name: "freq_monthly",
			in:   "FREQ=MONTHLY",
			want: Rule{Freq: FreqMonthly, Interval: 1, WeekStart: MO},
		},
		{
			name: "freq_yearly",
			in:   "FREQ=YEARLY",
			want: Rule{Freq: FreqYearly, Interval: 1, WeekStart: MO},
		},

		// ── Accepted: INTERVAL ───────────────────────────────────
		{
			name: "interval_explicit",
			in:   "FREQ=DAILY;INTERVAL=2",
			want: Rule{Freq: FreqDaily, Interval: 2, WeekStart: MO},
		},
		{
			name: "interval_large",
			in:   "FREQ=DAILY;INTERVAL=365",
			want: Rule{Freq: FreqDaily, Interval: 365, WeekStart: MO},
		},

		// ── Accepted: UNTIL (form #2) ───────────────────────────
		{
			name: "until_form2",
			in:   "FREQ=DAILY;UNTIL=20261231T235959Z",
			want: Rule{
				Freq:      FreqDaily,
				Interval:  1,
				Until:     mustUntil("20261231T235959Z"),
				WeekStart: MO,
			},
		},

		// ── Accepted: COUNT ──────────────────────────────────────
		{
			name: "count",
			in:   "FREQ=DAILY;COUNT=10",
			want: Rule{Freq: FreqDaily, Interval: 1, Count: 10, WeekStart: MO},
		},

		// ── Accepted: BYDAY (no ordinal) ────────────────────────
		{
			name: "byday_simple",
			in:   "FREQ=WEEKLY;BYDAY=MO,WE,FR",
			want: Rule{
				Freq:      FreqWeekly,
				Interval:  1,
				ByDay:     []ByDay{{0, MO}, {0, WE}, {0, FR}},
				WeekStart: MO,
			},
		},

		// ── Accepted: BYDAY (with positive ordinal) ─────────────
		{
			name: "byday_positive_ordinal",
			in:   "FREQ=MONTHLY;BYDAY=2MO",
			want: Rule{
				Freq:      FreqMonthly,
				Interval:  1,
				ByDay:     []ByDay{{2, MO}},
				WeekStart: MO,
			},
		},

		// ── Accepted: BYDAY (with negative ordinal) ─────────────
		{
			name: "byday_negative_ordinal",
			in:   "FREQ=MONTHLY;BYDAY=-1FR",
			want: Rule{
				Freq:      FreqMonthly,
				Interval:  1,
				ByDay:     []ByDay{{-1, FR}},
				WeekStart: MO,
			},
		},

		// ── Accepted: BYMONTH ────────────────────────────────────
		{
			name: "bymonth",
			in:   "FREQ=YEARLY;BYMONTH=1,6,12",
			want: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByMonth:   []int{1, 6, 12},
				WeekStart: MO,
			},
		},

		// ── Accepted: BYMONTHDAY (positive + negative) ──────────
		{
			name: "bymonthday_pos_neg",
			in:   "FREQ=MONTHLY;BYMONTHDAY=1,15,-1",
			want: Rule{
				Freq:       FreqMonthly,
				Interval:   1,
				ByMonthDay: []int{1, 15, -1},
				WeekStart:  MO,
			},
		},

		// ── Accepted: BYHOUR/BYMINUTE/BYSECOND ──────────────────
		{
			name: "byhour_byminute_bysecond",
			in:   "FREQ=DAILY;BYHOUR=9,17;BYMINUTE=0,30;BYSECOND=0",
			want: Rule{
				Freq:      FreqDaily,
				Interval:  1,
				ByHour:    []int{9, 17},
				ByMinute:  []int{0, 30},
				BySecond:  []int{0},
				WeekStart: MO,
			},
		},
		{
			name: "bysecond_leap_60",
			in:   "FREQ=DAILY;BYSECOND=60",
			want: Rule{
				Freq:      FreqDaily,
				Interval:  1,
				BySecond:  []int{60},
				WeekStart: MO,
			},
		},

		// ── Accepted: WKST ───────────────────────────────────────
		{
			name: "wkst_explicit_su",
			in:   "FREQ=WEEKLY;WKST=SU",
			want: Rule{Freq: FreqWeekly, Interval: 1, WeekStart: SU},
		},

		// ── Accepted: order independence ─────────────────────────
		{
			name: "order_freq_last",
			in:   "INTERVAL=2;FREQ=DAILY",
			want: Rule{Freq: FreqDaily, Interval: 2, WeekStart: MO},
		},

		// ── Accepted: combined typical VTODO recurrence ────────
		{
			name: "vtodo_typical",
			in:   "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE,FR;COUNT=20",
			want: Rule{
				Freq:      FreqWeekly,
				Interval:  2,
				Count:     20,
				ByDay:     []ByDay{{0, MO}, {0, WE}, {0, FR}},
				WeekStart: MO,
			},
		},

		// ─────────────────────────────────────────────────────────
		// Rejected: ErrMalformed (syntactic)
		// ─────────────────────────────────────────────────────────
		{
			name:    "missing_freq",
			in:      "INTERVAL=2",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "empty_input",
			in:      "",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "unknown_rule_part",
			in:      "FREQ=DAILY;FOO=BAR",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "freq_invalid_value",
			in:      "FREQ=NOPE",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "interval_zero",
			in:      "FREQ=DAILY;INTERVAL=0",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "interval_negative",
			in:      "FREQ=DAILY;INTERVAL=-1",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "interval_non_integer",
			in:      "FREQ=DAILY;INTERVAL=abc",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "until_and_count_together",
			in:      "FREQ=DAILY;UNTIL=20261231T235959Z;COUNT=10",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "until_form1_local",
			in:      "FREQ=DAILY;UNTIL=20261231T235959",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "until_form3_tzid",
			in:      "FREQ=DAILY;UNTIL=20261231T235959;TZID=America/New_York",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "count_zero",
			in:      "FREQ=DAILY;COUNT=0",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "count_negative",
			in:      "FREQ=DAILY;COUNT=-1",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "bymonthday_zero",
			in:      "FREQ=MONTHLY;BYMONTHDAY=0",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "bymonthday_out_of_range",
			in:      "FREQ=MONTHLY;BYMONTHDAY=32",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byday_ordinal_zero",
			in:      "FREQ=MONTHLY;BYDAY=0SU",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byday_unknown_weekday",
			in:      "FREQ=WEEKLY;BYDAY=XX",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "bymonth_out_of_range",
			in:      "FREQ=YEARLY;BYMONTH=13",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byhour_out_of_range",
			in:      "FREQ=DAILY;BYHOUR=24",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byminute_out_of_range",
			in:      "FREQ=DAILY;BYMINUTE=60",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "bysecond_out_of_range",
			in:      "FREQ=DAILY;BYSECOND=61",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "wkst_unknown",
			in:      "FREQ=WEEKLY;WKST=XX",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "missing_value",
			in:      "FREQ=DAILY;INTERVAL=",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "missing_equals",
			in:      "FREQ=DAILY;INTERVAL2",
			wantErr: vstar.ErrMalformed,
		},

		// ─────────────────────────────────────────────────────────
		// Rejected: ErrUnsupportedRRule (out of v0.2 scope)
		// ─────────────────────────────────────────────────────────
		{
			name:    "freq_secondly",
			in:      "FREQ=SECONDLY",
			wantErr: ErrUnsupportedRRule,
		},
		{
			name:    "rscale",
			in:      "FREQ=YEARLY;RSCALE=HEBREW",
			wantErr: ErrUnsupportedRRule,
		},

		// ── Accepted: BYYEARDAY (FREQ=YEARLY only) ───────────────
		{
			name: "byyearday_positive",
			in:   "FREQ=YEARLY;BYYEARDAY=100",
			want: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByYearDay: []int{100},
				WeekStart: MO,
			},
		},
		{
			name: "byyearday_negative",
			in:   "FREQ=YEARLY;BYYEARDAY=-1",
			want: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByYearDay: []int{-1},
				WeekStart: MO,
			},
		},
		{
			name: "byyearday_leap_366",
			in:   "FREQ=YEARLY;BYYEARDAY=366",
			want: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByYearDay: []int{366},
				WeekStart: MO,
			},
		},
		{
			name: "byyearday_list",
			in:   "FREQ=YEARLY;BYYEARDAY=1,100,-1",
			want: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByYearDay: []int{1, 100, -1},
				WeekStart: MO,
			},
		},
		{
			name:    "byyearday_zero",
			in:      "FREQ=YEARLY;BYYEARDAY=0",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byyearday_out_of_range_high",
			in:      "FREQ=YEARLY;BYYEARDAY=367",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byyearday_out_of_range_low",
			in:      "FREQ=YEARLY;BYYEARDAY=-367",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byyearday_with_monthly_rejected",
			in:      "FREQ=MONTHLY;BYYEARDAY=100",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byyearday_with_daily_rejected",
			in:      "FREQ=DAILY;BYYEARDAY=100",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byyearday_non_integer",
			in:      "FREQ=YEARLY;BYYEARDAY=abc",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byyearday_empty",
			in:      "FREQ=YEARLY;BYYEARDAY=",
			wantErr: vstar.ErrMalformed,
		},

		// ── Accepted: BYWEEKNO (FREQ=YEARLY only) ────────────────
		{
			name: "byweekno_positive",
			in:   "FREQ=YEARLY;BYWEEKNO=20",
			want: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByWeekNo:  []int{20},
				WeekStart: MO,
			},
		},
		{
			name: "byweekno_negative",
			in:   "FREQ=YEARLY;BYWEEKNO=-1",
			want: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByWeekNo:  []int{-1},
				WeekStart: MO,
			},
		},
		{
			name: "byweekno_53",
			in:   "FREQ=YEARLY;BYWEEKNO=53",
			want: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByWeekNo:  []int{53},
				WeekStart: MO,
			},
		},
		{
			name: "byweekno_list",
			in:   "FREQ=YEARLY;BYWEEKNO=1,26,-1",
			want: Rule{
				Freq:      FreqYearly,
				Interval:  1,
				ByWeekNo:  []int{1, 26, -1},
				WeekStart: MO,
			},
		},
		{
			name:    "byweekno_zero",
			in:      "FREQ=YEARLY;BYWEEKNO=0",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byweekno_out_of_range_high",
			in:      "FREQ=YEARLY;BYWEEKNO=54",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byweekno_out_of_range_low",
			in:      "FREQ=YEARLY;BYWEEKNO=-54",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byweekno_with_monthly_rejected",
			in:      "FREQ=MONTHLY;BYWEEKNO=20",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "byweekno_with_weekly_rejected",
			in:      "FREQ=WEEKLY;BYWEEKNO=20",
			wantErr: vstar.ErrMalformed,
		},

		// ── Accepted: BYSETPOS (with another BY-* clause) ────────
		{
			name: "bysetpos_last_weekday_of_month",
			in:   "FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1",
			want: Rule{
				Freq:      FreqMonthly,
				Interval:  1,
				ByDay:     []ByDay{{0, MO}, {0, TU}, {0, WE}, {0, TH}, {0, FR}},
				BySetPos:  []int{-1},
				WeekStart: MO,
			},
		},
		{
			name: "bysetpos_second_tuesday",
			in:   "FREQ=MONTHLY;BYDAY=TU;BYSETPOS=2",
			want: Rule{
				Freq:      FreqMonthly,
				Interval:  1,
				ByDay:     []ByDay{{0, TU}},
				BySetPos:  []int{2},
				WeekStart: MO,
			},
		},
		{
			name: "bysetpos_list_mixed_signs",
			in:   "FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=1,-1",
			want: Rule{
				Freq:      FreqMonthly,
				Interval:  1,
				ByDay:     []ByDay{{0, MO}, {0, TU}, {0, WE}, {0, TH}, {0, FR}},
				BySetPos:  []int{1, -1},
				WeekStart: MO,
			},
		},
		{
			name:    "bysetpos_zero",
			in:      "FREQ=MONTHLY;BYDAY=MO;BYSETPOS=0",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "bysetpos_out_of_range_high",
			in:      "FREQ=MONTHLY;BYDAY=MO;BYSETPOS=367",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "bysetpos_out_of_range_low",
			in:      "FREQ=MONTHLY;BYDAY=MO;BYSETPOS=-367",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "bysetpos_alone_rejected",
			in:      "FREQ=MONTHLY;BYSETPOS=-1",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "bysetpos_with_only_freq_rejected",
			in:      "FREQ=YEARLY;BYSETPOS=1",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "bysetpos_non_integer",
			in:      "FREQ=MONTHLY;BYDAY=MO;BYSETPOS=abc",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "bysetpos_empty",
			in:      "FREQ=MONTHLY;BYDAY=MO;BYSETPOS=",
			wantErr: vstar.ErrMalformed,
		},
	}
}

// TestParseRRule runs the shared parseCases table.
func TestParseRRule(t *testing.T) {
	for _, c := range parseCases() {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseRRule(c.in)
			if c.wantErr != nil {
				if err == nil {
					t.Fatalf("ParseRRule(%q): want error %v, got nil (rule=%+v)", c.in, c.wantErr, got)
				}
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("ParseRRule(%q): want errors.Is(%v), got %v", c.in, c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRRule(%q): unexpected error: %v", c.in, err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("ParseRRule(%q):\n  got:  %+v\n  want: %+v", c.in, got, c.want)
			}
		})
	}
}

// TestParseRRule_LowercaseRejected asserts the parser is strict on
// case for both keys and values — RFC 5545 wire form is uppercase.
// (Helps consumers detect bugs early; matches the strict-by-default
// posture in vstar.ParseTime.)
func TestParseRRule_LowercaseFreq(t *testing.T) {
	if _, err := ParseRRule("freq=daily"); err == nil {
		t.Fatal("ParseRRule(\"freq=daily\"): want ErrMalformed, got nil")
	}
}

// TestParseRRule_DuplicateRulePart asserts a repeated rule-part
// (e.g. FREQ given twice) is ErrMalformed.
func TestParseRRule_DuplicateRulePart(t *testing.T) {
	_, err := ParseRRule("FREQ=DAILY;FREQ=WEEKLY")
	if !errors.Is(err, vstar.ErrMalformed) {
		t.Fatalf("duplicate FREQ: want ErrMalformed, got %v", err)
	}
}
