// SPDX-License-Identifier: MIT

package validate

import (
	"errors"
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/rrule"
)

// propRRULE is the RFC 5545 §3.8.5.3 property name. Pulled to a
// constant to keep the goconst linter happy and to centralize
// the case-insensitive name we match in checkRRule.
const propRRULE = "RRULE"

// checkRRule emits one Diagnostic per RRULE property whose value
// fails rrule.ValidateRRule. The split is by sentinel:
//
//   - errors.Is(err, rrule.ErrUnsupportedRRule) → VS050 (Warning).
//   - errors.Is(err, vstar.ErrMalformed)        → VS051 (Error).
//
// When both classify (defensive — should not happen in practice)
// the Unsupported path wins because it is the more specific
// classification.
func checkRRule(c vstar.Component, path string) []Diagnostic {
	var out []Diagnostic
	for _, p := range c.Props {
		if !strings.EqualFold(p.Name, propRRULE) {
			continue
		}
		err := rrule.ValidateRRule(p.Value)
		if err == nil {
			continue
		}
		if errors.Is(err, rrule.ErrUnsupportedRRule) {
			out = append(out, Diagnostic{
				Severity: SeverityWarning,
				Code:     CodeRRuleUnsupported,
				Message:  "RRULE uses a feature outside the RRULE parsing scope (spec/03 §RRULE parsing scope): " + err.Error(),
				Path:     path + "." + propRRULE,
			})
			continue
		}
		if errors.Is(err, vstar.ErrMalformed) {
			out = append(out, Diagnostic{
				Severity: SeverityError,
				Code:     CodeRRuleMalformed,
				Message:  "RRULE is malformed (RFC 5545 §3.3.10): " + err.Error(),
				Path:     path + "." + propRRULE,
			})
			continue
		}
		// Unknown classification — emit as malformed conservatively
		// so the consumer at least sees the finding.
		out = append(out, Diagnostic{
			Severity: SeverityError,
			Code:     CodeRRuleMalformed,
			Message:  "RRULE failed validation: " + err.Error(),
			Path:     path + "." + propRRULE,
		})
	}
	return out
}
