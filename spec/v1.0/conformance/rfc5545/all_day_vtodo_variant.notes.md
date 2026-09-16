# all_day_vtodo_variant.ics

`all_day_vtodo.ics` with a non-canonical DUE spelling:
`DUE;TZID=America/Montreal;VALUE=date:20260516`. Every other byte
is identical to the clean fixture.

## Shape

- VCALENDAR with PRODID + VERSION
- One VTODO with UID, DTSTAMP, `DTSTART;VALUE=DATE`, SUMMARY, and a
  DUE carrying a lower-case `VALUE=date` plus a stray TZID

## Why

Pins the two normalizing halves of rule 11 in
`03-canonicalization.md`: the `VALUE` parameter's argument is
upper-cased (`VALUE=date` → `VALUE=DATE`) and a `TZID` parameter on
a DATE is stripped (RFC 5545 §3.2.19 scopes TZID to DATE-TIME and
TIME values). No VTIMEZONE for `America/Montreal` is present, and
none is needed: the resolution registry is not consulted for DATE.

Assertion: this fixture's `.canonical` and `.hash` are byte-identical
to `all_day_vtodo.canonical` and `all_day_vtodo.hash`. A divergence
means either the case-fold or the TZID strip regressed.

## RFC anchors

- RFC 5545 §3.2 — parameter names and registered tokens are
  case-insensitive.
- RFC 5545 §3.2.19 — TZID MUST NOT be applied to DATE properties.
- RFC 5545 §3.3.4 — DATE value type.
