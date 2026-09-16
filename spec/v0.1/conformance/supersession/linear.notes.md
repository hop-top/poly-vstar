# linear.ics

A clean linear supersession: one VTODO followed by one supersession
VJOURNAL flipping it to COMPLETED.

## Ledger

| UID                                    | Type     | Role                                       |
|----------------------------------------|----------|--------------------------------------------|
| `todo-1`                               | VTODO    | Original mission. SUMMARY "Buy milk".      |
| `journal:status:todo-1:20260504T143000Z` | VJOURNAL | Supersedes `todo-1` to COMPLETED.        |

## Why

The minimal happy path for spec/02. `Superseded(todo-1, ledger)`
MUST return `("COMPLETED", true)` from `supersession/`.

The original VTODO carries an `X-VSTAR-HASH` so the integrity
check inside `Supersedes` had material to verify against.
