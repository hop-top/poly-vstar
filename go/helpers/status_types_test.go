// SPDX-License-Identifier: MIT

package helpers_test

import (
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
	"hop.top/vstar/helpers"
)

func newEvent(t *testing.T) vstar.Component {
	t.Helper()
	start := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	c, err := helpers.NewEvent("uid-ev", start, start.Add(time.Hour))
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	return c
}

func newJournal(t *testing.T) vstar.Component {
	t.Helper()
	c, err := helpers.NewJournal("uid-jr", time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}
	return c
}

func newTodo(t *testing.T) vstar.Component {
	t.Helper()
	c, err := helpers.NewTodo("uid-td", time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewTodo: %v", err)
	}
	return c
}

// --- EventStatus ---------------------------------------------------

func TestEventStatus_returnsValueWhenSet(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	c.Set(vstar.Property{Name: "STATUS", Value: string(vstar.EventConfirmed)})
	got, ok := helpers.EventStatus(c)
	if !ok || got != vstar.EventConfirmed {
		t.Errorf("EventStatus = %q %v, want %q true", got, ok, vstar.EventConfirmed)
	}
}

func TestEventStatus_returnsFalseWhenAbsent(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	if got, ok := helpers.EventStatus(c); ok {
		t.Errorf("EventStatus ok = true, want false (got %q)", got)
	}
}

func TestEventStatus_returnsFalseForInvalidValue(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	// NEEDS-ACTION is a valid VTODO status but not a VEVENT one.
	c.Set(vstar.Property{Name: "STATUS", Value: string(vstar.TodoNeedsAction)})
	if _, ok := helpers.EventStatus(c); ok {
		t.Errorf("EventStatus ok = true for VTODO-only value, want false")
	}
}

func TestSetEventStatus_roundTripAndHashFresh(t *testing.T) {
	for _, want := range []vstar.EventStatus{
		vstar.EventTentative,
		vstar.EventConfirmed,
		vstar.EventCancelled,
	} {
		c := newEvent(t)
		pre, _ := c.Get(hashing.XVSTARHashProperty)
		helpers.SetEventStatus(&c, want)
		post, _ := c.Get(hashing.XVSTARHashProperty)
		if pre.Value == post.Value {
			t.Errorf("%q: X-VSTAR-HASH unchanged after SetEventStatus", want)
		}
		got, ok := helpers.EventStatus(c)
		if !ok || got != want {
			t.Errorf("round-trip %q: got %q ok=%v", want, got, ok)
		}
		// Hash must cover the STATUS write — i.e. refreshed LAST.
		if ok, wantHash, gotHash := hashing.VerifyXVSTAR(c); !ok {
			t.Errorf("%q: VerifyXVSTAR failed (stale hash): want %s got %s", want, wantHash, gotHash)
		}
	}
}

func TestSetEventStatus_invalidValueIsNoOp(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	c.Set(vstar.Property{Name: "UID", Value: "u"})
	helpers.SetEventStatus(&c, vstar.EventStatus("BOGUS"))
	if _, ok := c.Get("STATUS"); ok {
		t.Errorf("STATUS written for invalid value, want no-op")
	}
}

func TestSetEventStatus_nonEventIsNoOp(t *testing.T) {
	for _, ct := range []vstar.CompType{vstar.CompTodo, vstar.CompJournal, vstar.CompFreeBusy} {
		c := vstar.Component{Type: ct}
		c.Set(vstar.Property{Name: "UID", Value: "u"})
		helpers.SetEventStatus(&c, vstar.EventConfirmed)
		if _, ok := c.Get("STATUS"); ok {
			t.Errorf("STATUS written on %s, want no-op", ct)
		}
	}
}

func TestSetEventStatus_nilIsNoOp(t *testing.T) {
	helpers.SetEventStatus(nil, vstar.EventConfirmed)
}

// --- JournalStatus -------------------------------------------------

func TestJournalStatus_returnsValueWhenSet(t *testing.T) {
	c := vstar.Component{Type: vstar.CompJournal}
	c.Set(vstar.Property{Name: "STATUS", Value: string(vstar.JournalFinal)})
	got, ok := helpers.JournalStatus(c)
	if !ok || got != vstar.JournalFinal {
		t.Errorf("JournalStatus = %q %v, want %q true", got, ok, vstar.JournalFinal)
	}
}

func TestJournalStatus_returnsFalseWhenAbsent(t *testing.T) {
	c := vstar.Component{Type: vstar.CompJournal}
	if got, ok := helpers.JournalStatus(c); ok {
		t.Errorf("JournalStatus ok = true, want false (got %q)", got)
	}
}

func TestJournalStatus_returnsFalseForInvalidValue(t *testing.T) {
	c := vstar.Component{Type: vstar.CompJournal}
	// TENTATIVE is a valid VEVENT status but not a VJOURNAL one.
	c.Set(vstar.Property{Name: "STATUS", Value: string(vstar.EventTentative)})
	if _, ok := helpers.JournalStatus(c); ok {
		t.Errorf("JournalStatus ok = true for VEVENT-only value, want false")
	}
}

func TestSetJournalStatus_roundTripAndHashFresh(t *testing.T) {
	for _, want := range []vstar.JournalStatus{
		vstar.JournalDraft,
		vstar.JournalFinal,
		vstar.JournalCancelled,
	} {
		c := newJournal(t)
		pre, _ := c.Get(hashing.XVSTARHashProperty)
		helpers.SetJournalStatus(&c, want)
		post, _ := c.Get(hashing.XVSTARHashProperty)
		if pre.Value == post.Value {
			t.Errorf("%q: X-VSTAR-HASH unchanged after SetJournalStatus", want)
		}
		got, ok := helpers.JournalStatus(c)
		if !ok || got != want {
			t.Errorf("round-trip %q: got %q ok=%v", want, got, ok)
		}
		if ok, wantHash, gotHash := hashing.VerifyXVSTAR(c); !ok {
			t.Errorf("%q: VerifyXVSTAR failed (stale hash): want %s got %s", want, wantHash, gotHash)
		}
	}
}

func TestSetJournalStatus_invalidValueIsNoOp(t *testing.T) {
	c := vstar.Component{Type: vstar.CompJournal}
	c.Set(vstar.Property{Name: "UID", Value: "u"})
	helpers.SetJournalStatus(&c, vstar.JournalStatus("BOGUS"))
	if _, ok := c.Get("STATUS"); ok {
		t.Errorf("STATUS written for invalid value, want no-op")
	}
}

func TestSetJournalStatus_nonJournalIsNoOp(t *testing.T) {
	for _, ct := range []vstar.CompType{vstar.CompTodo, vstar.CompEvent, vstar.CompFreeBusy} {
		c := vstar.Component{Type: ct}
		c.Set(vstar.Property{Name: "UID", Value: "u"})
		helpers.SetJournalStatus(&c, vstar.JournalFinal)
		if _, ok := c.Get("STATUS"); ok {
			t.Errorf("STATUS written on %s, want no-op", ct)
		}
	}
}

func TestSetJournalStatus_nilIsNoOp(t *testing.T) {
	helpers.SetJournalStatus(nil, vstar.JournalFinal)
}

// --- cross-type isolation -----------------------------------------

// TestStatusSetters_doNotCrossAssign is the point of having three
// distinct status types: a VTODO must never end up carrying DRAFT,
// nor a VJOURNAL NEEDS-ACTION.
func TestStatusSetters_doNotCrossAssign(t *testing.T) {
	todo := newTodo(t)
	helpers.SetEventStatus(&todo, vstar.EventConfirmed)
	helpers.SetJournalStatus(&todo, vstar.JournalDraft)
	if p, ok := todo.Get("STATUS"); ok {
		t.Errorf("VTODO acquired STATUS=%q from a foreign setter", p.Value)
	}

	ev := newEvent(t)
	helpers.SetStatus(&ev, vstar.TodoNeedsAction)
	helpers.SetJournalStatus(&ev, vstar.JournalDraft)
	if p, ok := ev.Get("STATUS"); ok {
		t.Errorf("VEVENT acquired STATUS=%q from a foreign setter", p.Value)
	}

	jr := newJournal(t)
	helpers.SetStatus(&jr, vstar.TodoNeedsAction)
	helpers.SetEventStatus(&jr, vstar.EventConfirmed)
	if p, ok := jr.Get("STATUS"); ok {
		t.Errorf("VJOURNAL acquired STATUS=%q from a foreign setter", p.Value)
	}
}

// --- Class ---------------------------------------------------------

func TestClass_returnsValueWhenSet(t *testing.T) {
	for _, want := range []vstar.Class{vstar.ClassPublic, vstar.ClassPrivate, vstar.ClassConfidential} {
		c := vstar.Component{Type: vstar.CompEvent}
		c.Set(vstar.Property{Name: "CLASS", Value: string(want)})
		got, ok := helpers.Class(c)
		if !ok || got != want {
			t.Errorf("Class = %q %v, want %q true", got, ok, want)
		}
	}
}

// TestClass_returnsFalseWhenAbsent pins the getter contract: an
// absent CLASS reports ok=false and does NOT synthesize the RFC
// default. Callers wanting the default use ClassOrDefault.
func TestClass_returnsFalseWhenAbsent(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	if got, ok := helpers.Class(c); ok {
		t.Errorf("Class ok = true for absent CLASS, want false (got %q)", got)
	}
}

func TestClass_returnsFalseForInvalidValue(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	c.Set(vstar.Property{Name: "CLASS", Value: "SEMI-PRIVATE"})
	if _, ok := helpers.Class(c); ok {
		t.Errorf("Class ok = true for invalid value, want false")
	}
}

// TestClassOrDefault_appliesRFCDefault covers RFC 5545 §3.8.1.3:
// an absent CLASS means PUBLIC.
func TestClassOrDefault_appliesRFCDefault(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	if got := helpers.ClassOrDefault(c); got != vstar.ClassPublic {
		t.Errorf("ClassOrDefault = %q, want %q", got, vstar.ClassPublic)
	}
	c.Set(vstar.Property{Name: "CLASS", Value: string(vstar.ClassConfidential)})
	if got := helpers.ClassOrDefault(c); got != vstar.ClassConfidential {
		t.Errorf("ClassOrDefault = %q, want %q", got, vstar.ClassConfidential)
	}
	// An unrecognized value falls back to the RFC default too.
	c.Set(vstar.Property{Name: "CLASS", Value: "SEMI-PRIVATE"})
	if got := helpers.ClassOrDefault(c); got != vstar.ClassPublic {
		t.Errorf("ClassOrDefault = %q for invalid value, want %q", got, vstar.ClassPublic)
	}
}

func TestSetClass_roundTripAndHashFresh(t *testing.T) {
	for _, want := range []vstar.Class{vstar.ClassPublic, vstar.ClassPrivate, vstar.ClassConfidential} {
		for _, mk := range []func(*testing.T) vstar.Component{newEvent, newTodo, newJournal} {
			c := mk(t)
			pre, _ := c.Get(hashing.XVSTARHashProperty)
			helpers.SetClass(&c, want)
			post, _ := c.Get(hashing.XVSTARHashProperty)
			if pre.Value == post.Value {
				t.Errorf("%s/%q: X-VSTAR-HASH unchanged after SetClass", c.Type, want)
			}
			got, ok := helpers.Class(c)
			if !ok || got != want {
				t.Errorf("%s round-trip %q: got %q ok=%v", c.Type, want, got, ok)
			}
			if ok, wantHash, gotHash := hashing.VerifyXVSTAR(c); !ok {
				t.Errorf("%s/%q: VerifyXVSTAR failed (stale hash): want %s got %s", c.Type, want, wantHash, gotHash)
			}
		}
	}
}

func TestSetClass_invalidValueIsNoOp(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	c.Set(vstar.Property{Name: "UID", Value: "u"})
	helpers.SetClass(&c, vstar.Class("SEMI-PRIVATE"))
	if _, ok := c.Get("CLASS"); ok {
		t.Errorf("CLASS written for invalid value, want no-op")
	}
}

// TestSetClass_unsupportedTypeIsNoOp — CLASS applies to VEVENT,
// VTODO and VJOURNAL only (RFC 5545 §3.8.1.3).
func TestSetClass_unsupportedTypeIsNoOp(t *testing.T) {
	for _, ct := range []vstar.CompType{vstar.CompFreeBusy, vstar.CompTimezone, vstar.CompAlarm, vstar.CompCalendar} {
		c := vstar.Component{Type: ct}
		c.Set(vstar.Property{Name: "UID", Value: "u"})
		helpers.SetClass(&c, vstar.ClassPrivate)
		if _, ok := c.Get("CLASS"); ok {
			t.Errorf("CLASS written on %s, want no-op", ct)
		}
	}
}

func TestSetClass_nilIsNoOp(t *testing.T) {
	helpers.SetClass(nil, vstar.ClassPrivate)
}

// --- Transp --------------------------------------------------------

func TestTransp_returnsValueWhenSet(t *testing.T) {
	for _, want := range []vstar.Transp{vstar.TranspOpaque, vstar.TranspTransparent} {
		c := vstar.Component{Type: vstar.CompEvent}
		c.Set(vstar.Property{Name: "TRANSP", Value: string(want)})
		got, ok := helpers.Transp(c)
		if !ok || got != want {
			t.Errorf("Transp = %q %v, want %q true", got, ok, want)
		}
	}
}

func TestTransp_returnsFalseWhenAbsent(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	if got, ok := helpers.Transp(c); ok {
		t.Errorf("Transp ok = true for absent TRANSP, want false (got %q)", got)
	}
}

func TestTransp_returnsFalseForInvalidValue(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	c.Set(vstar.Property{Name: "TRANSP", Value: "TRANSLUCENT"})
	if _, ok := helpers.Transp(c); ok {
		t.Errorf("Transp ok = true for invalid value, want false")
	}
}

// TestTranspOrDefault_appliesRFCDefault covers RFC 5545 §3.8.2.7:
// an absent TRANSP means OPAQUE (the event blocks free/busy time).
func TestTranspOrDefault_appliesRFCDefault(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	if got := helpers.TranspOrDefault(c); got != vstar.TranspOpaque {
		t.Errorf("TranspOrDefault = %q, want %q", got, vstar.TranspOpaque)
	}
	c.Set(vstar.Property{Name: "TRANSP", Value: string(vstar.TranspTransparent)})
	if got := helpers.TranspOrDefault(c); got != vstar.TranspTransparent {
		t.Errorf("TranspOrDefault = %q, want %q", got, vstar.TranspTransparent)
	}
	c.Set(vstar.Property{Name: "TRANSP", Value: "TRANSLUCENT"})
	if got := helpers.TranspOrDefault(c); got != vstar.TranspOpaque {
		t.Errorf("TranspOrDefault = %q for invalid value, want %q", got, vstar.TranspOpaque)
	}
}

func TestSetTransp_roundTripAndHashFresh(t *testing.T) {
	for _, want := range []vstar.Transp{vstar.TranspOpaque, vstar.TranspTransparent} {
		c := newEvent(t)
		pre, _ := c.Get(hashing.XVSTARHashProperty)
		helpers.SetTransp(&c, want)
		post, _ := c.Get(hashing.XVSTARHashProperty)
		if pre.Value == post.Value {
			t.Errorf("%q: X-VSTAR-HASH unchanged after SetTransp", want)
		}
		got, ok := helpers.Transp(c)
		if !ok || got != want {
			t.Errorf("round-trip %q: got %q ok=%v", want, got, ok)
		}
		if ok, wantHash, gotHash := hashing.VerifyXVSTAR(c); !ok {
			t.Errorf("%q: VerifyXVSTAR failed (stale hash): want %s got %s", want, wantHash, gotHash)
		}
	}
}

func TestSetTransp_invalidValueIsNoOp(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	c.Set(vstar.Property{Name: "UID", Value: "u"})
	helpers.SetTransp(&c, vstar.Transp("TRANSLUCENT"))
	if _, ok := c.Get("TRANSP"); ok {
		t.Errorf("TRANSP written for invalid value, want no-op")
	}
}

// TestSetTransp_nonEventIsNoOp — TRANSP is VEVENT-only per
// RFC 5545 §3.8.2.7.
func TestSetTransp_nonEventIsNoOp(t *testing.T) {
	for _, ct := range []vstar.CompType{vstar.CompTodo, vstar.CompJournal, vstar.CompFreeBusy} {
		c := vstar.Component{Type: ct}
		c.Set(vstar.Property{Name: "UID", Value: "u"})
		helpers.SetTransp(&c, vstar.TranspTransparent)
		if _, ok := c.Get("TRANSP"); ok {
			t.Errorf("TRANSP written on %s, want no-op", ct)
		}
	}
}

func TestSetTransp_nilIsNoOp(t *testing.T) {
	helpers.SetTransp(nil, vstar.TranspTransparent)
}

// --- existing VTODO behavior unchanged ----------------------------

// TestSetStatus_stillVTODOOnly re-pins the pre-existing contract so
// the new sibling setters cannot quietly widen it.
func TestSetStatus_stillVTODOOnly(t *testing.T) {
	todo := newTodo(t)
	helpers.SetStatus(&todo, vstar.TodoInProcess)
	got, ok := helpers.Status(todo)
	if !ok || got != vstar.TodoInProcess {
		t.Errorf("Status = %q %v, want %q true", got, ok, vstar.TodoInProcess)
	}
	if ok, _, _ := hashing.VerifyXVSTAR(todo); !ok {
		t.Errorf("VerifyXVSTAR failed after SetStatus")
	}
	// Status() remains type-agnostic by design (documented on the
	// function): it parses whatever STATUS it finds.
	ev := vstar.Component{Type: vstar.CompEvent}
	ev.Set(vstar.Property{Name: "STATUS", Value: string(vstar.TodoCompleted)})
	if _, ok := helpers.Status(ev); !ok {
		t.Errorf("Status must stay type-agnostic for reads")
	}
}
