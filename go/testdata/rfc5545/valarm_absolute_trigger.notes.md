# valarm_absolute_trigger.ics

A VCALENDAR containing a single VEVENT whose VALARM fires at an
absolute instant rather than at an offset from the event.

## Shape

- VCALENDAR with PRODID + VERSION
- One VEVENT with UID, DTSTAMP, DTSTART, DTEND, SUMMARY
- One nested VALARM with ACTION,
  `TRIGGER;VALUE=DATE-TIME:20260515T090000Z`, DESCRIPTION

## Why

Coverage for the absolute form of TRIGGER: an explicit
`VALUE=DATE-TIME` parameter with a UTC form #2 instant, written the
way spec/03 "TRIGGER conventions" says emitters SHOULD write it, and
carrying no RELATED parameter (which applies to relative triggers
only). The trigger is not a duration, so canonical form passes both
the parameter and the value through untouched; the alarm fires at
the stated instant regardless of the event's DTSTART.

## RFC anchors

- RFC 5545 §3.2.14 — RELATED parameter (relative triggers only).
- RFC 5545 §3.3.5 — DATE-TIME value type, form #2 (UTC).
- RFC 5545 §3.8.6.3 — TRIGGER property, VALUE=DATE-TIME form.
