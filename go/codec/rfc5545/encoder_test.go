// SPDX-License-Identifier: MIT

package rfc5545

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	vstar "hop.top/vstar"
)

// encodeStr returns Encode(cal) as a string for table-test brevity.
func encodeStr(t *testing.T, cal vstar.Calendar) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Encode(&buf, cal); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return buf.String()
}

func TestEncode_EmptyCalendar(t *testing.T) {
	t.Parallel()

	cal := vstar.Calendar{ProdID: "-//V*//Empty//EN"}
	got := encodeStr(t, cal)
	want := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//V*//Empty//EN\r\n" +
		"END:VCALENDAR\r\n"
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestEncode_OneVTODO(t *testing.T) {
	t.Parallel()

	cal := vstar.Calendar{
		ProdID: "-//V*//Test//EN",
		Components: []vstar.Component{
			{
				Type: vstar.CompTodo,
				Props: []vstar.Property{
					{Name: "UID", Value: "abc-123"},
					{Name: "DTSTAMP", Value: "20260504T120000Z"},
					{Name: "SUMMARY", Value: "Buy milk"},
				},
			},
		},
	}
	got := encodeStr(t, cal)
	want := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//V*//Test//EN\r\n" +
		"BEGIN:VTODO\r\n" +
		"UID:abc-123\r\n" +
		"DTSTAMP:20260504T120000Z\r\n" +
		"SUMMARY:Buy milk\r\n" +
		"END:VTODO\r\n" +
		"END:VCALENDAR\r\n"
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestEncode_LongValueIsFolded(t *testing.T) {
	t.Parallel()

	// Build a property whose serialized line is longer than 75 octets.
	// Name is 6 octets ("X-LONG"), colon is 1, so 75 - 7 = 68 octets fit
	// on the first physical line; the remainder folds.
	value := strings.Repeat("a", 100)
	cal := vstar.Calendar{
		ProdID: "x",
		Components: []vstar.Component{
			{
				Type:  vstar.CompTodo,
				Props: []vstar.Property{{Name: "X-LONG", Value: value}},
			},
		},
	}
	out := encodeStr(t, cal)

	// Verify CRLF + space sequence appears at fold point and that no
	// physical line exceeds 75 octets (excluding terminator).
	for _, phys := range strings.Split(out, "\r\n") {
		if len(phys) > 75 {
			t.Errorf("physical line exceeds 75 octets (%d): %q", len(phys), phys)
		}
	}
	// Round-trip: Parse(Encode(cal)) should recover the original value.
	round, err := Parse(strings.NewReader(out))
	if err != nil {
		t.Fatalf("round-trip Parse: %v", err)
	}
	if v, _ := round.Components[0].Get("X-LONG"); v.Value != value {
		t.Errorf("round-trip value len=%d, want %d", len(v.Value), len(value))
	}
}

func TestEncode_NestedVTIMEZONE(t *testing.T) {
	t.Parallel()

	cal := vstar.Calendar{
		ProdID: "-//V*//TZ//EN",
		Components: []vstar.Component{
			{
				Type:  vstar.CompTimezone,
				Props: []vstar.Property{{Name: "TZID", Value: "America/Toronto"}},
				Sub: []vstar.Component{
					{
						Type: "STANDARD",
						Props: []vstar.Property{
							{Name: "DTSTART", Value: "19701101T020000"},
							{Name: "TZOFFSETFROM", Value: "-0400"},
							{Name: "TZOFFSETTO", Value: "-0500"},
						},
					},
				},
			},
		},
	}
	got := encodeStr(t, cal)
	if !strings.Contains(got, "BEGIN:VTIMEZONE\r\nTZID:America/Toronto\r\nBEGIN:STANDARD\r\n") {
		t.Errorf("nested BEGIN block missing in:\n%s", got)
	}
	if !strings.Contains(got, "END:STANDARD\r\nEND:VTIMEZONE\r\n") {
		t.Errorf("nested END block missing in:\n%s", got)
	}
}

func TestEncode_ParameterQuotingNeeded(t *testing.T) {
	t.Parallel()

	// Param value containing comma → needs DQUOTE wrapping.
	cal := vstar.Calendar{
		ProdID: "x",
		Components: []vstar.Component{
			{
				Type: vstar.CompTodo,
				Props: []vstar.Property{
					{
						Name:   "ATTENDEE",
						Params: []vstar.Param{{Name: "CN", Value: "Doe, John"}},
						Value:  "mailto:john@example.com",
					},
				},
			},
		},
	}
	got := encodeStr(t, cal)
	if !strings.Contains(got, `ATTENDEE;CN="Doe, John":mailto:john@example.com`) {
		t.Errorf("expected quoted CN param in:\n%s", got)
	}
}

func TestEncode_ParamValueWithInnerQuoteIsStripped(t *testing.T) {
	t.Parallel()

	cal := vstar.Calendar{
		ProdID: "x",
		Components: []vstar.Component{
			{
				Type: vstar.CompTodo,
				Props: []vstar.Property{
					{
						Name:   "ATTENDEE",
						Params: []vstar.Param{{Name: "CN", Value: `quote"in"value`}},
						Value:  "mailto:x@example.com",
					},
				},
			},
		},
	}
	got := encodeStr(t, cal)
	if strings.Contains(got, `"`) {
		// We expect the bare CN with quotes stripped, no DQUOTE wrap
		// (no comma/semicolon/colon left after stripping).
		if !strings.Contains(got, "ATTENDEE;CN=quoteinvalue:") {
			t.Errorf("inner DQUOTE not handled in:\n%s", got)
		}
	}
}

func TestEncode_LongValueExtremelyLong(t *testing.T) {
	t.Parallel()

	// 300-octet value forces multiple continuation chunks.
	value := strings.Repeat("z", 300)
	cal := vstar.Calendar{
		ProdID: "x",
		Components: []vstar.Component{
			{Type: vstar.CompTodo, Props: []vstar.Property{{Name: "X-Z", Value: value}}},
		},
	}
	out := encodeStr(t, cal)
	for _, phys := range strings.Split(out, "\r\n") {
		if len(phys) > 75 {
			t.Errorf("physical line exceeds 75 octets (%d): %q", len(phys), phys)
		}
	}
	round, err := Parse(strings.NewReader(out))
	if err != nil {
		t.Fatalf("round-trip Parse: %v", err)
	}
	if v, _ := round.Components[0].Get("X-Z"); v.Value != value {
		t.Errorf("round-trip value len=%d, want %d", len(v.Value), len(value))
	}
}

func TestEncode_RoundTrip_SemanticEquality(t *testing.T) {
	t.Parallel()

	// Conformance fixture round-trip: parse → encode → parse → assert
	// semantically equal trees (component types, prop counts, prop
	// values). Property order is preserved on both passes.
	src := strings.Join([]string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//V*//RT//EN",
		"BEGIN:VEVENT",
		"UID:rt-1",
		"DTSTAMP:20260504T120000Z",
		"DTSTART:20260601T090000Z",
		"SUMMARY:Round-trip",
		"BEGIN:VALARM",
		"ACTION:DISPLAY",
		"TRIGGER:-PT15M",
		"END:VALARM",
		"END:VEVENT",
		"END:VCALENDAR",
	}, "\r\n") + "\r\n"

	cal1, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	out := encodeStr(t, cal1)
	cal2, err := Parse(strings.NewReader(out))
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if !calendarsSemanticallyEqual(cal1, cal2) {
		t.Errorf("trees diverged:\ncal1=%+v\ncal2=%+v", cal1, cal2)
	}
}

// makeCalWithProp returns a Calendar carrying a single VTODO with
// one Property set, used by the TEXT-escape unit tests.
func makeCalWithProp(t *testing.T, p vstar.Property) string {
	t.Helper()
	cal := vstar.Calendar{
		ProdID: "x",
		Components: []vstar.Component{
			{Type: vstar.CompTodo, Props: []vstar.Property{p}},
		},
	}
	return encodeStr(t, cal)
}

func TestEncoder_EscapesBackslash(t *testing.T) {
	t.Parallel()

	out := makeCalWithProp(t, vstar.Property{Name: "SUMMARY", Value: `back\slash`})
	if !strings.Contains(out, `SUMMARY:back\\slash`) {
		t.Errorf("expected SUMMARY:back\\\\slash in output, got:\n%s", out)
	}
}

func TestEncoder_EscapesSemicolon(t *testing.T) {
	t.Parallel()

	out := makeCalWithProp(t, vstar.Property{Name: "SUMMARY", Value: "a;b"})
	if !strings.Contains(out, `SUMMARY:a\;b`) {
		t.Errorf("expected SUMMARY:a\\;b in output, got:\n%s", out)
	}
}

func TestEncoder_EscapesComma(t *testing.T) {
	t.Parallel()

	out := makeCalWithProp(t, vstar.Property{Name: "SUMMARY", Value: "a,b"})
	if !strings.Contains(out, `SUMMARY:a\,b`) {
		t.Errorf("expected SUMMARY:a\\,b in output, got:\n%s", out)
	}
}

func TestEncoder_EscapesNewline(t *testing.T) {
	t.Parallel()

	out := makeCalWithProp(t, vstar.Property{Name: "DESCRIPTION", Value: "line1\nline2"})
	if !strings.Contains(out, `DESCRIPTION:line1\nline2`) {
		t.Errorf("expected DESCRIPTION:line1\\nline2 in output, got:\n%s", out)
	}
}

func TestEncoder_EscapesCRLF(t *testing.T) {
	t.Parallel()

	out := makeCalWithProp(t, vstar.Property{Name: "DESCRIPTION", Value: "line1\r\nline2"})
	// CRLF in source collapses to a single escaped "\n" — CR is dropped,
	// LF becomes literal "\n" on the wire.
	if !strings.Contains(out, `DESCRIPTION:line1\nline2`) {
		t.Errorf("expected DESCRIPTION:line1\\nline2 in output, got:\n%s", out)
	}
	if strings.Contains(out, "DESCRIPTION:line1\r") {
		t.Errorf("unexpected raw CR in DESCRIPTION value, got:\n%s", out)
	}
}

func TestEncoder_NoEscapeNonText(t *testing.T) {
	t.Parallel()

	// DTSTART is DATE-TIME, NOT TEXT — value must pass through verbatim.
	// Stuffing in commas/semis (which are not legal for DATE-TIME but
	// are useful as escape canaries) MUST NOT trigger backslash escapes.
	out := makeCalWithProp(t, vstar.Property{Name: "DTSTART", Value: "20260504T120000Z;,X"})
	if !strings.Contains(out, "DTSTART:20260504T120000Z;,X") {
		t.Errorf("expected DTSTART value verbatim (no escape), got:\n%s", out)
	}
	if strings.Contains(out, `DTSTART:20260504T120000Z\;`) {
		t.Errorf("DTSTART value was escaped — non-TEXT properties must not be escaped, got:\n%s", out)
	}
}

func TestEncoder_RoundtripWithEscapedText(t *testing.T) {
	t.Parallel()

	// A calendar carrying TEXT properties with characters that require
	// §3.3.11 escape on emit. Parser unescapes on read; encoder
	// re-escapes on write — the wire bytes must round-trip identically.
	//
	// Raw string literals are used for property values so backslash
	// escape sequences read literally (e.g. `\n` is two chars: '\\'
	// + 'n') exactly as they appear on the wire.
	src := joinCRLF(
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//V*//Esc//EN",
		"BEGIN:VTODO",
		"UID:rt-esc-1",
		"DTSTAMP:20260504T120000Z",
		`SUMMARY:back\\slash and \, comma and \; semi`,
		`DESCRIPTION:line1\nline2`,
		"END:VTODO",
		"END:VCALENDAR",
	)
	cal, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := encodeStr(t, cal)
	if out != src {
		t.Fatalf("roundtrip bytes differ.\n got:\n%s\nwant:\n%s", out, src)
	}
}

// errWriter returns errFakeWrite from every Write after limit bytes.
type errWriter struct {
	limit   int
	written int
}

var errFakeWrite = errors.New("rfc5545_test: forced write error")

func (w *errWriter) Write(p []byte) (int, error) {
	w.written += len(p)
	if w.written > w.limit {
		return 0, errFakeWrite
	}
	return len(p), nil
}

func TestEncode_WriterError(t *testing.T) {
	t.Parallel()

	cal := vstar.Calendar{
		ProdID: strings.Repeat("p", 8000),
		Components: []vstar.Component{
			{
				Type:  vstar.CompTodo,
				Props: []vstar.Property{{Name: "X-LONG", Value: strings.Repeat("v", 8000)}},
			},
		},
	}
	// limit = 1: every WriteString past the first will error.
	w := &errWriter{limit: 1}
	if err := Encode(w, cal); err == nil {
		t.Fatalf("expected error from failing writer, got nil")
	}
}

// calendarsSemanticallyEqual compares two Calendars by ProdID,
// component count + types, recursive sub-component shape, and
// property-by-property semantic equality (vstar.Equal).
func calendarsSemanticallyEqual(a, b vstar.Calendar) bool {
	if a.ProdID != b.ProdID {
		return false
	}
	return componentsSemanticallyEqual(a.Components, b.Components)
}

func componentsSemanticallyEqual(a, b []vstar.Component) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type {
			return false
		}
		if len(a[i].Props) != len(b[i].Props) {
			return false
		}
		for j := range a[i].Props {
			if !vstar.Equal(a[i].Props[j], b[i].Props[j]) {
				return false
			}
		}
		if !componentsSemanticallyEqual(a[i].Sub, b[i].Sub) {
			return false
		}
	}
	return true
}
