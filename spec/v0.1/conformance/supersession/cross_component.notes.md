# cross_component.ics

A VTODO and the VJOURNAL that supersedes it, as siblings in one
VCALENDAR. Per spec/02 the V* "ledger" is one logical container;
component types may differ but the RELATED-TO chain links them.

## Ledger

| UID                                          | Type     | Role                                                                                          |
|----------------------------------------------|----------|-----------------------------------------------------------------------------------------------|
| `turn-42`                                    | VTODO    | Mission turn.                                                                                 |
| `journal:status:turn-42:20260504T143000Z`    | VJOURNAL | Supersedes `turn-42` to COMPLETED. Carries an extra DESCRIPTION explaining the state change. |

## Why

Spec/02 says the ledger is a single logical container. Even though
VJOURNAL and VTODO are different RFC 5545 component types, they
live side-by-side and the consumer projects across the boundary.
This fixture exercises that pattern.

The DESCRIPTION on the supersession VJOURNAL is real-world flavor;
the `X-VSTAR-HASH` is recomputed after the DESCRIPTION is added so
the journal stays integrity-verifiable.
