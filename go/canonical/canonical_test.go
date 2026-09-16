// SPDX-License-Identifier: MIT

package canonical_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/canonical"
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/codec/rfc6350"
)

// helperVTODO returns a hand-rolled simple VTODO Component used by
// multiple goldens.
func helperVTODO() vstar.Component {
	return vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "abc-123"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "SUMMARY", Value: "Buy milk"},
			{Name: "PRIORITY", Value: "3"},
		},
	}
}

func expectedVTODOCanonical() []byte {
	return []byte("BEGIN:VTODO\r\n" +
		"DTSTAMP:20260504T120000Z\r\n" +
		"PRIORITY:3\r\n" +
		"SUMMARY:Buy milk\r\n" +
		"UID:abc-123\r\n" +
		"END:VTODO\r\n")
}

func TestComponent_SimpleVTODO(t *testing.T) {
	got := canonical.Component(helperVTODO())
	want := expectedVTODOCanonical()
	if !bytes.Equal(got, want) {
		t.Fatalf("Component canonical mismatch.\n got:  %q\n want: %q", got, want)
	}
}

func TestComponent_PropertyOrderInputDoesNotMatter(t *testing.T) {
	a := helperVTODO()
	b := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "PRIORITY", Value: "3"},
			{Name: "SUMMARY", Value: "Buy milk"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "UID", Value: "abc-123"},
		},
	}
	if !bytes.Equal(canonical.Component(a), canonical.Component(b)) {
		t.Fatalf("property order on input must not affect canonical bytes")
	}
}

func TestComponent_ParameterOrderInputDoesNotMatter(t *testing.T) {
	a := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "UID", Value: "evt-1"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{
				Name: "ATTENDEE",
				Params: []vstar.Param{
					{Name: "CN", Value: "Jad"},
					{Name: "RSVP", Value: "TRUE"},
				},
				Value: "mailto:jad@example.com",
			},
		},
	}
	b := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{
				Name: "ATTENDEE",
				Params: []vstar.Param{
					{Name: "RSVP", Value: "TRUE"},
					{Name: "CN", Value: "Jad"},
				},
				Value: "mailto:jad@example.com",
			},
			{Name: "UID", Value: "evt-1"},
		},
	}
	if !bytes.Equal(canonical.Component(a), canonical.Component(b)) {
		t.Fatalf("param order on input must not affect canonical bytes")
	}
}

func TestComponent_StripsXVSTARHASH(t *testing.T) {
	withHash := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "abc-123"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "SUMMARY", Value: "Buy milk"},
			{Name: "X-VSTAR-HASH", Value: "sha256:deadbeef"},
		},
	}
	withoutHash := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "abc-123"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "SUMMARY", Value: "Buy milk"},
		},
	}
	if !bytes.Equal(canonical.Component(withHash), canonical.Component(withoutHash)) {
		t.Fatalf("X-VSTAR-HASH must be stripped from canonical form")
	}
}

func TestComponent_StripsXVSTARHASH_DoesNotMutateInput(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "abc-123"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "X-VSTAR-HASH", Value: "sha256:deadbeef"},
		},
	}
	beforeLen := len(c.Props)
	_ = canonical.Component(c)
	if len(c.Props) != beforeLen {
		t.Fatalf("Component mutated input Props (was %d, now %d)", beforeLen, len(c.Props))
	}
	if _, ok := c.Get("X-VSTAR-HASH"); !ok {
		t.Fatalf("Component mutated input — X-VSTAR-HASH gone from caller's Component")
	}
}

func TestComponent_NFCComposedAndDecomposed(t *testing.T) {
	composed := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "x"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "SUMMARY", Value: "café"},
		},
	}
	decomposed := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "x"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "SUMMARY", Value: "café"},
		},
	}
	if !bytes.Equal(canonical.Component(composed), canonical.Component(decomposed)) {
		t.Fatalf("composed and decomposed inputs must canonicalize to the same bytes")
	}
}

func TestComponent_NFCParameterValues(t *testing.T) {
	a := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "UID", Value: "x"},
			{
				Name:   "ATTENDEE",
				Params: []vstar.Param{{Name: "CN", Value: "café"}},
				Value:  "mailto:x@example.com",
			},
		},
	}
	b := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "UID", Value: "x"},
			{
				Name:   "ATTENDEE",
				Params: []vstar.Param{{Name: "CN", Value: "café"}},
				Value:  "mailto:x@example.com",
			},
		},
	}
	if !bytes.Equal(canonical.Component(a), canonical.Component(b)) {
		t.Fatalf("NFC must apply to parameter values")
	}
}

func TestComponent_DeterministicAcross100Runs(t *testing.T) {
	c := helperVTODO()
	first := canonical.Component(c)
	for i := 0; i < 100; i++ {
		got := canonical.Component(c)
		if !bytes.Equal(got, first) {
			t.Fatalf("Component canonical not deterministic at iteration %d", i)
		}
	}
}

func TestComponent_RecursesSubComponents(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "UID", Value: "evt-1"},
			{Name: "DTSTART", Value: "20260601T090000Z"},
			{Name: "DTEND", Value: "20260601T100000Z"},
			{Name: "SUMMARY", Value: "Standup"},
		},
		Sub: []vstar.Component{
			{
				Type: vstar.CompAlarm,
				Props: []vstar.Property{
					{Name: "TRIGGER", Value: "-PT15M"},
					{Name: "ACTION", Value: "DISPLAY"},
					{Name: "DESCRIPTION", Value: "Standup in 15 minutes"},
				},
			},
		},
	}
	want := []byte(
		"BEGIN:VEVENT\r\n" +
			"DTEND:20260601T100000Z\r\n" +
			"DTSTAMP:20260504T120000Z\r\n" +
			"DTSTART:20260601T090000Z\r\n" +
			"SUMMARY:Standup\r\n" +
			"UID:evt-1\r\n" +
			"BEGIN:VALARM\r\n" +
			"ACTION:DISPLAY\r\n" +
			"DESCRIPTION:Standup in 15 minutes\r\n" +
			"TRIGGER:-PT15M\r\n" +
			"END:VALARM\r\n" +
			"END:VEVENT\r\n",
	)
	got := canonical.Component(c)
	if !bytes.Equal(got, want) {
		t.Fatalf("recursive canonical mismatch.\n got:  %q\n want: %q", got, want)
	}
}

func TestComponent_ATTACHStripsValueBinary(t *testing.T) {
	binary := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "UID", Value: "x"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{
				Name: "ATTACH",
				Params: []vstar.Param{
					{Name: "VALUE", Value: "BINARY"},
					{Name: "ENCODING", Value: "BASE64"},
				},
				Value: "https://example.com/doc.pdf",
			},
		},
	}
	uri := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "UID", Value: "x"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "ATTACH", Value: "https://example.com/doc.pdf"},
		},
	}
	if !bytes.Equal(canonical.Component(binary), canonical.Component(uri)) {
		t.Fatalf("ATTACH with VALUE=BINARY/ENCODING=BASE64 should strip to URI form")
	}
}

func TestComponent_VTimezoneNestedPreservesAppendOrder(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompTimezone,
		Props: []vstar.Property{
			{Name: "TZID", Value: "America/Toronto"},
		},
		Sub: []vstar.Component{
			{
				Type: "STANDARD",
				Props: []vstar.Property{
					{Name: "DTSTART", Value: "19701101T020000"},
					{Name: "TZOFFSETFROM", Value: "-0400"},
					{Name: "TZOFFSETTO", Value: "-0500"},
					{Name: "TZNAME", Value: "EST"},
				},
			},
			{
				Type: "DAYLIGHT",
				Props: []vstar.Property{
					{Name: "DTSTART", Value: "19700308T020000"},
					{Name: "TZOFFSETFROM", Value: "-0500"},
					{Name: "TZOFFSETTO", Value: "-0400"},
					{Name: "TZNAME", Value: "EDT"},
				},
			},
		},
	}
	got := canonical.Component(c)
	stdIdx := bytes.Index(got, []byte("BEGIN:STANDARD"))
	dlIdx := bytes.Index(got, []byte("BEGIN:DAYLIGHT"))
	if stdIdx < 0 || dlIdx < 0 {
		t.Fatalf("missing STANDARD or DAYLIGHT block in canonical output")
	}
	if stdIdx > dlIdx {
		t.Fatalf("STANDARD should appear before DAYLIGHT (append order); got STANDARD at %d, DAYLIGHT at %d", stdIdx, dlIdx)
	}
}

func TestCalendar_Empty(t *testing.T) {
	cal := vstar.Calendar{ProdID: "-//V*//Empty//EN"}
	got := canonical.Calendar(cal)
	want := []byte(
		"BEGIN:VCALENDAR\r\n" +
			"VERSION:2.0\r\n" +
			"PRODID:-//V*//Empty//EN\r\n" +
			"END:VCALENDAR\r\n",
	)
	if !bytes.Equal(got, want) {
		t.Fatalf("empty calendar canonical mismatch.\n got:  %q\n want: %q", got, want)
	}
}

func TestCalendar_SortsByUIDLexicographic(t *testing.T) {
	cal := vstar.Calendar{
		ProdID: "-//V*//Sort//EN",
		Components: []vstar.Component{
			{Type: vstar.CompTodo, Props: []vstar.Property{
				{Name: "UID", Value: "ccc"},
				{Name: "DTSTAMP", Value: "20260504T120000Z"},
			}},
			{Type: vstar.CompTodo, Props: []vstar.Property{
				{Name: "UID", Value: "aaa"},
				{Name: "DTSTAMP", Value: "20260504T120000Z"},
			}},
			{Type: vstar.CompTodo, Props: []vstar.Property{
				{Name: "UID", Value: "bbb"},
				{Name: "DTSTAMP", Value: "20260504T120000Z"},
			}},
		},
	}
	got := canonical.Calendar(cal)
	aaaIdx := bytes.Index(got, []byte("UID:aaa"))
	bbbIdx := bytes.Index(got, []byte("UID:bbb"))
	cccIdx := bytes.Index(got, []byte("UID:ccc"))
	if aaaIdx < 0 || bbbIdx < 0 || cccIdx < 0 {
		t.Fatalf("missing one of UID:aaa, UID:bbb, UID:ccc in output: %q", got)
	}
	if aaaIdx >= bbbIdx || bbbIdx >= cccIdx {
		t.Fatalf("UID-lexicographic order violated: aaa@%d bbb@%d ccc@%d", aaaIdx, bbbIdx, cccIdx)
	}
}

func TestCalendar_VTimezoneSortByTZID(t *testing.T) {
	cal := vstar.Calendar{
		ProdID: "-//V*//Mixed//EN",
		Components: []vstar.Component{
			{Type: vstar.CompTodo, Props: []vstar.Property{
				{Name: "UID", Value: "z-task"},
				{Name: "DTSTAMP", Value: "20260504T120000Z"},
			}},
			{Type: vstar.CompTimezone, Props: []vstar.Property{
				{Name: "TZID", Value: "a-zone"},
			}},
		},
	}
	got := canonical.Calendar(cal)
	tzidIdx := bytes.Index(got, []byte("TZID:a-zone"))
	uidIdx := bytes.Index(got, []byte("UID:z-task"))
	if tzidIdx < 0 || uidIdx < 0 {
		t.Fatalf("missing TZID or UID in output: %q", got)
	}
	if tzidIdx > uidIdx {
		t.Fatalf("TZID:a-zone should sort before UID:z-task (a < z)")
	}
}

func TestCard_Simple(t *testing.T) {
	c := vstar.Card{
		UID:  "urn:uuid:11111111-1111-1111-1111-111111111111",
		Kind: "",
		Props: []vstar.Property{
			{Name: "FN", Value: "Jad Bitar"},
		},
	}
	got := canonical.Card(c)
	want := []byte(
		"BEGIN:VCARD\r\n" +
			"VERSION:4.0\r\n" +
			"FN:Jad Bitar\r\n" +
			"UID:urn:uuid:11111111-1111-1111-1111-111111111111\r\n" +
			"END:VCARD\r\n",
	)
	if !bytes.Equal(got, want) {
		t.Fatalf("Card canonical mismatch.\n got:  %q\n want: %q", got, want)
	}
}

func TestCard_PropertyOrderDoesNotMatter(t *testing.T) {
	a := vstar.Card{
		UID: "u",
		Props: []vstar.Property{
			{Name: "FN", Value: "Jad"},
			{Name: "EMAIL", Value: "jad@example.com"},
		},
	}
	b := vstar.Card{
		UID: "u",
		Props: []vstar.Property{
			{Name: "EMAIL", Value: "jad@example.com"},
			{Name: "FN", Value: "Jad"},
		},
	}
	if !bytes.Equal(canonical.Card(a), canonical.Card(b)) {
		t.Fatalf("Card property order on input must not affect canonical bytes")
	}
}

func TestCard_StripsXVSTARHASH(t *testing.T) {
	withHash := vstar.Card{
		UID: "u",
		Props: []vstar.Property{
			{Name: "FN", Value: "Jad"},
			{Name: "X-VSTAR-HASH", Value: "sha256:deadbeef"},
		},
	}
	withoutHash := vstar.Card{
		UID: "u",
		Props: []vstar.Property{
			{Name: "FN", Value: "Jad"},
		},
	}
	if !bytes.Equal(canonical.Card(withHash), canonical.Card(withoutHash)) {
		t.Fatalf("Card canonical must strip X-VSTAR-HASH")
	}
}

func TestCard_KindEmitted(t *testing.T) {
	c := vstar.Card{
		UID:  "u",
		Kind: vstar.KindOrg,
		Props: []vstar.Property{
			{Name: "FN", Value: "ACME"},
		},
	}
	got := canonical.Card(c)
	if !bytes.Contains(got, []byte("KIND:org")) {
		t.Fatalf("Card with Kind=org must emit KIND:org line: %q", got)
	}
}

// helperTorontoVTIMEZONE returns a VTIMEZONE component covering
// America/Toronto with EST/EDT transitions, suitable for shared
// use in datetime-resolution tests. Mirrors the
// nested_vtimezone.ics fixture.
func helperTorontoVTIMEZONE() vstar.Component {
	return vstar.Component{
		Type: vstar.CompTimezone,
		Props: []vstar.Property{
			{Name: "TZID", Value: "America/Toronto"},
		},
		Sub: []vstar.Component{
			{
				Type: "STANDARD",
				Props: []vstar.Property{
					{Name: "DTSTART", Value: "19701101T020000"},
					{Name: "TZOFFSETFROM", Value: "-0400"},
					{Name: "TZOFFSETTO", Value: "-0500"},
					{Name: "TZNAME", Value: "EST"},
					{Name: "RRULE", Value: "FREQ=YEARLY;BYMONTH=11;BYDAY=1SU"},
				},
			},
			{
				Type: "DAYLIGHT",
				Props: []vstar.Property{
					{Name: "DTSTART", Value: "19700308T020000"},
					{Name: "TZOFFSETFROM", Value: "-0500"},
					{Name: "TZOFFSETTO", Value: "-0400"},
					{Name: "TZNAME", Value: "EDT"},
					{Name: "RRULE", Value: "FREQ=YEARLY;BYMONTH=3;BYDAY=2SU"},
				},
			},
		},
	}
}

// helperVEVENTWithTZID returns a VEVENT whose DTSTART carries a
// TZID=America/Toronto parameter, used by the TZID-resolution
// tests. The wall-clock time 2026-06-01T09:00:00 in Toronto's EDT
// (UTC-4 on that date) is 2026-06-01T13:00:00Z.
func helperVEVENTWithTZID() vstar.Component {
	return vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "UID", Value: "evt-tz-1"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{
				Name:   "DTSTART",
				Params: []vstar.Param{{Name: "TZID", Value: "America/Toronto"}},
				Value:  "20260601T090000",
			},
			{Name: "SUMMARY", Value: "TZ test"},
		},
	}
}

func TestComponent_VerbatimDatetimeWithoutContext(t *testing.T) {
	got := canonical.Component(helperVEVENTWithTZID())
	if !bytes.Contains(got, []byte("DTSTART;TZID=America/Toronto:20260601T090000")) {
		t.Fatalf("Component (no context) MUST emit TZID-tagged DTSTART verbatim; got: %q", got)
	}
	if bytes.Contains(got, []byte("DTSTART:20260601T130000Z")) {
		t.Fatalf("Component (no context) MUST NOT resolve TZID to UTC; got: %q", got)
	}
}

func TestComponentInContext_ResolvesTZIDToUTC(t *testing.T) {
	cal := vstar.Calendar{
		ProdID: "-//V*//ResolveTZ//EN",
		Components: []vstar.Component{
			helperTorontoVTIMEZONE(),
			helperVEVENTWithTZID(),
		},
	}
	got := canonical.ComponentInContext(helperVEVENTWithTZID(), cal)
	if !bytes.Contains(got, []byte("DTSTART:20260601T130000Z")) {
		t.Fatalf("ComponentInContext MUST resolve TZID to UTC form #2; got: %q", got)
	}
	if bytes.Contains(got, []byte("TZID=")) {
		t.Fatalf("ComponentInContext MUST drop TZID parameter on successful resolve; got: %q", got)
	}
}

func TestComponentInContext_MissingVTIMEZONE(t *testing.T) {
	cal := vstar.Calendar{
		ProdID: "-//V*//MissingVTZ//EN",
		// Calendar has the event but NO matching VTIMEZONE — resolution
		// must fail gracefully and fall back to verbatim emit.
		Components: []vstar.Component{helperVEVENTWithTZID()},
	}
	got := canonical.ComponentInContext(helperVEVENTWithTZID(), cal)
	if !bytes.Contains(got, []byte("DTSTART;TZID=America/Toronto:20260601T090000")) {
		t.Fatalf("ComponentInContext (no matching VTIMEZONE) MUST emit verbatim; got: %q", got)
	}
	if bytes.Contains(got, []byte("DTSTART:20260601T130000Z")) {
		t.Fatalf("ComponentInContext (no matching VTIMEZONE) MUST NOT fabricate UTC; got: %q", got)
	}
}

func TestCalendar_ResolvesTZIDForChildren(t *testing.T) {
	cal := vstar.Calendar{
		ProdID: "-//V*//CalResolve//EN",
		Components: []vstar.Component{
			helperTorontoVTIMEZONE(),
			helperVEVENTWithTZID(),
		},
	}
	got := canonical.Calendar(cal)
	if !bytes.Contains(got, []byte("DTSTART:20260601T130000Z")) {
		t.Fatalf("Calendar MUST resolve TZID for child components via ComponentInContext; got: %q", got)
	}
	if bytes.Contains(got, []byte("DTSTART;TZID=")) {
		t.Fatalf("Calendar MUST drop TZID parameter on successful resolve; got: %q", got)
	}
	// The VTIMEZONE child STANDARD/DAYLIGHT DTSTART is wall-clock by
	// design — it must remain untouched.
	if !bytes.Contains(got, []byte("DTSTART:19701101T020000")) {
		t.Fatalf("VTIMEZONE STANDARD DTSTART must pass through verbatim; got: %q", got)
	}
}

func TestCanonical_AlreadyUTCDatetime_NoChange(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "UID", Value: "utc-evt"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "DTSTART", Value: "20260504T120000Z"},
		},
	}
	cal := vstar.Calendar{
		ProdID:     "-//V*//UTCNoChange//EN",
		Components: []vstar.Component{helperTorontoVTIMEZONE(), c},
	}
	noCtx := canonical.Component(c)
	withCtx := canonical.ComponentInContext(c, cal)
	if !bytes.Equal(noCtx, withCtx) {
		t.Fatalf("UTC form #2 datetimes (no TZID param) MUST be identical with or without context.\n no:  %q\n yes: %q", noCtx, withCtx)
	}
	if !bytes.Contains(withCtx, []byte("DTSTART:20260504T120000Z")) {
		t.Fatalf("UTC datetime must pass through unchanged; got: %q", withCtx)
	}
}

// TestGoldenFiles loads every fixture in testdata/rfc5545/*.ics
// and testdata/rfc6350/*.vcf, parses it, canonicalizes, and
// compares against the .canonical sibling file. Goldens on disk
// are stored LF for git friendliness; canonical bytes are CRLF
// per spec/03 rule 1, so we expand LF→CRLF before comparison.
func TestGoldenFiles(t *testing.T) {
	roots := []struct {
		dir  string
		ext  string
		card bool
	}{
		{"../testdata/rfc5545", ".ics", false},
		{"../testdata/rfc6350", ".vcf", true},
	}
	for _, r := range roots {
		entries, err := os.ReadDir(r.dir)
		if err != nil {
			t.Fatalf("read %s: %v", r.dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), r.ext) {
				continue
			}
			name := e.Name()
			fullPath := filepath.Join(r.dir, name)
			canonicalPath := strings.TrimSuffix(fullPath, r.ext) + ".canonical"
			t.Run(name, func(t *testing.T) {
				goldenBytes, err := os.ReadFile(canonicalPath)
				if err != nil {
					t.Fatalf("read golden %s: %v", canonicalPath, err)
				}
				goldenCRLF := bytes.ReplaceAll(goldenBytes, []byte("\n"), []byte("\r\n"))
				inputBytes, err := os.ReadFile(fullPath)
				if err != nil {
					t.Fatalf("read fixture %s: %v", fullPath, err)
				}
				var got []byte
				if r.card {
					cards, err := rfc6350.New().Parse(bytes.NewReader(inputBytes))
					if err != nil {
						t.Fatalf("rfc6350 parse %s: %v", name, err)
					}
					if len(cards) != 1 {
						t.Fatalf("expected exactly one card in %s, got %d", name, len(cards))
					}
					got = canonical.Card(cards[0])
				} else {
					cal, err := rfc5545.Parse(bytes.NewReader(inputBytes))
					if err != nil {
						t.Fatalf("rfc5545 parse %s: %v", name, err)
					}
					got = canonical.Calendar(cal)
				}
				if !bytes.Equal(got, goldenCRLF) {
					t.Fatalf("golden mismatch for %s.\n got:  %q\n want: %q", name, got, goldenCRLF)
				}
			})
		}
	}
}
