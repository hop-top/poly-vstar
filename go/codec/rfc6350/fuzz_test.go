// SPDX-License-Identifier: MIT

package rfc6350

import (
	"bytes"
	"errors"
	"testing"

	vstar "hop.top/vstar"
)

// FuzzParse_RFC6350 verifies the parser is panic-free across arbitrary
// byte input. Any error returned MUST wrap one of the package
// sentinels (ErrMalformed, ErrUnclosedBlock, ErrUnsupportedVersion,
// ErrMissingUID is encoder-only). On success, the parsed Cards MUST
// re-encode without panic — round-trip is best-effort because the
// parser is lenient on whitespace and the encoder is strict.
//
// Seed corpus lives at testdata/fuzz/FuzzParse_RFC6350/ and includes
// the minimal VCARD plus a few escaping/folding edge cases.
func FuzzParse_RFC6350(f *testing.F) {
	seed := [][]byte{
		[]byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u\r\nFN:Jad\r\nEND:VCARD\r\n"),
		[]byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u\r\nhome.TEL:tel:+15555550100\r\nEND:VCARD\r\n"),
		[]byte(`BEGIN:VCARD` + "\r\n" + `VERSION:4.0` + "\r\n" + `UID:u` + "\r\n" + `N:Last\,Comma;Jad;;;` + "\r\n" + `END:VCARD` + "\r\n"),
		[]byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:u\r\nEND:VCARD\r\n"),
		[]byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u\r\n"), // unclosed
		[]byte("END:VCARD\r\n"),    // stray END
		[]byte(""),                 // empty
		[]byte("garbage no colon"), // malformed
	}
	for _, s := range seed {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, in []byte) {
		cards, err := NewParser().Parse(bytes.NewReader(in))
		if err != nil {
			// Any error MUST be one of the documented sentinels.
			if !isExpectedSentinel(err) {
				t.Fatalf("unexpected error class: %v (input=%q)", err, in)
			}
			return
		}
		// Successful parse: re-encode each card and confirm no panic.
		// Encoder rejects empty UID; that's expected for inputs that
		// happen to omit UID — we don't fail on it.
		for _, c := range cards {
			var buf bytes.Buffer
			_ = NewEncoder().Encode(&buf, c)
		}
	})
}

func isExpectedSentinel(err error) bool {
	return errors.Is(err, vstar.ErrMalformed) ||
		errors.Is(err, vstar.ErrUnclosedBlock) ||
		errors.Is(err, vstar.ErrUnsupportedVersion) ||
		errors.Is(err, vstar.ErrMissingUID)
}
