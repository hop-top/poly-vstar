// SPDX-License-Identifier: MIT

package hashing_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/codec/rfc6350"
	"hop.top/vstar/hashing"
)

// TestHashGoldens loads every fixture in testdata/rfc5545/*.ics
// and testdata/rfc6350/*.vcf, parses it, computes
// hashing.Calendar (for .ics) or hashing.Card (for .vcf), and
// compares the result against the `<fixture>.hash` sibling file.
//
// The .hash file contains exactly `sha256:<64 hex>` followed by an
// LF terminator. Future TypeScript / Racket V* implementations
// MUST produce byte-identical .hash content for these fixtures —
// this is the cross-implementation parity contract.
func TestHashGoldens(t *testing.T) {
	roots := []struct {
		dir  string
		ext  string
		card bool
	}{
		{"../testdata/rfc5545", ".ics", false},
		{"../testdata/rfc6350", ".vcf", true},
	}
	for _, r := range roots {
		entries, err := os.ReadDir(r.dir)
		if err != nil {
			t.Fatalf("read %s: %v", r.dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), r.ext) {
				continue
			}
			name := e.Name()
			fullPath := filepath.Join(r.dir, name)
			hashPath := strings.TrimSuffix(fullPath, r.ext) + ".hash"
			t.Run(name, func(t *testing.T) {
				goldenBytes, err := os.ReadFile(hashPath)
				if err != nil {
					t.Fatalf("read hash golden %s: %v (run `go test -run TestHashGoldens -update` "+
						"or regenerate via TestHashGoldens_Regenerate)", hashPath, err)
				}
				want := strings.TrimRight(string(goldenBytes), "\n")
				inputBytes, err := os.ReadFile(fullPath)
				if err != nil {
					t.Fatalf("read fixture %s: %v", fullPath, err)
				}
				var got string
				if r.card {
					cards, err := rfc6350.New().Parse(bytes.NewReader(inputBytes))
					if err != nil {
						t.Fatalf("rfc6350 parse %s: %v", name, err)
					}
					if len(cards) != 1 {
						t.Fatalf("expected exactly one card in %s, got %d", name, len(cards))
					}
					got = hashing.Card(cards[0])
				} else {
					cal, err := rfc5545.Parse(bytes.NewReader(inputBytes))
					if err != nil {
						t.Fatalf("rfc5545 parse %s: %v", name, err)
					}
					got = hashing.Calendar(cal)
				}
				if got != want {
					t.Fatalf("hash golden mismatch for %s.\n got:  %s\n want: %s", name, got, want)
				}
			})
		}
	}
}
