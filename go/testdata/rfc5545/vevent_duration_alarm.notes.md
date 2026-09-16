# vevent_duration_alarm.ics

A VCALENDAR containing a single VEVENT that states its end as a
DURATION instead of a DTEND, carrying a VALARM anchored to that end
which repeats.

## Shape

- VCALENDAR with PRODID + VERSION
- One VEVENT with UID, DTSTAMP, DTSTART, `DURATION:P1D` (no DTEND),
  SUMMARY
- One nested VALARM with ACTION, `TRIGGER;RELATED=END:-P1D`,
  `REPEAT:2`, `DURATION:PT5M`, DESCRIPTION

## Why

Coverage for the DURATION-not-DTEND event form (RFC 5545 §3.6.1), a
relative TRIGGER measured from the event's end, and the VALARM
DURATION/REPEAT pair that travels together. Every duration value
passes through canonical form verbatim (spec/03 rule 12): `P1D`
stays `P1D` and the leading sign on the trigger is kept, so the
canonical bytes differ from the input only by property order. The
alarm resolves to DTSTART + P1D − P1D, i.e. the event's own DTSTART,
and then repeats twice at five-minute intervals.

## RFC anchors

- RFC 5545 §3.3.6 — DURATION value type.
- RFC 5545 §3.6.1 — VEVENT with DURATION in place of DTEND.
- RFC 5545 §3.8.6.2 — REPEAT property (paired with DURATION).
- RFC 5545 §3.8.6.3 — TRIGGER property, RELATED=END.
