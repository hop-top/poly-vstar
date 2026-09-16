// SPDX-License-Identifier: MIT

// Package rfc5545 implements the iCalendar (VCALENDAR) wire format
// per RFC 5545. It provides a content-line scanner (with RFC 5545 §3.1
// line unfolding), a content-line parser (RFC 5545 §3.2), a recursive
// BEGIN/END block parser (RFC 5545 §3.4–§3.6), and an encoder that
// produces canonical-ish CRLF + 75-octet-folded output.
//
// The scanner is exported because RFC 6350 (vCard) inherits the same
// line-folding mechanism; sister codec packages reuse Scanner directly.
// The implementation lives in codec/internal/contentline; this package
// re-exports the type and constructor so the public API stays stable.
package rfc5545

import (
	"io"

	"hop.top/vstar/codec/internal/contentline"
)

// Scanner reads a stream of RFC 5545 content lines from an io.Reader,
// transparently performing line unfolding per §3.1. See the
// codec/internal/contentline package for the implementation; this is a
// type alias so external callers (and the streaming codec) can keep
// using rfc5545.Scanner unchanged.
type Scanner = contentline.Scanner

// NewScanner returns a Scanner reading from r. r is wrapped with bufio
// internally; callers should not pre-buffer.
func NewScanner(r io.Reader) *Scanner {
	return contentline.NewScanner(r)
}
