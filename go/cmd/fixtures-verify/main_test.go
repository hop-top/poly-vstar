// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeRRuleFixture lays out <stem>.rrule plus the given sidecars
// (keyed by suffix, e.g. ".next.json") in dir and returns the
// .rrule path, ready for verifyOneRRuleFixture.
func writeRRuleFixture(t *testing.T, dir, stem, rule string, sidecars map[string]string) string {
	t.Helper()
	rrulePath := filepath.Join(dir, stem+".rrule")
	if err := os.WriteFile(rrulePath, []byte(rule+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for suffix, body := range sidecars {
		if err := os.WriteFile(filepath.Join(dir, stem+suffix), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return rrulePath
}

// TestVerifyNextJSONErrorClass pins the optional "error" field of a
// .next.json sidecar: once every "expected" step has been yielded,
// the following NextOccurrence call MUST fail with the named class.
// Without the field the sidecar keeps its original contract (the
// expected steps only).
func TestVerifyNextJSONErrorClass(t *testing.T) {
	const anchor = `"dtstart":"20260101T090000Z","after":"20260101T090000Z"`
	cases := []struct {
		name    string
		rule    string
		next    string
		wantErr bool
	}{
		{
			name: "cap_hit_matches_named_class",
			rule: "FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30",
			next: `{` + anchor + `,"expected":[],"error":"ErrIterationCap"}`,
		},
		{
			name:    "terminated_rule_does_not_satisfy_named_class",
			rule:    "FREQ=DAILY;COUNT=1",
			next:    `{` + anchor + `,"expected":[],"error":"ErrIterationCap"}`,
			wantErr: true,
		},
		{
			name:    "yielding_rule_does_not_satisfy_named_class",
			rule:    "FREQ=DAILY",
			next:    `{` + anchor + `,"expected":[],"error":"ErrIterationCap"}`,
			wantErr: true,
		},
		{
			name:    "unknown_class_token_is_rejected",
			rule:    "FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30",
			next:    `{` + anchor + `,"expected":[],"error":"ErrBogus"}`,
			wantErr: true,
		},
		{
			name:    "expected_steps_still_checked_before_the_error",
			rule:    "FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30",
			next:    `{` + anchor + `,"expected":["20270101T090000Z"],"error":"ErrIterationCap"}`,
			wantErr: true,
		},
		{
			name: "no_error_field_keeps_the_step_only_contract",
			rule: "FREQ=DAILY;COUNT=2",
			next: `{` + anchor + `,"expected":["20260102T090000Z"]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeRRuleFixture(t, t.TempDir(), "case", tc.rule, map[string]string{".next.json": tc.next})
			err := verifyOneRRuleFixture(path)
			if tc.wantErr && err == nil {
				t.Fatalf("verifyOneRRuleFixture: got nil error, want a verification failure")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("verifyOneRRuleFixture: %v", err)
			}
		})
	}
}

// TestVerifyExpectJSONSentinel keeps the .expect.json contract:
// the named class must be the one ValidateRRule wraps.
func TestVerifyExpectJSONSentinel(t *testing.T) {
	cases := []struct {
		name    string
		rule    string
		expect  string
		wantErr bool
	}{
		{name: "unsupported_matches", rule: "FREQ=SECONDLY", expect: `{"sentinel":"ErrUnsupportedRRule"}`},
		{name: "malformed_matches", rule: "FREQ=DAILY;INTERVAL=0", expect: `{"sentinel":"ErrMalformed"}`},
		{name: "wrong_class_fails", rule: "FREQ=SECONDLY", expect: `{"sentinel":"ErrMalformed"}`, wantErr: true},
		{name: "valid_rule_fails", rule: "FREQ=DAILY", expect: `{"sentinel":"ErrMalformed"}`, wantErr: true},
		{name: "unknown_token_fails", rule: "FREQ=SECONDLY", expect: `{"sentinel":"ErrBogus"}`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeRRuleFixture(t, t.TempDir(), "case", tc.rule, map[string]string{".expect.json": tc.expect})
			err := verifyOneRRuleFixture(path)
			if tc.wantErr && err == nil {
				t.Fatalf("verifyOneRRuleFixture: got nil error, want a verification failure")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("verifyOneRRuleFixture: %v", err)
			}
		})
	}
}

// TestVerifyFormatted pins the .formatted sidecar: the parsed rule
// rendered through Rule.String must equal the file byte-for-byte
// (trailing newline aside).
func TestVerifyFormatted(t *testing.T) {
	cases := []struct {
		name      string
		rule      string
		formatted string
		wantErr   bool
	}{
		{name: "scrambled_parts_reordered", rule: "BYDAY=MO,WE;INTERVAL=2;FREQ=WEEKLY;WKST=MO", formatted: "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE\n"},
		{name: "defaults_elided", rule: "FREQ=DAILY;INTERVAL=1;WKST=MO", formatted: "FREQ=DAILY\n"},
		{name: "mismatch_fails", rule: "FREQ=DAILY", formatted: "FREQ=DAILY;INTERVAL=1\n", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeRRuleFixture(t, t.TempDir(), "case", tc.rule, map[string]string{".formatted": tc.formatted})
			checkVerify(t, verifyOneRRuleFixture(path), tc.wantErr)
		})
	}
}

// TestVerifyExpandJSON pins the .expand.json sidecar: Occurrences
// with the given limit must yield the expected list AND the stated
// complete flag; an "error" field instead names the failure class
// the call must wrap.
func TestVerifyExpandJSON(t *testing.T) {
	const dtstart = `"dtstart":"20260401T120000Z"`
	cases := []struct {
		name    string
		rule    string
		expand  string
		wantErr bool
	}{
		{
			name:   "count_completes_within_limit",
			rule:   "FREQ=DAILY;COUNT=3",
			expand: `{` + dtstart + `,"limit":10,"expected":["20260401T120000Z","20260402T120000Z","20260403T120000Z"],"complete":true}`,
		},
		{
			name:   "unbounded_truncated_by_limit",
			rule:   "FREQ=DAILY",
			expand: `{` + dtstart + `,"limit":2,"expected":["20260401T120000Z","20260402T120000Z"],"complete":false}`,
		},
		{
			name:    "truncation_misreported_as_complete_fails",
			rule:    "FREQ=DAILY",
			expand:  `{` + dtstart + `,"limit":2,"expected":["20260401T120000Z","20260402T120000Z"],"complete":true}`,
			wantErr: true,
		},
		{
			name:    "wrong_occurrence_fails",
			rule:    "FREQ=DAILY;COUNT=2",
			expand:  `{` + dtstart + `,"limit":10,"expected":["20260401T120000Z","20260403T120000Z"],"complete":true}`,
			wantErr: true,
		},
		{
			name:   "error_class_matches",
			rule:   "FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30",
			expand: `{"dtstart":"20260101T090000Z","limit":5,"error":"ErrIterationCap"}`,
		},
		{
			name:    "error_class_unmet_fails",
			rule:    "FREQ=DAILY;COUNT=1",
			expand:  `{` + dtstart + `,"limit":5,"error":"ErrIterationCap"}`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeRRuleFixture(t, t.TempDir(), "case", tc.rule, map[string]string{".expand.json": tc.expand})
			checkVerify(t, verifyOneRRuleFixture(path), tc.wantErr)
		})
	}
}

// TestVerifyBetweenJSON pins the .between.json sidecar: Between over
// the half-open [start, end) window must yield the expected list.
func TestVerifyBetweenJSON(t *testing.T) {
	const dtstart = `"dtstart":"20260401T120000Z"`
	cases := []struct {
		name    string
		rule    string
		between string
		wantErr bool
	}{
		{
			name:    "window_matches",
			rule:    "FREQ=DAILY",
			between: `{` + dtstart + `,"start":"20260403T120000Z","end":"20260406T120000Z","expected":["20260403T120000Z","20260404T120000Z","20260405T120000Z"]}`,
		},
		{
			name:    "end_inclusive_claim_fails",
			rule:    "FREQ=DAILY",
			between: `{` + dtstart + `,"start":"20260403T120000Z","end":"20260406T120000Z","expected":["20260403T120000Z","20260404T120000Z","20260405T120000Z","20260406T120000Z"]}`,
			wantErr: true,
		},
		{
			name:    "inverted_window_names_its_class",
			rule:    "FREQ=DAILY",
			between: `{` + dtstart + `,"start":"20260406T120000Z","end":"20260403T120000Z","error":"ErrUnboundedExpansion"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeRRuleFixture(t, t.TempDir(), "case", tc.rule, map[string]string{".between.json": tc.between})
			checkVerify(t, verifyOneRRuleFixture(path), tc.wantErr)
		})
	}
}

const setICS = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//V*//Set//EN
BEGIN:VEVENT
UID:set-1
DTSTAMP:20260101T000000Z
DTSTART:20260401T120000Z
RRULE:FREQ=DAILY;COUNT=4
RDATE:20260410T080000Z
EXDATE:20260402T120000Z
END:VEVENT
END:VCALENDAR
`

const setTZIDICS = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//V*//Set//EN
BEGIN:VEVENT
UID:set-2
DTSTAMP:20260101T000000Z
DTSTART:20260401T120000Z
RRULE:FREQ=DAILY;COUNT=4
EXDATE;TZID=America/New_York:20260402T080000
END:VEVENT
END:VCALENDAR
`

// writeSetFixture lays out <stem>.ics plus sidecars and returns the
// .ics path, ready for verifyOneSetFixture.
func writeSetFixture(t *testing.T, dir, stem, ics string, sidecars map[string]string) string {
	t.Helper()
	icsPath := filepath.Join(dir, stem+".ics")
	if err := os.WriteFile(icsPath, []byte(ics), 0o644); err != nil {
		t.Fatal(err)
	}
	for suffix, body := range sidecars {
		if err := os.WriteFile(filepath.Join(dir, stem+suffix), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return icsPath
}

// TestVerifySetFixture pins the recurrence-set fixture class: the
// first component of <name>.ics goes through SetFromComponent, and
// either <name>.occurrences.json states the bounded expansion or
// <name>.expect.json names the class the constructor must wrap.
func TestVerifySetFixture(t *testing.T) {
	cases := []struct {
		name     string
		ics      string
		sidecars map[string]string
		wantErr  bool
	}{
		{
			name:     "occurrences_match",
			ics:      setICS,
			sidecars: map[string]string{".occurrences.json": `{"limit":10,"expected":["20260401T120000Z","20260403T120000Z","20260404T120000Z","20260410T080000Z"],"complete":true}`},
		},
		{
			name:     "exdate_ignored_claim_fails",
			ics:      setICS,
			sidecars: map[string]string{".occurrences.json": `{"limit":10,"expected":["20260401T120000Z","20260402T120000Z","20260403T120000Z","20260404T120000Z","20260410T080000Z"],"complete":true}`},
			wantErr:  true,
		},
		{
			name:     "wrong_complete_fails",
			ics:      setICS,
			sidecars: map[string]string{".occurrences.json": `{"limit":2,"expected":["20260401T120000Z","20260403T120000Z"],"complete":true}`},
			wantErr:  true,
		},
		{
			name:     "rejected_tzid_matches_class",
			ics:      setTZIDICS,
			sidecars: map[string]string{".expect.json": `{"sentinel":"ErrUnsupportedRRule"}`},
		},
		{
			name:     "rejected_claim_on_valid_set_fails",
			ics:      setICS,
			sidecars: map[string]string{".expect.json": `{"sentinel":"ErrUnsupportedRRule"}`},
			wantErr:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeSetFixture(t, t.TempDir(), "case", tc.ics, tc.sidecars)
			checkVerify(t, verifyOneSetFixture(path), tc.wantErr)
		})
	}
}

func checkVerify(t *testing.T, err error, wantErr bool) {
	t.Helper()
	if wantErr && err == nil {
		t.Fatalf("got nil error, want a verification failure")
	}
	if !wantErr && err != nil {
		t.Fatalf("unexpected verification failure: %v", err)
	}
}
