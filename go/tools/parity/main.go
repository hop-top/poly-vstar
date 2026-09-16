// SPDX-License-Identifier: MIT

// Command parity is the Go reference emitter for the cross-language
// parity harness.
//
// It takes the spec/ directory as its single argument, runs the V*
// public API over every fixture in spec/v1.0/conformance/ and
// spec/behavior/, and prints ONE JSON document to stdout. Each port
// (TypeScript, Python, Rust, PHP) ships an emitter making the same
// calls over the same fixtures; tools/parity/parity.py diffs each
// port's document against this one, key by key.
//
// Go is the reference. The normative description of the document
// this command prints — every key, every value shape, every
// ordering rule — is tools/parity/README.md. A port implements
// that document, not this source.
//
// Three rules shape every value:
//
//   - Nothing human-readable is emitted. Diagnostic messages, error
//     strings and prose reword between versions without the
//     behavior changing. Codes, paths, severities, op kinds and
//     failure-class tokens are the stable surface.
//   - A failure serializes as {"error": "<SentinelName>"} using the
//     Go sentinel's identifier as the cross-language token. A port
//     emits the same token however it words its own failure.
//   - Output is deterministic. Maps are rendered by encoding/json,
//     which sorts object keys; every slice is built in an order the
//     contract pins. Re-running the emitter is byte-identical.
//
// Usage:
//
//	go -C go run ./tools/parity ../spec
//
// Exits 0 having printed the document; exits 1 with a diagnostic on
// stderr when a fixture cannot be read or a contract the emitter
// depends on is broken.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// document is the emitted JSON's top level. Every key is always
// present — a port that implements one family still emits the other
// seven as empty objects, so a missing key is a structural fault
// rather than a coverage gap.
//
// The families mirror the two fixture trees: conformance and rrule
// come from spec/v1.0/conformance/, the remaining six from
// spec/behavior/.
type document struct {
	Conformance map[string]any `json:"conformance"`
	RRule       map[string]any `json:"rrule"`
	Validate    map[string]any `json:"validate"`
	Diff        map[string]any `json:"diff"`
	Supersede   map[string]any `json:"supersession"`
	Duration    map[string]any `json:"duration"`
	Ext         map[string]any `json:"ext"`
	Time        map[string]any `json:"time"`
}

// Fixture extensions and the failure-class token shared across
// families. classMalformed is the general class every layer may wrap,
// and so is the last entry of every sentinel table.
const (
	extICS   = ".ics"
	extVCF   = ".vcf"
	extRRule = ".rrule"

	classMalformed = "ErrMalformed"
)

// errorEntry is the universal failure shape: {"error": "<token>"}.
// The token is the Go sentinel's identifier (ErrMalformed,
// ErrMissingUID, ErrNoAnchor, …) used as a language-neutral name
// for the failure class.
type errorEntry struct {
	Error string `json:"error"`
}

func main() {
	if len(os.Args) != 2 {
		fail(fmt.Errorf("usage: parity <spec-dir>"))
	}
	spec, err := filepath.Abs(os.Args[1])
	if err != nil {
		fail(fmt.Errorf("resolve %s: %w", os.Args[1], err))
	}
	conformance := filepath.Join(spec, "v1.0", "conformance")
	behavior := filepath.Join(spec, "behavior")
	for _, dir := range []string{conformance, behavior} {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			fail(fmt.Errorf("not a directory: %s", dir))
		}
	}

	doc := document{}
	if doc.Conformance, err = emitConformance(conformance); err != nil {
		fail(err)
	}
	if doc.RRule, err = emitRRule(filepath.Join(conformance, "rrule")); err != nil {
		fail(err)
	}
	if doc.Validate, err = emitValidate(filepath.Join(behavior, "validate")); err != nil {
		fail(err)
	}
	if doc.Diff, err = emitDiff(filepath.Join(behavior, "diff")); err != nil {
		fail(err)
	}
	if doc.Supersede, err = emitSupersession(conformance, filepath.Join(behavior, "supersession")); err != nil {
		fail(err)
	}
	if doc.Duration, err = emitDuration(filepath.Join(behavior, "duration")); err != nil {
		fail(err)
	}
	if doc.Ext, err = emitExt(filepath.Join(behavior, "ext")); err != nil {
		fail(err)
	}
	if doc.Time, err = emitTime(conformance, filepath.Join(behavior, "time")); err != nil {
		fail(err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		fail(fmt.Errorf("encode document: %w", err))
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "parity: %v\n", err)
	os.Exit(1)
}
