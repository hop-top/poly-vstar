// SPDX-License-Identifier: MIT

// Package vstar is the Go reference implementation of the V*
// specification (a convention over RFC 5545 / RFC 6350 for agentic
// systems). This package exposes the in-memory data model —
// Property, Param, Component, Calendar, Card — plus the wire-string
// enums (CompType, Kind, TodoStatus, RelType) and error sentinels
// every later layer (codecs, canonicalization, validation, helpers)
// builds on.
// enums (CompType, Kind, TodoStatus, EventStatus, JournalStatus,
// Class, Transp) and error sentinels every later layer (codecs,
// canonicalization, validation, helpers) builds on.
//
// I/O, parsing, encoding, canonical form, time helpers, and
// validation live in dedicated packages; this file ships data types
// only.
//
// See README.md and the spec at
// https://github.com/hop-top/poly-vstar/tree/main/spec.
package vstar

import "strings"

// CompType is the wire-string component type of a Component
// (e.g. "VEVENT"). Constants below mirror the RFC 5545 §3.6 component
// identifiers exactly. VCARD is intentionally absent — vCards are
// represented by Card, not Component.
type CompType string

// RFC 5545 §3.4 + §3.6 component identifiers, plus the VCALENDAR
// container identifier from §3.4.
const (
	CompCalendar CompType = "VCALENDAR"
	CompTodo     CompType = "VTODO"
	CompJournal  CompType = "VJOURNAL"
	CompEvent    CompType = "VEVENT"
	CompFreeBusy CompType = "VFREEBUSY"
	CompTimezone CompType = "VTIMEZONE"
	CompAlarm    CompType = "VALARM"
)

// Kind is the wire-string KIND value for VCARD components per
// RFC 6350 §6.1.4. Values are lowercase per the RFC's IANA registry.
type Kind string

// RFC 6350 §6.1.4 KIND values. The RFC also lists "location"; only
// the enum values used by V* are defined here.
const (
	KindIndividual Kind = "individual"
	KindOrg        Kind = "org"
	KindGroup      Kind = "group"
)

// TodoStatus is the wire-string STATUS value for VTODO components
// per RFC 5545 §3.8.1.11. Values are uppercase per the RFC.
type TodoStatus string

// RFC 5545 §3.8.1.11 STATUS values applicable to VTODO.
const (
	TodoNeedsAction TodoStatus = "NEEDS-ACTION"
	TodoInProcess   TodoStatus = "IN-PROCESS"
	TodoCompleted   TodoStatus = "COMPLETED"
	TodoCancelled   TodoStatus = "CANCELLED" //nolint:misspell // RFC 5545 §3.8.1.11 wire string is "CANCELLED" (double L); not US "CANCELED".
)

// RelType is the wire-string RELTYPE parameter value on a RELATED-TO
// property (RFC 5545 §3.2.15, extended by RFC 9253 §4 and §5; the
// registry is RFC 9253 §11.4). RELTYPE values are compared
// case-insensitively per RFC 5545 §3.2 — use ParseRelType to fold
// wire input onto the constants below, or EqualFold to compare
// without allocating.
//
// The type is deliberately open: IANA may register further values and
// RFC 5545 permits X-prefixed extensions, so any string is a valid
// RelType. The constants name the registered vocabulary; they do not
// bound it.
type RelType string

// RFC 5545 §3.2.15 hierarchical relationship types. An omitted
// RELTYPE means PARENT.
const (
	RelParent  RelType = "PARENT"
	RelChild   RelType = "CHILD"
	RelSibling RelType = "SIBLING"
)

// RFC 9253 §4 temporal relationship types. The edge lives on the
// predecessor and points at the successor: FINISHTOSTART means the
// referenced component cannot start until the referencing one
// finishes, and the other three read the same way.
const (
	RelFinishToStart  RelType = "FINISHTOSTART"
	RelFinishToFinish RelType = "FINISHTOFINISH"
	RelStartToFinish  RelType = "STARTTOFINISH"
	RelStartToStart   RelType = "STARTTOSTART"
)

// RFC 9253 §5 relationship types. DEPENDS-ON: the referencing
// component depends on the referenced one. FIRST and NEXT order a
// chain: the referenced component is the first, or the next, member
// of a series the referencing one belongs to. CONCEPT and REFID
// reference every component whose CONCEPT or REFID property matches
// the RELATED-TO value.
const (
	RelDependsOn RelType = "DEPENDS-ON"
	RelFirst     RelType = "FIRST"
	RelNext      RelType = "NEXT"
	RelConcept   RelType = "CONCEPT"
	RelRefID     RelType = "REFID"
)

// DefaultRelType is the value RFC 5545 §3.2.15 assigns when the
// RELTYPE parameter is omitted from a RELATED-TO property.
const DefaultRelType = RelParent

// relTypes is the registered RELTYPE vocabulary (RFC 5545 §3.2.15 and
// RFC 9253 §11.4), keyed by canonical wire string, for ParseRelType's
// case-folded lookup.
var relTypes = map[string]RelType{
	string(RelParent):         RelParent,
	string(RelChild):          RelChild,
	string(RelSibling):        RelSibling,
	string(RelFinishToStart):  RelFinishToStart,
	string(RelFinishToFinish): RelFinishToFinish,
	string(RelStartToFinish):  RelStartToFinish,
	string(RelStartToStart):   RelStartToStart,
	string(RelDependsOn):      RelDependsOn,
	string(RelFirst):          RelFirst,
	string(RelNext):           RelNext,
	string(RelConcept):        RelConcept,
	string(RelRefID):          RelRefID,
}

// ParseRelType folds a wire RELTYPE value onto a registered constant,
// case-insensitively per RFC 5545 §3.2. It reports whether s named a
// registered value.
//
// An empty s yields DefaultRelType (PARENT) with ok=true, matching the
// RFC 5545 §3.2.15 rule that an omitted RELTYPE means PARENT. An
// unregistered value — an X-prefixed extension, or one from a later
// IANA registration — is returned verbatim with ok=false, so callers
// that accept extensions can keep the original spelling.
func ParseRelType(s string) (RelType, bool) {
	if s == "" {
		return DefaultRelType, true
	}
	if rt, found := relTypes[strings.ToUpper(s)]; found {
		return rt, true
	}
	return RelType(s), false
}

// EqualFold reports whether r and s name the same RELTYPE, comparing
// case-insensitively per RFC 5545 §3.2.
func (r RelType) EqualFold(s string) bool {
	return strings.EqualFold(string(r), s)
}

// EventStatus is the wire-string STATUS value for VEVENT components
// per RFC 5545 §3.8.1.11. Values are uppercase per the RFC.
//
// EventStatus, TodoStatus and JournalStatus are deliberately
// distinct Go types even though the cancellation value is spelled
// identically in all three: the RFC scopes each vocabulary to one
// component type, and separate types make a cross-type assignment
// a compile error rather than a wire-level conformance bug.
type EventStatus string

// RFC 5545 §3.8.1.11 STATUS values applicable to VEVENT.
const (
	EventTentative EventStatus = "TENTATIVE"
	EventConfirmed EventStatus = "CONFIRMED"
	EventCancelled EventStatus = "CANCELLED" //nolint:misspell // RFC 5545 §3.8.1.11 wire string is "CANCELLED" (double L); not US "CANCELED".
)

// JournalStatus is the wire-string STATUS value for VJOURNAL
// components per RFC 5545 §3.8.1.11. Values are uppercase per the
// RFC. See EventStatus for why the shared cancellation spelling does
// not collapse into a single constant.
type JournalStatus string

// RFC 5545 §3.8.1.11 STATUS values applicable to VJOURNAL.
const (
	JournalDraft     JournalStatus = "DRAFT"
	JournalFinal     JournalStatus = "FINAL"
	JournalCancelled JournalStatus = "CANCELLED" //nolint:misspell // RFC 5545 §3.8.1.11 wire string is "CANCELLED" (double L); not US "CANCELED".
)

// Class is the wire-string CLASS value per RFC 5545 §3.8.1.3,
// describing the access classification of a VEVENT, VTODO or
// VJOURNAL. Values are uppercase per the RFC.
//
// CLASS is optional; RFC 5545 §3.8.1.3 assigns PUBLIC when the
// property is absent. That default is applied by
// helpers.ClassOrDefault, not by the Class getter — see the helper
// docs for the rationale.
type Class string

// RFC 5545 §3.8.1.3 CLASS values. The RFC also admits IANA- and
// X-registered tokens; only the three enumerated values are
// modeled here.
const (
	ClassPublic       Class = "PUBLIC"
	ClassPrivate      Class = "PRIVATE"
	ClassConfidential Class = "CONFIDENTIAL"
)

// Transp is the wire-string TRANSP value per RFC 5545 §3.8.2.7,
// describing whether a VEVENT consumes free/busy time. TRANSP
// applies to VEVENT only. Values are uppercase per the RFC.
//
// RFC 5545 §3.8.2.7 assigns OPAQUE when the property is absent —
// applied by helpers.TranspOrDefault, not by the Transp getter.
type Transp string

// RFC 5545 §3.8.2.7 TRANSP values.
const (
	// TranspOpaque — the event blocks free/busy time.
	TranspOpaque Transp = "OPAQUE"
	// TranspTransparent — the event does not block free/busy time.
	TranspTransparent Transp = "TRANSPARENT"
)
