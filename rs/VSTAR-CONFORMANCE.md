# V* conformance — `hop-top-vstar` (Rust)

This is the self-certification
[spec/v0.1/05-conformance.md](../spec/v0.1/05-conformance.md#self-certification)
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
byte-identically. Both directions are exercised: `tests/roundtrip.rs`
round-trips every conformance fixture, and `tests/parity.rs` compares
this port's encoder output against `.encoded` files captured from the
Go reference as `Vec<u8>` — never as `String`, never trimmed — so
"byte-identical" means identical to the reference, not merely stable
under this port's own round trip.

## Conformance criteria

Criteria are numbered per
[spec/v0.1/05-conformance.md](../spec/v0.1/05-conformance.md).

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

Every failure is one variant of the crate-wide `Error` enum, and the
cross-language identifier is recoverable with `sentinel()`:

```rust
use hop_top_vstar::codec::rfc5545;

let err = rfc5545::parse(b"NOT A CALENDAR".as_slice()).unwrap_err();
assert_eq!(err.sentinel(), "ErrMalformed");
```

The twelve identifiers are spelled exactly as the Go reference spells
them — `ErrMissingUID` keeps Go's acronym run rather than being
re-cased to `ErrMissingUid` — and each is produced by exactly one arm
of `sentinel()` in `src/error.rs`, so a port-local respelling is a
one-line diff rather than a silent divergence.

Variants carry their positional context as a `String` payload,
mirroring Go's `fmt.Errorf("line %d: %w", n, ErrMalformed)`: the
context travels with the sentinel instead of replacing it, and
`context()` recovers it without parsing the `Display` output.

`Error` is `#[non_exhaustive]`, so callers must carry a wildcard arm
and a future sentinel is not a breaking change.

## Corpus gates

Every gate below runs in CI (`ci-rs.yml` for the port's own suite,
`ci-parity.yml` for the cross-language document).

| Gate | Status |
|---|---|
| conformance round-trip (rfc5545, rfc6350) | green — `tests/roundtrip.rs` |
| reference encoder bytes | green — `tests/parity.rs` |
| malformed sentinels | green — `tests/malformed.rs` |
| canonical bytes | green — `tests/canonical.rs`, `tests/canonical_rules.rs` |
| hash values | green — `tests/canonical.rs`, plus the emitter's own self-check against every committed `.hash` sibling |
| content lines and streaming | green — `tests/contentline.rs`, `tests/stream.rs` |
| fuzz seeds (no panic) | green — `tests/fuzz_seed.rs` |
| behavior/time, behavior/duration | green — `tests/time.rs`, `tests/duration.rs` |
| rrule sidecars | green — `tests/rrule.rs` |
| behavior/validate | green — `tests/validate.rs` |
| behavior/{ext,supersession,diff} | green — `tests/ext.rs`, `tests/supersession.rs`, `tests/diff.rs` |
| parity emitter | green — `make test-parity` reports `ok go php py rs ts (184 cases)` |

The parity gate is the strongest of these. `src/bin/parity/` and the Go
reference emitter independently run their own implementations over the
shared corpus and print one JSON document each; the harness fails on
any difference. The two documents carry identical values across all
eight families, so this port's agreement with the reference is measured
rather than asserted.

## Known deviations

Four, all deliberate, none affecting emitted bytes.

1. **One error enum, no type per sentinel.** Every failure is a variant
   of `Error` carrying a context `String`, where an idiomatic Rust
   library might expose a distinct type per class or a `thiserror`
   hierarchy. One enum means a caller composing parse → canonical →
   validate matches one type rather than converting between several,
   and the conformance corpus asserts one identifier space. The
   sentinel identifier is preserved exactly, so nothing about the
   cross-language contract is lost.

2. **`ComponentDiff::is_empty` renames Go's `Empty()`.** The Go name
   reads as an adjective despite being a predicate; every target
   language spells predicates with an `is` prefix, and Rust's own
   convention is `is_empty`. The behavior is identical — no change at
   this level nor in any nested sub-component.

3. **`Duration::signed()` returns a `chrono::Duration`, where the Go
   reference returns a nanosecond count.** The V\* `Duration` preserves
   the units a value was authored in, which a scalar cannot; the
   chrono type is the idiomatic carrier for the resolved span. Both
   divide down to the same second count, which is the unit the
   cross-language corpus compares, so this is a surface difference
   rather than a behavioral one.

4. **`supersession::superseded` returns `Option<String>` where Go
   returns a `(string, bool)` pair.** Go has no option type and spells
   absence as a second return value; in Rust the same "has this been
   superseded?" question has exactly two honest answers and `None` is
   the idiom for the negative one. No error case is folded in — a
   broken hash with no ledger entry targeting it is still `None`, not
   a failure, because supersession is a projection query rather than a
   validator.

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
- **Canonical bytes** — `tests/canonical.rs` compares this port's
  canonical output against each fixture's committed `.canonical`
  sibling as bytes.
- **Encoder bytes** — `tests/parity.rs` compares this port's encoder
  output against the Go reference's, captured under `tests/parity/` as
  `.encoded` files.
- **Cross-language document** — `make test-parity` emits one JSON
  document per implementation and requires them to be identical. At the
  spec revision above it reports `parity: ok go php py rs ts (184 cases)`.

To reproduce all four from a checkout:

```sh
make test-rs
make test-parity
```
