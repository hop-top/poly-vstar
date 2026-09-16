# corrupt_mutated.ics

A VTODO with `X-VSTAR-HASH` set, then `SUMMARY` mutated inline
without refreshing the hash. The stored `X-VSTAR-HASH` no longer
matches the canonical form.

## Ledger

| UID            | Notes                                                          |
|----------------|----------------------------------------------------------------|
| `todo-corrupt` | SUMMARY mutated post-hash. `X-VSTAR-HASH` is stale.            |

## Why this fixture is INTENTIONALLY corrupt

This is negative-test fodder.

- `validate.Validate` MUST emit **VS010** ("X-VSTAR-HASH is present
  but does not match the recomputed hash") when this fixture is
  fed in.
- `supersession.Supersedes(target, ...)` MUST refuse with
  `ErrTargetCorrupted` when this VTODO is the target.

The `.hash` sibling is the **current** hash from
`hashing.Calendar` over the post-mutation canonical form. Consumers
comparing the in-document `X-VSTAR-HASH` against the canonical hash
WILL notice the mismatch — that mismatch is the test signal.

The `.canonical` sibling reflects the post-mutation document; the
in-document `X-VSTAR-HASH` is stripped during canonicalization
(spec/03 rule 7) so the canonical bytes are well-defined and
match a clean re-canonicalization.
