// SPDX-License-Identifier: MIT

package vstar

import "testing"

func TestCalendar_Find(t *testing.T) {
	cal := Calendar{
		ProdID: "-//hop-top//vstar//EN",
		Components: []Component{
			{Type: CompType("VEVENT"), Props: []Property{{Name: "UID", Value: "evt:1"}}},
			{Type: CompType("VTODO"), Props: []Property{{Name: "UID", Value: "todo:1"}}},
		},
	}
	got, ok := cal.Find("todo:1")
	if !ok {
		t.Fatalf("Find(todo:1) ok=false, want true")
	}
	if got.Type != CompType("VTODO") {
		t.Errorf("Find(todo:1).Type = %q, want VTODO", got.Type)
	}
}

func TestCalendar_Find_CaseSensitive(t *testing.T) {
	// RFC 5545 UID is case-sensitive; Find must NOT match different case.
	cal := Calendar{
		Components: []Component{
			{Props: []Property{{Name: "UID", Value: "evt:ABC"}}},
		},
	}
	if _, ok := cal.Find("evt:abc"); ok {
		t.Errorf("Find should be case-sensitive on UID")
	}
}

func TestCalendar_Find_Missing(t *testing.T) {
	cal := Calendar{Components: []Component{{Props: []Property{{Name: "UID", Value: "x"}}}}}
	got, ok := cal.Find("missing")
	if ok {
		t.Errorf("Find(missing) ok=true, want false")
	}
	if got.Type != "" || got.Props != nil {
		t.Errorf("Find on missing returned non-zero %+v", got)
	}
}

func TestCalendar_Append(t *testing.T) {
	cal := Calendar{}
	cal.Append(Component{Type: CompType("VEVENT"), Props: []Property{{Name: "UID", Value: "e1"}}})
	cal.Append(Component{Type: CompType("VTODO"), Props: []Property{{Name: "UID", Value: "t1"}}})
	if len(cal.Components) != 2 {
		t.Fatalf("Append: Components = %d, want 2", len(cal.Components))
	}
	if cal.Components[0].Type != CompType("VEVENT") || cal.Components[1].Type != CompType("VTODO") {
		t.Errorf("Append order wrong: %+v", cal.Components)
	}
}

func TestCalendar_Filter(t *testing.T) {
	cal := Calendar{
		Components: []Component{
			{Type: CompType("VEVENT"), Props: []Property{{Name: "UID", Value: "e1"}}},
			{Type: CompType("VTODO"), Props: []Property{{Name: "UID", Value: "t1"}}},
			{Type: CompType("VEVENT"), Props: []Property{{Name: "UID", Value: "e2"}}},
		},
	}
	got := cal.Filter(CompType("VEVENT"))
	if len(got) != 2 {
		t.Fatalf("Filter(VEVENT) returned %d, want 2", len(got))
	}
	if got[0].UID() != "e1" || got[1].UID() != "e2" {
		t.Errorf("Filter order wrong: %+v", got)
	}
}

func TestCalendar_Filter_NoneReturnsNil(t *testing.T) {
	cal := Calendar{Components: []Component{{Type: CompType("VEVENT")}}}
	got := cal.Filter(CompType("VTODO"))
	if got != nil {
		t.Errorf("Filter no match = %+v, want nil", got)
	}
}
