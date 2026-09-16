// SPDX-License-Identifier: MIT

package duration

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	vstar "hop.top/vstar"
)

// Property names this file reads off components. Centralized so
// goconst stays quiet and renames stay sound.
const (
	propTRIGGER  = "TRIGGER"
	propDURATION = "DURATION"
	propREPEAT   = "REPEAT"
)

// Parameter names and values from RFC 5545 §3.2.
const (
	paramRELATED = "RELATED"
	paramVALUE   = "VALUE"

	valueDURATION = "DURATION"
	valueDATETIME = "DATE-TIME"
)

// ErrNoTrigger signals a VALARM with no TRIGGER property. RFC 5545
// §3.6.6 makes TRIGGER mandatory on VALARM, so this is a producer
// bug rather than an absent-optional; callers MUST match with
// errors.Is.
var ErrNoTrigger = errors.New("no TRIGGER property")

// ErrNoAnchor signals that a relative Trigger cannot be resolved
// because its parent component lacks the anchor the RELATED
// parameter selects — no DTSTART for RELATED=START, or no
// DTEND/DURATION/DUE for RELATED=END.
//
// This is reported rather than silently resolving against the zero
// time, which would place every such alarm in year 1.
var ErrNoAnchor = errors.New("parent component has no anchor for a relative trigger")

// Related identifies which end of the parent component a relative
// TRIGGER is measured from, per the RFC 5545 §3.2.14 RELATED
// parameter.
type Related int

// RELATED parameter values. RelatedStart is the zero value because
// it is also the RFC default when the parameter is absent.
const (
	// RelatedStart anchors the trigger to the parent's start
	// (DTSTART). This is the default when RELATED is absent.
	RelatedStart Related = iota
	// RelatedEnd anchors the trigger to the parent's end (DTEND,
	// or DTSTART+DURATION, or a VTODO's DUE).
	RelatedEnd
)

// String returns the RFC wire spelling ("START", "END").
func (r Related) String() string {
	switch r {
	case RelatedStart:
		return "START"
	case RelatedEnd:
		return "END"
	default:
		return "unknown"
	}
}

// Trigger is a parsed RFC 5545 §3.8.6.3 TRIGGER property: either a
// relative offset from one end of the parent component, or an
// absolute instant.
//
// Exactly one of the two forms is populated. When Relative is true,
// Duration and Related carry the offset and its anchor and Absolute
// is the zero time; when Relative is false, Absolute carries the
// instant and Duration is the zero Duration.
type Trigger struct {
	// Relative distinguishes the two forms.
	Relative bool
	// Duration is the offset, valid only when Relative is true. A
	// negative Duration fires before the anchor — the common case.
	Duration Duration
	// Related selects the anchor, valid only when Relative is true.
	Related Related
	// Absolute is the firing instant, valid only when Relative is
	// false.
	Absolute time.Time
}

// ParseTrigger decodes a TRIGGER property into a Trigger.
//
// The value form is chosen as follows:
//
//   - VALUE=DURATION, or no VALUE parameter with a value that parses
//     as a duration → relative.
//   - VALUE=DATE-TIME, or no VALUE parameter with a value that
//     parses as an RFC 5545 form #2 UTC instant → absolute.
//
// An explicit VALUE parameter is authoritative: a value that
// contradicts it is vstar.ErrMalformed rather than being silently
// re-read as the other form. With no VALUE parameter the two value
// shapes are unambiguous, so the value itself decides — producers in
// the wild routinely omit the parameter.
//
// RELATED is honored on relative triggers only; RFC 5545 §3.2.14
// scopes the parameter to DURATION-valued triggers, so RELATED on
// an absolute trigger is rejected. Both the parameter name and its
// value are matched case-insensitively per RFC 5545 §3.2.
//
// Every failure wraps vstar.ErrMalformed.
func ParseTrigger(p vstar.Property) (Trigger, error) {
	declared, hasValueParam := paramValue(p, paramVALUE)

	var t Trigger
	switch {
	case hasValueParam && strings.EqualFold(declared, valueDURATION):
		d, err := Parse(p.Value)
		if err != nil {
			return Trigger{}, fmt.Errorf("trigger: VALUE=DURATION but value is not a duration: %w", err)
		}
		t = Trigger{Relative: true, Duration: d}

	case hasValueParam && strings.EqualFold(declared, valueDATETIME):
		at, ok := vstar.ParseTime(p.Value)
		if !ok {
			return Trigger{}, fmt.Errorf(
				"trigger: VALUE=DATE-TIME but value %q is not an RFC 5545 form #2 instant: %w",
				p.Value, vstar.ErrMalformed,
			)
		}
		t = Trigger{Absolute: at}

	case hasValueParam:
		return Trigger{}, fmt.Errorf(
			"trigger: unsupported VALUE=%s (want DURATION or DATE-TIME): %w",
			declared, vstar.ErrMalformed,
		)

	default:
		// No VALUE parameter — infer from the value's shape.
		if d, err := Parse(p.Value); err == nil {
			t = Trigger{Relative: true, Duration: d}
			break
		}
		at, ok := vstar.ParseTime(p.Value)
		if !ok {
			return Trigger{}, fmt.Errorf(
				"trigger: value %q is neither a DURATION nor a DATE-TIME: %w",
				p.Value, vstar.ErrMalformed,
			)
		}
		t = Trigger{Absolute: at}
	}

	related, hasRelated := paramValue(p, paramRELATED)
	if !hasRelated {
		return t, nil
	}
	if !t.Relative {
		return Trigger{}, fmt.Errorf(
			"trigger: RELATED is meaningful only on a relative trigger (RFC 5545 §3.2.14): %w",
			vstar.ErrMalformed,
		)
	}
	switch {
	case strings.EqualFold(related, "START"):
		t.Related = RelatedStart
	case strings.EqualFold(related, "END"):
		t.Related = RelatedEnd
	default:
		return Trigger{}, fmt.Errorf(
			"trigger: unknown RELATED=%s (want START or END): %w", related, vstar.ErrMalformed,
		)
	}
	return t, nil
}

// AlarmTrigger reads and parses the TRIGGER property of a VALARM.
// Returns ErrNoTrigger when the property is absent, and whatever
// ParseTrigger reports otherwise.
func AlarmTrigger(alarm vstar.Component) (Trigger, error) {
	p, ok := alarm.Get(propTRIGGER)
	if !ok {
		return Trigger{}, fmt.Errorf("duration: VALARM has no TRIGGER: %w", ErrNoTrigger)
	}
	return ParseTrigger(p)
}

// Property renders t back to the wire property it came from.
//
// A relative trigger emits its duration as the value, adding
// RELATED=END only when the anchor is the end — RELATED=START is the
// RFC default and is left implicit. An absolute trigger emits the
// UTC form #2 instant and carries VALUE=DATE-TIME explicitly, so
// consumers never have to infer the form.
func (t Trigger) Property() vstar.Property {
	if !t.Relative {
		return vstar.Property{
			Name:   propTRIGGER,
			Params: []vstar.Param{{Name: paramVALUE, Value: valueDATETIME}},
			Value:  vstar.FormatTime(t.Absolute),
		}
	}
	p := vstar.Property{Name: propTRIGGER, Value: t.Duration.String()}
	if t.Related == RelatedEnd {
		p.Params = []vstar.Param{{Name: paramRELATED, Value: "END"}}
	}
	return p
}

// Resolve computes the instant at which t fires.
//
// An absolute trigger returns its instant directly and ignores
// parent and cal. A relative trigger resolves its anchor from
// parent — DTSTART for RelatedStart, and for RelatedEnd the end as
// computed by EventEnd (DTEND, else DTSTART+DURATION) or a VTODO's
// DUE — then offsets it with Duration.AddTo, so calendar days and
// weeks honor zone transitions.
//
// cal supplies the VTIMEZONE registry used to resolve TZID-bearing
// anchors; pass the parent's calendar, or the zero Calendar when
// anchors are plain UTC.
//
// Returns ErrNoAnchor when the required anchor is absent or
// unparseable.
func (t Trigger) Resolve(parent vstar.Component, cal vstar.Calendar) (time.Time, error) {
	if !t.Relative {
		return t.Absolute, nil
	}
	anchor, ok := t.anchor(parent, cal)
	if !ok {
		return time.Time{}, fmt.Errorf(
			"duration: %s has no %s anchor: %w", parent.Type, t.Related, ErrNoAnchor,
		)
	}
	return t.Duration.AddTo(anchor), nil
}

// anchor returns the parent instant the trigger is measured from.
func (t Trigger) anchor(parent vstar.Component, cal vstar.Calendar) (time.Time, bool) {
	if t.Related == RelatedStart {
		return parent.DTSTART(cal)
	}
	// RelatedEnd: a VTODO ends at DUE; everything else at DTEND or
	// DTSTART+DURATION.
	if parent.Type == vstar.CompTodo {
		if due, ok := parent.DUE(cal); ok {
			return due, true
		}
	}
	return EventEnd(parent, cal)
}

// EventEnd returns the end instant of a component that expresses it
// either as DTEND or as DTSTART plus a DURATION, per RFC 5545
// §3.6.1 (which allows exactly one of the two on a VEVENT).
//
// DTEND wins when both are present — it is the explicit statement.
// Falling back, DURATION is applied to DTSTART with Duration.AddTo
// so a "P1D" event keeps its wall-clock end across a DST
// transition.
//
// Returns ok=false when neither form is available, when DTSTART is
// missing for the DURATION form, or when the DURATION value is
// malformed.
func EventEnd(c vstar.Component, cal vstar.Calendar) (time.Time, bool) {
	if end, ok := c.DTEND(cal); ok {
		return end, true
	}
	p, ok := c.Get(propDURATION)
	if !ok {
		return time.Time{}, false
	}
	d, err := Parse(p.Value)
	if err != nil {
		return time.Time{}, false
	}
	start, ok := c.DTSTART(cal)
	if !ok {
		return time.Time{}, false
	}
	return d.AddTo(start), true
}

// AlarmRepeatCycle reads the VALARM DURATION/REPEAT pair from
// RFC 5545 §3.8.6.2 and §3.8.6.3: the interval between repetitions
// and how many additional times the alarm repeats after its initial
// trigger.
//
// The two properties travel together — the RFC requires that if one
// is present the other must be too. Returns (zero, 0, nil) when
// neither is present, and vstar.ErrMalformed when only one is,
// when the DURATION value is invalid, or when REPEAT is not a
// non-negative integer.
func AlarmRepeatCycle(alarm vstar.Component) (Duration, int, error) {
	durProp, hasDur := alarm.Get(propDURATION)
	repProp, hasRep := alarm.Get(propREPEAT)

	switch {
	case !hasDur && !hasRep:
		return Duration{}, 0, nil
	case hasDur && !hasRep:
		return Duration{}, 0, fmt.Errorf(
			"duration: VALARM has DURATION without REPEAT (RFC 5545 §3.8.6.2): %w", vstar.ErrMalformed,
		)
	case !hasDur && hasRep:
		return Duration{}, 0, fmt.Errorf(
			"duration: VALARM has REPEAT without DURATION (RFC 5545 §3.8.6.2): %w", vstar.ErrMalformed,
		)
	}

	d, err := Parse(durProp.Value)
	if err != nil {
		return Duration{}, 0, fmt.Errorf("duration: VALARM DURATION: %w", err)
	}
	repeat, convErr := strconv.Atoi(repProp.Value)
	if convErr != nil {
		return Duration{}, 0, fmt.Errorf(
			"duration: VALARM REPEAT %q is not an integer: %w", repProp.Value, vstar.ErrMalformed,
		)
	}
	if repeat < 0 {
		return Duration{}, 0, fmt.Errorf(
			"duration: VALARM REPEAT %d is negative: %w", repeat, vstar.ErrMalformed,
		)
	}
	return d, repeat, nil
}

// paramValue returns the value of the first parameter matching name
// (case-insensitive per RFC 5545 §3.2), and whether one was found.
func paramValue(p vstar.Property, name string) (string, bool) {
	for _, par := range p.Params {
		if strings.EqualFold(par.Name, name) {
			return par.Value, true
		}
	}
	return "", false
}
