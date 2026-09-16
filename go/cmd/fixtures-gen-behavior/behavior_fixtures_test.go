// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/diff"
	"hop.top/vstar/ext"
	"hop.top/vstar/supersession"
	"hop.top/vstar/validate"
)

// These tests consume spec/behavior/ the way a port will: load the
// .ics inputs, run the implementation, compare against the JSON
// sidecar. They are deliberately written as a port would write them
// — reading the committed fixtures rather than calling the
// generator — so each of TypeScript, Python, Rust and PHP has a
// working template to copy rather than a Go-specific harness.
//
// The generator's own assertions (assertCodes, assertDiffOps,
// assertTriggerExpectation) check that each case still exercises
// what it was authored for. These check something different: that
// the committed files load, decode, and describe the behavior the
// reference actually has. A fixture that is stale, hand-edited, or
// shaped wrong fails here even when the generator would happily
// rewrite it.

// behaviorDir locates the committed fixture tree, skipping the test
// when there is none. On the hop-top/vstar mirror the module is a
// subtree of go/ alone and ships no spec/ — the same guard
// corpusRoot and validate/codes_doc_test.go use.
func behaviorDir(t *testing.T) string {
	t.Helper()
	module, err := moduleRoot()
	if err != nil {
		t.Fatalf("moduleRoot: %v", err)
	}
	root, ok := behaviorRoot(module)
	if !ok {
		t.Skip("no sibling spec/ tree; behavior fixtures live in the poly-vstar monorepo")
	}
	if _, err := os.Stat(root); err != nil {
		t.Skipf("spec/behavior not present: %v", err)
	}
	return root
}

// loadJSON decodes a fixture sidecar into v.
func loadJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

// loadCalendar parses a fixture .ics.
func loadCalendar(t *testing.T, path string) vstar.Calendar {
	t.Helper()
	cal, err := readCalendar(path)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return cal
}

// stems lists the fixture base names in dir carrying the given
// sidecar suffix, sorted so subtests run in a stable order.
func stems(t *testing.T, dir, suffix string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		out = append(out, strings.TrimSuffix(e.Name(), suffix))
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("no %s fixtures under %s", suffix, dir)
	}
	return out
}

// TestBehaviorFixtures_Validate runs every validate/<name>.ics
// through validate.Validate and compares the sorted diagnostics
// against <name>.diagnostics.json.
func TestBehaviorFixtures_Validate(t *testing.T) {
	dir := filepath.Join(behaviorDir(t), "validate")
	for _, stem := range stems(t, dir, ".diagnostics.json") {
		t.Run(stem, func(t *testing.T) {
			cal := loadCalendar(t, filepath.Join(dir, stem+".ics"))
			var want []diagnosticFixture
			loadJSON(t, filepath.Join(dir, stem+".diagnostics.json"), &want)

			got := diagnosticFixtures(validate.Validate(cal))
			if len(got) == 0 && len(want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("diagnostics mismatch\n got: %+v\nwant: %+v", got, want)
			}
		})
	}
}

// TestBehaviorFixtures_ValidateCoverage asserts the committed
// fixtures still name every diagnostic code the reference can emit.
// The generator refuses to write an incomplete tree; this catches a
// tree that went stale after a code was added.
func TestBehaviorFixtures_ValidateCoverage(t *testing.T) {
	dir := filepath.Join(behaviorDir(t), "validate")
	seen := map[string]bool{}
	for _, stem := range stems(t, dir, ".diagnostics.json") {
		var ds []diagnosticFixture
		loadJSON(t, filepath.Join(dir, stem+".diagnostics.json"), &ds)
		for _, d := range ds {
			seen[d.Code] = true
		}
	}
	for _, code := range allValidateCodes {
		if !seen[code] {
			t.Errorf("no committed fixture emits %s", code)
		}
	}
}

// TestBehaviorFixtures_ValidateCleanDocuments asserts at least two
// fixtures are clean. Without them a port could pass the validate
// family by reporting every document as broken.
func TestBehaviorFixtures_ValidateCleanDocuments(t *testing.T) {
	dir := filepath.Join(behaviorDir(t), "validate")
	clean := 0
	for _, stem := range stems(t, dir, ".diagnostics.json") {
		var ds []diagnosticFixture
		loadJSON(t, filepath.Join(dir, stem+".diagnostics.json"), &ds)
		if len(ds) == 0 {
			clean++
		}
	}
	if clean < 2 {
		t.Errorf("want at least 2 clean validate fixtures, got %d", clean)
	}
}

// TestBehaviorFixtures_DiagnosticsSorted asserts every committed
// sidecar is in the documented (path, code) order. A port sorts its
// own output before comparing, so an unsorted fixture would make
// the comparison fail for a reason that has nothing to do with the
// implementation.
func TestBehaviorFixtures_DiagnosticsSorted(t *testing.T) {
	dir := filepath.Join(behaviorDir(t), "validate")
	for _, stem := range stems(t, dir, ".diagnostics.json") {
		var ds []diagnosticFixture
		loadJSON(t, filepath.Join(dir, stem+".diagnostics.json"), &ds)
		for i := 1; i < len(ds); i++ {
			prev, cur := ds[i-1], ds[i]
			if prev.Path > cur.Path || (prev.Path == cur.Path && prev.Code > cur.Code) {
				t.Errorf("%s: entry %d (%s %s) sorts before entry %d (%s %s)",
					stem, i, cur.Path, cur.Code, i-1, prev.Path, prev.Code)
			}
		}
	}
}

// TestBehaviorFixtures_Diff diffs every <name>.a.ics against
// <name>.b.ics and compares against <name>.diff.json.
func TestBehaviorFixtures_Diff(t *testing.T) {
	dir := filepath.Join(behaviorDir(t), "diff")
	for _, stem := range stems(t, dir, ".diff.json") {
		t.Run(stem, func(t *testing.T) {
			a := loadCalendar(t, filepath.Join(dir, stem+".a.ics"))
			b := loadCalendar(t, filepath.Join(dir, stem+".b.ics"))
			var want []componentDiffFixture
			loadJSON(t, filepath.Join(dir, stem+".diff.json"), &want)

			got := componentDiffFixtures(diff.OfCalendar(a, b))
			if len(got) == 0 && len(want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("diff mismatch\n got: %s\nwant: %s", mustJSON(t, got), mustJSON(t, want))
			}
		})
	}
}

// TestBehaviorFixtures_Supersession projects each conformance
// supersession fixture and compares against its .effective.json.
func TestBehaviorFixtures_Supersession(t *testing.T) {
	root := behaviorDir(t)
	dir := filepath.Join(root, "supersession")
	module, err := moduleRoot()
	if err != nil {
		t.Fatalf("moduleRoot: %v", err)
	}
	corpus := filepath.Join(filepath.Dir(module), specDirName, "v1.0", "conformance", "supersession")

	for _, stem := range stems(t, dir, ".effective.json") {
		t.Run(stem, func(t *testing.T) {
			cal := loadCalendar(t, filepath.Join(corpus, stem+".ics"))
			var want map[string]string
			loadJSON(t, filepath.Join(dir, stem+".effective.json"), &want)

			got := map[string]string{}
			for _, c := range cal.Components {
				if status, ok := supersession.Superseded(c, cal.Components); ok {
					got[c.UID()] = status
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("effective status mismatch\n got: %v\nwant: %v", got, want)
			}
		})
	}
}

// TestBehaviorFixtures_DurationParse replays duration/parse.json.
func TestBehaviorFixtures_DurationParse(t *testing.T) {
	path := filepath.Join(behaviorDir(t), "duration", "parse.json")
	var want []durationParseFixture
	loadJSON(t, path, &want)
	if len(want) == 0 {
		t.Fatal("duration/parse.json is empty")
	}

	// Rebuild the table from the committed inputs alone, so the
	// test proves the fixture's values are what the reference
	// reports rather than proving the generator is self-consistent.
	inputs := make([]string, 0, len(want))
	for _, f := range want {
		inputs = append(inputs, f.Value)
	}
	got := durationParseFixturesFor(inputs)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("duration parse mismatch\n got: %s\nwant: %s", mustJSON(t, got), mustJSON(t, want))
	}

	// Both outcome shapes must be exercised, or the table proves
	// only half the contract.
	var ok, failed int
	for _, f := range want {
		if f.Error != "" {
			failed++
		} else {
			ok++
		}
	}
	if ok == 0 || failed == 0 {
		t.Errorf("want both success and failure rows, got %d success / %d failure", ok, failed)
	}
}

// TestBehaviorFixtures_Trigger resolves every VALARM in each
// duration/<name>.ics and compares against <name>.trigger.json.
func TestBehaviorFixtures_Trigger(t *testing.T) {
	dir := filepath.Join(behaviorDir(t), "duration")
	for _, stem := range stems(t, dir, ".trigger.json") {
		t.Run(stem, func(t *testing.T) {
			cal := loadCalendar(t, filepath.Join(dir, stem+".ics"))
			var want []triggerFixture
			loadJSON(t, filepath.Join(dir, stem+".trigger.json"), &want)

			got := make([]triggerFixture, 0, len(want))
			for _, parent := range cal.Components {
				for _, alarm := range parent.Sub {
					if alarm.Type != vstar.CompAlarm {
						continue
					}
					got = append(got, resolveAlarm(alarm, parent, cal))
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("trigger mismatch\n got: %+v\nwant: %+v", got, want)
			}
		})
	}
}

// TestBehaviorFixtures_ExtScopes replays ext/scopes.json from its
// committed names.
func TestBehaviorFixtures_ExtScopes(t *testing.T) {
	path := filepath.Join(behaviorDir(t), "ext", "scopes.json")
	var want []scopeFixture
	loadJSON(t, path, &want)
	if len(want) < 12 {
		t.Fatalf("want at least 12 ext names, got %d", len(want))
	}

	got := make([]scopeFixture, 0, len(want))
	seenScopes := map[string]bool{}
	for _, f := range want {
		cur := scopeFixture{
			Name:  f.Name,
			Scope: strings.ToLower(ext.ScopeOf(f.Name).String()),
		}
		if sys, ok := ext.SystemName(f.Name); ok {
			cur.System = strPtr(sys)
		}
		got = append(got, cur)
		seenScopes[f.Scope] = true
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scope mismatch\n got: %s\nwant: %s", mustJSON(t, got), mustJSON(t, want))
	}
	for _, scope := range []string{"vstar", "system", "experimental", "none", "unknown"} {
		if !seenScopes[scope] {
			t.Errorf("no committed fixture covers scope %q", scope)
		}
	}
}

// TestBehaviorFixtures_TZID replays time/tzid.json against the
// conformance VTIMEZONE registries.
func TestBehaviorFixtures_TZID(t *testing.T) {
	root := behaviorDir(t)
	module, err := moduleRoot()
	if err != nil {
		t.Fatalf("moduleRoot: %v", err)
	}
	corpus := filepath.Join(filepath.Dir(module), specDirName, "v1.0", "conformance", "time")

	var want []tzidFixture
	loadJSON(t, filepath.Join(root, "time", "tzid.json"), &want)
	if len(want) == 0 {
		t.Fatal("time/tzid.json is empty")
	}

	cals := map[string]vstar.Calendar{}
	got := make([]tzidFixture, 0, len(want))
	var resolved, rejected int
	for _, f := range want {
		cal, ok := cals[f.Calendar]
		if !ok {
			cal = loadCalendar(t, filepath.Join(corpus, f.Calendar+".ics"))
			cals[f.Calendar] = cal
		}
		cur := tzidFixture{Calendar: f.Calendar, TZID: f.TZID, Value: f.Value}
		if at, ok := vstar.ParseTimeWithTZID(f.Value, f.TZID, cal); ok {
			cur.UTC = strPtr(vstar.FormatTime(at))
			resolved++
		} else {
			rejected++
		}
		got = append(got, cur)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tzid mismatch\n got: %s\nwant: %s", mustJSON(t, got), mustJSON(t, want))
	}
	if resolved == 0 || rejected == 0 {
		t.Errorf("want both resolved and rejected rows, got %d resolved / %d rejected", resolved, rejected)
	}
}

// TestBehaviorFixtures_TimestampsAreFormTwo asserts every timestamp
// a fixture states is RFC 5545 form #2 (UTC, Z-suffixed). The
// tzid family's `value` column is exempt: form #1 input is its
// whole subject.
func TestBehaviorFixtures_TimestampsAreFormTwo(t *testing.T) {
	root := behaviorDir(t)

	check := func(label, v string) {
		t.Helper()
		if v == "" {
			return
		}
		if _, ok := vstar.ParseTime(v); !ok {
			t.Errorf("%s: %q is not an RFC 5545 form #2 instant", label, v)
		}
	}

	dir := filepath.Join(root, "duration")
	for _, stem := range stems(t, dir, ".trigger.json") {
		var fs []triggerFixture
		loadJSON(t, filepath.Join(dir, stem+".trigger.json"), &fs)
		for _, f := range fs {
			check(stem+"/"+f.AlarmUID, f.FiresAt)
		}
	}

	var tz []tzidFixture
	loadJSON(t, filepath.Join(root, "time", "tzid.json"), &tz)
	for _, f := range tz {
		if f.UTC != nil {
			check("tzid/"+f.Value, *f.UTC)
		}
	}
}

// durationParseFixturesFor is durationParseFixtures restricted to a
// caller-supplied input list, so the consuming test can rebuild the
// table from the committed values.
func durationParseFixturesFor(values []string) []durationParseFixture {
	saved := durationParseValues
	durationParseValues = values
	defer func() { durationParseValues = saved }()
	return durationParseFixtures()
}

// mustJSON renders v for a failure message.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
