// SPDX-License-Identifier: MIT

package diff

import (
	"strings"
	"testing"

	vstar "hop.top/vstar"
)

func TestString_EmptyDiff(t *testing.T) {
	if got := (ComponentDiff{}).String(); got != "" {
		t.Errorf("empty diff String() = %q, want empty string", got)
	}
}

func TestString_AddedRemovedChanged(t *testing.T) {
	// Pull the cancel-status wire string from vstar's typed constant
	// (its declaration carries the lint suppression for the
	// double-L spelling required by RFC 5545 §3.8.1.11).
	cancelStatus := string(vstar.TodoCancelled)
	a := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("SUMMARY", "Old summary"),
		prop("STATUS", "COMPLETED"),
	)
	b := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("DTSTART", "20260504T120000Z"),
		prop("STATUS", cancelStatus),
	)
	d := OfComponent(a, b)
	got := d.String()

	// Header line.
	if !strings.HasPrefix(got, "--- ") {
		t.Errorf("expected leading --- header, got: %s", got)
	}

	wantLines := []string{
		"+ DTSTART:20260504T120000Z",
		"- SUMMARY:Old summary",
		"~ STATUS: COMPLETED -> " + cancelStatus,
	}
	for _, w := range wantLines {
		if !strings.Contains(got, w) {
			t.Errorf("output missing %q\nfull output:\n%s", w, got)
		}
	}
}

func TestString_PathRendered(t *testing.T) {
	a := mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "Old"))
	b := mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "New"))
	d := OfComponent(a, b)
	d.Path = "VCALENDAR.VEVENT[uid=evt-1]"
	got := d.String()
	if !strings.Contains(got, "--- VCALENDAR.VEVENT[uid=evt-1]") {
		t.Errorf("expected Path in header, got:\n%s", got)
	}
}

func TestString_NestedIndented(t *testing.T) {
	mkAlarm := func(trigger string) vstar.Component {
		return vstar.Component{Type: vstar.CompAlarm, Props: []vstar.Property{
			prop("ACTION", "DISPLAY"),
			prop("TRIGGER", trigger),
		}}
	}
	a := vstar.Component{Type: vstar.CompEvent, Props: []vstar.Property{
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
	}, Sub: []vstar.Component{mkAlarm("-PT15M")}}
	b := vstar.Component{Type: vstar.CompEvent, Props: []vstar.Property{
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
	}, Sub: []vstar.Component{mkAlarm("-PT5M")}}
	got := OfComponent(a, b).String()

	if !strings.Contains(got, "  --- ") && !strings.Contains(got, "  ~ TRIGGER:") {
		t.Errorf("nested sub-diff should be indented two spaces, got:\n%s", got)
	}
	if !strings.Contains(got, "VALARM[#0]") {
		t.Errorf("expected nested label VALARM[#0] in output, got:\n%s", got)
	}
}

func TestString_ParamsRendered(t *testing.T) {
	a := mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"))
	b := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop(
			"ATTENDEE", "mailto:jad@example.com",
			vstar.Param{Name: "CN", Value: "Jad"},
			vstar.Param{Name: "ROLE", Value: "REQ-PARTICIPANT"},
		),
	)
	got := OfComponent(a, b).String()
	if !strings.Contains(got, "+ ATTENDEE;CN=Jad;ROLE=REQ-PARTICIPANT:mailto:jad@example.com") {
		t.Errorf("ATTENDEE not rendered with params, got:\n%s", got)
	}
}
