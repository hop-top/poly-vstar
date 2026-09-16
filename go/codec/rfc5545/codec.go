// SPDX-License-Identifier: MIT

package rfc5545

import (
	"io"

	vstar "hop.top/vstar"
)

// codec is the concrete implementation of vstar.Codec for RFC 5545.
// It composes Parse and Encode without holding any state, so a single
// instance is safe for concurrent use across distinct calls.
type codec struct{}

// Parse satisfies vstar.Parser by delegating to the package-level
// Parse function.
func (codec) Parse(r io.Reader) (vstar.Calendar, error) { return Parse(r) }

// Encode satisfies vstar.Encoder by delegating to the package-level
// Encode function.
func (codec) Encode(w io.Writer, c vstar.Calendar) error { return Encode(w, c) }

// New returns a fresh vstar.Codec implementation backed by the
// RFC 5545 parser and encoder. The returned value is stateless — it
// is safe to share or reconstruct freely.
func New() vstar.Codec { return codec{} }

// Default is a package-level vstar.Codec callers can use directly
// when they don't need to construct their own. Equivalent to
// rfc5545.New() but pre-allocated at init.
var Default vstar.Codec = New()
