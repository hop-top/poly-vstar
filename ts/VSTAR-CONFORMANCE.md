# V* conformance — `@hop-top/vstar` (TypeScript)

This is the self-certification
[spec/v0.1/05-conformance.md](https://github.com/hop-top/poly-vstar/blob/main/spec/v0.1/05-conformance.md#self-certification)
asks every implementation to publish. Until a formal conformance suite
exists (planned for v0.2) it is the honor-system substitute, and for a
port most of it is a statement of which gates are green.

## Spec revision

Targets spec revision `93afaaf2546a154c3672611f00794c5969cca9a9`.

The spec is authored in the same repository as this port, so the
revision is a commit of that repository rather than an external pin. A
change to `spec/` that this port has not caught up with fails
`make test-parity` before it can go unnoticed.

## Implementation class

**Round-trip.**

Documents this implementation emits re-parse and re-emit
byte-identically. Both directions are exercised: `test/rfc5545.test.ts`
and `test/rfc6350.test.ts` round-trip every conformance fixture, and
`test/reference-parity.test.ts` pins the encoder's byte length and an
FNV-1a fingerprint of its output against values captured from the Go
reference — so "byte-identical" means identical to the reference, not
merely stable under this port's own round trip.

## Conformance criteria

Criteria are numbered per
[spec/v0.1/05-conformance.md](https://github.com/hop-top/poly-vstar/blob/main/spec/v0.1/05-conformance.md).

| # | Criterion | Status |
|---|---|---|
| 1 | Emits valid RFC 5545 / RFC 6350 | met |
| 2 | Emits required common properties | met |
| 3 | Honors canonicalization rules | met |
| 4 | Uses correct component types | met |
| 5 | Respects the extension namespace | met |
| 6 | Is append-only (supersession) | met |
| 7 | Keeps duration values well-formed | met |
| 8 | Keeps enumerated and integer values in their RFC domains | met |

Criterion 1 is "met" in the sense the spec defines it — output passes a
generic iCalendar/vCard validator — and is evidenced by the
byte-identical agreement with the Go reference recorded below, not by a
run against a third-party validator. No independent validator has been
run against this port's output.

## Failure classes

All twelve sentinels are surfaced and distinguishable: **yes**.

The identifier is recoverable from a caught failure as the `code`
property of the thrown `VstarError`:

```ts
try {
  parse(input);
} catch (e) {
  if (e instanceof VstarError && e.code === "ErrUnclosedBlock") { /* … */ }
}
```

The twelve identifiers are spelled exactly as the Go reference spells
them and are declared as one closed union (`VSTAR_ERROR_CODES` in
`src/errors.ts`), so a port-local respelling is a type error rather
than a silent divergence.

There is deliberately **no subclass per sentinel**. `instanceof` is
unreliable across module boundaries when a bundler duplicates the
module; dispatching on the string is not. This is a deviation from a
reader's likely expectation of an idiomatic TypeScript error hierarchy,
and it is recorded under [Known deviations](#known-deviations).

## Corpus gates

Every gate below runs in CI (`ci-ts.yml` for the port's own suite,
`ci-parity.yml` for the cross-language document).

| Gate | Status |
|---|---|
| conformance round-trip (rfc5545, rfc6350) | green — `test/rfc5545.test.ts`, `test/rfc6350.test.ts` |
| malformed sentinels | green — `test/malformed.test.ts` |
| canonical bytes | green — `test/canonical.test.ts`, `test/canonical-rules.test.ts` |
| hash values | green — `test/hashing.test.ts`, plus the emitter's own self-check against every committed `.hash` sibling |
| behavior/time, behavior/duration | green — `test/time.test.ts`, `test/duration.test.ts` |
| rrule sidecars | green — `test/rrule.test.ts` |
| behavior/validate | green — `test/validate.test.ts` |
| behavior/{ext,supersession,diff} | green — `test/ext.test.ts`, `test/supersession.test.ts`, `test/diff.test.ts` |
| parity emitter | green — `make test-parity` reports `parity: ok go php py rs ts` plus the case count it measured; see [tools/parity/README.md](https://github.com/hop-top/poly-vstar/blob/main/tools/parity/README.md) |

The parity gate is the strongest of these. `tools/parity.ts` and the Go
reference emitter independently run their own implementations over the
shared corpus and print one JSON document each; the harness fails on
any difference. The two documents are byte-identical across all eight
families, so this port's agreement with the reference is measured
rather than asserted.

## Known deviations

Three, all deliberate, none affecting emitted bytes.

1. **No error subclass per sentinel.** Every failure is a single
   `VstarError` carrying a `code` string, where an idiomatic
   TypeScript library might expose `MalformedError`,
   `UnclosedBlockError` and so on. `instanceof` breaks when a bundler
   duplicates a module — a realistic hazard for a library consumed
   through a dozen build pipelines — and a string comparison does not.
   The sentinel identifier is preserved exactly, so nothing about the
   cross-language contract is lost.

2. **`Instant` is a millisecond number, not a `Date`.** The model
   represents an absolute instant as a primitive number of
   milliseconds since the epoch. `Date` carries a mutable local-time
   presentation this port has no use for and would invite
   `getHours()`-style local-time bugs in code that must stay in UTC.
   The consequence for a caller is that instants are compared with
   `===` and formatted with `formatTime`, not with `Date` methods.
   Sub-millisecond precision is not representable — RFC 5545
   timestamps are second-resolution, so nothing is lost on the wire.

3. **Durations expose milliseconds where the Go reference exposes
   nanoseconds.** `VDuration.signed()` returns milliseconds, because
   JavaScript's number type cannot hold a nanosecond count across the
   useful range without precision loss. Both divide down to the same
   second count, which is the unit the cross-language corpus compares,
   so this is a surface difference rather than a behavioral one.

No deviation is known in canonical form, hashing, validation codes,
diff ordering, supersession projection, recurrence evaluation, or
extension scoping — the parity gate would fail on any of them.

## Test artifacts

The corpus run is the artifact. Rather than pin a handful of golden
documents here, where they would drift from the corpus they were copied
out of, this port is certified by the gates above over the whole of
`spec/v0.1/conformance/` and `spec/behavior/`:

- **Hashes** — the parity emitter recomputes the content hash of every
  parseable fixture and compares it against that fixture's committed
  `.hash` sibling, aborting on any mismatch. The corpus is therefore an
  independent expectation this port is checked against, not a record of
  what it happened to produce.
- **Canonical bytes** — `test/canonical.test.ts` compares this port's
  canonical output against each fixture's committed `.canonical`
  sibling as bytes.
- **Cross-language document** — `make test-parity` emits one JSON
  document per implementation and requires them to be identical. At the
  spec revision above it reports `parity: ok go php py rs ts` followed
  by the case count it measured. The count is printed by the run rather
  than recorded here, so it cannot drift from the corpus.

To reproduce all three from a checkout:

```sh
make test-ts
make test-parity
```
