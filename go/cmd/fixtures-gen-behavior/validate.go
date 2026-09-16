// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"path/filepath"
	"sort"

	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
	"hop.top/vstar/validate"
)

// diagnosticFixture is one entry of a <name>.diagnostics.json file:
// the stable surface of a validate.Diagnostic.
//
// Message is deliberately absent. It is human-readable prose that
// rewords between versions without the behavior changing; a port
// that pinned it would fail on an editing pass. Code, Severity and
// Path are the contract.
type diagnosticFixture struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
}

// validateCase is one generated validate fixture: a calendar and
// the code(s) it is authored to provoke. The expected diagnostics
// are not written by hand — they are whatever the reference emits,
// so the fixture cannot disagree with the implementation. `codes`
// is the authoring intent, asserted post-hoc so a case that stops
// exercising its code fails the generator rather than silently
// shrinking coverage.
type validateCase struct {
	name  string
	codes []string
	cal   vstar.Calendar
}

// stampTime is the fixed DTSTAMP every generated validate fixture
// uses. Fixed so re-running the generator is byte-identical.
const stampTime = "20260504T120000Z"

// writeValidateFamily writes one .ics + .diagnostics.json pair per
// case under dir and returns the file count.
//
// Coverage contract: every diagnostic code the validate package can
// emit gets at least one fixture, plus two clean documents whose
// sidecar is the empty list. checkValidateCoverage enforces the
// first half — a new VSnnn with no fixture fails generation.
func writeValidateFamily(dir string) (int, error) {
	cases := validateCases()
	if err := checkValidateCoverage(cases); err != nil {
		return 0, err
	}
	n := 0
	for _, tc := range cases {
		got := validate.Validate(tc.cal)
		if err := assertCodes(tc, got); err != nil {
			return 0, err
		}
		if err := writeCalendar(filepath.Join(dir, tc.name+".ics"), tc.cal); err != nil {
			return 0, err
		}
		if err := writeJSON(filepath.Join(dir, tc.name+".diagnostics.json"), diagnosticFixtures(got)); err != nil {
			return 0, err
		}
		n += 2
	}
	return n, nil
}

// diagnosticFixtures projects the reference's diagnostics onto the
// fixture shape, sorted by (path, code) so the file is stable
// regardless of the order the checks happen to run in. A port's
// table test sorts its own output the same way before comparing.
//
// Returns an empty (non-nil) slice for a clean document so the
// sidecar renders as `[]` rather than `null`: every port's JSON
// decoder handles the empty array, not all handle null as a list.
func diagnosticFixtures(ds []validate.Diagnostic) []diagnosticFixture {
	out := make([]diagnosticFixture, 0, len(ds))
	for _, d := range ds {
		out = append(out, diagnosticFixture{
			Code:     d.Code,
			Severity: d.Severity.String(),
			Path:     d.Path,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Code < out[j].Code
	})
	return out
}

// assertCodes verifies the case actually provoked the codes it was
// authored for. Without this a refactor could make a fixture stop
// exercising its rule and the sidecar would faithfully record the
// new (wrong) emptiness.
func assertCodes(tc validateCase, got []validate.Diagnostic) error {
	seen := map[string]bool{}
	for _, d := range got {
		seen[d.Code] = true
	}
	for _, want := range tc.codes {
		if !seen[want] {
			return fmt.Errorf("case %q: expected code %s, got %v", tc.name, want, codesOf(got))
		}
	}
	if len(tc.codes) == 0 && len(got) != 0 {
		return fmt.Errorf("case %q: expected a clean document, got %v", tc.name, codesOf(got))
	}
	return nil
}

func codesOf(ds []validate.Diagnostic) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Code)
	}
	return out
}

// allValidateCodes is every diagnostic code the validate package
// can emit, per docs/validate-codes.md. The list is duplicated here
// on purpose: it is the coverage contract this generator asserts
// against, and reading it out of the package would make the
// assertion vacuous.
var allValidateCodes = []string{
	validate.CodeMissingUID,               // VS001
	validate.CodeMissingDTSTAMP,           // VS002
	validate.CodeMissingXVSTARHash,        // VS003
	validate.CodeBadXVSTARHash,            // VS010
	validate.CodeUnknownProperty,          // VS020
	validate.CodeSupersessionMissingProps, // VS030
	validate.CodeSupersessionOrphan,       // VS031
	validate.CodeVTODOMissingDue,          // VS040
	validate.CodeVEVENTMissingDTSTART,     // VS041
	validate.CodeVFREEBUSYMissingTimes,    // VS042
	validate.CodeVCARDMissingRequired,     // VS043
	validate.CodeStatusNotInVocabulary,    // VS044
	validate.CodeRRuleUnsupported,         // VS050
	validate.CodeRRuleMalformed,           // VS051
	validate.CodeMalformedDuration,        // VS052
}

// checkValidateCoverage fails generation when a code in
// allValidateCodes has no fixture authored for it.
func checkValidateCoverage(cases []validateCase) error {
	covered := map[string]bool{}
	for _, tc := range cases {
		for _, c := range tc.codes {
			covered[c] = true
		}
	}
	var missing []string
	for _, c := range allValidateCodes {
		if !covered[c] {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("no validate fixture covers %v", missing)
	}
	return nil
}

// validateCases enumerates the generated documents. Each is the
// smallest calendar that provokes its code, so a port debugging a
// failure reads a handful of lines rather than a realistic
// document.
func validateCases() []validateCase {
	return []validateCase{
		{
			name:  "missing_uid",
			codes: []string{validate.CodeMissingUID},
			cal:   oneComponent("Validate-MissingUID", stamped(bare(vstar.CompJournal))),
		},
		{
			// Two UID-less components of one type, so the
			// positional path segment has to distinguish them:
			// the sidecar pins [#0] and [#1], not [#0] twice.
			//
			// Every other UID-less fixture carries exactly one
			// such component, which leaves the counter frozen at
			// [#0] and a port that never increments it passing
			// the whole gate. Three port lanes found that with
			// mutation testing and each pinned it with a local
			// unit test; this fixture covers every future port
			// without one.
			//
			// VTODO rather than VJOURNAL because a VTODO needs
			// DUE to be otherwise clean, which keeps the two
			// components distinguishable in the .ics — identical
			// components would round-trip but read as a
			// duplication bug.
			name:  "missing_uid_positional",
			codes: []string{validate.CodeMissingUID},
			cal: func() vstar.Calendar {
				cal := vstar.Calendar{ProdID: "-//V*//Validate-MissingUIDPositional//EN"}
				for _, due := range []string{"20260601T090000Z", "20260602T090000Z"} {
					cal.Append(hashed(withProps(
						bare(vstar.CompTodo),
						prop("DTSTAMP", stampTime),
						prop("DUE", due),
					)))
				}
				return cal
			}(),
		},
		{
			name:  "missing_dtstamp",
			codes: []string{validate.CodeMissingDTSTAMP},
			cal: oneComponent("Validate-MissingDTSTAMP", hashed(
				withProps(bare(vstar.CompJournal), prop("UID", "journal-no-dtstamp")),
			)),
		},
		{
			name:  "missing_hash",
			codes: []string{validate.CodeMissingXVSTARHash},
			cal: oneComponent("Validate-MissingHash", withProps(
				bare(vstar.CompJournal),
				prop("UID", "journal-no-hash"),
				prop("DTSTAMP", stampTime),
			)),
		},
		{
			name:  "bad_hash",
			codes: []string{validate.CodeBadXVSTARHash},
			cal: oneComponent("Validate-BadHash", mutateAfterHash(
				journal("journal-bad-hash"),
				prop("SUMMARY", "edited after the hash was stamped"),
			)),
		},
		{
			name:  "unknown_property",
			codes: []string{validate.CodeUnknownProperty},
			cal: oneComponent("Validate-UnknownProperty", hashed(withProps(
				journalBody("journal-unknown-prop"),
				prop("NOTAREALPROPERTY", "no X- prefix, not on the standard list"),
			))),
		},
		{
			name:  "supersession_missing_props",
			codes: []string{validate.CodeSupersessionMissingProps},
			cal: oneComponent("Validate-SupersessionMissingProps", hashed(withProps(
				journalBody("journal-supersession-incomplete"),
				prop("CATEGORIES", "status-supersession"),
			))),
		},
		{
			name:  "supersession_orphan",
			codes: []string{validate.CodeSupersessionOrphan},
			cal: oneComponent("Validate-SupersessionOrphan", hashed(withProps(
				journalBody("journal-supersession-orphan"),
				prop("CATEGORIES", "status-supersession"),
				prop("RELATED-TO", "todo-that-does-not-exist"),
				prop("X-VSTAR-EFFECTIVE-STATUS", "COMPLETED"),
			))),
		},
		{
			name:  "vtodo_missing_due",
			codes: []string{validate.CodeVTODOMissingDue},
			cal: oneComponent("Validate-VTODOMissingDue", hashed(withProps(
				bare(vstar.CompTodo),
				prop("UID", "todo-no-due"),
				prop("DTSTAMP", stampTime),
			))),
		},
		{
			name:  "vevent_missing_dtstart",
			codes: []string{validate.CodeVEVENTMissingDTSTART},
			cal: oneComponent("Validate-VEVENTMissingDTSTART", hashed(withProps(
				bare(vstar.CompEvent),
				prop("UID", "event-no-dtstart"),
				prop("DTSTAMP", stampTime),
			))),
		},
		{
			name:  "vfreebusy_missing_times",
			codes: []string{validate.CodeVFREEBUSYMissingTimes},
			cal: oneComponent("Validate-VFREEBUSYMissingTimes", hashed(withProps(
				bare(vstar.CompFreeBusy),
				prop("UID", "freebusy-no-times"),
				prop("DTSTAMP", stampTime),
			))),
		},
		{
			name:  "vcard_missing_required",
			codes: []string{validate.CodeVCARDMissingRequired},
			cal: oneComponent("Validate-VCARDMissingRequired", hashed(withProps(
				bare("VCARD"),
				prop("UID", "card-no-version"),
				prop("DTSTAMP", stampTime),
			))),
		},
		{
			name:  "status_not_in_vocabulary",
			codes: []string{validate.CodeStatusNotInVocabulary},
			cal: oneComponent("Validate-StatusNotInVocabulary", hashed(withProps(
				bare(vstar.CompEvent),
				prop("UID", "event-draft-status"),
				prop("DTSTAMP", stampTime),
				prop("DTSTART", "20260601T090000Z"),
				// DRAFT belongs to the VJOURNAL vocabulary, not
				// VEVENT's: legal iCalendar text, wrong component.
				prop("STATUS", "DRAFT"),
			))),
		},
		{
			name:  "rrule_unsupported",
			codes: []string{validate.CodeRRuleUnsupported},
			cal: oneComponent("Validate-RRuleUnsupported", hashed(withProps(
				bare(vstar.CompEvent),
				prop("UID", "event-secondly"),
				prop("DTSTAMP", stampTime),
				prop("DTSTART", "20260601T090000Z"),
				prop("RRULE", "FREQ=SECONDLY"),
			))),
		},
		{
			name:  "rrule_malformed",
			codes: []string{validate.CodeRRuleMalformed},
			cal: oneComponent("Validate-RRuleMalformed", hashed(withProps(
				bare(vstar.CompEvent),
				prop("UID", "event-bad-rrule"),
				prop("DTSTAMP", stampTime),
				prop("DTSTART", "20260601T090000Z"),
				prop("RRULE", "FREQ=FORTNIGHTLY;COUNT=3"),
			))),
		},
		{
			name:  "malformed_duration",
			codes: []string{validate.CodeMalformedDuration},
			cal: oneComponent("Validate-MalformedDuration", hashed(withProps(
				bare(vstar.CompEvent),
				prop("UID", "event-bad-duration"),
				prop("DTSTAMP", stampTime),
				prop("DTSTART", "20260601T090000Z"),
				prop("DURATION", "P1Y"),
			))),
		},
		{
			name:  "clean_vevent",
			codes: nil,
			cal: oneComponent("Validate-CleanVEVENT", hashed(withProps(
				bare(vstar.CompEvent),
				prop("UID", "event-clean"),
				prop("DTSTAMP", stampTime),
				prop("DTSTART", "20260601T090000Z"),
				prop("DTEND", "20260601T100000Z"),
				prop("SUMMARY", "Design review"),
				prop("STATUS", "CONFIRMED"),
			))),
		},
		{
			name:  "clean_vtodo_completed",
			codes: nil,
			// The RFC 5545 §3.6.2 escape hatch: a completed VTODO
			// may drop DUE so long as COMPLETED records when it
			// finished. Pairs with missing_due as the positive case.
			cal: oneComponent("Validate-CleanVTODOCompleted", hashed(withProps(
				bare(vstar.CompTodo),
				prop("UID", "todo-clean-completed"),
				prop("DTSTAMP", stampTime),
				prop("STATUS", "COMPLETED"),
				prop("COMPLETED", "20260504T113000Z"),
				prop("SUMMARY", "Ship the fixtures"),
			))),
		},
	}
}

// --- calendar construction helpers -------------------------------

func prop(name, value string) vstar.Property {
	return vstar.Property{Name: name, Value: value}
}

func bare(t vstar.CompType) vstar.Component {
	return vstar.Component{Type: t}
}

// withProps appends props to c in order. Set (not Add) so a
// repeated name replaces rather than duplicates.
func withProps(c vstar.Component, props ...vstar.Property) vstar.Component {
	for _, p := range props {
		c.Set(p)
	}
	return c
}

// stamped gives c the DTSTAMP + hash a component needs to be clean
// apart from whatever the case is testing.
func stamped(c vstar.Component) vstar.Component {
	return hashed(withProps(c, prop("DTSTAMP", stampTime)))
}

// hashed stamps X-VSTAR-HASH over c's current content. Must be the
// last mutation before a component is considered final — that is
// the discipline VS010 exists to police.
func hashed(c vstar.Component) vstar.Component {
	hashing.SetXVSTAR(&c)
	return c
}

// journalBody is a VJOURNAL with the common required triple minus
// the hash, ready for further properties then hashed().
func journalBody(uid string) vstar.Component {
	return withProps(
		bare(vstar.CompJournal),
		prop("UID", uid),
		prop("DTSTAMP", stampTime),
	)
}

// journal is a fully-stamped, clean VJOURNAL.
func journal(uid string) vstar.Component {
	return hashed(journalBody(uid))
}

// mutateAfterHash applies props to an already-hashed component
// without re-stamping, so its X-VSTAR-HASH no longer matches its
// content. This is the VS010 provocation.
func mutateAfterHash(c vstar.Component, props ...vstar.Property) vstar.Component {
	return withProps(c, props...)
}

// oneComponent wraps a single component in a calendar with the
// given PRODID slug.
func oneComponent(slug string, c vstar.Component) vstar.Calendar {
	cal := vstar.Calendar{ProdID: "-//V*//" + slug + "//EN"}
	cal.Append(c)
	return cal
}
