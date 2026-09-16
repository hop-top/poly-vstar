<!-- SPDX-License-Identifier: MIT -->

# Porting guide — building a V\* implementation in a new language

This is the recipe for the four language ports (TypeScript, Python,
Rust, PHP). It gives the order to build in, the gate each layer must
pass before the next one starts, and the rules that are not negotiable
because getting them wrong produces output that looks right and hashes
differently.

The companion document is the [API mapping](api-mapping.md), which
names every symbol in every language. This guide says what to build and
how to know it works; that one says what to call it.

## Lessons for the next lanes

TypeScript was the pilot; Python, Rust and PHP followed. Each item
below cost one of those lanes real time, and each is recoverable from
the repository's own history rather than from memory.

1. **Read the [API mapping](api-mapping.md), do not copy the port you
   were pointed at.** The TypeScript port shipped `property()` where
   the mapping spells `toProperty()`; Python and Rust both followed the
   document and were right. The divergence surfaced only because the
   Python rrule lane read the mapping instead of mirroring the
   TypeScript source it was handed as a template. The mapping is
   normative and wins — when it and a shipped port disagree, the port
   is the bug.

2. **A green corpus gate is not coverage.** `behavior/validate` cannot
   reach case-insensitive property-*name* matching, a present-but-blank
   supersession property, a whitespace-padded integer value, or the
   component scope of the §8 value-domain properties — the fixtures
   are all-uppercase, never spell an empty value, leave padding to
   each port's parser tests, and no implementation diagnoses scope.
   Three lanes had to
   add unit tests for gaps like these *after* the gate was green, and
   two more — the positional `[#n]` counter, which every fixture left
   frozen at `[#0]`, and the `TRIGGER` / `REPEAT` arms of the duration
   rule, which no fixture reached — were found by mutation testing
   before shared fixtures closed them. Write the tests while you are
   in the layer, not after. See [layer (d)](#layer-d--validation).

3. **Never publish a value your own example regenerates.** A TypeScript
   README example printed a literal `X-VSTAR-HASH` digest produced by a
   helper that stamps `DTSTAMP` from the wall clock — two runs a second
   apart disagreed, so the published digest matched no invocation.
   Publish the *invariant* (a helper restamps the hash; a raw property
   edit leaves it stale), never the digest. Run every snippet you
   publish and paste real output.

4. **Restating a generated table by hand is a staleness bug waiting to
   land.** One new corpus fixture took the parity count from 181 to 182
   and required nine hand-edits across five files. Diagnostic-code
   ranges, symbol counts and case counts all drift the same way. Cite
   the generated artifact — [`docs/validate-codes.md`](../validate-codes.md),
   [`spec/registry/`](../../spec/registry/) — or print the command that
   produces the number, rather than transcribing it.

5. **The port's toolchain will fail in CI before your code does.**
   pnpm 11 refuses to install while a dependency's build script is
   undecided, and its deps-status precheck runs a plain `pnpm install`
   that `--ignore-scripts` never reaches — so the parity harness failed
   on a cold runner while every other TypeScript job passed. Budget for
   one packaging defect per lane, and reproduce on a cold cache before
   calling a lane done.

## Before you start

Read, in this order:

1. [spec/v0.1/03-canonicalization.md](../../spec/v0.1/03-canonicalization.md)
   — rules 1–12, datetime resolution, the VTIMEZONE subset, RRULE
   parsing scope. This is the document the ports exist to agree with.
2. [spec/v0.1/05-conformance.md](../../spec/v0.1/05-conformance.md) —
   what "conformant" means and the failure classes.
3. [api-mapping.md](api-mapping.md) — the naming contract.

Then look at the Go reference for the layer you are building. It is not
a design to copy structurally — idiomatic Rust and idiomatic Go differ —
but it is the behavior of record.

## Build order

A port lands in six layers, in this order. Each layer's gate must be
green before the next layer starts. The order is not arbitrary: every
layer depends on the correctness of the one before it, and a hashing
bug found at layer (e) is a canonical-form bug you will debug through
five layers of indirection.

| Layer | Scope | Gate |
|---|---|---|
| **a** | model + `codec/rfc5545` + `codec/rfc6350` | corpus round-trip; every `malformed/*` fixture produces its named sentinel |
| **b** | `canonical` (spec rules 1–12) + `hashing` + TZID resolution + `duration` | every `.canonical` and `.hash` byte-identical; `behavior/time` and `behavior/duration` pass |
| **c** | `rrule` — parse, validate, format, next, All, Occurrences, Between, Set, RecurrenceID | every `rrule/` sidecar passes |
| **d** | `validate` with generated registry constants | `behavior/validate` passes; the conformance corpus is clean of every §8 value-domain diagnostic |
| **e** | `ext` + `helpers` + `supersession` + `diff` + `codec/stream` | `behavior/ext`, `behavior/supersession`, `behavior/diff` pass |
| **f** | parity emitter + README + `VSTAR-CONFORMANCE.md` | `make test-parity` green with the new emitter |

### Layer (a) — model and codecs

Build `Property`, `Param`, `Component`, `Calendar`, `Card`, the
content-line scanner with RFC 5545 §3.1 unfolding, and the two batch
codecs.

The gate is a round-trip over
[`spec/v0.1/conformance/rfc5545/`](../../spec/v0.1/conformance/rfc5545/)
and
[`spec/v0.1/conformance/rfc6350/`](../../spec/v0.1/conformance/rfc6350/):
parse each `.ics` / `.vcf`, and confirm the parsed model matches what
the reference produces. Plus
[`spec/v0.1/conformance/malformed/`](../../spec/v0.1/conformance/malformed/):
each input MUST fail, and MUST fail with the sentinel its sibling
`.error` file names.

The `.error` file holds one line — the sentinel identifier, e.g.
`ErrMalformed`. A port that fails these inputs with the *wrong*
sentinel has not passed; failing is not enough, failing correctly is
the gate.

Parsers are **liberal**: see [line endings](#fixtures-are-lf-on-disk-encoders-emit-crlf).

#### Pin the reference encoder's bytes, not just the round-trip

A round-trip proves the port agrees with itself. It does not prove the
port agrees with the reference: a parser that sorts properties, an
encoder that folds one octet late, or a closed `CompType` enum that
drops `STANDARD` / `DAYLIGHT` all round-trip perfectly and emit
different bytes. Every one of those was a real port defect that the
round-trip gate passed and the byte pin caught.

So layer (a) carries a second gate: for every `rfc5545/` and `rfc6350/`
fixture, encode the parsed model and compare against the **Go
encoder's exact output**. Each port keeps a table of those bytes —
TypeScript and Python record `(byte length, FNV-1a)`
(`ts/test/reference-parity.test.ts`, `py/tests/test_reference_parity.py`),
PHP records base64 (`php/tests/Fixtures/reference-encoding.json`), Rust
the raw bytes (`rs/tests/parity/`). Three rules keep the table honest:

- **Values come from running `go/codec/rfc5545` and `go/codec/rfc6350`
  over the corpus, never from the port.** Re-recording from the port
  turns the gate into a tautology.
- **Enumerate the corpus, not the table.** A fixture with no entry must
  fail loudly, naming the fixture and how to record it. When the table
  drove the case list, a new upstream fixture broke a count assertion
  instead of gaining a test — and would have passed silently once the
  count was bumped.
- **Keep one reverse check** that every table entry still names a
  fixture, so a renamed or deleted fixture does not leave a dead row.

A fixture added upstream therefore needs its row in all four tables
before the port suites go green; recording it is part of adding the
fixture.

### Layer (b) — canonical form, hashing, TZID, duration

This is the layer the whole project exists for, and the one where a
port most often looks finished and is not.

Implement canonicalization rules 1–12 in the order given in
[the ordering section](#nfc-normalization-and-where-it-sits) below, the
`X-VSTAR-HASH` computation (`sha256:` + lowercase hex of the canonical
bytes, hash property excluded per rule 7), TZID resolution against the
document's own VTIMEZONE (see
[no IANA timezone database](#no-iana-timezone-database)), and the
`duration` package including the `DayForm` flag.

The gate is byte identity. For every fixture with a `.canonical`
sibling, your canonical bytes MUST equal the file's; for every `.hash`
sibling, your hash string MUST equal the file's. Plus
[`spec/behavior/time/`](../../spec/behavior/time/) and
[`spec/behavior/duration/`](../../spec/behavior/duration/).

`ComponentInContext` versus `Component` is a real distinction, not an
overload: the context-taking form resolves `TZID` datetimes against a
supplied calendar, the plain form emits them verbatim. Calendar-level
canonicalization routes every child through the context-taking form
with the calendar itself as the registry. Implement both.

### Layer (c) — recurrence

Parse, validate, format, `nextOccurrence`, `all`, `occurrences`,
`between`, the recurrence `RuleSet` (`DTSTART` / `RRULE` / `RDATE` /
`EXDATE`), and `RecurrenceId`.

The gate is
[`spec/v0.1/conformance/rrule/`](../../spec/v0.1/conformance/rrule/) —
every subdirectory: `happy/`, `rejected/`, `bounds/`, `by-clauses/`,
`evaluator/`, `expansion/`, `format/`, `set/`.

Three sub-gates inside it are easy to half-pass:

- `rejected/` — each case MUST produce the sentinel its
  `.expect.json` names. `ErrMalformed` and `ErrUnsupportedRRule` are
  different answers and the fixtures distinguish them.
- `evaluator/unsatisfiable_feb_30` — MUST produce `ErrIterationCap`,
  not an empty result. The parser stays permissive about
  unsatisfiable `BY-*` combinations by design, so the failure has to
  surface at evaluation.
- `format/` — re-emitting a parsed rule MUST be idempotent, MUST use
  the rule-part order the spec fixes, and MUST omit `INTERVAL=1` and
  `WKST=MO`. Rule *parts* are ordered; list *values* are not — `BYDAY`
  and `BYMONTHDAY` keep their authored order. `format/unsorted_lists`
  pins it; before that fixture existed every other `.formatted` file
  spelled its lists ascending, so a port that sorted on parse passed
  the whole directory, and three ports found the gap independently.

Weekday numbering is `SU = 0` per RFC 5545, which is not ISO-8601's
`MO = 1`. Convert at the boundary, once.

The iteration cap walks far. A yearly rule that never matches steps
one period per year for `MaxIterations` periods, which is past year
9999 in every port. Two consequences: a language whose calendar type
stops at 9999 (Python's `datetime`) treats running out of calendar
with empty periods pending as a cap hit, not as termination — that is
how `unsatisfiable_feb_30` surfaces there; and any wall-clock
composition that goes through a formatted string fails past year 9999
(PHP's relative-string constructor and `createFromFormat` both do), so
compose date fields numerically.

### Layer (d) — validation

Implement `validate` over generated registry constants. The registry is
[`spec/registry/`](../../spec/registry/) — diagnostic codes, the RFC
property allow-list, extension scopes, status vocabularies — and every
port renders it into its own generated module. See
[registry constants are generated](#registry-constants-are-generated).

The gate is [`spec/behavior/validate/`](../../spec/behavior/validate/):
each `.ics` has a `.diagnostics.json` sibling listing the exact
diagnostics the reference emits. Same codes, same severities, same
paths — compared after sorting by `path`, then `code`. Emission order
is **not** contractual: the reference emits in check order, which is an
implementation detail, so a port sorts its own output the same way
before comparing.

Note what this gate cannot reach. Every fixture spells its property
names in uppercase (values are covered — one clean fixture carries a
lowercase `CLASS` and `TRANSP`); no fixture carries a present-but-blank
`RELATED-TO` or `X-VSTAR-EFFECTIVE-STATUS` (the reference treats a
whitespace-only value as absent); no fixture carries a
whitespace-padded integer value, because whether a parser preserves
the padding is a codec question the validate family must not pin (the
reference's does; a port's may not, and the rule must reject ` 3`
either way); the component scope of `CLASS`, `TRANSP`, `PRIORITY`,
`PERCENT-COMPLETE`, `SEQUENCE` and `REPEAT` is not diagnosed by any
implementation (a `TRANSP` on a `VTODO` is value-checked, never
flagged for placement); and sub-components are not validated —
`Validate` does not descend into a component's children, so nothing
inside a nested `VALARM` is reachable. That last point is why the
`repeat_*` and `malformed_trigger` fixtures carry a **top-level**
`VALARM`: structurally odd iCalendar, on purpose, the same device
`vcard_missing_required` uses for `VCARD`. Your parser must accept it
generically. So case-insensitive name matching, blank-value handling,
padded integers and property scope all pass the gate whether or not a
port gets them right. Each port carries its own unit tests for these;
yours should too. Two items used to be on this list until a shared
fixture pinned them for every port: the positional `[#n]` counter
(every fixture had one UID-less component per type, so a counter
frozen at `[#0]` passed, until `missing_uid_positional` added two) and
the `TRIGGER` / `REPEAT` arms of the duration rule (reached only
through `DURATION`, until the top-level `VALARM` fixtures).

**Build the `STATUS` vocabulary from your own wire enums**, not from
the generated `STATUS_VOCABULARY` table. The enums are what the codec
encodes against, so a table derived from them cannot disagree with
what the port writes; a lookup into the generated table gives that
linkage up. Reconcile the two in a test instead (the Go form is
`TestStatusVocabulary_matchesRegistry`) — that is what keeps five ports
agreeing on the vocabulary without any of them losing the codec
guarantee. The generator renders the table for that test and for
consumers, not for the validator.

Enforce the no-literal-codes rule with a test that greps the source
tree for the code pattern outside the generated module; every port has
one alongside its validate specs. And note that `registry-check`
compares **bytes, not loadability**: a generated module rendered into
the wrong namespace or path passes it and fails first in whatever
imports it — the validate layer, being the first consumer. Import the
generated module from a test at layer (d) rather than trusting the
check.

Plus: the whole conformance corpus MUST be clean of every §8
value-domain diagnostic — the codes covering a `STATUS` value outside
the vocabulary RFC 5545 §3.8.1.11 scopes to the component's own type, a
`CLASS` or `TRANSP` outside its closed vocabulary, and a `PRIORITY`,
`PERCENT-COMPLETE` or `SEQUENCE` that is not a canonical decimal inside
its domain. If your port reports one on a corpus fixture, your rule is
wrong, not the fixture. The canonical-decimal check is **textual**
(`^(0|[1-9][0-9]*)$`, then a bound compared on the digit string or on a
value proven to fit); a port that parses to a machine integer before
deciding well-formedness fails the fixture carrying a `SEQUENCE` of
2^64, by design. The `REPEAT` count applies the same textual check but
stays under the malformed-duration code: a published code never changes
meaning.

### Layer (e) — the remaining packages

`ext`, `helpers`, `supersession`, `diff`, `codec/stream`. Gates:
[`behavior/ext`](../../spec/behavior/ext/),
[`behavior/supersession`](../../spec/behavior/supersession/),
[`behavior/diff`](../../spec/behavior/diff/).

`supersession` depends on layer (b) being exact: `supersedes` recomputes
the target's canonical hash and refuses a mismatch with
`ErrTargetCorrupted`. If your canonical form drifted by a byte, this
layer refuses everything, and the failure points at supersession rather
than at the canonicalizer that caused it. Which is why (b)'s gate comes
first.

`codec/stream` must reach `ErrAlreadyClosed` and `ErrHeaderLocked`;
write tests that call `close` twice and `setHeader` late.

**A streamed parse MUST equal a batch parse of the same bytes.** Route
each stream parser's lines through its *own* format's content-line
parser — the one the batch codec uses, TEXT unescaping included — and
unfold on bytes before decoding. The reference and more than one port
shipped a stream parser that reused the RFC 5545 line parser for
vCards, or skipped the unescape step the batch path applied, so
`FN:Last\, Comma` came back escaped when streamed and unescaped when
batched from identical input. The unescape now lives inside the
content-line parser in every implementation so both paths get it by
construction. Gate it: re-parse every corpus file through the stream
parser and compare against the batch parse.

**Helpers discipline.** Every mutator refreshes `X-VSTAR-HASH` as its
last statement, rejects out-of-range input rather than clamping it
(clamping `120` to `100` turns an off-by-one into legitimate-looking
data), and no-ops on a component-type mismatch. The emitter gate cannot
see the hash stamping — canonicalization strips `X-VSTAR-HASH` (rule 7)
before the bytes are compared — so a constructor that omits it passes
the gate. Assert the stamping directly in a unit test.

### Layer (f) — parity and documentation

Add the port's emitter to the parity harness under `tools/parity/`,
write the README (see
[README profile](#readme-profile-for-a-port)), and publish a
`VSTAR-CONFORMANCE.md` (see
[the template](#vstar-conformancemd-template)).

The gate is `make test-parity` green with the new emitter participating
— meaning your port's output is byte-identical to every other
implementation's across the shared corpus, not merely self-consistent.

## Hard rules

These are the ones that produce plausible-looking, silently wrong
output. Each has cost a real implementation real time.

### Byte comparisons compare BYTES

Canonical form is a byte sequence. Compare it as a byte sequence.

| Language | Type |
|---|---|
| TypeScript | `Uint8Array` |
| Python | `bytes` |
| Rust | `Vec<u8>` / `&[u8]` |
| PHP | binary `string` (PHP strings are byte arrays) |

**Never** compare trimmed or re-decoded text against a `.canonical`
file. Every one of these turns a failing port into a passing one while
the bytes stay wrong:

- Trimming trailing whitespace before comparing. The canonical form's
  final line terminator is part of the value.
- Round-tripping through a string type that normalizes line endings.
- Re-encoding through anything that might apply Unicode normalization
  of its own — which would mask an NFC bug in your canonicalizer by
  fixing it at comparison time.
- Comparing with a diff library that reports "equal ignoring
  whitespace".

The `.hash` fixtures are the backstop: a hash mismatch is unforgeable,
because SHA-256 over trimmed bytes is a different hash. If your
`.canonical` comparisons pass and your `.hash` comparisons fail, your
comparison is lying to you, not your hasher.

### Fixtures are LF on disk; encoders emit CRLF

Three separate facts, and all three matter:

1. **Fixture files on disk are LF-terminated.** Every `.ics`, `.vcf`
   and `.canonical` file in the corpus uses `\n`, for diff-friendliness
   and so Git's own line-ending handling never rewrites them.
2. **The canonical form is CRLF**, per spec rule 1 and RFC 5545 §3.1.
   Your encoder MUST emit `\r\n`. Always. The `.hash` values are
   computed over CRLF bytes.
3. **Parsers are liberal on input.** A parser MUST accept LF-only,
   CRLF, and a mixture. The corpus itself demands it — the `.ics`
   inputs are LF — and there is a `fuzz-seed/rfc5545/seed_02_lf_only`
   fixture specifically for it.

So the comparison against a `.canonical` file has exactly one licensed
transform: strip `\r` before `\n` in **your produced bytes**, then
compare to the file's bytes. The Go reference spells this `crlfToLF` in
[`go/cmd/fixtures-verify/main.go`](../../go/cmd/fixtures-verify/main.go);
each port needs the same one-line helper.

Do not do it the other way round. Converting the *file's* LF to CRLF
and comparing that against your output would also pass, right up until
your encoder emits a bare `\n` somewhere — which the transform would
then silently repair.

This is the one transform allowed. It is not a licence for the
whitespace normalization in
[the previous rule](#byte-comparisons-compare-bytes).

### Folding is 75 OCTETS, measured in UTF-8 bytes

RFC 5545 §3.1: a physical line, excluding its CRLF terminator, MUST NOT
exceed 75 **octets**. Octets, not characters, not code points, not
UTF-16 code units, not grapheme clusters.

This bites every language whose string length is not a byte count:

- TypeScript `String#length` counts UTF-16 code units. `'é'` is 1, its
  UTF-8 encoding is 2 bytes. An emoji is 2 UTF-16 units and 4 UTF-8
  bytes.
- Python `len(str)` counts code points. Encode to UTF-8 and measure
  that.
- Rust `str::len()` **is** the UTF-8 byte count — correct by default —
  but `chars().count()` is not.
- PHP `strlen()` is a byte count — correct — but `mb_strlen()` is not.

Fold on the byte length. A fixture with non-ASCII text in a long
`SUMMARY` will catch a code-point-counting implementation, and it will
catch it as a canonical-byte mismatch several layers away from the
folding code.

The widths are asymmetric: the first physical line carries 75 payload
octets, every continuation carries SP + 74. An encoder that alternates
75 and 74 across loop iterations emits a legal first continuation and
a 76-octet second one — the reference shipped exactly that in its
vCard encoder, and no fixture folded past one continuation until
`rfc6350/fold_long_note` did. Nor can a round-trip see it: unfolding
is width-agnostic, so an over-long line decodes to the same model.
Measure the physical lines your encoder emits.

The boundary may split a multi-byte sequence, and must. Retreating the
cut to a character boundary moves every later fold point and changes
the canonical bytes and the hash; a language whose string type cannot
hold half a sequence (Rust `String`, JavaScript strings) folds and
unfolds on bytes instead. `rfc5545/fold_split_utf8` is the only fixture
that reaches a fold point with non-ASCII content, which is why the
defect stayed invisible in two ports until it existed.

**Folding is applied AFTER property assembly, not before** (spec rule
3). Assemble the complete logical line — name, all parameters in
canonical order, value, with escaping applied — and fold that string.
Folding a value before appending parameters puts the fold points in the
wrong place, and the result unfolds to the same logical line, so a
round-trip test will not catch it. Only a byte comparison will.

### NFC normalization and where it sits

Spec rule 9: every property **value** and every parameter **value** is
normalized to Unicode Normalization Form C before encoding. Property
and parameter **names** are not normalized — they are uppercased per
RFC 5545 §3.1 / RFC 6350 §3.3 and are ASCII-only already. vCard group
prefixes (`group.NAME`) keep their case but are NFC-normalized as a
unit.

Codec **parsers** do NOT normalize. They stay lossless with respect to
input bytes. NFC is a canonicalization-time concern only.

The ordering, restated precisely, is the order canonicalization applies
its transforms to one component:

1. **Select** the properties to emit: drop `X-VSTAR-HASH` when hashing
   (rule 7); apply the `ATTACH` reference-only treatment, stripping
   `VALUE=BINARY` and `ENCODING=BASE64` (rule 10).
2. **Resolve datetimes** on the rule-5 allow-list (`DTSTAMP`,
   `DTSTART`, `DTEND`, `DUE`, `COMPLETED`, `RECURRENCE-ID`, `CREATED`,
   `LAST-MODIFIED`) against the calendar's VTIMEZONE registry, re-emitting
   as UTC form #2 and dropping the `TZID` parameter where resolution
   succeeds. `VALUE=DATE` properties are governed by rule 11 and are
   never resolved; `RRULE` (rule 8) and `DURATION` (rule 12) values pass
   through verbatim.
3. **Normalize text to NFC** — each property value and each parameter
   value, individually. Not names.
4. **Sort** properties alphabetically by name, and each property's
   parameters alphabetically by name (rule 2). Sort top-level
   components by `UID` (or `TZID` for a VTIMEZONE; keyless components
   last), stable, byte-wise on UTF-8 (rule 6). Sub-components keep
   input order. Byte-wise means a UTF-16 code-unit comparison
   (JavaScript `<`) is wrong for astral UIDs — `rfc5545/sort_utf8_uids`
   is built so the two orders disagree; TypeScript needs a dedicated
   comparator, Python `str`, Rust `str` and PHP `strcmp` already agree
   with the reference. Make the stability explicit with an index
   tiebreak rather than inheriting it from the sort: it is a spec
   requirement, not a library guarantee.
5. **Assemble** each content line: name, parameters, value, with
   escaping and parameter quoting applied (rule 4).
6. **Fold** each assembled line at 75 octets (rule 3).
7. **Terminate** every physical line with CRLF (rule 1).

NFC therefore happens **before** sorting, assembly and folding, and
**after** datetime resolution. Two consequences a port must get right:

- NFC before **sorting**: a value's normalized form is what the sort
  sees. (Names are ASCII and unaffected either way, but parameter
  *values* participate in nothing that sorts, so the practical effect
  is confined to the emitted bytes.)
- NFC before **folding**: normalization can change a string's UTF-8
  byte length — NFC composes `e` + U+0301 (3 bytes) into `é` (2 bytes)
  — so folding a pre-normalization string puts the fold at the wrong
  octet. Normalize, then measure, then fold.

NFC is idempotent, which is what makes the canonical form a fixpoint
under repeated canonicalization. Producer-supplied non-NFC text
canonicalizes to NFC and the original byte form is lost by design: the
canonical form is the equivalence-class representative.

Library note: use your platform's standard normalizer — `String#normalize('NFC')`
in TypeScript, `unicodedata.normalize('NFC', s)` in Python,
`unicode-normalization` in Rust, `Normalizer::normalize($s, Normalizer::FORM_C)`
(intl) in PHP. Do not hand-roll it, and do check the fast path: the Go
reference tests `IsNormalString` first and skips the allocation, which
is worth copying.

### No IANA timezone database

A V\* implementation resolves local times **only** against the
VTIMEZONE definitions inside the document being processed. It never
consults a system or bundled timezone database.

Forbidden, in each language:

- Python: `zoneinfo`, `pytz`, `dateutil.tz.gettz`
- Rust: `chrono-tz`, `tzfile`, `iana-time-zone`
- TypeScript: `Intl.DateTimeFormat` with a `timeZone` option,
  `Temporal.TimeZone.from('America/Montreal')`, `date-fns-tz`, `luxon`
  zone lookups
- PHP: `DateTimeZone('America/Montreal')`, `date_default_timezone_get()`

Implement the [VTIMEZONE subset](../../spec/v0.1/03-canonicalization.md#vtimezone-subset-for-tzid-resolution)
the spec defines: single `STANDARD`; `STANDARD` + `DAYLIGHT` with
`FREQ=YEARLY` rules accepting `BYMONTH` and an ordinal `BYDAY`;
`DAYLIGHT` only. Everything outside the subset is a **resolution
failure**, and a resolution failure is not an error — rule 5 falls back
to passing the value and its `TZID` parameter through verbatim.

The reason is determinism. A calendar carrying its own VTIMEZONE hashes
the same on every machine, in every year, regardless of which tzdata
release is installed. The moment an implementation reaches for the
system database, two conformant implementations can produce different
canonical bytes for the same document, which is the exact failure the
whole specification exists to prevent.

`spec/behavior/time/tzid.json` and
`spec/v0.1/conformance/time/america_montreal.ics` are the gate. They
name a real IANA zone, which is the trap: a port that resolves it from
the system database will pass those two fixtures and fail every other
zone.

### Registry constants are generated

[`spec/registry/`](../../spec/registry/) holds the tables every port
carries identically: `diagnostic-codes.json`,
`standard-properties.json`, `extension-scopes.json`,
`status-vocabulary.json`. [`tools/registry/gen.py`](../../tools/registry/)
renders them into per-language constants and into
[`docs/validate-codes.md`](../validate-codes.md). A table is authored
once and never retyped per port.

**Hand-written port source MUST NOT contain a literal diagnostic
code** — not in a `switch`, not in a test assertion, not in a comment
a future reader might copy. Reference the generated constant by its
meaning-bearing name (`CodeMissingUID` and friends, re-cased per the
[API mapping](api-mapping.md#generated-diagnostic-code-constants)).
Generated files are the sole home of the literal code strings, and
they say so in their header.

This document follows its own rule: no code literal appears anywhere
in it, which is why the layer-(d) gate above describes the §8
value-domain diagnostics by their meaning instead of naming them. Do
the same in your port's prose, so a search for a code string finds
only the generated module and the generated catalog.

Each port needs its own **`registry-check`-equivalent step** in CI,
separate from its test suite. Running the generator in check mode must
diff the rendered output against what is committed and exit non-zero on
drift.

Exclude the generated module from every formatter and linter the port
runs (`eslint` ignores, `ruff` excludes, `rustfmt::skip` on the module
declaration, php-cs-fixer and PHPStan excludes). The generator emits
its own layout — gofmt column alignment in Go, for instance — and a
formatter that reflows it makes `registry-check` fail on a tree nobody
edited.

A test alone is not sufficient, and the reason is specific: the Go
build cache does **not** track files above the module root. The Go
module root is [`go/`](../../go/); the registry lives at
`spec/registry/`, above it. Edit the registry, change nothing under
`go/`, and `go test ./...` returns `ok (cached)` — reporting success
against the previous registry. The check step has to read the registry
files directly, every run, with no caching in the path.

Every port has an analogous hazard even though the mechanism differs:
a `tsc` incremental build, a `pytest` cache, a `cargo` fingerprint, a
Composer autoload map. Do not reason about whether yours is affected —
just run the generator in check mode as its own CI step.

### Generators must diff before writing

A code generator that writes unconditionally is not a gate. It is a
repair tool that hides the thing you wanted to catch.

The rule: **a generator diffs its rendered output against what is on
disk, and exits non-zero naming every path that differs.** In check
mode it writes nothing at all. In write mode it may write, but it still
reports what it changed.

The failure mode this prevents: someone hand-edits a generated file —
to add a code, to change a severity, to fix a typo — and the next
generator run silently overwrites it. The hand-edit is gone, no one is
told, and if the edit was an attempt to fix a real bug, that bug is
still there. Worse in the other direction: a generator that repairs a
tampered generated file on its way to a green build means a CI run can
pass while the committed tree is wrong.

**All three existing generators in this repository had this defect**
and all three were fixed by adding the diff-and-fail path. Assume yours
will have it too. The Go side's shape is worth copying: `gen --check`
renders in memory, compares against each committed file, prints a
unified diff for every mismatch, and exits 1 — it never opens a file
for writing in that mode.

The same discipline applies to the fixture generator. `make
fixtures-verify` regenerates the corpus and then fails if the working
tree changed, which is the same property expressed through Git rather
than through the generator's own diff. Both are acceptable; silently
writing is not.

## Make targets each port must provide

Every port provides these four, named for its language
(`test-ts`, `lint-ts`, `build-ts`, `package-ts`, and likewise `-py`,
`-rs`, `-php`):

| Target | Contract |
|---|---|
| `test-<lang>` | Runs the port's full test suite, including every corpus gate in the table above. Exit non-zero on any failure. No network access. |
| `lint-<lang>` | Runs the port's linter and formatter in check mode. Does not rewrite files. |
| `build-<lang>` | Compiles / type-checks the port. For interpreted languages this is the type-check step (`tsc --noEmit`, `mypy`, or equivalent). |
| `package-<lang>` | Produces the publishable artifact — tarball, wheel, crate, PHAR/Composer package — without publishing it. |

Plus the port's own `registry-check` equivalent, wired into whatever
aggregate CI gate the repository defines. Do not fold registry checking
into `test-<lang>`; see
[registry constants are generated](#registry-constants-are-generated)
for why.

The root `Makefile` and the per-language CI workflows are owned
elsewhere in the repository — add your targets there rather than
inventing a parallel entry point, so `make test-ts` works from the repo
root exactly as `make test` does for Go.

## Fixture loaders each port implements

Three loaders cover the whole corpus. Write them once, early — before
layer (a)'s gate — because every subsequent gate consumes them.

### Conformance quartets

Directories:
[`spec/v0.1/conformance/rfc5545/`](../../spec/v0.1/conformance/rfc5545/),
[`rfc6350/`](../../spec/v0.1/conformance/rfc6350/),
[`supersession/`](../../spec/v0.1/conformance/supersession/),
[`time/`](../../spec/v0.1/conformance/time/).

A case is a basename with up to four files:

| Extension | Content |
|---|---|
| `.ics` / `.vcf` | The input document. LF-terminated on disk. |
| `.canonical` | The expected canonical bytes, with CRLF folded to LF on disk. |
| `.hash` | The expected `X-VSTAR-HASH` value, `sha256:<hex>`, one line. |
| `.notes.md` | Prose explaining what the case exercises. Not asserted; read it when a case fails. |

The loader enumerates basenames in a directory, reads whichever
siblings exist, and yields a case. `.canonical` and `.hash` are
optional per case — `malformed/` has neither.

Comparison: parse the input, canonicalize, apply the CRLF→LF transform
to **your bytes**, compare to the `.canonical` file's bytes. Hash the
**CRLF** bytes (not the LF-transformed ones) and compare the string to
the `.hash` file's contents, trimmed of its trailing newline.

[`malformed/`](../../spec/v0.1/conformance/malformed/) is the same
directory shape with a different sibling: a `.error` file holding one
sentinel identifier. The input MUST fail with that sentinel.

### Behavior families

Directory: [`spec/behavior/`](../../spec/behavior/), one subdirectory
per family — `time/`, `duration/`, `validate/`, `ext/`,
`supersession/`, `diff/`.

Families are not uniform, and the loader should not pretend they are.
Two shapes appear:

- **Paired**: an `.ics` input with a same-basename `.json` sibling
  naming the expectation. `validate/` uses `.diagnostics.json`.
- **Standalone**: a single `.json` holding a list of self-contained
  cases, each with its own inputs and expectations. `time/tzid.json`
  and the `duration/` files are this shape.

Read [`spec/behavior/README.md`](../../spec/behavior/README.md) for
what each family asserts, and read the JSON of each family before
writing its loader — the field names are per-family and are the
contract.

### The rrule sidecar set

Directory:
[`spec/v0.1/conformance/rrule/`](../../spec/v0.1/conformance/rrule/),
with subdirectories `happy/`, `rejected/`, `bounds/`, `by-clauses/`,
`evaluator/`, `expansion/`, `format/`, `set/`.

A case is a `.rrule` file holding the rule text, plus one or more
sidecars naming what to assert:

| Sidecar | Assertion |
|---|---|
| `.expect.json` | `{"sentinel": "..."}` — parsing or validating MUST fail with this sentinel. |
| `.next.json` | `{"dtstart": ..., "after": ..., "expected": [...]}` — walking `nextOccurrence` from `after` yields exactly this list. |
| `.formatted` | Re-emitting the parsed rule produces exactly these bytes. |
| `.notes.md` | Prose. Not asserted. |

Instants in the JSON are RFC 5545 form #2 strings
(`YYYYMMDDTHHMMSSZ`) — parse them with your own `parseTime`, not with a
general-purpose ISO-8601 parser, so the sidecar exercises the same code
path the documents do.

A `.rrule` with no sidecar asserts only that it parses.

## README profile for a port

A port's README is the **OSS / Library-SDK** profile. Required
sections, in this order:

1. **Name + one-liner** — what the package is, in one sentence.
2. **Badges** — release, CI, license, and for a Library-SDK a
   **types badge** (published type definitions for TypeScript; the
   equivalent typing signal for the others: `py.typed`, `docs.rs`,
   PHPStan level).
3. **Why** — the problem this solves and why a reader would choose it.
   Two or three sentences, plus the honest counterpart: when *not* to
   use it.
4. **Install** — the package-manager command, and the runtime version
   floor.
5. **Usage example** — a minimal, complete, runnable program showing
   the most common path: parse, canonicalize, hash.
6. **API examples** — the Library-SDK addition: short examples of the
   main surfaces (validate, rrule, helpers) beyond the one usage
   example.
7. **License + links** — license, spec, the monorepo, the issue
   tracker.

### Three audiences, none generated from another

There are three README shapes in this repository and they are not
variants of one document:

| README | Audience | Shape |
|---|---|---|
| Root [`README.md`](../../README.md) | Someone landing on the repo | **Repo map.** What lives where, which language directory to open, how the spec and the implementations relate. |
| [`spec/README.md`](../../spec/README.md) | Someone implementing or citing the spec | **Spec front matter.** Versions, how to read it, what is out of scope. **No install. No usage example.** There is nothing to install — it is a document. |
| `<lang>/README.md` | Someone adding the library to their project | **OSS / Library-SDK**, the profile above. |

Do not generate one from another, and do not copy the root README's
structure into a port. A port README that opens with a repo map is
answering a question its reader did not ask; a spec README with an
`npm install` line is offering something that does not exist.

### `go/README.md` lacks the required **Why**

The current [`go/README.md`](../../go/README.md) goes
Install → Quick start → What you get → Where to go next → Related
projects → License. It has no **Why** section, and no badges.

It is the nearest model a port author will reach for, so: do not copy
its section order. Its opening paragraph does describe what the module
is, but describing what something is is not the same as saying why a
reader would choose it, and the Library-SDK profile wants both. Add the
**Why** to your port, and the Go README will be brought into line
separately.

## `VSTAR-CONFORMANCE.md` template

[spec/05](../../spec/v0.1/05-conformance.md#self-certification) asks
every implementation to publish one. Until a formal conformance suite
exists, this is the honor-system substitute — and for a port, most of
it is a statement of which gates are green.

Put it at the port's root (`ts/VSTAR-CONFORMANCE.md`, and so on):

```markdown
# V* conformance — <implementation name>

## Spec revision

Targets spec revision <commit hash of this repository>.

## Implementation class

<emitter | consumer | round-trip>

Round-trip means: documents this implementation emits re-parse and
re-emit byte-identically.

## Conformance criteria

Criteria are numbered per spec/v0.1/05-conformance.md.

| # | Criterion | Status |
|---|---|---|
| 1 | Emits valid RFC 5545 / RFC 6350 | <met / partial / not met> |
| 2 | Emits required common properties | ... |
| 3 | Honors canonicalization rules | ... |
| 4 | Uses correct component types | ... |
| 5 | Respects the extension namespace | ... |
| 6 | Is append-only (supersession) | ... |
| 7 | Keeps duration values well-formed | ... |
| 8 | Keeps enumerated and integer values in their RFC domains | ... |

## Failure classes

All twelve sentinels are surfaced and distinguishable: <yes / list the
gaps>. The identifier is recoverable from a caught failure via
<the language's mechanism>.

## Corpus gates

| Gate | Status |
|---|---|
| conformance round-trip (rfc5545, rfc6350) | ... |
| malformed sentinels | ... |
| canonical bytes | ... |
| hash values | ... |
| behavior/time, behavior/duration | ... |
| rrule sidecars | ... |
| behavior/validate | ... |
| behavior/{ext,supersession,diff} | ... |
| parity emitter | ... |

## Known deviations

<Each deviation with its rationale, or "None.">

## Test artifacts

<Golden documents and their X-VSTAR-HASH values, or a pointer to the
corpus run that produced them.>
```

Fill it honestly. "Partial" with a named gap is more useful to the next
port author than "met" that turns out not to be.

## See also

- [API mapping](api-mapping.md) — every exported symbol in every
  language, the twelve sentinels, the `(value, ok)` versus `error`
  distinction, and time representation per language.
- [Specification](../../spec/) — the normative text.
- [Setup the dev environment](setup.md) — clone, toolchain, `make ci`.
- [Add a feature track end-to-end](contributing-flow.md) — TDD posture,
  Conventional Commits, spec changes, conformance fixtures.
- [How to build a sister vstar implementation](../user/how-to-implement-vstar.md)
  — the adopter-facing walkthrough of cross-validation.
