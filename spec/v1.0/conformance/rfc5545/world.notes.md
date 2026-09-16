# world.ics

A VCALENDAR populated with three VTODOs of varying status. Models
spec/02's "world-shape" mapping where a VCALENDAR is a multi-
mission ledger.

## Shape

- VCALENDAR with PRODID + VERSION
- Three VTODO components:
  - `mission-alpha` — STATUS:NEEDS-ACTION, PRIORITY:1
  - `mission-bravo` — STATUS:IN-PROCESS, PRIORITY:2
  - `mission-charlie` — STATUS:COMPLETED + COMPLETED, PRIORITY:3

## Why

Exercises the top-level component sort (canonical orders by UID,
so output sequence is alpha → bravo → charlie regardless of input
order), the three relevant VTODO STATUS values, and the
VTODO-with-COMPLETED requirement (spec/05 VS040).

## RFC anchors

- RFC 5545 §3.6.2 — VTODO component definition.
- RFC 5545 §3.8.1.11 — STATUS property values.
- RFC 5545 §3.8.2.1 — COMPLETED property.
