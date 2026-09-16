# vtodo_sequence_percent.ics

A VCALENDAR containing a single VTODO that carries the three
integer-valued properties spec/05 criterion 7 bounds.

## Shape

- VCALENDAR with PRODID + VERSION
- One VTODO with UID, DTSTAMP, SUMMARY, `SEQUENCE:2`, `PRIORITY:0`,
  `PERCENT-COMPLETE:40`, DUE

## Why

Round-trip + canonical + hash coverage for SEQUENCE (non-negative),
PRIORITY (0–9) and PERCENT-COMPLETE (0–100) written as canonical
decimals. `PRIORITY:0` is the explicit "undefined" value of
RFC 5545 §3.8.1.9 — present on the wire, distinct from an absent
PRIORITY — so a consumer that treats 0 as "not set" and drops the
property changes the canonical bytes and fails the hash. The DUE
satisfies the VTODO requirement of spec/05 (VS040).

## RFC anchors

- RFC 5545 §3.6.2 — VTODO component definition.
- RFC 5545 §3.8.7.4 — SEQUENCE property.
- RFC 5545 §3.8.1.9 — PRIORITY property.
- RFC 5545 §3.8.1.8 — PERCENT-COMPLETE property.
- RFC 5545 §3.8.2.3 — DUE property.
