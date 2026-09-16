// SPDX-License-Identifier: MIT

package rfc5545

import (
	"bufio"
	"io"
	"strings"

	vstar "hop.top/vstar"
)

// maxLineOctets is the RFC 5545 §3.1 fold limit: physical lines
// (excluding the CRLF terminator) MUST NOT exceed 75 octets.
const maxLineOctets = 75

// crlf is the wire-format physical-line terminator. RFC 5545 §3.1
// specifies CRLF on output even though the parser is liberal on input.
const crlf = "\r\n"

// Property-name string constants. These are the bare wire names
// (uppercased per RFC 5545 §3.1 / RFC 6350 §3.3 conventions);
// callers should compare with strings.EqualFold or pre-uppercase.
const (
	propPRODID      = "PRODID"
	propUID         = "UID"
	propSUMMARY     = "SUMMARY"
	propDESCRIPTION = "DESCRIPTION"
)

// Encode writes cal to w in RFC 5545 wire format. Output is always
// CRLF-terminated; physical lines are folded at 75 octets per §3.1
// using SP as the continuation-line lead octet.
//
// Encode does NOT close w. On error, w may have received a partial
// write — callers should treat its contents as undefined.
//
// Property + component order is preserved from cal verbatim. The
// outer VCALENDAR wrapper is always emitted with VERSION:2.0 and
// PRODID derived from cal.ProdID; any VERSION/PRODID inside
// cal.Components or as inline calendar-level Props is ignored at
// this layer (callers wanting custom calendar-level props should
// land them through a higher-level helper).
//
// PRODID is emitted via encodeContentLine so RFC 5545 §3.3.11 TEXT
// escaping applies to its value (PRODID is a TEXT-typed property).
func Encode(w io.Writer, cal vstar.Calendar) error {
	bw := bufio.NewWriter(w)

	if err := writeFolded(bw, "BEGIN:VCALENDAR"); err != nil {
		return err
	}
	if err := writeFolded(bw, "VERSION:"+supportedVersion); err != nil {
		return err
	}
	prodID := encodeContentLine(vstar.Property{Name: propPRODID, Value: cal.ProdID})
	if err := writeFolded(bw, prodID); err != nil {
		return err
	}
	for _, c := range cal.Components {
		if err := encodeComponent(bw, c); err != nil {
			return err
		}
	}
	if err := writeFolded(bw, "END:VCALENDAR"); err != nil {
		return err
	}
	return bw.Flush()
}

// EncodeComponent writes a single Component (including its BEGIN/END
// wrapper, properties, and recursive sub-components) to w in the
// RFC 5545 wire format. Output is CRLF-terminated and folded at 75
// octets per §3.1, exactly as Encode would produce inside its
// VCALENDAR wrapper.
//
// EncodeComponent is the building block canonicalization uses to
// share fold/CRLF logic with the Calendar encoder; it does NOT
// inject any VCALENDAR wrapper or VERSION/PRODID. EncodeComponent
// does NOT close w. On error, w may have received a partial write.
func EncodeComponent(w io.Writer, c vstar.Component) error {
	bw := bufio.NewWriter(w)
	if err := encodeComponent(bw, c); err != nil {
		return err
	}
	return bw.Flush()
}

// encodeComponent writes a single Component (including its BEGIN/END
// wrapper, properties, and recursive sub-components) to bw.
func encodeComponent(bw *bufio.Writer, c vstar.Component) error {
	tname := strings.ToUpper(string(c.Type))
	if err := writeFolded(bw, "BEGIN:"+tname); err != nil {
		return err
	}
	for _, p := range c.Props {
		if err := writeFolded(bw, encodeContentLine(p)); err != nil {
			return err
		}
	}
	for _, s := range c.Sub {
		if err := encodeComponent(bw, s); err != nil {
			return err
		}
	}
	return writeFolded(bw, "END:"+tname)
}

// encodeContentLine renders a Property as a single (un-folded) wire
// content line: NAME[;PARAM=val[;PARAM=val]]:value. Param values
// containing ',' ';' ':' or '"' are DQUOTE-wrapped per §3.2 (the
// inner DQUOTE is rejected by the grammar, so we strip it on the
// rare chance a caller supplied one).
//
// TEXT-typed property values (per the textProperty allow-list) are
// escaped per RFC 5545 §3.3.11: '\' → "\\", ',' → "\,", ';' →
// "\;", LF → "\n" (CRLF normalized to a single "\n"). Non-TEXT
// properties (URI, INTEGER, DATE-TIME, etc.) emit verbatim.
func encodeContentLine(p vstar.Property) string {
	var b strings.Builder
	b.WriteString(strings.ToUpper(p.Name))
	for _, par := range p.Params {
		b.WriteByte(';')
		b.WriteString(strings.ToUpper(par.Name))
		b.WriteByte('=')
		b.WriteString(encodeParamValue(par.Value))
	}
	b.WriteByte(':')
	if textProperty(p.Name) {
		b.WriteString(escapeText(p.Value))
	} else {
		b.WriteString(p.Value)
	}
	return b.String()
}

// encodeParamValue wraps v in DQUOTE iff v contains any of the
// characters that disambiguate parameter boundaries (',' ';' ':').
// Inner DQUOTE characters are dropped — RFC 5545 §3.2 does not
// permit DQUOTE inside a quoted-string.
func encodeParamValue(v string) string {
	v = strings.ReplaceAll(v, `"`, "")
	if strings.ContainsAny(v, ",;:") {
		return `"` + v + `"`
	}
	return v
}

// textProperties is the allow-list of property names whose values
// are TEXT-typed per RFC 5545 §3.3.11 / §3.7-§3.8 and RFC 6350 §3.4.
// Only TEXT values are backslash-escaped on emit; URI, INTEGER,
// DATE-TIME and other value types pass through verbatim — escaping a
// comma in a URI would corrupt the address.
//
// The list is shared with the canonical package (intentional: the two
// callers MUST agree on what counts as TEXT so canonical output and
// raw encoder output stay byte-identical for matching inputs). When
// adding a property, mirror it in both lists.
//
// Custom properties (X-* extensions and unknown names) are NOT
// treated as TEXT by default. Producers wanting escape semantics for
// a custom property MUST land an entry here.
var textProperties = map[string]struct{}{
	// RFC 5545 calendar TEXT properties.
	"CATEGORIES":    {},
	"CLASS":         {},
	"COMMENT":       {},
	"CONTACT":       {},
	propDESCRIPTION: {},
	"LOCATION":      {},
	propPRODID:      {},
	"RELATED-TO":    {},
	"RESOURCES":     {},
	"STATUS":        {},
	propSUMMARY:     {},
	"TRANSP":        {},
	"TZID":          {},
	"TZNAME":        {},
	propUID:         {},
	// RFC 6350 vCard TEXT properties.
	"FN":       {},
	"N":        {},
	"NICKNAME": {},
	"NOTE":     {},
	"ORG":      {},
	"TITLE":    {},
	"ROLE":     {},
	"KIND":     {},
}

// textProperty reports whether name is a TEXT-typed property per
// the allow-list. Comparison is case-insensitive.
func textProperty(name string) bool {
	_, ok := textProperties[strings.ToUpper(name)]
	return ok
}

// escapeText applies RFC 5545 §3.3.11 / RFC 6350 §3.4 TEXT escaping
// for encoding:
//
//	'\\' → "\\\\"   (backslash, must come first)
//	','  → "\\,"
//	';'  → "\\;"
//	'\n' → "\\n"
//
// CR is dropped (per RFC: only LF is permitted in input TEXT). A
// literal CRLF in s collapses to a single escaped "\n".
//
// CATEGORIES and RESOURCES are multi-value TEXT — RFC §3.3.11 uses
// the unescaped comma as the value separator. This helper escapes
// every comma uniformly: producers wishing to preserve a comma as a
// list separator must pass already-comma-joined Values that omit the
// per-element commas (or land multi-value handling at a higher
// layer). Behavior matches the canonical package's earlier private
// implementation; see spec/03 rule 9.
//
// Idempotency: escapeText is NOT idempotent — a string containing a
// literal backslash gets re-escaped to two backslashes on a second
// pass. The encoder calls escapeText exactly once per emit and the
// parser is paired with unescapeText so the model holds raw values.
func escapeText(s string) string {
	if !strings.ContainsAny(s, "\\,;\n\r") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			b.WriteString(`\\`)
		case ',':
			b.WriteString(`\,`)
		case ';':
			b.WriteString(`\;`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			// Drop CR; canonical TEXT uses bare LF for embedded
			// newlines (escaped to "\n" above).
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// writeFolded writes a single logical content line to bw, folding
// it across multiple physical lines as needed so that no physical
// line exceeds maxLineOctets octets. Each physical line is followed
// by CRLF; continuation lines are prefixed with a single SP.
//
// Octet counting is byte-based per RFC 5545 §3.1 and spec rule 3:
// a multi-byte UTF-8 sequence straddling the boundary IS split, so
// the emitted bytes are not necessarily valid UTF-8 line by line.
// Round-trip parsing reassembles them verbatim because unfolding
// runs before any rune-level interpretation. An earlier version of
// this comment claimed runes are never split; that was wrong, and
// the rfc6350 encoder's comment has always described it correctly.
func writeFolded(bw *bufio.Writer, line string) error {
	if len(line) <= maxLineOctets {
		if _, err := bw.WriteString(line); err != nil {
			return err
		}
		_, err := bw.WriteString(crlf)
		return err
	}

	// First chunk: full 75 octets.
	if _, err := bw.WriteString(line[:maxLineOctets]); err != nil {
		return err
	}
	if _, err := bw.WriteString(crlf); err != nil {
		return err
	}
	rest := line[maxLineOctets:]

	// Continuation chunks: leading SP costs 1 octet, so 74 of payload.
	const contPayload = maxLineOctets - 1
	for len(rest) > 0 {
		take := contPayload
		if take > len(rest) {
			take = len(rest)
		}
		if _, err := bw.WriteString(" "); err != nil {
			return err
		}
		if _, err := bw.WriteString(rest[:take]); err != nil {
			return err
		}
		if _, err := bw.WriteString(crlf); err != nil {
			return err
		}
		rest = rest[take:]
	}
	return nil
}
