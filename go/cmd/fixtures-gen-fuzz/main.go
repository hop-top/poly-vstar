// SPDX-License-Identifier: MIT

// Command fixtures-gen-fuzz writes the canonical fuzz seed corpora
// under the conformance corpus' fuzz-seed/rfc5545/ and
// fuzz-seed/rfc6350/ directories — spec/v0.1/conformance/ in the
// poly-vstar monorepo, testdata/ on the hop-top/vstar mirror (see
// corpusRoot). Each seed is a raw byte file; fixtures-verify mirrors
// them into the go-fuzz convention dirs at
// codec/<rfc>/testdata/fuzz/FuzzParse_RFC<RFC>/seed_<stem>.
//
// Seeds aim for diversity over depth: CRLF vs LF, folded lines,
// escaped TEXT, nested blocks, empty/blank inputs, weird
// whitespace, multi-card files. ≥10 each per the plan.
//
// All seeds are deterministic (no random bytes); re-running this
// command MUST produce byte-identical files.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type seed struct {
	stem  string
	bytes []byte
}

// rfc5545Seeds — VCALENDAR fuzz seeds.
var rfc5545Seeds = []seed{
	{"01_minimal", []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\nEND:VCALENDAR\r\n")},
	{"02_lf_only", []byte("BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\nEND:VCALENDAR\n")},
	{"03_one_vtodo", []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\nBEGIN:VTODO\r\nUID:1\r\nDTSTAMP:20260504T120000Z\r\nEND:VTODO\r\nEND:VCALENDAR\r\n")},
	{"04_folded_long_summary", []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\nBEGIN:VTODO\r\nUID:fold\r\nDTSTAMP:20260504T120000Z\r\nSUMMARY:this summary is intentionally long enough to require folding ac\r\n ross multiple lines per RFC 5545 §3.1\r\nEND:VTODO\r\nEND:VCALENDAR\r\n")},
	{"05_escaped_text", []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\nBEGIN:VTODO\r\nUID:esc\r\nDTSTAMP:20260504T120000Z\r\nDESCRIPTION:line1\\nline2\\;quoted\\,comma\\\\backslash\r\nEND:VTODO\r\nEND:VCALENDAR\r\n")},
	{"06_nested_vtimezone", []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\nBEGIN:VTIMEZONE\r\nTZID:America/Montreal\r\nBEGIN:STANDARD\r\nDTSTART:20211107T020000\r\nTZOFFSETFROM:-0400\r\nTZOFFSETTO:-0500\r\nEND:STANDARD\r\nEND:VTIMEZONE\r\nEND:VCALENDAR\r\n")},
	{"07_vevent_valarm", []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\nBEGIN:VEVENT\r\nUID:e1\r\nDTSTAMP:20260504T120000Z\r\nDTSTART:20260601T090000Z\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")},
	{"08_empty_input", []byte("")},
	{"09_only_begin", []byte("BEGIN:VCALENDAR\r\n")},
	{"10_blank_lines", []byte("\r\n\r\nBEGIN:VCALENDAR\r\n\r\nVERSION:2.0\r\nPRODID:p\r\n\r\nEND:VCALENDAR\r\n")},
	{"11_param_with_quote", []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\nBEGIN:VEVENT\r\nUID:q\r\nDTSTAMP:20260504T120000Z\r\nDTSTART;TZID=\"Etc/GMT+0\":20260601T090000\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")},
	{"12_mixed_eol", []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\nPRODID:p\r\nEND:VCALENDAR\n")},
}

// rfc6350Seeds — VCARD fuzz seeds.
var rfc6350Seeds = []seed{
	{"01_minimal", []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u\r\nFN:Jad\r\nEND:VCARD\r\n")},
	{"02_lf_only", []byte("BEGIN:VCARD\nVERSION:4.0\nUID:u\nFN:Jad\nEND:VCARD\n")},
	{"03_kind_org", []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nKIND:org\r\nUID:o1\r\nFN:Acme\r\nEND:VCARD\r\n")},
	{"04_kind_group", []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nKIND:group\r\nUID:g1\r\nFN:Team\r\nMEMBER:m1\r\nMEMBER:m2\r\nEND:VCARD\r\n")},
	{"05_grouped_props", []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:gp\r\nFN:Jad\r\nitem1.TEL:+1-555-0100\r\nitem1.X-LABEL:Mobile\r\nEND:VCARD\r\n")},
	{"06_escaped_text", []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:esc\r\nFN:Foo\\, Bar\r\nNOTE:line1\\nline2\\;quoted\\\\back\r\nEND:VCARD\r\n")},
	{"07_folded_long_fn", []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:fold\r\nFN:Long name that is intentionally folded across\r\n  multiple physical lines\r\nEND:VCARD\r\n")},
	{"08_two_cards", []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:a\r\nFN:A\r\nEND:VCARD\r\nBEGIN:VCARD\r\nVERSION:4.0\r\nUID:b\r\nFN:B\r\nEND:VCARD\r\n")},
	{"09_with_extensions", []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:ext\r\nFN:Ext\r\nX-VSTAR-FOO:v\r\nX-AGR-INTENT:focus\r\nX-EXP-XX:exp\r\nEND:VCARD\r\n")},
	{"10_empty_input", []byte("")},
	{"11_only_begin", []byte("BEGIN:VCARD\r\n")},
	{"12_v3_unsupported", []byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:u\r\nFN:Old\r\nEND:VCARD\r\n")},
}

func main() {
	root, err := moduleRoot()
	if err != nil {
		fail(err)
	}
	if err := writeSeeds(filepath.Join(corpusRoot(root), "fuzz-seed", "rfc5545"), rfc5545Seeds); err != nil {
		fail(err)
	}
	if err := writeSeeds(filepath.Join(corpusRoot(root), "fuzz-seed", "rfc6350"), rfc6350Seeds); err != nil {
		fail(err)
	}
	fmt.Printf("fixtures-gen-fuzz: wrote %d rfc5545 + %d rfc6350 seeds\n",
		len(rfc5545Seeds), len(rfc6350Seeds))
}

func writeSeeds(dir string, seeds []seed) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, s := range seeds {
		path := filepath.Join(dir, "seed_"+s.stem+".bytes")
		if err := os.WriteFile(path, s.bytes, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// moduleRoot finds the Go module root by walking up from the current
// working directory looking for testdata/rfc5545; see fixtures-verify.
func moduleRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		st, err := os.Stat(filepath.Join(dir, "testdata", "rfc5545"))
		if err == nil && st.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find testdata above %s", cwd)
		}
		dir = parent
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fixtures-gen-fuzz:", err)
	os.Exit(1)
}
