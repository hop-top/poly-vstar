// SPDX-License-Identifier: MIT

package diff

import (
	"testing"

	vstar "hop.top/vstar"
)

func mkVEVENT(props ...vstar.Property) vstar.Component {
	return vstar.Component{Type: vstar.CompEvent, Props: props}
}

func prop(name, value string, params ...vstar.Param) vstar.Property {
	return vstar.Property{Name: name, Value: value, Params: params}
}

func TestComponent_IdenticalEqual(t *testing.T) {
	a := mkVEVENT(
		prop("UID", "evt-1@example.com"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("SUMMARY", "Standup"),
	)
	b := mkVEVENT(
		prop("UID", "evt-1@example.com"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("SUMMARY", "Standup"),
	)
	if !Component(a, b) {
		t.Fatal("identical components must be Equal")
	}
}

func TestComponent_PropertyOrderIrrelevant(t *testing.T) {
	a := mkVEVENT(
		prop("UID", "evt-1@example.com"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("SUMMARY", "Standup"),
	)
	b := mkVEVENT(
		prop("SUMMARY", "Standup"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("UID", "evt-1@example.com"),
	)
	if !Component(a, b) {
		t.Fatal("components with same props in different order must be Equal")
	}
}

func TestComponent_ParameterOrderIrrelevant(t *testing.T) {
	a := mkVEVENT(
		prop("UID", "evt-1@example.com"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop(
			"ATTENDEE", "mailto:jad@example.com",
			vstar.Param{Name: "CN", Value: "Jad"},
			vstar.Param{Name: "ROLE", Value: "REQ-PARTICIPANT"},
		),
	)
	b := mkVEVENT(
		prop("UID", "evt-1@example.com"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop(
			"ATTENDEE", "mailto:jad@example.com",
			vstar.Param{Name: "ROLE", Value: "REQ-PARTICIPANT"},
			vstar.Param{Name: "CN", Value: "Jad"},
		),
	)
	if !Component(a, b) {
		t.Fatal("components differing only in param order must be Equal")
	}
}

func TestComponent_ValueDifference_NotEqual(t *testing.T) {
	a := mkVEVENT(
		prop("UID", "evt-1@example.com"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("SUMMARY", "Standup"),
	)
	b := mkVEVENT(
		prop("UID", "evt-1@example.com"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("SUMMARY", "Retrospective"),
	)
	if Component(a, b) {
		t.Fatal("components differing in property value must NOT be Equal")
	}
}

// TestComponent_XVSTARHashIgnored proves that the hash sentinel does
// not factor into Equal — directly verifies spec/03 rule 7.
func TestComponent_XVSTARHashIgnored(t *testing.T) {
	a := mkVEVENT(
		prop("UID", "evt-1@example.com"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("SUMMARY", "Standup"),
		prop("X-VSTAR-HASH", "sha256:0000000000000000000000000000000000000000000000000000000000000000"),
	)
	b := mkVEVENT(
		prop("UID", "evt-1@example.com"),
		prop("DTSTAMP", "20260504T120000Z"),
		prop("SUMMARY", "Standup"),
		prop("X-VSTAR-HASH", "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
	)
	if !Component(a, b) {
		t.Fatal("components differing only in X-VSTAR-HASH must be Equal (spec/03 rule 7)")
	}
}

func TestComponent_TypeDifference_NotEqual(t *testing.T) {
	a := mkVEVENT(prop("UID", "u"), prop("DTSTAMP", "20260504T120000Z"))
	b := vstar.Component{Type: vstar.CompTodo, Props: []vstar.Property{
		prop("UID", "u"),
		prop("DTSTAMP", "20260504T120000Z"),
	}}
	if Component(a, b) {
		t.Fatal("components with different Type must NOT be Equal")
	}
}

func TestCard_IdenticalEqual(t *testing.T) {
	a := vstar.Card{
		UID:  "card-1@example.com",
		Kind: vstar.KindIndividual,
		Props: []vstar.Property{
			prop("FN", "Jad Bitar"),
			prop("EMAIL", "jad@example.com"),
		},
	}
	b := vstar.Card{
		UID:  "card-1@example.com",
		Kind: vstar.KindIndividual,
		Props: []vstar.Property{
			prop("FN", "Jad Bitar"),
			prop("EMAIL", "jad@example.com"),
		},
	}
	if !Card(a, b) {
		t.Fatal("identical cards must be Equal")
	}
}

func TestCard_PropertyOrderIrrelevant(t *testing.T) {
	a := vstar.Card{
		UID:  "card-1@example.com",
		Kind: vstar.KindIndividual,
		Props: []vstar.Property{
			prop("FN", "Jad"),
			prop("EMAIL", "jad@example.com"),
		},
	}
	b := vstar.Card{
		UID:  "card-1@example.com",
		Kind: vstar.KindIndividual,
		Props: []vstar.Property{
			prop("EMAIL", "jad@example.com"),
			prop("FN", "Jad"),
		},
	}
	if !Card(a, b) {
		t.Fatal("cards with different prop order must be Equal")
	}
}

func TestCard_XVSTARHashIgnored(t *testing.T) {
	a := vstar.Card{
		UID:  "card-1@example.com",
		Kind: vstar.KindIndividual,
		Props: []vstar.Property{
			prop("FN", "Jad"),
			prop("X-VSTAR-HASH", "sha256:0000000000000000000000000000000000000000000000000000000000000000"),
		},
	}
	b := vstar.Card{
		UID:  "card-1@example.com",
		Kind: vstar.KindIndividual,
		Props: []vstar.Property{
			prop("FN", "Jad"),
			prop("X-VSTAR-HASH", "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
		},
	}
	if !Card(a, b) {
		t.Fatal("cards differing only in X-VSTAR-HASH must be Equal")
	}
}

func TestCard_FNDifference_NotEqual(t *testing.T) {
	a := vstar.Card{UID: "u", Kind: vstar.KindIndividual, Props: []vstar.Property{prop("FN", "Jad")}}
	b := vstar.Card{UID: "u", Kind: vstar.KindIndividual, Props: []vstar.Property{prop("FN", "Alex")}}
	if Card(a, b) {
		t.Fatal("cards differing in FN must NOT be Equal")
	}
}

func TestCalendar_IdenticalEqual(t *testing.T) {
	mk := func() vstar.Calendar {
		return vstar.Calendar{
			ProdID: "-//V*//Test//EN",
			Components: []vstar.Component{
				mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "A")),
				mkVEVENT(prop("UID", "evt-2"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "B")),
			},
		}
	}
	if !Calendar(mk(), mk()) {
		t.Fatal("identical calendars must be Equal")
	}
}

func TestCalendar_ComponentOrderIrrelevant(t *testing.T) {
	a := vstar.Calendar{
		ProdID: "-//V*//Test//EN",
		Components: []vstar.Component{
			mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z")),
			mkVEVENT(prop("UID", "evt-2"), prop("DTSTAMP", "20260504T120000Z")),
		},
	}
	b := vstar.Calendar{
		ProdID: "-//V*//Test//EN",
		Components: []vstar.Component{
			mkVEVENT(prop("UID", "evt-2"), prop("DTSTAMP", "20260504T120000Z")),
			mkVEVENT(prop("UID", "evt-1"), prop("DTSTAMP", "20260504T120000Z")),
		},
	}
	if !Calendar(a, b) {
		t.Fatal("calendars differing only in component order must be Equal (canonical sorts by UID)")
	}
}

func TestCalendar_XVSTARHashIgnored(t *testing.T) {
	mk := func(hash string) vstar.Calendar {
		return vstar.Calendar{
			ProdID: "-//V*//Test//EN",
			Components: []vstar.Component{
				mkVEVENT(
					prop("UID", "evt-1"),
					prop("DTSTAMP", "20260504T120000Z"),
					prop("SUMMARY", "A"),
					prop("X-VSTAR-HASH", hash),
				),
			},
		}
	}
	if !Calendar(
		mk("sha256:0000000000000000000000000000000000000000000000000000000000000000"),
		mk("sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
	) {
		t.Fatal("calendars differing only in component X-VSTAR-HASH must be Equal")
	}
}

func TestCalendar_PRODIDMatters(t *testing.T) {
	a := vstar.Calendar{ProdID: "-//A//EN", Components: nil}
	b := vstar.Calendar{ProdID: "-//B//EN", Components: nil}
	if Calendar(a, b) {
		t.Fatal("calendars differing in PRODID must NOT be Equal")
	}
}
