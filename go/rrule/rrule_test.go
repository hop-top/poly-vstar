// SPDX-License-Identifier: MIT

package rrule

import (
	"errors"
	"testing"
	"time"

	vstar "hop.top/vstar"
)

// TestRule_FieldShapes asserts the Rule struct exposes the
// fields the v0.2 RRULE surface (spec/03 §RRULE parsing scope)
// mandates with the right
// types. This is a compile-time-ish check via assignment.
func TestRule_FieldShapes(t *testing.T) {
	r := Rule{
		Freq:       FreqDaily,
		Interval:   2,
		Until:      time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Count:      10,
		ByDay:      []ByDay{{Ordinal: -1, Weekday: SU}},
		ByMonth:    []int{1, 6, 12},
		ByMonthDay: []int{-1, 1, 15},
		ByHour:     []int{9, 17},
		ByMinute:   []int{0, 30},
		BySecond:   []int{0},
		WeekStart:  MO,
	}
	if r.Freq != FreqDaily {
		t.Fatalf("Freq round-trip: got %v", r.Freq)
	}
	if r.Interval != 2 {
		t.Fatalf("Interval: got %d", r.Interval)
	}
	if len(r.ByDay) != 1 || r.ByDay[0].Ordinal != -1 || r.ByDay[0].Weekday != SU {
		t.Fatalf("ByDay: got %+v", r.ByDay)
	}
	if r.WeekStart != MO {
		t.Fatalf("WeekStart: got %v", r.WeekStart)
	}
}

// TestFreq_ZeroValueInvalid pins the iota+1 contract: the zero
// value of Freq is not a valid frequency, so callers can detect
// "unset" by checking r.Freq == 0 or r.Freq == FreqInvalid.
func TestFreq_ZeroValueInvalid(t *testing.T) {
	var f Freq
	if f != FreqInvalid {
		t.Fatalf("zero Freq expected FreqInvalid (0), got %v", f)
	}
	if f == FreqHourly {
		t.Fatalf("zero Freq must NOT equal FreqHourly")
	}
}

// TestFreq_String covers every Freq value's String() rendering.
func TestFreq_String(t *testing.T) {
	cases := []struct {
		f    Freq
		want string
	}{
		{FreqInvalid, "INVALID"},
		{FreqMinutely, "MINUTELY"},
		{FreqHourly, "HOURLY"},
		{FreqDaily, "DAILY"},
		{FreqWeekly, "WEEKLY"},
		{FreqMonthly, "MONTHLY"},
		{FreqYearly, "YEARLY"},
		{Freq(99), "Freq(99)"},
	}
	for _, c := range cases {
		if got := c.f.String(); got != c.want {
			t.Errorf("Freq(%d).String() = %q, want %q", int(c.f), got, c.want)
		}
	}
}

// TestWeekday_String covers RFC 5545 §3.3.10 weekday symbols.
func TestWeekday_String(t *testing.T) {
	cases := []struct {
		w    Weekday
		want string
	}{
		{SU, "SU"},
		{MO, "MO"},
		{TU, "TU"},
		{WE, "WE"},
		{TH, "TH"},
		{FR, "FR"},
		{SA, "SA"},
		{Weekday(99), "Weekday(99)"},
	}
	for _, c := range cases {
		if got := c.w.String(); got != c.want {
			t.Errorf("Weekday(%d).String() = %q, want %q", int(c.w), got, c.want)
		}
	}
}

// TestWeekday_RFCNumbering pins SU=0..SA=6 per RFC 5545 §3.3.10.
func TestWeekday_RFCNumbering(t *testing.T) {
	if SU != 0 || MO != 1 || TU != 2 || WE != 3 || TH != 4 || FR != 5 || SA != 6 {
		t.Fatalf("Weekday numbering broken: SU=%d MO=%d TU=%d WE=%d TH=%d FR=%d SA=%d",
			SU, MO, TU, WE, TH, FR, SA)
	}
}

// TestSentinels_NonNilDistinct asserts ErrUnsupportedRRule and
// vstar.ErrMalformed are both non-nil and distinct sentinel
// values. (T1 — kept here so T0's package compiles standalone.)
func TestSentinels_NonNilDistinct(t *testing.T) {
	if ErrUnsupportedRRule == nil {
		t.Fatal("ErrUnsupportedRRule must not be nil")
	}
	if vstar.ErrMalformed == nil {
		t.Fatal("vstar.ErrMalformed must not be nil")
	}
	if errors.Is(ErrUnsupportedRRule, vstar.ErrMalformed) {
		t.Fatal("ErrUnsupportedRRule must NOT be ErrMalformed")
	}
	// The reverse direction is symmetric for sentinel errors but
	// staticcheck flags the literal call as "wrong arg order"
	// (target should be the second arg) — we instead check both
	// errors are distinct values (pointer-distinct since they are
	// errors.New results).
	if ErrUnsupportedRRule == vstar.ErrMalformed { //nolint:errorlint
		t.Fatal("sentinels must be distinct values")
	}
}
