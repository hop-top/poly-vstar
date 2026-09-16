// SPDX-License-Identifier: MIT

package rfc5545

import (
	"fmt"
	"strings"

	vstar "hop.top/vstar"
)

// ParseContentLine parses a single (already-unfolded) RFC 5545 content
// line into a vstar.Property. The grammar (RFC 5545 §3.1):
//
//	contentline = name *(";" param) ":" value
//	param       = param-name "=" param-value *("," param-value)
//	param-value = paramtext / quoted-string
//
// Quoted parameter values (DQUOTE-wrapped) may contain commas,
// semicolons, and colons. Unquoted parameter values may not.
//
// The wire case of names and parameter names is preserved verbatim;
// case-insensitive matching is the responsibility of consumers
// (vstar.Component.Get, etc.).
//
// TEXT-typed values (per the textProperty allow-list) are unescaped
// per RFC 5545 §3.3.11 so the returned Property holds the raw value,
// symmetric with what the encoder re-escapes on emit. Non-TEXT
// values (URI, INTEGER, DATE-TIME, …) pass through verbatim —
// unescaping a URI would corrupt the address.
//
// Unescaping lives here rather than in the batch parser alone because
// both the batch parser and the streaming VCALENDAR parser decode the
// same wire bytes and MUST agree on the result. While the streaming
// parser called this function and then skipped the unescape step, the
// two disagreed on any TEXT value containing a backslash.
//
// Returns an error wrapping vstar.ErrMalformed for: missing colon,
// empty name, parameter without "=", or unbalanced DQUOTE in a
// parameter value.
func ParseContentLine(line string) (vstar.Property, error) {
	// Find the value-introducing colon — the first colon that is
	// outside any DQUOTE-wrapped parameter value.
	colon, err := findValueColon(line)
	if err != nil {
		return vstar.Property{}, err
	}
	head := line[:colon]
	value := line[colon+1:]

	// Split head on unquoted ';' to separate name from each param.
	segs, err := splitUnquoted(head, ';')
	if err != nil {
		return vstar.Property{}, err
	}
	if len(segs) == 0 || segs[0] == "" {
		return vstar.Property{}, fmt.Errorf(
			"rfc5545: empty property name in %q: %w",
			line, vstar.ErrMalformed,
		)
	}

	if textProperty(segs[0]) {
		value = unescapeText(value)
	}
	prop := vstar.Property{Name: segs[0], Value: value}
	for _, seg := range segs[1:] {
		eq := strings.IndexByte(seg, '=')
		if eq <= 0 {
			return vstar.Property{}, fmt.Errorf(
				"rfc5545: malformed parameter %q in %q: %w",
				seg, line, vstar.ErrMalformed,
			)
		}
		pname := seg[:eq]
		pvalue := seg[eq+1:]
		if len(pvalue) >= 2 && pvalue[0] == '"' && pvalue[len(pvalue)-1] == '"' {
			pvalue = pvalue[1 : len(pvalue)-1]
		}
		prop.Params = append(prop.Params, vstar.Param{Name: pname, Value: pvalue})
	}
	return prop, nil
}

// findValueColon returns the index of the first ':' in line that lies
// outside any DQUOTE-wrapped span. Returns ErrMalformed when no such
// colon exists or when a DQUOTE is unbalanced.
func findValueColon(line string) (int, error) {
	inQuote := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			inQuote = !inQuote
		case ':':
			if !inQuote {
				return i, nil
			}
		}
	}
	if inQuote {
		return -1, fmt.Errorf(
			"rfc5545: unbalanced quote in %q: %w",
			line, vstar.ErrMalformed,
		)
	}
	return -1, fmt.Errorf(
		"rfc5545: missing colon in content line %q: %w",
		line, vstar.ErrMalformed,
	)
}

// splitUnquoted splits s on every occurrence of sep that lies outside
// any DQUOTE-wrapped span. Returns ErrMalformed on unbalanced DQUOTE.
func splitUnquoted(s string, sep byte) ([]string, error) {
	var out []string
	inQuote := false
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		default:
			if s[i] == sep && !inQuote {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	if inQuote {
		return nil, fmt.Errorf(
			"rfc5545: unbalanced quote in %q: %w",
			s, vstar.ErrMalformed,
		)
	}
	out = append(out, s[start:])
	return out, nil
}
