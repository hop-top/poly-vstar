// SPDX-License-Identifier: MIT

package validate_test

import (
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/validate"
)

// hasCode reports whether ds contains a Diagnostic whose Code
// matches code. Used by required/integrity/extension/type tests so
// failure assertions read in plain English.
func hasCode(ds []validate.Diagnostic, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}

// findCode returns the first Diagnostic in ds whose Code matches.
// Returns the zero Diagnostic and ok=false when absent.
func findCode(ds []validate.Diagnostic, code string) (validate.Diagnostic, bool) {
	for _, d := range ds {
		if d.Code == code {
			return d, true
		}
	}
	return validate.Diagnostic{}, false
}

func TestRequired_MissingUID_VS001(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "DUE", Value: "20260601T000000Z"},
			// X-VSTAR-HASH stub so VS003 doesn't trigger.
			{Name: "X-VSTAR-HASH", Value: "sha256:abc"},
		},
	}
	got := validate.ValidateComponent(c)
	d, ok := findCode(got, validate.CodeMissingUID)
	if !ok {
		t.Fatalf("expected VS001 (missing UID) diagnostic; got %v", got)
	}
	if d.Severity != validate.SeverityError {
		t.Errorf("VS001 must be Error; got %v", d.Severity)
	}
	if d.Path != "VTODO[#0].UID" {
		t.Errorf("expected path VTODO[#0].UID; got %q", d.Path)
	}
}

func TestRequired_MissingDTSTAMP_VS002(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "todo-1"},
			{Name: "DUE", Value: "20260601T000000Z"},
			{Name: "X-VSTAR-HASH", Value: "sha256:abc"},
		},
	}
	got := validate.ValidateComponent(c)
	d, ok := findCode(got, validate.CodeMissingDTSTAMP)
	if !ok {
		t.Fatalf("expected VS002 (missing DTSTAMP); got %v", got)
	}
	if d.Severity != validate.SeverityError {
		t.Errorf("VS002 must be Error; got %v", d.Severity)
	}
	if d.Path != "VTODO[uid=todo-1].DTSTAMP" {
		t.Errorf("expected path VTODO[uid=todo-1].DTSTAMP; got %q", d.Path)
	}
}

func TestRequired_MissingXVSTARHash_VS003(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "todo-1"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "DUE", Value: "20260601T000000Z"},
		},
	}
	got := validate.ValidateComponent(c)
	d, ok := findCode(got, validate.CodeMissingXVSTARHash)
	if !ok {
		t.Fatalf("expected VS003 (missing X-VSTAR-HASH); got %v", got)
	}
	if d.Severity != validate.SeverityError {
		t.Errorf("VS003 must be Error; got %v", d.Severity)
	}
	if d.Path != "VTODO[uid=todo-1].X-VSTAR-HASH" {
		t.Errorf("expected path VTODO[uid=todo-1].X-VSTAR-HASH; got %q", d.Path)
	}
	// VS010 must NOT fire when hash is absent (T3 contract).
	if hasCode(got, validate.CodeBadXVSTARHash) {
		t.Errorf("VS010 (bad hash) must not fire when hash is absent; got %v", got)
	}
}

func TestRequired_AllThreeMissing_AllThreeDiagnostics(t *testing.T) {
	c := vstar.Component{
		Type:  vstar.CompTodo,
		Props: []vstar.Property{{Name: "DUE", Value: "20260601T000000Z"}},
	}
	got := validate.ValidateComponent(c)
	for _, code := range []string{validate.CodeMissingUID, validate.CodeMissingDTSTAMP, validate.CodeMissingXVSTARHash} {
		if !hasCode(got, code) {
			t.Errorf("expected diagnostic %s; got %v", code, got)
		}
	}
}

func TestRequired_CaseInsensitive(t *testing.T) {
	// RFC 5545 §3.1 — names are case-insensitive. A lowercase
	// "uid" should still satisfy VS001.
	c := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "uid", Value: "todo-1"},
			{Name: "dtstamp", Value: "20260504T120000Z"},
			{Name: "DUE", Value: "20260601T000000Z"},
			{Name: "x-vstar-hash", Value: "sha256:abc"},
		},
	}
	got := validate.ValidateComponent(c)
	for _, code := range []string{validate.CodeMissingUID, validate.CodeMissingDTSTAMP, validate.CodeMissingXVSTARHash} {
		if hasCode(got, code) {
			t.Errorf("case-insensitive lookup failed; %s emitted on lowercase props: %v", code, got)
		}
	}
}
