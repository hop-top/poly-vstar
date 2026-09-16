// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"path/filepath"

	vstar "hop.top/vstar"
)

// tzidFixture is one entry of time/tzid.json: an RFC 5545 form #1
// (local, no Z) value read against a named zone, and the form #2
// UTC instant it resolves to.
//
// Calendar names the conformance fixture supplying the VTIMEZONE
// registry — spec/v0.1/conformance/time/<calendar>.ics. A port
// loads that .ics, then resolves Value against TZID.
//
// UTC is null where the reference returns ok=false: an unknown
// zone, an empty TZID, a form #2 value (already absolute — the
// TZID-aware entry point rejects it rather than double-applying an
// offset), or a value that is not a form #1 timestamp at all. Null
// is the whole contract for a rejection; there is no error class
// here because the API reports a boolean, not an error.
type tzidFixture struct {
	Calendar string  `json:"calendar"`
	TZID     string  `json:"tzid"`
	Value    string  `json:"value"`
	UTC      *string `json:"utc"`
}

// timeCalendars are the conformance-corpus VTIMEZONE registries the
// tzid table resolves against, keyed by the name the fixture
// records. Reused rather than re-minted for the same reason the
// supersession family reuses its inputs: one .ics per scenario.
var timeCalendars = []string{"america_montreal", "utc_only"}

// writeTimeFamily writes time/tzid.json.
func writeTimeFamily(dir string) (int, error) {
	module, err := moduleRoot()
	if err != nil {
		return 0, err
	}
	corpus := filepath.Join(filepath.Dir(module), specDirName, "v0.1", "conformance", "time")
	cals := map[string]vstar.Calendar{}
	for _, name := range timeCalendars {
		cal, err := readCalendar(filepath.Join(corpus, name+".ics"))
		if err != nil {
			return 0, err
		}
		cals[name] = cal
	}

	out := make([]tzidFixture, 0, len(tzidCases()))
	for _, tc := range tzidCases() {
		cal, ok := cals[tc.Calendar]
		if !ok {
			return 0, fmt.Errorf("tzid case references unknown calendar %q", tc.Calendar)
		}
		f := tzidFixture{Calendar: tc.Calendar, TZID: tc.TZID, Value: tc.Value}
		if got, ok := vstar.ParseTimeWithTZID(tc.Value, tc.TZID, cal); ok {
			f.UTC = strPtr(vstar.FormatTime(got))
		}
		out = append(out, f)
	}
	if err := writeJSON(filepath.Join(dir, "tzid.json"), out); err != nil {
		return 0, err
	}
	return 1, nil
}

// tzidCase is one (calendar, tzid, value) input triple. The
// expected UTC is not stated here — it is whatever the reference
// resolves, so the fixture cannot disagree with the implementation.
type tzidCase struct {
	Calendar string
	TZID     string
	Value    string
}

// winterLocal is the standard-time (EST) sample. It recurs across
// the rejection rows so each one differs from the resolving row by
// exactly one field — the zone, or the value's form.
const winterLocal = "20260104T133045"

// tzidCases draws its inputs from time_test.go: the two DST
// seasons, the transition instants themselves, the fixed-offset
// zone, and every rejection path.
func tzidCases() []tzidCase {
	const (
		mtl     = "america_montreal"
		utc     = "utc_only"
		zoneMTL = "America/Montreal"
	)
	return []tzidCase{
		// Standard time (EST, -0500).
		{mtl, zoneMTL, winterLocal},
		// Daylight time (EDT, -0400).
		{mtl, zoneMTL, "20260704T133045"},

		// DST boundary instants. Spring forward is the second
		// Sunday of March at 02:00 local; the minute before is
		// still EST and the hour after is EDT.
		{mtl, zoneMTL, "20260315T013000"},
		{mtl, zoneMTL, "20260315T020000"},
		{mtl, zoneMTL, "20260315T030000"},
		// Fall back is the first Sunday of November at 02:00.
		{mtl, zoneMTL, "20261101T013000"},
		{mtl, zoneMTL, "20261101T020000"},
		{mtl, zoneMTL, "20261101T030000"},

		// Fixed-offset zone: one STANDARD child, no DAYLIGHT.
		{utc, "UTC", "20260504T183045"},
		{utc, "UTC", "20260704T133045"},

		// Missing zone — the registry has no VTIMEZONE for it.
		{mtl, "America/Toronto", winterLocal},
		{utc, zoneMTL, winterLocal},
		// Empty TZID names no zone at all.
		{mtl, "", winterLocal},

		// Form #2 rejected: the value is already an absolute UTC
		// instant, so there is no local wall time to resolve.
		{mtl, zoneMTL, "20260315T060000Z"},
		// Date-only and free-form values are not form #1 either.
		{mtl, zoneMTL, "20260315"},
		{mtl, zoneMTL, "2026-03-15T02:00:00"},
		{mtl, zoneMTL, ""},
	}
}
