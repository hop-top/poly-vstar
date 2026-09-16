// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
)

// specDirName is the spec tree's directory, relative to the
// repository root. The behavior fixtures live under
// <spec>/behavior/ — a sibling of <spec>/v1.0/conformance/ rather
// than a member of it, because they are version-independent
// behavior tables rather than wire-format conformance inputs.
const specDirName = "spec"

// behaviorRoot resolves the behavior-fixture tree for a module
// rooted at module: <module>/../spec/behavior in the poly-vstar
// monorepo, where the spec tree is the module's sibling.
//
// Returns ("", false) when there is no sibling spec/ tree. That is
// the hop-top/vstar mirror, where the module is a subtree of go/
// alone: the mirror ships no spec/, so behavior generation is
// skipped entirely rather than inventing a destination. Same guard
// pattern as fixtures-verify's corpusRoot and
// validate/codes_doc_test.go.
func behaviorRoot(module string) (string, bool) {
	spec := filepath.Join(filepath.Dir(module), specDirName)
	if st, err := os.Stat(spec); err != nil || !st.IsDir() {
		return "", false
	}
	return filepath.Join(spec, "behavior"), true
}

// writeJSON marshals v as indented JSON with a trailing newline and
// writes it to path, creating parent directories as needed.
//
// Indented rather than compact so a drifting fixture produces a
// readable line-level git diff: these files are review surface, not
// just machine input. HTML escaping is off — the fixtures carry
// literal `<`, `>` and `&` in property values and a port reading
// them expects the bytes it sees.
func writeJSON(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	return writeFile(path, buf.Bytes())
}

// rewritten collects the fixture paths this run had to add, change
// or delete, relative to the behavior root.
//
// The caller needs that list because rewriting a fixture destroys
// the evidence of a hand-edit: restoring a tampered file leaves the
// working tree clean, so `git diff` alone cannot see that someone
// edited a generated fixture instead of the generator. Reporting
// the paths keeps that edit visible — the same reason
// fixtures-verify's mirrorCorpus returns its changed paths.
var rewritten []string

// noteRewritten records path as differing from what was committed.
func noteRewritten(root, path string) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rewritten = append(rewritten, filepath.ToSlash(rel))
}

// behaviorRootForReporting is the root that noteRewritten renders
// paths relative to. Set once by Generate.
var behaviorRootForReporting string

// writeFile writes b to path with the corpus' 0o644 convention,
// creating parent directories as needed, and records the path when
// the bytes differ from what is already there (a missing file
// counts as a difference).
func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	old, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(old, b) {
		noteRewritten(behaviorRootForReporting, path)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// writeCalendar encodes cal through the RFC 5545 codec and writes
// it to path with the corpus' on-disk LF convention (the wire form
// is CRLF; readers expand back at comparison time).
func writeCalendar(path string, cal vstar.Calendar) error {
	var buf bytes.Buffer
	if err := rfc5545.Encode(&buf, cal); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return writeFile(path, crlfToLF(buf.Bytes()))
}

// readCalendar parses an .ics file from disk.
func readCalendar(path string) (vstar.Calendar, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return vstar.Calendar{}, fmt.Errorf("read %s: %w", path, err)
	}
	cal, err := rfc5545.Parse(bytes.NewReader(raw))
	if err != nil {
		return vstar.Calendar{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return cal, nil
}

// crlfToLF strips \r before \n. Same convention as fixtures-verify
// and fixtures-gen-supersession.
func crlfToLF(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := range len(b) {
		if b[i] == '\r' && i+1 < len(b) && b[i+1] == '\n' {
			continue
		}
		out = append(out, b[i])
	}
	return out
}

// clearDir removes every entry of dir except the names listed in
// keep, then ensures dir exists. Delete-then-write so a fixture
// dropped from a generator disappears from the tree instead of
// lingering as a stale file the ports would still consume.
//
// A removed file that the run does not write back is a difference
// like any other, so it is recorded too. Files the run rewrites
// immediately afterwards are reconciled by settleRewritten, which
// drops any path whose bytes are unchanged end-to-end.
func clearDir(dir string, keep map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dir, 0o755)
		}
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, e := range entries {
		if keep[e.Name()] {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			return fmt.Errorf("unexpected directory in a fixture family: %s", full)
		}
		before, readErr := os.ReadFile(full)
		if err := os.RemoveAll(full); err != nil {
			return fmt.Errorf("remove %s: %w", full, err)
		}
		if readErr == nil {
			removedBytes[full] = before
		}
	}
	return nil
}

// removedBytes holds the content of every file clearDir deleted, so
// settleRewritten can tell a genuine deletion from a file the run
// rewrote with identical bytes.
var removedBytes = map[string][]byte{}

// settleRewritten reconciles the deletions clearDir recorded against
// what the run actually wrote, and returns the paths that really
// differ from what was committed: files whose bytes changed, files
// that are new, and files that are gone.
//
// Called once, after every family has been written.
func settleRewritten(root string) []string {
	changed := map[string]bool{}
	for _, rel := range rewritten {
		changed[rel] = true
	}
	for full, before := range removedBytes {
		rel, err := filepath.Rel(root, full)
		if err != nil {
			rel = full
		}
		rel = filepath.ToSlash(rel)
		after, err := os.ReadFile(full)
		switch {
		case err != nil:
			// Deleted and not written back.
			changed[rel] = true
		case bytes.Equal(before, after):
			// Rewritten identically: not a difference.
			delete(changed, rel)
		default:
			changed[rel] = true
		}
	}
	out := make([]string, 0, len(changed))
	for rel := range changed {
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}
