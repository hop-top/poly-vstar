// SPDX-License-Identifier: MIT

package rfc6350

import (
	"fmt"
	"io"
	"strings"

	vstar "hop.top/vstar"
)

// supportedVersion is the only VERSION value accepted per
// the codec-rfc6350 plan. The spec/03 open question on accepting 3.0
// on read is parked: until the spec resolves it, we reject anything
// other than 4.0 with ErrUnsupportedVersion.
const supportedVersion = "4.0"

// parser is the concrete vCard 4.0 reader behind the package-level
// Codec. It is stateless and safe for concurrent use across goroutines.
//
// Most callers should use New() (which returns the Parser interface)
// or NewParser() (which returns the concrete struct for direct
// embedding).
type parser struct{}

// NewParser returns a fresh Parser implementation. Use this when you
// only need the read side of the codec; for full Codec composition
// use New.
func NewParser() Parser { return &parser{} }

// Parse consumes the entire stream from r, parsing zero or more
// BEGIN:VCARD … END:VCARD blocks. Returns ErrMalformed for syntactic
// errors (missing VERSION, stray END, nested BEGIN), ErrUnsupportedVersion
// when VERSION is present but not "4.0", and ErrUnclosedBlock when an
// EOF interrupts an open VCARD.
//
// Empty input returns (nil, nil) — callers MUST treat a zero-length
// slice as "no cards" rather than an error.
//
// UID handling is asymmetric across the codec layer:
// Parse accepts a VCARD with no UID property (Card.UID is "");
// Encoder rejects an empty Card.UID with ErrMissingUID;
// validate.Validate emits VS001 for the missing UID at the
// semantic layer. The parser is intentionally permissive —
// adopters that need strict-on-read can wrap with validate
// before storing.
func (p *parser) Parse(r io.Reader) ([]vstar.Card, error) {
	lines, err := unfold(r)
	if err != nil {
		return nil, err
	}

	var (
		cards   []vstar.Card
		curOpen bool
		cur     vstar.Card
		curVer  string
	)

	for i, line := range lines {
		if line == "" {
			continue
		}
		if strings.EqualFold(line, "BEGIN:VCARD") {
			if curOpen {
				return nil, fmt.Errorf("rfc6350: line %d: nested BEGIN:VCARD: %w", i+1, vstar.ErrMalformed)
			}
			curOpen = true
			cur = vstar.Card{}
			curVer = ""
			continue
		}
		if strings.EqualFold(line, "END:VCARD") {
			if !curOpen {
				return nil, fmt.Errorf("rfc6350: line %d: stray END:VCARD: %w", i+1, vstar.ErrMalformed)
			}
			if curVer == "" {
				return nil, fmt.Errorf("rfc6350: VCARD missing VERSION: %w", vstar.ErrMalformed)
			}
			if curVer != supportedVersion {
				return nil, fmt.Errorf("rfc6350: VERSION:%s: %w", curVer, vstar.ErrUnsupportedVersion)
			}
			cards = append(cards, cur)
			curOpen = false
			cur = vstar.Card{}
			curVer = ""
			continue
		}
		if !curOpen {
			// Outside any VCARD block: ignore non-empty lines? Reject
			// to keep parser strict. Real vCards never have content
			// outside BEGIN/END.
			return nil, fmt.Errorf("rfc6350: line %d: content outside VCARD: %w", i+1, vstar.ErrMalformed)
		}

		prop, err := parseContentLine(line)
		if err != nil {
			return nil, fmt.Errorf("rfc6350: line %d: %w", i+1, err)
		}
		// Extract VERSION; do not store on Card.Props.
		if strings.EqualFold(prop.Name, "VERSION") {
			if curVer != "" {
				return nil, fmt.Errorf("rfc6350: line %d: duplicate VERSION: %w", i+1, vstar.ErrMalformed)
			}
			curVer = prop.Value
			continue
		}
		// Extract UID into Card.UID.
		if strings.EqualFold(prop.Name, "UID") {
			cur.UID = prop.Value
			continue
		}
		// Extract KIND into Card.Kind (lower-case per RFC 6350 §6.1.4
		// IANA registry; values are case-insensitive on read).
		if strings.EqualFold(prop.Name, "KIND") {
			cur.Kind = vstar.Kind(strings.ToLower(prop.Value))
			continue
		}
		cur.Props = append(cur.Props, prop)
	}

	if curOpen {
		return nil, fmt.Errorf("rfc6350: %w", vstar.ErrUnclosedBlock)
	}
	return cards, nil
}
