// SPDX-License-Identifier: MIT

package validate_test

import (
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
	"hop.top/vstar/validate"
)

// hashed returns c with X-VSTAR-HASH set to the correct value so
// tests for OTHER rules don't trigger VS003 / VS010 by accident.
func hashed(c vstar.Component) vstar.Component {
	cp := c
	hashing.SetXVSTAR(&cp)
	return cp
}

// minimalVTODO is the smallest VTODO that passes spec/05 §1 and §5
// for VTODO. Tests build on it by removing or perturbing one
// property at a time.
func minimalVTODO(t *testing.T) vstar.Component {
	t.Helper()
	c := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "todo-1"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "DUE", Value: "20260601T000000Z"},
		},
	}
	return hashed(c)
}

func TestSeverity_String(t *testing.T) {
	cases := []struct {
		s    validate.Severity
		want string
	}{
		{validate.SeverityError, "error"},
		{validate.SeverityWarning, "warning"},
		{validate.Severity(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tc.s, got, tc.want)
		}
	}
}

func TestValidate_CleanCalendar_NoDiagnostics(t *testing.T) {
	cal := vstar.Calendar{
		ProdID:     "-//test//EN",
		Components: []vstar.Component{minimalVTODO(t)},
	}
	if d := validate.Validate(cal); len(d) != 0 {
		t.Fatalf("expected clean Calendar to yield 0 diagnostics, got %d: %+v", len(d), d)
	}
}

func TestValidateComponent_Clean_NoDiagnostics(t *testing.T) {
	if d := validate.ValidateComponent(minimalVTODO(t)); len(d) != 0 {
		t.Fatalf("expected clean Component to yield 0 diagnostics, got %d: %+v", len(d), d)
	}
}

func TestValidateComponent_PathOmitsCalendarPrefix(t *testing.T) {
	c := vstar.Component{
		Type:  vstar.CompTodo,
		Props: []vstar.Property{{Name: "DTSTAMP", Value: "20260504T120000Z"}},
	}
	got := validate.ValidateComponent(c)
	if len(got) == 0 {
		t.Fatal("expected diagnostics on a UID-less component")
	}
	for _, d := range got {
		if strings.HasPrefix(d.Path, "VCALENDAR.") {
			t.Errorf("ValidateComponent paths must not be prefixed with VCALENDAR.; got %q", d.Path)
		}
	}
}

func TestValidate_PathStartsWithCalendar(t *testing.T) {
	c := vstar.Component{
		Type:  vstar.CompTodo,
		Props: []vstar.Property{{Name: "DTSTAMP", Value: "20260504T120000Z"}},
	}
	cal := vstar.Calendar{Components: []vstar.Component{c}}
	got := validate.Validate(cal)
	if len(got) == 0 {
		t.Fatal("expected diagnostics on a UID-less component inside a Calendar")
	}
	for _, d := range got {
		if !strings.HasPrefix(d.Path, "VCALENDAR.") {
			t.Errorf("Validate paths must be prefixed with VCALENDAR.; got %q", d.Path)
		}
	}
}

func TestValidate_PositionalIndexForUIDLessComponents(t *testing.T) {
	c1 := vstar.Component{Type: vstar.CompTodo}
	c2 := vstar.Component{Type: vstar.CompTodo}
	cal := vstar.Calendar{Components: []vstar.Component{c1, c2}}
	got := validate.Validate(cal)

	var saw0, saw1 bool
	for _, d := range got {
		if strings.HasPrefix(d.Path, "VCALENDAR.VTODO[#0]") {
			saw0 = true
		}
		if strings.HasPrefix(d.Path, "VCALENDAR.VTODO[#1]") {
			saw1 = true
		}
	}
	if !saw0 || !saw1 {
		t.Errorf("expected positional indices #0 and #1 for UID-less VTODOs; saw0=%v saw1=%v paths=%v", saw0, saw1, paths(got))
	}
}

func TestValidate_DiagnosticDoesNotMutateInput(t *testing.T) {
	c := minimalVTODO(t)
	before := len(c.Props)
	_ = validate.ValidateComponent(c)
	if got := len(c.Props); got != before {
		t.Errorf("Validate must not mutate Component; props len before=%d after=%d", before, got)
	}
}

// paths is a small helper to render diagnostic paths in failure
// messages.
func paths(ds []validate.Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Path
	}
	return out
}
