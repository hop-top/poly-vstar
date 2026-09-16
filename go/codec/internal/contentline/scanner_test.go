// SPDX-License-Identifier: MIT

package contentline

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// scanAll drains a Scanner and returns all logical content lines.
func scanAll(t *testing.T, src string) []string {
	t.Helper()
	s := NewScanner(strings.NewReader(src))
	var got []string
	for {
		line, err := s.Next()
		if errors.Is(err, io.EOF) {
			return got
		}
		if err != nil {
			t.Fatalf("Scanner.Next: unexpected error: %v", err)
		}
		got = append(got, line)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestScanner_SimpleCRLF(t *testing.T) {
	t.Parallel()

	src := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n"
	want := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "END:VCALENDAR"}
	got := scanAll(t, src)
	if !equalSlices(got, want) {
		t.Fatalf("scanned\n got=%q\nwant=%q", got, want)
	}
}

func TestScanner_MixedLFAndCRLF(t *testing.T) {
	t.Parallel()

	src := "BEGIN:VCALENDAR\nVERSION:2.0\r\nEND:VCALENDAR\n"
	want := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "END:VCALENDAR"}
	got := scanAll(t, src)
	if !equalSlices(got, want) {
		t.Fatalf("scanned\n got=%q\nwant=%q", got, want)
	}
}

func TestScanner_TrailingPartialLine(t *testing.T) {
	t.Parallel()

	// No trailing newline — last logical line still surfaces.
	src := "BEGIN:VCALENDAR\r\nLAST:value"
	want := []string{"BEGIN:VCALENDAR", "LAST:value"}
	got := scanAll(t, src)
	if !equalSlices(got, want) {
		t.Fatalf("scanned\n got=%q\nwant=%q", got, want)
	}
}

func TestScanner_FoldedLine_SP(t *testing.T) {
	t.Parallel()

	// RFC 5545 §3.1: a CRLF followed by SP or HTAB is a fold point;
	// the SP/HTAB is consumed and the remainder is appended to the
	// previous logical line.
	src := "DESCRIPTION:This is a long descrip\r\n tion that has been folded\r\n"
	want := []string{"DESCRIPTION:This is a long description that has been folded"}
	got := scanAll(t, src)
	if !equalSlices(got, want) {
		t.Fatalf("scanned\n got=%q\nwant=%q", got, want)
	}
}

func TestScanner_FoldedLine_HTAB(t *testing.T) {
	t.Parallel()

	src := "DESCRIPTION:tab-folded\r\n\tcontinuation\r\n"
	want := []string{"DESCRIPTION:tab-foldedcontinuation"}
	got := scanAll(t, src)
	if !equalSlices(got, want) {
		t.Fatalf("scanned\n got=%q\nwant=%q", got, want)
	}
}

func TestScanner_MultipleFolds(t *testing.T) {
	t.Parallel()

	src := "X-LONG:abc\r\n def\r\n ghi\r\n"
	want := []string{"X-LONG:abcdefghi"}
	got := scanAll(t, src)
	if !equalSlices(got, want) {
		t.Fatalf("scanned\n got=%q\nwant=%q", got, want)
	}
}

func TestScanner_FoldAcross75OctetBoundary(t *testing.T) {
	t.Parallel()

	// Encoder behavior, simulated: NAME: + 70-byte value, fold, then 5 more.
	value := strings.Repeat("a", 70)
	src := "X-NAME:" + value + "\r\n " + "bbbbb\r\n"
	want := []string{"X-NAME:" + value + "bbbbb"}
	got := scanAll(t, src)
	if !equalSlices(got, want) {
		t.Fatalf("scanned\n got=%q\nwant=%q", got, want)
	}
}

func TestScanner_BlankLinesAreSkipped(t *testing.T) {
	t.Parallel()

	// Blank lines (CRLF on their own) are not content lines and the
	// scanner skips them silently — they are tolerated for robustness
	// against producers that include trailing whitespace lines.
	src := "BEGIN:VCALENDAR\r\n\r\nEND:VCALENDAR\r\n"
	want := []string{"BEGIN:VCALENDAR", "END:VCALENDAR"}
	got := scanAll(t, src)
	if !equalSlices(got, want) {
		t.Fatalf("scanned\n got=%q\nwant=%q", got, want)
	}
}

func TestScanner_EmptyInput(t *testing.T) {
	t.Parallel()

	got := scanAll(t, "")
	if len(got) != 0 {
		t.Fatalf("scanned %q from empty input; want none", got)
	}
}

// TestScanner_BlankLineMidFold_StripsLeadingWSP is the regression test
// for the bug that motivated the dedup. A blank physical line breaks
// the fold sequence (per RFC 5545 §3.1 — a fold is CRLF + WSP, not
// CRLF + CRLF + WSP). The pre-consolidation rfc5545 scanner returned
// " more" with a leading space here; the lenient handler inherited
// from rfc6350 strips that leading WSP and starts a fresh logical line.
//
// Required output: ["DESCRIPTION:start", "more"] — no leading space.
func TestScanner_BlankLineMidFold_StripsLeadingWSP(t *testing.T) {
	t.Parallel()

	src := "DESCRIPTION:start\r\n\r\n more\r\n"
	want := []string{"DESCRIPTION:start", "more"}
	got := scanAll(t, src)
	if !equalSlices(got, want) {
		t.Fatalf("blank-line-mid-fold:\n got=%q\nwant=%q", got, want)
	}
}

// TestScanner_LeadingWSPNoPending verifies the lenient-on-input shape:
// a WSP-prefixed line at the very start of the stream (no pending
// logical line) drops its leading WSP byte and starts a fresh line.
func TestScanner_LeadingWSPNoPending(t *testing.T) {
	t.Parallel()

	src := " orphan\r\nNEXT:value\r\n"
	want := []string{"orphan", "NEXT:value"}
	got := scanAll(t, src)
	if !equalSlices(got, want) {
		t.Fatalf("leading-WSP-no-pending:\n got=%q\nwant=%q", got, want)
	}
}

// TestScanner_VCardCases mirrors the table-driven cases from the
// pre-consolidation rfc6350 unfold tests — kept to guard against
// behavior drift on the vCard side.
func TestScanner_VCardCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "single line CRLF",
			in:   "FN:Jad\r\n",
			want: []string{"FN:Jad"},
		},
		{
			name: "single line LF only",
			in:   "FN:Jad\n",
			want: []string{"FN:Jad"},
		},
		{
			name: "two lines",
			in:   "FN:Jad\r\nN:Bitar;Jad;;;\r\n",
			want: []string{"FN:Jad", "N:Bitar;Jad;;;"},
		},
		{
			name: "fold with SP",
			in:   "FN:Jad B\r\n itar\r\n",
			want: []string{"FN:Jad Bitar"},
		},
		{
			name: "fold with HTAB",
			in:   "FN:Jad B\r\n\titar\r\n",
			want: []string{"FN:Jad Bitar"},
		},
		{
			name: "multi-fold",
			in:   "ADR:;;123 \r\n Main \r\n St;;;;\r\n",
			want: []string{"ADR:;;123 Main St;;;;"},
		},
		{
			name: "trailing line without CRLF",
			in:   "FN:Jad",
			want: []string{"FN:Jad"},
		},
		{
			name: "blank line between logical lines is dropped",
			in:   "FN:Jad\r\n\r\nN:Bitar\r\n",
			want: []string{"FN:Jad", "N:Bitar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanAll(t, tt.in)
			if !equalSlices(got, tt.want) {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

// TestUnfoldAll_DrainsScannerEagerly covers the eager helper used by
// the rfc6350 batch parser bridge.
func TestUnfoldAll_DrainsScannerEagerly(t *testing.T) {
	t.Parallel()

	src := "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Jad\r\nEND:VCARD\r\n"
	want := []string{"BEGIN:VCARD", "VERSION:4.0", "FN:Jad", "END:VCARD"}

	got, err := UnfoldAll(strings.NewReader(src))
	if err != nil {
		t.Fatalf("UnfoldAll: %v", err)
	}
	if !equalSlices(got, want) {
		t.Fatalf("UnfoldAll:\n got=%q\nwant=%q", got, want)
	}
}

func TestUnfoldAll_EmptyInput(t *testing.T) {
	t.Parallel()

	got, err := UnfoldAll(strings.NewReader(""))
	if err != nil {
		t.Fatalf("UnfoldAll: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("UnfoldAll empty: got %q want none", got)
	}
}
