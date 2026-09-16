// SPDX-License-Identifier: MIT

package vstar

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorSentinels_NonNilDistinct(t *testing.T) {
	sentinels := []error{
		ErrUnsupportedVersion,
		ErrMalformed,
		ErrUnclosedBlock,
		ErrMissingUID,
	}
	for i, e := range sentinels {
		if e == nil {
			t.Errorf("sentinel %d is nil", i)
		}
	}
	for i := range sentinels {
		for j := range sentinels {
			if i == j {
				continue
			}
			if errors.Is(sentinels[i], sentinels[j]) {
				t.Errorf("sentinels %d and %d are not distinct", i, j)
			}
		}
	}
}

func TestErrorSentinels_IsThroughWrap(t *testing.T) {
	cases := []struct {
		name     string
		sentinel error
	}{
		{"ErrUnsupportedVersion", ErrUnsupportedVersion},
		{"ErrMalformed", ErrMalformed},
		{"ErrUnclosedBlock", ErrUnclosedBlock},
		{"ErrMissingUID", ErrMissingUID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := fmt.Errorf("at line 42: %w", tc.sentinel)
			if !errors.Is(wrapped, tc.sentinel) {
				t.Errorf("errors.Is on wrapped %s = false, want true", tc.name)
			}
		})
	}
}

func TestErrorSentinels_HaveMessages(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{ErrUnsupportedVersion.Error(), "unsupported version"},
		{ErrMalformed.Error(), "malformed"},
		{ErrUnclosedBlock.Error(), "unclosed block"},
		{ErrMissingUID.Error(), "missing UID"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("Error() = %q, want %q", tc.got, tc.want)
			}
		})
	}
}
