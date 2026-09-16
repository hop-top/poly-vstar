// SPDX-License-Identifier: MIT

// Package contentline implements the shared RFC 5545 §3.1 content-line
// unfolding scanner used by both the iCalendar (rfc5545) and vCard
// (rfc6350) codecs. RFC 6350 §3.2 references RFC 5545 §3.1 for line
// folding, so the two formats need byte-identical scanning behavior.
//
// The package lives under codec/internal so it is reachable from both
// rfc5545 and rfc6350 (and the streaming codec) but never escapes the
// codec subtree as a public API. Callers outside the V* codec packages
// MUST consume content lines via the public rfc5545.Scanner re-export.
//
// Design notes:
//   - Streaming shape (one logical line at a time) keeps the streaming
//     codec constant-memory; the eager rfc6350 unfold path wraps Next
//     in a small helper rather than the other way around.
//   - Lenient blank-then-WSP handling: when a WSP-prefixed line appears
//     with no pending logical line in flight (e.g. after a blank line
//     that broke a fold sequence in malformed input), the leading WSP
//     is stripped and a fresh logical line starts. This mirrors the
//     pre-consolidation rfc6350 behavior and fixes a stray-leading-space
//     bug that the pre-consolidation rfc5545 scanner exhibited.
//   - Both CRLF (RFC-correct) and bare LF (lenient) are accepted as
//     physical-line terminators. Trailing partial lines (no final
//     terminator) are surfaced as a final logical line before EOF.
package contentline

import (
	"bufio"
	"errors"
	"io"
)

// Scanner reads a stream of content lines from an io.Reader, performing
// line unfolding per RFC 5545 §3.1. The wire format permits a logical
// content line to be split across multiple physical lines by inserting
// CRLF + (SP | HTAB); on read, those sequences are replaced with
// nothing (the SP/HTAB and the CRLF are both consumed) and the
// continuation is appended to the previous logical line.
//
// Scanner accepts CRLF or LF as physical-line terminators (be liberal
// on input). It silently skips blank physical lines that do not appear
// inside a fold sequence; a WSP-prefixed line that follows a blank
// (and therefore has no pending line to extend) is treated as a fresh
// logical line with its leading WSP byte stripped, mirroring vCard
// real-world parser leniency.
//
// The zero value is not usable — construct with NewScanner. Scanner is
// not safe for concurrent use; callers wanting to scan multiple inputs
// should construct one Scanner per Reader.
type Scanner struct {
	br      *bufio.Reader
	pending string // already-decoded current logical line, mid-assembly
	hasPend bool
	done    bool
}

// NewScanner returns a Scanner reading from r. r is wrapped with bufio
// internally; callers should not pre-buffer.
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{br: bufio.NewReader(r)}
}

// Next returns the next logical content line (with line folds resolved)
// or io.EOF when the input is exhausted. Trailing partial lines (no
// final CRLF/LF) are surfaced as a final logical line before EOF.
//
// Returned strings do NOT include the line terminator.
func (s *Scanner) Next() (string, error) {
	for {
		if s.done && !s.hasPend {
			return "", io.EOF
		}

		raw, err := s.readPhysical()
		if errors.Is(err, io.EOF) {
			s.done = true
			if s.hasPend {
				out := s.pending
				s.pending = ""
				s.hasPend = false
				if out == "" {
					// Empty trailing line; skip and surface EOF.
					return "", io.EOF
				}
				return out, nil
			}
			return "", io.EOF
		}
		if err != nil {
			return "", err
		}

		// WSP-prefixed line: continuation of pending, OR (lenient)
		// the start of a new logical line if no pending exists.
		// RFC 5545 §3.1: a fold is a CRLF (already consumed by
		// readPhysical) followed by a SP or HTAB at the start of the
		// next physical line; the SP/HTAB is part of the fold sequence
		// and must NOT appear in the logical line.
		if len(raw) > 0 && (raw[0] == ' ' || raw[0] == '\t') {
			if s.hasPend {
				s.pending += raw[1:]
				continue
			}
			// No pending logical line (e.g. previous physical line was
			// blank, breaking the fold sequence). Treat the remainder
			// as a fresh logical line — strip the leading WSP byte and
			// start accumulating. This is the lenient-on-input shape
			// inherited from rfc6350; without it, malformed inputs like
			// "DESCRIPTION:start\r\n\r\n more\r\n" would either yield
			// a leading space (" more") or be silently dropped.
			s.pending = raw[1:]
			s.hasPend = true
			continue
		}

		// New logical line starts here. Flush any pending line first.
		if s.hasPend {
			out := s.pending
			s.pending = raw
			// raw may be empty (blank physical line); only retain it
			// if it has content. Empty new starts are dropped, since
			// they are not content lines.
			if raw == "" {
				s.hasPend = false
			}
			if out == "" {
				// Previous flush was an empty line — skip it and
				// continue scanning for the next non-empty.
				continue
			}
			return out, nil
		}

		// No pending; start one if non-empty, else loop.
		if raw == "" {
			continue
		}
		s.pending = raw
		s.hasPend = true
	}
}

// readPhysical reads a single physical line (terminated by CRLF, LF,
// or EOF), strips the terminator, and returns the line contents.
// On EOF with no trailing terminator, returns the partial line and
// io.EOF on the *following* call.
func (s *Scanner) readPhysical() (string, error) {
	line, err := s.br.ReadString('\n')
	if len(line) == 0 && err != nil {
		return "", err
	}
	// Strip trailing \n and optional \r.
	n := len(line)
	if n > 0 && line[n-1] == '\n' {
		line = line[:n-1]
		n--
	}
	if n > 0 && line[n-1] == '\r' {
		line = line[:n-1]
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return line, err
	}
	// EOF after a terminator-less last line: defer EOF to next call.
	return line, nil
}

// UnfoldAll drains a Scanner reading from r and returns every logical
// content line in input order. It exists for callers that need eager
// list-of-lines semantics (e.g. the rfc6350 batch parser pre-streaming
// migration). Streaming callers should consume Scanner.Next directly to
// keep memory constant.
func UnfoldAll(r io.Reader) ([]string, error) {
	s := NewScanner(r)
	var lines []string
	for {
		line, err := s.Next()
		if errors.Is(err, io.EOF) {
			return lines, nil
		}
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
}
