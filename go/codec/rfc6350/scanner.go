// SPDX-License-Identifier: MIT

// Package rfc6350 is the V* codec for vCard 4.0 (RFC 6350) wire form.
//
// It implements line unfolding (RFC 6350 §3.2 → RFC 5545 §3.1),
// vCard content-line parsing including group prefixes
// (RFC 6350 §3.3) and TEXT-value escaping (\\, \,, \;, \n),
// BEGIN:VCARD…END:VCARD framing, and the symmetric encoder with
// 75-octet folding and CRLF terminators. VERSION:4.0 is the only
// version accepted in v0.1; VERSION:3.0 returns ErrUnsupportedVersion.
//
// Use the package-level Default codec, or rfc6350.New() for a fresh
// instance — both are stateless and concurrency-safe for read-only use.
package rfc6350

import (
	"fmt"
	"io"
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/internal/contentline"
)

// unfold consumes r line-by-line, applying RFC 6350 §3.2 line
// unfolding. The implementation lives in contentline (shared with the
// rfc5545 codec); this thin adapter keeps parser.go's call site
// unchanged while the eager batch path migrates to streaming.
//
// Both CRLF (RFC-correct) and bare LF (lenient on read) are accepted
// as line terminators. WSP-prefixed lines with no pending logical line
// have the leading WSP stripped and are emitted as fresh logical lines.
func unfold(r io.Reader) ([]string, error) {
	lines, err := contentline.UnfoldAll(r)
	if err != nil {
		return nil, fmt.Errorf("rfc6350: read: %w", err)
	}
	return lines, nil
}

// ParseContentLine decomposes a single already-unfolded vCard content
// line into a vstar.Property per RFC 6350 §3.3 / §3.4, exactly as the
// batch parser does: group prefixes are preserved in Property.Name and
// the value is TEXT-unescaped.
//
// It is exported so the streaming vCard parser can share this one
// implementation. Routing vCard lines through rfc5545.ParseContentLine
// instead used to leave streamed values escaped, so identical bytes
// decoded to different in-memory values depending on which parser the
// caller reached for.
func ParseContentLine(line string) (vstar.Property, error) {
	return parseContentLine(line)
}

// parseContentLine decomposes a single unfolded vCard content line
// into a vstar.Property per RFC 6350 §3.3 / §3.4.
//
// Format:
//
//	[group "."] name *(";" param) ":" value
//
// The group prefix (when present) is preserved verbatim in
// Property.Name (e.g. "home.TEL"). TEXT-value escaping is unfolded
// per RFC 6350 §3.4: "\\" → "\", "\," → ",", "\;" → ";",
// "\n" → "\n" (literal newline), "\N" → "\n".
//
// Returns ErrMalformed (wrapped with %w) for any structural defect.
func parseContentLine(line string) (vstar.Property, error) {
	if line == "" {
		return vstar.Property{}, fmt.Errorf("rfc6350: empty line: %w", vstar.ErrMalformed)
	}

	// Locate the value separator (first unquoted ':').
	colonIdx, err := findValueColon(line)
	if err != nil {
		return vstar.Property{}, err
	}

	head := line[:colonIdx]
	rawValue := line[colonIdx+1:]

	// Split head on first unquoted ';' to separate name from params.
	nameEnd, err := findFirstUnquoted(head, ';')
	if err != nil {
		return vstar.Property{}, err
	}

	var (
		nameTok  string
		paramTok string
	)
	if nameEnd < 0 {
		nameTok = head
	} else {
		nameTok = head[:nameEnd]
		paramTok = head[nameEnd+1:]
	}

	if nameTok == "" {
		return vstar.Property{}, fmt.Errorf("rfc6350: empty property name: %w", vstar.ErrMalformed)
	}

	params, err := parseParams(paramTok)
	if err != nil {
		return vstar.Property{}, err
	}

	return vstar.Property{
		Name:   nameTok,
		Params: params,
		Value:  unescapeText(rawValue),
	}, nil
}

// findValueColon returns the byte index of the first ':' that is not
// inside a double-quoted parameter value. Returns ErrMalformed if no
// such colon exists.
func findValueColon(s string) (int, error) {
	idx, err := findFirstUnquoted(s, ':')
	if err != nil {
		return -1, err
	}
	if idx < 0 {
		return -1, fmt.Errorf("rfc6350: missing value separator: %w", vstar.ErrMalformed)
	}
	return idx, nil
}

// findFirstUnquoted returns the byte index of the first occurrence of
// target in s that is not inside a DQUOTE-delimited region.
// Returns -1 if not found. Returns ErrMalformed if the string ends
// inside an open quote.
func findFirstUnquoted(s string, target byte) (int, error) {
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && c == target {
			return i, nil
		}
	}
	if inQuote {
		return -1, fmt.Errorf("rfc6350: unterminated quoted value: %w", vstar.ErrMalformed)
	}
	return -1, nil
}

// parseParams parses the parameter portion of a content line head
// (everything between the first ';' after the name and the value
// separator ':'). Each parameter is "NAME=VALUE"; VALUE may be
// double-quoted to embed ',' ':' or ';'.
func parseParams(s string) ([]vstar.Param, error) {
	if s == "" {
		return nil, nil
	}
	var params []vstar.Param
	rest := s
	for len(rest) > 0 {
		end, err := findFirstUnquoted(rest, ';')
		if err != nil {
			return nil, err
		}
		var tok string
		if end < 0 {
			tok = rest
			rest = ""
		} else {
			tok = rest[:end]
			rest = rest[end+1:]
		}
		eq, err := findFirstUnquoted(tok, '=')
		if err != nil {
			return nil, err
		}
		if eq < 0 {
			return nil, fmt.Errorf("rfc6350: param %q missing '=': %w", tok, vstar.ErrMalformed)
		}
		name := tok[:eq]
		value := tok[eq+1:]
		if name == "" {
			return nil, fmt.Errorf("rfc6350: empty param name: %w", vstar.ErrMalformed)
		}
		// Strip surrounding DQUOTEs from the value if present.
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		params = append(params, vstar.Param{Name: name, Value: value})
	}
	return params, nil
}

// unescapeText reverses RFC 6350 §3.4 TEXT escaping.
func unescapeText(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		next := s[i+1]
		switch next {
		case '\\':
			b.WriteByte('\\')
		case ',':
			b.WriteByte(',')
		case ';':
			b.WriteByte(';')
		case 'n', 'N':
			b.WriteByte('\n')
		default:
			// Unknown escape — keep backslash + char (lenient).
			b.WriteByte('\\')
			b.WriteByte(next)
		}
		i++
	}
	return b.String()
}
