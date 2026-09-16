# Build a sister vstar implementation

Cross-validate a new V* implementation against the Go reference by
reproducing canonical bytes and `X-VSTAR-HASH` values byte-for-byte.

## Use this when

- You are writing a V* implementation in a language this repo does
  not ship (the TypeScript, Python, Rust and PHP ports live here
  already) and need a definitive correctness gate.
- You want your implementation to claim "V* conformant" per
  [spec/05](../../spec/v1.0/05-conformance.md).
- You are integrating an existing iCal/vCard codec and need to know
  exactly what V* layers on top of RFC 5545 / RFC 6350.

## Result

After completing this guide, you will:

- Run your implementation against the shared
  `spec/v1.0/conformance/` corpus and emit byte-identical
  `.canonical` and `.hash` files.
- Compute `X-VSTAR-HASH` using the same SHA-256-over-canonical-bytes
  algorithm the Go reference uses.
- Know which canonicalization rules to implement, in what order,
  and which spec rule backs each one.

## Before you begin

You need:

- A working iCalendar (RFC 5545) and/or vCard (RFC 6350) codec for
  your target language. V* does not replace these — it layers on
  top.
- A SHA-256 implementation in your standard library or crypto
  toolkit.
- A Unicode NFC normalizer (every modern platform ships one).
- A clone of `hop-top/poly-vstar` — it carries the spec, the
  conformance corpus and the Go reference.

## Quick version

1. Implement RFC 5545 / RFC 6350 codec (or use an existing one).
2. Implement canonical form per spec/03 (rules below).
3. Implement `X-VSTAR-HASH` as `"sha256:" + hex(sha256(canonical_bytes))`.
4. Run your implementation against `spec/v1.0/conformance/`; every
   `.canonical` and `.hash` sibling must match byte-for-byte.

## Steps

### 1. Implement the codec layer

V* is a convention over RFC 5545 / RFC 6350. Your codec must:

- Parse the wire form into a typed in-memory model (Component,
  Property, Param, Calendar, Card).
- Preserve property order, parameter order, and component order
  verbatim on parse — canonicalization happens later.
- Emit CRLF-terminated lines folded at 75 octets per RFC 5545 §3.1.
- Reject malformed input with a documented sentinel (V*'s Go
  reference uses `ErrMalformed`, `ErrUnclosedBlock`,
  `ErrUnsupportedVersion`).

The Go reference splits this work into:

- `codec/rfc5545` — VCALENDAR parser + encoder.
- `codec/rfc6350` — VCARD parser + encoder.
- `codec/stream` — bounded-memory iterator codecs.

Mirror the split in your language or collapse it; only the
external behavior is normative.

### 2. Implement canonical form

[spec/03](../../spec/v1.0/03-canonicalization.md) defines the canonical
byte form. Implement these rules in order:

| # | Rule | Source |
|---|---|---|
| 1 | CRLF line endings. | RFC 5545 §3.1 |
| 2 | Properties sorted alphabetically by `Name`; parameters within each property sorted alphabetically by `Name`. | spec/03 rule 2 |
| 3 | Lines folded at 75 octets AFTER property assembly. | spec/03 rule 3 / RFC 5545 §3.1 |
| 4 | Property and parameter values normalized to Unicode NFC. | spec/03 rule 9 |
| 5 | Datetime properties resolved to UTC form #2 (`YYYYMMDDTHHMMSSZ`) when `TZID=` parameter and matching VTIMEZONE are available; the `TZID` parameter is then stripped. | spec/03 rule 5 / §Datetime resolution |
| 6 | Top-level components sorted within a Calendar by `UID` (or `TZID` for VTIMEZONE) lexicographically. Nested sub-components (`STANDARD`/`DAYLIGHT`, `VALARM`) preserve input order. | spec/03 rule 6 |
| 7 | `X-VSTAR-HASH` stripped from output before hashing — defense in depth. | spec/03 rule 7 |
| 8 | `ATTACH` properties: `VALUE=BINARY`/`ENCODING=BASE64` reduced to URI form. | spec/03 rule 10 |
| 9 | `RRULE` property values preserved byte-for-byte. The canonical layer does NOT normalize rule-part order, drop defaulted rule-parts, or re-format integer lists. | spec/03 rule 8 |

The Go reference's `canonical.Calendar` is the executable definition.
When in doubt, run a fixture through it and inspect the output.

### 3. Implement X-VSTAR-HASH

The algorithm is exactly:

```
X-VSTAR-HASH = "sha256:" + lowercase_hex(sha256(canonical_bytes))
```

Where `canonical_bytes` is the output of step 2 with any existing
`X-VSTAR-HASH` property stripped first. The literal `"sha256:"`
prefix exists per [spec/03](../../spec/v1.0/03-canonicalization.md) §7
to allow algorithm migration in a later version without ambiguity
(a future `"sha3-256:"` or `"blake3:"` prefix). v1.0 only emits
`"sha256:"`.

The Go reference exposes:

- `hashing.Component(c) → "sha256:<hex>"` — single-component hash.
- `hashing.Calendar(cal) → "sha256:<hex>"` — Calendar-level hash
  (routes datetimes through the Calendar's VTIMEZONE registry).
- `hashing.Card(c) → "sha256:<hex>"` — vCard hash.
- `hashing.SetXVSTAR(&c)` — compute and store on the component.
- `hashing.VerifyXVSTAR(c) → (ok, want, got)` — recompute and
  compare against stored value.

Mirror the API shape in your language, or collapse — only the
output bytes are normative.

### 4. Cross-validate against the corpus

[`spec/v1.0/conformance/`](../../spec/v1.0/conformance/) is the
shared conformance corpus — the authored source every
implementation consumes identically. (`go/testdata/` is a
generated mirror of it, kept in the Go module so the published
`hop.top/vstar` module stays self-contained; consume the spec tree,
not the mirror.) Layout:

| Subdir | Format | Purpose |
|---|---|---|
| `rfc5545/` | `.ics` | VCALENDAR happy-path round-trip goldens. |
| `rfc6350/` | `.vcf` | VCARD happy-path round-trip goldens. |
| `supersession/` | `.ics` | Supersession edge cases. |
| `malformed/` | `.ics` / `.vcf` | Inputs that MUST fail to parse with a documented sentinel. |
| `fuzz-seed/` | `.bytes` | Codec fuzz-target seeds. |

Each parseable fixture is a quartet sharing a stem `<name>`:

| File | Required | Content |
|---|---|---|
| `<name>.ics` / `.vcf` | yes | Fixture input. LF on disk for diff-friendliness. |
| `<name>.canonical` | parseable inputs | Canonical bytes (CRLF-terminated). |
| `<name>.hash` | parseable inputs | `sha256:<64 hex>` + LF (exactly 72 bytes). |
| `<name>.notes.md` | optional | Human prose describing the fixture. |
| `<name>.error` | malformed inputs | Name of the expected sentinel (one per line). |

Your implementation must:

- **Consume**: parse each `.ics`/`.vcf`, canonicalize, hash —
  produce identical `.canonical` and `.hash` content byte-for-byte.
- **Emit**: build the equivalent Calendar/Card via your API, encode
  it, canonicalize, hash — produce the same `.canonical` and
  `.hash`.
- **Round-trip**: parse, encode, parse again — yield a semantically
  equal Calendar/Card.

The Go reference asserts these via `make fixtures-verify` (see the
target's source for the regeneration loop). Your implementation
should ship the same kind of CI gate.

### 5. Document deviations

Until a formal conformance suite exists, every implementation
SHOULD publish a `VSTAR-CONFORMANCE.md` per
[spec/05](../../spec/v1.0/05-conformance.md) §"Self-certification"
covering:

- Spec revision targeted (commit hash of `hop-top/poly-vstar`).
- Implementation class (emitter / consumer / round-trip).
- Known deviations + rationale.
- Test artifacts (golden V* documents + their `X-VSTAR-HASH`
  values).

This is the v1.0 honor-system substitute for a real test suite.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Your hash differs from the corpus `.hash` for `rfc5545/empty.ics` | Property order, NFC normalization, or line folding diverges. | Compare your canonical bytes against the corpus `.canonical` byte-by-byte. The first divergent byte points at the rule. |
| `nested_vtimezone.ics` produces the wrong hash | TZID resolution skipped — VTIMEZONE registry not consulted. | Implement the spec/03 §Datetime resolution two-tier API: Component-level canonicalization preserves wire form; Calendar-level resolves TZID through the Calendar's VTIMEZONE registry. |
| `RRULE` property values reordered or normalized | Canonical layer reformatted RRULE. | RRULE values are preserved byte-for-byte per spec/03 §"RRULE preserved verbatim". Strip your normalization pass. |
| `ATTACH:data:base64,...` survives canonicalization | ATTACH binary form not stripped to URI. | Per spec/03 rule 10, `VALUE=BINARY`/`ENCODING=BASE64` reduces to URI form. |
| `malformed/` fixtures parse successfully | Your parser is too lenient. | Compare against the matching `.error` sentinel; tighten the parser to reject. |

## How it works

The canonical form is the equivalence-class representative of all
wire forms that decode to the same logical content. The hash is
computed over the canonical form so two implementations that agree
on canonical bytes produce identical hashes — the foundational
invariant for cross-language integrity.

The Go reference's package layout (`canonical/` separate from
`hashing/` separate from `codec/rfc5545/`) is dictated by Go's
import-cycle rules and is not normative. You can collapse layers
in your language; only the output bytes matter.

The corpus is the executable spec. When the spec text and the
corpus disagree, the corpus wins until the next coordinated PR
updates both — see the [contributing flow](../dev/contributing-flow.md)
for how that PR is shaped.

## Next steps

- [Specification §01 — overview](../../spec/v1.0/01-overview.md) — design
  principles + scope.
- [Specification §03 — canonicalization](../../spec/v1.0/03-canonicalization.md) —
  the normative byte-form rules.
- [Diagnostic code catalog](../validate-codes.md) — every rule a
  conformant validator should emit, with severity and spec linkage.
- [Conformance corpus README](../../spec/v1.0/conformance/) — the
  cross-language fixture contract.
