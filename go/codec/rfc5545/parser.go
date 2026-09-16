// SPDX-License-Identifier: MIT

package rfc5545

import (
	"errors"
	"fmt"
	"io"
	"strings"

	vstar "hop.top/vstar"
)

// supportedVersion is the only iCalendar VERSION value V* honors
// at v0.1; producers emitting any other value will fail validation
// at the parser boundary.
const supportedVersion = "2.0"

// Parse reads a single VCALENDAR from r and returns the resulting
// vstar.Calendar. Errors are wrapped with positional context (the
// content line) and one of the package sentinels:
// vstar.ErrMalformed, vstar.ErrUnclosedBlock, vstar.ErrUnsupportedVersion.
//
// Property order, parameter order, and component order are preserved
// verbatim. No canonicalization happens here — the canonical track
// owns rules-based normalization.
func Parse(r io.Reader) (vstar.Calendar, error) {
	s := NewScanner(r)

	// First non-empty line MUST be BEGIN:VCALENDAR.
	first, err := s.Next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return vstar.Calendar{}, fmt.Errorf(
				"rfc5545: empty input: %w", vstar.ErrMalformed,
			)
		}
		return vstar.Calendar{}, err
	}
	if !strings.EqualFold(first, "BEGIN:VCALENDAR") {
		return vstar.Calendar{}, fmt.Errorf(
			"rfc5545: expected BEGIN:VCALENDAR, got %q: %w",
			first, vstar.ErrMalformed,
		)
	}

	root, err := parseBlock(s, "VCALENDAR")
	if err != nil {
		return vstar.Calendar{}, err
	}

	cal := vstar.Calendar{}
	for _, p := range root.Props {
		switch strings.ToUpper(p.Name) {
		case "VERSION":
			if p.Value != supportedVersion {
				return vstar.Calendar{}, fmt.Errorf(
					"rfc5545: VERSION=%q (only %q supported): %w",
					p.Value, supportedVersion, vstar.ErrUnsupportedVersion,
				)
			}
		case propPRODID:
			cal.ProdID = p.Value
		}
	}
	cal.Components = root.Sub

	// Trailing content after END:VCALENDAR is ignored intentionally:
	// scanners often round up trailing whitespace or producers may
	// concatenate streams. The strict-but-not-pedantic stance is "we
	// got a valid calendar; stop reading."
	return cal, nil
}

// parseBlock consumes content lines from s until END:<typeName> is
// seen. It returns the assembled Component and never the END line.
// Properties accumulate on the parent; nested BEGIN: blocks recurse
// and append to Sub. Mismatched END names are ErrMalformed; EOF
// before END is ErrUnclosedBlock.
func parseBlock(s *Scanner, typeName string) (vstar.Component, error) {
	out := vstar.Component{Type: vstar.CompType(strings.ToUpper(typeName))}
	for {
		line, err := s.Next()
		if errors.Is(err, io.EOF) {
			return out, fmt.Errorf(
				"rfc5545: BEGIN:%s never closed: %w",
				typeName, vstar.ErrUnclosedBlock,
			)
		}
		if err != nil {
			return out, err
		}

		prop, err := ParseContentLine(line)
		if err != nil {
			return out, err
		}

		switch strings.ToUpper(prop.Name) {
		case "BEGIN":
			child, err := parseBlock(s, prop.Value)
			if err != nil {
				return out, err
			}
			out.Sub = append(out.Sub, child)
		case "END":
			if !strings.EqualFold(prop.Value, typeName) {
				return out, fmt.Errorf(
					"rfc5545: END:%s does not match BEGIN:%s: %w",
					prop.Value, typeName, vstar.ErrMalformed,
				)
			}
			return out, nil
		default:
			// TEXT-typed values arrive already unescaped:
			// ParseContentLine owns that step so every caller —
			// batch and streaming alike — decodes identical bytes
			// to an identical model. Unescaping a second time here
			// would collapse a legitimately-doubled backslash.
			out.Props = append(out.Props, prop)
		}
	}
}

// unescapeText reverses RFC 5545 §3.3.11 TEXT escaping for parsing:
//
//	"\\\\" → '\\'   (backslash, must be considered before any other
//	                  pair so a literal '\\,' is not mis-decoded as
//	                  an escape pair)
//	"\\,"  → ','
//	"\\;"  → ';'
//	"\\n", "\\N" → '\n'
//
// A trailing solitary backslash (no following char) is preserved
// verbatim — this mirrors most real-world parser leniency and keeps
// the function total. Unknown two-char escapes (e.g. `\x`) drop the
// backslash and keep the second char, matching common practice.
func unescapeText(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		switch s[i+1] {
		case '\\':
			b.WriteByte('\\')
		case ',':
			b.WriteByte(',')
		case ';':
			b.WriteByte(';')
		case 'n', 'N':
			b.WriteByte('\n')
		default:
			b.WriteByte(s[i+1])
		}
		i++
	}
	return b.String()
}
