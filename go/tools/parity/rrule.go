// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/rrule"
)

// rruleClasses is the failure vocabulary the rrule family reports,
// most specific first. Every token is a class named by spec/05
// §Failure classes; ErrMalformed is the general case and so last.
var rruleClasses = []sentinel{
	{"ErrUnsupportedRRule", rrule.ErrUnsupportedRRule},
	{"ErrIterationCap", rrule.ErrIterationCap},
	{"ErrUnboundedExpansion", rrule.ErrUnboundedExpansion},
	{classMalformed, vstar.ErrMalformed},
}

// sidecarSpec is the union of every rrule sidecar's input fields,
// read from the committed JSON. The emitter reads inputs from the
// sidecar and reports outputs from the reference — it never copies
// a sidecar's expected values into the document, which would make
// the harness compare fixtures to themselves.
type sidecarSpec struct {
	DTStart string `json:"dtstart"`
	After   string `json:"after"`
	Start   string `json:"start"`
	End     string `json:"end"`
	Limit   int    `json:"limit"`
	// Expected is read only to learn how many NextOccurrence steps
	// the .next.json sidecar walks; its values are never emitted.
	Expected []string `json:"expected"`
}

// emitRRule walks the rrule corpus and emits one object per fixture
// stem, keyed by the stem's path relative to the conformance root
// ("rrule/happy/freq_daily").
//
// The object carries one key per sidecar the fixture has, so its
// shape states which contracts the fixture pins. A .rrule fixture
// with no sidecars at all pins only that the rule parses, and emits
// {"parsed": true}; a rejected one emits {"error": "<class>"}.
func emitRRule(root string) (map[string]any, error) {
	corpus := filepath.Dir(root)
	out := map[string]any{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		var entry map[string]any
		var ext string
		switch {
		case strings.HasSuffix(path, extRRule):
			ext = extRRule
			entry, err = ruleEntry(path)
		case strings.HasSuffix(path, extICS):
			ext = extICS
			entry, err = setEntry(path)
		default:
			return nil
		}
		if err != nil {
			return err
		}
		return record(out, corpus, strings.TrimSuffix(path, ext), entry)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ruleEntry evaluates one <stem>.rrule fixture against every
// sidecar it has.
//
// A .expect.json sidecar means the rule must be rejected: the
// emitter reports the class ValidateRRule produced and stops, since
// no other contract applies to a rule that does not parse. Without
// it the rule must parse, and the formatter and evaluator sidecars
// each add their key.
func ruleEntry(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	value := strings.TrimRight(string(body), "\r\n")
	stem := strings.TrimSuffix(path, extRRule)

	if has, err := hasSidecar(stem + ".expect.json"); err != nil {
		return nil, err
	} else if has {
		token, err := failureToken(path, rrule.ValidateRRule(value))
		if err != nil {
			return nil, err
		}
		return map[string]any{"error": token}, nil
	}

	rule, err := rrule.ParseRRule(value)
	if err != nil {
		return nil, fmt.Errorf("%s: ParseRRule failed: %w", path, err)
	}
	entry := map[string]any{"parsed": true}
	for _, add := range []func(string, rrule.Rule, map[string]any) error{
		addFormatted, addNext, addExpand, addBetween,
	} {
		if err := add(stem, rule, entry); err != nil {
			return nil, err
		}
	}
	return entry, nil
}

// addFormatted reports Rule.String() when a <stem>.formatted
// sidecar pins the wire form.
func addFormatted(stem string, rule rrule.Rule, entry map[string]any) error {
	has, err := hasSidecar(stem + ".formatted")
	if err != nil || !has {
		return err
	}
	entry["formatted"] = rule.String()
	return nil
}

// addNext walks NextOccurrence the way the sidecar's expected list
// is shaped: one step per entry, each feeding its result back as
// the next `after`. The emitted list is what the reference yielded.
//
// One extra step runs past the end, and its outcome is the
// terminal contract: "next_error" names the class the series
// failed with, or "next_complete" states whether it terminated.
// Emitting that step unconditionally means a port cannot pass by
// stopping early — a series that should terminate and one that
// should raise ErrIterationCap differ in the document.
func addNext(stem string, rule rrule.Rule, entry map[string]any) error {
	path := stem + ".next.json"
	spec, has, err := readSidecar(path)
	if err != nil || !has {
		return err
	}
	dt, err := parseStamp(path, "dtstart", spec.DTStart)
	if err != nil {
		return err
	}
	after, err := parseStamp(path, "after", spec.After)
	if err != nil {
		return err
	}
	stamps := []string{}
	for i := range spec.Expected {
		got, ok, err := rrule.NextOccurrence(rule, dt, after)
		if err != nil {
			return fmt.Errorf("%s: NextOccurrence step %d failed: %w", path, i, err)
		}
		if !ok {
			return fmt.Errorf("%s: NextOccurrence step %d terminated early", path, i)
		}
		stamps = append(stamps, vstar.FormatTime(got))
		after = got
	}
	entry["next"] = stamps

	_, ok, err := rrule.NextOccurrence(rule, dt, after)
	switch {
	case err != nil:
		token, cerr := failureToken(path, err)
		if cerr != nil {
			return cerr
		}
		entry["next_error"] = token
	case !ok:
		entry["next_complete"] = true
	default:
		entry["next_complete"] = false
	}
	return nil
}

// addExpand reports Occurrences over the sidecar's limit: the
// occurrence list and the complete flag, or the failure class.
func addExpand(stem string, rule rrule.Rule, entry map[string]any) error {
	path := stem + ".expand.json"
	spec, has, err := readSidecar(path)
	if err != nil || !has {
		return err
	}
	dt, err := parseStamp(path, "dtstart", spec.DTStart)
	if err != nil {
		return err
	}
	got, complete, err := rrule.Occurrences(rule, dt, spec.Limit)
	return addBounded(entry, path, "expand", got, complete, err)
}

// addBetween reports Between over the sidecar's half-open window.
// Between has no completeness notion — the window bounds the
// answer — so the success shape is the list alone.
func addBetween(stem string, rule rrule.Rule, entry map[string]any) error {
	path := stem + ".between.json"
	spec, has, err := readSidecar(path)
	if err != nil || !has {
		return err
	}
	dt, err := parseStamp(path, "dtstart", spec.DTStart)
	if err != nil {
		return err
	}
	start, err := parseStamp(path, "start", spec.Start)
	if err != nil {
		return err
	}
	end, err := parseStamp(path, "end", spec.End)
	if err != nil {
		return err
	}
	got, err := rrule.Between(rule, dt, start, end)
	if err != nil {
		token, cerr := failureToken(path, err)
		if cerr != nil {
			return cerr
		}
		entry["between_error"] = token
		return nil
	}
	entry["between"] = formatStamps(got)
	return nil
}

// setEntry evaluates one <stem>.ics recurrence-set fixture: the
// calendar's first component goes through SetFromComponent, then
// either .expect.json pins a rejection or .occurrences.json pins
// the bounded expansion.
func setEntry(path string) (map[string]any, error) {
	stem := strings.TrimSuffix(path, extICS)
	input, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	set, setErr := setFromCalendar(input)

	if has, err := hasSidecar(stem + ".expect.json"); err != nil {
		return nil, err
	} else if has {
		token, err := failureToken(path, setErr)
		if err != nil {
			return nil, err
		}
		return map[string]any{"error": token}, nil
	}
	if setErr != nil {
		return nil, fmt.Errorf("%s: %w", path, setErr)
	}

	entry := map[string]any{"parsed": true}
	occPath := stem + ".occurrences.json"
	spec, has, err := readSidecar(occPath)
	if err != nil || !has {
		return entry, err
	}
	got, complete, err := set.Occurrences(spec.Limit)
	if err := addBounded(entry, occPath, "occurrences", got, complete, err); err != nil {
		return nil, err
	}
	return entry, nil
}

// setFromCalendar parses an .ics fixture and builds a Set from its
// first component, mirroring the reference verifier's entry point.
func setFromCalendar(input []byte) (rrule.Set, error) {
	cal, err := rfc5545.Parse(bytes.NewReader(input))
	if err != nil {
		return rrule.Set{}, err
	}
	if len(cal.Components) == 0 {
		return rrule.Set{}, fmt.Errorf("%w: calendar has no components", vstar.ErrMalformed)
	}
	return rrule.SetFromComponent(cal.Components[0])
}

// addBounded records a bounded-expansion outcome on entry: the
// occurrence list under `key` plus a "<key>_complete" flag on
// success, or "<key>_error" naming the failure class.
//
// Every key is flat on the entry rather than nested under one
// sub-object, so which contracts a fixture pins is readable off the
// entry's key set and a port builds a flat record per fixture.
func addBounded(entry map[string]any, path, key string, got []time.Time, complete bool, err error) error {
	if err != nil {
		token, cerr := failureToken(path, err)
		if cerr != nil {
			return cerr
		}
		entry[key+"_error"] = token
		return nil
	}
	entry[key] = formatStamps(got)
	entry[key+"_complete"] = complete
	return nil
}

// failureToken classifies a failure into its rrule class token,
// refusing to emit anything for a nil or unrecognized error. An
// unrecognized class must fail the emitter rather than travel into
// the document as a token no port implements.
func failureToken(path string, err error) (string, error) {
	if err == nil {
		return "", fmt.Errorf("%s: expected a failure, the call succeeded", path)
	}
	token, ok := classify(err, rruleClasses)
	if !ok {
		return "", fmt.Errorf("%s: failure matches no known class: %w", path, err)
	}
	return token, nil
}

// formatStamps renders occurrences as RFC 5545 form #2, the one
// timestamp shape every family and every port uses. Always a
// non-nil slice so an empty result renders as [] rather than null.
func formatStamps(ts []time.Time) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, vstar.FormatTime(t))
	}
	return out
}

// parseStamp reads an RFC 5545 form #2 sidecar input value.
func parseStamp(path, field, value string) (time.Time, error) {
	t, ok := vstar.ParseTime(value)
	if !ok {
		return time.Time{}, fmt.Errorf("%s: bad %s %q", path, field, value)
	}
	return t, nil
}

// readSidecar decodes a sidecar's input fields, reporting has=false
// when it does not exist.
func readSidecar(path string) (sidecarSpec, bool, error) {
	var spec sidecarSpec
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return spec, false, nil
	}
	if err != nil {
		return spec, false, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return spec, false, fmt.Errorf("parse %s: %w", path, err)
	}
	return spec, true, nil
}

// hasSidecar reports whether a sidecar file exists.
func hasSidecar(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	return true, nil
}
