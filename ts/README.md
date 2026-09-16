# V\*

**`@hop-top/vstar`** on npm — canonical calendar and contact
interchange for agentic systems: RFC 5545 and RFC 6350, with
byte-stable output and a content hash.

[![npm](https://img.shields.io/npm/v/@hop-top/vstar?label=npm)](https://www.npmjs.com/package/@hop-top/vstar)
[![CI](https://img.shields.io/github/actions/workflow/status/hop-top/poly-vstar/ci-ts.yml?branch=main&label=ci)](https://github.com/hop-top/poly-vstar/actions/workflows/ci-ts.yml?query=branch%3Amain)
[![Types](https://img.shields.io/badge/types-included-blue)](https://www.typescriptlang.org/)
[![Spec](https://img.shields.io/badge/spec-draft%20v0.1-blue)](https://github.com/hop-top/poly-vstar/tree/main/spec)
[![License](https://img.shields.io/badge/license-MIT-green)](https://github.com/hop-top/poly-vstar/blob/main/LICENSE)

> **Read-only mirror.** This package is developed in the polyglot
> monorepo [`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar)
> under `ts/` and republished to
> [`hop-top/vstar-ts`](https://github.com/hop-top/vstar-ts) on each
> `vstar-ts/v*` release tag. Open issues and pull requests against the
> monorepo, not the mirror.

## What V\* is

V\* (pronounced "vee-star") represents agentic-system state — worlds,
missions, players, turns, observations, decisions — as **iCalendar
(RFC 5545) and vCard (RFC 6350) components**. Agent work that already
has time, identity and sequence semantics rides existing calendar,
scheduler and contact tooling instead of a bespoke protocol. This
package is the TypeScript implementation; the Go reference and the
Python, Rust and PHP ports produce the same bytes for the same input.

A generic iCalendar library parses an `.ics` and hands back a tree. It
cannot tell you whether two documents mean the same thing, because the
RFCs let one logical content be written many ways: properties in any
order, parameters in any order, datetimes in local or UTC form, folding
at any column. V\* pins that down with a **canonical form** — one byte
sequence per logical content — and an `X-VSTAR-HASH` over it, so "did
this change?" is `a === b` on two strings rather than a deep-equal over
two trees. On top it adds what agent state needs and a calendar library
does not carry: a `validate` pass with stable diagnostic codes, a
bounded `rrule` evaluator, structural `diff`, and append-only
`supersession` for state transitions.

## Use this when

- You emit agent state — todos, journals, events, contacts — and want
  it to interoperate with calendars, schedulers or contact directories
  with no custom serialization.
- Consumers in other runtimes must agree with you byte-for-byte, and
  you want a hash that proves they did. This port is verified against
  the Go reference over the whole conformance corpus — not merely
  self-consistent. The quick start below prints the same
  `sha256:e551d177…` every implementation prints for that input. See
  [Conformance](#conformance).
- You need content addressing — deduplicate, cache or compare documents
  across services by a stable hash — or append-only history, where a
  status change is a new record that points at the old one rather than
  a mutation.

In a TypeScript project specifically: it is ESM-only with one subpath
export per area, so a bundler pulls in only the areas you import; it
has one runtime dependency (`@noble/hashes`); bytes are `Uint8Array`
end to end, never a decoded string; and it ships its own type
definitions.

## Skip this if

You need a full calendaring client — timezone database management,
free/busy scheduling across attendees, CalDAV sync, or broad support
for RFC 5545's long tail. Use a general iCalendar library such as
`ical.js`. V\* deliberately implements a bounded subset chosen for
machine-generated state: its `RRULE` scope excludes `FREQ=SECONDLY`
and `RSCALE`, and its vCard codec accepts
version 4.0 only. If you simply want to read someone else's calendar
file, this is more machinery than you need.

## Install

```sh
pnpm add @hop-top/vstar
```

```sh
npm install @hop-top/vstar
```

Requires **Node 22 or newer**. The package is ESM-only — there is no
CommonJS build, so `require()` will not load it — and it ships its own
type definitions, with no `@types/*` companion. Versions are
prereleases (`1.0.0-alpha.*`); pin an exact version (`--save-exact`)
until 1.0.0.

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

// Stable across producers that spell the same instant differently.
console.log(hashCalendar(cal));

// Canonical form is a BYTE sequence; compare it as one.
const bytes = canonicalCalendar(cal);
console.log(bytes.constructor.name, bytes.byteLength);
```

```text
sha256:e551d17793785d5876edc6e33bca47f2aae73eb9feac49678d76159a78c91b18
Uint8Array 175
```

That hash is reproducible because `DTSTAMP` is a fixed literal, and it
is the same string every other V\* implementation prints for this
input. The hash is the cross-language contract.

## API

Each area has its own subpath so an application pulls in only what it
uses. Every subpath is also re-exported from the root under a namespace
of the same name (`import { rrule } from "@hop-top/vstar"`).

| Subpath | Area |
| --- | --- |
| `@hop-top/vstar` | Data model, `VstarError`, time helpers, plus every namespace below |
| `@hop-top/vstar/codec/rfc5545` | iCalendar parse and encode |
| `@hop-top/vstar/codec/rfc6350` | vCard parse and encode |
| `@hop-top/vstar/codec/stream` | Constant-memory stream parse and encode |
| `@hop-top/vstar/canonical` | Canonical byte form |
| `@hop-top/vstar/hashing` | `X-VSTAR-HASH` compute and verify |
| `@hop-top/vstar/validate` | Diagnostics with stable codes |
| `@hop-top/vstar/rrule` | Recurrence parse and bounded expansion |
| `@hop-top/vstar/duration` | ISO 8601 durations and alarm triggers |
| `@hop-top/vstar/ext` | `X-*` extension namespaces |
| `@hop-top/vstar/diff` | Structural diff |
| `@hop-top/vstar/supersession` | Append-only state transitions |
| `@hop-top/vstar/helpers` | Convenience constructors and accessors |

Two representation choices to know before you read further: parsers
accept `string | Uint8Array` and encoders return `Uint8Array`; and an
absolute instant (`Instant`) is a millisecond `number`, not a `Date` —
compare with `===`, format with `formatTime`. The reasons are recorded
in [`VSTAR-CONFORMANCE.md`](VSTAR-CONFORMANCE.md#known-deviations).

### Validate

Diagnostics carry a stable `code` and a dotted `path`. Match on the
code; the `message` is prose and rewords between versions.

```ts
import { validate } from "@hop-top/vstar/validate";

// `cal` is the quick-start calendar above: no X-VSTAR-HASH yet.
for (const d of validate(cal)) {
  console.log(d.severity, d.code, d.path);
}
```

```text
error VS003 VCALENDAR.VTODO[uid=todo-1].X-VSTAR-HASH
```

`severity` is the string-literal type `"error" | "warning"` and `code`
is the `Code` union, both taken from the generated registry module —
so `d.severity === "error"` narrows with no enum to import, and a typo
in a code is a compile error. The catalog is
[`docs/validate-codes.md`](https://github.com/hop-top/poly-vstar/blob/main/docs/validate-codes.md).

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
```

```text
[ '20260401T120000Z', '20260402T120000Z', '20260403T120000Z' ] true
```

### Helpers

Constructors produce components that already carry the required common
properties — `UID`, a `DTSTAMP` of now, and an `X-VSTAR-HASH` over the
result.

They also maintain the hash: every helper that changes a component
restamps `X-VSTAR-HASH`, so a component stays verifiable across edits.
Reach past them to the properties and it stops verifying — which is the
point.

```ts
import { newTodo, setPriority } from "@hop-top/vstar/helpers";
import { verifyXVstar } from "@hop-top/vstar/hashing";
import { parseTime } from "@hop-top/vstar";

const todo = newTodo("todo-9", parseTime("20260501T090000Z")!);
console.log(verifyXVstar(todo).ok);

setPriority(todo, 1);
console.log(verifyXVstar(todo).ok); // helper restamped the hash

todo.props.push({ name: "SUMMARY", params: [], value: "tampered" });
console.log(verifyXVstar(todo).ok); // raw edit, hash is stale
```

```text
true
true
false
```

The digest itself is deliberately absent from this example: `newTodo`
stamps `DTSTAMP` from the wall clock, so the hash differs on every run.
Assert on `verifyXVstar(...).ok`, never on a literal digest. To seal a
component you edited by hand, call `setXVstar` from
`@hop-top/vstar/hashing`.

### Streaming

The batch codecs load a whole `Calendar` into memory. The stream
parsers yield one top-level component at a time, so a large ledger is
processed in constant memory.

Hand them **bytes**. Canonical folding cuts at a 75-octet boundary and
will split a multi-byte UTF-8 sequence across the fold; the parser
unfolds over bytes and decodes only once a logical line is whole. If you
decode first, `TextDecoder` has already replaced each half of the split
sequence with U+FFFD, and no amount of unfolding gets the character
back. The file below is
[`fold_split_utf8.canonical`](https://github.com/hop-top/poly-vstar/blob/main/spec/v0.1/conformance/rfc5545/fold_split_utf8.canonical)
from the conformance corpus.

```ts
import { readFileSync } from "node:fs";
import { newVCalendarParser } from "@hop-top/vstar/codec/stream";

const bytes = readFileSync("fold_split_utf8.canonical"); // a Uint8Array

const parser = newVCalendarParser(bytes);
console.log(parser.header().prodId);
for (const c of parser) {
  const summary = c.props.find((p) => p.name === "SUMMARY")?.value;
  console.log(c.type, summary);
}

// Decoding first loses the split sequence before the parser sees it.
for (const c of newVCalendarParser(new TextDecoder().decode(bytes))) {
  console.log(c.type, c.props.find((p) => p.name === "SUMMARY")?.value);
}
```

```text
-//V*//conformance//EN
VEVENT aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaéééééééé
VEVENT aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaééééé��éé
```

The input type is `LineSource = string | Uint8Array | Iterable<string |
Uint8Array>`, so a generator that reads a file chunk by chunk keeps the
whole pipeline incremental; `byteLines` and `byteLinesFromAsyncIterable`
in the same subpath adapt a byte source into physical lines. The
`string` form exists for input you know is unfolded.

### Errors

Every failure throws a `VstarError` carrying a `code` — the same
sentinel identifier every V\* implementation uses. Dispatch on the
code, not on the class: there is deliberately no subclass per sentinel,
because `instanceof` breaks when a bundler duplicates a module and a
string comparison does not.

```ts
import { VstarError } from "@hop-top/vstar";
import { parse } from "@hop-top/vstar/codec/rfc5545";

try {
  parse("not a calendar\r\n");
} catch (e) {
  if (e instanceof VstarError && e.code === "ErrMalformed") {
    console.log(e.code, "-", e.message);
  }
}
```

```text
ErrMalformed - ErrMalformed: expected BEGIN:VCALENDAR, got "not a calendar"
```

`code` is the closed union `VstarErrorCode`, so a misspelled sentinel
in a comparison is a compile error.

## Conformance

This port is a **round-trip** implementation and self-certifies in
[`VSTAR-CONFORMANCE.md`](VSTAR-CONFORMANCE.md): spec revision, the
conformance criteria, the corpus gates, and the deviations from the
Go reference's surface.

Its output is checked against the Go reference by the cross-language
parity harness: every implementation runs the same corpus and must
produce a byte-identical document, so agreement is proven rather than
assumed. From a checkout of the monorepo:

```sh
make test-parity
```

## Develop

From the monorepo root:

```sh
make lint-ts test-ts build-ts
```

Or inside `ts/`: `pnpm install --ignore-scripts`, then `pnpm lint`,
`pnpm test`, `pnpm build`.

`src/generated/` is rendered from `spec/registry/` by `make
registry-gen`. Never hand-edit it; `make registry-check` fails on drift.

See [`CONTRIBUTING.md`](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md)
for repo-wide rules and
[`docs/dev/`](https://github.com/hop-top/poly-vstar/blob/main/docs/INDEX.md#for-developers)
for the development loop.

## Links

- [Specification](https://github.com/hop-top/poly-vstar/tree/main/spec) — normative text and the conformance corpus
- [Monorepo](https://github.com/hop-top/poly-vstar) — issues and pull requests
- [API mapping](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/api-mapping.md) — the Go surface and this port's spelling of it
- [Porting guide](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/porting-guide.md) — writing a sister implementation
- [Diagnostic codes](https://github.com/hop-top/poly-vstar/blob/main/docs/validate-codes.md) — the `VS***` catalog

## License

MIT. See [`LICENSE`](https://github.com/hop-top/poly-vstar/blob/main/LICENSE)
at the monorepo root.
