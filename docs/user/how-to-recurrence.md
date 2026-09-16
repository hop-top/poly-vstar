# Parse and evaluate RRULE

Decode an RFC 5545 §3.3.10 `RRULE` value into a typed `rrule.Rule`
and walk its occurrences with `NextOccurrence`.

## Use this when

- You receive a `VTODO` or `VEVENT` whose `RRULE` you need to inspect
  beyond round-tripping it as an opaque string.
- You drive recurring work (cron-like agentic tasks, recurring
  ledger postings) and need the next firing instant.
- You want to drop a non-vstar iCal library kept solely for
  `ParseRecurrenceRule` (the motivating use case).

## Result

After completing this guide, you will:

- Parse common RRULE shapes (`FREQ=DAILY`, `BYDAY=MO,WE,FR`,
  `BYSETPOS=-1` for "last weekday of month").
- Step through occurrences with `NextOccurrence` until the rule
  terminates.
- Distinguish *unsupported* RRULE features (`ErrUnsupportedRRule`,
  fall back gracefully) from *malformed* RRULE (`vstar.ErrMalformed`,
  refuse the input).

## Before you begin

You need:

- The Go reference implementation imported (the `rrule` subpackage
  ships with it).
- A working knowledge of RFC 5545 §3.3.10 RRULE syntax.

## Quick version

```go
import "hop.top/vstar/rrule"

rule, err := rrule.ParseRRule("FREQ=WEEKLY;BYDAY=MO,WE,FR;COUNT=6")
// → rule.Freq == rrule.FreqWeekly, rule.Count == 6,
//   rule.ByDay == [{0,MO},{0,WE},{0,FR}]

dtstart := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC) // Monday
cursor  := dtstart
for {
	next, ok, err := rrule.NextOccurrence(rule, dtstart, cursor)
	if err != nil || !ok {
		break // terminated, errored, or hit safety cap
	}
	fmt.Println(next)
	cursor = next
}
```

## Steps

### 1. Parse the RRULE value

`rrule.ParseRRule` takes the property *value* — the bytes after the
`RRULE:` prefix, with no surrounding quotes. The order of rule-parts
is irrelevant per RFC; the parser is strict on case (FREQ, BYDAY,
SECONDLY all uppercase).

```go
rule, err := rrule.ParseRRule("FREQ=WEEKLY;BYDAY=MO,WE,FR;COUNT=6")
if err != nil {
	return err
}
```

`ParseRRule` returns one of three outcomes:

| Outcome | Meaning | Recovery |
|---|---|---|
| `rule, nil` | Success. Defaults applied: `Interval=1`, `WeekStart=MO`. | Use `rule`. |
| `_, vstar.ErrMalformed` | Syntactic error: missing `FREQ`, `INTERVAL=0`, both `UNTIL`+`COUNT`, `BYMONTHDAY=0`, etc. | Refuse the input. The RRULE is invalid per RFC 5545. |
| `_, rrule.ErrUnsupportedRRule` | Syntactically valid but uses a feature outside the RRULE parsing scope (`FREQ=SECONDLY`, `RSCALE`). | Fall back: store the raw string and skip evaluation. The validator emits `VS050` (Warning) for this case. |

Use `errors.Is` to dispatch:

```go
switch {
case errors.Is(err, rrule.ErrUnsupportedRRule):
	// preserve the raw value, skip evaluation
case errors.Is(err, vstar.ErrMalformed):
	// refuse — malformed input
case err != nil:
	return err
}
```

### 2. Step through occurrences

`NextOccurrence(rule, dtstart, after)` returns the first time
strictly after `after` for which `rule` fires. The first occurrence
is `dtstart` itself per RFC 5545 — pass `after = dtstart` to
advance past it.

```go
dtstart := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC) // Monday
cursor  := dtstart
for {
	next, ok, err := rrule.NextOccurrence(rule, dtstart, cursor)
	if err != nil || !ok {
		break
	}
	fmt.Println(next.Format(time.RFC3339))
	cursor = next
}
```

Return semantics:

- `(t, true, nil)` — `t` is the next occurrence.
- `(zero, false, nil)` — the rule has terminated (`UNTIL` passed, `COUNT` exhausted). Reaching the safety iteration bound is reported separately as an error wrapping `rrule.ErrIterationCap` (`MaxIterations` is exported), so a caller can tell a finished series from an unsatisfiable rule.
- `(zero, false, ErrUnsupportedRRule)` — guard against future
  parser/evaluator skew. The RRULE parsing scope fixes parser scope
  == evaluator scope, so this branch is unreachable from valid inputs.

The evaluator operates on `time.Time` and uses its native
arithmetic. Caller's `dtstart` zone carries through — UTC dtstart
in → UTC results out; zoned dtstart in → zoned results out. DST
boundaries (`02:30 doesn't exist on spring-forward day`) are
handled by `time.Time` normalization.

### 3. Apply common patterns

**Every weekday for two weeks:**

```go
rrule.ParseRRule("FREQ=DAILY;BYDAY=MO,TU,WE,TH,FR;COUNT=10")
```

**First Monday of every month:**

```go
rrule.ParseRRule("FREQ=MONTHLY;BYDAY=1MO")
// ByDay=[{Ordinal:1, Weekday:MO}]
```

**Last weekday of every month** (the BYSETPOS workhorse):

```go
rrule.ParseRRule("FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1")
// Expands all weekdays in the month, then keeps the last one.
```

**Every other Tuesday until a deadline:**

```go
rrule.ParseRRule("FREQ=WEEKLY;INTERVAL=2;BYDAY=TU;UNTIL=20261231T235959Z")
```

**Yearly on a specific week of the year** (BYWEEKNO requires
`FREQ=YEARLY` per RFC):

```go
rrule.ParseRRule("FREQ=YEARLY;BYWEEKNO=23;BYDAY=MO")
```

### 4. Validate against the diagnostic catalog

`validate.Validate` automatically runs `rrule.ValidateRRule` on
every `RRULE` property in a Calendar and emits:

- `VS050` (Warning) — RRULE parses but uses a deferred feature.
  The property still round-trips through the codec; only its
  recurrence semantics are inaccessible. Consumers using
  `NextOccurrence` MUST check for `VS050` before relying on the
  evaluator output.
- `VS051` (Error) — RRULE is malformed per RFC 5545 §3.3.10.

See the [diagnostic code catalog](../validate-codes.md) for the
full list and spec cross-links.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `ParseRRule` returns `ErrUnsupportedRRule` for a feature you need | Feature is deferred (`FREQ=SECONDLY`, `RSCALE`). | These are intentionally outside the RRULE parsing scope per [spec/03 §RRULE parsing scope](../../spec/v1.0/03-canonicalization.md#rrule-parsing-scope). Open an issue with your use case to drive a scope review for a later version. |
| `NextOccurrence` returns `(zero, false, nil)` immediately | `cursor` is already past `UNTIL`, OR `Count` exhausted. | Check `rule.Until`, `rule.Count`. Print `dtstart` and confirm `cursor >= dtstart`. |
| `NextOccurrence` returns `ErrIterationCap` | The rule admitted no candidate within `MaxIterations` (100 000) FREQ periods: the BY-* limits are unsatisfiable, or they exclude a stretch longer than the cap covers. Under `FREQ=MINUTELY` at `INTERVAL=1` the cap spans about 69 days, so `FREQ=MINUTELY;BYMONTH=1` evaluated from February reports the cap rather than January. | Check the BY-* combination against the calendar. For a sparse limit under a sub-daily `FREQ`, start the cursor inside the admitted window. |
| BYDAY=MO under FREQ=MONTHLY fires on every Monday, not the first | `BYDAY=MO` (no ordinal) means *every* Monday in the period. | Use `BYDAY=1MO` for the first Monday only. |
| `BYSETPOS` with no other BY-* clause returns `ErrMalformed` | RFC 5545 requires at least one other BY-* clause when BYSETPOS is used. | Add a BY-* clause: `BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1`. |
| `UNTIL=20261231` returns `ErrMalformed` | V* requires UNTIL in RFC 5545 form #2 (UTC, `Z`-suffixed) — see [spec/03 §RRULE parsing scope](../../spec/v1.0/03-canonicalization.md#rrule-parsing-scope). | Use `UNTIL=20261231T235959Z`. |

## How it works

The `rrule` package depends only on the standard library plus
`vstar` root (for `ErrMalformed` and `ParseTime`). It is independent
of every other vstar subpackage — you can import it without pulling
the codec or canonical layers.

`canonical.Calendar` preserves `RRULE` property values byte-for-byte
(spec/03 §"Property values preserved verbatim"). The canonical layer
deliberately does not normalize rule-part order, drop defaulted
parts, or re-format integer lists. Two RRULEs that decompose to the
same `rrule.Rule` but differ in wire shape (e.g.
`FREQ=DAILY;INTERVAL=1` vs `FREQ=DAILY` — semantically identical
because INTERVAL defaults to 1) hash differently. Producers needing
semantic equivalence MUST normalize the wire form before emit.

## Scope boundaries

In scope:

- `FREQ` values: `MINUTELY`, `HOURLY`, `DAILY`, `WEEKLY`, `MONTHLY`,
  `YEARLY`.
- `INTERVAL`, `UNTIL` (form #2 / UTC only), `COUNT`.
- `BYDAY`, `BYMONTH`, `BYMONTHDAY`, `BYHOUR`, `BYMINUTE`, `BYSECOND`.
- `BYYEARDAY`, `BYWEEKNO` (FREQ=YEARLY only per RFC).
- `BYSETPOS` (positional filter; requires another BY-* clause).
- `WKST` (default MO).

Out of scope (returns `ErrUnsupportedRRule`):

- `FREQ=SECONDLY` — extreme expansion, no agentic use case.
- `RSCALE` (RFC 7529, non-Gregorian calendars) — out of V* roadmap
  indefinitely.

The full scope lives in
[spec/03 §RRULE parsing scope](../../spec/v1.0/03-canonicalization.md#rrule-parsing-scope).

## Next steps

- [How to validate and hash a Calendar](how-to-validate-and-hash.md) —
  surface `VS050`/`VS051` from your validation pipeline.
- [Specification §03 — RRULE parsing scope](../../spec/v1.0/03-canonicalization.md#rrule-parsing-scope) —
  every accepted, malformed, and deferred rule-part.
- [Specification §03 — canonicalization](../../spec/v1.0/03-canonicalization.md) —
  why RRULE values are preserved verbatim in canonical form.
