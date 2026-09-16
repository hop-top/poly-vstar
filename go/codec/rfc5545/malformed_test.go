// SPDX-License-Identifier: MIT

package rfc5545_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
)

// malformedSentinels maps the sentinel name encoded in
// testdata/malformed/<name>.error to the actual vstar package
// variable. Tests look up here so the .error file's text stays
// the source of truth.
var malformedSentinelsRFC5545 = map[string]error{
	"ErrMalformed":          vstar.ErrMalformed,
	"ErrUnclosedBlock":      vstar.ErrUnclosedBlock,
	"ErrUnsupportedVersion": vstar.ErrUnsupportedVersion,
	"ErrMissingUID":         vstar.ErrMissingUID,
}

// TestMalformed_RFC5545 walks testdata/malformed/, picks every
// `*.ics` (filtering out non-calendar files because the dir mixes
// formats), parses it, and asserts the parser returns the sentinel
// named in the matching `<stem>.error` sibling.
//
// `.error` files contain one sentinel name per line; only the
// first non-empty line is honored. This keeps the format simple
// while leaving room for human notes after a blank line.
func TestMalformed_RFC5545(t *testing.T) {
	t.Parallel()

	dir := malformedDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ics") {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			wantSentinel := loadSentinelName(t, dir, strings.TrimSuffix(name, ".ics"))
			want, ok := malformedSentinelsRFC5545[wantSentinel]
			if !ok {
				t.Fatalf("unknown sentinel name %q in .error file", wantSentinel)
			}

			_, parseErr := rfc5545.Parse(bytes.NewReader(input))
			if parseErr == nil {
				t.Fatalf("expected parse error wrapping %s; got nil", wantSentinel)
			}
			if !errors.Is(parseErr, want) {
				t.Fatalf("parse error %v does not wrap %s", parseErr, wantSentinel)
			}
		})
	}
}

// malformedDir resolves testdata/malformed/ relative to the test
// binary by walking up from the package dir.
func malformedDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, "testdata", "malformed")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find testdata/malformed above %s", dir)
		}
		dir = parent
	}
}

// loadSentinelName reads testdata/malformed/<stem>.error and
// returns the first non-empty trimmed line.
func loadSentinelName(t *testing.T, dir, stem string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, stem+".error"))
	if err != nil {
		t.Fatalf("read .error sibling: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	t.Fatalf(".error sibling for %s is empty", stem)
	return ""
}
