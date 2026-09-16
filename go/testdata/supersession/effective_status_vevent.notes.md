# effective_status_vevent.ics

A turn (VEVENT) followed by one supersession VJOURNAL flipping it
to CANCELLED.

## Ledger

| UID                                     | Type     | Role                                              |
|-----------------------------------------|----------|---------------------------------------------------|
| `turn-7`                                | VEVENT   | Original turn. SUMMARY "Pair on the parser".      |
| `journal:status:turn-7:20260504T143000Z` | VJOURNAL | Supersedes `turn-7` to CANCELLED.               |

## Why

spec/02 "Status supersession" scopes `X-VSTAR-EFFECTIVE-STATUS` to
the STATUS vocabulary of the *superseded* component's type: a turn
is a VEVENT, so the effective status is a VEVENT value (TENTATIVE /
CONFIRMED / CANCELLED) even though the carrier is a VJOURNAL.
`Superseded(turn-7, ledger)` MUST return `("CANCELLED", true)`.

`CANCELLED` is the one token all three RFC 5545 §3.8.1.11
vocabularies share, so this fixture is clean under every reading;
the notes, not the bytes, pin which vocabulary the spec means. A
consumer projecting the ledger MUST NOT reject the journal for
carrying a value outside the VJOURNAL list — the VJOURNAL list is
not the one that applies.

The original VEVENT carries an `X-VSTAR-HASH` so the integrity
check inside `Supersedes` had material to verify against.
