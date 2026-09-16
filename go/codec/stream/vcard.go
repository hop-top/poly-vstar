// SPDX-License-Identifier: MIT

package stream

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/codec/rfc6350"
)

// Compile-time guards: VCardParser and VCardEncoder MUST satisfy the
// CardStreamParser / CardStreamEncoder interfaces from stream.go.
var (
	_ CardStreamParser  = (*VCardParser)(nil)
	_ CardStreamEncoder = (*VCardEncoder)(nil)
)

// supportedCardVersion mirrors rfc6350.supportedVersion (unexported).
// Streaming VCARD parser MUST also reject VERSION values other than
// 4.0 to stay consistent with the batch codec.
const supportedCardVersion = "4.0"

// VCardParser is the streaming VCARD reader. VCARD files are a
// concatenation of independent BEGIN:VCARD…END:VCARD blocks with no
// outer wrapper; the parser yields one Card per Next call and
// returns io.EOF when the underlying reader is exhausted with no
// pending block.
//
// The line-folding scanner is shared with rfc5545 (RFC 6350 §3.2
// inherits RFC 5545 §3.1 line folding); content lines are decoded
// using the rfc6350 helpers indirectly via shadowing in this file.
//
// VCardParser is not safe for concurrent use; construct one per
// reader.
type VCardParser struct {
	scanner *rfc5545.Scanner

	// done is true once io.EOF has been propagated to the caller.
	done bool
}

// NewVCardParser returns a VCardParser reading from r. r is wrapped
// internally; callers should not pre-buffer.
func NewVCardParser(r io.Reader) *VCardParser {
	return &VCardParser{scanner: rfc5545.NewScanner(r)}
}

// Next reads and returns the next vCard from the stream. Returns
// io.EOF when no further BEGIN:VCARD block is found.
//
// Errors wrapping vstar sentinels:
//   - vstar.ErrMalformed for missing BEGIN, content outside a block,
//     or duplicate VERSION.
//   - vstar.ErrUnsupportedVersion when VERSION != 4.0.
//   - vstar.ErrUnclosedBlock when EOF interrupts an open BEGIN:VCARD.
func (p *VCardParser) Next() (vstar.Card, error) {
	if p.done {
		return vstar.Card{}, io.EOF
	}

	// Skip blank lines / wait for BEGIN:VCARD.
	for {
		line, err := p.scanner.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				p.done = true
				return vstar.Card{}, io.EOF
			}
			return vstar.Card{}, err
		}
		if line == "" {
			continue
		}
		if !strings.EqualFold(line, "BEGIN:VCARD") {
			return vstar.Card{}, fmt.Errorf(
				"stream/vcard: expected BEGIN:VCARD, got %q: %w",
				line, vstar.ErrMalformed,
			)
		}
		break
	}

	var (
		card    vstar.Card
		version string
	)

	for {
		line, err := p.scanner.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return vstar.Card{}, fmt.Errorf(
					"stream/vcard: BEGIN:VCARD never closed: %w",
					vstar.ErrUnclosedBlock,
				)
			}
			return vstar.Card{}, err
		}
		if line == "" {
			continue
		}
		if strings.EqualFold(line, "END:VCARD") {
			if version == "" {
				return vstar.Card{}, fmt.Errorf(
					"stream/vcard: VCARD missing VERSION: %w",
					vstar.ErrMalformed,
				)
			}
			if version != supportedCardVersion {
				return vstar.Card{}, fmt.Errorf(
					"stream/vcard: VERSION:%s: %w",
					version, vstar.ErrUnsupportedVersion,
				)
			}
			return card, nil
		}
		if strings.EqualFold(line, "BEGIN:VCARD") {
			return vstar.Card{}, fmt.Errorf(
				"stream/vcard: nested BEGIN:VCARD: %w",
				vstar.ErrMalformed,
			)
		}

		// Parse a single content line through the rfc6350 grammar —
		// the same entry point the batch codec uses. It is NOT enough
		// to reach for rfc5545.ParseContentLine because the vCard
		// grammar differs where it matters: RFC 6350 §3.4 TEXT
		// escaping is applied to every value, so `FN:Last\, Comma`
		// decodes to `Last, Comma`. Routing through rfc5545 left the
		// backslash in place and made the streaming parser disagree
		// with the batch parser on identical bytes.
		prop, err := rfc6350.ParseContentLine(line)
		if err != nil {
			return vstar.Card{}, err
		}

		switch strings.ToUpper(propBareName(prop.Name)) {
		case "VERSION":
			if version != "" {
				return vstar.Card{}, fmt.Errorf(
					"stream/vcard: duplicate VERSION: %w",
					vstar.ErrMalformed,
				)
			}
			version = prop.Value
		case "UID":
			card.UID = prop.Value
		case "KIND":
			card.Kind = vstar.Kind(strings.ToLower(prop.Value))
		default:
			card.Props = append(card.Props, prop)
		}
	}
}

// propBareName strips the optional "group." prefix from a vCard
// property name per RFC 6350 §3.3. UID/VERSION/KIND are recognized
// regardless of group qualifier on the wire.
func propBareName(name string) string {
	if i := strings.IndexByte(name, '.'); i >= 0 {
		return name[i+1:]
	}
	return name
}

// VCardEncoder is the streaming VCARD writer. Each Encode call writes
// one self-contained BEGIN:VCARD…END:VCARD block. There is no header
// to emit and no trailer to flush, but Close exists for symmetry with
// VCalendarEncoder and to flush bufio buffering. Calling Encode after
// Close, or Close more than once, returns ErrAlreadyClosed.
type VCardEncoder struct {
	bw     *bufio.Writer
	codec  rfc6350.Encoder
	closed bool
}

// NewVCardEncoder returns a VCardEncoder writing to w. Output is
// buffered through bufio internally; callers should not pre-buffer.
func NewVCardEncoder(w io.Writer) *VCardEncoder {
	return &VCardEncoder{
		bw:    bufio.NewWriter(w),
		codec: rfc6350.NewEncoder(),
	}
}

// Encode writes a single Card to the underlying writer in canonical
// VCARD wire form (VERSION:4.0, UID, KIND, then Card.Props in input
// order, all CRLF-folded at 75 octets).
//
// Returns ErrAlreadyClosed if called after Close.
func (e *VCardEncoder) Encode(c vstar.Card) error {
	if e.closed {
		return fmt.Errorf("stream/vcard: %w", ErrAlreadyClosed)
	}
	if err := e.codec.Encode(e.bw, c); err != nil {
		return err
	}
	return nil
}

// Close flushes any buffered writes to the underlying io.Writer. Safe
// to call on an encoder that has never seen Encode (no-op flush).
//
// Returns ErrAlreadyClosed if called more than once.
func (e *VCardEncoder) Close() error {
	if e.closed {
		return fmt.Errorf("stream/vcard: %w", ErrAlreadyClosed)
	}
	if err := e.bw.Flush(); err != nil {
		return err
	}
	e.closed = true
	return nil
}
