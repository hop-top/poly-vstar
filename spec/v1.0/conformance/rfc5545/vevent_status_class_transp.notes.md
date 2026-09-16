# vevent_status_class_transp.ics

A VCALENDAR containing a single VEVENT that carries the three
enumerated VEVENT properties spec/05 criterion 7 scopes to the
component type.

## Shape

- VCALENDAR with PRODID + VERSION
- One VEVENT with UID, DTSTAMP, DTSTART, DTEND, SUMMARY,
  `STATUS:CONFIRMED`, `CLASS:PRIVATE`, `TRANSP:TRANSPARENT`

## Why

Round-trip + canonical + hash coverage for the VEVENT-scoped
STATUS vocabulary (TENTATIVE / CONFIRMED / CANCELLED) alongside
CLASS and TRANSP. The values are deliberately the non-default
choices (RFC 5545 defaults are `CLASS:PUBLIC` and
`TRANSP:OPAQUE`) so a consumer that drops explicit values in favor
of the defaults changes the canonical bytes and fails the hash.

Every value here is inside its vocabulary, so a validator MUST NOT
report a vocabulary diagnostic for this fixture.

## RFC anchors

- RFC 5545 §3.6.1 — VEVENT component definition.
- RFC 5545 §3.8.1.11 — STATUS property values.
- RFC 5545 §3.8.1.3 — CLASS property.
- RFC 5545 §3.8.2.7 — TRANSP property.
