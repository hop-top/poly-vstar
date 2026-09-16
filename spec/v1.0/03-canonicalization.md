<!-- SPDX-License-Identifier: CC-BY-4.0 -->

# V* — Canonicalization

> The byte-for-byte rules below are normative, implemented in the Go
> reference implementation ([`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar))
> and cross-validated by every port against the same corpus;
> divergence is a bug in whichever implementation drifts.

## Goal

Two V\* documents containing the same logical content MUST produce
identical canonical byte sequences and identical `X-VSTAR-HASH`
values.

## Areas requiring canonical form

1. **Line endings.** CRLF, per RFC 5545.
2. **Property order within a component.** Alphabetical by property
   name; parameters in the same alphabetical order.
3. **Folding.** RFC 5545 line folding (75-octet boundary) applied
   AFTER property assembly, not before. The boundary counts **octets,
   not characters**: a multi-byte UTF-8 sequence straddling it is split
   across the fold, so an individual physical line is not necessarily
   valid UTF-8 on its own. Decoders MUST unfold before any
   character-level interpretation — unfolding rejoins the sequence and
   the logical line decodes cleanly. Implementations whose string type
   cannot hold a split sequence must fold and unfold on bytes rather
   than retreating the cut to a character boundary: retreating changes
   where the folds land, and therefore changes the canonical bytes and
   the hash.
4. **Property parameter values.** Quoted forms canonical (per RFC).
5. **Datetime forms.** For DATE-TIME values only (date-only values
   follow rule 11): UTC (`Z`-suffixed) preferred for V\*; if local
   time is needed, an explicit `TZID=` parameter referencing a
   VTIMEZONE in the same VCALENDAR. Canonical form resolves such
   local times to UTC — see [Datetime resolution](#datetime-resolution).
6. **Component order within a VCALENDAR.** Top-level components are
   sorted by `UID` property value using a byte-wise (UTF-8
   lexicographic) comparison. The sort is stable: components with
   equal UIDs (a producer bug) preserve their relative input order.
   A top-level VTIMEZONE has no UID; its `TZID` value is the sort
   key. A top-level component with neither UID nor TZID (a producer
   bug) sorts to the END of the calendar in stable input order.
   Nested sub-components (`STANDARD`/`DAYLIGHT` inside VTIMEZONE,
   `VALARM` inside VEVENT) have no natural sort key and preserve
   their input order.
7. **`X-VSTAR-HASH` exclusion.** When computing the hash for a
   component, the `X-VSTAR-HASH` property itself is excluded.
8. **`RRULE` property values preserved verbatim.** `RRULE` property
   values are preserved byte-for-byte in canonical form; the
   canonical layer does not normalize rule-part order, drop
   defaulted rule-parts, or re-format integer lists. Two RRULEs
   that decompose to the same `rrule.Rule` but differ in wire
   shape (e.g. `FREQ=DAILY;INTERVAL=1` vs `FREQ=DAILY` —
   semantically identical because INTERVAL defaults to 1) hash
   differently. Producers needing semantic equivalence MUST
   normalize the wire form before emit; canonicalization is a
   byte-level discipline. Which RRULE features an implementation
   MUST parse is a separate question — see
   [RRULE parsing scope](#rrule-parsing-scope).
9. **Text normalization.** Every property value and every parameter
   value is normalized to Unicode Normalization Form C (NFC) before
   encoding. Property and parameter names are not normalized — they
   are uppercased per RFC 5545 §3.1 / RFC 6350 §3.3 and already
   ASCII-only. vCard group prefixes (`group.NAME`) keep their case
   but are NFC-normalized as a unit. Codec parsers do NOT normalize
   on parse; they stay lossless with respect to input bytes. NFC is
   a canonicalization-time concern only, so producer-supplied
   non-NFC text canonicalizes to NFC and the original byte form is
   lost by design (the canonical form is the equivalence-class
   representative). NFC is idempotent, which keeps the canonical
   form a fixpoint under repeated canonicalization.
10. **`ATTACH` handling.** Reference-only on emit. Canonical form
    treats every `ATTACH` value as a URI: the `VALUE=BINARY`
    parameter and any `ENCODING=BASE64` parameter are stripped
    before encoding. If the value is not a syntactically valid URI,
    the property is preserved as-is (canonical form is best-effort
    for malformed inputs). Inline (`VALUE=BINARY`) attachments
    round-trip losslessly through the codec layer — parse and
    encode preserve them byte-for-byte — but no V\* helper writes
    them, and validators MAY warn on them. Inline emit is not part
    of v1.0.
11. **`DATE` values canonicalize as themselves.** A property whose
    value type is DATE (`VALUE=DATE`, RFC 5545 §3.3.4) is emitted
    verbatim as `YYYYMMDD`. The `VALUE=DATE` parameter is retained
    and upper-cased; it is load-bearing, since the default value
    type for `DTSTART`, `DTEND`, `DUE`, and `COMPLETED` is
    DATE-TIME. Any `TZID` parameter is stripped (RFC 5545 §3.2.19
    scopes TZID to DATE-TIME and TIME values). A DATE value is never
    promoted to a DATE-TIME; the
    [Datetime resolution](#datetime-resolution) registry is not
    consulted.
12. **`DURATION` values preserved verbatim.** A `DURATION` value
    (RFC 5545 §3.3.6), including the relative form of `TRIGGER`, is
    emitted exactly as authored: the canonical form MUST NOT
    normalize units (`P1D` and `PT24H` are distinct values — a day
    is not 24 hours across a UTC-offset transition), MUST NOT mix
    the week form with the day/time form, and MUST preserve the
    sign, which applies to the whole value. A zero duration is
    `PT0S`. The `TRIGGER` property's two forms are told apart under
    [TRIGGER conventions](#trigger-conventions).

## Datetime resolution

Rule 5 applies to the v1.0 datetime allow-list: `DTSTAMP`,
`DTSTART`, `DTEND`, `DUE`, `COMPLETED`, `RECURRENCE-ID`, `CREATED`,
and `LAST-MODIFIED`. For each such property:

- **No `TZID` parameter.** Pass through verbatim. UTC form #2
  (`Z`-suffixed) values are already canonical; a bare local-time
  value without a TZID is non-conforming producer output and
  cannot be recovered.
- **`TZID` present, value already form #2 (`Z` suffix).** Pass
  through verbatim (contradictory wire output; canonical preserves
  the producer bytes rather than re-shaping).
- **`TZID` present, value is form #1 (local), and the referenced
  VTIMEZONE resolves** under the [VTIMEZONE subset](#vtimezone-subset-for-tzid-resolution)
  below. Resolve the wall-clock time against that VTIMEZONE,
  re-emit the value as UTC form #2 (`YYYYMMDDTHHMMSSZ`), and drop
  the `TZID` parameter.
- **`TZID` present, VTIMEZONE missing or outside the subset.** Pass
  the value AND the `TZID` parameter through verbatim. Canonical
  bytes are NOT deterministic across calendars carrying different
  VTIMEZONE definitions in this branch; the producer is expected to
  ship VTIMEZONE coverage inside the subset. The wire bytes carry
  no marker for this fallback.

A listed property carrying `VALUE=DATE` is governed by rule 11 and
is never resolved.

Resolution needs the enclosing VCALENDAR's VTIMEZONE registry, so
implementations expose two tiers: a component-level canonical form
that emits datetimes verbatim (for components carrying no TZID
datetimes) and a context-taking form that resolves them against a
supplied Calendar. Calendar-level canonicalization routes every
child through the context-taking form using the calendar itself as
the registry.

`STANDARD` and `DAYLIGHT` children inside a VTIMEZONE carry a
wall-clock `DTSTART` that defines the transition rule itself; those
values are deliberately not TZID-tagged and pass through verbatim.

A form #1 value that falls in a DST gap (a wall time that does not
exist) or overlap (one that occurs twice) is **not** resolved as
ambiguous or absent: the offset selected by the VTIMEZONE rules is
applied arithmetically. A port reaching for an idiomatic local-time
conversion — one that returns "none" or "ambiguous" for those instants —
will disagree with the reference. The `time/tzid` behaviour fixtures pin
both cases.

### VTIMEZONE subset for TZID resolution

TZID resolution accepts a documented subset of VTIMEZONE shapes in
v1.0. Accepted:

1. **Single `STANDARD` only** (no `DAYLIGHT`): fixed-offset zones.
   The child MUST carry `TZOFFSETTO` and `TZOFFSETFROM` (`±HHMM` or
   `±HHMMSS`) and `DTSTART` (form #1); MAY carry `TZNAME`; MAY omit
   `RRULE`.
2. **Single `STANDARD` + single `DAYLIGHT`.** Both children MUST
   carry `TZOFFSETTO`, `TZOFFSETFROM`, `DTSTART`, and an `RRULE`
   with `FREQ=YEARLY`. The RRULE accepts `BYMONTH=<1-12>` and
   `BYDAY=<n><WEEKDAY>` (non-zero signed ordinal; weekday
   `SU|MO|TU|WE|TH|FR|SA`), and `INTERVAL=1` (no-op). Any other
   `INTERVAL` is rejected.
3. **`DAYLIGHT` only**: treated as a single fixed-offset rule using
   the child's `TZOFFSETTO`.

Rejected (resolution fails and rule 5 falls back to verbatim):

- Multiple `STANDARD` or multiple `DAYLIGHT` children (split-zone
  histories).
- `RRULE` without `FREQ=YEARLY`.
- `RRULE` containing `UNTIL`, `COUNT`, `BYWEEKNO`, `BYSETPOS`,
  `BYHOUR`, `BYMINUTE`, `BYSECOND`, `BYYEARDAY`, `BYMONTHDAY`,
  `WKST`, or any unknown key.
- `BYDAY` without an explicit ordinal (`SU`, `0SU`).
- RDATE-only zones (transitions enumerated as discrete dates).
- Missing `TZOFFSETTO`, `TZOFFSETFROM`, or `DTSTART` in any child.

This subset covers every current IANA zone with active DST. A
failed resolution is a strict failure mode: consumers see the
unresolved local time immediately rather than a silently partial
rule. This narrow, VTIMEZONE-only RRULE handling is distinct from
the generic RRULE parsing scope below.

## RRULE parsing scope

Canonical form never interprets an `RRULE` (rule 8). Implementations
that expose a generic RRULE parser and evaluator follow this
scope; the `rrule/` conformance fixtures exercise it.

Accepted rule-parts:

- `FREQ` — required; `MINUTELY`, `HOURLY`, `DAILY`, `WEEKLY`,
  `MONTHLY`, `YEARLY`.
- `INTERVAL` — positive integer; default 1.
- `UNTIL` — RFC 5545 form #2 (UTC, `Z`-suffixed) only. Form #1
  (local) and form #3 (with TZID) are rejected as malformed: V\*'s
  strict-UTC posture for RRULE bounds.
- `COUNT` — positive integer; mutually exclusive with `UNTIL`.
- `BYDAY` (`[<ordinal>]<weekday>`, ordinal in -53..-1, 1..53 or
  omitted), `BYMONTH` (1..12), `BYMONTHDAY` (-31..-1, 1..31; 0
  rejected), `BYHOUR` (0..23), `BYMINUTE` (0..59), `BYSECOND`
  (0..60; 60 retained for leap seconds).
- `BYSETPOS` — positional filter on the expanded BY-set; requires
  at least one other BY-* clause.
- `BYWEEKNO` — ISO 8601 week-number expansion, `FREQ=YEARLY` only,
  WKST-aware.
- `BYYEARDAY` — day-of-year expansion (1..366 or -366..-1, 0
  rejected), `FREQ=YEARLY` only.
- `WKST` — single weekday; default `MO`.

Rule-part order is irrelevant on parse.

Malformed (hard error): unknown rule-part name, missing `FREQ`,
`BYMONTHDAY=0`, `BYDAY` ordinal 0, `INTERVAL` zero or negative, both
`UNTIL` and `COUNT` present, `UNTIL` in form #1 or form #3.

Deferred (parses syntactically but reported as unsupported):

- `FREQ=SECONDLY` — extreme expansion; no realistic agentic use
  case.
- `RSCALE` (RFC 7529, non-Gregorian calendars) — deferred
  indefinitely; not on the V\* roadmap.

Evaluator behaviour: `FREQ=MINUTELY|HOURLY|DAILY|WEEKLY|MONTHLY|YEARLY`
with any combination of the accepted rule-parts; WKST-aware week
boundaries; `BYMONTHDAY=29` for February in a non-leap year skips
that occurrence, and `BYMONTHDAY=-1` always selects the last day.
Each `BY-*` rule-part limits or expands per the RFC 5545 §3.3.10
table for the rule's `FREQ` — in particular `BYHOUR` limits under
`HOURLY` and `MINUTELY`, and `BYMINUTE` limits under `MINUTELY`;
fixtures `rrule/evaluator/hourly_byhour_limit` and
`rrule/evaluator/minutely_byhour_byminute_limit`.
Parser scope equals evaluator scope: everything the parser accepts,
the evaluator evaluates.

### RRULE wire form

Rule 8 governs canonical form: an `RRULE` value is preserved
verbatim and hashed as authored. This section governs emitters. An
implementation that emits an `RRULE` value from a parsed rule MUST:

- Emit rule-parts in this order, omitting absent ones: `FREQ`,
  `INTERVAL`, `UNTIL`, `COUNT`, `BYMONTH`, `BYWEEKNO`, `BYYEARDAY`,
  `BYMONTHDAY`, `BYDAY`, `BYHOUR`, `BYMINUTE`, `BYSECOND`,
  `BYSETPOS`, `WKST`.
- Omit `INTERVAL=1` and `WKST=MO` (the RFC 5545 defaults).
- Keep list values (`BYDAY=MO,WE`, `BYMONTHDAY=-1,15`) in their
  authored order; lists are not sorted.

Parsing the wire form and re-emitting it MUST be idempotent. This is
the normalization rule 8 already requires of producers ("MUST
normalize the wire form before emit"); rule 8 itself is unchanged —
two values that differ only in rule-part order or a spelled-out
default still hash differently. Fixtures: `rrule/format/`.

### Expansion

A rule with neither `UNTIL` nor `COUNT` is unbounded. An
implementation MUST bound expansion by an occurrence count (a
limit), by a window, or by laziness (a sequence the consumer
stops). A bounded expansion MUST report whether the series
completed within the bound or was truncated by it.

A window is half-open: `start` inclusive, `end` exclusive. A request
for an unbounded window — no `end`, or `end` not after `start` — is
the failure class `ErrUnboundedExpansion`, not an empty result.

The evaluator MUST have a finite, documented iteration bound; its
value is implementation-defined. The reference bound is 100 000
periods; under `FREQ=MINUTELY` at `INTERVAL=1` that is about 69
days, so a rule whose limits admit nothing for longer (for example
`FREQ=MINUTELY;BYMONTH=1` evaluated from February) reports
`ErrIterationCap` rather than the eventual occurrence. Fixture:
`rrule/evaluator/minutely_sparse_limit_caps`, which assumes a bound
below 480 960 minute periods. Reaching the bound MUST be
distinguished from the rule terminating (`UNTIL` reached, `COUNT`
exhausted): it is the failure class `ErrIterationCap`, never an
empty or completed result. The parser stays permissive on
unsatisfiable BY-* combinations
(`FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30` parses); such rules surface
at evaluation as `ErrIterationCap`. Fixtures: `rrule/expansion/`,
`rrule/evaluator/unsatisfiable_feb_30`.

### Recurrence sets (RDATE/EXDATE/RECURRENCE-ID)

A recurrence set is evaluated in this order:

1. `DTSTART` is the first occurrence.
2. `RRULE`, if present, expands from `DTSTART`.
3. `RDATE` values are merged in.
4. `EXDATE` values are removed — last, so an excluded instant stays
   excluded even when an `RDATE` names it.
5. The result is sorted and de-duplicated by instant.

A set with `RDATE` and no `RRULE` is legal and finite. Bounded
expansion of a set follows the [Expansion](#expansion) rules;
`EXDATE` removals do not count toward a limit.

In v1.0, `EXDATE` and `RDATE` values MUST be UTC form #2; `VALUE=DATE`
and `TZID` forms are rejected with `ErrUnsupportedRRule`. `EXDATE`
and `RDATE` are not on the [datetime resolution](#datetime-resolution)
allow-list, so a `TZID`-bearing value passes through canonical form
verbatim; adding them to the list is a change for a later version.
`RECURRENCE-ID` is on the list and is resolved before set evaluation;
it parses to an instant plus the `RANGE` parameter (`THISANDFUTURE`).
Applying overrides to the expanded set is out of v1.0 scope. Fixtures:
`rrule/set/`.

## TRIGGER conventions

Rule 12 covers the relative form of `TRIGGER` (RFC 5545 §3.8.6.3).
The property's two forms are told apart as follows:

- An explicit `VALUE` parameter is authoritative. `VALUE=DURATION`
  with a value that is not a duration, or `VALUE=DATE-TIME` with a
  value that is not an instant, is malformed.
- Absent `VALUE`, the value's form decides: a duration is a relative
  trigger; a form #2 instant is an absolute trigger.
- `RELATED` applies only to relative triggers. It defaults to `START`
  and MUST NOT appear on an absolute trigger (RFC 5545 §3.2.14).
- Emitters SHOULD omit `RELATED=START` and SHOULD write absolute
  triggers as `VALUE=DATE-TIME` in UTC form #2.

Canonical form leaves every `TRIGGER` as authored; these conventions
bind emitters and validators, not the canonical layer.

## Hashing

`X-VSTAR-HASH` SHOULD be `sha256:<hex>` of the component's
canonical form (with the hash property removed). The `sha256:`
prefix allows future algorithm migration.

## Open questions

Resolved at v1.0 (now normative above):

- Locale-specific normalization for text-bearing properties
  (SUMMARY, DESCRIPTION) — resolved as NFC required (rule 9).
- Handling of binary attachments (`ATTACH`) — inline vs reference
  — resolved as reference-only on emit (rule 10).
- Canonical form of `DATE` values — resolved as verbatim `YYYYMMDD`
  with `VALUE=DATE` retained (rule 11). Inputs spelling `VALUE=date`
  (RFC-conforming; parameter tokens are case-insensitive) or
  carrying a stray `TZID` on a DATE (non-conforming per RFC 5545
  §3.2.19) change hash under rule 11 — accepted as gap-fill, since
  rule 5 never covered DATE.

**Deferred to a later version:**

- General case-folding of `VALUE=` tokens other than `DATE`. In
  v1.0 a `VALUE=date-time` spelling hashes differently from
  `VALUE=DATE-TIME`.
- vCard profile selection (vCard 4.0 baseline vs allowing 3.0?).
- Cross-VCALENDAR references (URI scheme for `RELATED-TO` across
  ledger files).
- RRULE canonical normalization — normalizing rule-part order and
  elided defaults in the canonical form. Breaking: it changes every
  stored hash, VTIMEZONE rules included. The emitter rule in
  [RRULE wire form](#rrule-wire-form) makes the switch a no-op for
  conformant emitters.

The Go implementation in this repository is the reference; the
TypeScript, Python, Rust and PHP ports validate the locked rules
against it, emitter for emitter, in parity CI. Any downstream emitter
cross-validates against the Go output the same way.
