// SPDX-License-Identifier: MIT

package stream_test

import (
	"errors"
	"testing"

	"hop.top/vstar/codec/stream"
)

// TestErrAlreadyClosed_Identity sanity-checks that the package
// sentinel matches itself via errors.Is. Wrapped error matching is
// covered by the per-codec tests; this guards against an accidental
// re-declaration of the sentinel breaking that match.
func TestErrAlreadyClosed_Identity(t *testing.T) {
	t.Parallel()
	if !errors.Is(stream.ErrAlreadyClosed, stream.ErrAlreadyClosed) {
		t.Fatal("errors.Is(ErrAlreadyClosed, ErrAlreadyClosed) returned false")
	}
}

// TestErrHeaderLocked_Identity is the sibling sanity check for the
// SetHeader-after-Encode sentinel.
func TestErrHeaderLocked_Identity(t *testing.T) {
	t.Parallel()
	if !errors.Is(stream.ErrHeaderLocked, stream.ErrHeaderLocked) {
		t.Fatal("errors.Is(ErrHeaderLocked, ErrHeaderLocked) returned false")
	}
}
