// SPDX-License-Identifier: MIT

// Command fixtures-gen-behavior generates the language-agnostic
// behavior fixtures under spec/behavior/ from the Go reference
// implementation.
//
// Motivation: the behavior of validate, diff, supersession,
// duration, ext and the TZID-aware time parser was, until this
// command existed, knowledge locked inside Go unit tests. A port
// (TypeScript, Python, Rust, PHP) could only recover it by reading
// Go and hand-translating assertions — slow, and silently divergent
// the moment the reference moves. The fixtures replace that with
// table-driven tests every port reads from the same JSON.
//
// Two rules shape every fixture format:
//
//   - Messages are never part of a fixture. Diagnostic.Message,
//     error strings and any other human-readable prose change
//     across versions without the behavior changing; a port that
//     pinned them would fail on a reword. Codes, paths, severities
//     and failure classes are the stable surface.
//   - Every timestamp is RFC 5545 form #2 — UTC, Z-suffixed
//     (spec/03). A port parses one shape, not a dialect per family.
//
// Families written (see spec/behavior/README.md for each format):
//
//   - validate/<name>.ics + <name>.diagnostics.json
//   - diff/<name>.a.ics + <name>.b.ics + <name>.diff.json
//   - supersession/<name>.effective.json
//   - duration/parse.json
//   - duration/<name>.ics + <name>.trigger.json
//   - ext/scopes.json
//   - time/tzid.json
//
// The command is invoked by fixtures-verify, which regenerates the
// tree and lets the caller's `git diff` detect drift. It is not run
// standalone in CI.
//
// On the hop-top/vstar mirror the module is a subtree of go/ alone
// and there is no sibling spec/ tree: generation is skipped
// entirely (behaviorRoot reports ok=false). The mirror's own
// behavior coverage is its Go unit tests; the fixtures are a
// monorepo artifact the ports consume.
//
// Exits 0 on success; non-zero with a diagnostic on the first error.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	module, err := moduleRoot()
	if err != nil {
		fail(err)
	}
	root, ok := behaviorRoot(module)
	if !ok {
		fmt.Println("fixtures-gen-behavior: no sibling spec/ tree; nothing to generate")
		return
	}
	n, changed, err := Generate(root)
	if err != nil {
		fail(err)
	}
	// Rewriting a fixture restores any hand-edited file, so the
	// working tree goes clean again and the caller's `git diff`
	// cannot see the edit. Report the paths here instead: they are
	// either a hand-edit of a generated fixture (change the
	// generator, not the fixture) or a behavior change the tree had
	// yet to pick up (commit the regenerated fixtures). Both need
	// the caller's attention. Same contract as fixtures-verify's
	// mirror report.
	if len(changed) > 0 {
		fmt.Fprintf(os.Stderr, "fixtures-gen-behavior: %d fixture path(s) differed from what is committed and were rewritten:\n", len(changed))
		for _, rel := range changed {
			fmt.Fprintf(os.Stderr, "  spec/behavior/%s\n", rel)
		}
		fmt.Fprintln(os.Stderr, "spec/behavior/ is generated: change the reference or the generator,")
		fmt.Fprintln(os.Stderr, "then commit the regenerated fixtures alongside the change.")
		os.Exit(1)
	}
	fmt.Printf("fixtures-gen-behavior: %d behavior fixture file(s) under %s are in sync\n", n, root)
}

// Generate writes every behavior-fixture family under root. It
// returns the number of files written and, second, the fixture
// paths whose bytes differed from what was already on disk —
// added, changed or removed. An empty second result means the
// committed tree already matched the reference.
//
// README.md is preserved: it is hand-authored documentation, not
// generated output. Only the family subdirectories are cleared.
//
// Root-parameterised so the command's own tests can drive it
// against a temp dir.
func Generate(root string) (int, []string, error) {
	behaviorRootForReporting = root
	rewritten = nil
	removedBytes = map[string][]byte{}

	families := []struct {
		dir   string
		write func(string) (int, error)
	}{
		{"validate", writeValidateFamily},
		{"diff", writeDiffFamily},
		{"supersession", writeSupersessionFamily},
		{"duration", writeDurationFamily},
		{"ext", writeExtFamily},
		{"time", writeTimeFamily},
	}
	total := 0
	for _, f := range families {
		dir := filepath.Join(root, f.dir)
		if err := clearDir(dir, nil); err != nil {
			return 0, nil, err
		}
		n, err := f.write(dir)
		if err != nil {
			return 0, nil, fmt.Errorf("%s: %w", f.dir, err)
		}
		total += n
	}
	return total, settleRewritten(root), nil
}

// moduleRoot finds the Go module root by walking up from the
// current working directory looking for a testdata/rfc5545
// directory; see fixtures-verify's moduleRoot for the rationale.
func moduleRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	dir := cwd
	for {
		if st, err := os.Stat(filepath.Join(dir, "testdata", "rfc5545")); err == nil && st.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find testdata/rfc5545 walking up from %s", cwd)
		}
		dir = parent
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "fixtures-gen-behavior: %v\n", err)
	os.Exit(1)
}
