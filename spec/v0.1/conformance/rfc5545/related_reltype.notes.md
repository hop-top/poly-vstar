# related_reltype.ics

A VCALENDAR with one mission VTODO and two assignment VTODOs joined
by RELATED-TO edges of three relationship types.

## Shape

- VCALENDAR with PRODID + VERSION
- `mission-relay` — VTODO, no RELATED-TO.
- `assignment-survey` — VTODO with two edges:
  - `RELATED-TO:mission-relay` — RELTYPE omitted, so PARENT.
  - `RELATED-TO;RELTYPE=FINISHTOSTART:assignment-build` — the build
    cannot start until the survey finishes; the temporal edge lives
    on the predecessor and points at the successor.
- `assignment-build` — VTODO with two edges:
  - `RELATED-TO:mission-relay` — RELTYPE omitted, so PARENT.
  - `RELATED-TO;RELTYPE=DEPENDS-ON:assignment-survey` — the build
    cannot proceed until the survey is satisfied.
- Every component carries UID, DTSTAMP, SUMMARY and DUE.

## Why

Pins the spec/02 "Relationship types (RELTYPE)" rules in bytes: the
PARENT edge is spelled without RELTYPE (rule 2), the explicit types
are uppercase (rule 3), and every edge appears exactly once, on the
referencing component (rule 1). Canonical form keeps the RELTYPE
parameter verbatim, so `.canonical` and `.hash` change if an
implementation ever rewrites `RELATED-TO:x` as
`RELATED-TO;RELTYPE=PARENT:x` or folds parameter case.

## RFC anchors

- RFC 5545 §3.8.4.5 — RELATED-TO property.
- RFC 5545 §3.2.15 — RELTYPE parameter; PARENT is the default.
- RFC 9253 §4 — temporal relationship types (FINISHTOSTART).
- RFC 9253 §5 — DEPENDS-ON.
- RFC 9253 §11.4 — RELTYPE value registry.
