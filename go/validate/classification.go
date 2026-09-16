// SPDX-License-Identifier: MIT

package validate

import (
	"strings"

	vstar "hop.top/vstar"
)

// Closed-vocabulary property names checked by this rule.
const (
	propCLASS  = "CLASS"
	propTRANSP = "TRANSP"
)

// classVocabulary and transpVocabulary are the wire values RFC 5545
// §3.8.1.3 and §3.8.2.7 allow. Built from the package's own wire
// constants rather than the generated ClassVocabulary and
// TranspVocabulary, for the reason statusVocabularies gives: these
// are what the codec encodes against, so the table cannot disagree
// with what the library writes. The registry's copy is reconciled
// against the constants by
// TestClassTranspRelTypeVocabulary_matchesRegistry.
var (
	classVocabulary = []string{
		string(vstar.ClassPublic),
		string(vstar.ClassPrivate),
		string(vstar.ClassConfidential),
	}
	transpVocabulary = []string{
		string(vstar.TranspOpaque),
		string(vstar.TranspTransparent),
	}
)

// checkClassification emits VS053 when c carries a CLASS outside the
// RFC 5545 §3.8.1.3 vocabulary and VS054 when it carries a TRANSP
// outside §3.8.2.7's.
//
// Comparison is case-insensitive (spec/05 §8, RFC 5545 §3.1). The
// value is checked on any component carrying the property — there is
// no type gating, because V* diagnoses no scope rule for any
// property. An X- or IANA token on CLASS, which the RFC's ABNF
// admits, is still VS053: spec/05 §8 binds the value to the three
// registered names.
func checkClassification(c vstar.Component, path string) []Diagnostic {
	var out []Diagnostic
	if d, ok := vocabularyDiagnostic(c, path, propCLASS, classVocabulary,
		CodeClassNotInVocabulary, "RFC 5545 §3.8.1.3"); ok {
		out = append(out, d)
	}
	if d, ok := vocabularyDiagnostic(c, path, propTRANSP, transpVocabulary,
		CodeTranspNotInVocabulary, "RFC 5545 §3.8.2.7"); ok {
		out = append(out, d)
	}
	return out
}

// vocabularyDiagnostic returns the VS044-shaped diagnostic for
// property name on c when its value is outside allowed, and ok=false
// when the property is absent or its value is allowed.
func vocabularyDiagnostic(c vstar.Component, path, name string, allowed []string, code, ref string) (Diagnostic, bool) {
	p, ok := c.Get(name)
	if !ok {
		return Diagnostic{}, false
	}
	for _, want := range allowed {
		if strings.EqualFold(p.Value, want) {
			return Diagnostic{}, false
		}
	}
	return Diagnostic{
		Severity: SeverityError,
		Code:     code,
		Message: name + " value " + p.Value + " is not valid; allowed: " +
			strings.Join(allowed, ", ") + " (" + ref + ")",
		Path: path + "." + name,
	}, true
}
