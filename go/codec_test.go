// SPDX-License-Identifier: MIT

package vstar

import (
	"bytes"
	"io"
	"testing"
)

// stubCodec is a test-local zero-effort implementation used to prove the
// Parser/Encoder/Codec interfaces compose correctly.
type stubCodec struct{}

func (stubCodec) Parse(_ io.Reader) (Calendar, error)  { return Calendar{}, nil }
func (stubCodec) Encode(_ io.Writer, _ Calendar) error { return nil }

// TestCodecInterfaces verifies that the three exported interfaces are
// structurally satisfied by a single value, that Parser and Encoder are
// independently usable, and that Codec embeds both. This is the
// load-bearing contract for codec/rfc5545 and codec/rfc6350.
func TestCodecInterfaces(t *testing.T) {
	t.Parallel()

	var c Codec = stubCodec{}
	var p Parser = c
	var e Encoder = c

	if _, err := p.Parse(bytes.NewReader(nil)); err != nil {
		t.Fatalf("Parser.Parse: unexpected error: %v", err)
	}
	if err := e.Encode(io.Discard, Calendar{}); err != nil {
		t.Fatalf("Encoder.Encode: unexpected error: %v", err)
	}
}
