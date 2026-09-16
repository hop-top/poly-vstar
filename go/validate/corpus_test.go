// SPDX-License-Identifier: MIT

package validate_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/validate"
)

// corpusCalendar parses testdata/<dir>/<name> into a Calendar.
func corpusCalendar(t *testing.T, dir, name string) vstar.Calendar {
	t.Helper()
	path := filepath.Join("..", "testdata", dir, name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	cal, err := rfc5545.Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return cal
}

// TestStatusVocabulary_corpusIsClean walks every VCALENDAR fixture
// under testdata/rfc5545 and testdata/supersession and asserts none
// yields VS044: the conformance corpus carries only STATUS values
// inside their component type's vocabulary (spec/05 §7).
func TestStatusVocabulary_corpusIsClean(t *testing.T) {
	for _, dir := range []string{"rfc5545", "supersession"} {
		entries, err := os.ReadDir(filepath.Join("..", "testdata", dir))
		if err != nil {
			t.Fatalf("read testdata/%s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".ics") {
				continue
			}
			t.Run(dir+"/"+e.Name(), func(t *testing.T) {
				cal := corpusCalendar(t, dir, e.Name())
				for _, d := range validate.Validate(cal) {
					if d.Code == validate.CodeStatusNotInVocabulary {
						t.Errorf("unexpected VS044 at %s: %s", d.Path, d.Message)
					}
				}
			})
		}
	}
}

// TestStatusVocabulary_fixtureWithJournalStatusOnEventIsVS044 takes
// the clean VEVENT fixture, rewrites its STATUS to a value that is
// legal for VJOURNAL only, and asserts VS044 fires at the STATUS
// path — the cross-type shape the rule exists for.
func TestStatusVocabulary_fixtureWithJournalStatusOnEventIsVS044(t *testing.T) {
	cal := corpusCalendar(t, "rfc5545", "vevent_status_class_transp.ics")
	if len(cal.Components) != 1 || cal.Components[0].Type != vstar.CompEvent {
		t.Fatalf("fixture shape changed: want exactly one VEVENT, got %d components", len(cal.Components))
	}
	if hasCode(validate.Validate(cal), validate.CodeStatusNotInVocabulary) {
		t.Fatalf("fixture must be clean before the STATUS rewrite")
	}

	cal.Components[0].Set(vstar.Property{Name: "STATUS", Value: string(vstar.JournalDraft)})
	d, ok := findCode(validate.Validate(cal), validate.CodeStatusNotInVocabulary)
	if !ok {
		t.Fatalf("STATUS=DRAFT on the fixture VEVENT must yield VS044")
	}
	if want := "VCALENDAR.VEVENT[uid=evt-status-1].STATUS"; d.Path != want {
		t.Errorf("VS044 path = %q, want %q", d.Path, want)
	}
}
