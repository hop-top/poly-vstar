<!-- SPDX-License-Identifier: CC-BY-4.0 -->

# V* — Conformance

> Status: **draft**. Conformance criteria firm up alongside a
> conformance test suite (planned for v0.2).

## What "V\* conformant" means

An implementation is V\* conformant when it:

1. **Emits valid RFC 5545 / RFC 6350.** Output passes a generic
   iCalendar/vCard validator.

2. **Emits the required common properties** on every component:
   `UID`, `DTSTAMP`, `X-VSTAR-HASH` (per `02-component-mapping.md`).

3. **Honors canonicalization rules** (per `03-canonicalization.md`):
   identical logical content → identical bytes → identical hash.

4. **Uses the correct component types** for the agentic concepts it
   represents (per the mapping table).

5. **Respects the extension namespace** (per `04-extensions.md`):
   no top-level new components; all extensions in `X-*`.

6. **Is append-only**: original components are never mutated;
   state changes use the supersession pattern.

7. **Keeps duration values well-formed.** `DURATION`, the relative
   form of `TRIGGER`, and the `REPEAT` count MUST be well-formed per
   RFC 5545 §3.3.6 / §3.8.6.2, and `TRIGGER` MUST honor the `VALUE`
   and `RELATED` constraints of `03-canonicalization.md` rule 12.
8. **Keeps enumerated and integer values inside their RFC 5545
   domains.** `STATUS` MUST be a value RFC 5545 §3.8.1.11 scopes to
   the component's own type: `TENTATIVE`, `CONFIRMED` or `CANCELLED`
   on VEVENT; `NEEDS-ACTION`, `IN-PROCESS`, `COMPLETED` or
   `CANCELLED` on VTODO; `DRAFT`, `FINAL` or `CANCELLED` on VJOURNAL.
   `CLASS` (§3.8.1.3: `PUBLIC`, `PRIVATE`, `CONFIDENTIAL`) and
   `TRANSP` (§3.8.2.7: `OPAQUE`, `TRANSPARENT`) MUST likewise be
   drawn from their RFC vocabularies. Vocabulary values compare
   case-insensitively. `PRIORITY` MUST be 0–9, 0 meaning undefined
   (§3.8.1.9); `PERCENT-COMPLETE` MUST be 0–100 (§3.8.1.8);
   `SEQUENCE` (§3.8.7.4) and `REPEAT` (§3.8.6.2) MUST be
   non-negative. Every such integer is written as a canonical
   decimal: no sign, no leading zeros, no whitespace.

## Implementation classes

V\* implementations come in three flavors:

- **Emitter**: produces V\* documents (e.g. AGR's compiler).
- **Consumer**: reads V\* documents and projects state.
- **Round-trip**: both, with byte-identical re-emit guaranteed
  for documents it emitted.

The conformance test suite (v0.2) will exercise each class
separately.

## Failure classes

An implementation reports rejected or abandoned input by failure
class. The corpus names a class by its token: `.expect.json`
(`"sentinel"`) and the evaluator sidecars (`"error"`) under
`conformance/rrule/` state which class an input MUST produce.

The token is the cross-language identity of the class. An
implementation MUST expose every class below under exactly the token
spelled here, whatever its host language names the underlying error
value, so that a failure raised by one implementation is recognizable
to every other. The table is the complete set for v0.1: an
implementation MUST NOT invent further tokens, and SHOULD map any
condition of its own onto the class that already covers it.

| class | corpus token | meaning |
|---|---|---|
| Malformed | `ErrMalformed` | Structurally invalid input: bad escape, bad parameter syntax, unparseable value, or an RRULE on the parsing scope's hard-error list. |
| Unclosed block | `ErrUnclosedBlock` | A `BEGIN` line lacks its matching `END` before end of input. |
| Unsupported version | `ErrUnsupportedVersion` | A `VERSION` property is present but is neither vCard 4.0 nor iCalendar 2.0. |
| Missing UID | `ErrMissingUID` | A component that requires `UID` has none. |
| Unsupported RRULE | `ErrUnsupportedRRule` | Syntactically valid input outside the RRULE parsing scope: `FREQ=SECONDLY`, `FREQ=MINUTELY`, `RSCALE`, or a non-UTC `EXDATE`/`RDATE`/`RECURRENCE-ID` value. |
| Iteration cap | `ErrIterationCap` | The evaluator reached its iteration bound without finding an occurrence; the rule did not terminate. |
| Unbounded expansion | `ErrUnboundedExpansion` | A request to expand into a list with no bound: a zero or inverted window, or a negative limit. |
| Missing trigger | `ErrNoTrigger` | A `VALARM` without a `TRIGGER`; the property is mandatory (RFC 5545 §3.6.6), so the alarm cannot be scheduled. |
| Unanchored relative trigger | `ErrNoAnchor` | A relative trigger resolved against a component lacking the anchoring `DTSTART` (`RELATED=START`) or `DTEND` / `DTSTART`+`DURATION` / `DUE` (`RELATED=END`); resolution fails rather than resolving against the zero time. |
| Corrupted supersession target | `ErrTargetCorrupted` | A component offered as a supersession target carries an `X-VSTAR-HASH` that does not match its recomputed canonical hash — it was mutated after hashing. Supersession MUST refuse rather than chain a new entry onto an unverifiable target. A target carrying no `X-VSTAR-HASH` makes no integrity claim, so it is accepted and MUST NOT raise this class. |
| Encoder already closed | `ErrAlreadyClosed` | A streaming encoder was used after its `Close`: either a second `Close`, or an encode of a further component. The closed encoder has already written its trailer, so it MUST report this class rather than silently discarding the write. |
| Encoder header locked | `ErrHeaderLocked` | A streaming encoder's calendar header was set after the first component was encoded. The header is written once, on that first encode, and is immutable thereafter so that the `BEGIN:VCALENDAR` / `VERSION` / `PRODID` ordering on the wire stays deterministic. |

## Reference implementations

All five in-tree implementations live in `hop-top/poly-vstar`, one
tree per language, and are republished to per-language read-only
mirrors on release.

| Implementation | Class | Status | Source tree | Mirror |
|---|---|---|---|---|
| `hop.top/vstar` (Go) | Round-trip | Reference implementation | `go/` | `hop-top/vstar` |
| `@hop-top/vstar` (TypeScript) | Round-trip | Cross-validated against the Go reference by parity CI | `ts/` | `hop-top/vstar-ts` |
| `hop-top-vstar` (Python) | Round-trip | Cross-validated against the Go reference by parity CI | `py/` | `hop-top/vstar-py` |
| `hop-top-vstar` (Rust) | Round-trip | Cross-validated against the Go reference by parity CI | `rs/` | `hop-top/vstar-rs` |
| `hop-top/vstar` (PHP) | Round-trip | Cross-validated against the Go reference by parity CI | `php/` | `hop-top/vstar-php` |
| AGR (Racket) | Emitter + Consumer | In development | `hop-top/agr` | — |

Each port publishes its own `VSTAR-CONFORMANCE.md` beside its source,
stating its class, its green gates and its known deviations. The Go
tree publishes none: it is the reference the others certify against,
so its emitted bytes define conformance rather than attest to it.
Those
deviations are confined to API surface — error shape, instant
representation, duration units, method naming — and none of them
changes emitted bytes; the parity gate would fail if one did.

## Self-certification

Until a formal conformance suite exists, implementations
SHOULD publish a `VSTAR-CONFORMANCE.md` documenting:

- Spec revision targeted (commit hash of this repo)
- Implementation class (emitter / consumer / round-trip)
- Known deviations + rationale
- Test artifacts (golden V\* documents + their `X-VSTAR-HASH`
  values)

This is the v0.1 honor-system substitute for a real test suite.
