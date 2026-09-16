# V* conformance — `hop-top/vstar` (PHP)

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
byte-identically. Both directions are exercised:
`tests/Codec/Rfc5545RoundTripTest.php` and
`tests/Codec/Rfc6350RoundTripTest.php` round-trip every conformance
fixture, and `tests/Codec/ReferenceParityTest.php` asserts this port's
encoder output equals the Go reference's bytes for every fixture,
compared against base64 captured from the reference in
`tests/Fixtures/reference-encoding.json` — so "byte-identical" means
identical to the reference, not merely stable under this port's own
round trip.

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

Each is its own `final` class under `HopTop\Vstar\Exception`, all
extending the abstract `VstarException`. The identifier is recoverable
from a caught failure as `sentinel()`:

```php
try {
    Parser::parse($input);
} catch (UnclosedBlockException $e) {
    // Catch the class when you know which failure you are handling.
    echo 'unclosed: ', $e->sentinel(), "\n";
} catch (VstarException $e) {
    // Or catch the base and dispatch on the identifier.
    if ($e->sentinel() !== 'ErrMalformed') {
        throw $e;
    }
    echo "malformed\n";
}
```

The twelve identifiers are spelled exactly as the Go reference spells
them, each returned by one `sentinel()` implementation, so a port-local
respelling is a one-line diff in one file rather than a divergence
scattered across call sites.

A subclass per sentinel is the idiomatic PHP shape and this port takes
it, unlike the TypeScript port, where bundler module duplication makes
`instanceof` unreliable. PHP has no equivalent hazard: the autoloader
resolves a class name once per process. Both spellings preserve the
sentinel identifier, so the cross-language contract is the same either
way.

## Corpus gates

Every gate below runs in CI (`ci-php.yml` for the port's own suite on
PHP 8.2 and 8.4, `ci-parity.yml` for the cross-language document).

| Gate | Status |
|---|---|
| conformance round-trip (rfc5545, rfc6350) | green — `tests/Codec/Rfc5545RoundTripTest.php`, `tests/Codec/Rfc6350RoundTripTest.php` |
| reference byte parity | green — `tests/Codec/ReferenceParityTest.php` |
| malformed sentinels | green — `tests/Codec/MalformedSentinelTest.php` |
| fuzz seeds | green — `tests/Codec/FuzzSeedTest.php` |
| stream codec | green — `tests/Codec/Stream/StreamTest.php` |
| canonical bytes | green — `tests/Canonical/CorpusTest.php`, `tests/Canonical/RulesTest.php` |
| hash values | green — `tests/Hashing/HashingTest.php`, plus the emitter's own self-check against every committed `.hash` sibling |
| behavior/time, behavior/duration | green — `tests/TimeTest.php`, `tests/Duration/ParseTest.php`, `tests/Duration/TriggerTest.php` |
| rrule sidecars | green — `tests/Rrule/ParseTest.php`, `tests/Rrule/EvaluatorTest.php`, `tests/Rrule/SetTest.php` |
| behavior/validate | green — `tests/Validate/ValidateTest.php` |
| behavior/{ext,supersession,diff} | green — `tests/Ext/ScopesTest.php`, `tests/Supersession/SupersessionTest.php`, `tests/Diff/DiffTest.php` |
| static analysis | green — PHPStan level 9 over `src/` and `tests/` |
| parity emitter | green — `make test-parity` reports `parity: ok go php py rs ts` plus the case count it measured; see [tools/parity/README.md](https://github.com/hop-top/poly-vstar/blob/main/tools/parity/README.md) |

The parity gate is the strongest of these. `tools/parity.php` and the Go
reference emitter independently run their own implementations over the
shared corpus and print one JSON document each; the harness fails on any
difference. The two documents are identical across all eight families,
so this port's agreement with the reference is measured rather than
asserted.

## Known deviations

Three, all deliberate, none affecting emitted bytes.

1. **Enums spell the display method `toString()`, not `__toString()`.**
   PHP rejects `__toString` on an enum at declaration time — the engine
   refuses the declaration outright, so this is not a style choice.
   `Severity`, `Scope`, `DiffOp`, `Related`, `Freq`, `Weekday` and
   `RecurrenceRange` therefore expose the reference's `String()` as a
   plain `toString()` method, and the seven enums the reference gives
   no `String()` — `CompType`, `Kind`, `VClass`, `Transp`,
   `EventStatus`, `TodoStatus`, `JournalStatus` — declare the same
   `toString()` returning `->value`, so every enum in the port carries
   one accessor. Non-enum value classes — `VDate`, `VDuration`,
   `RelType`, `Rrule\Rule`, `Diff\ComponentDiff` — keep `__toString()`.
   The contract in every case is the backing `->value`, which is the
   wire token the corpus compares; `toString()` is display text.

2. **An absolute instant is a `DateTimeImmutable`, not an integer.**
   The Go reference carries a `time.Time`; this port carries PHP's
   immutable date type rather than a Unix second count, so the standard
   library's comparison and arithmetic operators work directly and no
   caller has to remember which unit an integer is in. The mutable
   `DateTime` is deliberately never used: it would let a callee modify a
   caller's value. Sub-second precision is not represented — RFC 5545
   timestamps are second-resolution, so nothing is lost on the wire.

3. **Durations expose `totalSeconds(): int` alongside `signed():
   DateInterval`, where the Go reference exposes a nanosecond
   `Signed()`.** PHP has no native nanosecond duration type;
   `DateInterval` is the idiomatic one and carries no sub-second field.
   `totalSeconds()` is the second count the cross-language corpus
   compares, and `signed()` builds its interval from that same count
   rather than from the authored fields, so a `P1W` never reaches
   `DateTimeImmutable::add()` as a calendar quantity that a DST
   transition would reinterpret. This is a surface difference rather
   than a behavioral one.

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
- **Canonical bytes** — `tests/Canonical/CorpusTest.php` compares this
  port's canonical output against each fixture's committed `.canonical`
  sibling as bytes.
- **Reference bytes** — `tests/Codec/ReferenceParityTest.php` compares
  the encoder's output against bytes captured from the Go
  implementation, so the encoder is pinned to the reference and not
  only to itself.
- **Cross-language document** — `make test-parity` emits one JSON
  document per implementation and requires them to be identical. At the
  spec revision above it reports `parity: ok go php py rs ts` followed
  by the case count it measured. The count is printed by the run rather
  than recorded here, so it cannot drift from the corpus.

To reproduce all four from a checkout:

```sh
make test-php
make test-parity
```
