# V* behavior fixtures

Language-agnostic tables stating what a conformant V* implementation
does. The `spec/v0.1/conformance/` corpus pins the *wire* contract —
what parses, what canonicalizes to which bytes, which hash. These
fixtures pin the *behavior* contract: which diagnostics a document
raises, what a diff between two documents reports, which status a
supersession ledger projects, when an alarm fires, how an extension
name classifies, and how a local timestamp resolves against a
VTIMEZONE.

Every file here is generated from the Go reference implementation by
`go -C go run ./cmd/fixtures-gen-behavior`, which
`make fixtures-verify` runs for you. Do not hand-edit them: the
target regenerates the tree and the caller's `git diff` fails on any
difference, so an edit either reverts or shows up as drift.

This README is the exception — it is hand-authored and the generator
leaves it alone.

## Why these exist

Until this tree existed, the behavior above lived only in the Go
module's unit tests. A port (TypeScript, Python, Rust, PHP) could
recover it only by reading Go and hand-translating assertions: slow
to write, and silently divergent the moment the reference moved.

A port now writes one table-driven test per family, reads the JSON,
and compares. When the reference's behavior changes deliberately,
the regenerated fixtures carry the change to every port in the same
commit.

## Rules that hold across every family

**Messages are never normative.** No fixture records a diagnostic
message, an error string, or any other human-readable prose. That
text is reworded between versions without the behavior changing; a
port that pinned it would fail on an editing pass. The stable
surface is codes, paths, severities, op kinds, and failure classes.

**`error` may replace `expected` in any sidecar.** Where a call can
fail, the fixture states the failure instead of a result: the
`error` field carries a failure-class token from
[spec/05 §Failure classes](../v0.1/05-conformance.md#failure-classes)
— `ErrMalformed`, `ErrNoTrigger`, `ErrNoAnchor` and the rest — and
the success fields are absent. A port branches on which field is
present. The token names a class, not a message: an implementation
satisfies it by failing in that class, however it words the failure.

**Every timestamp is RFC 5545 form #2.** UTC, `Z`-suffixed,
`YYYYMMDDTHHMMSSZ`. One shape to parse, in every family, on both
sides of a comparison. The one deliberate exception is the `value`
column of `time/tzid.json`, whose whole subject is form #1 (local,
no `Z`) input.

**JSON is indented with two spaces and ends in a newline.** These
files are review surface: a drifting fixture must produce a readable
line-level diff.

**A list is never `null`.** An empty result renders as `[]` and an
empty map as `{}`, so a port's decoder handles one shape.

## Families

### `validate/<name>.ics` + `<name>.diagnostics.json`

The diagnostics `validate.Validate` raises for a whole calendar.

```json
[
  {
    "code": "VS040",
    "severity": "error",
    "path": "VCALENDAR.VTODO[uid=todo-no-due]"
  }
]
```

- `code` — the stable catalog identifier, documented in
  `docs/validate-codes.md`.
- `severity` — `"error"` (a MUST violation) or `"warning"`.
- `path` — the dotted locator. `VCALENDAR.<TYPE>[uid=<uid>]` for a
  component, `…[#<n>]` when it has no UID to key on, with a trailing
  `.<PROPERTY>` when the finding is property-level.

**Sort order: by `path`, then by `code`.** The reference emits
diagnostics in check order, which is an implementation detail; the
sorted form is the contract. A port sorts its own output the same
way before comparing.

Every diagnostic code the reference can emit has at least one
fixture, and the generator refuses to run if one does not — adding a
new code without a fixture fails the build. Two fixtures
(`clean_vevent`, `clean_vtodo_completed`) are clean documents whose
sidecar is `[]`; they are what stops a port from passing by
reporting every document as broken.

`missing_uid_positional` carries **two** UID-less VTODOs so the
sidecar pins `[#0]` against `[#1]`. Every other fixture has at most
one UID-less component per type, so a counter frozen at `[#0]` passed
the whole family — three ports found that by mutation testing before
this fixture existed.

**What this family still cannot reach.** Property names are all
uppercase, so case-insensitive matching is untested; the
malformed-duration code is reached only through `DURATION`, never
through a relative `TRIGGER` or a `REPEAT`; and no fixture carries a
present-but-blank `RELATED-TO` or `X-VSTAR-EFFECTIVE-STATUS`, which
the reference treats as absent. A port pins those with its own unit
tests.

### `diff/<name>.a.ics` + `<name>.b.ics` + `<name>.diff.json`

The structural changes between two calendars.

```json
[
  {
    "uid": "todo-change",
    "path": "VCALENDAR.VTODO[uid=todo-change]",
    "ops": [
      {
        "op": "change",
        "property": "DUE",
        "before": "20260101T000000Z",
        "after": "20260202T000000Z"
      }
    ]
  }
]
```

- `uid` — the component's identity, lifted out of `path` so the
  common case needs no path parsing. Empty when the component has no
  UID and is addressed positionally.
- `path` — the same locator syntax the validate family uses.
- `ops` — the property-level changes, `"add"`, `"remove"` or
  `"change"`. `before` is null on an add, `after` is null on a
  remove.
- `before_params` / `after_params` — the property's parameters, when
  it has any. A parameter-only change is a `"change"` op whose
  `before` and `after` values are equal and whose parameter lists
  differ; this is why the format carries them apart from the value.
- `subs` — the same shape recursively, for changed sub-components
  (a VALARM inside a VEVENT). Present only when non-empty.

**Sort order: components in pairing order, `ops` by property name,
case-insensitively.** Unchanged components and unchanged
sub-components do not appear at all. `X-VSTAR-HASH` is excluded from
both sides, matching the canonicalization rule — a diff reports what
changed in the content, not the restamped hash that followed.

`identical.diff.json` is `[]`: two equal documents produce no entry,
not an entry with no ops.

**Neither sort rule can be verified from these fixtures alone, and a
port that relies on them to check its ordering is checking nothing.**
Every case here yields exactly one component diff, so pairing order
and any sorted order coincide; and the RFC 5545 encoder upper-cases
property names on write, so mixed-case names reach a port already
folded (`X-ALPHA`, `X-BETA`) and a byte sort agrees with a
case-insensitive one on every input the corpus can express. Adding a
fixture does not fix this — the limitation is in the wire format, not
in the case list. A port pins these two rules with its own tests
built on in-memory components: one calendar mixing a UID-bearing with
a UID-less component (pairing order differs from sorted order), and
one component carrying two properties whose case-insensitive and
byte orderings disagree (`X-alpha` before `X-Beta`, since `'B' <
'a'`). The Go reference does this in `go/diff/diff_test.go`.

### `supersession/<name>.effective.json`

The status a supersession ledger projects onto each component.

```json
{
  "todo-1": "COMPLETED"
}
```

Keyed by component UID; the value is the effective status. The
inputs are the existing conformance fixtures at
`spec/v0.1/conformance/supersession/<name>.ics` — no new `.ics`
files are minted here, so a port that already loads that corpus gets
this table keyed by the same file names.

**A UID absent from the map is not superseded.** The ledger has
nothing to say about it and it stands as written. That is why
`corrupt_mutated.effective.json` is `{}`: the fixture's hash is
broken, but no supersession entry targets it. Supersession is a
projection query, not a validator — the hash violation belongs to
the validate family.

`multi_step` is the tie-break case: two entries target the same
component and the later `DTSTAMP` wins.

### `duration/parse.json`

Every RFC 5545 §3.3.6 duration value the reference was asked to
parse.

```json
[
  { "value": "-PT15M", "seconds": -900, "negative": true },
  { "value": "P1W2D", "error": "ErrMalformed" }
]
```

- `seconds` — the whole duration as a signed second count, the one
  representation every target language has.
- `negative` — the sign flag, kept separate because a zero-length
  duration can be written with a leading `-` and `seconds` alone
  cannot express that.
- `error` — replaces both on a malformed value.

Note that the reference normalizes the sign of a zero-length
duration: `-PT0S` reports `"negative": false`. That is behavior a
port must reproduce, not a rounding artifact.

Coverage spans the weeks form, the days form, time-only forms, the
combined day-and-time form, both signs, zero, and the malformed
space: empty input, a missing `P`, an empty body, mixing weeks with
other units, mixing weeks with a time part, an unknown unit, units
out of RFC order, digits with no unit, a unit with no digits, and
lowercase input.

### `duration/<name>.ics` + `<name>.trigger.json`

When each VALARM in the document fires.

```json
[
  { "alarm_uid": "alarm-start-explicit", "fires_at": "20260601T084500Z" },
  { "alarm_uid": "alarm-no-anchor", "error": "ErrNoAnchor" }
]
```

- `alarm_uid` — the VALARM's UID, keying the entry to the `.ics`.
- `fires_at` — the resolved instant.
- `error` — replaces it with `ErrNoTrigger`, `ErrNoAnchor` or
  `ErrMalformed`.

**Order: document order** — parent components in calendar order,
VALARMs in the order they appear inside their parent.

Coverage: a relative trigger against `RELATED=START` both explicit
and implicit (the parameter's default, which a port that ignores it
will still get right for one of the two and wrong for neither —
hence both are present), a relative trigger against `RELATED=END`,
the absolute form with and without `VALUE=DATE-TIME`, a VTODO whose
`RELATED=END` anchors to `DUE` rather than `DTEND`, a VEVENT with no
end anchor at all (`ErrNoAnchor`), a VALARM with no TRIGGER
(`ErrNoTrigger`), and both directions of the `VALUE`-parameter
contradiction — a duration under `VALUE=DATE-TIME` and an instant
under `VALUE=DURATION` (`ErrMalformed`). An explicit `VALUE` is
authoritative: a contradicting value is rejected, never silently
re-read as the other form.

### `ext/scopes.json`

How each extension name classifies under spec/04.

```json
[
  { "name": "X-VSTAR-HASH", "scope": "vstar", "system": null },
  { "name": "X-AGR-FOO", "scope": "system", "system": "AGR" }
]
```

- `scope` — one of `"vstar"`, `"system"`, `"experimental"`,
  `"none"` (not an extension at all) or `"unknown"` (has the `X-`
  prefix, matches no sanctioned tier). Lowercase, so a port needs no
  case convention of its own.
- `system` — the owning system's slug, and `null` for every scope
  but `"system"`. It answers "which system owns this property?",
  which only a `"system"`-scoped name has an answer to.

**Order: as listed.** The file is one flat table, not a set.

Classification is case-insensitive: `x-vstar-hash` classifies the
same as `X-VSTAR-HASH`. The reservation on `VSTAR` and `EXP` covers
the whole slug segment, not a prefix of it — `X-VSTARLIKE-FOO` is an
ordinary system extension owned by `VSTARLIKE`.

### `time/tzid.json`

How a local timestamp resolves against a named zone.

```json
[
  {
    "calendar": "america_montreal",
    "tzid": "America/Montreal",
    "value": "20260315T020000",
    "utc": "20260315T060000Z"
  }
]
```

- `calendar` — the conformance fixture supplying the VTIMEZONE
  registry: `spec/v0.1/conformance/time/<calendar>.ics`. Load that
  `.ics`, then resolve `value` against `tzid` within it.
- `value` — RFC 5545 form #1: local wall time, no `Z`. This is the
  one family whose input is not form #2.
- `utc` — the resolved instant, in form #2.

**`utc` is `null` for every rejection**, and null is the whole
contract: the reference reports a boolean, not an error, so there is
no failure class to name here. A port must reject an unknown zone,
an empty TZID, a form #2 value (already absolute — resolving it
against a zone would apply an offset twice), a date-only value, and
anything that is not a form #1 timestamp.

Coverage draws on both DST seasons, both transition boundaries
(spring forward at 02:00 on the second Sunday of March, fall back at
02:00 on the first Sunday of November, each sampled either side),
and the trivial fixed-offset zone.

The VTIMEZONE subset the reference implements is deliberately small;
`spec/v0.1/conformance/time/README.md` states what is supported and
what returns a rejection.

## Regenerating

```sh
make fixtures-verify
```

That regenerates this tree along with the conformance corpus and
fails if anything differs from what is committed. A difference means
one of two things: the implementation drifted, and the fix is in the
implementation — or the behavior changed on purpose, and the
regenerated fixtures belong in the same commit as the change.

The generator writes nothing when there is no sibling `spec/` tree.
The published Go module is a subtree of `go/` alone and ships no
spec, so it verifies its own corpus in place and skips this family
entirely; its behavior coverage there is its unit tests.
