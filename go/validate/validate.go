// SPDX-License-Identifier: MIT

// Package validate enforces V* semantic invariants that the codec
// layer cannot catch. A document can be syntactically valid RFC 5545
// or RFC 6350 (so parsing succeeds) yet still violate V* discipline:
// missing X-VSTAR-HASH, corrupted hash, malformed extension namespace,
// or a type-specific required property absent.
//
// The two entry points are Validate (Calendar) and ValidateComponent
// (single Component). Both return a slice of Diagnostic; an empty
// slice means clean.
//
// Diagnostic carries a stable Code (e.g. "VS001") that consumers may
// match programmatically. The Code → meaning catalog lives at
// docs/validate-codes.md at the repository root and is part of the
// library's public surface. Codes are stable across minor versions
// per semver.
//
// Implemented coverage of spec/05 (the conformance criteria):
//
//   - §1 required common properties (UID, DTSTAMP, X-VSTAR-HASH) →
//     codes VS001, VS002, VS003 (errors).
//   - §2 X-VSTAR-HASH integrity (recompute and compare) → code
//     VS010 (error). VS010 only fires when the property is present
//     and wrong; an absent property is reported by §1 as VS003.
//   - §3 extension namespace compliance — non-standard properties
//     without an X- prefix → code VS020 (warning). The standard
//     property allow-list is sourced from RFC 5545 §3.7-§3.8 and
//     RFC 6350 §6, rendered from spec/registry/ into codes_gen.go.
//   - §4 supersession discipline — a superseding component must
//     carry RELATED-TO and X-VSTAR-EFFECTIVE-STATUS → VS030 (error);
//     one whose target is absent from the ledger → VS031 (error,
//     Validate only). See supersession.go.
//   - §5 type-specific required properties — VTODO needs DUE or
//     (STATUS=COMPLETED + COMPLETED), VEVENT needs DTSTART,
//     VFREEBUSY needs DTSTART + DTEND, VCARD needs VERSION + UID
//     → codes VS040, VS041, VS042, VS043 (errors).
//   - §6 RRULE conformance → codes VS050 (warning), VS051 (error).
//   - §7 DURATION well-formedness — the DURATION property, the
//     relative form of TRIGGER, and the REPEAT count → code VS052
//     (error).
//   - §8 enumerated and integer value domains — STATUS values must
//     lie in the vocabulary RFC 5545 §3.8.1.11 scopes to the
//     component's own type → code VS044 (error). The other domains
//     §8 bounds (CLASS, TRANSP, PRIORITY, PERCENT-COMPLETE,
//     SEQUENCE, REPEAT) have no code yet.
//
// Path syntax: Diagnostic.Path is a dotted component/property
// locator. Examples:
//
//   - "VCALENDAR" — calendar-level diagnostic.
//   - "VCALENDAR.VTODO[uid=foo]" — component-level diagnostic on
//     the VTODO whose UID is "foo".
//   - "VCALENDAR.VTODO[uid=foo].DTSTAMP" — property-level
//     diagnostic on the DTSTAMP of that VTODO.
//
// When a component has no UID (so it cannot be identified), the
// segment uses a positional index instead: "VCALENDAR.VTODO[#3]"
// is the fourth (0-indexed) VTODO with no UID.
//
// ValidateComponent is the single-component variant; its diagnostic
// paths omit the "VCALENDAR." prefix and start at the component
// itself, e.g. "VTODO[uid=foo].DTSTAMP".
package validate

import (
	"strconv"

	vstar "hop.top/vstar"
)

// Severity is the impact level of a Diagnostic. Error means the
// document violates a MUST in spec/05; Warning means a SHOULD or
// a stylistic concern (e.g. unknown property name without X-*
// prefix).
type Severity int

// Severity levels. Numeric values are an implementation detail; do
// not depend on them — match by constant name.
const (
	// SeverityError marks a MUST violation: the document is not
	// V* conformant.
	SeverityError Severity = iota
	// SeverityWarning marks a SHOULD violation or stylistic
	// concern.
	SeverityWarning
)

// String returns the lowercase name of the severity ("error",
// "warning"). Useful for display and logging; programmatic checks
// should still match the Severity constant.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "unknown"
	}
}

// Diagnostic is a single validation finding. Severity, Code, and
// Message describe the finding; Path is a dotted locator that
// pinpoints where in the document the problem lives.
//
// Code is a stable identifier (e.g. "VS001") cataloged in
// docs/validate-codes.md. Codes are stable across minor versions
// per semver — consumers may rely on them for programmatic match.
type Diagnostic struct {
	// Severity is Error or Warning.
	Severity Severity
	// Code is the stable catalog identifier (e.g. "VS001").
	Code string
	// Message is human-readable detail. Not stable across
	// versions; use Code for programmatic match.
	Message string
	// Path is the dotted locator (see package doc).
	Path string
}

// Validate checks every component in cal for V* semantic violations
// and returns the accumulated diagnostics. An empty slice means cal
// is clean. The order of returned diagnostics follows component
// order, then rule order within a component.
//
// Validate does not mutate cal. Calendar-level diagnostics use the
// path "VCALENDAR"; component-level diagnostics use
// "VCALENDAR.<TYPE>[<id>]" where <id> is "uid=<uid>" if the
// component has a UID, or "#<index>" otherwise.
func Validate(cal vstar.Calendar) []Diagnostic {
	var out []Diagnostic
	uidIndex := map[vstar.CompType]int{}
	for _, comp := range cal.Components {
		path := componentPath(comp, uidIndex)
		full := "VCALENDAR." + path
		out = append(out, validateComponentAtWithLedger(comp, full, cal.Components)...)
	}
	return out
}

// ValidateComponent checks a single Component in isolation and
// returns the accumulated diagnostics. The diagnostic paths start
// at the component itself, e.g. "VTODO[uid=foo].DTSTAMP".
//
// ValidateComponent is the right entry point when the caller has
// no parent Calendar context (e.g. validating a freshly minted
// component before appending it to a Calendar). When a Calendar
// exists, prefer Validate — it gives more accurate paths and a
// consistent diagnostic stream.
func ValidateComponent(c vstar.Component) []Diagnostic {
	uidIndex := map[vstar.CompType]int{}
	path := componentPath(c, uidIndex)
	return validateComponentAt(c, path)
}

// validateComponentAt runs every component-local check against c
// and prepends path to any per-property paths the checks emit.
// VS031 (supersession orphan) is intentionally NOT run from this
// path — it requires the full ledger and only fires from
// Validate (calendar-context).
func validateComponentAt(c vstar.Component, path string) []Diagnostic {
	return validateComponentAtWithLedger(c, path, nil)
}

// validateComponentAtWithLedger runs every check against c at path,
// using ledger as the cross-component context for VS031 (orphan
// supersession RELATED-TO). When ledger is nil, VS031 is skipped
// — VS030 still fires.
func validateComponentAtWithLedger(c vstar.Component, path string, ledger []vstar.Component) []Diagnostic {
	var out []Diagnostic
	out = append(out, checkRequiredCommon(c, path)...)
	out = append(out, checkHashIntegrity(c, path)...)
	out = append(out, checkExtensionNamespace(c, path)...)
	out = append(out, checkTypeSpecific(c, path)...)
	out = append(out, checkStatusVocabulary(c, path)...)
	out = append(out, checkSupersessionDiscipline(c, ledger, path)...)
	out = append(out, checkRRule(c, path)...)
	out = append(out, checkDuration(c, path)...)
	return out
}

// componentPath returns the Path segment identifying c relative to
// its container. uidIndex tracks the running positional index per
// CompType so unidentified components get a stable "#N" suffix.
func componentPath(c vstar.Component, uidIndex map[vstar.CompType]int) string {
	uid := c.UID()
	if uid != "" {
		return string(c.Type) + "[uid=" + uid + "]"
	}
	idx := uidIndex[c.Type]
	uidIndex[c.Type] = idx + 1
	return string(c.Type) + "[#" + strconv.Itoa(idx) + "]"
}
