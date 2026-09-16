// SPDX-License-Identifier: MIT

// Command fixtures-verify regenerates `<name>.canonical` and
// `<name>.hash` siblings for every parseable fixture in the
// conformance corpus, verifies the corpus' rrule contracts, mirrors
// the corpus into the Go module's testdata/, syncs the canonical
// fuzz seeds into the go-fuzz convention dirs at
// codec/<rfc>/testdata/fuzz/, and regenerates the language-agnostic
// behavior fixtures under spec/behavior/ (see
// generateBehaviorFixtures).
//
// The corpus is authored at spec/v1.0/conformance/ in the poly-vstar
// monorepo — the single source for both spec text and fixtures. The
// module's testdata/ is a generated, committed mirror of it, which
// keeps the published Go module self-contained. On the hop-top/vstar
// mirror the module is a subtree of go/ alone, so there is no sibling
// spec/ tree: the spec-writing and mirroring steps are skipped and
// testdata/ is verified in place. See corpusRoot.
//
// testdata/fuzz/ is the root package's go-fuzz corpus (read by
// fuzz_test.go), not part of the spec corpus, and is preserved across
// the mirror.
//
// After regeneration, the caller (typically `make fixtures-verify`
// or CI) runs `git diff --exit-code spec/v1.0/conformance spec/behavior
// go/testdata go/codec/*/testdata/fuzz` to detect drift. Any non-empty diff means
// either the implementation drifted (fix the implementation) or the
// fixture's canonical/hash changed deliberately (commit the
// regenerated siblings as part of the same PR).
//
// Walks, relative to the corpus root:
//
//   - rfc5545/*.ics             → .canonical + .hash via Calendar
//   - rfc6350/*.vcf             → .canonical + .hash via Card
//   - supersession/*.ics        → .canonical + .hash via Calendar
//   - malformed/*               → no-op (these don't parse)
//   - fuzz-seed/rfc5545/*.bytes → copied into
//     codec/rfc5545/testdata/fuzz/
//     FuzzParse_RFC5545/seed_<stem>
//     (wrapped in go-fuzz format)
//   - fuzz-seed/rfc6350/*.bytes → same for rfc6350
//   - rrule/**/<name>.rrule     → parser, formatter and
//     evaluator contracts per the optional sidecars (.expect.json,
//     .formatted, .next.json, .expand.json, .between.json)
//   - rrule/**/<name>.ics       → recurrence-set contracts
//     per the optional sidecars (.expect.json, .occurrences.json)
//     See verifyRRuleFixtures for each sidecar's shape.
//
// Exits 0 on success; non-zero with a diagnostic on the first error.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/canonical"
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/codec/rfc6350"
	"hop.top/vstar/hashing"
	"hop.top/vstar/rrule"
)

// moduleRoot finds the Go module root by walking up from the current
// working directory looking for a `testdata/rfc5545` directory. The
// intended invocation is `go -C go run ./cmd/fixtures-verify` (what
// `make fixtures-verify` does), so the search starts in go/ and matches
// immediately; running from a package directory ascends to the same
// place. The corpus the command reads and writes is then resolved
// from the module root by corpusRoot.
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

func main() {
	module, err := moduleRoot()
	if err != nil {
		fail(err)
	}
	corpus, authored := corpusRoot(module)
	mirror := filepath.Join(module, "testdata")

	if err := regenerateCalendars(filepath.Join(corpus, "rfc5545")); err != nil {
		fail(err)
	}
	if err := regenerateCalendars(filepath.Join(corpus, "supersession")); err != nil {
		fail(err)
	}
	if err := regenerateCards(filepath.Join(corpus, "rfc6350")); err != nil {
		fail(err)
	}
	if err := verifyRRuleFixtures(filepath.Join(corpus, "rrule")); err != nil {
		fail(err)
	}
	var mirrored []string
	if authored {
		var err error
		if mirrored, err = mirrorCorpus(corpus, mirror); err != nil {
			fail(err)
		}
	}
	if err := syncFuzzSeeds(module, corpus); err != nil {
		fail(err)
	}
	if err := generateBehaviorFixtures(module, authored); err != nil {
		fail(err)
	}

	if authored {
		// Rewriting the mirror restores any hand-edited file, so the
		// working tree goes clean again and the caller's `git diff`
		// cannot see the edit. Report the paths here instead: they are
		// either a hand-edit of the generated mirror (edit the corpus)
		// or a corpus change the mirror had yet to pick up (commit the
		// regenerated mirror). Both need the caller's attention.
		if len(mirrored) > 0 {
			fmt.Fprintf(os.Stderr, "fixtures-verify: %d mirror path(s) differed from spec/v1.0/conformance and were rewritten:\n", len(mirrored))
			for _, rel := range mirrored {
				fmt.Fprintf(os.Stderr, "  testdata/%s\n", filepath.ToSlash(rel))
			}
			fmt.Fprintln(os.Stderr, "testdata/ is a generated mirror: author fixtures in spec/v1.0/conformance/,")
			fmt.Fprintln(os.Stderr, "then commit the regenerated mirror alongside them.")
			os.Exit(1)
		}
		fmt.Println("fixtures-verify: regenerated spec/v1.0/conformance canonical/hash siblings and spec/behavior fixtures, mirror is in sync, synced fuzz seeds; rrule fixtures verified")
		return
	}
	fmt.Println("fixtures-verify: regenerated testdata/ canonical/hash siblings + fuzz seeds; rrule fixtures verified (no sibling spec/ tree; corpus verified in place, behavior fixtures skipped)")
}

// verifyRRuleFixtures walks testdata/rrule and checks every
// <name>.rrule and <name>.ics against its sidecar contracts. A
// sidecar names an expected failure class by its corpus token
// (spec/05 §Failure classes; see errorClasses) and the reference
// must wrap that sentinel.
//
// <name>.rrule sidecars:
//
//   - <name>.expect.json — {"sentinel": <class>} — ValidateRRule
//     must fail with that class. Absent → parsing must succeed and
//     the remaining sidecars apply.
//   - <name>.formatted — the parsed rule rendered by Rule.String
//     must equal the file (trailing newline aside).
//   - <name>.next.json — {"dtstart", "after", "expected": [...],
//     "error"?: <class>} — NextOccurrence iteratively must yield
//     each expected timestamp (using the previous result as
//     `after`); with "error", the call after the last expected
//     timestamp must fail with that class.
//   - <name>.expand.json — {"dtstart", "limit", "expected": [...],
//     "complete"} — Occurrences must yield exactly `expected` and
//     report `complete`; or {"dtstart", "limit", "error": <class>}
//     — the call must fail with that class.
//   - <name>.between.json — {"dtstart", "start", "end",
//     "expected": [...]} — Between over [start, end) must yield
//     exactly `expected`; or {..., "error": <class>}.
//
// <name>.ics sidecars (the fixture's first component goes through
// SetFromComponent):
//
//   - <name>.expect.json — {"sentinel": <class>} — parsing the
//     calendar or building the Set must fail with that class.
//   - <name>.occurrences.json — {"limit", "expected": [...],
//     "complete"} — Set.Occurrences must yield exactly `expected`
//     and report `complete`; or {"limit", "error": <class>}.
//
// All paths are resolved relative to `root`. Absent root is a
// silent no-op.
func verifyRRuleFixtures(root string) error {
	if _, err := os.Stat(root); errIsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return nil
		case strings.HasSuffix(path, ".rrule"):
			return verifyOneRRuleFixture(path)
		case strings.HasSuffix(path, ".ics"):
			return verifyOneSetFixture(path)
		}
		return nil
	})
}

// sentinelSpec is the .expect.json shape.
type sentinelSpec struct {
	Sentinel string `json:"sentinel"`
}

// outcomeSpec is the shape shared by the JSON sidecars that state
// the result of one evaluator call: the call's inputs plus either
// an expected occurrence list (and, for bounded expansion, the
// complete flag) or the failure class the call must wrap.
type outcomeSpec struct {
	DTStart  string   `json:"dtstart"`
	After    string   `json:"after"`
	Start    string   `json:"start"`
	End      string   `json:"end"`
	Limit    int      `json:"limit"`
	Expected []string `json:"expected"`
	Complete bool     `json:"complete"`
	Error    string   `json:"error"`
}

// readSidecar unmarshals path into v, reporting found=false when
// the sidecar does not exist.
func readSidecar(path string, v any) (bool, error) {
	raw, err := os.ReadFile(path)
	if errIsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	return true, nil
}

func verifyOneRRuleFixture(rrulePath string) error {
	body, err := os.ReadFile(rrulePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", rrulePath, err)
	}
	value := strings.TrimRight(string(body), "\r\n")
	stem := strings.TrimSuffix(rrulePath, ".rrule")

	// Sentinel expectation.
	var expect sentinelSpec
	found, err := readSidecar(stem+".expect.json", &expect)
	if err != nil {
		return err
	}
	if found {
		return expectClass(rrulePath, expect.Sentinel, rrule.ValidateRRule(value))
	}

	// No sentinel sidecar → parse must succeed.
	rule, err := rrule.ParseRRule(value)
	if err != nil {
		return fmt.Errorf("%s: ParseRRule failed: %w", rrulePath, err)
	}
	for _, check := range []func(string, rrule.Rule) error{
		verifyFormatted, verifyNext, verifyExpand, verifyBetween,
	} {
		if err := check(stem, rule); err != nil {
			return err
		}
	}
	return nil
}

// verifyFormatted checks the optional <stem>.formatted sidecar.
func verifyFormatted(stem string, rule rrule.Rule) error {
	path := stem + ".formatted"
	raw, err := os.ReadFile(path)
	if errIsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	want := strings.TrimRight(string(raw), "\r\n")
	if got := rule.String(); got != want {
		return fmt.Errorf("%s: Rule.String() = %q, want %q", path, got, want)
	}
	return nil
}

// verifyNext checks the optional <stem>.next.json sidecar.
func verifyNext(stem string, rule rrule.Rule) error {
	path := stem + ".next.json"
	var spec outcomeSpec
	if found, err := readSidecar(path, &spec); err != nil || !found {
		return err
	}
	wantErr, err := optionalClass(path, spec.Error)
	if err != nil {
		return err
	}
	dt, err := parseTimeField(path, "dtstart", spec.DTStart)
	if err != nil {
		return err
	}
	after, err := parseTimeField(path, "after", spec.After)
	if err != nil {
		return err
	}
	for i, want := range spec.Expected {
		got, ok, err := rrule.NextOccurrence(rule, dt, after)
		if err != nil {
			return fmt.Errorf("%s: NextOccurrence step %d failed: %w", path, i, err)
		}
		if !ok {
			return fmt.Errorf("%s: NextOccurrence step %d returned (zero, false), want %s", path, i, want)
		}
		wantT, err := parseTimeField(path, fmt.Sprintf("expected[%d]", i), want)
		if err != nil {
			return err
		}
		if !got.Equal(wantT) {
			return fmt.Errorf("%s: NextOccurrence step %d: got %s, want %s",
				path, i, got.Format(time.RFC3339), wantT.Format(time.RFC3339))
		}
		after = got
	}
	if wantErr == nil {
		return nil
	}
	// The sidecar names a failure class: the step after the last
	// expected occurrence must fail with it — neither yield another
	// occurrence nor report ordinary termination.
	got, ok, err := rrule.NextOccurrence(rule, dt, after)
	switch {
	case err == nil && ok:
		return fmt.Errorf("%s: expected %s after step %d, NextOccurrence yielded %s",
			path, spec.Error, len(spec.Expected), got.Format(time.RFC3339))
	case err == nil:
		return fmt.Errorf("%s: expected %s after step %d, NextOccurrence reported termination",
			path, spec.Error, len(spec.Expected))
	case !errors.Is(err, wantErr):
		return fmt.Errorf("%s: expected %s after step %d, got %w", path, spec.Error, len(spec.Expected), err)
	}
	return nil
}

// verifyExpand checks the optional <stem>.expand.json sidecar.
func verifyExpand(stem string, rule rrule.Rule) error {
	path := stem + ".expand.json"
	var spec outcomeSpec
	if found, err := readSidecar(path, &spec); err != nil || !found {
		return err
	}
	dt, err := parseTimeField(path, "dtstart", spec.DTStart)
	if err != nil {
		return err
	}
	got, complete, err := rrule.Occurrences(rule, dt, spec.Limit)
	return checkBounded(path, spec, got, complete, err)
}

// verifyBetween checks the optional <stem>.between.json sidecar.
func verifyBetween(stem string, rule rrule.Rule) error {
	path := stem + ".between.json"
	var spec outcomeSpec
	if found, err := readSidecar(path, &spec); err != nil || !found {
		return err
	}
	dt, err := parseTimeField(path, "dtstart", spec.DTStart)
	if err != nil {
		return err
	}
	start, err := parseTimeField(path, "start", spec.Start)
	if err != nil {
		return err
	}
	end, err := parseTimeField(path, "end", spec.End)
	if err != nil {
		return err
	}
	got, err := rrule.Between(rule, dt, start, end)
	return checkOutcome(path, spec, got, err)
}

// verifyOneSetFixture checks a <name>.ics recurrence-set fixture:
// the calendar's first component goes through SetFromComponent,
// then either .expect.json names the class that must have failed
// or .occurrences.json states the bounded expansion.
func verifyOneSetFixture(icsPath string) error {
	stem := strings.TrimSuffix(icsPath, ".ics")
	input, err := os.ReadFile(icsPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", icsPath, err)
	}
	set, err := setFromCalendar(input)

	var expect sentinelSpec
	found, rerr := readSidecar(stem+".expect.json", &expect)
	if rerr != nil {
		return rerr
	}
	if found {
		return expectClass(icsPath, expect.Sentinel, err)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", icsPath, err)
	}

	path := stem + ".occurrences.json"
	var spec outcomeSpec
	if found, err := readSidecar(path, &spec); err != nil || !found {
		return err
	}
	got, complete, err := set.Occurrences(spec.Limit)
	return checkBounded(path, spec, got, complete, err)
}

// setFromCalendar parses an .ics fixture and builds a Set from its
// first component.
func setFromCalendar(input []byte) (rrule.Set, error) {
	cal, err := rfc5545.Parse(bytes.NewReader(input))
	if err != nil {
		return rrule.Set{}, err
	}
	if len(cal.Components) == 0 {
		return rrule.Set{}, errors.New("calendar has no components")
	}
	return rrule.SetFromComponent(cal.Components[0])
}

// checkBounded compares a bounded-expansion result against the
// sidecar: the failure class when one is named, else the
// occurrence list and the complete flag.
func checkBounded(path string, spec outcomeSpec, got []time.Time, complete bool, err error) error {
	if err := checkOutcome(path, spec, got, err); err != nil || spec.Error != "" {
		return err
	}
	if complete != spec.Complete {
		return fmt.Errorf("%s: complete = %v, want %v", path, complete, spec.Complete)
	}
	return nil
}

// checkOutcome compares one evaluator call's (result, error) with
// the sidecar: with "error" named the call must fail with that
// class; otherwise it must succeed and yield exactly `expected`.
func checkOutcome(path string, spec outcomeSpec, got []time.Time, err error) error {
	if spec.Error != "" {
		return expectClass(path, spec.Error, err)
	}
	if err != nil {
		return fmt.Errorf("%s: unexpected error: %w", path, err)
	}
	if len(got) != len(spec.Expected) {
		return fmt.Errorf("%s: got %d occurrences %v, want %d %v",
			path, len(got), formatTimes(got), len(spec.Expected), spec.Expected)
	}
	for i, want := range spec.Expected {
		wantT, err := parseTimeField(path, fmt.Sprintf("expected[%d]", i), want)
		if err != nil {
			return err
		}
		if !got[i].Equal(wantT) {
			return fmt.Errorf("%s: occurrence %d: got %s, want %s",
				path, i, got[i].Format(time.RFC3339), wantT.Format(time.RFC3339))
		}
	}
	return nil
}

func formatTimes(ts []time.Time) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.UTC().Format(time.RFC3339)
	}
	return out
}

// parseTimeField parses an RFC 5545 form #2 (UTC) sidecar value.
func parseTimeField(path, field, value string) (time.Time, error) {
	t, ok := vstar.ParseTime(value)
	if !ok {
		return time.Time{}, fmt.Errorf("%s: bad %s %q", path, field, value)
	}
	return t, nil
}

// errorClasses maps each failure-class token the corpus may name
// (spec/05 §Failure classes) to the sentinel the Go reference wraps
// for it. Sidecars name the class by token; errors.Is does the
// matching so wrapped context never breaks a fixture.
var errorClasses = map[string]error{
	"ErrMalformed":          vstar.ErrMalformed,
	"ErrUnsupportedRRule":   rrule.ErrUnsupportedRRule,
	"ErrIterationCap":       rrule.ErrIterationCap,
	"ErrUnboundedExpansion": rrule.ErrUnboundedExpansion,
}

// optionalClass resolves a sidecar's failure-class token, returning
// (nil, nil) when the sidecar names none and a diagnostic naming
// the sidecar when the token is unknown.
func optionalClass(path, token string) (error, error) {
	if token == "" {
		return nil, nil
	}
	want, ok := errorClasses[token]
	if !ok {
		return nil, fmt.Errorf("%s: unknown failure class %q", path, token)
	}
	return want, nil
}

// expectClass asserts err wraps the sentinel token names.
func expectClass(path, token string, err error) error {
	want, cerr := optionalClass(path, token)
	if cerr != nil {
		return cerr
	}
	if want == nil {
		return fmt.Errorf("%s: no failure class named", path)
	}
	if err == nil {
		return fmt.Errorf("%s: expected %s, call succeeded", path, token)
	}
	if !errors.Is(err, want) {
		return fmt.Errorf("%s: expected %s, got %w", path, token, err)
	}
	return nil
}

// regenerateCalendars walks dir for *.ics, parses each, writes
// .canonical and .hash siblings.
func regenerateCalendars(dir string) error {
	if _, err := os.Stat(dir); errIsNotExist(err) {
		return nil // optional dir; skip silently.
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ics") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		stem := strings.TrimSuffix(full, ".ics")
		input, err := os.ReadFile(full)
		if err != nil {
			return fmt.Errorf("read %s: %w", full, err)
		}
		cal, err := rfc5545.Parse(bytes.NewReader(input))
		if err != nil {
			return fmt.Errorf("parse %s: %w", full, err)
		}
		canonBytes := crlfToLF(canonical.Calendar(cal))
		if err := os.WriteFile(stem+".canonical", canonBytes, 0o644); err != nil {
			return fmt.Errorf("write %s.canonical: %w", stem, err)
		}
		hash := hashing.Calendar(cal) + "\n"
		if err := os.WriteFile(stem+".hash", []byte(hash), 0o644); err != nil {
			return fmt.Errorf("write %s.hash: %w", stem, err)
		}
	}
	return nil
}

// crlfToLF strips \r before \n. The on-disk convention is LF for
// diff-friendliness; the canonical bytes per spec/03 are CRLF and
// callers expand back at comparison time (TestGoldenFiles in
// canonical_test.go does this; TestHashGoldens reads bytes that
// hashing.Calendar already produced from the parsed Calendar so
// no expansion is needed there).
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

// regenerateCards walks dir for *.vcf, parses each, writes
// .canonical and .hash siblings.
func regenerateCards(dir string) error {
	if _, err := os.Stat(dir); errIsNotExist(err) {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".vcf") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		stem := strings.TrimSuffix(full, ".vcf")
		input, err := os.ReadFile(full)
		if err != nil {
			return fmt.Errorf("read %s: %w", full, err)
		}
		cards, err := rfc6350.New().Parse(bytes.NewReader(input))
		if err != nil {
			return fmt.Errorf("parse %s: %w", full, err)
		}
		if len(cards) != 1 {
			return fmt.Errorf("parse %s: expected exactly one card, got %d", full, len(cards))
		}
		canonBytes := crlfToLF([]byte(canonical.Card(cards[0])))
		if err := os.WriteFile(stem+".canonical", canonBytes, 0o644); err != nil {
			return fmt.Errorf("write %s.canonical: %w", stem, err)
		}
		hash := hashing.Card(cards[0]) + "\n"
		if err := os.WriteFile(stem+".hash", []byte(hash), 0o644); err != nil {
			return fmt.Errorf("write %s.hash: %w", stem, err)
		}
	}
	return nil
}

// syncFuzzSeeds copies every <corpus>/fuzz-seed/<rfc>/seed_*.bytes
// into <module>/codec/<rfc>/testdata/fuzz/FuzzParse_RFC<RFC>/seed_<stem>
// using the go-fuzz wrapper format. The seeds are corpus content;
// the go-fuzz dirs are Go-native and always live in the module.
func syncFuzzSeeds(module, corpus string) error {
	pairs := []struct {
		src     string
		dst     string
		fuzzFn  string
		isBytes bool
	}{
		{
			filepath.Join(corpus, "fuzz-seed", "rfc5545"),
			filepath.Join(module, "codec", "rfc5545", "testdata", "fuzz", "FuzzParse_RFC5545"),
			"FuzzParse_RFC5545",
			false, // .ics seeds wrap as string for nicer fuzz panics.
		},
		{
			filepath.Join(corpus, "fuzz-seed", "rfc6350"),
			filepath.Join(module, "codec", "rfc6350", "testdata", "fuzz", "FuzzParse_RFC6350"),
			"FuzzParse_RFC6350",
			true, // .vcf seeds wrap as []byte (existing convention).
		},
	}
	for _, p := range pairs {
		if _, err := os.Stat(p.src); errIsNotExist(err) {
			continue
		}
		if err := os.MkdirAll(p.dst, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", p.dst, err)
		}
		entries, err := os.ReadDir(p.src)
		if err != nil {
			return fmt.Errorf("read %s: %w", p.src, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".bytes") {
				continue
			}
			stem := strings.TrimSuffix(e.Name(), ".bytes")
			input, err := os.ReadFile(filepath.Join(p.src, e.Name()))
			if err != nil {
				return fmt.Errorf("read %s: %w", e.Name(), err)
			}
			wrapped := wrapFuzzSeed(input, p.isBytes)
			out := filepath.Join(p.dst, stem)
			if err := os.WriteFile(out, []byte(wrapped), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", out, err)
			}
		}
	}
	return nil
}

// wrapFuzzSeed renders the go-fuzz "v1" wrapper around raw input.
// When isBytes is true the value is encoded as []byte("..."); when
// false it is encoded as string("..."). Both forms use Go-style
// escaping so any byte is representable.
func wrapFuzzSeed(input []byte, isBytes bool) string {
	var b strings.Builder
	b.WriteString("go test fuzz v1\n")
	if isBytes {
		b.WriteString("[]byte(")
	} else {
		b.WriteString("string(")
	}
	b.WriteString(quoteGoString(input))
	b.WriteString(")\n")
	return b.String()
}

// quoteGoString returns input as a double-quoted Go string literal.
// Uses fmt's %q which produces the canonical Go escape form (\r,
// \n, \t, \xNN, etc.) deterministically.
func quoteGoString(input []byte) string {
	return fmt.Sprintf("%q", string(input))
}

func errIsNotExist(err error) bool {
	return err != nil && (os.IsNotExist(err) || errors_Is_PathError(err))
}

// errors_Is_PathError treats fs.PathError wrapping ENOENT the same
// as os.IsNotExist (which it should already, but defensively).
// Uses errors.As so wrapped errors are still recognized.
func errors_Is_PathError(err error) bool {
	var p *fs.PathError
	if !errors.As(err, &p) {
		return false
	}
	return os.IsNotExist(p.Err)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "fixtures-verify: %v\n", err)
	os.Exit(1)
}

// behaviorGenPackage is the generator fixtures-verify delegates the
// spec/behavior/ tree to. It is a sibling command rather than a
// package this one imports, because both are package main: Go has
// no way to call into another main. Running it as a subprocess is
// the same shape `make fixtures-verify` uses to run this command.
const behaviorGenPackage = "./cmd/fixtures-gen-behavior"

// generateBehaviorFixtures regenerates spec/behavior/ by running
// the fixtures-gen-behavior command against the module.
//
// The behavior fixtures are the language-agnostic tables the
// future TypeScript, Python, Rust and PHP ports read instead of
// hand-translating this module's unit-test assertions. They are
// generated from the reference for the same reason the corpus'
// .canonical/.hash siblings are: a hand-written expectation drifts
// silently, a generated one shows up in the caller's `git diff`.
//
// Skipped entirely when authored is false. That is the
// hop-top/vstar mirror, where the module is a subtree of go/ alone:
// there is no sibling spec/ tree to write into, and the mirror's
// own behavior coverage is its Go unit tests. The mirror must
// still build and self-verify, so this is a silent no-op there
// rather than an error.
func generateBehaviorFixtures(module string, authored bool) error {
	if !authored {
		return nil
	}
	cmd := exec.Command("go", "run", behaviorGenPackage)
	cmd.Dir = module
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", behaviorGenPackage, err)
	}
	return nil
}
