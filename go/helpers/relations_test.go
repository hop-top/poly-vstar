// SPDX-License-Identifier: MIT

package helpers_test

import (
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/helpers"
)

func TestRelatedTo_emptyWhenAbsent(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	if got := helpers.RelatedTo(c); len(got) != 0 {
		t.Errorf("RelatedTo = %v, want empty", got)
	}
}

func TestAddRelatedTo_appendsWithRELTYPE(t *testing.T) {
	c, _ := helpers.NewTodo("child-uid", time.Now().UTC())
	pre, _ := c.Get("X-VSTAR-HASH")
	helpers.AddRelatedTo(&c, "parent-uid", "PARENT")
	post, _ := c.Get("X-VSTAR-HASH")
	if pre.Value == post.Value {
		t.Errorf("X-VSTAR-HASH unchanged")
	}
	got := helpers.RelatedTo(c)
	if len(got) != 1 {
		t.Fatalf("RelatedTo len = %d, want 1", len(got))
	}
	if got[0].UID != "parent-uid" || got[0].RelType != "PARENT" {
		t.Errorf("ref = %+v, want {parent-uid PARENT}", got[0])
	}
}

func TestAddRelatedTo_multipleRoundTrip(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	helpers.AddRelatedTo(&c, "p", "PARENT")
	helpers.AddRelatedTo(&c, "s1", "SIBLING")
	helpers.AddRelatedTo(&c, "s2", "SIBLING")
	helpers.AddRelatedTo(&c, "k", "CHILD")
	got := helpers.RelatedTo(c)
	if len(got) != 4 {
		t.Fatalf("RelatedTo len = %d, want 4: %+v", len(got), got)
	}
	wantPairs := []helpers.RelatedRef{
		{UID: "p", RelType: "PARENT"},
		{UID: "s1", RelType: "SIBLING"},
		{UID: "s2", RelType: "SIBLING"},
		{UID: "k", RelType: "CHILD"},
	}
	for i, w := range wantPairs {
		if got[i] != w {
			t.Errorf("ref[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestAddRelatedTo_emptyRelTypeOmitsParam(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	helpers.AddRelatedTo(&c, "parent-uid", "")
	got := helpers.RelatedTo(c)
	if len(got) != 1 {
		t.Fatalf("RelatedTo len = %d, want 1", len(got))
	}
	// RFC 5545 §3.2.15 default RELTYPE is PARENT when omitted.
	if got[0].UID != "parent-uid" || got[0].RelType != "PARENT" {
		t.Errorf("ref = %+v, want {parent-uid PARENT (default)}", got[0])
	}
}

func TestAddRelatedTo_nilIsNoOp(t *testing.T) {
	helpers.AddRelatedTo(nil, "p", "PARENT")
}

func TestAddRelatedTo_emptyUIDIsNoOp(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	helpers.AddRelatedTo(&c, "", "PARENT")
	if got := helpers.RelatedTo(c); len(got) != 0 {
		t.Errorf("RelatedTo len = %d, want 0 (empty UID skipped)", len(got))
	}
}

func TestAddRelatedTo_typedConstantsRoundTrip(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	helpers.AddRelatedTo(&c, "p", vstar.RelParent)
	helpers.AddRelatedTo(&c, "k", vstar.RelChild)
	helpers.AddRelatedTo(&c, "s", vstar.RelSibling)
	helpers.AddRelatedTo(&c, "d", vstar.RelDependsOn)
	got := helpers.RelatedTo(c)
	want := []helpers.RelatedRef{
		{UID: "p", RelType: vstar.RelParent},
		{UID: "k", RelType: vstar.RelChild},
		{UID: "s", RelType: vstar.RelSibling},
		{UID: "d", RelType: vstar.RelDependsOn},
	}
	if len(got) != len(want) {
		t.Fatalf("RelatedTo len = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("ref[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestAddRelatedTo_temporalConstantsRoundTrip(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	helpers.AddRelatedTo(&c, "a", vstar.RelFinishToStart)
	helpers.AddRelatedTo(&c, "b", vstar.RelFinishToFinish)
	helpers.AddRelatedTo(&c, "c", vstar.RelStartToFinish)
	helpers.AddRelatedTo(&c, "d", vstar.RelStartToStart)
	got := helpers.RelatedTo(c)
	want := []helpers.RelatedRef{
		{UID: "a", RelType: vstar.RelFinishToStart},
		{UID: "b", RelType: vstar.RelFinishToFinish},
		{UID: "c", RelType: vstar.RelStartToFinish},
		{UID: "d", RelType: vstar.RelStartToStart},
	}
	if len(got) != len(want) {
		t.Fatalf("RelatedTo len = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("ref[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestRelatedTo_normalizesCaseToConstant(t *testing.T) {
	// RFC 5545 §3.2 parameter values compare case-insensitively; a
	// lowercase wire RELTYPE must read back as the canonical constant.
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	c.Add(vstar.Property{
		Name:   "RELATED-TO",
		Value:  "parent-uid",
		Params: []vstar.Param{{Name: "reltype", Value: "child"}},
	})
	got := helpers.RelatedTo(c)
	if len(got) != 1 {
		t.Fatalf("RelatedTo len = %d, want 1", len(got))
	}
	if got[0].RelType != vstar.RelChild {
		t.Errorf("RelType = %q, want %q", got[0].RelType, vstar.RelChild)
	}
}

func TestRelatedTo_unknownRelTypePreservedVerbatim(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	c.Add(vstar.Property{
		Name:   "RELATED-TO",
		Value:  "x-uid",
		Params: []vstar.Param{{Name: "RELTYPE", Value: "X-BLOCKS"}},
	})
	got := helpers.RelatedTo(c)
	if len(got) != 1 {
		t.Fatalf("RelatedTo len = %d, want 1", len(got))
	}
	if string(got[0].RelType) != "X-BLOCKS" {
		t.Errorf("RelType = %q, want verbatim %q", got[0].RelType, "X-BLOCKS")
	}
}

func TestAddRelatedTo_defaultRelTypeIsParent(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	helpers.AddRelatedTo(&c, "parent-uid", "")
	got := helpers.RelatedTo(c)
	if len(got) != 1 {
		t.Fatalf("RelatedTo len = %d, want 1", len(got))
	}
	if got[0].RelType != vstar.RelParent {
		t.Errorf("RelType = %q, want default %q", got[0].RelType, vstar.RelParent)
	}
}

func TestAddRelatedTo_orderingAndReferenceConstantsRoundTrip(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	helpers.AddRelatedTo(&c, "f", vstar.RelFirst)
	helpers.AddRelatedTo(&c, "n", vstar.RelNext)
	helpers.AddRelatedTo(&c, "c", vstar.RelConcept)
	helpers.AddRelatedTo(&c, "r", vstar.RelRefID)
	got := helpers.RelatedTo(c)
	want := []helpers.RelatedRef{
		{UID: "f", RelType: vstar.RelFirst},
		{UID: "n", RelType: vstar.RelNext},
		{UID: "c", RelType: vstar.RelConcept},
		{UID: "r", RelType: vstar.RelRefID},
	}
	if len(got) != len(want) {
		t.Fatalf("RelatedTo len = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("ref[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}
