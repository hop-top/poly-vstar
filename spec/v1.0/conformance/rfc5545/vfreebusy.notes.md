# vfreebusy.ics

A VCALENDAR containing a single VFREEBUSY component with two
FREEBUSY periods.

## Shape

- VCALENDAR with PRODID + VERSION
- One VFREEBUSY with UID, DTSTAMP, DTSTART, DTEND, ORGANIZER, two
  FREEBUSY properties (each carrying FBTYPE=BUSY)

## Why

Coverage for the VFREEBUSY component type and for properties that
carry a parameter (FBTYPE) — the canonical sort within a property
must keep parameters in alphabetical order. Two FREEBUSY entries
exercise the same-name multi-property path.

## RFC anchors

- RFC 5545 §3.6.4 — VFREEBUSY component definition.
- RFC 5545 §3.8.2.6 — FREEBUSY property + FBTYPE parameter.
