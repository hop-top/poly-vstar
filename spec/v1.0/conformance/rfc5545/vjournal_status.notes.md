# vjournal_status.ics

A VCALENDAR containing a single VJOURNAL that carries a
VJOURNAL-scoped STATUS.

## Shape

- VCALENDAR with PRODID + VERSION
- One VJOURNAL with UID, DTSTAMP, DTSTART, SUMMARY, `STATUS:FINAL`

## Why

Round-trip + canonical + hash coverage for the VJOURNAL STATUS
vocabulary (DRAFT / FINAL / CANCELLED) per spec/05 criterion 7.
`FINAL` exists only in the VJOURNAL vocabulary, so a validator that
checks STATUS against the VEVENT or VTODO lists by mistake reports
this fixture as invalid; a conformant one reports nothing.

## RFC anchors

- RFC 5545 §3.6.3 — VJOURNAL component definition.
- RFC 5545 §3.8.1.11 — STATUS property values.
