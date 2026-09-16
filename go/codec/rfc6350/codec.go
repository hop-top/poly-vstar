// SPDX-License-Identifier: MIT

package rfc6350

import (
	"io"

	vstar "hop.top/vstar"
)

// Parser reads zero or more vCards from r per RFC 6350. It is the
// vCard analog of vstar.Parser (which is Calendar-typed); vCards
// have a different top-level shape, so the codec defines its own
// interface here rather than reusing the root one.
type Parser interface {
	Parse(r io.Reader) ([]vstar.Card, error)
}

// Encoder writes a single vCard to w per RFC 6350. Encode does NOT
// close the writer; the caller owns its lifecycle.
type Encoder interface {
	Encode(w io.Writer, c vstar.Card) error
}

// Codec composes Parser and Encoder. Construct with New; or use the
// package-level Default.
type Codec interface {
	Parser
	Encoder
}

// codec embeds the concrete *parser and *encoder types defined in
// parser.go and encoder.go. Both are stateless structs, so a single
// codec value is safe for concurrent use across goroutines.
type codec struct {
	*parser
	*encoder
}

// New returns a Codec that satisfies Parser and Encoder. The returned
// value is safe for concurrent use across goroutines.
func New() Codec {
	return &codec{parser: &parser{}, encoder: &encoder{}}
}

// Default is a package-level Codec instance for callers that do not
// need an isolated configuration. It is constructed at package init
// and is safe for concurrent use across goroutines.
//
//nolint:gochecknoglobals // intentional public default.
var Default = New()
