// SPDX-License-Identifier: MIT

package validate_test

import (
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/validate"
)

func TestIntegrity_GoodHash_NoDiagnostic(t *testing.T) {
	c := minimalVTODO(t) // SetXVSTAR-stamped already.
	got := validate.ValidateComponent(c)
	if hasCode(got, validate.CodeBadXVSTARHash) {
		t.Errorf("VS010 must not fire on a freshly hashed component; got %v", got)
	}
}

func TestIntegrity_TamperedHash_VS010(t *testing.T) {
	c := minimalVTODO(t)
	// Overwrite the hash with a syntactically-valid but wrong
	// value. VS010 fires only because the hash is *present and
	// wrong*.
	c.Set(vstar.Property{Name: "X-VSTAR-HASH", Value: "sha256:0000000000000000000000000000000000000000000000000000000000000000"})
	got := validate.ValidateComponent(c)
	d, ok := findCode(got, validate.CodeBadXVSTARHash)
	if !ok {
		t.Fatalf("expected VS010 (tampered hash); got %v", got)
	}
	if d.Severity != validate.SeverityError {
		t.Errorf("VS010 must be Error; got %v", d.Severity)
	}
	if !strings.Contains(d.Message, "want=") || !strings.Contains(d.Message, "got=") {
		t.Errorf("VS010 message should report both want= and got= values; got %q", d.Message)
	}
}

func TestIntegrity_TamperedAfterHash_VS010(t *testing.T) {
	// Component starts hashed-correct, but a property is mutated
	// AFTER the hash was stamped → recompute disagrees.
	c := minimalVTODO(t)
	c.Set(vstar.Property{Name: "DUE", Value: "20990101T000000Z"})
	got := validate.ValidateComponent(c)
	if !hasCode(got, validate.CodeBadXVSTARHash) {
		t.Errorf("expected VS010 when DUE is mutated after stamping; got %v", got)
	}
}

func TestIntegrity_AbsentHash_VS003OnlyNotVS010(t *testing.T) {
	// Per the package contract: when X-VSTAR-HASH is absent,
	// VS003 fires (T2) and VS010 stays silent. This split keeps
	// the diagnostic surface unambiguous.
	c := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "todo-1"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "DUE", Value: "20260601T000000Z"},
		},
	}
	got := validate.ValidateComponent(c)
	if !hasCode(got, validate.CodeMissingXVSTARHash) {
		t.Errorf("expected VS003 (missing hash); got %v", got)
	}
	if hasCode(got, validate.CodeBadXVSTARHash) {
		t.Errorf("VS010 must not fire when hash is absent; got %v", got)
	}
}
