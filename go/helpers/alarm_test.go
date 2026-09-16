// SPDX-License-Identifier: MIT

package helpers_test

import (
	"errors"
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/duration"
	"hop.top/vstar/helpers"
)

// TestNewRelativeAlarm builds the overwhelmingly common alarm shape
// — fire N minutes before the parent starts — without the caller
// hand-assembling a wire string.
func TestNewRelativeAlarm(t *testing.T) {
	d := duration.Duration{Negative: true, Minutes: 15}
	c, err := helpers.NewRelativeAlarm("alarm-1", "DISPLAY", d, duration.RelatedStart)
	if err != nil {
		t.Fatalf("NewRelativeAlarm: %v", err)
	}
	if c.Type != vstar.CompAlarm {
		t.Fatalf("Type = %q, want VALARM", c.Type)
	}
	p, ok := c.Get("TRIGGER")
	if !ok {
		t.Fatal("no TRIGGER property")
	}
	if p.Value != "-PT15M" {
		t.Fatalf("TRIGGER = %q, want -PT15M", p.Value)
	}
	// RELATED=START is the RFC default and must not be emitted.
	if len(p.Params) != 0 {
		t.Fatalf("TRIGGER params = %+v, want none for the default anchor", p.Params)
	}
	if a, ok := c.Get("ACTION"); !ok || a.Value != "DISPLAY" {
		t.Fatalf("ACTION = %+v, want DISPLAY", a)
	}
	for _, name := range []string{"UID", "DTSTAMP", "X-VSTAR-HASH"} {
		if _, ok := c.Get(name); !ok {
			t.Fatalf("missing required property %s", name)
		}
	}
	// The constructed trigger reads back through the typed API.
	tr, err := duration.AlarmTrigger(c)
	if err != nil {
		t.Fatalf("AlarmTrigger: %v", err)
	}
	if !tr.Relative || tr.Related != duration.RelatedStart || tr.Duration != d {
		t.Fatalf("round-trip trigger = %+v, want relative %+v at START", tr, d)
	}
}

// TestNewRelativeAlarm_EndAnchor asserts RELATED=END is emitted
// when the caller selects the end anchor.
func TestNewRelativeAlarm_EndAnchor(t *testing.T) {
	d := duration.Duration{Minutes: 10}
	c, err := helpers.NewRelativeAlarm("alarm-2", "DISPLAY", d, duration.RelatedEnd)
	if err != nil {
		t.Fatalf("NewRelativeAlarm: %v", err)
	}
	p, ok := c.Get("TRIGGER")
	if !ok {
		t.Fatal("no TRIGGER property")
	}
	if p.Value != "PT10M" {
		t.Fatalf("TRIGGER = %q, want PT10M", p.Value)
	}
	if len(p.Params) != 1 || p.Params[0].Name != "RELATED" || p.Params[0].Value != "END" {
		t.Fatalf("TRIGGER params = %+v, want RELATED=END", p.Params)
	}
}

// TestNewAbsoluteAlarm builds the absolute form, which must carry
// VALUE=DATE-TIME so consumers do not guess.
func TestNewAbsoluteAlarm(t *testing.T) {
	at := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	c, err := helpers.NewAbsoluteAlarm("alarm-3", "EMAIL", at)
	if err != nil {
		t.Fatalf("NewAbsoluteAlarm: %v", err)
	}
	p, ok := c.Get("TRIGGER")
	if !ok {
		t.Fatal("no TRIGGER property")
	}
	if p.Value != "20260601T083000Z" {
		t.Fatalf("TRIGGER = %q, want 20260601T083000Z", p.Value)
	}
	if len(p.Params) != 1 || p.Params[0].Name != "VALUE" || p.Params[0].Value != "DATE-TIME" {
		t.Fatalf("TRIGGER params = %+v, want VALUE=DATE-TIME", p.Params)
	}
	tr, err := duration.AlarmTrigger(c)
	if err != nil {
		t.Fatalf("AlarmTrigger: %v", err)
	}
	if tr.Relative || !tr.Absolute.Equal(at) {
		t.Fatalf("round-trip trigger = %+v, want absolute %v", tr, at)
	}
}

// TestTypedAlarms_RequireUID asserts the typed constructors keep
// NewAlarm's UID discipline.
func TestTypedAlarms_RequireUID(t *testing.T) {
	if _, err := helpers.NewRelativeAlarm("", "DISPLAY", duration.Duration{Minutes: 5}, duration.RelatedStart); !errors.Is(err, vstar.ErrMissingUID) {
		t.Fatalf("NewRelativeAlarm error = %v, want ErrMissingUID", err)
	}
	if _, err := helpers.NewAbsoluteAlarm("", "DISPLAY", time.Now()); !errors.Is(err, vstar.ErrMissingUID) {
		t.Fatalf("NewAbsoluteAlarm error = %v, want ErrMissingUID", err)
	}
}

// TestNewAlarm_Unchanged pins the existing raw-string constructor:
// this change must not alter its behavior.
func TestNewAlarm_Unchanged(t *testing.T) {
	c, err := helpers.NewAlarm("alarm-legacy", "DISPLAY", "-PT15M")
	if err != nil {
		t.Fatalf("NewAlarm: %v", err)
	}
	p, ok := c.Get("TRIGGER")
	if !ok || p.Value != "-PT15M" || len(p.Params) != 0 {
		t.Fatalf("TRIGGER = %+v, want bare -PT15M", p)
	}
}

// TestAlarmFiresAt resolves a helper-built alarm against its parent
// event end to end.
func TestAlarmFiresAt(t *testing.T) {
	start := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	evt, err := helpers.NewEvent("evt-1", start, start.Add(time.Hour))
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	alrm, err := helpers.NewRelativeAlarm("alarm-1", "DISPLAY",
		duration.Duration{Negative: true, Minutes: 15}, duration.RelatedStart)
	if err != nil {
		t.Fatalf("NewRelativeAlarm: %v", err)
	}
	evt.Sub = append(evt.Sub, alrm)

	got, err := helpers.AlarmFiresAt(alrm, evt, vstar.Calendar{})
	if err != nil {
		t.Fatalf("AlarmFiresAt: %v", err)
	}
	want := start.Add(-15 * time.Minute)
	if !got.Equal(want) {
		t.Fatalf("AlarmFiresAt = %v, want %v", got, want)
	}
}
