// SPDX-License-Identifier: MIT

package validate

import (
	vstar "hop.top/vstar"
)

// xvstarHashProperty is the property name V* uses to carry the
// content hash. Mirrors hashing.XVSTARHashProperty without
// importing hashing here (validate has its own const so it can
// inspect raw property names without a transitive cycle risk).
const xvstarHashProperty = "X-VSTAR-HASH"

// checkRequiredCommon emits one Error diagnostic per missing
// required common property. UID, DTSTAMP, X-VSTAR-HASH are checked
// in that order so the diagnostic stream is deterministic.
//
// Path is the dotted component locator (e.g. "VCALENDAR.VTODO[uid=
// foo]"); per-property paths append the property name (e.g.
// "...].UID").
func checkRequiredCommon(c vstar.Component, path string) []Diagnostic {
	var out []Diagnostic
	if _, ok := c.Get("UID"); !ok {
		out = append(out, Diagnostic{
			Severity: SeverityError,
			Code:     CodeMissingUID,
			Message:  "required common property UID is missing (spec/02)",
			Path:     path + ".UID",
		})
	}
	if _, ok := c.Get("DTSTAMP"); !ok {
		out = append(out, Diagnostic{
			Severity: SeverityError,
			Code:     CodeMissingDTSTAMP,
			Message:  "required common property DTSTAMP is missing (spec/02)",
			Path:     path + ".DTSTAMP",
		})
	}
	if _, ok := c.Get(xvstarHashProperty); !ok {
		out = append(out, Diagnostic{
			Severity: SeverityError,
			Code:     CodeMissingXVSTARHash,
			Message:  "required common property X-VSTAR-HASH is missing (spec/02)",
			Path:     path + "." + xvstarHashProperty,
		})
	}
	return out
}
