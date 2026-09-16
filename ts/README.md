# @hop-top/vstar

Canonical calendar and contact interchange for agentic systems —
RFC 5545 and RFC 6350, with byte-stable output and a content hash.

[![npm](https://img.shields.io/npm/v/@hop-top/vstar?label=npm)](https://www.npmjs.com/package/@hop-top/vstar)
[![CI](https://img.shields.io/github/actions/workflow/status/hop-top/poly-vstar/ci-ts.yml?branch=main&label=ci)](https://github.com/hop-top/poly-vstar/actions/workflows/ci-ts.yml?query=branch%3Amain)
[![Types](https://img.shields.io/badge/types-included-blue)](https://www.typescriptlang.org/)
[![Spec](https://img.shields.io/badge/spec-draft%20v0.1-blue)](../spec/)
[![License](https://img.shields.io/badge/license-MIT-green)](../LICENSE)

> **Read-only mirror.** This package is developed in the polyglot
> monorepo [`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar)
> under `ts/` and republished on release to
> [`hop-top/vstar-ts`](https://github.com/hop-top/vstar-ts). Open issues
> and pull requests against the monorepo, not the mirror.

## Why

A generic iCalendar library will parse your `.ics` and hand you back a
tree. It will not tell you whether two documents mean the same thing —
because the RFCs let the same logical content be written many ways:
properties in any order, parameters in any order, datetimes in local or
UTC form, folding at any column. Serialize the same calendar twice and
you can get different bytes.

V\* pins that down. It defines a **canonical form** — one byte sequence
per logical content — and an `X-VSTAR-HASH` content hash over it, so
"did this change?" is a string comparison rather than a tree walk. On
top of that it adds the things agentic state actually needs: structural
`diff`, append-only `supersession` for state transitions, a diagnostic
`validate` pass with stable codes, and a bounded `RRULE` evaluator.

Reach for this over a generic iCalendar library when you need any of:

- **Byte-identical output across runtimes.** This port is verified
  byte-for-byte against the Go reference over the whole shared
  conformance corpus — not merely self-consistent. See
  [Conformance](#conformance).
- **Content addressing.** A stable hash over canonical bytes, so
  documents can be deduplicated, cached, or compared across services.
- **Append-only state.** Supersession chains rather than mutation, so
  history survives.

**When not to use this:** if you need a full calendaring client —
timezone database management, free/busy scheduling across attendees,
CalDAV sync, or broad support for RFC 5545's long tail — use a general
iCalendar library. V\* deliberately implements a bounded subset chosen
for machine-generated state, and its `RRULE` scope excludes
`FREQ=SECONDLY`/`MINUTELY` and `RSCALE`. If you simply want to read
someone else's calendar file, this is more machinery than you need.

## Install

```sh
pnpm add @hop-top/vstar
```

Requires **Node 22 or newer**. The package is ESM-only and ships its
own type definitions — no `@types/*` companion.

## Usage

Parse, hash, canonicalize — the most common path:

```ts
import { parse } from "@hop-top/vstar/codec/rfc5545";
import { calendar as canonicalCalendar } from "@hop-top/vstar/canonical";
import { calendar as hashCalendar } from "@hop-top/vstar/hashing";

const ics = [
  "BEGIN:VCALENDAR",
  "VERSION:2.0",
  "PRODID:-//example//EN",
  "BEGIN:VTODO",
  "UID:todo-1",
  "DTSTAMP:20260101T000000Z",
  "DUE:20260102T000000Z",
  "SUMMARY:Ship the port",
  "END:VTODO",
  "END:VCALENDAR",
  "",
].join("\r\n");

const cal = parse(ics);

// "sha256:e551d177…" — stable across producers that spell the same
// instant differently.
console.log(hashCalendar(cal));

// Uint8Array. Canonical form is a BYTE sequence; compare it as one.
const bytes = canonicalCalendar(cal);
```

## API

Each area has its own subpath so an application pulls in only what it
uses. Every subpath is also re-exported from the root under a namespace
of the same name (`import { rrule } from "@hop-top/vstar"`).

| Subpath | Area |
| --- | --- |
| `@hop-top/vstar` | Data model, error sentinels, time helpers, plus every namespace below |
| `@hop-top/vstar/codec/rfc5545` | iCalendar parse and serialize |
| `@hop-top/vstar/codec/rfc6350` | vCard parse and serialize |
| `@hop-top/vstar/codec/stream` | Incremental stream decoding |
| `@hop-top/vstar/canonical` | Canonical byte form |
| `@hop-top/vstar/hashing` | `X-VSTAR-HASH` compute and verify |
| `@hop-top/vstar/validate` | Diagnostics with stable codes |
| `@hop-top/vstar/rrule` | Recurrence parse and bounded expansion |
| `@hop-top/vstar/duration` | ISO 8601 durations and alarm triggers |
| `@hop-top/vstar/ext` | `X-*` extension namespaces |
| `@hop-top/vstar/diff` | Structural diff |
| `@hop-top/vstar/supersession` | Append-only state transitions |
| `@hop-top/vstar/helpers` | Convenience constructors and accessors |

### Validate

Diagnostics carry a stable `code` and a dotted `path`. Match on the
code; the `message` is prose and rewords between versions.

```ts
import { validate } from "@hop-top/vstar/validate";

for (const d of validate(cal)) {
  console.log(d.severity, d.code, d.path);
  // error VS003 VCALENDAR.VTODO[uid=todo-1].X-VSTAR-HASH
}
```

### Recurrence

Expansion is always bounded: `occurrences` takes a limit and reports
whether the series ended within it, so an unbounded rule cannot hang a
caller.

```ts
import { occurrences, parseRRule } from "@hop-top/vstar/rrule";
import { formatTime, parseTime } from "@hop-top/vstar";

const rule = parseRRule("FREQ=DAILY;COUNT=3");
const dtstart = parseTime("20260401T120000Z")!;

const { times, complete } = occurrences(rule, dtstart, 10);
console.log(times.map(formatTime), complete);
// [ '20260401T120000Z', '20260402T120000Z', '20260403T120000Z' ] true
```

### Helpers

Constructors produce components that already carry the required common
properties.

They also maintain the hash: every helper that changes a component
restamps `X-VSTAR-HASH`, so a component stays verifiable across edits.
Reach past them to the properties and it stops verifying — which is the
point.

```ts
import { newTodo, setPriority } from "@hop-top/vstar/helpers";
import { verifyXVstar } from "@hop-top/vstar/hashing";
import { parseTime } from "@hop-top/vstar";

const todo = newTodo("todo-9", parseTime("20260501T090000Z")!);
console.log(verifyXVstar(todo).ok); // true

setPriority(todo, 1);
console.log(verifyXVstar(todo).ok); // true — helper restamped the hash

todo.props.push({ name: "SUMMARY", params: [], value: "tampered" });
console.log(verifyXVstar(todo).ok); // false — raw edit, hash is stale
```

The digest itself is deliberately absent from this example: `newTodo`
stamps `DTSTAMP` from the wall clock, so the hash differs on every run.
Assert on `verifyXVstar(...).ok`, never on a literal digest.

### Errors

Every failure throws a `VstarError` carrying a `code` — the same
sentinel identifier every V\* implementation uses. Dispatch on the
code, not on the class.

```ts
import { VstarError } from "@hop-top/vstar";

try {
  parse(input);
} catch (e) {
  if (e instanceof VstarError && e.code === "ErrMalformed") {
    // …
  }
}
```

## Conformance

This port is a **round-trip** implementation and self-certifies in
[`VSTAR-CONFORMANCE.md`](VSTAR-CONFORMANCE.md).

Its output is checked against the Go reference by the cross-language
parity harness: both emitters run the same corpus and must produce a
byte-identical document, so agreement is proven rather than assumed.

```sh
make test-parity   # from the repository root
```

## Develop

From the repository root:

```sh
make lint-ts test-ts build-ts
```

`src/generated/` is rendered from `spec/registry/` by `make
registry-gen`. Never hand-edit it; `make registry-check` fails on drift.

See [`CONTRIBUTING.md`](../CONTRIBUTING.md) for repo-wide rules and
[`docs/dev/`](../docs/dev/) for the development loop.

## Links

- [Specification](../spec/) — normative text and the conformance corpus
- [Monorepo](https://github.com/hop-top/poly-vstar) — issues and pull requests
- [Porting guide](../docs/dev/porting-guide.md) — writing a sister implementation
- [Diagnostic codes](../docs/validate-codes.md) — the `VS***` catalog

## License

MIT. See [`LICENSE`](../LICENSE) at the repository root.
