// SPDX-License-Identifier: MIT

package rfc5545

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// TestScanner_Reexport_SmokeTest verifies the rfc5545.Scanner type
// alias and NewScanner re-export still resolve and behave per
// RFC 5545 §3.1. The full behavior matrix lives in
// codec/internal/contentline/scanner_test.go.
func TestScanner_Reexport_SmokeTest(t *testing.T) {
	t.Parallel()

	src := "BEGIN:VCALENDAR\r\nDESCRIPTION:long descrip\r\n tion\r\nEND:VCALENDAR\r\n"
	want := []string{"BEGIN:VCALENDAR", "DESCRIPTION:long description", "END:VCALENDAR"}

	s := NewScanner(strings.NewReader(src))
	var got []string
	for {
		line, err := s.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Scanner.Next: %v", err)
		}
		got = append(got, line)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines want %d: %q", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("line %d: got %q want %q", i, got[i], want[i])
		}
	}
}
