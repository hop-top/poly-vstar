// SPDX-License-Identifier: MIT

package rfc6350

import (
	"bytes"
	"strings"
	"testing"

	vstar "hop.top/vstar"
)

// TestNew verifies New returns a non-nil Codec satisfying both Parser
// and Encoder behavior for vstar.Card.
func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New returned nil")
	}

	in := "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"UID:u\r\n" +
		"FN:Jad\r\n" +
		"END:VCARD\r\n"
	cards, err := c.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cards) != 1 || cards[0].UID != "u" {
		t.Fatalf("unexpected parse: %#v", cards)
	}

	var buf bytes.Buffer
	if err := c.Encode(&buf, cards[0]); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(buf.String(), "BEGIN:VCARD\r\n") {
		t.Errorf("encode framing: got %q", buf.String())
	}
}

// TestDefault verifies the package-level Default Codec is non-nil
// and round-trips identically to a freshly-created instance.
func TestDefault(t *testing.T) {
	if Default == nil {
		t.Fatal("Default is nil")
	}
	in := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u\r\nFN:Jad\r\nEND:VCARD\r\n"
	a, err := Default.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Default.Parse: %v", err)
	}
	b, err := New().Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("New().Parse: %v", err)
	}
	if len(a) != 1 || len(b) != 1 || a[0].UID != b[0].UID {
		t.Errorf("Default vs New diverge: %#v vs %#v", a, b)
	}
}

// TestCodec_InterfaceShape compile-asserts that *codec satisfies the
// package's local Parser, Encoder, and Codec interfaces.
func TestCodec_InterfaceShape(t *testing.T) {
	t.Helper()
	// New() returns Codec; assigning to the Parser/Encoder interface
	// vars asserts the embedded interfaces are reachable via the
	// returned value (compile-only — passes if it builds).
	var (
		_ Parser  = New()
		_ Encoder = New()
	)
	// Static-only check — passes if compile succeeds.
	_ = vstar.Card{}
}
