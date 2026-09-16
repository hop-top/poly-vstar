// SPDX-License-Identifier: MIT

package helpers_test

import (
	"errors"
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/helpers"
)

// withinLastSecond reports whether t is within the last 1s window
// from now. Used for DTSTAMP freshness checks where the constructor
// is responsible for stamping but we cannot pin the exact instant.
func withinLastSecond(t time.Time) bool {
	d := time.Since(t)
	return d >= 0 && d < time.Second
}

func TestNewTodo_setsRequiredProps(t *testing.T) {
	due := time.Date(2026, 5, 4, 17, 0, 0, 0, time.UTC)
	c, err := helpers.NewTodo("uid-1", due)
	if err != nil {
		t.Fatalf("NewTodo: %v", err)
	}
	if c.Type != vstar.CompTodo {
		t.Errorf("Type = %q, want %q", c.Type, vstar.CompTodo)
	}
	if got := c.UID(); got != "uid-1" {
		t.Errorf("UID = %q, want uid-1", got)
	}
	stamp, ok := c.DTSTAMP()
	if !ok {
		t.Fatalf("DTSTAMP missing")
	}
	if !withinLastSecond(stamp) {
		t.Errorf("DTSTAMP = %v, not within 1s of now", stamp)
	}
	got, ok := c.DUE(vstar.Calendar{})
	if !ok {
		t.Fatalf("DUE missing")
	}
	if !got.Equal(due) {
		t.Errorf("DUE = %v, want %v", got, due)
	}
	if _, ok := c.Get("X-VSTAR-HASH"); !ok {
		t.Errorf("X-VSTAR-HASH missing")
	}
}

func TestNewTodo_emptyUIDReturnsErrMissingUID(t *testing.T) {
	_, err := helpers.NewTodo("", time.Now())
	if !errors.Is(err, vstar.ErrMissingUID) {
		t.Errorf("err = %v, want ErrMissingUID", err)
	}
}

func TestNewJournal_setsRequiredProps(t *testing.T) {
	dtstart := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	c, err := helpers.NewJournal("uid-j", dtstart)
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}
	if c.Type != vstar.CompJournal {
		t.Errorf("Type = %q, want %q", c.Type, vstar.CompJournal)
	}
	if c.UID() != "uid-j" {
		t.Errorf("UID = %q", c.UID())
	}
	got, ok := c.DTSTART(vstar.Calendar{})
	if !ok || !got.Equal(dtstart) {
		t.Errorf("DTSTART = %v %v, want %v true", got, ok, dtstart)
	}
	if _, ok := c.Get("X-VSTAR-HASH"); !ok {
		t.Errorf("X-VSTAR-HASH missing")
	}
}

func TestNewJournal_emptyUID(t *testing.T) {
	_, err := helpers.NewJournal("", time.Now())
	if !errors.Is(err, vstar.ErrMissingUID) {
		t.Errorf("err = %v, want ErrMissingUID", err)
	}
}

func TestNewEvent_setsStartAndEnd(t *testing.T) {
	start := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	c, err := helpers.NewEvent("uid-e", start, end)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if c.Type != vstar.CompEvent {
		t.Errorf("Type = %q", c.Type)
	}
	gs, ok := c.DTSTART(vstar.Calendar{})
	if !ok || !gs.Equal(start) {
		t.Errorf("DTSTART = %v %v, want %v true", gs, ok, start)
	}
	ge, ok := c.DTEND(vstar.Calendar{})
	if !ok || !ge.Equal(end) {
		t.Errorf("DTEND = %v %v, want %v true", ge, ok, end)
	}
}

func TestNewEvent_emptyUID(t *testing.T) {
	_, err := helpers.NewEvent("", time.Now(), time.Now())
	if !errors.Is(err, vstar.ErrMissingUID) {
		t.Errorf("err = %v, want ErrMissingUID", err)
	}
}

func TestNewFreeBusy_setsStartAndEnd(t *testing.T) {
	start := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	c, err := helpers.NewFreeBusy("uid-fb", start, end)
	if err != nil {
		t.Fatalf("NewFreeBusy: %v", err)
	}
	if c.Type != vstar.CompFreeBusy {
		t.Errorf("Type = %q", c.Type)
	}
	gs, ok := c.DTSTART(vstar.Calendar{})
	if !ok || !gs.Equal(start) {
		t.Errorf("DTSTART = %v %v", gs, ok)
	}
	ge, ok := c.DTEND(vstar.Calendar{})
	if !ok || !ge.Equal(end) {
		t.Errorf("DTEND = %v %v", ge, ok)
	}
}

func TestNewFreeBusy_emptyUID(t *testing.T) {
	_, err := helpers.NewFreeBusy("", time.Now(), time.Now())
	if !errors.Is(err, vstar.ErrMissingUID) {
		t.Errorf("err = %v", err)
	}
}

func TestNewAlarm_setsActionAndTrigger(t *testing.T) {
	c, err := helpers.NewAlarm("uid-a", "DISPLAY", "-PT15M")
	if err != nil {
		t.Fatalf("NewAlarm: %v", err)
	}
	if c.Type != vstar.CompAlarm {
		t.Errorf("Type = %q", c.Type)
	}
	if p, ok := c.Get("ACTION"); !ok || p.Value != "DISPLAY" {
		t.Errorf("ACTION = %v %v, want DISPLAY", p, ok)
	}
	if p, ok := c.Get("TRIGGER"); !ok || p.Value != "-PT15M" {
		t.Errorf("TRIGGER = %v %v, want -PT15M", p, ok)
	}
	if c.UID() != "uid-a" {
		t.Errorf("UID = %q", c.UID())
	}
	if _, ok := c.Get("X-VSTAR-HASH"); !ok {
		t.Errorf("X-VSTAR-HASH missing")
	}
}

func TestNewAlarm_emptyUID(t *testing.T) {
	_, err := helpers.NewAlarm("", "DISPLAY", "-PT5M")
	if !errors.Is(err, vstar.ErrMissingUID) {
		t.Errorf("err = %v", err)
	}
}

func TestNewCalendar_defaultProdID(t *testing.T) {
	cal := helpers.NewCalendar("")
	if cal.ProdID == "" {
		t.Errorf("ProdID empty; want vstar default")
	}
	// Default must follow the "-//hop-top//vstar-go vX.Y.Z//EN" pattern.
	wantPrefix := "-//hop-top//vstar-go"
	wantSuffix := "//EN"
	if len(cal.ProdID) < len(wantPrefix)+len(wantSuffix) ||
		cal.ProdID[:len(wantPrefix)] != wantPrefix ||
		cal.ProdID[len(cal.ProdID)-len(wantSuffix):] != wantSuffix {
		t.Errorf("ProdID = %q, want %s vX.Y.Z%s", cal.ProdID, wantPrefix, wantSuffix)
	}
}

func TestNewCalendar_customProdID(t *testing.T) {
	cal := helpers.NewCalendar("-//acme//cal v1//EN")
	if cal.ProdID != "-//acme//cal v1//EN" {
		t.Errorf("ProdID = %q", cal.ProdID)
	}
}

func TestNewCard_individualByDefault(t *testing.T) {
	card := helpers.NewCard("uid-c", "")
	if card.UID != "uid-c" {
		t.Errorf("UID = %q", card.UID)
	}
	if card.Kind != vstar.KindIndividual {
		t.Errorf("Kind = %q, want %q", card.Kind, vstar.KindIndividual)
	}
	if p, ok := card.Get("VERSION"); !ok || p.Value != "4.0" {
		t.Errorf("VERSION = %v %v", p, ok)
	}
	if p, ok := card.Get("KIND"); !ok || p.Value != string(vstar.KindIndividual) {
		t.Errorf("KIND = %v %v", p, ok)
	}
	if p, ok := card.Get("UID"); !ok || p.Value != "uid-c" {
		t.Errorf("UID prop = %v %v", p, ok)
	}
}

func TestNewCard_explicitKind(t *testing.T) {
	card := helpers.NewCard("uid-org", vstar.KindOrg)
	if card.Kind != vstar.KindOrg {
		t.Errorf("Kind = %q", card.Kind)
	}
	if p, _ := card.Get("KIND"); p.Value != string(vstar.KindOrg) {
		t.Errorf("KIND = %q, want %q", p.Value, vstar.KindOrg)
	}
}
