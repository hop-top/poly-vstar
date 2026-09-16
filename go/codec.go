// SPDX-License-Identifier: MIT

package vstar

import "io"

// Parser reads a single Calendar from an io.Reader. Implementations MUST
// be safe for concurrent use across distinct calls (i.e. construct one
// codec, share it). Callers that care about latency are expected to
// pass an already-buffered Reader (e.g. *bufio.Reader); implementations
// SHOULD NOT wrap with their own buffering layers.
//
// Parse returns Calendar{} together with an error wrapping one of the
// package sentinels (ErrMalformed, ErrUnclosedBlock,
// ErrUnsupportedVersion, …) on failure. Use errors.Is to dispatch.
type Parser interface {
	Parse(r io.Reader) (Calendar, error)
}

// Encoder writes a Calendar to an io.Writer in the codec's wire
// format. Encode does NOT close the Writer — the caller owns its
// lifecycle. Encode returns nil on success and a wrapped error on
// failure (a partial write may have occurred — the caller should
// treat the Writer's contents as undefined on error).
type Encoder interface {
	Encode(w io.Writer, c Calendar) error
}

// Codec composes Parser and Encoder. Concrete codec packages
// (codec/rfc5545, codec/rfc6350, …) expose a New() Codec constructor
// returning an implementation of all three interfaces.
type Codec interface {
	Parser
	Encoder
}
