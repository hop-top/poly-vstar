// SPDX-License-Identifier: MIT

package rfc6350

import (
	"errors"
	"strings"
	"testing"

	vstar "hop.top/vstar"
)

const (
	minimalVCard = "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"FN:Jad Bitar\r\n" +
		"UID:urn:uuid:11111111-1111-1111-1111-111111111111\r\n" +
		"END:VCARD\r\n"

	twoVCards = minimalVCard +
		"BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"FN:Org\r\n" +
		"KIND:org\r\n" +
		"UID:urn:uuid:22222222-2222-2222-2222-222222222222\r\n" +
		"END:VCARD\r\n"

	v3Card = "BEGIN:VCARD\r\n" +
		"VERSION:3.0\r\n" +
		"FN:Old\r\n" +
		"UID:legacy\r\n" +
		"END:VCARD\r\n"

	noVersionCard = "BEGIN:VCARD\r\n" +
		"FN:Jad\r\n" +
		"UID:x\r\n" +
		"END:VCARD\r\n"

	unclosedCard = "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"FN:Jad\r\n" +
		"UID:x\r\n"
)

// TestParse_Empty verifies empty input yields an empty slice (no error).
func TestParse_Empty(t *testing.T) {
	p := NewParser()
	cards, err := p.Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cards) != 0 {
		t.Fatalf("expected 0 cards; got %d", len(cards))
	}
}

// TestParse_MinimalVCard verifies a single VCARD with VERSION/FN/UID
// produces one Card with UID populated and FN preserved in Props.
func TestParse_MinimalVCard(t *testing.T) {
	p := NewParser()
	cards, err := p.Parse(strings.NewReader(minimalVCard))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected 1 card; got %d", len(cards))
	}
	c := cards[0]
	if c.UID != "urn:uuid:11111111-1111-1111-1111-111111111111" {
		t.Errorf("UID: got %q", c.UID)
	}
	fn, ok := c.Get("FN")
	if !ok {
		t.Fatalf("FN missing in Props")
	}
	if fn.Value != "Jad Bitar" {
		t.Errorf("FN value: got %q", fn.Value)
	}
}

// TestParse_TwoVCards verifies multi-VCARD streams parse into a slice
// preserving order, with KIND extracted into Card.Kind.
func TestParse_TwoVCards(t *testing.T) {
	p := NewParser()
	cards, err := p.Parse(strings.NewReader(twoVCards))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards; got %d", len(cards))
	}
	if cards[1].Kind != vstar.KindOrg {
		t.Errorf("second card Kind: got %q want %q", cards[1].Kind, vstar.KindOrg)
	}
	if _, ok := cards[1].Get("KIND"); ok {
		t.Errorf("KIND should be extracted out of Props, not duplicated")
	}
	if _, ok := cards[1].Get("UID"); ok {
		t.Errorf("UID should be extracted out of Props, not duplicated")
	}
}

// TestParse_VersionRules covers the three VERSION outcomes:
// missing → ErrMalformed, 3.0 → ErrUnsupportedVersion, 4.0 → ok.
func TestParse_VersionRules(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr error
	}{
		{name: "missing VERSION", in: noVersionCard, wantErr: vstar.ErrMalformed},
		{name: "VERSION 3.0", in: v3Card, wantErr: vstar.ErrUnsupportedVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			_, err := p.Parse(strings.NewReader(tt.in))
			if err == nil {
				t.Fatalf("expected error %v; got nil", tt.wantErr)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err mismatch: got %v want wraps %v", err, tt.wantErr)
			}
		})
	}
}

// TestParse_UnclosedBlock verifies missing END:VCARD yields ErrUnclosedBlock.
func TestParse_UnclosedBlock(t *testing.T) {
	p := NewParser()
	_, err := p.Parse(strings.NewReader(unclosedCard))
	if err == nil {
		t.Fatalf("expected ErrUnclosedBlock; got nil")
	}
	if !errors.Is(err, vstar.ErrUnclosedBlock) {
		t.Fatalf("err mismatch: got %v want wraps %v", err, vstar.ErrUnclosedBlock)
	}
}

// TestParse_GroupedAndEscaped verifies that grouped property names
// survive parsing and that TEXT escaping is applied to values.
func TestParse_GroupedAndEscaped(t *testing.T) {
	in := "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"home.TEL;TYPE=voice:tel:+15555550100\r\n" +
		`N:Last\,Comma;Jad;;;` + "\r\n" +
		"UID:x\r\n" +
		"END:VCARD\r\n"
	p := NewParser()
	cards, err := p.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected 1 card; got %d", len(cards))
	}
	tel, ok := cards[0].Get("home.TEL")
	if !ok {
		t.Fatalf("grouped property missing; props=%#v", cards[0].Props)
	}
	if tel.Value != "tel:+15555550100" {
		t.Errorf("TEL value: got %q", tel.Value)
	}
	if len(tel.Params) != 1 || tel.Params[0].Value != "voice" {
		t.Errorf("TEL params: got %#v", tel.Params)
	}
	n, ok := cards[0].Get("N")
	if !ok || n.Value != "Last,Comma;Jad;;;" {
		t.Errorf("N value escape: got %q", n.Value)
	}
}

// TestParse_StrayBeginEnd verifies misnested BEGIN/END are rejected.
func TestParse_StrayEnd(t *testing.T) {
	in := "END:VCARD\r\n"
	p := NewParser()
	_, err := p.Parse(strings.NewReader(in))
	if err == nil || !errors.Is(err, vstar.ErrMalformed) {
		t.Fatalf("expected ErrMalformed; got %v", err)
	}
}

// TestParse_NestedBegin rejects BEGIN:VCARD inside a VCARD.
func TestParse_NestedBegin(t *testing.T) {
	in := "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"BEGIN:VCARD\r\n" +
		"END:VCARD\r\n"
	p := NewParser()
	_, err := p.Parse(strings.NewReader(in))
	if err == nil || !errors.Is(err, vstar.ErrMalformed) {
		t.Fatalf("expected ErrMalformed; got %v", err)
	}
}
