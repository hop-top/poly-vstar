// SPDX-License-Identifier: MIT

package rrule

import (
	"reflect"
	"testing"
)

// TestRuleStringFixedOrder pins the canonical rule-part order.
// RFC 5545 §3.3.10 imposes no order; spec/03 canonicalization
// requires byte-identical output for identical logical content, so
// the order is fixed here and must not drift.
func TestRuleStringFixedOrder(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "freq_only",
			in:   "FREQ=DAILY",
			want: "FREQ=DAILY",
		},
		{
			name: "default_interval_omitted",
			in:   "FREQ=DAILY;INTERVAL=1",
			want: "FREQ=DAILY",
		},
		{
			name: "default_wkst_omitted",
			in:   "FREQ=WEEKLY;WKST=MO",
			want: "FREQ=WEEKLY",
		},
		{
			name: "non_default_wkst_emitted_last",
			in:   "FREQ=WEEKLY;WKST=SU;BYDAY=MO",
			want: "FREQ=WEEKLY;BYDAY=MO;WKST=SU",
		},
		{
			name: "input_order_normalised",
			in:   "BYMONTH=3;COUNT=5;FREQ=MONTHLY;INTERVAL=2",
			want: "FREQ=MONTHLY;INTERVAL=2;COUNT=5;BYMONTH=3",
		},
		{
			name: "until_before_by_clauses",
			in:   "BYDAY=MO;UNTIL=20261231T235959Z;FREQ=WEEKLY",
			want: "FREQ=WEEKLY;UNTIL=20261231T235959Z;BYDAY=MO",
		},
		{
			name: "all_by_clauses_in_rfc_listing_order",
			in:   "FREQ=YEARLY;BYSETPOS=1;BYSECOND=30;BYMINUTE=15;BYHOUR=9;BYDAY=MO;BYMONTHDAY=15;BYYEARDAY=100;BYWEEKNO=20;BYMONTH=4",
			want: "FREQ=YEARLY;BYMONTH=4;BYWEEKNO=20;BYYEARDAY=100;BYMONTHDAY=15;BYDAY=MO;BYHOUR=9;BYMINUTE=15;BYSECOND=30;BYSETPOS=1",
		},
		{
			name: "byday_ordinals",
			in:   "FREQ=MONTHLY;BYDAY=-1FR,2TU,SU",
			want: "FREQ=MONTHLY;BYDAY=-1FR,2TU,SU",
		},
		{
			name: "negative_bymonthday",
			in:   "FREQ=MONTHLY;BYMONTHDAY=-1,15",
			want: "FREQ=MONTHLY;BYMONTHDAY=-1,15",
		},
		{
			name: "count_and_interval",
			in:   "FREQ=HOURLY;INTERVAL=6;COUNT=12",
			want: "FREQ=HOURLY;INTERVAL=6;COUNT=12",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := mustParse(t, tc.in)
			if got := r.String(); got != tc.want {
				t.Errorf("Rule.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRuleStringRoundTrip: String → ParseRRule → String is stable,
// and the re-parsed Rule is deep-equal to the original. Covers
// every supported BY-part.
func TestRuleStringRoundTrip(t *testing.T) {
	inputs := []string{
		"FREQ=HOURLY",
		"FREQ=DAILY;INTERVAL=2;COUNT=10",
		"FREQ=DAILY;UNTIL=20261231T235959Z",
		"FREQ=WEEKLY;BYDAY=MO,WE,FR;WKST=SU",
		"FREQ=WEEKLY;INTERVAL=3;BYDAY=TH",
		"FREQ=MONTHLY;BYMONTHDAY=-1",
		"FREQ=MONTHLY;BYDAY=2TU",
		"FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1",
		"FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=29",
		"FREQ=YEARLY;BYYEARDAY=1,-1",
		"FREQ=YEARLY;BYWEEKNO=20,-1;BYDAY=MO",
		"FREQ=DAILY;BYHOUR=9,17;BYMINUTE=0,30;BYSECOND=0",
		"FREQ=YEARLY;BYMONTH=4;BYWEEKNO=20;BYYEARDAY=100;BYMONTHDAY=15;BYDAY=MO;BYHOUR=9;BYMINUTE=15;BYSECOND=30;BYSETPOS=1",
	}

	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			r1 := mustParse(t, in)
			s1 := r1.String()
			r2, err := ParseRRule(s1)
			if err != nil {
				t.Fatalf("ParseRRule(%q) (round 2) = %v, want nil", s1, err)
			}
			if !reflect.DeepEqual(r1, r2) {
				t.Errorf("round-trip Rule mismatch:\n first: %+v\nsecond: %+v", r1, r2)
			}
			if s2 := r2.String(); s2 != s1 {
				t.Errorf("String not idempotent: %q then %q", s1, s2)
			}
		})
	}
}

// TestRuleStringInvalid: a Rule that cannot produce a valid RRULE
// value renders the empty string rather than partial garbage.
func TestRuleStringInvalid(t *testing.T) {
	if got := (Rule{}).String(); got != "" {
		t.Errorf("Rule{}.String() = %q, want \"\"", got)
	}
}

// TestRuleProperty renders a Rule as a complete RRULE property.
func TestRuleProperty(t *testing.T) {
	p := mustParse(t, "FREQ=WEEKLY;BYDAY=MO").Property()
	if p.Name != "RRULE" {
		t.Errorf("Name = %q, want RRULE", p.Name)
	}
	if p.Value != "FREQ=WEEKLY;BYDAY=MO" {
		t.Errorf("Value = %q", p.Value)
	}
	if got := (Rule{}).Property(); got.Name != "" {
		t.Errorf("Rule{}.Property() = %+v, want zero Property", got)
	}
}
