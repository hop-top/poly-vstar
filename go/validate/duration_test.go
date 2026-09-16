// SPDX-License-Identifier: MIT

package validate_test

import (
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/validate"
)

// TestCheckDuration_MalformedDuration asserts VS052 fires for a
// DURATION property whose value violates RFC 5545 §3.3.6.
func TestCheckDuration_MalformedDuration(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "DURATION", Value: "P1Y"},
		},
	}
	diags := validate.ValidateComponent(c)
	if !hasCode(diags, validate.CodeMalformedDuration) {
		t.Fatalf("expected %s for DURATION:P1Y, got %+v", validate.CodeMalformedDuration, diags)
	}
	for _, d := range diags {
		if d.Code == validate.CodeMalformedDuration {
			if d.Severity != validate.SeverityError {
				t.Fatalf("severity = %v, want SeverityError", d.Severity)
			}
			if want := "VEVENT[#0].DURATION"; d.Path != want {
				t.Fatalf("path = %q, want %q", d.Path, want)
			}
		}
	}
}

// TestCheckDuration_ValidDurationClean asserts a well-formed
// DURATION produces no VS052.
func TestCheckDuration_ValidDurationClean(t *testing.T) {
	for _, v := range []string{"P15D", "PT1H", "-PT15M", "P1DT2H30M", "P2W", "PT0S"} {
		c := vstar.Component{
			Type:  vstar.CompEvent,
			Props: []vstar.Property{{Name: "DURATION", Value: v}},
		}
		if diags := validate.ValidateComponent(c); hasCode(diags, validate.CodeMalformedDuration) {
			t.Fatalf("DURATION:%s wrongly flagged: %+v", v, diags)
		}
	}
}

// TestCheckDuration_MalformedTrigger asserts VS052 also covers a
// relative TRIGGER whose DURATION value is malformed — the property
// where a bad duration silently drops an alarm.
func TestCheckDuration_MalformedTrigger(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompAlarm,
		Props: []vstar.Property{
			{Name: "ACTION", Value: "DISPLAY"},
			{Name: "TRIGGER", Value: "15 minutes before"},
		},
	}
	diags := validate.ValidateComponent(c)
	if !hasCode(diags, validate.CodeMalformedDuration) {
		t.Fatalf("expected %s for malformed TRIGGER, got %+v", validate.CodeMalformedDuration, diags)
	}
}

// TestCheckDuration_ValidTriggersClean asserts both trigger forms —
// relative DURATION and absolute DATE-TIME — pass clean.
func TestCheckDuration_ValidTriggersClean(t *testing.T) {
	props := []vstar.Property{
		{Name: "TRIGGER", Value: "-PT15M"},
		{Name: "TRIGGER", Params: []vstar.Param{{Name: "RELATED", Value: "END"}}, Value: "PT10M"},
		{Name: "TRIGGER", Params: []vstar.Param{{Name: "VALUE", Value: "DATE-TIME"}}, Value: "20260601T083000Z"},
		{Name: "TRIGGER", Value: "20260601T083000Z"},
	}
	for _, p := range props {
		c := vstar.Component{
			Type:  vstar.CompAlarm,
			Props: []vstar.Property{{Name: "ACTION", Value: "DISPLAY"}, p},
		}
		if diags := validate.ValidateComponent(c); hasCode(diags, validate.CodeMalformedDuration) {
			t.Fatalf("TRIGGER %+v wrongly flagged: %+v", p, diags)
		}
	}
}

// TestCheckDuration_MalformedRepeat asserts a REPEAT that is not a
// non-negative integer is flagged.
func TestCheckDuration_MalformedRepeat(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompAlarm,
		Props: []vstar.Property{
			{Name: "ACTION", Value: "DISPLAY"},
			{Name: "TRIGGER", Value: "-PT15M"},
			{Name: "DURATION", Value: "PT5M"},
			{Name: "REPEAT", Value: "many"},
		},
	}
	if diags := validate.ValidateComponent(c); !hasCode(diags, validate.CodeMalformedDuration) {
		t.Fatalf("expected %s for REPEAT:many, got %+v", validate.CodeMalformedDuration, diags)
	}
}

// TestCheckDuration_RepeatCanonicalDecimal — the REPEAT arm requires
// the canonical non-negative decimal spec/05 §8 prescribes: a sign or
// a leading zero is VS052 even though the number is in range.
func TestCheckDuration_RepeatCanonicalDecimal(t *testing.T) {
	cases := []struct {
		value string
		clean bool
	}{
		{"0", true},
		{"3", true},
		{"01", false},
		{"+1", false},
		{"-1", false},
		{" 1", false},
		{"many", false},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			c := vstar.Component{
				Type: vstar.CompAlarm,
				Props: []vstar.Property{
					{Name: "ACTION", Value: "DISPLAY"},
					{Name: "TRIGGER", Value: "-PT15M"},
					{Name: "DURATION", Value: "PT5M"},
					{Name: "REPEAT", Value: tc.value},
				},
			}
			d, got := findCode(validate.ValidateComponent(c), validate.CodeMalformedDuration)
			if tc.clean && got {
				t.Errorf("REPEAT:%q must be clean, got %+v", tc.value, d)
			}
			if !tc.clean && !got {
				t.Errorf("REPEAT:%q must yield VS052", tc.value)
			}
			if got {
				if want := "VALARM[#0].REPEAT"; d.Path != want {
					t.Errorf("path = %q, want %q", d.Path, want)
				}
			}
		})
	}
}

// TestCheckDuration_NoDurationClean asserts components without any
// duration-bearing property are untouched by the new rule.
func TestCheckDuration_NoDurationClean(t *testing.T) {
	c := vstar.Component{
		Type:  vstar.CompEvent,
		Props: []vstar.Property{{Name: "DTSTART", Value: "20260601T090000Z"}},
	}
	if diags := validate.ValidateComponent(c); hasCode(diags, validate.CodeMalformedDuration) {
		t.Fatalf("component without DURATION wrongly flagged: %+v", diags)
	}
}
