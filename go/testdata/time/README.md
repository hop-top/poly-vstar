# vstar/go time test fixtures

These fixtures back the TZID-aware parser (`ParseTimeWithTZID`).
They are wire-decoded: the Go suite parses each `.ics` through the
rfc5545 codec and feeds the resulting `Calendar` to the tests (see
`time_fixture_test.go::americaMontrealCalendar` and its siblings).
Editing a fixture changes what those tests assert, so the file and
the suite cannot drift apart.

The Go tests read the mirrored copy under `go/testdata/time/`, not
this directory: the published `hop.top/vstar` module is the `go/`
subtree alone and ships no sibling `spec/`. Author fixtures here;
`make fixtures-verify` regenerates the mirror and fails on drift.

## Fixtures

- `america_montreal.ics` — STANDARD + DAYLIGHT with `FREQ=YEARLY`
  RRULEs (BYMONTH=3;BYDAY=2SU and BYMONTH=11;BYDAY=1SU). Exercises
  the standard US/Canada DST transitions.
- `utc_only.ics` — STANDARD-only VTIMEZONE for the trivial fixed-
  offset path.

## VTIMEZONE feature subset (v1.0)

The TZID parser implements a deliberately small slice of
RFC 5545 §3.6.5 — enough to handle the world's most common DST
zones expressed via `FREQ=YEARLY` RRULEs, plus single-offset zones.

Supported:

- One STANDARD child, no DAYLIGHT — fixed offset.
- One DAYLIGHT child, no STANDARD — fixed offset (rare, tolerated).
- One STANDARD + one DAYLIGHT — both must carry an
  `RRULE:FREQ=YEARLY` with `BYMONTH=<m>` and `BYDAY=<n><WD>` parts.
  `INTERVAL=1` accepted; any other INTERVAL rejected.

Rejected (returns `(zero, false)`):

- Multiple STANDARD or DAYLIGHT entries (historical zone with
  offset changes — e.g. America/Caracas pre-2007). Workaround:
  caller maintains its own zone registry.
- `RDATE`-only zones (no RRULE).
- `FREQ` other than `YEARLY`.
- `UNTIL`, `COUNT`, `BYWEEKNO`, `BYSETPOS`, `BYMONTHDAY`, `WKST`,
  or any RRULE part not enumerated above.
- `BYDAY` without an ordinal (`SU` instead of `2SU`) — RFC permits
  it but VTIMEZONE rules require an explicit nth-of-month.

When a fixture exercises a rejected case, the test should assert
`ok == false` rather than try to coerce the zone.

This subset is documented inline in `time_tzid.go` and normatively
in spec/03 §VTIMEZONE subset for TZID resolution.
