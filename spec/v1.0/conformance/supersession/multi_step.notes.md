# multi_step.ics

A multi-step supersession: one original VTODO followed by two
supersession VJOURNALs walking the status from the implicit default
(NEEDS-ACTION) through IN-PROCESS to COMPLETED.

## Ledger

| Order | UID                                            | Effective status |
|-------|------------------------------------------------|------------------|
| 1     | `todo-multi`                                   | (original)       |
| 2     | `journal:status:todo-multi:20260504T143000Z`   | IN-PROCESS       |
| 3     | `journal:status:todo-multi:20260504T164500Z`   | COMPLETED        |

## Why

Verifies the "latest wins by DTSTAMP" projection rule from spec/02.
`Superseded(todo-multi, ledger)` MUST return `("COMPLETED", true)`.
Reordering the supersession entries (e.g. shuffling the journal
positions) MUST still yield COMPLETED because the rule is
timestamp-driven, not position-driven.
