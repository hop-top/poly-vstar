# vjournal.ics

A VCALENDAR containing a single VJOURNAL component.

## Shape

- VCALENDAR with PRODID + VERSION
- One VJOURNAL with UID, DTSTAMP, DTSTART, SUMMARY, DESCRIPTION

## Why

Round-trip + canonical + hash coverage for the VJOURNAL component
type. VJOURNAL is the carrier for V*'s status-supersession entries
(see `testdata/supersession/`); a clean happy-path baseline keeps
the supersession tests focused on the semantics rather than the
shape of a journal.

## RFC anchors

- RFC 5545 §3.6.3 — VJOURNAL component definition.
