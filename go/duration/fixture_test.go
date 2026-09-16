// SPDX-License-Identifier: MIT

package duration_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/duration"
)

// openFixture opens a shared conformance fixture for reading.
func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	path := filepath.Join("..", "testdata", "rfc5545", name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// TestFixture_VEventValarmRelativeTrigger closes the loop the audit
// opened: vstar has shipped testdata/rfc5545/vevent_valarm.ics —
// carrying TRIGGER:-PT15M — since v0.1 without being able to
// interpret it. This test parses the shipped fixture and asserts the
// alarm resolves to 15 minutes before the event's DTSTART.
//
// The fixture is consumed read-only; no fixture bytes change, so no
// spec/fixture coordination is required (CONTRIBUTING.md
// "Shared testdata/ corpus").
func TestFixture_VEventValarmRelativeTrigger(t *testing.T) {
	const name = "vevent_valarm.ics"
	cal, err := rfc5545.Parse(openFixture(t, name))
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}

	evt, ok := findComponent(cal.Components, vstar.CompEvent)
	if !ok {
		t.Fatalf("fixture %s has no VEVENT", name)
	}
	alrm, ok := findComponent(evt.Sub, vstar.CompAlarm)
	if !ok {
		t.Fatalf("fixture %s VEVENT has no VALARM", name)
	}

	tr, err := duration.AlarmTrigger(alrm)
	if err != nil {
		t.Fatalf("AlarmTrigger: %v", err)
	}
	if !tr.Relative {
		t.Fatal("fixture TRIGGER read as absolute, want relative")
	}
	if tr.Related != duration.RelatedStart {
		t.Fatalf("Related = %v, want RelatedStart", tr.Related)
	}
	if got, want := tr.Duration.String(), "-PT15M"; got != want {
		t.Fatalf("Duration.String() = %q, want %q", got, want)
	}

	fireAt, err := tr.Resolve(evt, cal)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// Fixture DTSTART is 20260601T090000Z.
	want := time.Date(2026, 6, 1, 8, 45, 0, 0, time.UTC)
	if !fireAt.Equal(want) {
		t.Fatalf("alarm fires at %v, want %v", fireAt, want)
	}
}

// findComponent returns the first component of the given type.
func findComponent(comps []vstar.Component, typ vstar.CompType) (vstar.Component, bool) {
	for _, c := range comps {
		if c.Type == typ {
			return c, true
		}
	}
	return vstar.Component{}, false
}

// TestFixture_VEventDurationAlarm consumes the shared
// vevent_duration_alarm.ics fixture: a VEVENT that states its end as
// DURATION:P1D (no DTEND) and carries a VALARM anchored to that end
// with a REPEAT/DURATION pair.
func TestFixture_VEventDurationAlarm(t *testing.T) {
	const name = "vevent_duration_alarm.ics"
	cal, err := rfc5545.Parse(openFixture(t, name))
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	evt, ok := findComponent(cal.Components, vstar.CompEvent)
	if !ok {
		t.Fatalf("fixture %s has no VEVENT", name)
	}
	alrm, ok := findComponent(evt.Sub, vstar.CompAlarm)
	if !ok {
		t.Fatalf("fixture %s VEVENT has no VALARM", name)
	}

	// Fixture DTSTART is 20260601T090000Z, DURATION:P1D.
	start := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	end, ok := duration.EventEnd(evt, cal)
	if !ok {
		t.Fatal("EventEnd: no end derivable from DTSTART+DURATION")
	}
	if want := start.AddDate(0, 0, 1); !end.Equal(want) {
		t.Fatalf("EventEnd = %v, want %v", end, want)
	}

	tr, err := duration.AlarmTrigger(alrm)
	if err != nil {
		t.Fatalf("AlarmTrigger: %v", err)
	}
	if !tr.Relative {
		t.Fatal("fixture TRIGGER read as absolute, want relative")
	}
	if tr.Related != duration.RelatedEnd {
		t.Fatalf("Related = %v, want RelatedEnd", tr.Related)
	}
	if got, want := tr.Duration.String(), "-P1D"; got != want {
		t.Fatalf("Duration.String() = %q, want %q", got, want)
	}
	fireAt, err := tr.Resolve(evt, cal)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// One day before an end that is one day after DTSTART.
	if !fireAt.Equal(start) {
		t.Fatalf("alarm fires at %v, want %v", fireAt, start)
	}

	cycle, repeat, err := duration.AlarmRepeatCycle(alrm)
	if err != nil {
		t.Fatalf("AlarmRepeatCycle: %v", err)
	}
	if got, want := cycle.String(), "PT5M"; got != want {
		t.Fatalf("repeat cycle = %q, want %q", got, want)
	}
	if repeat != 2 {
		t.Fatalf("repeat = %d, want 2", repeat)
	}
}

// TestFixture_VAlarmAbsoluteTrigger consumes the shared
// valarm_absolute_trigger.ics fixture: a VALARM whose TRIGGER is an
// explicit VALUE=DATE-TIME instant, resolved without reference to
// the parent's DTSTART.
func TestFixture_VAlarmAbsoluteTrigger(t *testing.T) {
	const name = "valarm_absolute_trigger.ics"
	cal, err := rfc5545.Parse(openFixture(t, name))
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	evt, ok := findComponent(cal.Components, vstar.CompEvent)
	if !ok {
		t.Fatalf("fixture %s has no VEVENT", name)
	}
	alrm, ok := findComponent(evt.Sub, vstar.CompAlarm)
	if !ok {
		t.Fatalf("fixture %s VEVENT has no VALARM", name)
	}

	tr, err := duration.AlarmTrigger(alrm)
	if err != nil {
		t.Fatalf("AlarmTrigger: %v", err)
	}
	if tr.Relative {
		t.Fatal("fixture TRIGGER read as relative, want absolute")
	}
	want := time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC)
	if !tr.Absolute.Equal(want) {
		t.Fatalf("Absolute = %v, want %v", tr.Absolute, want)
	}
	fireAt, err := tr.Resolve(evt, cal)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !fireAt.Equal(want) {
		t.Fatalf("alarm fires at %v, want %v", fireAt, want)
	}
}
