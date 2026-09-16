# hop.top/vstar

Canonical calendar and contact interchange for agentic systems —
RFC 5545 and RFC 6350, with byte-stable output and a content hash.
This module is the V\* reference implementation: the bytes every other
implementation is measured against.

[![Release](https://img.shields.io/github/v/tag/hop-top/poly-vstar?filter=vstar/*&label=release&color=00ADD8&sort=semver)](https://github.com/hop-top/poly-vstar/releases)
[![CI (Go)](https://img.shields.io/github/actions/workflow/status/hop-top/poly-vstar/ci-go.yml?branch=main&label=ci)](https://github.com/hop-top/poly-vstar/actions/workflows/ci-go.yml?query=branch%3Amain)
[![Go Reference](https://pkg.go.dev/badge/hop.top/vstar.svg)](https://pkg.go.dev/hop.top/vstar)
[![Go](https://img.shields.io/badge/go-1.25-00ADD8)](https://github.com/hop-top/poly-vstar/blob/main/go/go.mod)
[![Spec](https://img.shields.io/badge/spec-draft%20v0.1-blue)](https://github.com/hop-top/poly-vstar/tree/main/spec)
[![License](https://img.shields.io/badge/license-MIT-green)](https://github.com/hop-top/poly-vstar/blob/main/LICENSE)

> **Read-only mirror.** This module is developed in the polyglot
> monorepo [`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar)
> under `go/` and republished on each `vstar/v*` tag to
> [`hop-top/vstar`](https://github.com/hop-top/vstar), which is where
> `go get hop.top/vstar` resolves. The mirror is created by the first
> release. Open issues and pull requests against the monorepo, not the
> mirror.

## Why

Go has iCalendar parsers. What they return is a tree, and a tree
cannot answer the question agent state keeps asking: *is this the
same thing I stored?* RFC 5545 lets one logical content be written
many ways — properties in any order, parameters in any order,
datetimes in local or UTC form, folding at any column — so two
`[]byte` that mean the same thing rarely satisfy `bytes.Equal`, and
`reflect.DeepEqual` on two parsed trees fails in the other direction,
calling a reordering a change.

V\* fixes the equivalence class to one representative. A **canonical
form** — one byte sequence per logical content — and an
`X-VSTAR-HASH` computed over it turn "did this change?" into `a == b`
on two strings, and "is this still the record I hashed?" into one
call, `hashing.VerifyXVSTAR`. Around that core the module carries
what machine-generated state needs and a calendar library does not:

- **`validate`** reports diagnostics with codes that are stable across
  minor versions, so a caller switches on `validate.CodeBadXVSTARHash`
  rather than matching prose.
- **`rrule`** evaluates recurrence under a published iteration bound:
  a rule that never terminates returns `ErrIterationCap` instead of
  hanging the goroutine.
- **`supersession`** records a status change as a new `VJOURNAL` that
  points at the old component instead of mutating it, so the ledger —
  and every hash that referenced it — stays valid.
- **`diff`** compares through canonical form, so a reorder is not a
  change and a changed value is located by path.

Reach for it when you emit agent state — todos, journals, events,
contacts — and want it to ride existing calendar and contact tooling
with no bespoke wire format; when the same document must hash
identically on every machine in every year (datetimes resolve only
against the document's own `VTIMEZONE`, never the host's tz
database); or when consumers in other languages must agree with you
byte-for-byte — the TypeScript, Python, Rust and PHP ports reproduce
this module's output over the whole conformance corpus, so a hash
computed here verifies there.

**When not to use this:** if you need a full calendaring client —
timezone database management, free/busy scheduling across attendees,
CalDAV sync, or broad support for RFC 5545's long tail — use a
general iCalendar library. V\* deliberately implements a bounded
subset chosen for machine-generated state: its `RRULE` scope excludes
`FREQ=SECONDLY`/`MINUTELY` and `RSCALE`, and its vCard codec accepts
version 4.0 only. If you simply want to read someone else's calendar
file, this is more machinery than you need.

## Install

```sh
go get hop.top/vstar
```

Requires **Go 1.25 or newer**. The module depends on the standard
library and one external module, `golang.org/x/text` (NFC
normalization in the canonical form); it takes no other dependency.

## Usage

Parse, hash, canonicalize — the most common path:

```go
package main

import (
	"fmt"
	"strings"

	"hop.top/vstar/canonical"
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/hashing"
)

func main() {
	ics := strings.Join([]string{
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
	}, "\r\n")

	cal, err := rfc5545.Parse(strings.NewReader(ics))
	if err != nil {
		panic(err)
	}

	// Stable across producers that spell the same instant differently.
	fmt.Println(hashing.Calendar(cal))

	// Canonical form is a BYTE sequence; compare it with bytes.Equal.
	fmt.Println(len(canonical.Calendar(cal)), "bytes")
}
```

```text
sha256:e551d17793785d5876edc6e33bca47f2aae73eb9feac49678d76159a78c91b18
175 bytes
```

That hash is the string the TypeScript, Python, Rust and PHP ports
print for the same input — this module is where it comes from.

## API

Each area is its own package, so an application imports only what it
uses. `hop.top/vstar` itself is the data model and nothing else: no
I/O, no import cycles.

| Package | Area |
| --- | --- |
| `hop.top/vstar` | Data model (`Calendar`, `Card`, `Component`, `Property`, `Param`), wire-string enums, error sentinels, `ParseTime`/`FormatTime` |
| `hop.top/vstar/codec/rfc5545` | iCalendar parse and encode |
| `hop.top/vstar/codec/rfc6350` | vCard 4.0 parse and encode |
| `hop.top/vstar/codec/stream` | Incremental parse and encode, for ledgers too large to hold at once |
| `hop.top/vstar/canonical` | Canonical byte form |
| `hop.top/vstar/hashing` | `X-VSTAR-HASH` compute, stamp and verify |
| `hop.top/vstar/validate` | Diagnostics with stable codes |
| `hop.top/vstar/rrule` | Recurrence parse, bounded expansion, `RDATE`/`EXDATE` sets |
| `hop.top/vstar/duration` | RFC 5545 durations and alarm triggers |
| `hop.top/vstar/ext` | `X-*` extension namespaces |
| `hop.top/vstar/diff` | Semantic equality and structural diff |
| `hop.top/vstar/supersession` | Append-only status transitions |
| `hop.top/vstar/helpers` | Constructors and accessors that keep the hash fresh |

Reference: `go doc hop.top/vstar/<package>` or
[pkg.go.dev/hop.top/vstar](https://pkg.go.dev/hop.top/vstar).

### Validate

Diagnostics carry a stable `Code` and a dotted `Path`. Match on the
generated constant; `Message` is prose and rewords between versions.

```go
	// The VTODO from Usage, stamped with a hash that is not its hash.
	cal.Components[0].Set(vstar.Property{
		Name:  hashing.XVSTARHashProperty,
		Value: "sha256:" + strings.Repeat("0", 64),
	})

	for _, d := range validate.Validate(cal) {
		fmt.Println(d.Severity, d.Code, d.Path)
		if d.Code == validate.CodeBadXVSTARHash {
			// dispatch on the constant, never on the message
		}
	}
```

```text
error VS010 VCALENDAR.VTODO[uid=todo-1].X-VSTAR-HASH
```

`Validate` never returns an error: validation *is* the error channel,
and an empty slice means clean.

### Recurrence

Expansion is always bounded. `Occurrences` takes a limit and reports
whether the series ended within it, so an unbounded rule cannot hang
a caller; `All` is the lazy `iter.Seq` for when the consumer decides
where to stop.

```go
	rule, err := rrule.ParseRRule("FREQ=DAILY;COUNT=3")
	if err != nil {
		panic(err)
	}
	dtstart, _ := vstar.ParseTime("20260401T120000Z")

	times, complete, err := rrule.Occurrences(rule, dtstart, 10)
	if err != nil {
		panic(err)
	}
	for _, t := range times {
		fmt.Println(vstar.FormatTime(t))
	}
	fmt.Println("complete:", complete)
```

```text
20260401T120000Z
20260402T120000Z
20260403T120000Z
complete: true
```

### Helpers and tamper detection

Constructors return components that already carry the required
common properties — `UID`, a `DTSTAMP` of now, and an `X-VSTAR-HASH`
over the result. Every helper that mutates a component restamps the
hash last; reach past them to `Component.Set` and the stamp goes
stale, which is the point.

```go
	due, _ := vstar.ParseTime("20260501T090000Z")
	todo, err := helpers.NewTodo("todo-9", due)
	if err != nil {
		panic(err)
	}
	ok, _, _ := hashing.VerifyXVSTAR(todo)
	fmt.Println("fresh:", ok)

	helpers.SetPriority(&todo, 1)
	ok, _, _ = hashing.VerifyXVSTAR(todo)
	fmt.Println("after helper:", ok)

	todo.Set(vstar.Property{Name: "SUMMARY", Value: "edited by hand"})
	ok, _, _ = hashing.VerifyXVSTAR(todo)
	fmt.Println("after raw Set:", ok)
```

```text
fresh: true
after helper: true
after raw Set: false
```

The digest itself is deliberately absent from this example: `NewTodo`
stamps `DTSTAMP` from the wall clock, so it differs on every run.
Assert on `VerifyXVSTAR`'s `ok`, never on a literal hash. When you
need to say *what* differed, the other two returns are `want` and
`got`.

### Supersession

A status change is a new `VJOURNAL` that names its target, not an edit
to the target. `Supersedes` refuses a target whose stored hash no
longer matches its content (`ErrTargetCorrupted`); `Superseded` reads
the latest effective status back out of a ledger.

```go
	at, _ := vstar.ParseTime("20260502T100000Z")
	entry, err := supersession.Supersedes(todo, "COMPLETED", at)
	if err != nil {
		panic(err)
	}
	fmt.Println(entry.UID())

	ledger := []vstar.Component{todo, entry}
	status, ok := supersession.Superseded(todo, ledger)
	fmt.Println(status, ok)
```

```text
journal:status:todo-9:20260502T100000Z
COMPLETED true
```

### Errors

Every failure wraps one of twelve sentinels with positional context.
Dispatch with `errors.Is`; the sentinel's identifier (`ErrMalformed`)
is the same token every V\* implementation reports for that class,
and the conformance corpus asserts it by name.

```go
	_, err := rfc5545.Parse(strings.NewReader("BEGIN:VCALENDAR\r\nnot a content line\r\n"))
	fmt.Println(errors.Is(err, vstar.ErrMalformed))
	fmt.Println(err)
```

```text
true
rfc5545: missing colon in content line "not a content line": malformed
```

The twelve, by package: `vstar.ErrMalformed`, `ErrUnclosedBlock`,
`ErrUnsupportedVersion`, `ErrMissingUID`; `rrule.ErrUnsupportedRRule`,
`ErrIterationCap`, `ErrUnboundedExpansion`;
`supersession.ErrTargetCorrupted`; `stream.ErrAlreadyClosed`,
`ErrHeaderLocked`; `duration.ErrNoTrigger`, `ErrNoAnchor`.

## The reference implementation

The four ports each publish a `VSTAR-CONFORMANCE.md` attesting which
gates are green. This module has none, because it does not attest to
conformance — it defines it. An implementation is conformant when it
meets every MUST in
[spec/05](https://github.com/hop-top/poly-vstar/blob/main/spec/v0.1/05-conformance.md)
*and* reproduces this module's canonical bytes and hashes over the
whole corpus. The cross-language parity harness diffs every port's
output against this one's on every push; a value wrong here is
corrected here and every port regenerated, never the other way round.

```sh
make test-parity   # from the repository root
```

`testdata/` is a generated, committed mirror of the corpus authored
at
[`spec/v0.1/conformance/`](https://github.com/hop-top/poly-vstar/tree/main/spec/v0.1/conformance),
so the published module is self-contained: every test reads its
fixtures from there, and `go test ./...` passes without the monorepo
present. Nobody hand-edits it; `cmd/fixtures-verify` regenerates
every `.canonical` and `.hash` sibling and fails on drift.

## Develop

From the repository root:

```sh
make ci-go   # vet + gofumpt check + golangci-lint + tests with -race and coverage
```

Or, inside `go/`, plain `go test ./...`.

`validate/codes_gen.go` is rendered from `spec/registry/` by `make
registry-gen`. Never hand-edit it; `make registry-check` fails on
drift. The same rule covers `testdata/` and `make fixtures-verify`.

See
[`CONTRIBUTING.md`](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md)
for repo-wide rules and
[`docs/dev/setup.md`](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/setup.md)
for the development loop.

## Links

- [Quickstart](https://github.com/hop-top/poly-vstar/blob/main/docs/user/quickstart.md) — parse your first VCALENDAR, then the [how-tos](https://github.com/hop-top/poly-vstar/blob/main/docs/INDEX.md) for validation, hashing and recurrence
- [Specification](https://github.com/hop-top/poly-vstar/tree/main/spec) — normative text and the conformance corpus; [spec/03](https://github.com/hop-top/poly-vstar/blob/main/spec/v0.1/03-canonicalization.md) is every canonical-form rule
- [Diagnostic codes](https://github.com/hop-top/poly-vstar/blob/main/docs/validate-codes.md) — the `VS***` catalog
- [API mapping](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/api-mapping.md) — this module's exported surface and each port's spelling of it
- [Porting guide](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/porting-guide.md) — writing a sister implementation
- [Monorepo](https://github.com/hop-top/poly-vstar) — issues and pull requests

## License

MIT. See
[`LICENSE`](https://github.com/hop-top/poly-vstar/blob/main/LICENSE)
at the monorepo root. The conformance corpus mirrored into
`testdata/` is MIT as well; the spec prose it pins is CC-BY-4.0.
