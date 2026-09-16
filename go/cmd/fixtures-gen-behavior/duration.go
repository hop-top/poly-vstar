// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"path/filepath"

	vstar "hop.top/vstar"
	"hop.top/vstar/duration"
	"hop.top/vstar/helpers"
)

// durationParseFixture is one entry of duration/parse.json.
//
// Success carries Seconds (the whole duration as a signed second
// count — the one representation every target language has) and
// Negative (the sign flag, kept separate because a zero-length
// negative duration is legal RFC 5545 and Seconds alone cannot
// express it).
//
// Failure carries Error instead: the failure-class token from
// spec/05 §Failure classes, never a message. `error` replaces
// `seconds`/`negative` — a port branches on the field's presence.
type durationParseFixture struct {
	Value    string `json:"value"`
	Seconds  *int64 `json:"seconds,omitempty"`
	Negative *bool  `json:"negative,omitempty"`
	Error    string `json:"error,omitempty"`
}

// triggerFixture is one entry of a <name>.trigger.json file: the
// instant a VALARM fires, or the class its resolution failed with.
//
// AlarmUID keys the entry to the VALARM inside the .ics. FiresAt is
// RFC 5545 form #2 (UTC, Z-suffixed). Error replaces FiresAt on
// failure, naming ErrNoAnchor, ErrNoTrigger or ErrMalformed.
type triggerFixture struct {
	AlarmUID string `json:"alarm_uid"`
	FiresAt  string `json:"fires_at,omitempty"`
	Error    string `json:"error,omitempty"`
}

// writeDurationFamily writes duration/parse.json plus one .ics +
// .trigger.json pair per alarm scenario.
func writeDurationFamily(dir string) (int, error) {
	if err := writeJSON(filepath.Join(dir, "parse.json"), durationParseFixtures()); err != nil {
		return 0, err
	}
	n := 1
	cases, err := triggerCases()
	if err != nil {
		return 0, err
	}
	for _, tc := range cases {
		results := make([]triggerFixture, 0, len(tc.cal.Components))
		for _, parent := range tc.cal.Components {
			for _, alarm := range parent.Sub {
				if alarm.Type != vstar.CompAlarm {
					continue
				}
				results = append(results, resolveAlarm(alarm, parent, tc.cal))
			}
		}
		if err := assertTriggerExpectation(tc, results); err != nil {
			return 0, err
		}
		if err := writeCalendar(filepath.Join(dir, tc.name+".ics"), tc.cal); err != nil {
			return 0, err
		}
		if err := writeJSON(filepath.Join(dir, tc.name+".trigger.json"), results); err != nil {
			return 0, err
		}
		n += 2
	}
	return n, nil
}

// resolveAlarm runs the reference and projects the outcome onto the
// fixture shape, mapping a failure onto its spec/05 class token.
func resolveAlarm(alarm, parent vstar.Component, cal vstar.Calendar) triggerFixture {
	f := triggerFixture{AlarmUID: alarm.UID()}
	at, err := helpers.AlarmFiresAt(alarm, parent, cal)
	if err != nil {
		f.Error = triggerErrorClass(err)
		return f
	}
	f.FiresAt = vstar.FormatTime(at)
	return f
}

// The failure-class tokens from spec/05 §Failure classes that these
// families can record. Unclassified is the escape hatch: a failure
// the reference grew that maps to none of them shows up under a name
// no port recognizes, rather than silently under a wrong one.
const (
	classMalformed    = "ErrMalformed"
	classNoTrigger    = "ErrNoTrigger"
	classNoAnchor     = "ErrNoAnchor"
	classUnclassified = "Unclassified"
)

// Wire names the trigger fixtures build properties from.
const (
	propTRIGGER   = "TRIGGER"
	paramRELATED  = "RELATED"
	paramVALUE    = "VALUE"
	relatedSTART  = "START"
	relatedEND    = "END"
	valueDATETIME = "DATE-TIME"
	valueDURATION = "DURATION"
)

// triggerErrorClass maps a resolution failure onto its corpus
// token. ErrNoTrigger and ErrNoAnchor are checked first: they are
// the specific classes, and a future implementation that also wraps
// ErrMalformed around them must still report the specific one.
func triggerErrorClass(err error) string {
	switch {
	case errors.Is(err, duration.ErrNoTrigger):
		return classNoTrigger
	case errors.Is(err, duration.ErrNoAnchor):
		return classNoAnchor
	case errors.Is(err, vstar.ErrMalformed):
		return classMalformed
	default:
		return classUnclassified
	}
}

// durNegative15M appears both as a parse input and as the value of
// the relative triggers, which is the point: the same wire duration
// means the same thing in both families.
const durNegative15M = "-PT15M"

// durationParseValues is the RFC 5545 §3.3.6 value space
// duration/parse.json is generated from. A package-level var rather
// than a literal inside the generator so the consuming test can
// rebuild the table from the committed inputs and compare — which
// is what proves the fixture states what the reference reports,
// instead of merely proving the generator agrees with itself.
var durationParseValues = []string{
	// Weeks form.
	"P1W",
	"P26W",
	// Days form.
	"P7D",
	// Time-only forms.
	"PT1H",
	"PT15M",
	"PT30S",
	// Combined day-and-time form.
	"P1DT2H30M45S",
	"PT1H30M",
	// Negative.
	durNegative15M,
	"-P1DT2H",
	// Zero-length, both signs. The sign survives on a zero
	// duration, which is why the fixture keeps `negative`
	// alongside `seconds`.
	"PT0S",
	"-PT0S",
	// Explicit plus sign (RFC 5545 permits it).
	"+PT15M",
	// Malformed: empty, no designator, no body, week/unit mix,
	// week/time mix, unknown unit, unit out of order, digits
	// with no unit, unit with no digits, lowercase.
	"",
	"T15M",
	"P",
	"P1W2D",
	"P1WT1H",
	"PT1X",
	"PT1M1H",
	"P1DT30",
	"PT1HM",
	"pt15m",
}

// durationParseFixtures runs duration.Parse over
// durationParseValues and records what the reference reports.
func durationParseFixtures() []durationParseFixture {
	out := make([]durationParseFixture, 0, len(durationParseValues))
	for _, v := range durationParseValues {
		f := durationParseFixture{Value: v}
		d, err := duration.Parse(v)
		if err != nil {
			// Every duration.Parse failure wraps ErrMalformed;
			// classify rather than assume so a new class shows up
			// as "Unclassified" instead of a wrong token.
			if errors.Is(err, vstar.ErrMalformed) {
				f.Error = classMalformed
			} else {
				f.Error = classUnclassified
			}
			out = append(out, f)
			continue
		}
		secs := int64(d.Signed() / 1e9)
		neg := d.IsNegative()
		f.Seconds = &secs
		f.Negative = &neg
		out = append(out, f)
	}
	return out
}

// triggerCase is one generated alarm scenario.
type triggerCase struct {
	name string
	// wantErrors lists the class tokens the scenario must produce
	// (empty means every alarm must resolve).
	wantErrors []string
	cal        vstar.Calendar
}

// assertTriggerExpectation checks the scenario produced the failure
// classes it was authored for, so a refactor cannot hollow it out.
func assertTriggerExpectation(tc triggerCase, got []triggerFixture) error {
	if len(got) == 0 {
		return fmt.Errorf("trigger case %q: no VALARM found", tc.name)
	}
	seen := map[string]bool{}
	for _, f := range got {
		if f.Error != "" {
			seen[f.Error] = true
		}
	}
	for _, want := range tc.wantErrors {
		if !seen[want] {
			return fmt.Errorf("trigger case %q: expected class %s, got %v", tc.name, want, seen)
		}
	}
	if len(tc.wantErrors) == 0 && len(seen) != 0 {
		return fmt.Errorf("trigger case %q: expected every alarm to resolve, got %v", tc.name, seen)
	}
	return nil
}

// triggerCases enumerates the alarm scenarios: the two relative
// anchors, the absolute form, and the two failure classes.
func triggerCases() ([]triggerCase, error) {
	const (
		dtstart = "20260601T090000Z"
		dtend   = "20260601T170000Z"
		due     = "20260601T170000Z"
	)
	cases := []triggerCase{
		{
			name: "relative_start",
			// RELATED=START is the RFC default; the fixture states
			// it explicitly and a second alarm leaves it implicit,
			// so a port that ignores the parameter fails on one of
			// the two.
			cal: eventWithAlarms(
				"Duration-RelativeStart", dtstart, dtend,
				alarm("alarm-start-explicit", vstar.Property{
					Name:   propTRIGGER,
					Params: []vstar.Param{{Name: paramRELATED, Value: relatedSTART}},
					Value:  durNegative15M,
				}),
				alarm("alarm-start-implicit", prop(propTRIGGER, "-PT30M")),
			),
		},
		{
			name: "relative_end",
			cal: eventWithAlarms(
				"Duration-RelativeEnd", dtstart, dtend,
				alarm("alarm-end", vstar.Property{
					Name:   propTRIGGER,
					Params: []vstar.Param{{Name: paramRELATED, Value: relatedEND}},
					Value:  "-PT10M",
				}),
			),
		},
		{
			name: "absolute",
			cal: eventWithAlarms(
				"Duration-Absolute", dtstart, dtend,
				alarm("alarm-absolute", vstar.Property{
					Name:   propTRIGGER,
					Params: []vstar.Param{{Name: paramVALUE, Value: valueDATETIME}},
					Value:  "20260531T220000Z",
				}),
				// Same form with the VALUE parameter omitted: the
				// value's shape decides.
				alarm("alarm-absolute-implicit", prop(propTRIGGER, "20260531T230000Z")),
			),
		},
		{
			name:       "missing_anchor",
			wantErrors: []string{classNoAnchor},
			// A VEVENT with no DTEND and no DURATION cannot anchor
			// a RELATED=END trigger.
			cal: eventWithAlarms(
				"Duration-MissingAnchor", dtstart, "",
				alarm("alarm-no-anchor", vstar.Property{
					Name:   propTRIGGER,
					Params: []vstar.Param{{Name: paramRELATED, Value: relatedEND}},
					Value:  "-PT10M",
				}),
			),
		},
		{
			name:       "value_contradiction",
			wantErrors: []string{classMalformed},
			// VALUE=DATE-TIME is authoritative: a duration value
			// under it is malformed, not silently re-read as
			// relative.
			cal: eventWithAlarms(
				"Duration-ValueContradiction", dtstart, dtend,
				alarm("alarm-value-contradiction", vstar.Property{
					Name:   propTRIGGER,
					Params: []vstar.Param{{Name: paramVALUE, Value: valueDATETIME}},
					Value:  durNegative15M,
				}),
				// The mirror image: VALUE=DURATION over an instant.
				alarm("alarm-value-contradiction-inverse", vstar.Property{
					Name:   propTRIGGER,
					Params: []vstar.Param{{Name: paramVALUE, Value: valueDURATION}},
					Value:  "20260531T220000Z",
				}),
			),
		},
		{
			name:       "missing_trigger",
			wantErrors: []string{classNoTrigger},
			// TRIGGER is mandatory on a VALARM (RFC 5545 §3.6.6);
			// without it the alarm cannot be scheduled at all.
			cal: eventWithAlarms(
				"Duration-MissingTrigger", dtstart, dtend,
				alarmWithoutTrigger("alarm-no-trigger"),
			),
		},
		{
			name: "vtodo_due_anchor",
			// RELATED=END on a VTODO anchors to DUE, not DTEND.
			cal: todoWithAlarms(
				"Duration-VTODODue", due,
				alarm("alarm-todo-due", vstar.Property{
					Name:   propTRIGGER,
					Params: []vstar.Param{{Name: paramRELATED, Value: relatedEND}},
					Value:  "-PT1H",
				}),
			),
		},
	}
	return cases, nil
}

// alarm builds a VALARM carrying trigger, stamped and hashed.
func alarm(uid string, trigger vstar.Property) vstar.Component {
	return hashed(withProps(
		bare(vstar.CompAlarm),
		prop("UID", uid),
		prop("DTSTAMP", stampTime),
		prop("ACTION", "DISPLAY"),
		prop("DESCRIPTION", "Reminder"),
		trigger,
	))
}

// alarmWithoutTrigger builds the ErrNoTrigger provocation.
func alarmWithoutTrigger(uid string) vstar.Component {
	return hashed(withProps(
		bare(vstar.CompAlarm),
		prop("UID", uid),
		prop("DTSTAMP", stampTime),
		prop("ACTION", "DISPLAY"),
		prop("DESCRIPTION", "Reminder"),
	))
}

// eventWithAlarms wraps alarms in a VEVENT. An empty dtend omits
// the property, which is how the missing-anchor case is built.
func eventWithAlarms(slug, dtstart, dtend string, alarms ...vstar.Component) vstar.Calendar {
	c := withProps(
		bare(vstar.CompEvent),
		prop("UID", "event-"+slug),
		prop("DTSTAMP", stampTime),
		prop("DTSTART", dtstart),
	)
	if dtend != "" {
		c.Set(prop("DTEND", dtend))
	}
	c.Sub = append(c.Sub, alarms...)
	return oneComponent(slug, hashed(c))
}

// todoWithAlarms wraps alarms in a VTODO anchored by DUE.
func todoWithAlarms(slug, due string, alarms ...vstar.Component) vstar.Calendar {
	c := withProps(
		bare(vstar.CompTodo),
		prop("UID", "todo-"+slug),
		prop("DTSTAMP", stampTime),
		prop("DUE", due),
	)
	c.Sub = append(c.Sub, alarms...)
	return oneComponent(slug, hashed(c))
}
