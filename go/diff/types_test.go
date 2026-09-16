// SPDX-License-Identifier: MIT

package diff

import (
	"testing"

	vstar "hop.top/vstar"
)

func TestDiffOpString(t *testing.T) {
	cases := []struct {
		op   DiffOp
		want string
	}{
		{OpAdded, "Added"},
		{OpRemoved, "Removed"},
		{OpChanged, "Changed"},
		{DiffOp(0), "Unknown"},
		{DiffOp(99), "Unknown"},
	}
	for _, tc := range cases {
		if got := tc.op.String(); got != tc.want {
			t.Errorf("DiffOp(%d).String() = %q, want %q", tc.op, got, tc.want)
		}
	}
}

func TestDiffOpDistinct(t *testing.T) {
	if OpAdded == OpRemoved || OpAdded == OpChanged || OpRemoved == OpChanged {
		t.Fatalf("DiffOp constants must be distinct: added=%d removed=%d changed=%d", OpAdded, OpRemoved, OpChanged)
	}
}

func TestPropertyDiffZero(t *testing.T) {
	var pd PropertyDiff
	if pd.Op != 0 {
		t.Errorf("zero PropertyDiff.Op = %d, want 0", pd.Op)
	}
	if pd.Property.Name != "" || pd.Old.Name != "" {
		t.Errorf("zero PropertyDiff should have empty Property/Old, got %+v", pd)
	}
}

func TestPropertyDiffFields(t *testing.T) {
	p := vstar.Property{Name: "SUMMARY", Value: "new"}
	old := vstar.Property{Name: "SUMMARY", Value: "old"}
	pd := PropertyDiff{Op: OpChanged, Property: p, Old: old}
	if pd.Op != OpChanged || pd.Property.Value != "new" || pd.Old.Value != "old" {
		t.Errorf("PropertyDiff fields not retained: %+v", pd)
	}
}

func TestComponentDiffEmpty(t *testing.T) {
	if !(ComponentDiff{}).Empty() {
		t.Error("zero ComponentDiff must be Empty")
	}
	d := ComponentDiff{Properties: []PropertyDiff{{Op: OpAdded}}}
	if d.Empty() {
		t.Error("ComponentDiff with Properties must not be Empty")
	}
	nested := ComponentDiff{SubDiffs: []ComponentDiff{{Properties: []PropertyDiff{{Op: OpRemoved}}}}}
	if nested.Empty() {
		t.Error("ComponentDiff with non-empty SubDiff must not be Empty")
	}
	emptyNested := ComponentDiff{SubDiffs: []ComponentDiff{{}}}
	if !emptyNested.Empty() {
		t.Error("ComponentDiff whose SubDiffs are all empty must be Empty")
	}
}

func TestComponentDiffPath(t *testing.T) {
	d := ComponentDiff{Path: "VCALENDAR.VEVENT[uid=abc]"}
	if d.Path != "VCALENDAR.VEVENT[uid=abc]" {
		t.Errorf("Path field not retained: %q", d.Path)
	}
}
