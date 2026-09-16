// SPDX-License-Identifier: MIT

package rfc6350

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	vstar "hop.top/vstar"
)

// TestEncode_MissingUID verifies that a Card with empty UID is rejected
// per the plan's T3 RED rule (no placeholder).
func TestEncode_MissingUID(t *testing.T) {
	enc := NewEncoder()
	var buf bytes.Buffer
	err := enc.Encode(&buf, vstar.Card{Props: []vstar.Property{{Name: "FN", Value: "Jad"}}})
	if err == nil {
		t.Fatalf("expected ErrMissingUID; got nil")
	}
	if !errors.Is(err, vstar.ErrMissingUID) {
		t.Fatalf("err mismatch: got %v want wraps %v", err, vstar.ErrMissingUID)
	}
}

// TestEncode_MinimalVCard checks the framing: BEGIN, VERSION:4.0, UID,
// other props, END — all CRLF-terminated, property names uppercased.
func TestEncode_MinimalVCard(t *testing.T) {
	enc := NewEncoder()
	var buf bytes.Buffer
	err := enc.Encode(&buf, vstar.Card{
		UID: "urn:uuid:1",
		Props: []vstar.Property{
			{Name: "fn", Value: "Jad Bitar"},
		},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"UID:urn:uuid:1\r\n" +
		"FN:Jad Bitar\r\n" +
		"END:VCARD\r\n"
	if got := buf.String(); got != want {
		t.Errorf("encoded mismatch\n got=%q\nwant=%q", got, want)
	}
}

// TestEncode_KindEmittedFirst verifies KIND, when set, is emitted
// after VERSION+UID and before other properties (RFC 6350 §6.1.4 is
// silent on order; we lock it for determinism).
func TestEncode_KindEmittedFirst(t *testing.T) {
	enc := NewEncoder()
	var buf bytes.Buffer
	err := enc.Encode(&buf, vstar.Card{
		UID:  "u",
		Kind: vstar.KindOrg,
		Props: []vstar.Property{
			{Name: "FN", Value: "ACME"},
		},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"UID:u\r\n" +
		"KIND:org\r\n" +
		"FN:ACME\r\n" +
		"END:VCARD\r\n"
	if got := buf.String(); got != want {
		t.Errorf("mismatch\n got=%q\nwant=%q", got, want)
	}
}

// TestEncode_EscapesText verifies TEXT escaping is applied on encode
// for ',' ';' '\n' '\\' inside property values.
func TestEncode_EscapesText(t *testing.T) {
	enc := NewEncoder()
	var buf bytes.Buffer
	err := enc.Encode(&buf, vstar.Card{
		UID: "u",
		Props: []vstar.Property{
			{Name: "N", Value: "Last,Comma;Jad;;;"},
			{Name: "NOTE", Value: "line\nbreak"},
		},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got := buf.String()
	// Note: N's '\;' is part of structured-value semantics in RFC
	// 6350, but at the codec layer we treat all property values as
	// TEXT; consumers needing structured-value semantics live above.
	if !strings.Contains(got, `N:Last\,Comma\;Jad\;\;\;`+"\r\n") {
		t.Errorf("N escape: got=%q", got)
	}
	if !strings.Contains(got, `NOTE:line\nbreak`+"\r\n") {
		t.Errorf("NOTE escape: got=%q", got)
	}
}

// TestEncode_ParamSerialization verifies parameters are written as
// NAME;P1=V1;P2=V2:value with DQUOTE'ing when the value contains
// reserved chars (',' ':' ';').
func TestEncode_ParamSerialization(t *testing.T) {
	enc := NewEncoder()
	var buf bytes.Buffer
	err := enc.Encode(&buf, vstar.Card{
		UID: "u",
		Props: []vstar.Property{
			{
				Name:   "EMAIL",
				Value:  "jad@example.com",
				Params: []vstar.Param{{Name: "TYPE", Value: "work,home"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(buf.String(), `EMAIL;TYPE="work,home":jad@example.com`+"\r\n") {
		t.Errorf("EMAIL param: got=%q", buf.String())
	}
}

// TestEncode_PreservesGroupPrefix verifies group prefixes survive
// encode (lowercase preserved; only the *property name* segment is
// uppercased per RFC 6350 §3.3 conventions).
func TestEncode_PreservesGroupPrefix(t *testing.T) {
	enc := NewEncoder()
	var buf bytes.Buffer
	err := enc.Encode(&buf, vstar.Card{
		UID: "u",
		Props: []vstar.Property{
			{Name: "home.tel", Value: "tel:+15555550100"},
		},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(buf.String(), "home.TEL:tel:+15555550100\r\n") {
		t.Errorf("group preserve + name upper: got=%q", buf.String())
	}
}

// TestEncode_Folds verifies CRLF + SP folding at 75 octets per RFC
// 6350 §3.2 (which references RFC 5545 §3.1).
//
// The value is long enough to need THREE continuation lines. At 200
// octets it needed only one, which is why this test stayed green while
// the encoder emitted a 76-octet second continuation: the bug only
// showed from the second continuation onward.
func TestEncode_Folds(t *testing.T) {
	long := strings.Repeat("a", 400)
	enc := NewEncoder()
	var buf bytes.Buffer
	err := enc.Encode(&buf, vstar.Card{
		UID: "u",
		Props: []vstar.Property{
			{Name: "ADR", Value: long},
		},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got := buf.String()
	// Each non-first physical line in the fold begins with "\r\n " (SP).
	// Confirm at least two folds (200 + name overhead → ≥3 physical lines).
	if strings.Count(got, "\r\n ") < 2 {
		t.Errorf("expected fold markers; got=%q", got)
	}
	// No physical line >75 octets (count between CRLFs).
	for i, line := range strings.Split(got, "\r\n") {
		if len(line) > 75 {
			t.Errorf("line %d exceeds 75 octets (%d): %q", i, len(line), line)
		}
	}
}

// physicalLines splits encoder output into its physical lines,
// dropping the trailing empty element left by the final CRLF.
func physicalLines(wire string) []string {
	return strings.Split(strings.TrimSuffix(wire, "\r\n"), "\r\n")
}

// TestEncode_FoldOctetBudget pins the exact octet budget of every
// physical line: the first chunk of a folded logical line carries a
// full 75 octets, and every continuation carries a leading SP plus 74
// octets of payload, for 75 total.
//
// The encoder used to take 75 payload octets at the top of each loop
// iteration and 74 inside the same iteration, then prefix BOTH with
// the continuation SP — so every second continuation went out at 76
// octets, over the RFC 5545 §3.1 limit. Asserting the counts (rather
// than only "no line is too long") also catches the opposite mistake
// of folding early and wasting the budget.
func TestEncode_FoldOctetBudget(t *testing.T) {
	var buf bytes.Buffer
	err := NewEncoder().Encode(&buf, vstar.Card{
		UID:   "u",
		Props: []vstar.Property{{Name: "NOTE", Value: strings.Repeat("A", 300)}},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// BEGIN:VCARD / VERSION:4.0 / UID:u then the folded NOTE then
	// END:VCARD. "NOTE:" + 300 A's = 305 octets of logical line:
	// 75 on the first physical line, then 74 payload octets on each
	// of three continuations (75 + 74*3 = 297) and 8 on the last,
	// each continuation costing one further octet for its leading SP
	// — so 75, 75, 75, then 9.
	want := []int{11, 11, 5, 75, 75, 75, 75, 9, 9}
	var got []int
	for _, line := range physicalLines(buf.String()) {
		got = append(got, len(line))
	}
	if len(got) != len(want) {
		t.Fatalf("physical line count = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("physical line %d = %d octets, want %d (all lines: %v)", i, got[i], want[i], got)
		}
	}
}

// TestEncode_FoldSplitsMultiByteRune pins that folding counts OCTETS,
// not runes: a UTF-8 sequence straddling the 75-octet boundary IS
// split across the fold, per spec/v1.0/03-canonicalization.md rule 3.
//
// This is the guard against "fixing" the 76-octet overflow by
// retreating the cut to a rune boundary — that would keep every line
// under 75 while silently changing the wire format.
func TestEncode_FoldSplitsMultiByteRune(t *testing.T) {
	// "NOTE:" is 5 octets, so 69 filler octets leave the logical line
	// at 74 and put the 75-octet boundary exactly in the middle of
	// the next 2-octet "é".
	value := strings.Repeat("a", 69) + strings.Repeat("é", 10)
	var buf bytes.Buffer
	err := NewEncoder().Encode(&buf, vstar.Card{
		UID:   "u",
		Props: []vstar.Property{{Name: "NOTE", Value: value}},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	lines := physicalLines(buf.String())
	folded := lines[3]
	if len(folded) != 75 {
		t.Fatalf("first physical line of the folded value = %d octets, want 75", len(folded))
	}
	// The line must end on the LEAD octet of "é" (0xC3) with its
	// continuation octet (0xA9) carried to the next physical line.
	if folded[len(folded)-1] != 0xC3 {
		t.Errorf("last octet of the fold = %#x, want 0xC3 (the é lead octet, split mid-rune)", folded[len(folded)-1])
	}
	cont := lines[4]
	if cont[0] != ' ' {
		t.Fatalf("continuation line does not start with SP: %q", cont)
	}
	if cont[1] != 0xA9 {
		t.Errorf("first payload octet of the continuation = %#x, want 0xA9 (the é trail octet)", cont[1])
	}

	// Unfolding must reassemble the original bytes exactly.
	cards, err := Default.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cards[0].Props[0].Value != value {
		t.Errorf("round-trip value mismatch:\n got %q\nwant %q", cards[0].Props[0].Value, value)
	}
}

// TestRoundTrip_FullCard parse → encode → parse should round-trip to
// a semantically equal Card.
func TestRoundTrip_FullCard(t *testing.T) {
	in := "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"UID:urn:uuid:abc\r\n" +
		"KIND:individual\r\n" +
		"FN:Jad Bitar\r\n" +
		"home.TEL;TYPE=voice:tel:+15555550100\r\n" +
		`N:Last\,Comma;Jad;;;` + "\r\n" +
		"END:VCARD\r\n"

	p := NewParser()
	cards, err := p.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse(in): %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected 1 card; got %d", len(cards))
	}

	enc := NewEncoder()
	var buf bytes.Buffer
	if err := enc.Encode(&buf, cards[0]); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	cards2, err := p.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse(out): %v\nwire=%q", err, buf.String())
	}
	if len(cards2) != 1 {
		t.Fatalf("expected 1 card on round-trip; got %d", len(cards2))
	}

	if !cardsSemEqual(cards[0], cards2[0]) {
		t.Errorf("round-trip mismatch\noriginal=%#v\nround-trip=%#v", cards[0], cards2[0])
	}
}

// cardsSemEqual returns true when two Cards carry the same UID, Kind,
// and property set (Property.Equal applied to each pair, in order-
// independent fashion via Get).
func cardsSemEqual(a, b vstar.Card) bool {
	if a.UID != b.UID || a.Kind != b.Kind {
		return false
	}
	if len(a.Props) != len(b.Props) {
		return false
	}
	for _, ap := range a.Props {
		bp, ok := b.Get(ap.Name)
		if !ok {
			return false
		}
		if !vstar.Equal(ap, bp) {
			return false
		}
	}
	return true
}

// TestEncode_DoesNotCloseWriter verifies Encode is composable with
// streaming writers — it must not call Close.
type noCloseWriter struct {
	bytes.Buffer
	closed bool
}

func (w *noCloseWriter) Close() error {
	w.closed = true
	return nil
}

func TestEncode_DoesNotCloseWriter(t *testing.T) {
	enc := NewEncoder()
	w := &noCloseWriter{}
	err := enc.Encode(w, vstar.Card{
		UID:   "u",
		Props: []vstar.Property{{Name: "FN", Value: "x"}},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if w.closed {
		t.Errorf("Encode must not call Close")
	}
}
