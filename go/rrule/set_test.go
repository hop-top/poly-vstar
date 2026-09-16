// SPDX-License-Identifier: MIT

package rrule

import (
	"errors"
	"testing"
	"time"

	vstar "hop.top/vstar"
)

// ── EXDATE / RDATE value parsing ──────────────────────────────────

// TestParseDateTimeList covers the multi-valued EXDATE/RDATE value
// form: comma-separated RFC 5545 form #2 (UTC) datetimes.
func TestParseDateTimeList(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{
			name: "single",
			in:   "20260402T120000Z",
			want: []string{"20260402T120000Z"},
		},
		{
			name: "multi",
			in:   "20260402T120000Z,20260403T120000Z",
			want: []string{"20260402T120000Z", "20260403T120000Z"},
		},
		{
			name: "unsorted_input_is_sorted",
			in:   "20260403T120000Z,20260402T120000Z",
			want: []string{"20260402T120000Z", "20260403T120000Z"},
		},
		{
			name: "duplicates_removed",
			in:   "20260402T120000Z,20260402T120000Z",
			want: []string{"20260402T120000Z"},
		},
		{
			name:    "empty",
			in:      "",
			wantErr: true,
		},
		{
			name:    "local_form_one_rejected",
			in:      "20260402T120000",
			wantErr: true,
		},
		{
			name:    "garbage",
			in:      "not-a-time",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseDateTimeList(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseDateTimeList(%q): got nil error, want non-nil", tc.in)
				}
				if !errors.Is(err, vstar.ErrMalformed) {
					t.Errorf("error %v, want ErrMalformed", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDateTimeList(%q) = %v, want nil", tc.in, err)
			}
			if !eqStrs(utcs(got), tc.want) {
				t.Errorf("= %v, want %v", utcs(got), tc.want)
			}
		})
	}
}

// TestParseDateTimeListRejectsValueDate documents the VALUE=DATE
// boundary: date-only EXDATE/RDATE values are not handled by this
// entry point — recurrence sets are UTC form #2 only by spec
// (spec/03 §Recurrence sets).
func TestParseDateTimeListRejectsValueDate(t *testing.T) {
	if _, err := ParseDateTimeList("20260402"); err == nil {
		t.Fatalf("ParseDateTimeList(date-only): got nil error, want non-nil")
	}
}

// ── Set expansion ─────────────────────────────────────────────────

// TestSetExcludesEXDATE: an EXDATE removes exactly one occurrence
// from the RRULE expansion and does not shift the rest.
func TestSetExcludesEXDATE(t *testing.T) {
	dtstart := mustParseUTC("20260401T120000Z")
	set := Set{
		DTStart: dtstart,
		RRule:   ptrRule(mustParse(t, "FREQ=DAILY;COUNT=4")),
		ExDate:  mustTimes(t, "20260402T120000Z"),
	}
	got, complete, err := set.Occurrences(10)
	if err != nil {
		t.Fatalf("Set.Occurrences: %v", err)
	}
	want := []string{
		"20260401T120000Z",
		"20260403T120000Z",
		"20260404T120000Z",
	}
	if !eqStrs(utcs(got), want) {
		t.Errorf("= %v, want %v", utcs(got), want)
	}
	if !complete {
		t.Errorf("complete = false, want true")
	}
}

// TestSetIncludesRDATE: an RDATE outside the RRULE pattern is
// merged into the result in sorted position.
func TestSetIncludesRDATE(t *testing.T) {
	dtstart := mustParseUTC("20260401T120000Z")
	set := Set{
		DTStart: dtstart,
		RRule:   ptrRule(mustParse(t, "FREQ=DAILY;COUNT=2")),
		RDate:   mustTimes(t, "20260401T180000Z", "20260501T090000Z"),
	}
	got, _, err := set.Occurrences(10)
	if err != nil {
		t.Fatalf("Set.Occurrences: %v", err)
	}
	want := []string{
		"20260401T120000Z",
		"20260401T180000Z", // RDATE, same day, later hour
		"20260402T120000Z",
		"20260501T090000Z", // RDATE, well outside the rule
	}
	if !eqStrs(utcs(got), want) {
		t.Errorf("= %v, want %v", utcs(got), want)
	}
}

// TestSetEXDATEBeatsRDATE: RFC 5545 §3.8.5.1 — EXDATE removal is
// applied after RDATE merge, so an excluded date stays excluded
// even if an RDATE names it.
func TestSetEXDATEBeatsRDATE(t *testing.T) {
	dtstart := mustParseUTC("20260401T120000Z")
	set := Set{
		DTStart: dtstart,
		RRule:   ptrRule(mustParse(t, "FREQ=DAILY;COUNT=2")),
		RDate:   mustTimes(t, "20260410T120000Z"),
		ExDate:  mustTimes(t, "20260410T120000Z"),
	}
	got, _, err := set.Occurrences(10)
	if err != nil {
		t.Fatalf("Set.Occurrences: %v", err)
	}
	want := []string{"20260401T120000Z", "20260402T120000Z"}
	if !eqStrs(utcs(got), want) {
		t.Errorf("= %v, want %v", utcs(got), want)
	}
}

// TestSetDeduplicates: an RDATE that coincides with an RRULE
// occurrence yields one entry, not two.
func TestSetDeduplicates(t *testing.T) {
	dtstart := mustParseUTC("20260401T120000Z")
	set := Set{
		DTStart: dtstart,
		RRule:   ptrRule(mustParse(t, "FREQ=DAILY;COUNT=2")),
		RDate:   mustTimes(t, "20260402T120000Z"),
	}
	got, _, err := set.Occurrences(10)
	if err != nil {
		t.Fatalf("Set.Occurrences: %v", err)
	}
	want := []string{"20260401T120000Z", "20260402T120000Z"}
	if !eqStrs(utcs(got), want) {
		t.Errorf("= %v, want %v", utcs(got), want)
	}
}

// TestSetRDateOnly: a Set with no RRULE is valid — RDATE-only
// recurrence is legal per RFC 5545 and always terminates.
func TestSetRDateOnly(t *testing.T) {
	dtstart := mustParseUTC("20260401T120000Z")
	set := Set{
		DTStart: dtstart,
		RDate:   mustTimes(t, "20260405T120000Z", "20260403T120000Z"),
	}
	got, complete, err := set.Occurrences(10)
	if err != nil {
		t.Fatalf("Set.Occurrences: %v", err)
	}
	want := []string{
		"20260401T120000Z", // DTSTART is always occurrence #1
		"20260403T120000Z",
		"20260405T120000Z",
	}
	if !eqStrs(utcs(got), want) {
		t.Errorf("= %v, want %v", utcs(got), want)
	}
	if !complete {
		t.Errorf("complete = false, want true (RDATE-only sets are finite)")
	}
}

// TestSetExcludingDTStart: EXDATE may remove DTSTART itself.
func TestSetExcludingDTStart(t *testing.T) {
	dtstart := mustParseUTC("20260401T120000Z")
	set := Set{
		DTStart: dtstart,
		RRule:   ptrRule(mustParse(t, "FREQ=DAILY;COUNT=3")),
		ExDate:  mustTimes(t, "20260401T120000Z"),
	}
	got, _, err := set.Occurrences(10)
	if err != nil {
		t.Fatalf("Set.Occurrences: %v", err)
	}
	want := []string{"20260402T120000Z", "20260403T120000Z"}
	if !eqStrs(utcs(got), want) {
		t.Errorf("= %v, want %v", utcs(got), want)
	}
}

// TestSetBetween applies the window to the combined set.
func TestSetBetween(t *testing.T) {
	dtstart := mustParseUTC("20260401T120000Z")
	set := Set{
		DTStart: dtstart,
		RRule:   ptrRule(mustParse(t, "FREQ=DAILY")),
		ExDate:  mustTimes(t, "20260403T120000Z"),
		RDate:   mustTimes(t, "20260404T180000Z"),
	}
	got, err := set.Between(mustParseUTC("20260402T000000Z"), mustParseUTC("20260405T000000Z"))
	if err != nil {
		t.Fatalf("Set.Between: %v", err)
	}
	want := []string{
		"20260402T120000Z",
		// 04-03 excluded by EXDATE
		"20260404T120000Z",
		"20260404T180000Z",
	}
	if !eqStrs(utcs(got), want) {
		t.Errorf("= %v, want %v", utcs(got), want)
	}
}

// TestSetFromComponent wires a parsed VEVENT into a Set.
func TestSetFromComponent(t *testing.T) {
	c := vstar.Component{
		Type: "VEVENT",
		Props: []vstar.Property{
			{Name: "DTSTART", Value: "20260401T120000Z"},
			{Name: "RRULE", Value: "FREQ=DAILY;COUNT=4"},
			{Name: "EXDATE", Value: "20260402T120000Z"},
			{Name: "RDATE", Value: "20260410T080000Z"},
		},
	}
	set, err := SetFromComponent(c)
	if err != nil {
		t.Fatalf("SetFromComponent: %v", err)
	}
	got, _, err := set.Occurrences(10)
	if err != nil {
		t.Fatalf("Set.Occurrences: %v", err)
	}
	want := []string{
		"20260401T120000Z",
		"20260403T120000Z",
		"20260404T120000Z",
		"20260410T080000Z",
	}
	if !eqStrs(utcs(got), want) {
		t.Errorf("= %v, want %v", utcs(got), want)
	}
}

// TestSetFromComponentMultipleExdateProperties: EXDATE and RDATE
// may each appear more than once; all values accumulate.
func TestSetFromComponentMultipleExdateProperties(t *testing.T) {
	c := vstar.Component{
		Type: "VEVENT",
		Props: []vstar.Property{
			{Name: "DTSTART", Value: "20260401T120000Z"},
			{Name: "RRULE", Value: "FREQ=DAILY;COUNT=5"},
			{Name: "EXDATE", Value: "20260402T120000Z"},
			{Name: "EXDATE", Value: "20260403T120000Z,20260404T120000Z"},
		},
	}
	set, err := SetFromComponent(c)
	if err != nil {
		t.Fatalf("SetFromComponent: %v", err)
	}
	if len(set.ExDate) != 3 {
		t.Fatalf("ExDate = %v, want 3 entries", utcs(set.ExDate))
	}
	got, _, err := set.Occurrences(10)
	if err != nil {
		t.Fatalf("Set.Occurrences: %v", err)
	}
	want := []string{"20260401T120000Z", "20260405T120000Z"}
	if !eqStrs(utcs(got), want) {
		t.Errorf("= %v, want %v", utcs(got), want)
	}
}

// TestSetFromComponentValueDateUnsupported: EXDATE carrying
// VALUE=DATE is reported as unsupported rather than silently
// mis-parsed — outside the spec's recurrence-set scope.
func TestSetFromComponentValueDateUnsupported(t *testing.T) {
	c := vstar.Component{
		Type: "VEVENT",
		Props: []vstar.Property{
			{Name: "DTSTART", Value: "20260401T120000Z"},
			{Name: "RRULE", Value: "FREQ=DAILY;COUNT=4"},
			{
				Name:   "EXDATE",
				Params: []vstar.Param{{Name: "VALUE", Value: "DATE"}},
				Value:  "20260402",
			},
		},
	}
	_, err := SetFromComponent(c)
	if err == nil {
		t.Fatalf("SetFromComponent with VALUE=DATE EXDATE: got nil error, want non-nil")
	}
	if !errors.Is(err, ErrUnsupportedRRule) {
		t.Errorf("error %v, want ErrUnsupportedRRule", err)
	}
}

// TestSetFromComponentTZIDUnsupported: a TZID-tagged EXDATE needs
// the VTIMEZONE registry (Calendar-level), unavailable to a
// Component-scoped constructor.
func TestSetFromComponentTZIDUnsupported(t *testing.T) {
	c := vstar.Component{
		Type: "VEVENT",
		Props: []vstar.Property{
			{Name: "DTSTART", Value: "20260401T120000Z"},
			{Name: "RRULE", Value: "FREQ=DAILY;COUNT=4"},
			{
				Name:   "EXDATE",
				Params: []vstar.Param{{Name: "TZID", Value: "America/New_York"}},
				Value:  "20260402T080000",
			},
		},
	}
	_, err := SetFromComponent(c)
	if err == nil {
		t.Fatalf("SetFromComponent with TZID EXDATE: got nil error, want non-nil")
	}
	if !errors.Is(err, ErrUnsupportedRRule) {
		t.Errorf("error %v, want ErrUnsupportedRRule", err)
	}
}

// ── RECURRENCE-ID ─────────────────────────────────────────────────

// TestParseRecurrenceID covers typed access to the RECURRENCE-ID
// property, including the RANGE parameter.
func TestParseRecurrenceID(t *testing.T) {
	cases := []struct {
		name      string
		prop      vstar.Property
		wantTime  string
		wantRange Range
		wantErr   bool
	}{
		{
			name:      "plain",
			prop:      vstar.Property{Name: "RECURRENCE-ID", Value: "20260403T120000Z"},
			wantTime:  "20260403T120000Z",
			wantRange: RangeThisInstance,
		},
		{
			name: "range_thisandfuture",
			prop: vstar.Property{
				Name:   "RECURRENCE-ID",
				Params: []vstar.Param{{Name: "RANGE", Value: "THISANDFUTURE"}},
				Value:  "20260403T120000Z",
			},
			wantTime:  "20260403T120000Z",
			wantRange: RangeThisAndFuture,
		},
		{
			name:    "wrong_property",
			prop:    vstar.Property{Name: "DTSTART", Value: "20260403T120000Z"},
			wantErr: true,
		},
		{
			name:    "malformed_value",
			prop:    vstar.Property{Name: "RECURRENCE-ID", Value: "nope"},
			wantErr: true,
		},
		{
			name: "unknown_range",
			prop: vstar.Property{
				Name:   "RECURRENCE-ID",
				Params: []vstar.Param{{Name: "RANGE", Value: "SOMETHING"}},
				Value:  "20260403T120000Z",
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRecurrenceID(tc.prop)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseRecurrenceID: got nil error, want non-nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRecurrenceID = %v, want nil", err)
			}
			if !eqStrs(utcs([]time.Time{got.Time}), []string{tc.wantTime}) {
				t.Errorf("Time = %v, want %v", utcs([]time.Time{got.Time}), tc.wantTime)
			}
			if got.Range != tc.wantRange {
				t.Errorf("Range = %v, want %v", got.Range, tc.wantRange)
			}
		})
	}
}

// TestRecurrenceIDProperty round-trips a RecurrenceID back to
// wire form.
func TestRecurrenceIDProperty(t *testing.T) {
	rid := RecurrenceID{Time: mustParseUTC("20260403T120000Z"), Range: RangeThisAndFuture}
	p := rid.Property()
	if p.Name != "RECURRENCE-ID" {
		t.Errorf("Name = %q, want RECURRENCE-ID", p.Name)
	}
	if p.Value != "20260403T120000Z" {
		t.Errorf("Value = %q, want 20260403T120000Z", p.Value)
	}
	back, err := ParseRecurrenceID(p)
	if err != nil {
		t.Fatalf("round-trip ParseRecurrenceID: %v", err)
	}
	if !back.Time.Equal(rid.Time) || back.Range != rid.Range {
		t.Errorf("round-trip = %+v, want %+v", back, rid)
	}
}

// ── helpers ───────────────────────────────────────────────────────

func ptrRule(r Rule) *Rule { return &r }

func mustTimes(t *testing.T, ss ...string) []time.Time {
	t.Helper()
	out := make([]time.Time, 0, len(ss))
	for _, s := range ss {
		out = append(out, mustParseUTC(s))
	}
	return out
}

// TestSetOccurrencesTruncates pins the Set-level complete flag:
// an unbounded rule truncated by the limit reports complete=false.
func TestSetOccurrencesTruncates(t *testing.T) {
	set := Set{
		DTStart: mustParseUTC("20260401T120000Z"),
		RRule:   ptrRule(mustParse(t, "FREQ=DAILY")),
	}
	got, complete, err := set.Occurrences(2)
	if err != nil {
		t.Fatalf("Set.Occurrences: %v", err)
	}
	want := []string{"20260401T120000Z", "20260402T120000Z"}
	if !eqStrs(utcs(got), want) {
		t.Errorf("= %v, want %v", utcs(got), want)
	}
	if complete {
		t.Errorf("complete = true, want false (limit truncated an infinite rule)")
	}
}

// TestSetOccurrencesExclusionsDoNotConsumeLimit: EXDATE removals
// must not count against the limit, or a heavily-excluded series
// would return short.
func TestSetOccurrencesExclusionsDoNotConsumeLimit(t *testing.T) {
	set := Set{
		DTStart: mustParseUTC("20260401T120000Z"),
		RRule:   ptrRule(mustParse(t, "FREQ=DAILY")),
		ExDate:  mustTimes(t, "20260402T120000Z", "20260403T120000Z"),
	}
	got, _, err := set.Occurrences(2)
	if err != nil {
		t.Fatalf("Set.Occurrences: %v", err)
	}
	want := []string{"20260401T120000Z", "20260404T120000Z"}
	if !eqStrs(utcs(got), want) {
		t.Errorf("= %v, want %v", utcs(got), want)
	}
}

// TestFormatDateTimeList round-trips an EXDATE/RDATE value,
// normalizing order and duplicates.
func TestFormatDateTimeList(t *testing.T) {
	in := mustTimes(t, "20260403T120000Z", "20260402T120000Z", "20260403T120000Z")
	got := FormatDateTimeList(in)
	want := "20260402T120000Z,20260403T120000Z"
	if got != want {
		t.Errorf("FormatDateTimeList = %q, want %q", got, want)
	}
	back, err := ParseDateTimeList(got)
	if err != nil {
		t.Fatalf("round-trip ParseDateTimeList: %v", err)
	}
	if !eqStrs(utcs(back), []string{"20260402T120000Z", "20260403T120000Z"}) {
		t.Errorf("round-trip = %v", utcs(back))
	}
	if FormatDateTimeList(nil) != "" {
		t.Errorf("FormatDateTimeList(nil) = %q, want \"\"", FormatDateTimeList(nil))
	}
}

// ── Iteration cap through a Set ───────────────────────────────────

// TestSetOccurrencesReportsIterationCap: RDATE additions do not turn
// a starved rule into a finite series; the cap propagates.
func TestSetOccurrencesReportsIterationCap(t *testing.T) {
	set := Set{
		DTStart: mustParseUTC("20260101T090000Z"),
		RRule:   ptrRule(mustParse(t, starvedRule)),
		RDate:   mustTimes(t, "20260201T090000Z"),
	}
	got, complete, err := set.Occurrences(5)
	if !errors.Is(err, ErrIterationCap) {
		t.Fatalf("Set.Occurrences(starved) = (%v, %v, %v), want ErrIterationCap", utcs(got), complete, err)
	}
	if complete {
		t.Errorf("complete = true alongside ErrIterationCap")
	}
}

// TestSetBetweenReportsIterationCap mirrors the package-level
// Between contract through a Set.
func TestSetBetweenReportsIterationCap(t *testing.T) {
	set := Set{
		DTStart: mustParseUTC("20260101T090000Z"),
		RRule:   ptrRule(mustParse(t, starvedRule)),
	}
	_, err := set.Between(mustParseUTC("20260101T090000Z"), mustParseUTC("20270101T090000Z"))
	if !errors.Is(err, ErrIterationCap) {
		t.Fatalf("Set.Between(starved) error = %v, want ErrIterationCap", err)
	}
}
