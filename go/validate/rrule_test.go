// SPDX-License-Identifier: MIT

package validate

import (
	"testing"

	vstar "hop.top/vstar"
)

// TestCheckRRule covers the three RRULE classes the VS050/VS051
// rule cares about: supported (no diagnostic), out-of-scope
// (VS050 warning), syntactically malformed (VS051 error).
func TestCheckRRule(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		wantCode string // "" = no diagnostic
		wantSev  Severity
	}{
		{
			name:  "supported_no_diagnostic",
			value: "FREQ=DAILY;INTERVAL=2",
		},
		{
			name:     "unsupported_freq_secondly",
			value:    "FREQ=SECONDLY",
			wantCode: CodeRRuleUnsupported,
			wantSev:  SeverityWarning,
		},
		{
			// FREQ=MINUTELY is in the RRULE parsing scope; SECONDLY and
			// RSCALE remain the VS050 cases.
			name:  "supported_freq_minutely",
			value: "FREQ=MINUTELY",
		},
		{
			name:     "unsupported_rscale",
			value:    "FREQ=YEARLY;RSCALE=HEBREW",
			wantCode: CodeRRuleUnsupported,
			wantSev:  SeverityWarning,
		},
		{
			// BYSETPOS is in the RRULE parsing scope (no
			// VS050). With another BY-* clause it's accepted clean.
			name:  "supported_bysetpos_with_byday",
			value: "FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1",
		},
		{
			// BYWEEKNO is in the RRULE parsing scope.
			name:  "supported_byweekno",
			value: "FREQ=YEARLY;BYWEEKNO=20",
		},
		{
			// BYYEARDAY is in the RRULE parsing scope.
			name:  "supported_byyearday",
			value: "FREQ=YEARLY;BYYEARDAY=100",
		},
		{
			name:     "malformed_missing_freq",
			value:    "INTERVAL=2",
			wantCode: CodeRRuleMalformed,
			wantSev:  SeverityError,
		},
		{
			name:     "malformed_until_form1",
			value:    "FREQ=DAILY;UNTIL=20261231T235959",
			wantCode: CodeRRuleMalformed,
			wantSev:  SeverityError,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			comp := vstar.Component{
				Type: "VTODO",
				Props: []vstar.Property{
					{Name: "RRULE", Value: c.value},
				},
			}
			diags := checkRRule(comp, "VTODO[uid=test]")
			if c.wantCode == "" {
				if len(diags) != 0 {
					t.Fatalf("want no diagnostic, got %+v", diags)
				}
				return
			}
			if len(diags) != 1 {
				t.Fatalf("want 1 diagnostic, got %d: %+v", len(diags), diags)
			}
			if diags[0].Code != c.wantCode {
				t.Fatalf("Code: want %q, got %q", c.wantCode, diags[0].Code)
			}
			if diags[0].Severity != c.wantSev {
				t.Fatalf("Severity: want %v, got %v", c.wantSev, diags[0].Severity)
			}
			if diags[0].Path != "VTODO[uid=test].RRULE" {
				t.Fatalf("Path: want VTODO[uid=test].RRULE, got %q", diags[0].Path)
			}
		})
	}
}

// TestCheckRRule_NoRRULEProperty asserts the check is a no-op for
// components without an RRULE property.
func TestCheckRRule_NoRRULEProperty(t *testing.T) {
	comp := vstar.Component{
		Type: "VEVENT",
		Props: []vstar.Property{
			{Name: "UID", Value: "x"},
		},
	}
	if diags := checkRRule(comp, "VEVENT[uid=x]"); len(diags) != 0 {
		t.Fatalf("want no diagnostic, got %+v", diags)
	}
}

// TestCheckRRule_MultipleProperties: a component with multiple
// RRULE properties (rare but legal) yields one diagnostic per
// problematic value.
func TestCheckRRule_MultipleProperties(t *testing.T) {
	comp := vstar.Component{
		Type: "VTODO",
		Props: []vstar.Property{
			{Name: "RRULE", Value: "FREQ=DAILY"},    // ok
			{Name: "RRULE", Value: "FREQ=SECONDLY"}, // VS050
			{Name: "RRULE", Value: "INTERVAL=1"},    // VS051
		},
	}
	diags := checkRRule(comp, "VTODO[uid=x]")
	if len(diags) != 2 {
		t.Fatalf("want 2 diagnostics, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != CodeRRuleUnsupported || diags[1].Code != CodeRRuleMalformed {
		t.Fatalf("codes: got %q, %q", diags[0].Code, diags[1].Code)
	}
}

// TestValidate_VS050_Integration runs the full Validate pipeline
// on a calendar carrying an unsupported RRULE and asserts the
// VS050 diagnostic is surfaced (covers the wiring into the
// validateComponentAtWithLedger pipeline).
func TestValidate_VS050_Integration(t *testing.T) {
	cal := vstar.Calendar{
		Components: []vstar.Component{
			{
				Type: "VTODO",
				Props: []vstar.Property{
					{Name: "UID", Value: "abc"},
					{Name: "DTSTAMP", Value: "20260504T120000Z"},
					{Name: "DUE", Value: "20260601T000000Z"},
					{Name: "X-VSTAR-HASH", Value: "ignored"}, // hash mismatch ok
					{Name: "RRULE", Value: "RSCALE=HEBREW;FREQ=YEARLY"},
				},
			},
		},
	}
	diags := Validate(cal)
	foundVS050 := false
	for _, d := range diags {
		if d.Code == CodeRRuleUnsupported {
			foundVS050 = true
			if d.Severity != SeverityWarning {
				t.Errorf("VS050 should be Warning, got %v", d.Severity)
			}
		}
	}
	if !foundVS050 {
		t.Fatalf("VS050 not surfaced; diags: %+v", diags)
	}
}
