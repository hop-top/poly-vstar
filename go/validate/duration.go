// SPDX-License-Identifier: MIT

package validate

import (
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/duration"
)

// Duration-bearing property names checked by this rule.
const (
	propDURATION = "DURATION"
	propTRIGGER  = "TRIGGER"
	propREPEAT   = "REPEAT"
)

// checkDuration emits one Diagnostic per duration-bearing property
// whose value is malformed.
//
// TRIGGER is routed through duration.ParseTrigger so both value
// forms and the RELATED/VALUE parameter rules are enforced together;
// DURATION and REPEAT are checked directly, since a bare DURATION
// property has only the one legal form.
func checkDuration(c vstar.Component, path string) []Diagnostic {
	var out []Diagnostic
	for _, p := range c.Props {
		switch {
		case strings.EqualFold(p.Name, propTRIGGER):
			if _, err := duration.ParseTrigger(p); err != nil {
				out = append(out, Diagnostic{
					Severity: SeverityError,
					Code:     CodeMalformedDuration,
					Message:  "TRIGGER is malformed (RFC 5545 §3.8.6.3): " + err.Error(),
					Path:     path + "." + propTRIGGER,
				})
			}
		case strings.EqualFold(p.Name, propDURATION):
			if _, err := duration.Parse(p.Value); err != nil {
				out = append(out, Diagnostic{
					Severity: SeverityError,
					Code:     CodeMalformedDuration,
					Message:  "DURATION is malformed (RFC 5545 §3.3.6): " + err.Error(),
					Path:     path + "." + propDURATION,
				})
			}
		case strings.EqualFold(p.Name, propREPEAT):
			// The same textual canonical-decimal rule VS055
			// applies to the bounded integers (spec/05 §8): a
			// sign or a leading zero is malformed even though
			// the number is in range. REPEAT stays under VS052
			// because a published code never changes meaning.
			if !isCanonicalDecimal(p.Value) {
				out = append(out, Diagnostic{
					Severity: SeverityError,
					Code:     CodeMalformedDuration,
					Message:  "REPEAT is not a canonical non-negative integer (RFC 5545 §3.8.6.2, spec/05 §8): " + p.Value,
					Path:     path + "." + propREPEAT,
				})
			}
		}
	}
	return out
}
