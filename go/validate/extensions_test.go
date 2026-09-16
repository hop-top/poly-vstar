// SPDX-License-Identifier: MIT

package validate_test

import (
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/validate"
)

func TestExtensions_StandardProperty_NoWarning(t *testing.T) {
	c := minimalVTODO(t)
	got := validate.ValidateComponent(c)
	if hasCode(got, validate.CodeUnknownProperty) {
		t.Errorf("VS020 must not fire on standard properties; got %v", got)
	}
}

func TestExtensions_XPrefixProperty_NoWarning(t *testing.T) {
	c := minimalVTODO(t)
	c.Add(vstar.Property{Name: "X-AGR-PROVENANCE", Value: "agr-compiler"})
	c.Add(vstar.Property{Name: "X-VSTAR-EFFECTIVE-STATUS", Value: "active"})
	c.Add(vstar.Property{Name: "X-EXP-FOO", Value: "bar"})
	// rehash since props changed; otherwise VS010 also fires.
	c = hashed(c)

	got := validate.ValidateComponent(c)
	if hasCode(got, validate.CodeUnknownProperty) {
		t.Errorf("VS020 must not fire on X-* properties; got %v", got)
	}
}

func TestExtensions_UnknownNonXProperty_VS020(t *testing.T) {
	c := minimalVTODO(t)
	c.Add(vstar.Property{Name: "WIDGET", Value: "frob"})
	c = hashed(c)

	got := validate.ValidateComponent(c)
	d, ok := findCode(got, validate.CodeUnknownProperty)
	if !ok {
		t.Fatalf("expected VS020 on WIDGET; got %v", got)
	}
	if d.Severity != validate.SeverityWarning {
		t.Errorf("VS020 must be Warning; got %v", d.Severity)
	}
	if d.Path != "VTODO[uid=todo-1].WIDGET" {
		t.Errorf("expected path VTODO[uid=todo-1].WIDGET; got %q", d.Path)
	}
}

func TestExtensions_LowercaseXPrefix_NoWarning(t *testing.T) {
	// RFC names are case-insensitive; "x-agr-foo" should be
	// treated identically to "X-AGR-FOO".
	c := minimalVTODO(t)
	c.Add(vstar.Property{Name: "x-agr-foo", Value: "v"})
	c = hashed(c)

	got := validate.ValidateComponent(c)
	if hasCode(got, validate.CodeUnknownProperty) {
		t.Errorf("VS020 must not fire on lowercase x- prefix; got %v", got)
	}
}

func TestExtensions_StandardPropertyCount_Reasonable(t *testing.T) {
	// Sanity bound to catch accidental wholesale deletion of the
	// allow-list. RFC 5545 contributes ~50 properties and RFC
	// 6350 ~30; the combined list should comfortably exceed 70.
	got := validate.StandardPropertyCount()
	if got < 70 {
		t.Errorf("standard property allow-list too small: got %d, want >= 70", got)
	}
}
