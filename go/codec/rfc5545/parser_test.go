// SPDX-License-Identifier: MIT

package rfc5545

import (
	"errors"
	"strings"
	"testing"

	vstar "hop.top/vstar"
)

// joinCRLF joins lines with CRLF and adds a trailing CRLF — the canonical
// physical layout the parser is expected to consume.
func joinCRLF(lines ...string) string {
	return strings.Join(lines, "\r\n") + "\r\n"
}

func TestParse_EmptyVCALENDAR(t *testing.T) {
	t.Parallel()

	src := joinCRLF(
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//V*//Empty//EN",
		"END:VCALENDAR",
	)
	cal, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cal.ProdID != "-//V*//Empty//EN" {
		t.Errorf("ProdID = %q", cal.ProdID)
	}
	if len(cal.Components) != 0 {
		t.Errorf("Components = %v, want empty", cal.Components)
	}
}

func TestParse_OneVTODO(t *testing.T) {
	t.Parallel()

	src := joinCRLF(
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//V*//Test//EN",
		"BEGIN:VTODO",
		"UID:abc-123",
		"DTSTAMP:20260504T120000Z",
		"SUMMARY:Buy milk",
		"END:VTODO",
		"END:VCALENDAR",
	)
	cal, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cal.Components) != 1 {
		t.Fatalf("Components len = %d, want 1", len(cal.Components))
	}
	todo := cal.Components[0]
	if todo.Type != vstar.CompTodo {
		t.Errorf("Type = %q, want VTODO", todo.Type)
	}
	if todo.UID() != "abc-123" {
		t.Errorf("UID = %q", todo.UID())
	}
	if got, _ := todo.Get("SUMMARY"); got.Value != "Buy milk" {
		t.Errorf("SUMMARY = %q", got.Value)
	}
}

func TestParse_NestedVTIMEZONE(t *testing.T) {
	t.Parallel()

	src := joinCRLF(
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//V*//TZ//EN",
		"BEGIN:VTIMEZONE",
		"TZID:America/Toronto",
		"BEGIN:STANDARD",
		"DTSTART:19701101T020000",
		"TZOFFSETFROM:-0400",
		"TZOFFSETTO:-0500",
		"END:STANDARD",
		"BEGIN:DAYLIGHT",
		"DTSTART:19700308T020000",
		"TZOFFSETFROM:-0500",
		"TZOFFSETTO:-0400",
		"END:DAYLIGHT",
		"END:VTIMEZONE",
		"END:VCALENDAR",
	)
	cal, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cal.Components) != 1 {
		t.Fatalf("Components len = %d, want 1", len(cal.Components))
	}
	tz := cal.Components[0]
	if tz.Type != vstar.CompTimezone {
		t.Errorf("Type = %q", tz.Type)
	}
	if len(tz.Sub) != 2 {
		t.Fatalf("Sub len = %d, want 2 (STANDARD + DAYLIGHT)", len(tz.Sub))
	}
	// Subs preserve input order.
	if string(tz.Sub[0].Type) != "STANDARD" {
		t.Errorf("Sub[0].Type = %q", tz.Sub[0].Type)
	}
	if string(tz.Sub[1].Type) != "DAYLIGHT" {
		t.Errorf("Sub[1].Type = %q", tz.Sub[1].Type)
	}
}

func TestParse_VEVENTWithVALARM(t *testing.T) {
	t.Parallel()

	src := joinCRLF(
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//V*//Alarm//EN",
		"BEGIN:VEVENT",
		"UID:evt-1",
		"DTSTAMP:20260504T120000Z",
		"DTSTART:20260601T090000Z",
		"BEGIN:VALARM",
		"ACTION:DISPLAY",
		"TRIGGER:-PT15M",
		"END:VALARM",
		"END:VEVENT",
		"END:VCALENDAR",
	)
	cal, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	evt := cal.Components[0]
	if evt.Type != vstar.CompEvent {
		t.Errorf("Type = %q", evt.Type)
	}
	if len(evt.Sub) != 1 || evt.Sub[0].Type != vstar.CompAlarm {
		t.Errorf("Sub = %+v, want one VALARM", evt.Sub)
	}
}

func TestParse_UnclosedBlock(t *testing.T) {
	t.Parallel()

	src := joinCRLF(
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"BEGIN:VTODO",
		"UID:no-end",
		// no END:VTODO, no END:VCALENDAR
	)
	_, err := Parse(strings.NewReader(src))
	if !errors.Is(err, vstar.ErrUnclosedBlock) {
		t.Errorf("err = %v, want wraps ErrUnclosedBlock", err)
	}
}

func TestParse_MismatchedEND(t *testing.T) {
	t.Parallel()

	src := joinCRLF(
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"BEGIN:VTODO",
		"UID:wrong-end",
		"END:VEVENT", // mismatched
		"END:VCALENDAR",
	)
	_, err := Parse(strings.NewReader(src))
	if !errors.Is(err, vstar.ErrMalformed) {
		t.Errorf("err = %v, want wraps ErrMalformed", err)
	}
}

func TestParse_NoOuterVCALENDAR(t *testing.T) {
	t.Parallel()

	// Properties at top level with no enclosing BEGIN:VCALENDAR.
	src := joinCRLF("VERSION:2.0", "PRODID:foo")
	_, err := Parse(strings.NewReader(src))
	if !errors.Is(err, vstar.ErrMalformed) {
		t.Errorf("err = %v, want wraps ErrMalformed", err)
	}
}

func TestParse_UnsupportedVersion(t *testing.T) {
	t.Parallel()

	src := joinCRLF(
		"BEGIN:VCALENDAR",
		"VERSION:3.0",
		"PRODID:-//V*//Test//EN",
		"END:VCALENDAR",
	)
	_, err := Parse(strings.NewReader(src))
	if !errors.Is(err, vstar.ErrUnsupportedVersion) {
		t.Errorf("err = %v, want wraps ErrUnsupportedVersion", err)
	}
}

func TestParse_PropertyOrderPreserved(t *testing.T) {
	t.Parallel()

	src := joinCRLF(
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//V*//Order//EN",
		"BEGIN:VTODO",
		"UID:a",
		"DTSTAMP:20260504T120000Z",
		"SUMMARY:first",
		"DESCRIPTION:second",
		"PRIORITY:3",
		"END:VTODO",
		"END:VCALENDAR",
	)
	cal, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	props := cal.Components[0].Props
	wantOrder := []string{"UID", "DTSTAMP", "SUMMARY", "DESCRIPTION", "PRIORITY"}
	if len(props) != len(wantOrder) {
		t.Fatalf("props len = %d, want %d", len(props), len(wantOrder))
	}
	for i, name := range wantOrder {
		if props[i].Name != name {
			t.Errorf("props[%d].Name = %q, want %q", i, props[i].Name, name)
		}
	}
}
