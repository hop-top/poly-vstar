// SPDX-License-Identifier: MIT

package helpers

import (
	"fmt"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/duration"
	"hop.top/vstar/hashing"
)

// propACTION is the RFC 5545 §3.8.6.1 property name.
const propACTION = "ACTION"

// NewRelativeAlarm returns a fresh VALARM whose TRIGGER is a
// relative offset from one end of its parent component — the
// overwhelmingly common alarm shape, and the one Apple Reminders
// and Thunderbird emit.
//
// A negative offset fires before the anchor, so a 15-minute warning
// is:
//
//	helpers.NewRelativeAlarm(uid, "DISPLAY",
//		duration.Duration{Negative: true, Minutes: 15},
//		duration.RelatedStart)
//
// RELATED=START is the RFC default and is left implicit in the
// emitted property; RelatedEnd emits RELATED=END.
//
// Unlike NewAlarm, which takes the trigger as an unvalidated wire
// string, the offset here is a typed value that cannot encode a
// malformed duration. NewAlarm remains available and unchanged.
//
// UID, DTSTAMP=now (UTC), and ACTION are set as for NewAlarm, and
// X-VSTAR-HASH is computed last. Returns ErrMissingUID when uid is
// empty.
func NewRelativeAlarm(uid, action string, offset duration.Duration, related duration.Related) (vstar.Component, error) {
	if uid == "" {
		return vstar.Component{}, fmt.Errorf("helpers.NewRelativeAlarm: %w", vstar.ErrMissingUID)
	}
	trigger := duration.Trigger{Relative: true, Duration: offset, Related: related}
	return newAlarmWithTrigger(uid, action, trigger.Property()), nil
}

// NewAbsoluteAlarm returns a fresh VALARM whose TRIGGER is a fixed
// instant rather than an offset. The emitted property carries
// VALUE=DATE-TIME explicitly so consumers never infer the form, and
// the instant is written in UTC form #2.
//
// UID, DTSTAMP=now (UTC), and ACTION are set as for NewAlarm, and
// X-VSTAR-HASH is computed last. Returns ErrMissingUID when uid is
// empty.
func NewAbsoluteAlarm(uid, action string, at time.Time) (vstar.Component, error) {
	if uid == "" {
		return vstar.Component{}, fmt.Errorf("helpers.NewAbsoluteAlarm: %w", vstar.ErrMissingUID)
	}
	trigger := duration.Trigger{Absolute: at}
	return newAlarmWithTrigger(uid, action, trigger.Property()), nil
}

// newAlarmWithTrigger assembles a VALARM around an already-rendered
// TRIGGER property, applying the shared UID/DTSTAMP/ACTION seeding
// and the hash-last discipline.
func newAlarmWithTrigger(uid, action string, trigger vstar.Property) vstar.Component {
	c := stampUID(vstar.Component{Type: vstar.CompAlarm}, uid)
	c.Set(vstar.Property{Name: propACTION, Value: action})
	c.Set(trigger)
	hashing.SetXVSTAR(&c)
	return c
}

// AlarmFiresAt resolves the instant at which alarm fires, given the
// parent component it hangs off and that parent's calendar.
//
// A relative trigger is offset from the anchor its RELATED
// parameter selects — DTSTART for START, and for END the parent's
// DTEND, DTSTART+DURATION, or a VTODO's DUE. Calendar days and
// weeks are advanced by date, so an offset crossing a DST
// transition keeps its wall-clock meaning. An absolute trigger
// returns its instant directly.
//
// cal supplies the VTIMEZONE registry for TZID-bearing anchors;
// pass the zero Calendar when the parent's times are plain UTC.
//
// Errors wrap duration.ErrNoTrigger when the VALARM has no TRIGGER,
// duration.ErrNoAnchor when the parent lacks the required anchor,
// and vstar.ErrMalformed when the TRIGGER value does not parse.
func AlarmFiresAt(alarm, parent vstar.Component, cal vstar.Calendar) (time.Time, error) {
	tr, err := duration.AlarmTrigger(alarm)
	if err != nil {
		return time.Time{}, err
	}
	return tr.Resolve(parent, cal)
}
