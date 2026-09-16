# Validate and hash a Calendar

Compute `X-VSTAR-HASH`, run `validate.Validate`, and interpret the
resulting diagnostic codes.

## Use this when

- You build a `Calendar` programmatically and need to seal it with
  `X-VSTAR-HASH` before persisting or transmitting it.
- You receive a Calendar from another system and want to confirm
  semantic conformance (`UID`/`DTSTAMP`/`X-VSTAR-HASH` present, hash
  intact, supersession discipline obeyed).
- You hit a `VS0xx` diagnostic in your tests and need to know what
  it means.

## Result

After completing this guide, you will:

- Have a Calendar whose every component carries a correct
  `X-VSTAR-HASH`.
- Run `validate.Validate` and read back zero (or correctly-handled)
  diagnostics.
- Map each diagnostic code to the corresponding spec rule.

## Before you begin

You need:

- A `vstar.Calendar` value — either parsed via
  [`codec/rfc5545.Parse`](quickstart.md) or built up with
  `helpers.NewCalendar` + `helpers.NewTodo`/`NewEvent`/etc.
- The Go reference implementation imported.

## Quick version

```go
import (
	"hop.top/vstar/hashing"
	"hop.top/vstar/validate"
)

// 1. Stamp every component with X-VSTAR-HASH.
for i := range cal.Components {
	hashing.SetXVSTAR(&cal.Components[i])
}

// 2. Run validation; an empty slice means clean.
diags := validate.Validate(cal)
for _, d := range diags {
	log.Printf("[%s] %s %s — %s", d.Severity, d.Code, d.Path, d.Message)
}
```

## Steps

### 1. Stamp X-VSTAR-HASH on every component

Every V* component MUST carry `X-VSTAR-HASH` (spec/05 §1, code
`VS003`). `hashing.SetXVSTAR` computes the canonical hash and writes
it to the component, replacing any existing value. The function
mutates its argument — pass a pointer.

```go
for i := range cal.Components {
	hashing.SetXVSTAR(&cal.Components[i])
}
```

`SetXVSTAR` is idempotent: calling it twice produces the same value
(the hash is computed over the X-VSTAR-HASH-stripped canonical
bytes, so the stored hash never feeds back into its own digest).

If you need to verify a stored hash without rewriting it, use
`hashing.VerifyXVSTAR`:

```go
ok, want, got := hashing.VerifyXVSTAR(comp)
if !ok {
	log.Printf("hash mismatch on %s: want %s got %s", comp.UID(), want, got)
}
```

### 2. Run validate.Validate

`validate.Validate` returns a flat `[]Diagnostic` covering every
component in the Calendar. An empty slice means the Calendar is
conformant.

```go
diags := validate.Validate(cal)
if len(diags) == 0 {
	// clean
}
```

For a single component (no parent Calendar context), use
`validate.ValidateComponent`. It runs the same per-component rules
but skips cross-component checks like `VS031` (supersession orphan
resolution) that need a ledger.

### 3. Interpret diagnostic codes

Each `Diagnostic` carries:

- `Severity` — `SeverityError` (MUST violation, document is not
  V* conformant) or `SeverityWarning` (SHOULD violation or
  stylistic concern).
- `Code` — stable identifier (e.g. `"VS001"`). Match programmatically.
- `Message` — human-readable detail. May change across minor
  versions. Do not match.
- `Path` — dotted locator (e.g. `VCALENDAR.VTODO[uid=foo].DTSTAMP`).

The full code catalog with severity, rule, and spec cross-link lives
at [`docs/validate-codes.md`](../validate-codes.md). Common cases:

| Code | Severity | Meaning |
|---|---|---|
| `VS001` | Error | `UID` missing on a component. |
| `VS002` | Error | `DTSTAMP` missing on a component. |
| `VS003` | Error | `X-VSTAR-HASH` missing — call `hashing.SetXVSTAR`. |
| `VS010` | Error | `X-VSTAR-HASH` present but does not match recomputed hash. |
| `VS020` | Warning | Property name is non-standard and lacks the `X-` prefix. |
| `VS040`–`VS043` | Error | Type-specific required properties missing (`VTODO` needs `DUE`, `VEVENT` needs `DTSTART`, etc.). |
| `VS050` | Warning | `RRULE` parses but uses a feature outside the RRULE parsing scope. |
| `VS051` | Error | `RRULE` is malformed per RFC 5545 §3.3.10. |

### 4. Decide your error policy

Errors block conformance — refuse the Calendar or fix the cause.
Warnings are advisory: `VS020` may be expected if your system uses
a non-`X-` extension namespace by design (and you accept the
deviation in your `VSTAR-CONFORMANCE.md` per spec/05).

A typical CI gate:

```go
for _, d := range diags {
	if d.Severity == validate.SeverityError {
		t.Fatalf("validate: %s %s — %s", d.Code, d.Path, d.Message)
	}
	if d.Severity == validate.SeverityWarning {
		t.Logf("validate warning: %s %s — %s", d.Code, d.Path, d.Message)
	}
}
```

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `VS003` after `hashing.SetXVSTAR` | You called `SetXVSTAR` on a copy — the function mutates a pointer. | Iterate by index: `for i := range cal.Components { hashing.SetXVSTAR(&cal.Components[i]) }`. |
| `VS010` on round-trip from disk | Source bytes were edited after the hash was set; or the producer used a different canonical-form discipline. | Recompute with `hashing.Calendar` and compare; investigate whether the producer follows spec/03 (especially NFC text and TZID resolution). |
| `VS031` never fires from `ValidateComponent` | `ValidateComponent` has no parent Calendar to resolve `RELATED-TO` against. | Use `validate.Validate(cal)` instead; supersession resolution is Calendar-level. |
| Hash mismatches between Go and a sister implementation | Canonical bytes diverge before the hash. | Compare `canonical.Calendar(cal)` byte-for-byte; see [How to build a sister implementation](how-to-implement-vstar.md). |

## How it works

Validation operates on the parsed `Component` graph; it does not
re-parse the wire form. Codes catalog stable rules — once a code
ships in a release, its meaning never changes (renaming is a major
version event). New codes can appear in any release; consumers must
tolerate unknown codes (log them, do not crash).

`hashing.Calendar` and `hashing.Component` strip any existing
`X-VSTAR-HASH` from their input before computing the canonical
bytes, so the stored hash never recursively contributes to its own
digest. The canonical layer (`canonical.Calendar`) also strips
`X-VSTAR-HASH` per its own contract — defense in depth.

## Next steps

- [Diagnostic code catalog](../validate-codes.md) — full per-code
  rule and spec linkage.
- [How to parse and evaluate RRULE](how-to-recurrence.md) — the
  next surface area where `VS050`/`VS051` come into play.
- [Specification §05 — conformance](../../spec/v1.0/05-conformance.md) —
  what "V* conformant" means in normative text.
