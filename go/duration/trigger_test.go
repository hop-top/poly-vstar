// SPDX-License-Identifier: MIT

package duration_test

import (
	"errors"
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/duration"
)

// alarm builds a VALARM carrying the supplied TRIGGER property.
func alarm(trigger vstar.Property) vstar.Component {
	return vstar.Component{
		Type: vstar.CompAlarm,
		Props: []vstar.Property{
			{Name: "ACTION", Value: "DISPLAY"},
			trigger,
		},
	}
}

// event builds a VEVENT with the supplied DTSTART/DTEND wire values.
// An empty value omits the property.
func event(dtstart, dtend string) vstar.Component {
	c := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "UID", Value: "evt-1"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
		},
	}
	if dtstart != "" {
		c.Add(vstar.Property{Name: "DTSTART", Value: dtstart})
	}
	if dtend != "" {
		c.Add(vstar.Property{Name: "DTEND", Value: dtend})
	}
	return c
}

// TestParseTrigger_RelativeDefaultAnchor asserts a bare DURATION
// TRIGGER is relative and defaults to RELATED=START per
// RFC 5545 §3.8.6.3. This is the "-PT15M" form Apple Reminders and
// Thunderbird emit.
func TestParseTrigger_RelativeDefaultAnchor(t *testing.T) {
	tr, err := duration.ParseTrigger(vstar.Property{Name: "TRIGGER", Value: "-PT15M"})
	if err != nil {
		t.Fatalf("ParseTrigger: %v", err)
	}
	if !tr.Relative {
		t.Fatal("Relative = false, want true")
	}
	if tr.Related != duration.RelatedStart {
		t.Fatalf("Related = %v, want RelatedStart (the RFC default)", tr.Related)
	}
	want := duration.Duration{Negative: true, Minutes: 15}
	if tr.Duration != want {
		t.Fatalf("Duration = %+v, want %+v", tr.Duration, want)
	}
	if !tr.Absolute.IsZero() {
		t.Fatalf("Absolute = %v, want zero for a relative trigger", tr.Absolute)
	}
}

// TestParseTrigger_RelatedEnd asserts RELATED=END selects the end
// anchor, and that the parameter name/value are matched
// case-insensitively per RFC 5545 §3.2.
func TestParseTrigger_RelatedEnd(t *testing.T) {
	cases := []struct {
		name  string
		param vstar.Param
	}{
		{"uppercase", vstar.Param{Name: "RELATED", Value: "END"}},
		{"lowercase name", vstar.Param{Name: "related", Value: "END"}},
		{"lowercase value", vstar.Param{Name: "RELATED", Value: "end"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr, err := duration.ParseTrigger(vstar.Property{
				Name:   "TRIGGER",
				Params: []vstar.Param{tc.param},
				Value:  "PT10M",
			})
			if err != nil {
				t.Fatalf("ParseTrigger: %v", err)
			}
			if !tr.Relative {
				t.Fatal("Relative = false, want true")
			}
			if tr.Related != duration.RelatedEnd {
				t.Fatalf("Related = %v, want RelatedEnd", tr.Related)
			}
		})
	}
}

// TestParseTrigger_ExplicitRelatedStart asserts RELATED=START is
// accepted explicitly, matching the implicit default.
func TestParseTrigger_ExplicitRelatedStart(t *testing.T) {
	tr, err := duration.ParseTrigger(vstar.Property{
		Name:   "TRIGGER",
		Params: []vstar.Param{{Name: "RELATED", Value: "START"}},
		Value:  "-PT5M",
	})
	if err != nil {
		t.Fatalf("ParseTrigger: %v", err)
	}
	if tr.Related != duration.RelatedStart {
		t.Fatalf("Related = %v, want RelatedStart", tr.Related)
	}
}

// TestParseTrigger_Absolute asserts VALUE=DATE-TIME yields an
// absolute trigger carrying a parsed instant and no duration.
func TestParseTrigger_Absolute(t *testing.T) {
	tr, err := duration.ParseTrigger(vstar.Property{
		Name:   "TRIGGER",
		Params: []vstar.Param{{Name: "VALUE", Value: "DATE-TIME"}},
		Value:  "20260601T083000Z",
	})
	if err != nil {
		t.Fatalf("ParseTrigger: %v", err)
	}
	if tr.Relative {
		t.Fatal("Relative = true, want false for VALUE=DATE-TIME")
	}
	want := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	if !tr.Absolute.Equal(want) {
		t.Fatalf("Absolute = %v, want %v", tr.Absolute, want)
	}
	if tr.Duration != (duration.Duration{}) {
		t.Fatalf("Duration = %+v, want zero for an absolute trigger", tr.Duration)
	}
}

// TestParseTrigger_AbsoluteWithoutValueParam asserts an
// absolute-looking value is read as absolute even when the
// VALUE=DATE-TIME parameter is omitted — producers in the wild
// leave it off, and the two value shapes are unambiguous.
func TestParseTrigger_AbsoluteWithoutValueParam(t *testing.T) {
	tr, err := duration.ParseTrigger(vstar.Property{Name: "TRIGGER", Value: "20260601T083000Z"})
	if err != nil {
		t.Fatalf("ParseTrigger: %v", err)
	}
	if tr.Relative {
		t.Fatal("Relative = true, want false for a DATE-TIME shaped value")
	}
	want := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	if !tr.Absolute.Equal(want) {
		t.Fatalf("Absolute = %v, want %v", tr.Absolute, want)
	}
}

// TestParseTrigger_ExplicitValueDuration asserts VALUE=DURATION is
// accepted as the explicit spelling of the relative form.
func TestParseTrigger_ExplicitValueDuration(t *testing.T) {
	tr, err := duration.ParseTrigger(vstar.Property{
		Name:   "TRIGGER",
		Params: []vstar.Param{{Name: "VALUE", Value: "DURATION"}},
		Value:  "-PT15M",
	})
	if err != nil {
		t.Fatalf("ParseTrigger: %v", err)
	}
	if !tr.Relative {
		t.Fatal("Relative = false, want true")
	}
}

// TestParseTrigger_Malformed asserts strictness: a value that is
// neither a valid DURATION nor a valid form #2 DATE-TIME is an
// error, as is a contradictory VALUE parameter.
func TestParseTrigger_Malformed(t *testing.T) {
	bad := []struct {
		name string
		prop vstar.Property
	}{
		{"empty value", vstar.Property{Name: "TRIGGER", Value: ""}},
		{"garbage", vstar.Property{Name: "TRIGGER", Value: "soon"}},
		{"bad duration", vstar.Property{Name: "TRIGGER", Value: "P1Y"}},
		{"bad datetime", vstar.Property{Name: "TRIGGER", Value: "2026-06-01T08:30:00Z"}},
		{
			"duration declared but datetime supplied",
			vstar.Property{
				Name:   "TRIGGER",
				Params: []vstar.Param{{Name: "VALUE", Value: "DURATION"}},
				Value:  "20260601T083000Z",
			},
		},
		{
			"datetime declared but duration supplied",
			vstar.Property{
				Name:   "TRIGGER",
				Params: []vstar.Param{{Name: "VALUE", Value: "DATE-TIME"}},
				Value:  "-PT15M",
			},
		},
		{
			"unknown related value",
			vstar.Property{
				Name:   "TRIGGER",
				Params: []vstar.Param{{Name: "RELATED", Value: "MIDDLE"}},
				Value:  "-PT15M",
			},
		},
		{
			"related on an absolute trigger",
			vstar.Property{
				Name:   "TRIGGER",
				Params: []vstar.Param{{Name: "RELATED", Value: "END"}},
				Value:  "20260601T083000Z",
			},
		},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			got, err := duration.ParseTrigger(tc.prop)
			if err == nil {
				t.Fatalf("ParseTrigger(%+v) = %+v, want error", tc.prop, got)
			}
			if !errors.Is(err, vstar.ErrMalformed) {
				t.Fatalf("error = %v, want errors.Is(vstar.ErrMalformed)", err)
			}
		})
	}
}

// TestAlarmTrigger reads the TRIGGER straight off a VALARM
// component, the shape consumers actually hold.
func TestAlarmTrigger(t *testing.T) {
	a := alarm(vstar.Property{Name: "TRIGGER", Value: "-PT15M"})
	tr, err := duration.AlarmTrigger(a)
	if err != nil {
		t.Fatalf("AlarmTrigger: %v", err)
	}
	if !tr.Relative || tr.Duration.Minutes != 15 || !tr.Duration.Negative {
		t.Fatalf("AlarmTrigger = %+v, want relative -PT15M", tr)
	}
}

// TestAlarmTrigger_Missing asserts a VALARM with no TRIGGER is a
// distinguishable condition, not a silent zero value.
func TestAlarmTrigger_Missing(t *testing.T) {
	a := vstar.Component{Type: vstar.CompAlarm}
	if _, err := duration.AlarmTrigger(a); !errors.Is(err, duration.ErrNoTrigger) {
		t.Fatalf("error = %v, want errors.Is(ErrNoTrigger)", err)
	}
}

// TestResolve_StartAnchor resolves a relative trigger against the
// parent's DTSTART — the "-PT15M fires 15 minutes before the event"
// case from the shipped fixture.
func TestResolve_StartAnchor(t *testing.T) {
	parent := event("20260601T090000Z", "20260601T100000Z")
	tr, err := duration.ParseTrigger(vstar.Property{Name: "TRIGGER", Value: "-PT15M"})
	if err != nil {
		t.Fatalf("ParseTrigger: %v", err)
	}
	got, err := tr.Resolve(parent, vstar.Calendar{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := time.Date(2026, 6, 1, 8, 45, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("Resolve = %v, want %v", got, want)
	}
}

// TestResolve_EndAnchor resolves against DTEND when RELATED=END.
func TestResolve_EndAnchor(t *testing.T) {
	parent := event("20260601T090000Z", "20260601T100000Z")
	tr, err := duration.ParseTrigger(vstar.Property{
		Name:   "TRIGGER",
		Params: []vstar.Param{{Name: "RELATED", Value: "END"}},
		Value:  "PT30M",
	})
	if err != nil {
		t.Fatalf("ParseTrigger: %v", err)
	}
	got, err := tr.Resolve(parent, vstar.Calendar{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("Resolve = %v, want %v", got, want)
	}
}

// TestResolve_EndAnchorFromDuration asserts a VEVENT carrying
// DURATION instead of DTEND still yields an END anchor, computed as
// DTSTART + DURATION per RFC 5545 §3.6.1.
func TestResolve_EndAnchorFromDuration(t *testing.T) {
	parent := event("20260601T090000Z", "")
	parent.Add(vstar.Property{Name: "DURATION", Value: "PT90M"})
	tr, err := duration.ParseTrigger(vstar.Property{
		Name:   "TRIGGER",
		Params: []vstar.Param{{Name: "RELATED", Value: "END"}},
		Value:  "-PT15M",
	})
	if err != nil {
		t.Fatalf("ParseTrigger: %v", err)
	}
	got, err := tr.Resolve(parent, vstar.Calendar{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// DTSTART 09:00 + PT90M = 10:30; minus 15m = 10:15.
	want := time.Date(2026, 6, 1, 10, 15, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("Resolve = %v, want %v", got, want)
	}
}

// TestResolve_VTodoDueAnchor asserts a VTODO resolves its END
// anchor from DUE per RFC 5545 §3.6.2.
func TestResolve_VTodoDueAnchor(t *testing.T) {
	parent := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "todo-1"},
			{Name: "DUE", Value: "20260601T170000Z"},
		},
	}
	tr, err := duration.ParseTrigger(vstar.Property{
		Name:   "TRIGGER",
		Params: []vstar.Param{{Name: "RELATED", Value: "END"}},
		Value:  "-PT1H",
	})
	if err != nil {
		t.Fatalf("ParseTrigger: %v", err)
	}
	got, err := tr.Resolve(parent, vstar.Calendar{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := time.Date(2026, 6, 1, 16, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("Resolve = %v, want %v", got, want)
	}
}

// TestResolve_Absolute returns the instant verbatim, ignoring the
// parent entirely.
func TestResolve_Absolute(t *testing.T) {
	tr, err := duration.ParseTrigger(vstar.Property{Name: "TRIGGER", Value: "20260601T083000Z"})
	if err != nil {
		t.Fatalf("ParseTrigger: %v", err)
	}
	got, err := tr.Resolve(vstar.Component{}, vstar.Calendar{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("Resolve = %v, want %v", got, want)
	}
}

// TestResolve_MissingAnchor asserts a relative trigger whose parent
// lacks the required anchor reports ErrNoAnchor rather than
// silently resolving against the zero time.
func TestResolve_MissingAnchor(t *testing.T) {
	tests := []struct {
		name    string
		parent  vstar.Component
		related string
	}{
		{"no DTSTART", event("", ""), "START"},
		{"no end anchor", event("20260601T090000Z", ""), "END"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr, err := duration.ParseTrigger(vstar.Property{
				Name:   "TRIGGER",
				Params: []vstar.Param{{Name: "RELATED", Value: tc.related}},
				Value:  "-PT15M",
			})
			if err != nil {
				t.Fatalf("ParseTrigger: %v", err)
			}
			if _, err := tr.Resolve(tc.parent, vstar.Calendar{}); !errors.Is(err, duration.ErrNoAnchor) {
				t.Fatalf("error = %v, want errors.Is(ErrNoAnchor)", err)
			}
		})
	}
}

// TestTrigger_Property round-trips a Trigger back to the wire
// property it came from, so producers and consumers agree.
func TestTrigger_Property(t *testing.T) {
	tests := []struct {
		name       string
		in         vstar.Property
		wantValue  string
		wantParams []vstar.Param
	}{
		{
			name:      "relative default anchor omits RELATED",
			in:        vstar.Property{Name: "TRIGGER", Value: "-PT15M"},
			wantValue: "-PT15M",
		},
		{
			name:       "relative end anchor keeps RELATED",
			in:         vstar.Property{Name: "TRIGGER", Params: []vstar.Param{{Name: "RELATED", Value: "END"}}, Value: "PT10M"},
			wantValue:  "PT10M",
			wantParams: []vstar.Param{{Name: "RELATED", Value: "END"}},
		},
		{
			name:       "absolute carries VALUE=DATE-TIME",
			in:         vstar.Property{Name: "TRIGGER", Value: "20260601T083000Z"},
			wantValue:  "20260601T083000Z",
			wantParams: []vstar.Param{{Name: "VALUE", Value: "DATE-TIME"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr, err := duration.ParseTrigger(tc.in)
			if err != nil {
				t.Fatalf("ParseTrigger: %v", err)
			}
			got := tr.Property()
			if got.Name != "TRIGGER" {
				t.Fatalf("Name = %q, want TRIGGER", got.Name)
			}
			if got.Value != tc.wantValue {
				t.Fatalf("Value = %q, want %q", got.Value, tc.wantValue)
			}
			if len(got.Params) != len(tc.wantParams) {
				t.Fatalf("Params = %+v, want %+v", got.Params, tc.wantParams)
			}
			for i := range tc.wantParams {
				if got.Params[i] != tc.wantParams[i] {
					t.Fatalf("Params[%d] = %+v, want %+v", i, got.Params[i], tc.wantParams[i])
				}
			}
			// Re-parsing the emitted property yields an equal Trigger.
			again, err := duration.ParseTrigger(got)
			if err != nil {
				t.Fatalf("re-ParseTrigger: %v", err)
			}
			if again.Relative != tr.Relative || again.Related != tr.Related ||
				again.Duration != tr.Duration || !again.Absolute.Equal(tr.Absolute) {
				t.Fatalf("re-parsed = %+v, want %+v", again, tr)
			}
		})
	}
}

// TestEventEnd covers the VEVENT DTEND-or-DURATION rule directly.
func TestEventEnd(t *testing.T) {
	t.Run("explicit DTEND wins", func(t *testing.T) {
		c := event("20260601T090000Z", "20260601T100000Z")
		got, ok := duration.EventEnd(c, vstar.Calendar{})
		if !ok {
			t.Fatal("ok = false, want true")
		}
		want := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("EventEnd = %v, want %v", got, want)
		}
	})
	t.Run("DURATION derives the end", func(t *testing.T) {
		c := event("20260601T090000Z", "")
		c.Add(vstar.Property{Name: "DURATION", Value: "P1D"})
		got, ok := duration.EventEnd(c, vstar.Calendar{})
		if !ok {
			t.Fatal("ok = false, want true")
		}
		want := time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("EventEnd = %v, want %v", got, want)
		}
	})
	t.Run("neither yields not-ok", func(t *testing.T) {
		c := event("20260601T090000Z", "")
		if _, ok := duration.EventEnd(c, vstar.Calendar{}); ok {
			t.Fatal("ok = true, want false")
		}
	})
	t.Run("malformed DURATION yields not-ok", func(t *testing.T) {
		c := event("20260601T090000Z", "")
		c.Add(vstar.Property{Name: "DURATION", Value: "P1Y"})
		if _, ok := duration.EventEnd(c, vstar.Calendar{}); ok {
			t.Fatal("ok = true, want false")
		}
	})
}

// TestAlarmRepeatCycle covers the VALARM DURATION/REPEAT pair from
// RFC 5545 §3.8.6.2/§3.8.6.3: the repetition interval and count.
func TestAlarmRepeatCycle(t *testing.T) {
	t.Run("pair present", func(t *testing.T) {
		a := alarm(vstar.Property{Name: "TRIGGER", Value: "-PT15M"})
		a.Add(vstar.Property{Name: "DURATION", Value: "PT5M"})
		a.Add(vstar.Property{Name: "REPEAT", Value: "3"})
		d, repeat, err := duration.AlarmRepeatCycle(a)
		if err != nil {
			t.Fatalf("AlarmRepeatCycle: %v", err)
		}
		if repeat != 3 {
			t.Fatalf("repeat = %d, want 3", repeat)
		}
		if d.Minutes != 5 || d.Negative {
			t.Fatalf("duration = %+v, want PT5M", d)
		}
	})
	t.Run("absent pair reports zero", func(t *testing.T) {
		a := alarm(vstar.Property{Name: "TRIGGER", Value: "-PT15M"})
		d, repeat, err := duration.AlarmRepeatCycle(a)
		if err != nil {
			t.Fatalf("AlarmRepeatCycle: %v", err)
		}
		if repeat != 0 || d != (duration.Duration{}) {
			t.Fatalf("got (%+v, %d), want (zero, 0)", d, repeat)
		}
	})
	t.Run("REPEAT without DURATION is malformed", func(t *testing.T) {
		a := alarm(vstar.Property{Name: "TRIGGER", Value: "-PT15M"})
		a.Add(vstar.Property{Name: "REPEAT", Value: "3"})
		if _, _, err := duration.AlarmRepeatCycle(a); !errors.Is(err, vstar.ErrMalformed) {
			t.Fatalf("error = %v, want errors.Is(vstar.ErrMalformed)", err)
		}
	})
	t.Run("negative REPEAT is malformed", func(t *testing.T) {
		a := alarm(vstar.Property{Name: "TRIGGER", Value: "-PT15M"})
		a.Add(vstar.Property{Name: "DURATION", Value: "PT5M"})
		a.Add(vstar.Property{Name: "REPEAT", Value: "-1"})
		if _, _, err := duration.AlarmRepeatCycle(a); !errors.Is(err, vstar.ErrMalformed) {
			t.Fatalf("error = %v, want errors.Is(vstar.ErrMalformed)", err)
		}
	})
}
