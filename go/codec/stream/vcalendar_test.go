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
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/codec/stream"
)

const minHeader = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n"

func calBytes(body string) []byte {
	var b bytes.Buffer
	b.WriteString(minHeader)
	b.WriteString(body)
	b.WriteString("END:VCALENDAR\r\n")
	return b.Bytes()
}

// TestVCalendarParser_EmptyCalendar — first Next on an empty (legal)
// calendar must return io.EOF.
func TestVCalendarParser_EmptyCalendar(t *testing.T) {
	t.Parallel()
	p := stream.NewVCalendarParser(bytes.NewReader(calBytes("")))
	_, err := p.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Next on empty calendar = %v, want io.EOF", err)
	}
}

// TestVCalendarParser_OneComponent — first Next returns the one
// VEVENT, second returns io.EOF.
func TestVCalendarParser_OneComponent(t *testing.T) {
	t.Parallel()
	body := "BEGIN:VEVENT\r\nUID:e1\r\nDTSTAMP:20260101T000000Z\r\nEND:VEVENT\r\n"
	p := stream.NewVCalendarParser(bytes.NewReader(calBytes(body)))

	c, err := p.Next()
	if err != nil {
		t.Fatalf("first Next: %v", err)
	}
	if c.Type != vstar.CompEvent {
		t.Fatalf("first Next type = %q, want VEVENT", c.Type)
	}
	if c.UID() != "e1" {
		t.Fatalf("first Next UID = %q, want e1", c.UID())
	}

	if _, err := p.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("second Next = %v, want io.EOF", err)
	}
}

// TestVCalendarParser_TenComponents — exactly N successive Next
// calls return components, then io.EOF.
func TestVCalendarParser_TenComponents(t *testing.T) {
	t.Parallel()
	const n = 10
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb,
			"BEGIN:VEVENT\r\nUID:e%d\r\nDTSTAMP:20260101T000000Z\r\nEND:VEVENT\r\n",
			i)
	}
	p := stream.NewVCalendarParser(bytes.NewReader(calBytes(sb.String())))

	for i := 0; i < n; i++ {
		c, err := p.Next()
		if err != nil {
			t.Fatalf("Next #%d: %v", i, err)
		}
		want := fmt.Sprintf("e%d", i)
		if c.UID() != want {
			t.Fatalf("Next #%d UID = %q, want %q", i, c.UID(), want)
		}
	}
	if _, err := p.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("trailing Next = %v, want io.EOF", err)
	}
}

// TestVCalendarParser_MalformedMidStream — a malformed component
// after some valid ones surfaces ErrMalformed; the previously
// returned components are unaffected.
func TestVCalendarParser_MalformedMidStream(t *testing.T) {
	t.Parallel()
	body := "BEGIN:VEVENT\r\nUID:good\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nbroken-line-no-colon\r\nEND:VEVENT\r\n"
	p := stream.NewVCalendarParser(bytes.NewReader(calBytes(body)))

	c, err := p.Next()
	if err != nil {
		t.Fatalf("first valid Next: %v", err)
	}
	if c.UID() != "good" {
		t.Fatalf("first UID = %q, want good", c.UID())
	}
	if _, err := p.Next(); !errors.Is(err, vstar.ErrMalformed) {
		t.Fatalf("malformed Next = %v, want ErrMalformed", err)
	}
}

// TestVCalendarParser_UnclosedBlock — input ending before
// END:VCALENDAR returns ErrUnclosedBlock.
func TestVCalendarParser_UnclosedBlock(t *testing.T) {
	t.Parallel()
	// Missing END:VCALENDAR.
	body := minHeader + "BEGIN:VEVENT\r\nUID:x\r\nEND:VEVENT\r\n"
	p := stream.NewVCalendarParser(strings.NewReader(body))

	if _, err := p.Next(); err != nil {
		t.Fatalf("first Next: %v", err)
	}
	if _, err := p.Next(); !errors.Is(err, vstar.ErrUnclosedBlock) {
		t.Fatalf("trailing Next = %v, want ErrUnclosedBlock", err)
	}
}

// TestVCalendarParser_UnsupportedVersion — VERSION:1.0 surfaces
// ErrUnsupportedVersion on first Next.
func TestVCalendarParser_UnsupportedVersion(t *testing.T) {
	t.Parallel()
	body := "BEGIN:VCALENDAR\r\nVERSION:1.0\r\nPRODID:-//x//EN\r\nEND:VCALENDAR\r\n"
	p := stream.NewVCalendarParser(strings.NewReader(body))
	_, err := p.Next()
	if !errors.Is(err, vstar.ErrUnsupportedVersion) {
		t.Fatalf("Next = %v, want ErrUnsupportedVersion", err)
	}
}

// TestVCalendarParser_HeaderProperties — Header() returns the
// captured PRODID before any Next call.
func TestVCalendarParser_HeaderProperties(t *testing.T) {
	t.Parallel()
	p := stream.NewVCalendarParser(bytes.NewReader(calBytes("")))
	h := p.Header()
	if h.ProdID != "-//test//EN" {
		t.Fatalf("Header().ProdID = %q, want -//test//EN", h.ProdID)
	}
	if len(h.Components) != 0 {
		t.Fatalf("Header().Components len = %d, want 0", len(h.Components))
	}
}

// TestVCalendarParser_EmptyInput — completely empty reader returns
// ErrMalformed (an empty stream is not a legal VCALENDAR).
func TestVCalendarParser_EmptyInput(t *testing.T) {
	t.Parallel()
	p := stream.NewVCalendarParser(strings.NewReader(""))
	if _, err := p.Next(); !errors.Is(err, vstar.ErrMalformed) {
		t.Fatalf("Next = %v, want ErrMalformed", err)
	}
}

// TestVCalendarEncoder_ZeroEncode — Close on a fresh encoder still
// emits a legal empty calendar (BEGIN/VERSION/PRODID/END).
func TestVCalendarEncoder_ZeroEncode(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCalendarEncoder(&buf)
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n",
		"VERSION:2.0\r\n",
		"END:VCALENDAR\r\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q; got:\n%s", want, got)
		}
	}
	// Round-trip via batch parser.
	if _, err := rfc5545.Parse(strings.NewReader(got)); err != nil {
		t.Fatalf("round-trip parse failed: %v", err)
	}
}

// TestVCalendarEncoder_SingleComponent — one Encode + Close yields a
// wrapped calendar.
func TestVCalendarEncoder_SingleComponent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCalendarEncoder(&buf)

	c := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "UID", Value: "x1"},
			{Name: "DTSTAMP", Value: "20260101T000000Z"},
		},
	}
	if err := enc.Encode(c); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cal, err := rfc5545.Parse(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if len(cal.Components) != 1 || cal.Components[0].UID() != "x1" {
		t.Fatalf("round-trip components = %+v; want [UID=x1]", cal.Components)
	}
}

// TestVCalendarEncoder_MultipleEncodes — n Encodes + Close emit n
// components in order.
func TestVCalendarEncoder_MultipleEncodes(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCalendarEncoder(&buf)
	for i := 0; i < 5; i++ {
		c := vstar.Component{
			Type:  vstar.CompEvent,
			Props: []vstar.Property{{Name: "UID", Value: fmt.Sprintf("e%d", i)}},
		}
		if err := enc.Encode(c); err != nil {
			t.Fatalf("Encode #%d: %v", i, err)
		}
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cal, err := rfc5545.Parse(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if len(cal.Components) != 5 {
		t.Fatalf("components len = %d, want 5", len(cal.Components))
	}
}

// TestVCalendarEncoder_DoubleClose — second Close returns
// ErrAlreadyClosed.
func TestVCalendarEncoder_DoubleClose(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCalendarEncoder(&buf)
	if err := enc.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := enc.Close(); !errors.Is(err, stream.ErrAlreadyClosed) {
		t.Fatalf("second Close = %v, want ErrAlreadyClosed", err)
	}
}

// TestVCalendarEncoder_EncodeAfterClose — Encode after Close returns
// ErrAlreadyClosed.
func TestVCalendarEncoder_EncodeAfterClose(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCalendarEncoder(&buf)
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	c := vstar.Component{Type: vstar.CompEvent}
	if err := enc.Encode(c); !errors.Is(err, stream.ErrAlreadyClosed) {
		t.Fatalf("Encode after Close = %v, want ErrAlreadyClosed", err)
	}
}

// TestVCalendarEncoder_SetHeader — setting custom PRODID before
// first Encode lands on the wire.
func TestVCalendarEncoder_SetHeader(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCalendarEncoder(&buf)
	if err := enc.SetHeader(vstar.Calendar{ProdID: "-//acme//Cal//EN"}); err != nil {
		t.Fatalf("SetHeader: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !strings.Contains(buf.String(), "PRODID:-//acme//Cal//EN") {
		t.Fatalf("custom PRODID missing; got:\n%s", buf.String())
	}
}

// TestVCalendarEncoder_SetHeaderAfterEncode — SetHeader after Encode
// returns ErrHeaderLocked.
func TestVCalendarEncoder_SetHeaderAfterEncode(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCalendarEncoder(&buf)
	if err := enc.Encode(vstar.Component{Type: vstar.CompEvent}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	err := enc.SetHeader(vstar.Calendar{ProdID: "x"})
	if !errors.Is(err, stream.ErrHeaderLocked) {
		t.Fatalf("SetHeader after Encode = %v, want ErrHeaderLocked", err)
	}
}

// TestVCalendarEncoder_EscapedPRODID — PRODID containing TEXT-special
// characters (',' ';' '\') is escaped on the wire and survives a
// round-trip parse.
func TestVCalendarEncoder_EscapedPRODID(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCalendarEncoder(&buf)
	if err := enc.SetHeader(vstar.Calendar{ProdID: `-//acme,inc;test\01//EN`}); err != nil {
		t.Fatalf("SetHeader: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	cal, err := rfc5545.Parse(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if cal.ProdID != `-//acme,inc;test\01//EN` {
		t.Fatalf("ProdID round-trip = %q, want literal", cal.ProdID)
	}
}

// TestVCalendarParser_NestedSubtree — a VEVENT with a nested VALARM
// produces one Component with one Sub.
func TestVCalendarParser_NestedSubtree(t *testing.T) {
	t.Parallel()
	body := "BEGIN:VEVENT\r\nUID:e1\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nEND:VALARM\r\nEND:VEVENT\r\n"
	p := stream.NewVCalendarParser(bytes.NewReader(calBytes(body)))
	c, err := p.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if len(c.Sub) != 1 || c.Sub[0].Type != vstar.CompAlarm {
		t.Fatalf("Sub = %+v, want one VALARM", c.Sub)
	}
}

// TestVCalendarParser_MismatchedEndInSubtree — a nested END that does
// not match its BEGIN surfaces ErrMalformed.
func TestVCalendarParser_MismatchedEndInSubtree(t *testing.T) {
	t.Parallel()
	body := "BEGIN:VEVENT\r\nUID:e1\r\nBEGIN:VALARM\r\nEND:VEVENT\r\n"
	p := stream.NewVCalendarParser(bytes.NewReader(calBytes(body)))
	if _, err := p.Next(); !errors.Is(err, vstar.ErrMalformed) {
		t.Fatalf("Next = %v, want ErrMalformed", err)
	}
}

// TestVCalendarRoundTrip_StreamToStream — encode N components via the
// streaming encoder, parse them back via the streaming parser, verify
// shape preservation.
func TestVCalendarRoundTrip_StreamToStream(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := stream.NewVCalendarEncoder(&buf)
	for i := 0; i < 3; i++ {
		if err := enc.Encode(vstar.Component{
			Type:  vstar.CompEvent,
			Props: []vstar.Property{{Name: "UID", Value: fmt.Sprintf("u%d", i)}},
		}); err != nil {
			t.Fatalf("Encode #%d: %v", i, err)
		}
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	p := stream.NewVCalendarParser(&buf)
	for i := 0; i < 3; i++ {
		c, err := p.Next()
		if err != nil {
			t.Fatalf("Next #%d: %v", i, err)
		}
		want := fmt.Sprintf("u%d", i)
		if c.UID() != want {
			t.Fatalf("Next #%d UID = %q, want %q", i, c.UID(), want)
		}
	}
	if _, err := p.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("trailing Next = %v, want io.EOF", err)
	}
}

// TestVCalendarParser_UnescapesTextLikeBatch pins that the streaming
// VCALENDAR parser and the batch rfc5545 parser decode identical bytes
// to identical values — both for component TEXT properties and for the
// calendar-level PRODID.
//
// The streaming parser called rfc5545.ParseContentLine and then, unlike
// the batch parser, never applied the RFC 5545 §3.3.11 TEXT unescape.
// It went unnoticed only because no committed .ics fixture contained a
// backslash. Unescaping now lives inside ParseContentLine so both
// callers get it by construction.
func TestVCalendarParser_UnescapesTextLikeBatch(t *testing.T) {
	t.Parallel()

	const src = "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//Acme\\, Inc.//V*\\; test//EN\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:evt-esc-1\r\n" +
		"DTSTAMP:20260101T000000Z\r\n" +
		"SUMMARY:Review\\, then ship\\; no excuses\r\n" +
		"DESCRIPTION:line one\\nline two\r\n" +
		"LOCATION:C:\\\\temp\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	batch, err := rfc5545.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("batch Parse: %v", err)
	}
	p := stream.NewVCalendarParser(strings.NewReader(src))
	comp, err := p.Next()
	if err != nil {
		t.Fatalf("stream Next: %v", err)
	}

	if got, want := p.Header().ProdID, batch.ProdID; got != want {
		t.Errorf("PRODID: stream %q, batch %q", got, want)
	}
	if got, want := p.Header().ProdID, `-//Acme, Inc.//V*; test//EN`; got != want {
		t.Errorf("PRODID decoded to %q, want %q", got, want)
	}

	want := map[string]string{
		"SUMMARY":     "Review, then ship; no excuses",
		"DESCRIPTION": "line one\nline two",
		"LOCATION":    `C:\temp`,
	}
	batchProps := map[string]string{}
	for _, prop := range batch.Components[0].Props {
		batchProps[prop.Name] = prop.Value
	}
	for _, prop := range comp.Props {
		if b, ok := batchProps[prop.Name]; ok && prop.Value != b {
			t.Errorf("%s: stream %q, batch %q", prop.Name, prop.Value, b)
		}
		if w, ok := want[prop.Name]; ok && prop.Value != w {
			t.Errorf("%s decoded to %q, want %q", prop.Name, prop.Value, w)
		}
	}
}

// TestVCalendarParser_LeavesNonTextVerbatim pins the other half of the
// contract: only TEXT-typed values are unescaped. A backslash inside a
// URI or a DATE-TIME is data, and unescaping it would corrupt the
// value — so the streaming and batch parsers must both pass it through.
func TestVCalendarParser_LeavesNonTextVerbatim(t *testing.T) {
	t.Parallel()

	const src = "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//test//EN\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:evt-uri-1\r\n" +
		"DTSTAMP:20260101T000000Z\r\n" +
		"ATTACH:https://example.com/a\\,b\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	batch, err := rfc5545.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("batch Parse: %v", err)
	}
	comp, err := stream.NewVCalendarParser(strings.NewReader(src)).Next()
	if err != nil {
		t.Fatalf("stream Next: %v", err)
	}

	const want = `https://example.com/a\,b`
	for _, got := range []struct {
		from string
		prop vstar.Component
	}{{"batch", batch.Components[0]}, {"stream", comp}} {
		v, ok := got.prop.Get("ATTACH")
		if !ok {
			t.Fatalf("%s: ATTACH missing", got.from)
		}
		if v.Value != want {
			t.Errorf("%s ATTACH = %q, want %q (non-TEXT must stay verbatim)", got.from, v.Value, want)
		}
	}
}
