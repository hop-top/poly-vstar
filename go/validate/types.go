// SPDX-License-Identifier: MIT

package validate

import (
	"strings"

	vstar "hop.top/vstar"
)

// checkTypeSpecific dispatches per CompType. Components whose type
// has no extra MUST in spec/05 §5 (VJOURNAL, VTIMEZONE, VALARM,
// VCALENDAR) yield nothing here.
//
// VCARD-as-Component: package vstar models top-level vCards via
// the dedicated Card type, not Component. This validator does not
// see Component{Type: "VCARD"} from a parsed VCALENDAR (the codec
// produces Card via a separate path). The case is included
// defensively so Wave 4 callers building Components by hand still
// get the rule.
func checkTypeSpecific(c vstar.Component, path string) []Diagnostic {
	switch c.Type {
	case vstar.CompTodo:
		return checkVTODO(c, path)
	case vstar.CompEvent:
		return checkVEVENT(c, path)
	case vstar.CompFreeBusy:
		return checkVFREEBUSY(c, path)
	case "VCARD":
		return checkVCARDComponent(c, path)
	}
	return nil
}

// checkVTODO emits VS040 when neither route to a "scheduled" VTODO
// is satisfied: a present DUE OR (STATUS=COMPLETED with COMPLETED).
//
// RFC 5545 §3.6.2 lets a completed VTODO drop DUE so long as its
// COMPLETED timestamp records when it finished — that path is
// honored here.
func checkVTODO(c vstar.Component, path string) []Diagnostic {
	if _, ok := c.Get("DUE"); ok {
		return nil
	}
	status, hasStatus := c.Get("STATUS")
	if hasStatus && strings.EqualFold(status.Value, string(vstar.TodoCompleted)) {
		if _, ok := c.Get("COMPLETED"); ok {
			return nil
		}
	}
	return []Diagnostic{{
		Severity: SeverityError,
		Code:     CodeVTODOMissingDue,
		Message:  "VTODO requires DUE, or STATUS=COMPLETED paired with COMPLETED (spec/05 §5; RFC 5545 §3.6.2)",
		Path:     path,
	}}
}

// checkVEVENT emits VS041 when DTSTART is absent. RFC 5545 §3.6.1
// allows DTSTART to be absent in a non-PUBLISH METHOD context, but
// V* is strict: agentic playthroughs always anchor to a start time.
func checkVEVENT(c vstar.Component, path string) []Diagnostic {
	if _, ok := c.Get("DTSTART"); ok {
		return nil
	}
	return []Diagnostic{{
		Severity: SeverityError,
		Code:     CodeVEVENTMissingDTSTART,
		Message:  "VEVENT requires DTSTART (spec/05 §5; RFC 5545 §3.6.1)",
		Path:     path + ".DTSTART",
	}}
}

// checkVFREEBUSY emits VS042 when either DTSTART or DTEND (or
// both) is missing. The diagnostic names which.
func checkVFREEBUSY(c vstar.Component, path string) []Diagnostic {
	_, hasStart := c.Get("DTSTART")
	_, hasEnd := c.Get("DTEND")
	if hasStart && hasEnd {
		return nil
	}
	missing := make([]string, 0, 2)
	if !hasStart {
		missing = append(missing, "DTSTART")
	}
	if !hasEnd {
		missing = append(missing, "DTEND")
	}
	return []Diagnostic{{
		Severity: SeverityError,
		Code:     CodeVFREEBUSYMissingTimes,
		Message:  "VFREEBUSY requires DTSTART and DTEND; missing: " + strings.Join(missing, ", ") + " (spec/05 §5; RFC 5545 §3.6.4)",
		Path:     path,
	}}
}

// checkVCARDComponent handles the (uncommon) case of a vCard
// modeled as Component{Type: "VCARD"}. The native Card path uses
// ValidateCard (not yet implemented; hand-built vCards go through
// the dedicated Card constructors in helpers — out of scope for
// this track).
func checkVCARDComponent(c vstar.Component, path string) []Diagnostic {
	_, hasVersion := c.Get("VERSION")
	_, hasUID := c.Get("UID")
	if hasVersion && hasUID {
		return nil
	}
	missing := make([]string, 0, 2)
	if !hasVersion {
		missing = append(missing, "VERSION")
	}
	if !hasUID {
		missing = append(missing, "UID")
	}
	return []Diagnostic{{
		Severity: SeverityError,
		Code:     CodeVCARDMissingRequired,
		Message:  "VCARD requires VERSION and UID; missing: " + strings.Join(missing, ", ") + " (spec/05 §5; RFC 6350 §6.7.6, §6.7.9)",
		Path:     path,
	}}
}
