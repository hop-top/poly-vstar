// SPDX-License-Identifier: MIT

package rfc6350

import (
	"fmt"
	"io"
	"strings"

	vstar "hop.top/vstar"
)

// foldOctets is the maximum length of a single physical line on the
// wire per RFC 6350 §3.2 (which references RFC 5545 §3.1). Longer
// logical lines are split with CRLF + SP at this boundary.
const foldOctets = 75

// encoder is the concrete vCard 4.0 writer behind the package-level
// Codec. It is stateless and safe for concurrent use.
type encoder struct{}

// NewEncoder returns a fresh Encoder implementation. Encoders are
// stateless and safe for concurrent use across goroutines.
func NewEncoder() Encoder { return &encoder{} }

// Encode writes a single Card as a BEGIN:VCARD…END:VCARD block to w.
// VERSION:4.0 is always emitted; UID is required (empty Card.UID
// returns ErrMissingUID per the codec-rfc6350 plan T3).
//
// Property emission order is deterministic for byte-stable output:
//
//  1. UID
//  2. KIND (when set)
//  3. Card.Props in input order
//
// Property names (the segment after any "group." prefix) are
// uppercased on encode per RFC 6350 §3.3 conventions; group prefixes
// preserve their original case for round-trip fidelity.
//
// Lines are CRLF-terminated and folded at 75 octets with CRLF + SP.
// Encode does NOT call Close on w.
func (e *encoder) Encode(w io.Writer, c vstar.Card) error {
	if c.UID == "" {
		return fmt.Errorf("rfc6350: encode: %w", vstar.ErrMissingUID)
	}

	var sb strings.Builder
	writeFolded(&sb, "BEGIN:VCARD")
	writeFolded(&sb, "VERSION:"+supportedVersion)
	writeFolded(&sb, "UID:"+escapeText(c.UID))
	if c.Kind != "" {
		writeFolded(&sb, "KIND:"+strings.ToLower(string(c.Kind)))
	}
	for _, p := range c.Props {
		writeFolded(&sb, formatProperty(p))
	}
	writeFolded(&sb, "END:VCARD")

	if _, err := io.WriteString(w, sb.String()); err != nil {
		return fmt.Errorf("rfc6350: write: %w", err)
	}
	return nil
}

// formatProperty serializes a single Property to its on-the-wire form,
// excluding the trailing CRLF (folding adds it). Group prefixes survive
// verbatim; the bare property name is uppercased; param values are
// DQUOTE'd when they contain ',' ':' or ';'.
func formatProperty(p vstar.Property) string {
	var sb strings.Builder
	sb.Grow(len(p.Name) + len(p.Value) + 16)

	group, name := splitGroup(p.Name)
	if group != "" {
		sb.WriteString(group)
		sb.WriteByte('.')
	}
	sb.WriteString(strings.ToUpper(name))

	for _, prm := range p.Params {
		sb.WriteByte(';')
		sb.WriteString(strings.ToUpper(prm.Name))
		sb.WriteByte('=')
		sb.WriteString(formatParamValue(prm.Value))
	}
	sb.WriteByte(':')
	sb.WriteString(escapeText(p.Value))
	return sb.String()
}

// splitGroup separates a "group.NAME" property identifier into its
// group and bare-name pieces. When no '.' is present, group is "" and
// name is the full input.
func splitGroup(s string) (group, name string) {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}

// formatParamValue returns the parameter value with DQUOTEs applied
// when it contains characters that would otherwise be ambiguous on the
// wire (',' ':' ';') per RFC 6350 §3.3.
func formatParamValue(v string) string {
	if strings.ContainsAny(v, ",;:") {
		return `"` + v + `"`
	}
	return v
}

// escapeText applies RFC 6350 §3.4 TEXT escaping for encoding:
// '\\' → "\\\\", ',' → "\\,", ';' → "\\;", '\n' → "\\n".
func escapeText(s string) string {
	if !strings.ContainsAny(s, "\\,;\n") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			b.WriteString(`\\`)
		case ',':
			b.WriteString(`\,`)
		case ';':
			b.WriteString(`\;`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// writeFolded appends line to dst with RFC 6350 §3.2 folding applied:
// at most 75 octets per physical line; continuation lines start with
// CRLF + single SP; terminator is CRLF.
//
// Folding is octet-based (per the RFC), not rune-based; multi-byte
// UTF-8 sequences may be split. Decoders MUST reassemble before any
// rune-level interpretation.
func writeFolded(dst *strings.Builder, line string) {
	if len(line) <= foldOctets {
		dst.WriteString(line)
		dst.WriteString("\r\n")
		return
	}

	// First physical line carries a full 75 octets of payload: it has
	// no continuation SP to pay for.
	dst.WriteString(line[:foldOctets])
	dst.WriteString("\r\n")
	rest := line[foldOctets:]

	// EVERY continuation line thereafter spends one of its 75 octets on
	// the leading SP, so it carries 74 of payload. An earlier version
	// alternated 75/74 across loop iterations and emitted a 76-octet
	// physical line (SP + 75) on every second continuation.
	const contPayload = foldOctets - 1
	for len(rest) > 0 {
		take := contPayload
		if take > len(rest) {
			take = len(rest)
		}
		dst.WriteString(" ")
		dst.WriteString(rest[:take])
		dst.WriteString("\r\n")
		rest = rest[take:]
	}
}
