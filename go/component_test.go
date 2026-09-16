// SPDX-License-Identifier: MIT

package vstar

import "testing"

func newAttendee(value, role string) Property {
	return Property{
		Name:   "ATTENDEE",
		Params: []Param{{Name: "ROLE", Value: role}},
		Value:  value,
	}
}

func TestComponent_Get(t *testing.T) {
	c := Component{
		Props: []Property{
			{Name: "UID", Value: "abc"},
			{Name: "SUMMARY", Value: "first"},
			{Name: "SUMMARY", Value: "second"},
		},
	}
	got, ok := c.Get("summary")
	if !ok {
		t.Fatalf("Get(summary) ok=false, want true")
	}
	if got.Value != "first" {
		t.Errorf("Get(summary).Value = %q, want %q", got.Value, "first")
	}
}

func TestComponent_Get_MissingReturnsZero(t *testing.T) {
	c := Component{Props: []Property{{Name: "UID", Value: "abc"}}}
	got, ok := c.Get("DTSTAMP")
	if ok {
		t.Errorf("Get(DTSTAMP) ok=true on missing prop")
	}
	if got.Name != "" || got.Value != "" || got.Params != nil {
		t.Errorf("Get on missing returned non-zero %+v", got)
	}
}

func TestComponent_GetAll(t *testing.T) {
	c := Component{
		Props: []Property{
			{Name: "UID", Value: "abc"},
			{Name: "ATTENDEE", Value: "mailto:a@example.com"},
			{Name: "SUMMARY", Value: "hi"},
			{Name: "attendee", Value: "mailto:b@example.com"},
		},
	}
	got := c.GetAll("ATTENDEE")
	if len(got) != 2 {
		t.Fatalf("GetAll(ATTENDEE) returned %d, want 2", len(got))
	}
	if got[0].Value != "mailto:a@example.com" || got[1].Value != "mailto:b@example.com" {
		t.Errorf("GetAll order wrong: %+v", got)
	}
}

func TestComponent_GetAll_MissingReturnsNil(t *testing.T) {
	c := Component{Props: []Property{{Name: "UID", Value: "abc"}}}
	got := c.GetAll("ATTENDEE")
	if got != nil {
		t.Errorf("GetAll on missing = %+v, want nil", got)
	}
}

func TestComponent_Set_ReplacesAll(t *testing.T) {
	c := Component{
		Props: []Property{
			{Name: "UID", Value: "abc"},
			{Name: "ATTENDEE", Value: "mailto:a@example.com"},
			{Name: "ATTENDEE", Value: "mailto:b@example.com"},
		},
	}
	c.Set(Property{Name: "attendee", Value: "mailto:c@example.com"})
	got := c.GetAll("ATTENDEE")
	if len(got) != 1 {
		t.Fatalf("after Set, GetAll(ATTENDEE) returned %d, want 1", len(got))
	}
	if got[0].Value != "mailto:c@example.com" {
		t.Errorf("Set value = %q, want %q", got[0].Value, "mailto:c@example.com")
	}
	if uid, ok := c.Get("UID"); !ok || uid.Value != "abc" {
		t.Errorf("Set should not touch UID, got %+v ok=%v", uid, ok)
	}
}

func TestComponent_Set_AppendsWhenAbsent(t *testing.T) {
	c := Component{Props: []Property{{Name: "UID", Value: "abc"}}}
	c.Set(Property{Name: "SUMMARY", Value: "hi"})
	got, ok := c.Get("SUMMARY")
	if !ok || got.Value != "hi" {
		t.Errorf("Set on absent prop = %+v ok=%v", got, ok)
	}
}

func TestComponent_Add_Appends(t *testing.T) {
	c := Component{Props: []Property{newAttendee("mailto:a@example.com", "REQ-PARTICIPANT")}}
	c.Add(newAttendee("mailto:b@example.com", "OPT-PARTICIPANT"))
	got := c.GetAll("ATTENDEE")
	if len(got) != 2 {
		t.Fatalf("Add: got %d ATTENDEEs, want 2", len(got))
	}
	if got[0].Value != "mailto:a@example.com" || got[1].Value != "mailto:b@example.com" {
		t.Errorf("Add order wrong: %+v", got)
	}
}

func TestComponent_Remove_DeletesAll(t *testing.T) {
	c := Component{
		Props: []Property{
			{Name: "UID", Value: "abc"},
			{Name: "ATTENDEE", Value: "mailto:a@example.com"},
			{Name: "ATTENDEE", Value: "mailto:b@example.com"},
		},
	}
	c.Remove("attendee")
	if got := c.GetAll("ATTENDEE"); got != nil {
		t.Errorf("after Remove, GetAll(ATTENDEE) = %+v, want nil", got)
	}
	if uid, ok := c.Get("UID"); !ok || uid.Value != "abc" {
		t.Errorf("Remove should not touch UID, got %+v ok=%v", uid, ok)
	}
}

func TestComponent_Remove_MissingNoOp(t *testing.T) {
	c := Component{Props: []Property{{Name: "UID", Value: "abc"}}}
	c.Remove("SUMMARY")
	if len(c.Props) != 1 {
		t.Errorf("Remove on missing changed Props: %+v", c.Props)
	}
}

func TestComponent_UID(t *testing.T) {
	c := Component{Props: []Property{{Name: "UID", Value: "evt:1"}}}
	if got := c.UID(); got != "evt:1" {
		t.Errorf("UID() = %q, want %q", got, "evt:1")
	}
	empty := Component{}
	if got := empty.UID(); got != "" {
		t.Errorf("UID() on empty = %q, want %q", got, "")
	}
}

func TestComponent_DTSTAMPRaw(t *testing.T) {
	c := Component{Props: []Property{{Name: "DTSTAMP", Value: "20260504T180000Z"}}}
	if got := c.DTSTAMPRaw(); got != "20260504T180000Z" {
		t.Errorf("DTSTAMPRaw() = %q, want %q", got, "20260504T180000Z")
	}
	empty := Component{}
	if got := empty.DTSTAMPRaw(); got != "" {
		t.Errorf("DTSTAMPRaw() on empty = %q, want %q", got, "")
	}
}

func TestComponent_Sub(t *testing.T) {
	// Verify Component.Sub field exists and holds nested components.
	tz := Component{Type: CompType("VTIMEZONE"), Props: []Property{{Name: "TZID", Value: "America/New_York"}}}
	cal := Component{
		Type: CompType("VCALENDAR"),
		Sub:  []Component{tz},
	}
	if len(cal.Sub) != 1 {
		t.Fatalf("Sub length = %d, want 1", len(cal.Sub))
	}
	if cal.Sub[0].Type != CompType("VTIMEZONE") {
		t.Errorf("Sub[0].Type = %q, want VTIMEZONE", cal.Sub[0].Type)
	}
}
