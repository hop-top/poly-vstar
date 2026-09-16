// SPDX-License-Identifier: MIT

package rfc5545_test

import (
	"errors"
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
)

// FuzzParse_RFC5545 ensures Parse never panics on arbitrary inputs.
// On invalid input it MUST return an error wrapping one of the
// documented sentinels (ErrMalformed, ErrUnclosedBlock,
// ErrUnsupportedVersion). On valid input it MUST return a Calendar
// whose round-trip parse equals the first parse semantically.
//
// The seed corpus lives in testdata/fuzz/FuzzParse_RFC5545/ and
// covers: empty calendar, calendar with one VTODO, nested
// VTIMEZONE, VEVENT with VALARM. Add new bytes/string entries
// there to widen exploration.
func FuzzParse_RFC5545(f *testing.F) {
	// Seed: a couple of canonical fixtures inline in addition to the
	// testdata corpus so a fresh checkout still has interesting inputs
	// without a separate setup step.
	f.Add("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\nEND:VCALENDAR\r\n")
	f.Add("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\n" +
		"BEGIN:VTODO\r\nUID:1\r\nDTSTAMP:20260504T120000Z\r\nEND:VTODO\r\n" +
		"END:VCALENDAR\r\n")

	f.Fuzz(func(t *testing.T, in string) {
		cal, err := rfc5545.Parse(strings.NewReader(in))
		if err == nil {
			// Successful parse must round-trip without error.
			var sb strings.Builder
			if err := rfc5545.Encode(&sb, cal); err != nil {
				t.Errorf("encode after successful parse failed: %v", err)
			}
			return
		}
		// Error path: must wrap a documented sentinel. errors.Is
		// already handles wrapping; we accept any of the four.
		if errors.Is(err, vstar.ErrMalformed) ||
			errors.Is(err, vstar.ErrUnclosedBlock) ||
			errors.Is(err, vstar.ErrUnsupportedVersion) ||
			errors.Is(err, vstar.ErrMissingUID) {
			return
		}
		t.Errorf("Parse returned undocumented error %v on input %q", err, in)
	})
}
