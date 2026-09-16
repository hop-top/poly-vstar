// SPDX-License-Identifier: MIT

package ext

import (
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vstar "hop.top/vstar"
)

func TestIsExtension(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"upper X- prefix", "X-FOO", true},
		{"lower x- prefix", "x-foo", true},
		{"mixed case", "X-AGR-INTENT", true},
		{"plain DTSTART", "DTSTART", false},
		{"plain FOO", "FOO", false},
		{"empty string", "", false},
		{"single X", "X", false},
		{"X-", "X-", true}, // X- prefix is present; classification is ScopeUnknown.
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsExtension(tc.in)
			if got != tc.want {
				t.Fatalf("IsExtension(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestScopeOf(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Scope
	}{
		{"VStar canonical", "X-VSTAR-HASH", ScopeVStar},
		{"VStar lower", "x-vstar-hash", ScopeVStar},
		{"System AGR", "X-AGR-INTENT", ScopeSystem},
		{"System lower agr", "x-agr-intent", ScopeSystem},
		{"experimental upper", "X-EXP-FOO", ScopeExperimental},
		{"experimental lower", "x-exp-foo", ScopeExperimental},
		{"None DTSTART", "DTSTART", ScopeNone},
		{"None empty", "", ScopeNone},
		{"None X", "X", ScopeNone},
		{"Unknown bare X-", "X-", ScopeUnknown},
		{"Unknown VSTAR no name", "X-VSTAR-", ScopeUnknown},
		{"Unknown EXP no name", "X-EXP-", ScopeUnknown},
		{"Unknown system no name", "X-AGR-", ScopeUnknown},
		{"Unknown X-FOO no system suffix", "X-FOO", ScopeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ScopeOf(tc.in)
			if got != tc.want {
				t.Fatalf("ScopeOf(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSystemName(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantSlug string
		wantOK   bool
	}{
		{"AGR upper", "X-AGR-INTENT", "AGR", true},
		{"AGR lower", "x-agr-intent", "AGR", true},
		{"AGR mixed segments", "X-AGR-EFFECTIVE-STATUS", "AGR", true},
		{"crm system", "X-CRM-DEAL-STAGE", "CRM", true},
		{"VStar excluded", "X-VSTAR-HASH", "", false},
		{"Experimental excluded", "X-EXP-FOO", "", false},
		{"VStar lower excluded", "x-vstar-hash", "", false},
		{"non-extension", "DTSTART", "", false},
		{"empty", "", "", false},
		{"X- bare", "X-", "", false},
		{"X-AGR- no name", "X-AGR-", "", false},
		{"X-FOO no name", "X-FOO", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotSlug, gotOK := SystemName(tc.in)
			if gotSlug != tc.wantSlug || gotOK != tc.wantOK {
				t.Fatalf("SystemName(%q) = (%q, %v), want (%q, %v)",
					tc.in, gotSlug, gotOK, tc.wantSlug, tc.wantOK)
			}
		})
	}
}

func TestExtensionsByScope(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "UID", Value: "u@example"},
			{Name: "DTSTART", Value: "20260504T000000Z"},
			{Name: "X-VSTAR-HASH", Value: "sha256:abc"},
			{Name: "X-AGR-INTENT", Value: "schedule"},
			{Name: "X-AGR-ROLE", Value: "host"},
			{Name: "X-CRM-DEAL", Value: "open"},
			{Name: "X-EXP-FOO", Value: "bar"},
			{Name: "X-", Value: "malformed"},
			{Name: "X-AGR-", Value: "malformed slug"},
		},
	}

	t.Run("VStar", func(t *testing.T) {
		got := ExtensionsByScope(c, ScopeVStar)
		if len(got) != 1 || got[0].Name != "X-VSTAR-HASH" {
			t.Fatalf("ScopeVStar = %+v, want one X-VSTAR-HASH", got)
		}
	})

	t.Run("System", func(t *testing.T) {
		got := ExtensionsByScope(c, ScopeSystem)
		if len(got) != 3 {
			t.Fatalf("ScopeSystem len = %d, want 3 (got %+v)", len(got), got)
		}
		wantNames := map[string]bool{
			"X-AGR-INTENT": true,
			"X-AGR-ROLE":   true,
			"X-CRM-DEAL":   true,
		}
		for _, p := range got {
			if !wantNames[p.Name] {
				t.Fatalf("unexpected property in ScopeSystem: %q", p.Name)
			}
		}
	})

	t.Run("Experimental", func(t *testing.T) {
		got := ExtensionsByScope(c, ScopeExperimental)
		if len(got) != 1 || got[0].Name != "X-EXP-FOO" {
			t.Fatalf("ScopeExperimental = %+v, want one X-EXP-FOO", got)
		}
	})

	t.Run("Unknown", func(t *testing.T) {
		got := ExtensionsByScope(c, ScopeUnknown)
		if len(got) != 2 {
			t.Fatalf("ScopeUnknown len = %d, want 2 (got %+v)", len(got), got)
		}
	})

	t.Run("None matches plain props", func(t *testing.T) {
		got := ExtensionsByScope(c, ScopeNone)
		if len(got) != 2 {
			t.Fatalf("ScopeNone len = %d, want 2 (UID + DTSTART; got %+v)",
				len(got), got)
		}
	})

	t.Run("empty component returns nil", func(t *testing.T) {
		empty := vstar.Component{Type: vstar.CompEvent}
		got := ExtensionsByScope(empty, ScopeSystem)
		if got != nil {
			t.Fatalf("empty component = %+v, want nil", got)
		}
	})
}

func TestScopeString(t *testing.T) {
	cases := []struct {
		s    Scope
		want string
	}{
		{ScopeNone, "None"},
		{ScopeVStar, "VStar"},
		{ScopeSystem, "System"},
		{ScopeExperimental, "Experimental"},
		{ScopeUnknown, "Unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.s.String(); got != tc.want {
				t.Fatalf("Scope(%d).String() = %q, want %q", tc.s, got, tc.want)
			}
		})
	}
}

// TestGodocReferencesSpec04 enforces that the package and the
// public symbols carry godoc that links back to spec/04. Future
// edits that drop these references will fail this test loudly,
// which is the whole point — see plan T-0053.
func TestGodocReferencesSpec04(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatal("no source files found in package dir")
	}
	docPkg, err := doc.NewFromFiles(fset, files, "hop.top/vstar/ext")
	if err != nil {
		t.Fatalf("doc.NewFromFiles: %v", err)
	}

	if !strings.Contains(docPkg.Doc, "spec/04") {
		t.Errorf("package godoc missing spec/04 reference:\n%s", docPkg.Doc)
	}

	// Functions whose godoc must mention spec/04.
	wantFuncs := []string{"IsExtension", "ScopeOf", "SystemName", "ExtensionsByScope"}
	gotFuncs := map[string]string{}
	for _, fn := range docPkg.Funcs {
		gotFuncs[fn.Name] = fn.Doc
	}
	// ScopeOf is a constructor of Scope so it appears under the type.
	for _, typ := range docPkg.Types {
		for _, fn := range typ.Funcs {
			gotFuncs[fn.Name] = fn.Doc
		}
		for _, m := range typ.Methods {
			gotFuncs[typ.Name+"."+m.Name] = m.Doc
		}
		if typ.Name == "Scope" {
			if !strings.Contains(typ.Doc, "spec/04") {
				t.Errorf("Scope type godoc missing spec/04 reference:\n%s", typ.Doc)
			}
			if !strings.Contains(typ.Doc, "X-EXP") || !strings.Contains(typ.Doc, "X-VSTAR") {
				t.Errorf("Scope type godoc missing promotion path (X-EXP-* → X-VSTAR-*):\n%s", typ.Doc)
			}
		}
	}
	for _, name := range wantFuncs {
		d, ok := gotFuncs[name]
		if !ok {
			t.Errorf("public function %s missing godoc", name)
			continue
		}
		if d == "" {
			t.Errorf("public function %s has empty godoc", name)
		}
	}
}
