// SPDX-License-Identifier: MIT

package vstar_test

import (
	"os"
	"path/filepath"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
)

// This file and its siblings in package vstar_test cover the parts of
// the time API whose input is a VTIMEZONE registry. They live in the
// external test package because the fixtures are read through
// codec/rfc5545, which imports hop.top/vstar — an internal test could
// not import it back without a cycle. Everything under test is
// exported, so nothing is lost by the move.
//
// Fixtures are read from testdata/time/, not from the authored corpus
// at spec/v1.0/conformance/time/. testdata/ is a generated mirror of
// that corpus (see cmd/fixtures-verify) and is the only copy that
// ships with the published module: on the hop-top/vstar mirror the
// module is the go/ subtree alone and there is no sibling spec/. A
// test that reached across to spec/ would pass here and fail for
// anyone who ran `go test hop.top/vstar/...` from the module proxy.
// Authoring still happens in spec/; `make fixtures-verify` fails the
// build on any drift between the two.

// loadTimeFixture parses testdata/time/<stem>.ics into a Calendar.
func loadTimeFixture(t *testing.T, stem string) vstar.Calendar {
	t.Helper()
	path := filepath.Join("testdata", "time", stem+".ics")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only fixture handle.

	cal, err := rfc5545.Parse(f)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return cal
}

// americaMontrealCalendar is the america_montreal.ics registry: a
// VTIMEZONE with STANDARD + DAYLIGHT children driven by FREQ=YEARLY
// RRULEs (the US/Canada DST shape).
func americaMontrealCalendar(t *testing.T) vstar.Calendar {
	t.Helper()
	return loadTimeFixture(t, "america_montreal")
}

// utcOnlyCalendar is the utc_only.ics registry: a STANDARD-only
// VTIMEZONE for the trivial fixed-offset path.
func utcOnlyCalendar(t *testing.T) vstar.Calendar {
	t.Helper()
	return loadTimeFixture(t, "utc_only")
}

// TestTimeFixture_MontrealShape asserts the fixture still decodes to
// the VTIMEZONE the TZID tests below assume. Those tests only ever
// observe resolved instants, so a fixture that lost its DAYLIGHT
// child or had an offset retyped could still produce plausible-
// looking failures somewhere else. This names the shape directly.
func TestTimeFixture_MontrealShape(t *testing.T) {
	cal := americaMontrealCalendar(t)
	if len(cal.Components) != 1 {
		t.Fatalf("got %d components, want 1", len(cal.Components))
	}
	tz := cal.Components[0]
	if tz.Type != vstar.CompTimezone {
		t.Errorf("component type = %q, want %q", tz.Type, vstar.CompTimezone)
	}
	if p, ok := tz.Get("TZID"); !ok || p.Value != "America/Montreal" {
		t.Errorf("TZID = %q (present=%v), want %q", p.Value, ok, "America/Montreal")
	}
	if len(tz.Sub) != 2 {
		t.Fatalf("got %d VTIMEZONE children, want 2 (STANDARD + DAYLIGHT)", len(tz.Sub))
	}

	want := map[vstar.CompType]map[string]string{
		vstar.CompType("DAYLIGHT"): {
			"DTSTART":      "20070311T020000",
			"TZOFFSETFROM": "-0500",
			"TZOFFSETTO":   "-0400",
			"TZNAME":       "EDT",
			"RRULE":        "FREQ=YEARLY;BYMONTH=3;BYDAY=2SU",
		},
		vstar.CompType("STANDARD"): {
			"DTSTART":      "20071104T020000",
			"TZOFFSETFROM": "-0400",
			"TZOFFSETTO":   "-0500",
			"TZNAME":       "EST",
			"RRULE":        "FREQ=YEARLY;BYMONTH=11;BYDAY=1SU",
		},
	}
	assertZoneRules(t, tz.Sub, want)
}

// TestTimeFixture_UTCOnlyShape asserts the single-STANDARD fixture
// still decodes to a zero-offset zone with no DAYLIGHT child.
func TestTimeFixture_UTCOnlyShape(t *testing.T) {
	cal := utcOnlyCalendar(t)
	if len(cal.Components) != 1 {
		t.Fatalf("got %d components, want 1", len(cal.Components))
	}
	tz := cal.Components[0]
	if tz.Type != vstar.CompTimezone {
		t.Errorf("component type = %q, want %q", tz.Type, vstar.CompTimezone)
	}
	if p, ok := tz.Get("TZID"); !ok || p.Value != "UTC" {
		t.Errorf("TZID = %q (present=%v), want %q", p.Value, ok, "UTC")
	}
	if len(tz.Sub) != 1 {
		t.Fatalf("got %d VTIMEZONE children, want 1 (STANDARD only)", len(tz.Sub))
	}
	assertZoneRules(t, tz.Sub, map[vstar.CompType]map[string]string{
		vstar.CompType("STANDARD"): {
			"DTSTART":      "19700101T000000",
			"TZOFFSETFROM": "+0000",
			"TZOFFSETTO":   "+0000",
			"TZNAME":       "UTC",
		},
	})
}

// assertZoneRules checks that subs contains exactly the component
// types in want, each carrying the named property values.
func assertZoneRules(t *testing.T, subs []vstar.Component, want map[vstar.CompType]map[string]string) {
	t.Helper()
	seen := map[vstar.CompType]bool{}
	for _, sub := range subs {
		props, ok := want[sub.Type]
		if !ok {
			t.Errorf("unexpected VTIMEZONE child %q", sub.Type)
			continue
		}
		if seen[sub.Type] {
			t.Errorf("duplicate VTIMEZONE child %q", sub.Type)
		}
		seen[sub.Type] = true
		for name, value := range props {
			p, ok := sub.Get(name)
			if !ok {
				t.Errorf("%s: %s missing", sub.Type, name)
				continue
			}
			if p.Value != value {
				t.Errorf("%s: %s = %q, want %q", sub.Type, name, p.Value, value)
			}
		}
	}
	for typ := range want {
		if !seen[typ] {
			t.Errorf("VTIMEZONE child %q missing", typ)
		}
	}
}
