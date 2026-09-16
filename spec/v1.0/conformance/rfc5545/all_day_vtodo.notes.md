# all_day_vtodo.ics

A VCALENDAR containing a single all-day VTODO: both DTSTART and DUE
are date-only (`VALUE=DATE`).

## Shape

- VCALENDAR with PRODID + VERSION
- One VTODO with UID, DTSTAMP, `DTSTART;VALUE=DATE`, `DUE;VALUE=DATE`,
  SUMMARY

## Why

Round-trip + canonical + hash coverage for RFC 5545 §3.3.4 DATE
values under rule 11 of `03-canonicalization.md`: the eight-octet
value is emitted verbatim, the `VALUE=DATE` parameter is retained,
and the value is never promoted to a midnight DATE-TIME. The
`.canonical` sibling keeps `VALUE=DATE` on both properties — strip
it and `20260515` reads as a malformed DATE-TIME, since the default
value type for DTSTART/DUE is DATE-TIME.

`all_day_vtodo_variant.ics` is the same component with a
non-canonical DUE spelling; its `.canonical` and `.hash` MUST be
byte-identical to this fixture's.

## RFC anchors

- RFC 5545 §3.3.4 — DATE value type.
- RFC 5545 §3.2.20 — VALUE parameter.
- RFC 5545 §3.8.2.3 / §3.8.2.4 — DUE / DTSTART default to DATE-TIME.
