// SPDX-License-Identifier: MIT

package rfc6350

import (
	"errors"
	"strings"
	"testing"

	vstar "hop.top/vstar"
)

// TestUnfold_Smoke is the rfc6350-side smoke test for the shared
// content-line scanner; the full behavior matrix lives in
// codec/internal/contentline/scanner_test.go.
func TestUnfold_Smoke(t *testing.T) {
	t.Parallel()

	got, err := unfold(strings.NewReader("FN:Jad B\r\n itar\r\nN:Bitar;Jad;;;\r\n"))
	if err != nil {
		t.Fatalf("unfold: %v", err)
	}
	want := []string{"FN:Jad Bitar", "N:Bitar;Jad;;;"}
	if len(got) != len(want) {
		t.Fatalf("got %d want %d: %q", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, got[i], want[i])
		}
	}
}

// TestParseContentLine verifies that vCard content lines decompose into
// (group, name, params, value) per RFC 6350 §3.3 / §3.4. Group prefix
// is preserved verbatim on the Property.Name (e.g. "home.TEL"); escapes
// in TEXT-typed values are unfolded (\, \; \n).
func TestParseContentLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantName  string
		wantVal   string
		wantParam map[string]string
		wantErr   error
	}{
		{
			name:     "plain FN",
			line:     "FN:Jad Bitar",
			wantName: "FN",
			wantVal:  "Jad Bitar",
		},
		{
			name:     "grouped TEL",
			line:     "home.TEL:tel:+15555550100",
			wantName: "home.TEL",
			wantVal:  "tel:+15555550100",
		},
		{
			name:      "TEL with TYPE param",
			line:      "TEL;TYPE=voice:tel:+15555550100",
			wantName:  "TEL",
			wantVal:   "tel:+15555550100",
			wantParam: map[string]string{"TYPE": "voice"},
		},
		{
			name:      "param with quoted value containing colon",
			line:      `EMAIL;TYPE="work,home":jad@example.com`,
			wantName:  "EMAIL",
			wantVal:   "jad@example.com",
			wantParam: map[string]string{"TYPE": "work,home"},
		},
		{
			name:     "N with escaped comma",
			line:     `N:Bitar\,Jr;Jad;;;`,
			wantName: "N",
			wantVal:  "Bitar,Jr;Jad;;;",
		},
		{
			name:     "value with escaped newline",
			line:     `NOTE:line one\nline two`,
			wantName: "NOTE",
			wantVal:  "line one\nline two",
		},
		{
			name:     "value with escaped semicolon",
			line:     `NOTE:semi\;here`,
			wantName: "NOTE",
			wantVal:  "semi;here",
		},
		{
			name:     "value with escaped backslash",
			line:     `NOTE:back\\slash`,
			wantName: "NOTE",
			wantVal:  `back\slash`,
		},
		{
			name:    "missing colon",
			line:    "FN value with no colon",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "empty name",
			line:    ":value",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "empty input",
			line:    "",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "param without equals",
			line:    "TEL;TYPE:value",
			wantErr: vstar.ErrMalformed,
		},
		{
			name:    "unterminated quoted param",
			line:    `EMAIL;TYPE="work:value`,
			wantErr: vstar.ErrMalformed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prop, err := parseContentLine(tt.line)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v; got prop=%#v", tt.wantErr, prop)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err mismatch: got %v want wraps %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseContentLine: %v", err)
			}
			if prop.Name != tt.wantName {
				t.Errorf("Name: got %q want %q", prop.Name, tt.wantName)
			}
			if prop.Value != tt.wantVal {
				t.Errorf("Value: got %q want %q", prop.Value, tt.wantVal)
			}
			if len(prop.Params) != len(tt.wantParam) {
				t.Errorf("param count: got %d want %d (%#v)", len(prop.Params), len(tt.wantParam), prop.Params)
			}
			for _, p := range prop.Params {
				want, ok := tt.wantParam[p.Name]
				if !ok {
					t.Errorf("unexpected param %s=%q", p.Name, p.Value)
					continue
				}
				if want != p.Value {
					t.Errorf("param %s: got %q want %q", p.Name, p.Value, want)
				}
			}
		})
	}
}

// TestParseContentLine_GroupCasePreserved verifies the group prefix is
// preserved with original case on read per RFC 6350 §3.3 (group names
// are case-insensitive but we round-trip the wire form for fidelity).
func TestParseContentLine_GroupCasePreserved(t *testing.T) {
	prop, err := parseContentLine("Home.tel:tel:+15555550100")
	if err != nil {
		t.Fatalf("parseContentLine: %v", err)
	}
	if prop.Name != "Home.tel" {
		t.Errorf("group/name preserved: got %q want %q", prop.Name, "Home.tel")
	}
}
