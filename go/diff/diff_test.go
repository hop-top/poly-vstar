// SPDX-License-Identifier: MIT

package diff

import (
	"reflect"
	"testing"

	vstar "hop.top/vstar"
)

func TestComponentDiff_Identical_Empty(t *testing.T) {
	a := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("SUMMARY", "Standup"),
	)
	d := OfComponent(a, a)
	if !d.Empty() {
		t.Fatalf("identical components should produce empty diff, got %+v", d)
	}
}

func TestComponentDiff_PropertyAdded(t *testing.T) {
	a := mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"))
	b := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("SUMMARY", "Standup"),
	)
	d := OfComponent(a, b)
	if len(d.Properties) != 1 {
		t.Fatalf("expected exactly 1 property diff, got %d: %+v", len(d.Properties), d)
	}
	pd := d.Properties[0]
	if pd.Op != OpAdded {
		t.Errorf("expected OpAdded, got %v", pd.Op)
	}
	if pd.Property.Name != "SUMMARY" || pd.Property.Value != "Standup" {
		t.Errorf("unexpected added property: %+v", pd.Property)
	}
	if pd.Old.Name != "" {
		t.Errorf("Old must be zero on OpAdded, got %+v", pd.Old)
	}
}

func TestComponentDiff_PropertyRemoved(t *testing.T) {
	a := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("SUMMARY", "Standup"),
	)
	b := mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"))
	d := OfComponent(a, b)
	if len(d.Properties) != 1 {
		t.Fatalf("expected exactly 1 property diff, got %d", len(d.Properties))
	}
	pd := d.Properties[0]
	if pd.Op != OpRemoved {
		t.Errorf("expected OpRemoved, got %v", pd.Op)
	}
	if pd.Property.Name != "SUMMARY" || pd.Property.Value != "Standup" {
		t.Errorf("unexpected removed property: %+v", pd.Property)
	}
}

func TestComponentDiff_PropertyChanged(t *testing.T) {
	// Pull the cancel-status wire string from vstar's typed constant
	// (its declaration carries the lint suppression for the
	// double-L spelling required by RFC 5545 §3.8.1.11).
	cancelStatus := string(vstar.TodoCancelled)
	a := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("STATUS", "COMPLETED"),
	)
	b := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("STATUS", cancelStatus),
	)
	d := OfComponent(a, b)
	if len(d.Properties) != 1 {
		t.Fatalf("expected exactly 1 property diff, got %d", len(d.Properties))
	}
	pd := d.Properties[0]
	if pd.Op != OpChanged {
		t.Errorf("expected OpChanged, got %v", pd.Op)
	}
	if pd.Property.Value != cancelStatus {
		t.Errorf("expected new value %q, got %q", cancelStatus, pd.Property.Value)
	}
	if pd.Old.Value != "COMPLETED" {
		t.Errorf("expected old value COMPLETED, got %q", pd.Old.Value)
	}
}

func TestComponentDiff_ParamChanged_IsChanged(t *testing.T) {
	a := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop(
			"ATTENDEE", "mailto:jad@example.com",
			vstar.Param{Name: "CN", Value: "Jad"},
		),
	)
	b := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop(
			"ATTENDEE", "mailto:jad@example.com",
			vstar.Param{Name: "CN", Value: "Jad B"},
		),
	)
	d := OfComponent(a, b)
	if len(d.Properties) != 1 || d.Properties[0].Op != OpChanged {
		t.Fatalf("expected one OpChanged for ATTENDEE param change, got %+v", d)
	}
}

func TestComponentDiff_PropertiesSortedByName(t *testing.T) {
	a := mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"))
	b := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("SUMMARY", "Z"),
		prop("DESCRIPTION", "A"),
		prop("LOCATION", "M"),
	)
	d := OfComponent(a, b)
	if len(d.Properties) != 3 {
		t.Fatalf("expected 3 added properties, got %d", len(d.Properties))
	}
	names := []string{
		d.Properties[0].Property.Name,
		d.Properties[1].Property.Name,
		d.Properties[2].Property.Name,
	}
	want := []string{"DESCRIPTION", "LOCATION", "SUMMARY"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("Properties not sorted by Name: got %v, want %v", names, want)
	}
}

func TestComponentDiff_XVSTARHashIgnored(t *testing.T) {
	a := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("X-VSTAR-HASH", "sha256:0000000000000000000000000000000000000000000000000000000000000000"),
	)
	b := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("X-VSTAR-HASH", "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
	)
	d := OfComponent(a, b)
	if !d.Empty() {
		t.Fatalf("X-VSTAR-HASH must be excluded from diff, got %+v", d)
	}
}

func TestComponentDiff_NestedSubAdded(t *testing.T) {
	alarm := vstar.Component{Type: vstar.CompAlarm, Props: []vstar.Property{
		prop("ACTION", "DISPLAY"),
		prop("TRIGGER", "-PT15M"),
	}}
	a := vstar.Component{Type: vstar.CompEvent, Props: []vstar.Property{
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
	}}
	b := vstar.Component{Type: vstar.CompEvent, Props: []vstar.Property{
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
	}, Sub: []vstar.Component{alarm}}
	d := OfComponent(a, b)
	if len(d.SubDiffs) != 1 {
		t.Fatalf("expected 1 sub diff, got %d: %+v", len(d.SubDiffs), d)
	}
	sd := d.SubDiffs[0]
	if len(sd.Properties) != 2 {
		t.Fatalf("expected 2 added properties on new VALARM, got %d", len(sd.Properties))
	}
	for _, pd := range sd.Properties {
		if pd.Op != OpAdded {
			t.Errorf("new VALARM properties should all be OpAdded, got %v for %s", pd.Op, pd.Property.Name)
		}
	}
}

func TestComponentDiff_NestedSubChanged(t *testing.T) {
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
	d := OfComponent(a, b)
	if len(d.SubDiffs) != 1 || len(d.SubDiffs[0].Properties) != 1 || d.SubDiffs[0].Properties[0].Op != OpChanged {
		t.Fatalf("expected nested OpChanged for TRIGGER, got %+v", d)
	}
	pd := d.SubDiffs[0].Properties[0]
	if pd.Property.Name != "TRIGGER" || pd.Property.Value != "-PT5M" || pd.Old.Value != "-PT15M" {
		t.Errorf("unexpected nested change: %+v", pd)
	}
}

func TestOfCard_PropertyChanged(t *testing.T) {
	a := vstar.Card{UID: "u", Kind: vstar.KindIndividual, Props: []vstar.Property{
		prop("FN", "Jad"),
		prop("EMAIL", "jad@example.com"),
	}}
	b := vstar.Card{UID: "u", Kind: vstar.KindIndividual, Props: []vstar.Property{
		prop("FN", "Jad B."),
		prop("EMAIL", "jad@example.com"),
	}}
	d := OfCard(a, b)
	if len(d.Properties) != 1 || d.Properties[0].Op != OpChanged {
		t.Fatalf("expected one OpChanged for FN, got %+v", d)
	}
	if d.Properties[0].Old.Value != "Jad" || d.Properties[0].Property.Value != "Jad B." {
		t.Errorf("Old/new mismatch: %+v", d.Properties[0])
	}
	if len(d.SubDiffs) != 0 {
		t.Errorf("Card diff should never have SubDiffs, got %d", len(d.SubDiffs))
	}
}

func TestOfCard_XVSTARHashIgnored(t *testing.T) {
	a := vstar.Card{UID: "u", Kind: vstar.KindIndividual, Props: []vstar.Property{
		prop("FN", "Jad"),
		prop("X-VSTAR-HASH", "sha256:0000000000000000000000000000000000000000000000000000000000000000"),
	}}
	b := vstar.Card{UID: "u", Kind: vstar.KindIndividual, Props: []vstar.Property{
		prop("FN", "Jad"),
		prop("X-VSTAR-HASH", "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
	}}
	d := OfCard(a, b)
	if !d.Empty() {
		t.Fatalf("X-VSTAR-HASH must be ignored in card diff, got %+v", d)
	}
}

func TestOfCalendar_Identical_NoEntries(t *testing.T) {
	mk := func() vstar.Calendar {
		return vstar.Calendar{
			ProdID: "-//V*//Test//EN",
			Components: []vstar.Component{
				mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "A")),
			},
		}
	}
	if got := OfCalendar(mk(), mk()); len(got) != 0 {
		t.Fatalf("identical calendars should produce zero diff entries, got %+v", got)
	}
}

func TestOfCalendar_ComponentAdded(t *testing.T) {
	a := vstar.Calendar{
		ProdID: "-//V*//Test//EN",
		Components: []vstar.Component{
			mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "A")),
		},
	}
	b := vstar.Calendar{
		ProdID: "-//V*//Test//EN",
		Components: []vstar.Component{
			mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "A")),
			mkVEVENT(prop("UID", "evt-2"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "B")),
		},
	}
	got := OfCalendar(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 diff entry for added component, got %d", len(got))
	}
	if got[0].Path != "VCALENDAR.VEVENT[uid=evt-2]" {
		t.Errorf("unexpected Path: %q", got[0].Path)
	}
	if len(got[0].Properties) != 3 {
		t.Fatalf("expected 3 added properties, got %d", len(got[0].Properties))
	}
	for _, pd := range got[0].Properties {
		if pd.Op != OpAdded {
			t.Errorf("all properties of new component should be OpAdded, got %v for %s", pd.Op, pd.Property.Name)
		}
	}
}

func TestOfCalendar_ComponentRemoved(t *testing.T) {
	a := vstar.Calendar{
		ProdID: "-//V*//Test//EN",
		Components: []vstar.Component{
			mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "A")),
			mkVEVENT(prop("UID", "evt-2"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "B")),
		},
	}
	b := vstar.Calendar{
		ProdID: "-//V*//Test//EN",
		Components: []vstar.Component{
			mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "A")),
		},
	}
	got := OfCalendar(a, b)
	if len(got) != 1 || got[0].Path != "VCALENDAR.VEVENT[uid=evt-2]" {
		t.Fatalf("expected one VEVENT[uid=evt-2] diff entry, got %+v", got)
	}
	for _, pd := range got[0].Properties {
		if pd.Op != OpRemoved {
			t.Errorf("removed component properties must all be OpRemoved, got %v", pd.Op)
		}
	}
}

func TestOfCalendar_ComponentChanged(t *testing.T) {
	a := vstar.Calendar{
		ProdID: "-//V*//Test//EN",
		Components: []vstar.Component{
			mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "Old")),
		},
	}
	b := vstar.Calendar{
		ProdID: "-//V*//Test//EN",
		Components: []vstar.Component{
			mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "New")),
		},
	}
	got := OfCalendar(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 entry for changed VEVENT, got %d", len(got))
	}
	if got[0].Path != "VCALENDAR.VEVENT[uid=evt-1]" {
		t.Errorf("unexpected Path: %q", got[0].Path)
	}
	if len(got[0].Properties) != 1 || got[0].Properties[0].Op != OpChanged {
		t.Fatalf("expected one OpChanged for SUMMARY, got %+v", got[0])
	}
}

func TestOfCalendar_OrderingByUID(t *testing.T) {
	// Same calendar, different input order — diff entries should be
	// emitted in canonical (UID-sorted) order regardless of input.
	a := vstar.Calendar{
		ProdID: "-//V*//Test//EN",
		Components: []vstar.Component{
			mkVEVENT(prop("UID", "evt-2"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "Old2")),
			mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "Old1")),
		},
	}
	b := vstar.Calendar{
		ProdID: "-//V*//Test//EN",
		Components: []vstar.Component{
			mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "New1")),
			mkVEVENT(prop("UID", "evt-2"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "New2")),
		},
	}
	got := OfCalendar(a, b)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].Path != "VCALENDAR.VEVENT[uid=evt-1]" {
		t.Errorf("first entry Path = %q, want VCALENDAR.VEVENT[uid=evt-1]", got[0].Path)
	}
	if got[1].Path != "VCALENDAR.VEVENT[uid=evt-2]" {
		t.Errorf("second entry Path = %q, want VCALENDAR.VEVENT[uid=evt-2]", got[1].Path)
	}
}

func TestComponentDiff_SubMatchedByUID(t *testing.T) {
	mkChild := func(uid, summary string) vstar.Component {
		return vstar.Component{Type: vstar.CompEvent, Props: []vstar.Property{
			prop("UID", uid),
			prop("DTSTAMP", "20260504T120000Z"),
			prop("SUMMARY", summary),
		}}
	}
	// Same children, different order
	a := vstar.Component{Type: vstar.CompEvent, Props: []vstar.Property{
		prop("UID", "parent"),
		prop("DTSTAMP", "20260504T120000Z"),
	}, Sub: []vstar.Component{mkChild("c1", "A"), mkChild("c2", "B")}}
	b := vstar.Component{Type: vstar.CompEvent, Props: []vstar.Property{
		prop("UID", "parent"),
		prop("DTSTAMP", "20260504T120000Z"),
	}, Sub: []vstar.Component{mkChild("c2", "B"), mkChild("c1", "A")}}
	d := OfComponent(a, b)
	if !d.Empty() {
		t.Fatalf("UID-paired identical sub-components should produce empty diff, got %+v", d)
	}
}

// Property names are compared and ordered case-insensitively, so
// "X-alpha" precedes "X-Beta". A byte sort would reverse them, since
// every uppercase letter sorts below every lowercase one ('B' < 'a').
//
// This cannot be pinned by a spec/behavior/diff fixture: the RFC 5545
// encoder upper-cases every property name on write, so both names
// reach a port as "X-ALPHA"/"X-BETA" and the two orderings become
// indistinguishable. The rule only has teeth on in-memory components,
// which is what this test builds.
func TestComponentDiff_OpsOrderedCaseInsensitively(t *testing.T) {
	a := mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"))
	b := mkVEVENT(
		prop("UID", "evt-1"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("X-Beta", "b"),
		prop("X-alpha", "a"),
	)
	d := OfComponent(a, b)
	var names []string
	for _, pd := range d.Properties {
		names = append(names, pd.Property.Name)
	}
	want := []string{"X-alpha", "X-Beta"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("ops order = %v, want %v (case-insensitive, not a byte sort)", names, want)
	}
}

// Case-insensitive pairing: a property renamed only in case is not a
// change at all. A byte-keyed pairing would report it as one remove
// plus one add.
func TestComponentDiff_PropertyPairedCaseInsensitively(t *testing.T) {
	a := mkVEVENT(prop("UID", "evt-1"), prop("summary", "Standup"))
	b := mkVEVENT(prop("UID", "evt-1"), prop("SUMMARY", "Standup"))
	d := OfComponent(a, b)
	if !d.Empty() {
		t.Errorf("a case-only rename must pair, not diff; got %+v", d.Properties)
	}
}

// A wholly added or removed component takes a separate code path
// (allAddedDiff / allRemovedDiff) that sorts its own ops rather than
// inheriting the order from pairing. That sort is case-insensitive
// too, and nothing else in this package exercises it.
func TestOfCalendar_AddedComponentOpsOrderedCaseInsensitively(t *testing.T) {
	a := vstar.Calendar{ProdID: "-//V*//Test//EN"}
	b := vstar.Calendar{
		ProdID: "-//V*//Test//EN",
		Components: []vstar.Component{
			mkVEVENT(
				prop("UID", "evt-1"),
				prop("DTSTAMP", "20260504T120000Z"),
				prop("X-Beta", "b"),
				prop("X-alpha", "a"),
			),
		},
	}
	got := OfCalendar(a, b)
	if len(got) != 1 {
		t.Fatalf("expected 1 component diff, got %d", len(got))
	}
	var names []string
	for _, pd := range got[0].Properties {
		names = append(names, pd.Property.Name)
	}
	want := []string{"DTSTAMP", "UID", "X-alpha", "X-Beta"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("added-component ops order = %v, want %v", names, want)
	}
}
