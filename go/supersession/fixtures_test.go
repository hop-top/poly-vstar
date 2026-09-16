// SPDX-License-Identifier: MIT

package supersession_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/hashing"
	"hop.top/vstar/supersession"
)

// fixturesDir resolves testdata/supersession/ relative to this
// test file. Walking up from the package dir is the simplest
// cross-platform-friendly path.
func fixturesDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, "testdata", "supersession")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find testdata/supersession above %s", dir)
		}
		dir = parent
	}
}

// loadFixtureCalendar parses testdata/supersession/<stem>.ics into
// a Calendar.
func loadFixtureCalendar(t *testing.T, stem string) vstar.Calendar {
	t.Helper()
	path := filepath.Join(fixturesDir(t), stem+".ics")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	cal, err := rfc5545.Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return cal
}

// TestFixtures_Linear loads testdata/supersession/linear.ics and
// asserts the supersession projection: todo-1 should report
// COMPLETED via Superseded.
func TestFixtures_Linear(t *testing.T) {
	t.Parallel()

	cal := loadFixtureCalendar(t, "linear")
	target, ok := cal.Find("todo-1")
	if !ok {
		t.Fatalf("linear.ics missing todo-1")
	}
	status, found := supersession.Superseded(target, cal.Components)
	if !found {
		t.Fatalf("Superseded(todo-1, ledger) returned ok=false; want true")
	}
	if got, want := status, string(vstar.TodoCompleted); got != want {
		t.Errorf("Superseded(todo-1) = %q, want %q", got, want)
	}
}

// TestFixtures_MultiStep loads multi_step.ics and asserts that the
// projection picks the LATEST entry by DTSTAMP regardless of input
// order — both sup1 (IN-PROCESS) and sup2 (COMPLETED) supersede
// todo-multi; the rule says "latest wins".
func TestFixtures_MultiStep(t *testing.T) {
	t.Parallel()

	cal := loadFixtureCalendar(t, "multi_step")
	target, ok := cal.Find("todo-multi")
	if !ok {
		t.Fatalf("multi_step.ics missing todo-multi")
	}
	status, found := supersession.Superseded(target, cal.Components)
	if !found {
		t.Fatalf("Superseded(todo-multi, ledger) returned ok=false; want true")
	}
	if got, want := status, string(vstar.TodoCompleted); got != want {
		t.Errorf("Superseded(todo-multi) = %q, want %q (latest wins)", got, want)
	}
}

// TestFixtures_CrossComponent loads cross_component.ics and
// confirms supersession works across the VTODO ↔ VJOURNAL boundary
// in one VCALENDAR (V*'s "ledger is one logical container" model).
func TestFixtures_CrossComponent(t *testing.T) {
	t.Parallel()

	cal := loadFixtureCalendar(t, "cross_component")
	target, ok := cal.Find("turn-42")
	if !ok {
		t.Fatalf("cross_component.ics missing turn-42")
	}
	status, found := supersession.Superseded(target, cal.Components)
	if !found {
		t.Fatalf("Superseded(turn-42, ledger) returned ok=false; want true")
	}
	if got, want := status, string(vstar.TodoCompleted); got != want {
		t.Errorf("Superseded(turn-42) = %q, want %q", got, want)
	}
}

// TestFixtures_CorruptMutated loads corrupt_mutated.ics and asserts
// that the in-document X-VSTAR-HASH does NOT verify — the SUMMARY
// was mutated post-stamp. The fixture is intentionally corrupt; the
// test value is the negative confirmation that the integrity layer
// notices.
func TestFixtures_CorruptMutated(t *testing.T) {
	t.Parallel()

	cal := loadFixtureCalendar(t, "corrupt_mutated")
	target, ok := cal.Find("todo-corrupt")
	if !ok {
		t.Fatalf("corrupt_mutated.ics missing todo-corrupt")
	}

	verifyOK, _, _ := hashing.VerifyXVSTAR(target)
	if verifyOK {
		t.Fatalf("hashing.VerifyXVSTAR(todo-corrupt) = ok; want false (the fixture is intentionally corrupt)")
	}

	// And Supersedes must refuse with ErrTargetCorrupted.
	_, err := supersession.Supersedes(target, string(vstar.TodoCompleted), mustTime(t, "20260504T143000Z"))
	if !errors.Is(err, supersession.ErrTargetCorrupted) {
		t.Fatalf("Supersedes(corrupt target) error = %v; want ErrTargetCorrupted", err)
	}
}

// TestFixtures_EffectiveStatusVEvent loads effective_status_vevent.ics
// and asserts the projection across the VEVENT ↔ VJOURNAL boundary:
// the turn reports the cancellation token, spelled as the VEVENT
// vocabulary has it — spec/02 scopes X-VSTAR-EFFECTIVE-STATUS to the superseded
// component's type, not the journal's.
func TestFixtures_EffectiveStatusVEvent(t *testing.T) {
	t.Parallel()

	cal := loadFixtureCalendar(t, "effective_status_vevent")
	target, ok := cal.Find("turn-7")
	if !ok {
		t.Fatalf("effective_status_vevent.ics missing turn-7")
	}
	if target.Type != vstar.CompEvent {
		t.Fatalf("turn-7 type = %s, want VEVENT", target.Type)
	}
	status, found := supersession.Superseded(target, cal.Components)
	if !found {
		t.Fatalf("Superseded(turn-7, ledger) returned ok=false; want true")
	}
	if got, want := status, string(vstar.EventCancelled); got != want {
		t.Errorf("Superseded(turn-7) = %q, want %q", got, want)
	}
}
