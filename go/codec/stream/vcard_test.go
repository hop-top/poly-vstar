// SPDX-License-Identifier: MIT

package stream_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc6350"
	"hop.top/vstar/codec/stream"
)

func vcardWire(uid, fn string) string {
	return "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:" + uid + "\r\nFN:" + fn + "\r\nEND:VCARD\r\n"
}

// TestVCardParser_Empty — empty reader returns io.EOF immediately.
func TestVCardParser_Empty(t *testing.T) {
	t.Parallel()
	p := stream.NewVCardParser(strings.NewReader(""))
	if _, err := p.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("Next on empty = %v, want io.EOF", err)
	}
}

// TestVCardParser_OneCard — single card returned, then io.EOF.
func TestVCardParser_OneCard(t *testing.T) {
	t.Parallel()
	p := stream.NewVCardParser(strings.NewReader(vcardWire("u1", "Jad")))

	c, err := p.Next()
	if err != nil {
		t.Fatalf("first Next: %v", err)
	}
	if c.UID != "u1" {
		t.Fatalf("UID = %q, want u1", c.UID)
	}
	fn, ok := c.Get("FN")
	if !ok || fn.Value != "Jad" {
		t.Fatalf("FN = %v ok=%v, want Jad", fn, ok)
	}
	if _, err := p.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("second Next = %v, want io.EOF", err)
	}
}

// TestVCardParser_TenCards — ten consecutive blocks yield ten cards.
func TestVCardParser_TenCards(t *testing.T) {
	t.Parallel()
	const n = 10
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(vcardWire(fmt.Sprintf("u%d", i), fmt.Sprintf("Name%d", i)))
	}
	p := stream.NewVCardParser(strings.NewReader(sb.String()))

	for i := 0; i < n; i++ {
		c, err := p.Next()
		if err != nil {
			t.Fatalf("Next #%d: %v", i, err)
		}
		want := fmt.Sprintf("u%d", i)
		if c.UID != want {
			t.Fatalf("Next #%d UID = %q, want %q", i, c.UID, want)
		}
	}
	if _, err := p.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("trailing Next = %v, want io.EOF", err)
	}
}

// TestVCardParser_MissingVersion — missing VERSION → ErrMalformed.
func TestVCardParser_MissingVersion(t *testing.T) {
	t.Parallel()
	body := "BEGIN:VCARD\r\nUID:u1\r\nFN:x\r\nEND:VCARD\r\n"
	p := stream.NewVCardParser(strings.NewReader(body))
	if _, err := p.Next(); !errors.Is(err, vstar.ErrMalformed) {
		t.Fatalf("Next = %v, want ErrMalformed", err)
	}
}

// TestVCardParser_UnsupportedVersion — VERSION:3.0 → ErrUnsupportedVersion.
func TestVCardParser_UnsupportedVersion(t *testing.T) {
	t.Parallel()
	body := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:u1\r\nFN:x\r\nEND:VCARD\r\n"
	p := stream.NewVCardParser(strings.NewReader(body))
	if _, err := p.Next(); !errors.Is(err, vstar.ErrUnsupportedVersion) {
		t.Fatalf("Next = %v, want ErrUnsupportedVersion", err)
	}
}

// TestVCardParser_UnclosedBlock — EOF inside a VCARD → ErrUnclosedBlock.
func TestVCardParser_UnclosedBlock(t *testing.T) {
	t.Parallel()
	body := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u1\r\nFN:x\r\n"
	p := stream.NewVCardParser(strings.NewReader(body))
	if _, err := p.Next(); !errors.Is(err, vstar.ErrUnclosedBlock) {
		t.Fatalf("Next = %v, want ErrUnclosedBlock", err)
	}
}

// TestVCardParser_NestedBegin — nested BEGIN:VCARD → ErrMalformed.
func TestVCardParser_NestedBegin(t *testing.T) {
	t.Parallel()
	body := "BEGIN:VCARD\r\nVERSION:4.0\r\nBEGIN:VCARD\r\nEND:VCARD\r\nEND:VCARD\r\n"
	p := stream.NewVCardParser(strings.NewReader(body))
	if _, err := p.Next(); !errors.Is(err, vstar.ErrMalformed) {
		t.Fatalf("Next = %v, want ErrMalformed", err)
	}
}

// TestVCardParser_ContentOutsideBlock — non-empty content before
// BEGIN:VCARD → ErrMalformed.
func TestVCardParser_ContentOutsideBlock(t *testing.T) {
	t.Parallel()
	body := "FOO:bar\r\nBEGIN:VCARD\r\nVERSION:4.0\r\nUID:u1\r\nEND:VCARD\r\n"
	p := stream.NewVCardParser(strings.NewReader(body))
	if _, err := p.Next(); !errors.Is(err, vstar.ErrMalformed) {
		t.Fatalf("Next = %v, want ErrMalformed", err)
	}
}

// TestVCardEncoder_SingleCard — Encode + Close emits one VCARD block
// that round-trips via the batch parser.
func TestVCardEncoder_SingleCard(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCardEncoder(&buf)
	card := vstar.Card{
		UID:   "u1",
		Props: []vstar.Property{{Name: "FN", Value: "Jad"}},
	}
	if err := enc.Encode(card); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cards, err := rfc6350.Default.Parse(&buf)
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if len(cards) != 1 || cards[0].UID != "u1" {
		t.Fatalf("round-trip = %+v; want [UID=u1]", cards)
	}
}

// TestVCardEncoder_MultipleCards — n Encodes + Close emit n cards.
func TestVCardEncoder_MultipleCards(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCardEncoder(&buf)
	for i := 0; i < 4; i++ {
		card := vstar.Card{
			UID:   fmt.Sprintf("u%d", i),
			Props: []vstar.Property{{Name: "FN", Value: fmt.Sprintf("N%d", i)}},
		}
		if err := enc.Encode(card); err != nil {
			t.Fatalf("Encode #%d: %v", i, err)
		}
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	cards, err := rfc6350.Default.Parse(&buf)
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if len(cards) != 4 {
		t.Fatalf("cards len = %d, want 4", len(cards))
	}
}

// TestVCardEncoder_DoubleClose — second Close → ErrAlreadyClosed.
func TestVCardEncoder_DoubleClose(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCardEncoder(&buf)
	if err := enc.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := enc.Close(); !errors.Is(err, stream.ErrAlreadyClosed) {
		t.Fatalf("second Close = %v, want ErrAlreadyClosed", err)
	}
}

// TestVCardEncoder_EncodeAfterClose — Encode after Close → ErrAlreadyClosed.
func TestVCardEncoder_EncodeAfterClose(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCardEncoder(&buf)
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	err := enc.Encode(vstar.Card{UID: "x"})
	if !errors.Is(err, stream.ErrAlreadyClosed) {
		t.Fatalf("Encode after Close = %v, want ErrAlreadyClosed", err)
	}
}

// TestVCardParser_GroupPrefixedUID — the bare-name detection strips
// the optional "group." prefix so UID is captured even when wrapped.
func TestVCardParser_GroupPrefixedUID(t *testing.T) {
	t.Parallel()
	body := "BEGIN:VCARD\r\nVERSION:4.0\r\nitem1.UID:gp1\r\nFN:x\r\nEND:VCARD\r\n"
	p := stream.NewVCardParser(strings.NewReader(body))
	c, err := p.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if c.UID != "gp1" {
		t.Fatalf("UID = %q, want gp1", c.UID)
	}
}

// TestVCardRoundTrip_StreamToStream — encode N cards, stream-parse
// them back and verify UID preservation.
func TestVCardRoundTrip_StreamToStream(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCardEncoder(&buf)
	for i := 0; i < 3; i++ {
		if err := enc.Encode(vstar.Card{
			UID:   fmt.Sprintf("u%d", i),
			Props: []vstar.Property{{Name: "FN", Value: fmt.Sprintf("N%d", i)}},
		}); err != nil {
			t.Fatalf("Encode #%d: %v", i, err)
		}
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	p := stream.NewVCardParser(&buf)
	for i := 0; i < 3; i++ {
		c, err := p.Next()
		if err != nil {
			t.Fatalf("Next #%d: %v", i, err)
		}
		want := fmt.Sprintf("u%d", i)
		if c.UID != want {
			t.Fatalf("Next #%d UID = %q, want %q", i, c.UID, want)
		}
	}
	if _, err := p.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("trailing Next = %v, want io.EOF", err)
	}
}

// TestVCardParser_UnescapesTextLikeBatch pins that the streaming
// parser and the batch rfc6350 codec decode identical bytes to
// identical values.
//
// The streaming parser used to route vCard content lines through
// rfc5545.ParseContentLine, which applies no RFC 6350 §3.4 TEXT
// unescaping — so the very same file yielded FN="Last, Comma Test"
// through the batch codec and FN=`Last\, Comma Test` through the
// stream. The two paths are the same reference implementation and
// MUST NOT disagree.
func TestVCardParser_UnescapesTextLikeBatch(t *testing.T) {
	t.Parallel()

	const src = "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"UID:urn:uuid:esc-1\r\n" +
		"FN:Last\\, Comma Test\r\n" +
		"N:Last\\,Comma;Jad;;;\r\n" +
		"NOTE:line one\\nline two\r\n" +
		"X-BACKSLASH:a\\\\b\r\n" +
		"END:VCARD\r\n"

	batch, err := rfc6350.Default.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("batch Parse: %v", err)
	}
	streamed, err := stream.NewVCardParser(strings.NewReader(src)).Next()
	if err != nil {
		t.Fatalf("stream Next: %v", err)
	}

	if streamed.UID != batch[0].UID {
		t.Errorf("UID: stream %q, batch %q", streamed.UID, batch[0].UID)
	}
	if len(streamed.Props) != len(batch[0].Props) {
		t.Fatalf("prop count: stream %d, batch %d", len(streamed.Props), len(batch[0].Props))
	}
	for i := range batch[0].Props {
		if streamed.Props[i].Value != batch[0].Props[i].Value {
			t.Errorf("%s: stream %q, batch %q",
				batch[0].Props[i].Name, streamed.Props[i].Value, batch[0].Props[i].Value)
		}
	}

	// Spot-check the decoded values themselves, so a future change
	// that breaks BOTH paths identically still trips this test.
	want := map[string]string{
		"FN":          "Last, Comma Test",
		"N":           "Last,Comma;Jad;;;",
		"NOTE":        "line one\nline two",
		"X-BACKSLASH": `a\b`,
	}
	for _, p := range streamed.Props {
		if w, ok := want[p.Name]; ok && p.Value != w {
			t.Errorf("%s decoded to %q, want %q", p.Name, p.Value, w)
		}
	}
}

// TestVCardParser_PreservesGroupPrefix pins that the streaming parser
// keeps a vCard group prefix in Property.Name, matching the batch
// codec. rfc5545's parser has no notion of groups; it only happened to
// pass them through because it splits on the first colon.
func TestVCardParser_PreservesGroupPrefix(t *testing.T) {
	t.Parallel()

	const src = "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u\r\n" +
		"Home.TEL;TYPE=voice:tel:+15555550100\r\nEND:VCARD\r\n"

	batch, err := rfc6350.Default.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("batch Parse: %v", err)
	}
	streamed, err := stream.NewVCardParser(strings.NewReader(src)).Next()
	if err != nil {
		t.Fatalf("stream Next: %v", err)
	}
	if streamed.Props[0].Name != batch[0].Props[0].Name {
		t.Errorf("name: stream %q, batch %q", streamed.Props[0].Name, batch[0].Props[0].Name)
	}
	if streamed.Props[0].Name != "Home.TEL" {
		t.Errorf("group prefix lost: %q", streamed.Props[0].Name)
	}
}
