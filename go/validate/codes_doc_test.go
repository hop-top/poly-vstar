// SPDX-License-Identifier: MIT

package validate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"hop.top/vstar/validate"
)

// registryCode is one entry of spec/registry/diagnostic-codes.json.
type registryCode struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Section  string `json:"section"`
	Rule     string `json:"rule"`
}

// registryProperties is spec/registry/standard-properties.json: the
// RFC allow-list, split by the RFC that defines each name.
type registryProperties map[string][]string

// TestRegistry_CodesMatchPackage asserts set equality between the
// diagnostic codes this package emits and the codes the registry
// declares, in both directions, with matching severities.
//
// The registry is the source of truth: go/validate/codes_gen.go is
// rendered from it by tools/registry/gen.py. This test is the runtime
// half of that contract — `make registry-check` catches a stale
// generated file, and this catches a registry that disagrees with the
// package it generated (a hand-edit to codes_gen.go, or a severity
// changed in one place only).
//
// Equality, not containment: a code in the JSON that the package never
// emits is as much a defect as an undeclared code. The first ships a
// promise no implementation keeps; the second ships a diagnostic no
// consumer can look up.
func TestRegistry_CodesMatchPackage(t *testing.T) {
	entries := loadRegistryCodes(t)

	registry := make(map[string]registryCode, len(entries))
	for _, e := range entries {
		if prev, dup := registry[e.Code]; dup {
			t.Errorf("registry declares %s twice (%q and %q)", e.Code, prev.Rule, e.Rule)
		}
		registry[e.Code] = e
	}

	pkg := validate.Codes()
	if !sort.StringsAreSorted(pkg) {
		t.Errorf("validate.Codes() is not sorted: %v", pkg)
	}

	seen := make(map[string]bool, len(pkg))
	for _, code := range pkg {
		if seen[code] {
			t.Errorf("validate.Codes() lists %s twice", code)
		}
		seen[code] = true

		entry, ok := registry[code]
		if !ok {
			t.Errorf("code %s is emitted by the package but absent from spec/registry/diagnostic-codes.json", code)
			continue
		}

		got, known := validate.SeverityOf(code)
		if !known {
			t.Errorf("code %s has no severity in the package", code)
			continue
		}
		if want := severityFromRegistry(t, entry); got != want {
			t.Errorf("code %s severity: package has %v, registry has %q", code, got, entry.Severity)
		}
		if entry.Rule == "" {
			t.Errorf("code %s has an empty rule in the registry", code)
		}
		if entry.Section == "" {
			t.Errorf("code %s has an empty section in the registry", code)
		}
	}

	for code := range registry {
		if !seen[code] {
			t.Errorf("code %s is declared in spec/registry/diagnostic-codes.json but the package never emits it", code)
		}
	}

	if len(pkg) != len(registry) {
		t.Errorf("code count: package has %d, registry has %d", len(pkg), len(registry))
	}
}

// TestRegistry_StandardPropertyCount asserts the generated allow-list
// carries exactly the names the registry lists, across both RFCs.
//
// The count is the cheap invariant; the duplicate check below is the
// one that matters, because a name listed under both RFCs would make
// the Go map smaller than the JSON without either file looking wrong
// on its own.
func TestRegistry_StandardPropertyCount(t *testing.T) {
	groups := loadRegistryProperties(t)

	unique := map[string]string{}
	total := 0
	for rfc, names := range groups {
		for _, name := range names {
			total++
			if other, dup := unique[name]; dup {
				t.Errorf("property %s is listed under both %s and %s", name, other, rfc)
			}
			unique[name] = rfc
		}
	}

	if got := validate.StandardPropertyCount(); got != len(unique) {
		t.Errorf("StandardPropertyCount() = %d, registry has %d unique names", got, len(unique))
	}
	if total != len(unique) {
		t.Errorf("registry lists %d names but only %d are unique", total, len(unique))
	}
}

// severityFromRegistry maps a registry severity string onto the
// package's Severity value, failing the test on an unknown spelling.
func severityFromRegistry(t *testing.T, e registryCode) validate.Severity {
	t.Helper()
	switch e.Severity {
	case "error":
		return validate.SeverityError
	case "warning":
		return validate.SeverityWarning
	default:
		t.Fatalf("code %s has unknown severity %q in the registry; expected error or warning", e.Code, e.Severity)
		return validate.SeverityError
	}
}

func loadRegistryCodes(t *testing.T) []registryCode {
	t.Helper()
	var out []registryCode
	decodeRegistry(t, "diagnostic-codes.json", &out)
	if len(out) == 0 {
		t.Fatal("spec/registry/diagnostic-codes.json is empty; the test cannot meaningfully assert")
	}
	return out
}

func loadRegistryProperties(t *testing.T) registryProperties {
	t.Helper()
	out := registryProperties{}
	decodeRegistry(t, "standard-properties.json", &out)
	if len(out) == 0 {
		t.Fatal("spec/registry/standard-properties.json is empty; the test cannot meaningfully assert")
	}
	return out
}

// decodeRegistry reads one spec/registry file into dst.
//
// The registry lives at the monorepo root, two levels above this
// package. The hop-top/vstar mirror republishes only the Go module, so
// a checkout with no spec/registry above the package skips the gate
// instead of failing it — same contract the docs catalog gate had.
func decodeRegistry(t *testing.T, name string, dst any) {
	t.Helper()
	path, ok := findRegistryFile(t, name)
	if !ok {
		t.Skipf("spec/registry/%s not found above this package; the registry gate runs in the poly-vstar monorepo", name)
	}
	// Caching caveat: go test's build cache tracks files a test opens
	// under the module directory, and the registry sits above it. So
	// editing only spec/registry/ leaves a previously passing result
	// cached, and this gate does not re-run until something in the
	// module changes (or -count=1 is passed). Verified empirically, not
	// assumed.
	//
	// That is why `make registry-check` — not this test — is the
	// authoritative drift gate: it re-renders every output on every run
	// and has no cache. This test covers the other direction, catching a
	// generated file hand-edited out of step with the registry, which
	// does dirty the module and so does invalidate the cache.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}

// findRegistryFile walks up from the working directory looking for
// spec/registry/<name>, stopping at the repository boundary (the first
// directory holding a .git entry) or the filesystem root.
func findRegistryFile(t *testing.T, name string) (string, bool) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, "spec", "registry", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return "", false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
