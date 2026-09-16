// SPDX-License-Identifier: MIT

package validate

import (
	"strings"

	vstar "hop.top/vstar"
)

// Integer-valued property names bounded by this rule.
const (
	propPRIORITY        = "PRIORITY"
	propPERCENTCOMPLETE = "PERCENT-COMPLETE"
	propSEQUENCE        = "SEQUENCE"
)

// integerDomain is one bounded integer property: its name, the RFC
// 5545 section that bounds it, and its upper bound as a digit string
// ("" when unbounded). The lower bound is always 0, which the
// canonical-decimal form already enforces (no sign).
type integerDomain struct {
	name    string
	section string
	max     string
}

// integerDomains is the table VS055 checks, in the order the codes
// catalog lists the properties.
var integerDomains = []integerDomain{
	{name: propPRIORITY, section: "§3.8.1.9", max: "9"},
	{name: propPERCENTCOMPLETE, section: "§3.8.1.8", max: "100"},
	{name: propSEQUENCE, section: "§3.8.7.4", max: ""},
}

// isCanonicalDecimal reports whether s is a canonical non-negative
// decimal per spec/05 §8: digits only, no sign, no whitespace, and no
// leading zero unless s is exactly "0". Equivalent to the regular
// expression ^(0|[1-9][0-9]*)$.
//
// The check is textual on purpose. It never converts s to a machine
// integer, so a value past int range (a SEQUENCE of 2^64) is
// well-formed — the spec bounds SEQUENCE below, never above.
func isCanonicalDecimal(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s == "0" || s[0] != '0'
}

// exceedsDigitString reports whether the canonical decimal v is
// numerically greater than the canonical decimal max. Both must
// already satisfy isCanonicalDecimal: with no leading zeros, a longer
// string is a larger number and equal lengths compare lexically.
func exceedsDigitString(v, max string) bool {
	if len(v) != len(max) {
		return len(v) > len(max)
	}
	return v > max
}

// checkIntegerDomains emits one VS055 per bounded integer property
// whose value is not a canonical decimal inside its RFC 5545 domain.
//
// The value is checked wherever the property appears; the rule does
// not gate on component type (PERCENT-COMPLETE on a VEVENT is
// bounded, not flagged for scope). One diagnostic per offending
// property, at the property's path, like VS044 and VS052.
func checkIntegerDomains(c vstar.Component, path string) []Diagnostic {
	var out []Diagnostic
	for _, p := range c.Props {
		for _, dom := range integerDomains {
			if !strings.EqualFold(p.Name, dom.name) {
				continue
			}
			if !isCanonicalDecimal(p.Value) {
				out = append(out, Diagnostic{
					Severity: SeverityError,
					Code:     CodeIntegerOutOfDomain,
					Message: dom.name + " is not a canonical non-negative decimal (RFC 5545 " +
						dom.section + ", spec/05 §8): " + p.Value,
					Path: path + "." + dom.name,
				})
				break
			}
			if dom.max != "" && exceedsDigitString(p.Value, dom.max) {
				out = append(out, Diagnostic{
					Severity: SeverityError,
					Code:     CodeIntegerOutOfDomain,
					Message: dom.name + " value " + p.Value + " is outside 0–" + dom.max +
						" (RFC 5545 " + dom.section + ")",
					Path: path + "." + dom.name,
				})
			}
			break
		}
	}
	return out
}
