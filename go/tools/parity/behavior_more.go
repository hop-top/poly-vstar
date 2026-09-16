// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/duration"
	"hop.top/vstar/ext"
	"hop.top/vstar/helpers"
)

// triggerClasses is the failure vocabulary alarm resolution
// reports, most specific first. ErrNoTrigger and ErrNoAnchor are
// the specific classes; an implementation that also wraps
// ErrMalformed around one of them must still report the specific
// one, which is what first-match ordering guarantees.
var triggerClasses = []sentinel{
	{"ErrNoTrigger", duration.ErrNoTrigger},
	{"ErrNoAnchor", duration.ErrNoAnchor},
	{classMalformed, vstar.ErrMalformed},
}

// durationClasses is the vocabulary duration.Parse failures report.
// Every failure wraps ErrMalformed today; classifying rather than
// assuming means a class the reference grows fails the emitter
// instead of traveling as a wrong token.
var durationClasses = []sentinel{
	{classMalformed, vstar.ErrMalformed},
}

// durationParseEntry is one parsed duration value. Seconds is the
// whole duration as a signed second count, the one representation
// every target language has. Negative is kept separate because a
// zero-length duration written with a leading sign cannot be
// distinguished by Seconds alone.
type durationParseEntry struct {
	Value    string `json:"value"`
	Seconds  *int64 `json:"seconds"`
	Negative *bool  `json:"negative"`
	Error    string `json:"error"`
}

// triggerEntry is when one VALARM fires, or the class its
// resolution failed with. AlarmUID keys the entry to the VALARM
// inside the .ics.
type triggerEntry struct {
	AlarmUID string `json:"alarm_uid"`
	FiresAt  string `json:"fires_at"`
	Error    string `json:"error"`
}

// emitDuration covers both duration families: the flat parse table
// at duration/parse.json, and one alarm-resolution list per
// <name>.ics.
//
// The parse table's inputs come from the committed fixture's
// `value` column rather than a list hard-coded here, so a value
// added to the corpus reaches every port without an emitter edit.
func emitDuration(dir string) (map[string]any, error) {
	root := filepath.Dir(dir)
	out := map[string]any{}

	values, err := durationParseValues(filepath.Join(dir, "parse.json"))
	if err != nil {
		return nil, err
	}
	entries := make([]durationParseEntry, 0, len(values))
	for _, v := range values {
		entries = append(entries, parseDuration(v))
	}
	out["duration/parse"] = entries

	err = eachFixture(dir, extICS, func(path, stem string) error {
		cal, err := readCalendar(path)
		if err != nil {
			return err
		}
		return record(out, root, stem, alarmEntries(cal))
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// durationParseValues reads the `value` column out of the
// committed parse.json. Only the inputs are read; what the
// reference makes of them is computed, never copied.
func durationParseValues(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var rows []struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Value)
	}
	return out, nil
}

// parseDuration reports what duration.Parse makes of one value:
// the signed second count and sign flag, or the failure class.
func parseDuration(value string) durationParseEntry {
	e := durationParseEntry{Value: value}
	d, err := duration.Parse(value)
	if err != nil {
		if token, ok := classify(err, durationClasses); ok {
			e.Error = token
		} else {
			e.Error = classUnclassified
		}
		return e
	}
	secs := int64(d.Signed() / 1e9)
	neg := d.IsNegative()
	e.Seconds = &secs
	e.Negative = &neg
	return e
}

// classUnclassified is the escape hatch for a failure the
// reference grew that matches no known class. It shows up under a
// name no port implements, which fails loudly, rather than
// silently under a wrong one.
const classUnclassified = "Unclassified"

// alarmEntries resolves every VALARM in the calendar, in document
// order: parent components in calendar order, VALARMs in the order
// they appear inside their parent.
func alarmEntries(cal vstar.Calendar) []triggerEntry {
	out := make([]triggerEntry, 0)
	for _, parent := range cal.Components {
		for _, alarm := range parent.Sub {
			if alarm.Type != vstar.CompAlarm {
				continue
			}
			out = append(out, resolveAlarm(alarm, parent, cal))
		}
	}
	return out
}

func resolveAlarm(alarm, parent vstar.Component, cal vstar.Calendar) triggerEntry {
	e := triggerEntry{AlarmUID: alarm.UID()}
	at, err := helpers.AlarmFiresAt(alarm, parent, cal)
	if err != nil {
		if token, ok := classify(err, triggerClasses); ok {
			e.Error = token
		} else {
			e.Error = classUnclassified
		}
		return e
	}
	e.FiresAt = vstar.FormatTime(at)
	return e
}

// scopeEntry is one extension name's spec/04 classification.
// System is the owning system's slug, and null for every scope but
// "system" — only a system-scoped name has an owner.
type scopeEntry struct {
	Name   string  `json:"name"`
	Scope  string  `json:"scope"`
	System *string `json:"system"`
}

// emitExt classifies every name in the committed ext/scopes.json,
// in the order the file lists them. The file is a flat table, not
// a set: order is part of what a port reproduces.
//
// The scope token is lowercased. Scope.String() is capitalized for
// human display; one flat convention spares every port a case
// mapping of its own.
func emitExt(dir string) (map[string]any, error) {
	path := filepath.Join(dir, "scopes.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var rows []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	entries := make([]scopeEntry, 0, len(rows))
	for _, r := range rows {
		e := scopeEntry{
			Name:  r.Name,
			Scope: strings.ToLower(ext.ScopeOf(r.Name).String()),
		}
		if sys, ok := ext.SystemName(r.Name); ok {
			e.System = strPtr(sys)
		}
		entries = append(entries, e)
	}
	return map[string]any{"ext/scopes": entries}, nil
}

// tzidEntry is one local timestamp resolved against a named zone.
// UTC is null for every rejection: the reference reports a boolean,
// not an error, so there is no failure class to name here.
type tzidEntry struct {
	Calendar string  `json:"calendar"`
	TZID     string  `json:"tzid"`
	Value    string  `json:"value"`
	UTC      *string `json:"utc"`
}

// emitTime resolves every (calendar, tzid, value) triple in the
// committed time/tzid.json, in file order.
//
// `calendar` names a conformance fixture supplying the VTIMEZONE
// registry, which is loaded from spec/v0.1/conformance/time/. Each
// registry is parsed once and reused, so a fixture's cost does not
// grow with the number of rows citing it.
func emitTime(conformance, dir string) (map[string]any, error) {
	path := filepath.Join(dir, "tzid.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var rows []struct {
		Calendar string `json:"calendar"`
		TZID     string `json:"tzid"`
		Value    string `json:"value"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	registries := map[string]vstar.Calendar{}
	for _, r := range rows {
		if _, ok := registries[r.Calendar]; ok {
			continue
		}
		cal, err := readCalendar(filepath.Join(conformance, "time", r.Calendar+".ics"))
		if err != nil {
			return nil, err
		}
		registries[r.Calendar] = cal
	}

	entries := make([]tzidEntry, 0, len(rows))
	for _, r := range rows {
		e := tzidEntry{Calendar: r.Calendar, TZID: r.TZID, Value: r.Value}
		if got, ok := vstar.ParseTimeWithTZID(r.Value, r.TZID, registries[r.Calendar]); ok {
			e.UTC = strPtr(vstar.FormatTime(got))
		}
		entries = append(entries, e)
	}
	return map[string]any{"time/tzid": entries}, nil
}
