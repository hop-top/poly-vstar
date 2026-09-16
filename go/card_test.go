// SPDX-License-Identifier: MIT

package vstar

import "testing"

func TestCard_Get(t *testing.T) {
	card := Card{
		UID: "card:1",
		Props: []Property{
			{Name: "FN", Value: "Jad"},
			{Name: "EMAIL", Value: "jad@example.com"},
		},
	}
	got, ok := card.Get("fn")
	if !ok || got.Value != "Jad" {
		t.Errorf("Get(fn) = %+v ok=%v, want FN=Jad", got, ok)
	}
}

func TestCard_Get_Missing(t *testing.T) {
	card := Card{Props: []Property{{Name: "FN", Value: "Jad"}}}
	got, ok := card.Get("EMAIL")
	if ok {
		t.Errorf("Get on missing ok=true")
	}
	if got.Name != "" {
		t.Errorf("Get on missing returned non-zero %+v", got)
	}
}

func TestCard_GetAll(t *testing.T) {
	card := Card{
		Props: []Property{
			{Name: "EMAIL", Value: "a@example.com"},
			{Name: "FN", Value: "X"},
			{Name: "email", Value: "b@example.com"},
		},
	}
	got := card.GetAll("EMAIL")
	if len(got) != 2 {
		t.Fatalf("GetAll(EMAIL) = %d, want 2", len(got))
	}
	if got[0].Value != "a@example.com" || got[1].Value != "b@example.com" {
		t.Errorf("GetAll order wrong: %+v", got)
	}
}

func TestCard_Set_ReplacesAll(t *testing.T) {
	card := Card{
		Props: []Property{
			{Name: "FN", Value: "Jad"},
			{Name: "EMAIL", Value: "a@example.com"},
			{Name: "EMAIL", Value: "b@example.com"},
		},
	}
	card.Set(Property{Name: "email", Value: "c@example.com"})
	got := card.GetAll("EMAIL")
	if len(got) != 1 || got[0].Value != "c@example.com" {
		t.Errorf("Set replace: %+v", got)
	}
	if fn, ok := card.Get("FN"); !ok || fn.Value != "Jad" {
		t.Errorf("Set should not touch FN, got %+v ok=%v", fn, ok)
	}
}

func TestCard_Set_AppendsWhenAbsent(t *testing.T) {
	card := Card{Props: []Property{{Name: "FN", Value: "Jad"}}}
	card.Set(Property{Name: "EMAIL", Value: "jad@example.com"})
	got, ok := card.Get("EMAIL")
	if !ok || got.Value != "jad@example.com" {
		t.Errorf("Set absent: %+v ok=%v", got, ok)
	}
}

func TestCard_Add(t *testing.T) {
	card := Card{Props: []Property{{Name: "EMAIL", Value: "a@example.com"}}}
	card.Add(Property{Name: "EMAIL", Value: "b@example.com"})
	got := card.GetAll("EMAIL")
	if len(got) != 2 {
		t.Errorf("Add: %d, want 2", len(got))
	}
}

func TestCard_Remove(t *testing.T) {
	card := Card{
		Props: []Property{
			{Name: "FN", Value: "Jad"},
			{Name: "EMAIL", Value: "a@example.com"},
			{Name: "EMAIL", Value: "b@example.com"},
		},
	}
	card.Remove("email")
	if got := card.GetAll("EMAIL"); got != nil {
		t.Errorf("after Remove, GetAll(EMAIL) = %+v", got)
	}
	if fn, ok := card.Get("FN"); !ok || fn.Value != "Jad" {
		t.Errorf("Remove should not touch FN, got %+v ok=%v", fn, ok)
	}
}

func TestCard_Kind(t *testing.T) {
	// Card carries a Kind field per RFC 6350 §6.1.4.
	card := Card{UID: "g1", Kind: Kind("group")}
	if card.Kind != Kind("group") {
		t.Errorf("Kind = %q, want group", card.Kind)
	}
}
