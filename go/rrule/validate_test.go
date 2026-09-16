// SPDX-License-Identifier: MIT

package rrule

import (
	"errors"
	"testing"
)

// TestValidateRRule_MirrorsParseRRule shares the parseCases table
// with TestParseRRule and asserts ValidateRRule returns the same
// error category (sentinel) on every input. Equality of err is
// not asserted — wrapping context may differ — only errors.Is
// equivalence on the target sentinel.
func TestValidateRRule_MirrorsParseRRule(t *testing.T) {
	for _, c := range parseCases() {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateRRule(c.in)
			if c.wantErr != nil {
				if err == nil {
					t.Fatalf("ValidateRRule(%q): want %v, got nil", c.in, c.wantErr)
				}
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("ValidateRRule(%q): want errors.Is(%v), got %v", c.in, c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateRRule(%q): unexpected error: %v", c.in, err)
			}
		})
	}
}
