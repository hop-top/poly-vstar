// SPDX-License-Identifier: MIT

package rfc6350_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc6350"
)

// malformedSentinelsRFC6350 maps the sentinel name encoded in
// testdata/malformed/<name>.error to the actual vstar package
// variable.
var malformedSentinelsRFC6350 = map[string]error{
	"ErrMalformed":          vstar.ErrMalformed,
	"ErrUnclosedBlock":      vstar.ErrUnclosedBlock,
	"ErrUnsupportedVersion": vstar.ErrUnsupportedVersion,
	"ErrMissingUID":         vstar.ErrMissingUID,
}

// TestMalformed_RFC6350 walks testdata/malformed/, picks every
// `*.vcf`, and asserts the codec produces the sentinel named in
// `<stem>.error`.
//
// Most fixtures fail at parse-time. ErrMissingUID is encoder-only
// at v1.0 — for that sentinel the test parses successfully and
// then asserts the encoder refuses with the expected error.
func TestMalformed_RFC6350(t *testing.T) {
	t.Parallel()

	dir := malformedDirVCF(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".vcf") {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			wantSentinel := loadSentinelNameVCF(t, dir, strings.TrimSuffix(name, ".vcf"))
			want, ok := malformedSentinelsRFC6350[wantSentinel]
			if !ok {
				t.Fatalf("unknown sentinel name %q in .error file", wantSentinel)
			}

			cards, parseErr := rfc6350.New().Parse(bytes.NewReader(input))
			if parseErr != nil {
				if !errors.Is(parseErr, want) {
					t.Fatalf("parse error %v does not wrap %s", parseErr, wantSentinel)
				}
				return // parse-time sentinel fired; done.
			}

			// Parse succeeded — the fixture targets an encoder-time
			// sentinel (e.g. ErrMissingUID). Re-encode every parsed
			// card and assert at least one returns the expected
			// sentinel.
			if len(cards) == 0 {
				t.Fatalf("parse returned no cards and no error — cannot exercise encoder-time sentinel")
			}
			for _, c := range cards {
				var sb strings.Builder
				encErr := rfc6350.New().Encode(&sb, c)
				if encErr == nil {
					continue
				}
				if errors.Is(encErr, want) {
					return
				}
				t.Fatalf("encode error %v does not wrap %s", encErr, wantSentinel)
			}
			t.Fatalf("expected %s during parse or encode; both succeeded", wantSentinel)
		})
	}
}

// malformedDirVCF resolves testdata/malformed/ relative to the
// rfc6350 package dir.
func malformedDirVCF(t *testing.T) string {
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

// loadSentinelNameVCF reads testdata/malformed/<stem>.error and
// returns the first non-empty trimmed line.
func loadSentinelNameVCF(t *testing.T, dir, stem string) string {
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
