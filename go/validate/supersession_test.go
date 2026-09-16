// SPDX-License-Identifier: MIT

package validate_test

import (
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/supersession"
	"hop.top/vstar/validate"
)

// supersessionEntry returns a VJOURNAL with CATEGORIES=status-
// supersession and the supplied (possibly empty) RELATED-TO and
// X-VSTAR-EFFECTIVE-STATUS property values. Hash is stamped so
// VS003/VS010 stay quiet and the test surface is just VS030/VS031.
//
// uid is the VJOURNAL's own UID; relatedTo is the UID it claims to
// supersede. Pass empty string to omit a property entirely.
func supersessionEntry(t *testing.T, uid, relatedTo, status string) vstar.Component {
	t.Helper()
	c := vstar.Component{
		Type: vstar.CompJournal,
		Props: []vstar.Property{
			{Name: "UID", Value: uid},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "CATEGORIES", Value: supersession.CategoryStatusSupersession},
		},
	}
	if relatedTo != "" {
		c.Add(vstar.Property{Name: "RELATED-TO", Value: relatedTo})
	}
	if status != "" {
		c.Add(vstar.Property{Name: supersession.PropEffectiveStatus, Value: status})
	}
	return hashed(c)
}

// targetVTODO returns a minimal hashed VTODO with the given UID,
// suitable as a supersession target in a Calendar.
func targetVTODO(t *testing.T, uid string) vstar.Component {
	t.Helper()
	c := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: uid},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "DUE", Value: "20260601T000000Z"},
		},
	}
	return hashed(c)
}

func TestSupersession_Healthy_NoDiagnostic(t *testing.T) {
	cal := vstar.Calendar{
		ProdID: "-//test//EN",
		Components: []vstar.Component{
			targetVTODO(t, "todo-1"),
			supersessionEntry(t, "j-1", "todo-1", "completed"),
		},
	}
	got := validate.Validate(cal)
	if hasCode(got, validate.CodeSupersessionMissingProps) {
		t.Errorf("VS030 must not fire on a complete supersession entry; got %v", got)
	}
	if hasCode(got, validate.CodeSupersessionOrphan) {
		t.Errorf("VS031 must not fire when RELATED-TO resolves; got %v", got)
	}
}

func TestSupersession_MissingRelatedTo_VS030(t *testing.T) {
	c := supersessionEntry(t, "j-1", "", "completed")
	got := validate.ValidateComponent(c)
	d, ok := findCode(got, validate.CodeSupersessionMissingProps)
	if !ok {
		t.Fatalf("expected VS030 (missing RELATED-TO); got %v", got)
	}
	if d.Severity != validate.SeverityError {
		t.Errorf("VS030 must be Error; got %v", d.Severity)
	}
	if !strings.HasSuffix(d.Path, ".RELATED-TO") {
		t.Errorf("VS030 RELATED-TO path should end .RELATED-TO; got %q", d.Path)
	}
	if !strings.Contains(d.Message, "RELATED-TO") {
		t.Errorf("VS030 message should name RELATED-TO; got %q", d.Message)
	}
}

func TestSupersession_MissingEffectiveStatus_VS030(t *testing.T) {
	c := supersessionEntry(t, "j-1", "todo-1", "")
	got := validate.ValidateComponent(c)
	d, ok := findCode(got, validate.CodeSupersessionMissingProps)
	if !ok {
		t.Fatalf("expected VS030 (missing X-VSTAR-EFFECTIVE-STATUS); got %v", got)
	}
	if d.Severity != validate.SeverityError {
		t.Errorf("VS030 must be Error; got %v", d.Severity)
	}
	if !strings.HasSuffix(d.Path, "."+supersession.PropEffectiveStatus) {
		t.Errorf("VS030 path should end .%s; got %q", supersession.PropEffectiveStatus, d.Path)
	}
	if !strings.Contains(d.Message, supersession.PropEffectiveStatus) {
		t.Errorf("VS030 message should name %s; got %q", supersession.PropEffectiveStatus, d.Message)
	}
}

func TestSupersession_MissingBoth_TwoVS030s(t *testing.T) {
	c := supersessionEntry(t, "j-1", "", "")
	got := validate.ValidateComponent(c)
	count := 0
	for _, d := range got {
		if d.Code == validate.CodeSupersessionMissingProps {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected exactly 2 VS030 diagnostics (RELATED-TO + EFFECTIVE-STATUS); got %d in %v", count, got)
	}
}

func TestSupersession_Orphan_VS031(t *testing.T) {
	cal := vstar.Calendar{
		ProdID: "-//test//EN",
		Components: []vstar.Component{
			supersessionEntry(t, "j-1", "todo-missing", "completed"),
		},
	}
	got := validate.Validate(cal)
	d, ok := findCode(got, validate.CodeSupersessionOrphan)
	if !ok {
		t.Fatalf("expected VS031 (orphan supersession); got %v", got)
	}
	if d.Severity != validate.SeverityError {
		t.Errorf("VS031 must be Error; got %v", d.Severity)
	}
	if !strings.HasSuffix(d.Path, ".RELATED-TO") {
		t.Errorf("VS031 path should end .RELATED-TO; got %q", d.Path)
	}
	if !strings.Contains(d.Message, "todo-missing") {
		t.Errorf("VS031 message should name the missing UID; got %q", d.Message)
	}
}

func TestSupersession_ValidateComponent_SkipsVS031(t *testing.T) {
	// VS031 needs cross-component context; ValidateComponent has
	// none and so MUST NOT emit it. VS030 still fires when props
	// are missing.
	c := supersessionEntry(t, "j-1", "todo-missing", "completed")
	got := validate.ValidateComponent(c)
	if hasCode(got, validate.CodeSupersessionOrphan) {
		t.Errorf("ValidateComponent must not emit VS031 (no ledger); got %v", got)
	}
}

func TestSupersession_NonJournal_NoCheck(t *testing.T) {
	// CATEGORIES on a non-VJOURNAL is not a supersession entry; the
	// check must not fire on any component type other than VJOURNAL.
	c := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "todo-1"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "DUE", Value: "20260601T000000Z"},
			{Name: "CATEGORIES", Value: supersession.CategoryStatusSupersession},
		},
	}
	c = hashed(c)
	got := validate.ValidateComponent(c)
	if hasCode(got, validate.CodeSupersessionMissingProps) {
		t.Errorf("VS030 must only fire on VJOURNAL; got %v", got)
	}
}

func TestSupersession_VJOURNAL_WithoutCategory_NoCheck(t *testing.T) {
	// A VJOURNAL without CATEGORIES=status-supersession is just a
	// regular journal entry; the check must not fire.
	c := vstar.Component{
		Type: vstar.CompJournal,
		Props: []vstar.Property{
			{Name: "UID", Value: "j-1"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
		},
	}
	c = hashed(c)
	got := validate.ValidateComponent(c)
	if hasCode(got, validate.CodeSupersessionMissingProps) {
		t.Errorf("VS030 must not fire on a non-supersession VJOURNAL; got %v", got)
	}
}

func TestSupersession_CategoryCaseInsensitive(t *testing.T) {
	// CATEGORIES tokens are case-insensitive per RFC 5545. The
	// supersession check should recognize "STATUS-SUPERSESSION"
	// just as well as "status-supersession".
	c := vstar.Component{
		Type: vstar.CompJournal,
		Props: []vstar.Property{
			{Name: "UID", Value: "j-1"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "CATEGORIES", Value: "STATUS-SUPERSESSION"},
		},
	}
	c = hashed(c)
	got := validate.ValidateComponent(c)
	if !hasCode(got, validate.CodeSupersessionMissingProps) {
		t.Errorf("VS030 must fire on uppercase CATEGORIES too; got %v", got)
	}
}

func TestSupersession_MultipleCategoryTokens(t *testing.T) {
	// CATEGORIES is a comma-separated list; supersession membership
	// must be detected when the magic token is one of several.
	c := vstar.Component{
		Type: vstar.CompJournal,
		Props: []vstar.Property{
			{Name: "UID", Value: "j-1"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "CATEGORIES", Value: "audit," + supersession.CategoryStatusSupersession + ",critical"},
		},
	}
	c = hashed(c)
	got := validate.ValidateComponent(c)
	if !hasCode(got, validate.CodeSupersessionMissingProps) {
		t.Errorf("VS030 must fire when supersession is one token of many; got %v", got)
	}
}
