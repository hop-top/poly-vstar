// SPDX-License-Identifier: MIT

// Command fixtures-gen-supersession generates the five supersession
// edge-case .ics fixtures under the conformance corpus'
// supersession/ directory — spec/v0.1/conformance/supersession/ in
// the poly-vstar monorepo, testdata/supersession/ on the
// hop-top/vstar mirror (see corpusRoot). The `.canonical` and `.hash`
// regeneration, and the mirroring of the corpus into the module's
// testdata/, are delegated to fixtures-verify; the `.notes.md` and
// README files are written by hand alongside the .ics and are NOT
// regenerated here.
//
// All times are fixed so re-running this generator produces
// byte-identical output. UIDs are spec/02-style human-readable
// stems for diff legibility.
//
// Outputs (each is a single VCALENDAR with UTF-8 / LF on disk):
//
//   - linear.ics          — original VTODO + one supersession VJOURNAL
//     flipping it to COMPLETED.
//   - multi_step.ics      — original VTODO + two supersession VJOURNALs
//     walking IN-PROCESS → COMPLETED.
//   - cross_component.ics — VTODO superseded by a VJOURNAL whose
//     RELATED-TO targets it; both live as
//     siblings in the same VCALENDAR (V*'s
//     "ledger is one container" model).
//   - corrupt_mutated.ics — original VTODO with X-VSTAR-HASH set,
//     then SUMMARY mutated post-stamp. The
//     hash no longer matches; consumers MUST
//     notice. NEGATIVE TEST FIXTURE.
//   - effective_status_vevent.ics — turn VEVENT + one supersession
//     VJOURNAL flipping it to the cancellation
//     status. The effective status is drawn
//     from the VEVENT vocabulary because the
//     superseded component is a VEVENT
//     (spec/02 "Status supersession").
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/hashing"
	"hop.top/vstar/helpers"
	"hop.top/vstar/supersession"
)

// propSummary is the SUMMARY property name every generated
// component carries for diff legibility.
const propSummary = "SUMMARY"

func main() {
	root, err := moduleRoot()
	if err != nil {
		fail(err)
	}
	out := filepath.Join(corpusRoot(root), "supersession")
	if err := os.MkdirAll(out, 0o755); err != nil {
		fail(fmt.Errorf("mkdir %s: %w", out, err))
	}

	// Fixed times — every run yields identical bytes.
	t0 := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 5, 4, 14, 30, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 4, 16, 45, 0, 0, time.UTC)

	if err := writeLinear(out, t0, t1); err != nil {
		fail(err)
	}
	if err := writeMultiStep(out, t0, t1, t2); err != nil {
		fail(err)
	}
	if err := writeCrossComponent(out, t0, t1); err != nil {
		fail(err)
	}
	if err := writeCorruptMutated(out, t0); err != nil {
		fail(err)
	}
	if err := writeEffectiveStatusVEvent(out, t0, t1); err != nil {
		fail(err)
	}
	fmt.Println("fixtures-gen-supersession: wrote 5 .ics fixtures under", out)
}

// writeLinear: original VTODO at t0 + one supersession VJOURNAL at t1
// flipping it to COMPLETED.
func writeLinear(dir string, t0, t1 time.Time) error {
	target := newTodo("todo-1", "Buy milk", t0)
	hashing.SetXVSTAR(&target)
	sup, err := supersession.Supersedes(target, string(vstar.TodoCompleted), t1)
	if err != nil {
		return fmt.Errorf("linear: %w", err)
	}
	cal := vstar.Calendar{ProdID: "-//V*//Supersession-Linear//EN"}
	cal.Append(target)
	cal.Append(sup)
	return writeCalendar(dir, "linear", cal)
}

// writeMultiStep: original VTODO + two supersession VJOURNALs
// walking IN-PROCESS → COMPLETED.
func writeMultiStep(dir string, t0, t1, t2 time.Time) error {
	target := newTodo("todo-multi", "Ship Wave 5", t0)
	hashing.SetXVSTAR(&target)
	sup1, err := supersession.Supersedes(target, string(vstar.TodoInProcess), t1)
	if err != nil {
		return fmt.Errorf("multi_step sup1: %w", err)
	}
	sup2, err := supersession.Supersedes(target, string(vstar.TodoCompleted), t2)
	if err != nil {
		return fmt.Errorf("multi_step sup2: %w", err)
	}
	cal := vstar.Calendar{ProdID: "-//V*//Supersession-MultiStep//EN"}
	cal.Append(target)
	cal.Append(sup1)
	cal.Append(sup2)
	return writeCalendar(dir, "multi_step", cal)
}

// writeCrossComponent: VTODO superseded by a VJOURNAL whose
// RELATED-TO targets it; both as siblings in one VCALENDAR. The
// supersession journal carries an extra DESCRIPTION to flavor the
// "real" cross-component reference.
func writeCrossComponent(dir string, t0, t1 time.Time) error {
	target := newTodo("turn-42", "Resolve branch conflict", t0)
	hashing.SetXVSTAR(&target)

	sup, err := supersession.Supersedes(target, string(vstar.TodoCompleted), t1)
	if err != nil {
		return fmt.Errorf("cross_component sup: %w", err)
	}
	sup.Set(vstar.Property{Name: "DESCRIPTION", Value: "Resolved by Sami's review"})
	// Re-stamp the hash since we mutated the journal after Supersedes.
	hashing.SetXVSTAR(&sup)

	cal := vstar.Calendar{ProdID: "-//V*//Supersession-CrossComponent//EN"}
	cal.Append(target)
	cal.Append(sup)
	return writeCalendar(dir, "cross_component", cal)
}

// writeCorruptMutated: VTODO with X-VSTAR-HASH set, then SUMMARY
// mutated inline so the hash no longer matches. INTENTIONALLY
// CORRUPT — negative test fodder.
func writeCorruptMutated(dir string, t0 time.Time) error {
	target := newTodo("todo-corrupt", "Original summary", t0)
	hashing.SetXVSTAR(&target)
	// Hash now stamped. Mutate SUMMARY without re-stamping → the
	// stored hash no longer matches the canonical form.
	target.Set(vstar.Property{Name: propSummary, Value: "MUTATED summary (hash NOT refreshed)"})
	cal := vstar.Calendar{ProdID: "-//V*//Supersession-Corrupt//EN"}
	cal.Append(target)
	return writeCalendar(dir, "corrupt_mutated", cal)
}

// writeEffectiveStatusVEvent: turn VEVENT at t0 + one supersession
// VJOURNAL at t1 flipping it to EventCancelled. spec/02 scopes
// X-VSTAR-EFFECTIVE-STATUS to the STATUS vocabulary of the
// superseded component's type, so the value comes from EventStatus
// even though the carrier is a VJOURNAL.
func writeEffectiveStatusVEvent(dir string, t0, t1 time.Time) error {
	target := newEvent("turn-7", "Pair on the parser", t0)
	hashing.SetXVSTAR(&target)
	sup, err := supersession.Supersedes(target, string(vstar.EventCancelled), t1)
	if err != nil {
		return fmt.Errorf("effective_status_vevent: %w", err)
	}
	cal := vstar.Calendar{ProdID: "-//V*//Supersession-EffectiveStatusVEvent//EN"}
	cal.Append(target)
	cal.Append(sup)
	return writeCalendar(dir, "effective_status_vevent", cal)
}

// newTodo builds a deterministic VTODO with UID/DTSTAMP/SUMMARY.
// Uses helpers.NewTodo for the constructor discipline (hash
// stamped at end), then strips DUE since this generator only
// wants UID + DTSTAMP + SUMMARY for the supersession story.
func newTodo(uid, summary string, t time.Time) vstar.Component {
	c, err := helpers.NewTodo(uid, t.Add(24*time.Hour))
	if err != nil {
		panic(fmt.Sprintf("newTodo(%s): %v", uid, err))
	}
	c.Remove("DUE")
	// Override DTSTAMP (helpers.NewTodo uses time.Now); we need
	// determinism.
	c.Set(vstar.Property{Name: "DTSTAMP", Value: vstar.FormatTime(t)})
	c.Set(vstar.Property{Name: propSummary, Value: summary})
	return c
}

// newEvent builds a deterministic VEVENT with UID/DTSTAMP/DTSTART/
// DTEND/SUMMARY: a one-hour turn the day after t. Uses
// helpers.NewEvent for the constructor discipline, then overrides
// DTSTAMP (helpers.NewEvent uses time.Now) for determinism.
func newEvent(uid, summary string, t time.Time) vstar.Component {
	c, err := helpers.NewEvent(uid, t.Add(24*time.Hour), t.Add(25*time.Hour))
	if err != nil {
		panic(fmt.Sprintf("newEvent(%s): %v", uid, err))
	}
	c.Set(vstar.Property{Name: "DTSTAMP", Value: vstar.FormatTime(t)})
	c.Set(vstar.Property{Name: propSummary, Value: summary})
	return c
}

// writeCalendar encodes cal via the Go RFC 5545 codec, normalizes
// CRLF→LF for on-disk LF convention, then writes <stem>.ics under
// dir. The generator does NOT touch <stem>.notes.md or README.md;
// those are hand-authored to capture intent.
func writeCalendar(dir, stem string, cal vstar.Calendar) error {
	var buf bytes.Buffer
	if err := rfc5545.Encode(&buf, cal); err != nil {
		return fmt.Errorf("encode %s: %w", stem, err)
	}
	icsLF := crlfToLF(buf.Bytes())
	return os.WriteFile(filepath.Join(dir, stem+".ics"), icsLF, 0o644)
}

// crlfToLF strips \r before \n. Same convention as fixtures-verify.
func crlfToLF(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		if b[i] == '\r' && i+1 < len(b) && b[i+1] == '\n' {
			continue
		}
		out = append(out, b[i])
	}
	return out
}

// moduleRoot finds the Go module root by walking up from the current
// working directory looking for testdata/rfc5545; see fixtures-verify.
func moduleRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		st, err := os.Stat(filepath.Join(dir, "testdata", "rfc5545"))
		if err == nil && st.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find testdata/ above %s", cwd)
		}
		dir = parent
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fixtures-gen-supersession:", err)
	os.Exit(1)
}
