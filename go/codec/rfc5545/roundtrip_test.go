// SPDX-License-Identifier: MIT

package rfc5545

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vstar "hop.top/vstar"
)

// fixtureDir is the path to vstar/testdata/rfc5545/, located two levels
// up from this Go file (codec/rfc5545/ → vstar/), plus the testdata leg.
// The path is computed at runtime so a worktree move doesn't break tests.
func fixtureDir(t *testing.T) string {
	t.Helper()
	// codec/rfc5545/foo_test.go runs with cwd = codec/rfc5545/. From
	// there, ../../testdata/rfc5545 reaches vstar/testdata/rfc5545.
	return filepath.Join("..", "..", "testdata", "rfc5545")
}

// TestRoundTripFixture_All loads every .ics fixture under
// testdata/rfc5545/, parses it, encodes it, re-parses the encoded
// output, and asserts the two parses are semantically equal. This
// is the load-bearing round-trip guarantee the codec ships.
func TestRoundTripFixture_All(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixture dir %s: %v", dir, err)
	}
	any := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ics") {
			continue
		}
		any = true
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			cal1, err := Parse(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("first parse %s: %v", name, err)
			}

			var enc bytes.Buffer
			if err := Encode(&enc, cal1); err != nil {
				t.Fatalf("encode %s: %v", name, err)
			}

			cal2, err := Parse(&enc)
			if err != nil {
				t.Fatalf("re-parse %s: %v", name, err)
			}

			if !calsEqualSemantic(cal1, cal2) {
				t.Errorf("%s: trees diverged after round-trip\ncal1=%+v\ncal2=%+v",
					name, cal1, cal2)
			}
		})
	}
	if !any {
		t.Fatalf("no .ics fixtures found in %s", dir)
	}
}

func calsEqualSemantic(a, b vstar.Calendar) bool {
	if a.ProdID != b.ProdID {
		return false
	}
	return compsEqualSemantic(a.Components, b.Components)
}

func compsEqualSemantic(a, b []vstar.Component) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type {
			return false
		}
		if len(a[i].Props) != len(b[i].Props) {
			return false
		}
		for j := range a[i].Props {
			if !vstar.Equal(a[i].Props[j], b[i].Props[j]) {
				return false
			}
		}
		if !compsEqualSemantic(a[i].Sub, b[i].Sub) {
			return false
		}
	}
	return true
}
