# V\* conformance corpus

> [!NOTE]
> The corpus is **authored** at `spec/v1.0/conformance/`, alongside
> the specification text it pins. The Go module carries a generated,
> committed copy at `go/testdata/` so the published module stays
> self-contained — if you are reading this file there, edit the
> `spec/` copy instead. `make fixtures-verify` regenerates the mirror
> and reports any path it had to rewrite.

This directory is the cross-language conformance corpus for the V\*
specification. Every implementation — the Go reference, the
TypeScript, Python, Rust and PHP ports, and any downstream emitter
that cross-validates against them — consumes these fixtures
identically: same input, same canonical bytes, same hash. CI asserts
the property; drift fails the PR.

## Directory layout

| Subdir            | Format          | Purpose                                              |
|-------------------|-----------------|------------------------------------------------------|
| `rfc5545/`        | `.ics`          | VCALENDAR happy-path round-trip goldens.             |
| `rfc6350/`        | `.vcf`          | VCARD happy-path round-trip goldens.                 |
| `supersession/`   | `.ics`          | Supersession edge cases (linear, multi-step, cross-component, corrupt). |
| `malformed/`      | `.ics` / `.vcf` | Inputs that MUST fail to parse with a documented sentinel. |
| `rrule/`          | `.rrule` / `.ics` | RRULE parse, validation, wire-form, expansion and recurrence-set cases — see [RRULE fixture classes](#rrule-fixture-classes). |
| `time/`           | `.ics`          | VTIMEZONE fixtures for TZID resolution.              |
| `fuzz-seed/`      | `.bytes`        | Minimal-but-diverse byte sequences seeding the codec fuzz targets. |

The `rfc5545/` and `rfc6350/` directories were established by
earlier waves (codec/canonical/hashing tracks) and are kept as the
canonical layout for happy-path corpora. New categories live in
their own subdirs to keep the surface easy to navigate.

In the Go module's generated copy one extra directory appears,
`fuzz/`: that is the root package's own go-fuzz corpus, Go-native
rather than spec content, and it is preserved across the mirror
rather than being written by it.

## Fixture quartet convention

Every fixture is a quartet of sibling files sharing a stem `<name>`:

| File                | Required            | Content                                                      |
|---------------------|---------------------|--------------------------------------------------------------|
| `<name>.ics` / `.vcf` | yes               | The fixture input. LF on disk for diff-friendliness.         |
| `<name>.canonical`  | parseable inputs    | Canonical bytes from `canonical.Calendar` or `canonical.Card`. |
| `<name>.hash`       | parseable inputs    | `sha256:<64 hex>` + LF (exactly 72 bytes) from `hashing.Calendar` / `hashing.Card`. |
| `<name>.notes.md`   | optional            | Human prose describing the fixture's shape and purpose.      |
| `<name>.error`      | malformed inputs    | Name of the expected `vstar.Err*` sentinel (one per line).   |

Fixtures under `malformed/` carry **no** `.canonical` or `.hash`
because they fail to parse. They carry `.error` instead.

Files use **LF** line terminators on disk for diff-friendliness;
the parser is liberal on input (CRLF or LF) and the encoder always
emits CRLF on output. Round-trip semantic equality is asserted by
codec tests; byte-identity of `.canonical` and `.hash` is asserted
by `make fixtures-verify` (see CI parity below).

## RRULE fixture classes

Fixtures under `rrule/` follow their own convention. The input is
`<name>.rrule` (one RRULE property value) or, for recurrence sets,
`<name>.ics` (a VCALENDAR whose first component carries `DTSTART`
and any of `RRULE`, `RDATE`, `EXDATE`). Each JSON sidecar states the
result of one evaluator call; every timestamp is RFC 5545 form #2
(UTC, `Z`-suffixed).

| Sidecar | Input | Content |
|---------|-------|---------|
| `<name>.expect.json` | `.rrule` / `.ics` | `{"sentinel": <class>}` — the rule (or the recurrence set built from the `.ics`) MUST be rejected with the named failure class. |
| `<name>.formatted` | `.rrule` | Expected wire form: the parsed rule re-emitted in the fixed rule-part order with defaults elided. |
| `<name>.next.json` | `.rrule` | `dtstart`, `after`, `expected[]` — stepping from `after`, each expected occurrence in turn. Optional `error`: the step after the last expected occurrence MUST fail with the named class. |
| `<name>.expand.json` | `.rrule` | `dtstart`, `limit`, `expected[]`, `complete` — bounded expansion; `complete` is `true` only when the series ended within the limit. |
| `<name>.between.json` | `.rrule` | `dtstart`, `start`, `end`, `expected[]` — every occurrence in the half-open window `[start, end)`. |
| `<name>.occurrences.json` | `.ics` | `limit`, `expected[]`, `complete` — bounded expansion of the recurrence set (`DTSTART`, `RRULE`, `RDATE` merged, `EXDATE` removed). |

`.expand.json`, `.between.json` and `.occurrences.json` MAY carry
`error` in place of `expected`: the call MUST fail with the named
class. Classes are named by token — `ErrMalformed`,
`ErrUnsupportedRRule`, `ErrIterationCap`, `ErrUnboundedExpansion` —
per the spec's `05-conformance.md` §Failure classes.

Subdirectories: `happy/`, `rejected/`, `bounds/`, `by-clauses/`
(parser scope), `evaluator/` (`.next.json`), `format/`
(`.formatted`), `expansion/` (`.expand.json`, `.between.json`),
`set/` and `set/rejected/` (`.ics` recurrence sets).

## Implementation-class promise

For every fixture in `rfc5545/`, `rfc6350/`, and `supersession/`
that has a `.canonical` and `.hash` sibling:

- An **emitter** (build a Calendar/Card via the API and encode it)
  MUST produce bytes that, when canonicalized, match `.canonical`
  exactly and hash to `.hash` exactly.
- A **consumer** (parse the input, canonicalize, hash) MUST produce
  identical `.canonical` bytes and `.hash` content.
- A **round-trip** (parse, encode, parse again) MUST yield a
  semantically equal Calendar/Card.

This is enforced for the Go reference implementation by
`make fixtures-verify` and by `TestHashGoldens` in
`hashing/golden_test.go`, and across every port by `make test-parity`.
Each port, and any other implementation that claims V\* conformance,
MUST produce byte-identical `.canonical` and `.hash` content for
these fixtures.

## Change policy

Spec text, corpus and implementation live in one repository, so
changing a fixture is a **single PR** covering:

1. Spec text — the section of `spec/v1.0/` that motivates the new
   behavior.
2. The fixture itself, under this directory.
3. Implementation (the codec / canonical / hashing change).
4. The regenerated `.canonical` and `.hash` siblings, plus the
   `go/testdata/` mirror.

Run `make fixtures-verify` at the repository root (it invokes
`go -C go run ./cmd/fixtures-verify`). The target regenerates every
`.canonical` and `.hash` here, mirrors the result into
`go/testdata/`, and then diffs both trees. If either is dirty the
target fails — commit the regenerated files in the same PR as the
change that caused them.

Renaming a fixture is also a single coordinated PR; the stem change
ripples to every sibling and to the mirror.

Byte identity against this corpus is necessary, not sufficient: the
corpus pins only the rules its fixtures happen to reach. A new
fixture must reach a rule no existing fixture reaches, and the proof
is a mutation — disable the rule in the reference and confirm
`make fixtures-verify` rewrites exactly the new fixture's siblings.
The blind-spot fixtures in `rfc5545/` and `rfc6350/` (see their
READMEs) each closed a rule that four ports had skipped while
staying byte-identical on everything else. Every `rfc5545/` and
`rfc6350/` fixture also needs its row in each port's reference-bytes
table before the port suites go green; the porting guide names the
files.

## CI parity

`.github/workflows/ci-go.yml` runs `make fixtures-verify` after the
test step. A green run proves:

- The Go implementation reproduces every `.canonical` byte-for-byte.
- The Go implementation reproduces every `.hash` byte-for-byte.
- The `go/testdata/` mirror matches this corpus byte-for-byte.

Drift in any of those fails the PR. The expected reaction is
"investigate root cause" — either the implementation drifted
(revert the change), the spec/encoding intentionally changed
(regenerate, commit, document), or someone hand-edited the mirror
instead of this corpus (move the edit here and regenerate).

## Per-directory READMEs

- [`rfc5545/README.md`](rfc5545/README.md) — VCALENDAR happy paths.
- [`rfc6350/README.md`](rfc6350/README.md) — VCARD happy paths.
- [`supersession/README.md`](supersession/README.md) — append-only ledger edge cases.
- [`malformed/README.md`](malformed/README.md) — sentinel-bearing parse failures.
- [`time/README.md`](time/README.md) — VTIMEZONE fixtures.
- [`fuzz-seed/README.md`](fuzz-seed/README.md) — fuzz-target byte seeds.

## Fuzz seed convention

`fuzz-seed/rfc5545/seed_*.bytes` and `fuzz-seed/rfc6350/seed_*.bytes`
are the **canonical source** for the codec fuzz seed corpora. Each
file is the raw byte input; no go-fuzz wrapper. The matching
go-fuzz corpus directories live at:

- `codec/rfc5545/testdata/fuzz/FuzzParse_RFC5545/`
- `codec/rfc6350/testdata/fuzz/FuzzParse_RFC6350/`

These are kept in sync via `make fixtures-verify`, which also
copies any new `.bytes` file into the matching go-fuzz format
wrapper. Update the canonical source under `fuzz-seed/` — generated
by `go -C go run ./cmd/fixtures-gen-fuzz` — and rerun the target;
never hand-edit the go-fuzz corpus.
