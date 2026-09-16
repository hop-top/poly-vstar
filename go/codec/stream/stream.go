// SPDX-License-Identifier: MIT

// Package stream provides constant-memory iterator-style codecs for V*
// wire formats. Where the batch codecs (codec/rfc5545, codec/rfc6350)
// load an entire Calendar or Card slice into memory, the stream codecs
// in this package surface one Component or Card at a time so callers
// processing large ledgers (AGR L2 projection, AGR L7 playthrough fork
// comparison) can keep memory flat regardless of input size.
//
// The package exposes two interface pairs:
//
//   - VCALENDAR: VCalendarParser yields one Component per Next() call;
//     VCalendarEncoder writes a BEGIN:VCALENDAR header on first Encode
//     and an END:VCALENDAR trailer on Close.
//   - VCARD: VCardParser yields one Card per Next() call; VCardEncoder
//     writes one self-contained BEGIN:VCARD…END:VCARD block per Encode.
//
// All implementations are designed for sequential, single-goroutine
// use; callers wanting concurrent reads or writes must construct one
// stream codec per goroutine. Backpressure and cancellation are the
// caller's responsibility — wrap the underlying io.Reader / io.Writer
// with a context-aware adapter if needed.
package stream

import (
	"errors"

	vstar "hop.top/vstar"
)

// StreamParser is the iterator-style read interface for VCALENDAR
// streaming codecs. Each call to Next consumes one component's worth
// of input and returns the parsed Component, or io.EOF when the input
// is exhausted (END:VCALENDAR seen for VCALENDAR streams).
//
// Implementations MUST return errors wrapping the package-level
// vstar sentinels (ErrMalformed, ErrUnclosedBlock, …) where applicable
// so callers can dispatch with errors.Is.
//
// StreamParser is intentionally small so callers may compose it with
// any underlying codec (rfc5545, rfc6350, or future formats); concrete
// constructors live alongside their codec-specific implementations
// (NewVCalendarParser, NewVCardParser).
type StreamParser interface {
	Next() (vstar.Component, error)
}

// StreamEncoder is the iterator-style write interface for VCALENDAR
// streaming codecs. Encode writes one component immediately to the
// underlying io.Writer; Close emits any required trailer (e.g.
// END:VCALENDAR) and flushes buffered output.
//
// Calling Close more than once returns ErrAlreadyClosed (wrapped via
// fmt.Errorf so errors.Is still matches). Calling Encode after Close
// also returns ErrAlreadyClosed.
//
// StreamEncoder does NOT close the underlying io.Writer; the caller
// owns the writer lifecycle.
type StreamEncoder interface {
	Encode(c vstar.Component) error
	Close() error
}

// CardStreamParser is the VCARD analog of StreamParser. It yields one
// Card per call to Next and returns io.EOF when the underlying reader
// is exhausted. VCARD streams are a sequence of self-contained
// BEGIN:VCARD…END:VCARD blocks with no enclosing wrapper, so there is
// no header to skip and no trailer to emit.
type CardStreamParser interface {
	Next() (vstar.Card, error)
}

// CardStreamEncoder is the VCARD analog of StreamEncoder. Each call to
// Encode writes one full BEGIN:VCARD…END:VCARD block. Close exists
// for symmetry with StreamEncoder and to release any buffered writes
// to the underlying io.Writer; calling it twice returns
// ErrAlreadyClosed. Calling Encode after Close returns ErrAlreadyClosed.
type CardStreamEncoder interface {
	Encode(c vstar.Card) error
	Close() error
}

// ErrAlreadyClosed is returned by stream encoders when Close is called
// more than once, or when Encode is called after Close. Wrapped via
// fmt.Errorf in production paths; callers should match with errors.Is.
var ErrAlreadyClosed = errors.New("stream encoder: already closed")

// ErrHeaderLocked is returned by VCalendarEncoder.SetHeader when the
// caller attempts to set the calendar header after the first Encode
// has already written it to the underlying io.Writer. The header is
// locked at the first Encode call so that the BEGIN:VCALENDAR /
// VERSION / PRODID / METHOD ordering on the wire is deterministic.
var ErrHeaderLocked = errors.New("stream encoder: header set after first encode")
