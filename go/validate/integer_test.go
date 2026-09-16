// SPDX-License-Identifier: MIT

package validate_test

import (
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/validate"
)

// integerProps are the three properties VS055 bounds.
var integerProps = []string{"PRIORITY", "PERCENT-COMPLETE", "SEQUENCE"}

// TestIntegerDomains_boundaries pins each property's domain edge:
// the last value inside is clean, the first value outside is VS055.
func TestIntegerDomains_boundaries(t *testing.T) {
	cases := []struct {
		prop  string
		value string
		clean bool
	}{
		{"PRIORITY", "0", true},
		{"PRIORITY", "9", true},
		{"PRIORITY", "10", false},
		{"PRIORITY", "-1", false},
		{"PERCENT-COMPLETE", "0", true},
		{"PERCENT-COMPLETE", "100", true},
		{"PERCENT-COMPLETE", "101", false},
		{"PERCENT-COMPLETE", "-1", false},
		{"SEQUENCE", "0", true},
		{"SEQUENCE", "-1", false},
		// The rule is textual: no machine integer decides
		// well-formedness, so 2^64 is a well-formed SEQUENCE.
		{"SEQUENCE", "18446744073709551616", true},
	}
	for _, tc := range cases {
		t.Run(tc.prop+"/"+tc.value, func(t *testing.T) {
			c := withValue(vstar.CompTodo, tc.prop, tc.value)
			got := hasCode(validate.ValidateComponent(c), validate.CodeIntegerOutOfDomain)
			if tc.clean && got {
				t.Errorf("%s:%s must be clean", tc.prop, tc.value)
			}
			if !tc.clean && !got {
				t.Errorf("%s:%s must yield VS055", tc.prop, tc.value)
			}
		})
	}
}

// TestIntegerDomains_nonCanonicalIsVS055 — a sign, a leading zero or
// whitespace is a VS055 even when the numeric value would be in
// range (spec/05 §8: canonical decimal).
func TestIntegerDomains_nonCanonicalIsVS055(t *testing.T) {
	for _, prop := range integerProps {
		for _, v := range []string{"+3", "07", " 3", "3 "} {
			t.Run(prop+"/"+strings.ReplaceAll(v, " ", "_"), func(t *testing.T) {
				c := withValue(vstar.CompTodo, prop, v)
				if !hasCode(validate.ValidateComponent(c), validate.CodeIntegerOutOfDomain) {
					t.Errorf("%s:%q must yield VS055", prop, v)
				}
			})
		}
	}
}

// TestIntegerDomains_pathAndSeverity — the path ends in the property
// name, like VS044 and VS052, and the severity is Error.
func TestIntegerDomains_pathAndSeverity(t *testing.T) {
	for _, prop := range integerProps {
		c := withValue(vstar.CompTodo, prop, "+3")
		d, ok := findCode(validate.ValidateComponent(c), validate.CodeIntegerOutOfDomain)
		if !ok {
			t.Fatalf("%s:+3 must yield VS055", prop)
		}
		if d.Severity != validate.SeverityError {
			t.Errorf("VS055 severity = %v, want Error", d.Severity)
		}
		if want := "VTODO[uid=value-1]." + prop; d.Path != want {
			t.Errorf("VS055 path = %q, want %q", d.Path, want)
		}
	}
}

// TestIntegerDomains_oneDiagnosticPerProperty — a component carrying
// two bad values yields two rows, one per offending property.
func TestIntegerDomains_oneDiagnosticPerProperty(t *testing.T) {
	c := withValue(vstar.CompTodo, "PRIORITY", "07")
	c.Set(vstar.Property{Name: "SEQUENCE", Value: "+3"})
	c = hashed(c)
	var paths []string
	for _, d := range validate.ValidateComponent(c) {
		if d.Code == validate.CodeIntegerOutOfDomain {
			paths = append(paths, d.Path)
		}
	}
	if len(paths) != 2 {
		t.Fatalf("want 2 VS055 rows, got %d: %v", len(paths), paths)
	}
	want := map[string]bool{
		"VTODO[uid=value-1].PRIORITY": true,
		"VTODO[uid=value-1].SEQUENCE": true,
	}
	for _, p := range paths {
		if !want[p] {
			t.Errorf("unexpected VS055 path %q", p)
		}
	}
}

// TestIntegerDomains_notTypeGated — the value is checked wherever the
// property appears; PERCENT-COMPLETE on a VEVENT is bounded, not
// flagged for scope.
func TestIntegerDomains_notTypeGated(t *testing.T) {
	c := withValue(vstar.CompEvent, "PERCENT-COMPLETE", "101")
	if !hasCode(validate.ValidateComponent(c), validate.CodeIntegerOutOfDomain) {
		t.Errorf("PERCENT-COMPLETE:101 on VEVENT must yield VS055")
	}
	c = withValue(vstar.CompEvent, "PERCENT-COMPLETE", "50")
	if ds := validate.ValidateComponent(c); len(ds) != 0 {
		t.Errorf("PERCENT-COMPLETE:50 on VEVENT must be clean (scope is not diagnosed), got %+v", ds)
	}
}

func TestIntegerDomains_absentIsClean(t *testing.T) {
	c := withValue(vstar.CompTodo, "SUMMARY", "no integers here")
	if hasCode(validate.ValidateComponent(c), validate.CodeIntegerOutOfDomain) {
		t.Errorf("absent integer properties must be clean")
	}
}
