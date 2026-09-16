# V\* validate — diagnostic code catalog

This file enumerates every `Diagnostic.Code` emitted by the
`hop.top/vstar/validate` package.

## Stability

Codes are part of the library's public surface. They are **stable
across Go library minor versions** per semver:

- A given Code never changes its meaning.
- A code is never recycled for an unrelated rule.
- New codes may be added in any release; consumers must tolerate
  unknown codes (and log them rather than crash).
- Renaming a code is a major-version event.

`Diagnostic.Message` is human-readable and may evolve across minor
versions; do not match it programmatically. Use Code instead.

## Severity

| Severity            | Meaning                                                     |
|---------------------|-------------------------------------------------------------|
| `SeverityError`     | MUST violation per spec/05 — document is not V\* conformant |
| `SeverityWarning`   | SHOULD violation or stylistic concern                       |

## Codes

### §1 — required common properties (spec/05 §1)

| Code  | Severity | Rule                                                |
|-------|----------|-----------------------------------------------------|
| VS001 | Error    | Required common property `UID` is missing.          |
| VS002 | Error    | Required common property `DTSTAMP` is missing.      |
| VS003 | Error    | Required common property `X-VSTAR-HASH` is missing. |

### §2 — X-VSTAR-HASH integrity (spec/05 §2)

| Code  | Severity | Rule                                                              |
|-------|----------|-------------------------------------------------------------------|
| VS010 | Error    | `X-VSTAR-HASH` is present but does not match the recomputed hash. |

VS010 fires only when the hash property is **present and wrong**.
When it is absent entirely, VS003 fires instead.

### §3 — extension namespace compliance (spec/05 §3, spec/04)

| Code  | Severity | Rule                                                                            |
|-------|----------|---------------------------------------------------------------------------------|
| VS020 | Warning  | Property name is not on the RFC 5545/6350 allow-list and lacks the `X-` prefix. |

The standard property allow-list is sourced from RFC 5545 §3.7-§3.8
and RFC 6350 §6, authored once in
`spec/registry/standard-properties.json` and rendered into every
implementation's generated module (`go/validate/codes_gen.go` for
Go). The `X-` classification itself is delegated to `ext`; the
allow-list stays with `validate` because it answers the orthogonal
question "is this a known RFC property?", which `ext` has no business
knowing.

### §4 — supersession discipline (spec/05 §4)

| Code  | Severity | Rule                                                                                                                                              |
|-------|----------|---------------------------------------------------------------------------------------------------------------------------------------------------|
| VS030 | Error    | A supersession `VJOURNAL` (CATEGORIES contains `status-supersession`) is missing a required property: `RELATED-TO` or `X-VSTAR-EFFECTIVE-STATUS`. |
| VS031 | Error    | A supersession `VJOURNAL`'s `RELATED-TO` does not resolve to any component in the same Calendar (orphan supersession).                            |

VS030 fires from both `Validate` and `ValidateComponent` (it is a
component-local check). VS031 requires cross-component resolution,
so it fires from `Validate` only — `ValidateComponent` (single-
component entry) silently skips it because it has no ledger to
resolve against.

### §5 — type-specific required properties (spec/05 §5)

| Code  | Severity | Rule                                                                          |
|-------|----------|-------------------------------------------------------------------------------|
| VS040 | Error    | `VTODO` requires `DUE`, OR `STATUS=COMPLETED` paired with `COMPLETED`.        |
| VS041 | Error    | `VEVENT` requires `DTSTART`.                                                  |
| VS042 | Error    | `VFREEBUSY` requires `DTSTART` AND `DTEND`.                                   |
| VS043 | Error    | `VCARD` (modeled as `Component{Type: "VCARD"}`) requires `VERSION` AND `UID`. |

### §6 — RRULE conformance (spec/03 §RRULE parsing scope)

| Code  | Severity | Rule                                                                                                                                                                                           |
|-------|----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| VS050 | Warning  | `RRULE` value parses but uses a feature outside the RRULE parsing scope (FREQ=SECONDLY, RSCALE — see [spec/03 §RRULE parsing scope](../spec/v1.0/03-canonicalization.md#rrule-parsing-scope)). |
| VS051 | Error    | `RRULE` value is malformed per RFC 5545 §3.3.10 (missing FREQ, INTERVAL≤0, both UNTIL+COUNT, BYMONTHDAY=0, UNTIL not in form #2, etc.).                                                        |

VS050 is a Warning because the property still round-trips through
the codec layer; only its recurrence semantics are inaccessible
to the RRULE evaluator. Consumers using `rrule.NextOccurrence` MUST
check for VS050 before relying on the result.

VS051 is an Error because a malformed RRULE means no consumer
(vstar or otherwise) can evaluate the recurrence correctly.

### §7 — DURATION well-formedness (spec/03 rule 12, spec/05 criterion 7)

| Code  | Severity | Rule                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
|-------|----------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| VS052 | Error    | A duration-bearing property value is malformed: the `DURATION` property, the relative (`DURATION`-valued) form of `TRIGGER`, or a `REPEAT` count that is not a non-negative integer (RFC 5545 §3.3.6 / §3.8.6.2) written as a canonical decimal — no sign, no leading zeros, no whitespace (see [spec/05 §8](../spec/v1.0/05-conformance.md#what-v-conformant-means)); or a `TRIGGER` whose `VALUE` parameter contradicts its value, or that carries `RELATED` on an absolute trigger — see [spec/03 §TRIGGER conventions](../spec/v1.0/03-canonicalization.md#trigger-conventions). |

VS052 is an Error because a duration that will not parse is not a
cosmetic defect. A malformed `TRIGGER` means the alarm cannot be
scheduled at all and consumers drop it silently; a malformed
`DURATION` on a `VEVENT` leaves the event with no computable end.
spec/05 criterion 7 makes well-formed duration values a MUST.

An **absolute** `TRIGGER` — `VALUE=DATE-TIME`, or an RFC 5545
form #2 instant with no `VALUE` parameter — is not a duration and is
validated as a date-time instead. Only a value that is neither a
well-formed duration nor a well-formed instant is flagged, as is a
`VALUE` parameter that contradicts its value, or `RELATED` on an
absolute trigger (which RFC 5545 §3.2.14 scopes to relative
triggers). The wire form itself is never rewritten: spec/03 rule 12
keeps `DURATION` and `TRIGGER` values verbatim in canonical form.

Parsing and resolution live in `hop.top/vstar/duration`.

### §8 — enumerated and integer value domains (spec/05 §8)

| Code  | Severity | Rule                                                                                                                                                                                                                                                                                                        |
|-------|----------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| VS044 | Error    | `STATUS` value is outside the vocabulary RFC 5545 §3.8.1.11 scopes to the component's own type.                                                                                                                                                                                                             |
| VS053 | Error    | `CLASS` value is outside the RFC 5545 §3.8.1.3 vocabulary `PUBLIC`, `PRIVATE`, `CONFIDENTIAL` (compared case-insensitively — see [spec/05 §8](../spec/v1.0/05-conformance.md#what-v-conformant-means)).                                                                                                     |
| VS054 | Error    | `TRANSP` value is outside the RFC 5545 §3.8.2.7 vocabulary `OPAQUE`, `TRANSPARENT` (compared case-insensitively — see [spec/05 §8](../spec/v1.0/05-conformance.md#what-v-conformant-means)).                                                                                                                |
| VS055 | Error    | An integer-valued property is not a canonical decimal (no sign, no leading zeros, no whitespace) inside its RFC 5545 domain: `PRIORITY` 0–9 (§3.8.1.9), `PERCENT-COMPLETE` 0–100 (§3.8.1.8), `SEQUENCE` non-negative (§3.8.7.4) — see [spec/05 §8](../spec/v1.0/05-conformance.md#what-v-conformant-means). |

VS044 covers two shapes with one code: a value unknown to every
vocabulary (`STATUS:FROBNICATED`), and a value that is legal
iCalendar text but wrong for this component type (`STATUS:DRAFT`
on a `VEVENT`). The vocabularies are:

| Component  | Allowed `STATUS` values                                |
|------------|--------------------------------------------------------|
| `VEVENT`   | `TENTATIVE`, `CONFIRMED`, `CANCELLED`                  |
| `VJOURNAL` | `DRAFT`, `FINAL`, `CANCELLED`                          |
| `VTODO`    | `NEEDS-ACTION`, `IN-PROCESS`, `COMPLETED`, `CANCELLED` |

`CANCELLED` is the only value shared by all three. Comparison is
case-insensitive per spec/05 §8 (and RFC 5545 §3.1). Component
types with no `STATUS` vocabulary (`VFREEBUSY`, `VTIMEZONE`,
`VALARM`, `VCALENDAR`) are skipped entirely.

VS053 and VS054 are the `VS044` shape for the two closed vocabularies
spec/05 §8 binds beside `STATUS`: `CLASS` (RFC 5545 §3.8.1.3:
`PUBLIC`, `PRIVATE`, `CONFIDENTIAL`) and `TRANSP` (§3.8.2.7:
`OPAQUE`, `TRANSPARENT`). Comparison is case-insensitive, like
VS044. A `CLASS` value outside the three — including an `X-` or
IANA token, which RFC 5545's ABNF admits — is an Error, because
spec/05 §8 says the value MUST be drawn from the vocabulary and V\*
consumers agree on every byte they hash; the reference's own
accessor would otherwise fold an unrecognized classification to the
least restrictive value.

VS055 is the `VS052` shape for the three bounded integer properties:
one parser, one code, the path names the property. A value is
well-formed when it is a **canonical decimal** — it matches
`^(0|[1-9][0-9]*)$`: digits only, no sign, no leading zero unless
the value is exactly `0`, no whitespace. The rule is textual: the
value is never converted to a machine integer to decide
well-formedness, so a `SEQUENCE` past any language's integer range
(`18446744073709551616`) is well-formed. Only where RFC 5545 bounds
the value is a magnitude compared: `PRIORITY` ≤ 9 (§3.8.1.9),
`PERCENT-COMPLETE` ≤ 100 (§3.8.1.8); `SEQUENCE` (§3.8.7.4) has no
upper bound.

`REPEAT` (§3.8.6.2) stays under VS052, not VS055: a published code
never changes meaning, `REPEAT` counts repetitions of the alarm
`DURATION`, and spec/05 criterion 7 lists it with `DURATION` and
`TRIGGER`. Its arm now applies the same canonical-decimal check, so
`REPEAT:+1` and `REPEAT:01` are VS052 where `SEQUENCE:+1` is VS055.

All three codes check the value wherever the property appears; none
gates on component type. **Component scope is not diagnosed**:
`TRANSP` on a `VTODO`, `PERCENT-COMPLETE` on a `VEVENT`, `REPEAT`
outside a `VALARM`, and the RFC 5545 §3.6.6 `DURATION`/`REPEAT`
pairing all pass. V\* diagnoses no scope rule for any property
today, and `Validate` does not descend into sub-components, so a
scope rule would be unreachable for every nested `VALARM`. The
`helpers` write-time gating stays the only scope enforcement.

## Path syntax

`Diagnostic.Path` is a dotted component/property locator.

Examples:

| Path                                 | Meaning                                  |
|--------------------------------------|------------------------------------------|
| `VCALENDAR`                          | Calendar-level diagnostic.               |
| `VCALENDAR.VTODO[uid=foo]`           | Component-level diagnostic on a VTODO.   |
| `VCALENDAR.VTODO[uid=foo].DTSTAMP`   | Property-level diagnostic on its DTSTAMP. |
| `VCALENDAR.VTODO[#3]`                | UID-less VTODO at positional index 3.    |
| `VTODO[uid=foo].DTSTAMP`             | `ValidateComponent` (no calendar prefix).|
