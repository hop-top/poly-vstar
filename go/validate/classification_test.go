// SPDX-License-Identifier: MIT

package validate_test

import (
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/validate"
)

// withValue builds a minimally-valid component of type ct carrying
// one extra property, so the only interesting diagnostic is the one
// the property provokes. The UID is fixed so paths are predictable.
func withValue(ct vstar.CompType, name, value string) vstar.Component {
	props := append(commonProps("value-1"), vstar.Property{Name: name, Value: value})
	switch ct {
	case vstar.CompTodo:
		props = append(props, vstar.Property{Name: "DUE", Value: "20260601T000000Z"})
	case vstar.CompEvent:
		props = append(props, vstar.Property{Name: "DTSTART", Value: "20260601T000000Z"})
	}
	return hashed(vstar.Component{Type: ct, Props: props})
}

// TestClassification_classOutsideVocabularyIsVS053 — an X- token is
// legal RFC 5545 ABNF for CLASS, and still an error here: spec/05 §8
// binds the value to the three registered names.
func TestClassification_classOutsideVocabularyIsVS053(t *testing.T) {
	c := withValue(vstar.CompEvent, "CLASS", "X-SECRET")
	d, ok := findCode(validate.ValidateComponent(c), validate.CodeClassNotInVocabulary)
	if !ok {
		t.Fatalf("CLASS:X-SECRET on VEVENT must yield VS053")
	}
	if d.Severity != validate.SeverityError {
		t.Errorf("VS053 severity = %v, want Error", d.Severity)
	}
	if want := "VEVENT[uid=value-1].CLASS"; d.Path != want {
		t.Errorf("VS053 path = %q, want %q", d.Path, want)
	}
}

func TestClassification_classRegistryValuesAreClean(t *testing.T) {
	for _, v := range []vstar.Class{vstar.ClassPublic, vstar.ClassPrivate, vstar.ClassConfidential} {
		c := withValue(vstar.CompEvent, "CLASS", string(v))
		if hasCode(validate.ValidateComponent(c), validate.CodeClassNotInVocabulary) {
			t.Errorf("CLASS:%s must be clean", v)
		}
	}
}

// TestClassification_classCaseInsensitive — spec/05 §8 compares
// vocabulary values case-insensitively (RFC 5545 §3.1).
func TestClassification_classCaseInsensitive(t *testing.T) {
	c := withValue(vstar.CompEvent, "CLASS", "private")
	if hasCode(validate.ValidateComponent(c), validate.CodeClassNotInVocabulary) {
		t.Errorf("lowercase CLASS:private must be accepted")
	}
}

// TestClassification_classNotTypeGated — the value is checked on
// any component carrying CLASS; component scope is not diagnosed.
func TestClassification_classNotTypeGated(t *testing.T) {
	for _, ct := range []vstar.CompType{vstar.CompTodo, vstar.CompJournal} {
		c := withValue(ct, "CLASS", "X-SECRET")
		if !hasCode(validate.ValidateComponent(c), validate.CodeClassNotInVocabulary) {
			t.Errorf("CLASS:X-SECRET on %s must yield VS053", ct)
		}
	}
}

func TestClassification_absentClassIsClean(t *testing.T) {
	c := withValue(vstar.CompEvent, "SUMMARY", "no CLASS here")
	if hasCode(validate.ValidateComponent(c), validate.CodeClassNotInVocabulary) {
		t.Errorf("absent CLASS must be clean")
	}
}

func TestClassification_transpOutsideVocabularyIsVS054(t *testing.T) {
	c := withValue(vstar.CompEvent, "TRANSP", "BUSY")
	d, ok := findCode(validate.ValidateComponent(c), validate.CodeTranspNotInVocabulary)
	if !ok {
		t.Fatalf("TRANSP:BUSY on VEVENT must yield VS054")
	}
	if d.Severity != validate.SeverityError {
		t.Errorf("VS054 severity = %v, want Error", d.Severity)
	}
	if want := "VEVENT[uid=value-1].TRANSP"; d.Path != want {
		t.Errorf("VS054 path = %q, want %q", d.Path, want)
	}
}

func TestClassification_transpRegistryValuesAreClean(t *testing.T) {
	for _, v := range []vstar.Transp{vstar.TranspOpaque, vstar.TranspTransparent} {
		c := withValue(vstar.CompEvent, "TRANSP", string(v))
		if hasCode(validate.ValidateComponent(c), validate.CodeTranspNotInVocabulary) {
			t.Errorf("TRANSP:%s must be clean", v)
		}
	}
}

func TestClassification_transpCaseInsensitive(t *testing.T) {
	c := withValue(vstar.CompEvent, "TRANSP", "transparent")
	if hasCode(validate.ValidateComponent(c), validate.CodeTranspNotInVocabulary) {
		t.Errorf("lowercase TRANSP:transparent must be accepted")
	}
}

// TestClassification_transpNotTypeGated documents the no-scope
// decision: RFC 5545 admits TRANSP on VEVENT only, but V* diagnoses
// no scope rule for any property, so a VTODO carrying TRANSP gets
// the same value check and nothing else.
func TestClassification_transpNotTypeGated(t *testing.T) {
	c := withValue(vstar.CompTodo, "TRANSP", "BUSY")
	if !hasCode(validate.ValidateComponent(c), validate.CodeTranspNotInVocabulary) {
		t.Errorf("TRANSP:BUSY on VTODO must yield VS054")
	}
	c = withValue(vstar.CompTodo, "TRANSP", string(vstar.TranspOpaque))
	if ds := validate.ValidateComponent(c); len(ds) != 0 {
		t.Errorf("TRANSP:OPAQUE on VTODO must be clean (scope is not diagnosed), got %+v", ds)
	}
}
