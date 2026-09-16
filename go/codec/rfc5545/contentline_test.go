// SPDX-License-Identifier: MIT

package rfc5545

import (
	"errors"
	"testing"

	vstar "hop.top/vstar"
)

func paramsEqual(a, b []vstar.Param) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParseContentLine_Plain(t *testing.T) {
	t.Parallel()

	got, err := ParseContentLine("VERSION:2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "VERSION" {
		t.Errorf("Name = %q, want %q", got.Name, "VERSION")
	}
	if got.Value != "2.0" {
		t.Errorf("Value = %q, want %q", got.Value, "2.0")
	}
	if len(got.Params) != 0 {
		t.Errorf("Params = %v, want empty", got.Params)
	}
}

func TestParseContentLine_OneParam(t *testing.T) {
	t.Parallel()

	got, err := ParseContentLine("ATTENDEE;CN=Jad:mailto:jad@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "ATTENDEE" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.Value != "mailto:jad@example.com" {
		t.Errorf("Value = %q (colons inside value preserved)", got.Value)
	}
	want := []vstar.Param{{Name: "CN", Value: "Jad"}}
	if !paramsEqual(got.Params, want) {
		t.Errorf("Params = %v, want %v", got.Params, want)
	}
}

func TestParseContentLine_QuotedParam(t *testing.T) {
	t.Parallel()

	// DQUOTE-wrapped param value containing comma, semicolon, colon.
	got, err := ParseContentLine(`X-FOO;P="hello, world; and: more":value`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "X-FOO" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.Value != "value" {
		t.Errorf("Value = %q", got.Value)
	}
	if len(got.Params) != 1 {
		t.Fatalf("Params len = %d, want 1", len(got.Params))
	}
	if got.Params[0].Name != "P" || got.Params[0].Value != "hello, world; and: more" {
		t.Errorf("Param = %+v, want {P, hello, world; and: more}", got.Params[0])
	}
}

func TestParseContentLine_MultipleParams(t *testing.T) {
	t.Parallel()

	got, err := ParseContentLine("ATTENDEE;CN=Jad;ROLE=CHAIR:mailto:jad@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []vstar.Param{
		{Name: "CN", Value: "Jad"},
		{Name: "ROLE", Value: "CHAIR"},
	}
	if !paramsEqual(got.Params, want) {
		t.Errorf("Params = %v, want %v", got.Params, want)
	}
}

func TestParseContentLine_CaseInsensitiveNames(t *testing.T) {
	t.Parallel()

	// Names + param names: case-insensitive on parse — the model's
	// Get() does case-insensitive lookup, but we preserve the wire
	// case so the encoder can round-trip if desired.
	got, err := ParseContentLine("version;foo=bar:2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "version" {
		t.Errorf("Name = %q (preserved verbatim)", got.Name)
	}
	if got.Params[0].Name != "foo" {
		t.Errorf("Param name = %q (preserved verbatim)", got.Params[0].Name)
	}
}

func TestParseContentLine_EmptyValue(t *testing.T) {
	t.Parallel()

	// Empty value after colon is permitted by the grammar.
	got, err := ParseContentLine("X-EMPTY:")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "X-EMPTY" || got.Value != "" {
		t.Errorf("got %+v", got)
	}
}

func TestParseContentLine_Malformed_NoColon(t *testing.T) {
	t.Parallel()

	_, err := ParseContentLine("NOCOLONHERE")
	if !errors.Is(err, vstar.ErrMalformed) {
		t.Errorf("err = %v, want wraps ErrMalformed", err)
	}
}

func TestParseContentLine_Malformed_UnbalancedQuote(t *testing.T) {
	t.Parallel()

	_, err := ParseContentLine(`X-FOO;P="unclosed:value`)
	if !errors.Is(err, vstar.ErrMalformed) {
		t.Errorf("err = %v, want wraps ErrMalformed", err)
	}
}

func TestParseContentLine_Malformed_EmptyName(t *testing.T) {
	t.Parallel()

	_, err := ParseContentLine(":value")
	if !errors.Is(err, vstar.ErrMalformed) {
		t.Errorf("err = %v, want wraps ErrMalformed", err)
	}
}

func TestParseContentLine_Malformed_ParamNoEquals(t *testing.T) {
	t.Parallel()

	_, err := ParseContentLine("X-FOO;PNOEQ:value")
	if !errors.Is(err, vstar.ErrMalformed) {
		t.Errorf("err = %v, want wraps ErrMalformed", err)
	}
}
