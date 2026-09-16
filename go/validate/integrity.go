// SPDX-License-Identifier: MIT

package validate

import (
	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
)

// checkHashIntegrity recomputes c's content hash via
// hashing.VerifyXVSTAR and emits an Error diagnostic when the
// stored X-VSTAR-HASH is present but does not match.
//
// Absent X-VSTAR-HASH is intentionally NOT flagged here — that
// case is owned by checkRequiredCommon (VS003). This split keeps
// the diagnostic surface unambiguous: present-but-wrong is a
// different bug than absent.
func checkHashIntegrity(c vstar.Component, path string) []Diagnostic {
	if _, ok := c.Get(xvstarHashProperty); !ok {
		// Absence is handled by VS003. Stay silent here.
		return nil
	}
	ok, want, got := hashing.VerifyXVSTAR(c)
	if ok {
		return nil
	}
	return []Diagnostic{{
		Severity: SeverityError,
		Code:     CodeBadXVSTARHash,
		Message:  "X-VSTAR-HASH does not match recomputed canonical hash; want=" + want + " got=" + got + " (spec/05 §2)",
		Path:     path + "." + xvstarHashProperty,
	}}
}
