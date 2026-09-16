// SPDX-License-Identifier: MIT

// GENERATED — do not edit.
// Source: spec/registry/ · Generator: tools/registry/gen.py
// Run `make registry-gen` after editing the registry JSON.

package validate

// Diagnostic codes. Each is stable across minor versions: a
// code never changes meaning and is never recycled. See
// docs/validate-codes.md for the full catalog.
const (
	// CodeMissingUID — Required common property UID is missing.
	// (spec/05 criterion 1)
	CodeMissingUID = "VS001"

	// CodeMissingDTSTAMP — Required common property DTSTAMP is
	// missing. (spec/05 criterion 1)
	CodeMissingDTSTAMP = "VS002"

	// CodeMissingXVSTARHash — Required common property X-VSTAR-HASH is
	// missing. (spec/05 criterion 1)
	CodeMissingXVSTARHash = "VS003"

	// CodeBadXVSTARHash — X-VSTAR-HASH is present but does not match
	// the recomputed hash. (spec/05 criterion 2)
	CodeBadXVSTARHash = "VS010"

	// CodeUnknownProperty — Property name is not on the RFC 5545/6350
	// allow-list and lacks the X- prefix. (spec/05 criterion 3)
	CodeUnknownProperty = "VS020"

	// CodeSupersessionMissingProps — A supersession VJOURNAL
	// (CATEGORIES contains status-supersession) is missing a required
	// property: RELATED-TO or X-VSTAR-EFFECTIVE-STATUS. (spec/05
	// criterion 4)
	CodeSupersessionMissingProps = "VS030"

	// CodeSupersessionOrphan — A supersession VJOURNAL's RELATED-TO
	// does not resolve to any component in the same Calendar (orphan
	// supersession). (spec/05 criterion 4)
	CodeSupersessionOrphan = "VS031"

	// CodeVTODOMissingDue — VTODO requires DUE, OR STATUS=COMPLETED
	// paired with COMPLETED. (spec/05 criterion 5)
	CodeVTODOMissingDue = "VS040"

	// CodeVEVENTMissingDTSTART — VEVENT requires DTSTART. (spec/05
	// criterion 5)
	CodeVEVENTMissingDTSTART = "VS041"

	// CodeVFREEBUSYMissingTimes — VFREEBUSY requires DTSTART AND
	// DTEND. (spec/05 criterion 5)
	CodeVFREEBUSYMissingTimes = "VS042"

	// CodeVCARDMissingRequired — VCARD (modeled as Component{Type:
	// "VCARD"}) requires VERSION AND UID. (spec/05 criterion 5)
	CodeVCARDMissingRequired = "VS043"

	// CodeStatusNotInVocabulary — STATUS value is outside the
	// vocabulary RFC 5545 §3.8.1.11 scopes to the component's own
	// type. (spec/05 criterion 8)
	CodeStatusNotInVocabulary = "VS044"

	// CodeRRuleUnsupported — RRULE value parses but uses a feature
	// outside the v0.2 rrule scope (FREQ=SECONDLY/MINUTELY, RSCALE —
	// see spec/03 §RRULE parsing scope). (spec/05 criterion 6)
	CodeRRuleUnsupported = "VS050"

	// CodeRRuleMalformed — RRULE value is malformed per RFC 5545
	// §3.3.10 (missing FREQ, INTERVAL≤0, both UNTIL+COUNT,
	// BYMONTHDAY=0, UNTIL not in form #2, etc.). (spec/05 criterion 6)
	CodeRRuleMalformed = "VS051"

	// CodeMalformedDuration — A duration-bearing property value is
	// malformed: the DURATION property, the relative (DURATION-valued)
	// form of TRIGGER, or a REPEAT count that is not a non-negative
	// integer (RFC 5545 §3.3.6 / §3.8.6.2); or a TRIGGER whose VALUE
	// parameter contradicts its value, or that carries RELATED on an
	// absolute trigger — see spec/03 §TRIGGER conventions. (spec/05
	// criterion 7)
	CodeMalformedDuration = "VS052"
)

// codeSeverities maps every diagnostic code to the severity the
// registry assigns it. Emitters use the constants directly; this
// map exists so tooling can classify a code it did not emit.
var codeSeverities = map[string]Severity{
	CodeMissingUID:               SeverityError,
	CodeMissingDTSTAMP:           SeverityError,
	CodeMissingXVSTARHash:        SeverityError,
	CodeBadXVSTARHash:            SeverityError,
	CodeUnknownProperty:          SeverityWarning,
	CodeSupersessionMissingProps: SeverityError,
	CodeSupersessionOrphan:       SeverityError,
	CodeVTODOMissingDue:          SeverityError,
	CodeVEVENTMissingDTSTART:     SeverityError,
	CodeVFREEBUSYMissingTimes:    SeverityError,
	CodeVCARDMissingRequired:     SeverityError,
	CodeStatusNotInVocabulary:    SeverityError,
	CodeRRuleUnsupported:         SeverityWarning,
	CodeRRuleMalformed:           SeverityError,
	CodeMalformedDuration:        SeverityError,
}

// SeverityOf reports the severity the registry assigns to code,
// and whether the code is known at all. An unknown code yields
// (SeverityError, false) — callers must check ok before acting.
func SeverityOf(code string) (Severity, bool) {
	s, ok := codeSeverities[code]
	return s, ok
}

// Codes returns every diagnostic code the package defines, sorted.
// The slice is freshly allocated on each call, so callers may
// retain or mutate it.
func Codes() []string {
	return []string{
		CodeMissingUID,
		CodeMissingDTSTAMP,
		CodeMissingXVSTARHash,
		CodeBadXVSTARHash,
		CodeUnknownProperty,
		CodeSupersessionMissingProps,
		CodeSupersessionOrphan,
		CodeVTODOMissingDue,
		CodeVEVENTMissingDTSTART,
		CodeVFREEBUSYMissingTimes,
		CodeVCARDMissingRequired,
		CodeStatusNotInVocabulary,
		CodeRRuleUnsupported,
		CodeRRuleMalformed,
		CodeMalformedDuration,
	}
}

// standardProperties is the allow-list of property names defined
// by RFC 5545 (iCalendar) §3.7-§3.8 and RFC 6350 (vCard) §6. A
// name that is absent here AND does not begin with "X-" earns a
// SeverityWarning VS020 from spec/05 criterion 3.
//
// Keys are upper-cased; lookup is case-insensitive at the call
// site (see isStandardProperty).
var standardProperties = map[string]struct{}{
	// RFC 5545 (iCalendar) §3.7-§3.8.
	"ACTION":           {},
	"ATTACH":           {},
	"ATTENDEE":         {},
	"CALSCALE":         {},
	"CATEGORIES":       {},
	"CLASS":            {},
	"COMMENT":          {},
	"COMPLETED":        {},
	"CONTACT":          {},
	"CREATED":          {},
	"DESCRIPTION":      {},
	"DTEND":            {},
	"DTSTAMP":          {},
	"DTSTART":          {},
	"DUE":              {},
	"DURATION":         {},
	"EXDATE":           {},
	"EXRULE":           {},
	"FREEBUSY":         {},
	"GEO":              {},
	"LAST-MODIFIED":    {},
	"LOCATION":         {},
	"METHOD":           {},
	"ORGANIZER":        {},
	"PERCENT-COMPLETE": {},
	"PRIORITY":         {},
	"PRODID":           {},
	"RDATE":            {},
	"RECURRENCE-ID":    {},
	"RELATED-TO":       {},
	"REPEAT":           {},
	"REQUEST-STATUS":   {},
	"RESOURCES":        {},
	"RRULE":            {},
	"SEQUENCE":         {},
	"STATUS":           {},
	"SUMMARY":          {},
	"TRANSP":           {},
	"TRIGGER":          {},
	"TZID":             {},
	"TZNAME":           {},
	"TZOFFSETFROM":     {},
	"TZOFFSETTO":       {},
	"TZURL":            {},
	"UID":              {},
	"URL":              {},
	"VERSION":          {},
	// RFC 6350 (vCard) §6.
	"ADR":          {},
	"ANNIVERSARY":  {},
	"BDAY":         {},
	"CALADRURI":    {},
	"CALURI":       {},
	"CLIENTPIDMAP": {},
	"EMAIL":        {},
	"FBURL":        {},
	"FN":           {},
	"GENDER":       {},
	"IMPP":         {},
	"KEY":          {},
	"KIND":         {},
	"LANG":         {},
	"LOGO":         {},
	"MEMBER":       {},
	"N":            {},
	"NICKNAME":     {},
	"NOTE":         {},
	"ORG":          {},
	"PHOTO":        {},
	"RELATED":      {},
	"REV":          {},
	"ROLE":         {},
	"SOUND":        {},
	"SOURCE":       {},
	"TEL":          {},
	"TITLE":        {},
	"TZ":           {},
	"XML":          {},
}

// StandardPropertyCount returns the number of property names in
// the standard allow-list. Exported for diagnostic and test code
// — not part of the validator hot path.
func StandardPropertyCount() int {
	return len(standardProperties)
}

// StatusVocabulary is the registry's STATUS value vocabulary,
// keyed by the component type RFC 5545 §3.8.1.11 scopes it to.
// A type absent from the map admits no STATUS vocabulary.
//
// This is the cross-language table, not the Go wire constants.
// The validator keeps deriving its table from the constants the
// codec encodes against — see status_vocabulary.go — and a test
// asserts the two agree. That way the table cannot drift from
// the codec, and neither can drift from the other ports.
var StatusVocabulary = map[string][]string{
	"VEVENT":   {"TENTATIVE", "CONFIRMED", "CANCELLED"},
	"VJOURNAL": {"DRAFT", "FINAL", "CANCELLED"},
	"VTODO":    {"NEEDS-ACTION", "IN-PROCESS", "COMPLETED", "CANCELLED"},
}

// ClassVocabulary is the registry's CLASS value vocabulary, RFC
// 5545 §3.8.1.3.
var ClassVocabulary = []string{
	"PUBLIC",
	"PRIVATE",
	"CONFIDENTIAL",
}

// TranspVocabulary is the registry's TRANSP value vocabulary, RFC
// 5545 §3.8.2.7.
var TranspVocabulary = []string{
	"OPAQUE",
	"TRANSPARENT",
}

// RelTypeVocabulary is the registry's registered RELTYPE
// vocabulary, RFC 5545 §3.2.15 plus RFC 9253 §11.4. The vocabulary
// is registered, not closed: RELTYPE admits IANA and X- values
// outside it.
var RelTypeVocabulary = []string{
	"PARENT",
	"CHILD",
	"SIBLING",
	"FINISHTOSTART",
	"FINISHTOFINISH",
	"STARTTOFINISH",
	"STARTTOSTART",
	"DEPENDS-ON",
	"FIRST",
	"NEXT",
	"CONCEPT",
	"REFID",
}
