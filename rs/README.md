# V\*

**`hop-top-vstar`** on crates.io — canonical calendar and contact
interchange for agentic systems: RFC 5545 and RFC 6350, with
byte-stable output and a content hash.
The Rust port of the V\* reference implementation.

[![crates.io](https://img.shields.io/crates/v/hop-top-vstar?label=crates.io)](https://crates.io/crates/hop-top-vstar)
[![docs.rs](https://img.shields.io/docsrs/hop-top-vstar?label=docs.rs)](https://docs.rs/hop-top-vstar)
[![CI](https://img.shields.io/github/actions/workflow/status/hop-top/poly-vstar/ci-rs.yml?branch=main&label=ci)](https://github.com/hop-top/poly-vstar/actions/workflows/ci-rs.yml?query=branch%3Amain)
[![MSRV](https://img.shields.io/badge/msrv-1.98-blue)](https://releases.rs/)
[![Spec](https://img.shields.io/badge/spec-draft%20v0.1-blue)](https://github.com/hop-top/poly-vstar/tree/main/spec)
[![License](https://img.shields.io/badge/license-MIT-green)](https://github.com/hop-top/poly-vstar/blob/main/LICENSE)

> **Where this crate lives.** It is developed under `rs/` in the
> polyglot monorepo
> [`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar) and
> republished on each `vstar-rs/v*` tag to the read-only mirror
> [`hop-top/vstar-rs`](https://github.com/hop-top/vstar-rs). The first
> release creates that mirror, so it may not exist yet. Open issues and
> pull requests against the monorepo.

## What V\* is

V\* (pronounced "vee-star") represents agentic-system state — worlds,
missions, players, turns, observations, decisions — as **iCalendar
(RFC 5545) and vCard (RFC 6350) components**. Agent work that already
has time, identity and sequence semantics rides existing calendar,
scheduler and contact tooling instead of a bespoke protocol. This
crate is the Rust implementation; the Go reference and the
TypeScript, Python and PHP ports produce the same bytes for the same
input.

A generic iCalendar crate parses an `.ics` and hands back a tree. It
cannot tell you whether two documents mean the same thing, because the
RFCs let one logical content be written many ways: properties in any
order, parameters in any order, datetimes in local or UTC form, folding
at any column. V\* pins that down with a **canonical form** — one byte
sequence per logical content — and an `X-VSTAR-HASH` over it, so "did
this change?" is a byte comparison rather than a tree walk. On top it
adds what agent state needs and a calendar library does not carry: a
`validate` pass with stable diagnostic codes, a bounded `rrule`
evaluator, structural `diff`, and append-only `supersession` for state
transitions.

## Use this when

- You emit agent state — todos, journals, events, contacts — and want
  it to interoperate with calendars, schedulers or contact directories
  with no custom serialization.
- Consumers in other runtimes must agree with you byte-for-byte, and
  you want a hash that proves they did. This port is checked against
  the Go reference over the whole conformance corpus — agreement is
  measured, not assumed. The quick start below prints the same
  `sha256:e551d177…` every implementation prints for that input. See
  [Conformance](#conformance).
- You need content addressing — deduplicate, cache or compare documents
  across services by a stable hash — or append-only history, where a
  status change is a new record that points at the old one rather than
  a mutation.

In a Rust project specifically: `#![forbid(unsafe_code)]`; three
direct dependencies and nothing else — `sha2`, `unicode-normalization`,
and `chrono` built without its `clock` feature — so no `serde`, no
async runtime, and no IANA timezone database: a `TZID` resolves only
against the `VTIMEZONE` in the document being parsed, which is what
lets the bytes come out the same on any machine. Folding and unfolding
are done on octets before any UTF-8 decoding, so a multibyte character
split across a fold survives intact. Every failure is one
`#[non_exhaustive]` `Error` enum whose variants carry positional
context.

## Skip this if

You need a full calendaring client — timezone database management,
free/busy scheduling across attendees, CalDAV sync, or broad support
for RFC 5545's long tail. Use a general iCalendar crate. V\*
deliberately implements a bounded subset chosen for machine-generated
state: its `RRULE` scope excludes `FREQ=SECONDLY` and `RSCALE`, and
its vCard codec accepts version 4.0 only. If you simply
want to read someone else's calendar file, this is more machinery than
you need.

## Install

```sh
cargo add hop-top-vstar
```

The crate is `hop-top-vstar`; the library is `hop_top_vstar`. MSRV is
**Rust 1.98**. API documentation:
[docs.rs/hop-top-vstar](https://docs.rs/hop-top-vstar). Versions are
prereleases (`1.0.0-alpha.*`); until 1.0.0, pin an exact version
(`=1.0.0-alpha.0`). <!-- x-release-please-version -->

## Usage

Parse, hash, canonicalize — the most common path:

```rust
use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::{canonical, hashing};

let ics = "BEGIN:VCALENDAR\r\n\
           VERSION:2.0\r\n\
           PRODID:-//example//EN\r\n\
           BEGIN:VTODO\r\n\
           UID:todo-1\r\n\
           DTSTAMP:20260101T000000Z\r\n\
           DUE:20260102T000000Z\r\n\
           SUMMARY:Ship the port\r\n\
           END:VTODO\r\n\
           END:VCALENDAR\r\n";

let cal = rfc5545::parse(ics.as_bytes())?;

// sha256:e551d17793785d5876edc6e33bca47f2aae73eb9feac49678d76159a78c91b18
println!("{}", hashing::calendar(&cal));

// Vec<u8>, 175 bytes. Canonical form is a BYTE sequence; compare it
// as one rather than as a decoded string.
let bytes = canonical::calendar(&cal);
assert_eq!(bytes.len(), 175);
# Ok::<(), hop_top_vstar::Error>(())
```

The hash above is reproducible because `DTSTAMP` is a fixed literal.
Anything that stamps `DTSTAMP` from the clock — the `helpers`
constructors do — yields a digest that changes with the clock; verify
those instead of comparing them (see [Helpers](#helpers)).

## API

| Module | Area |
| --- | --- |
| `hop_top_vstar` | Data model, `Error`, enums, time and date helpers |
| `hop_top_vstar::codec::rfc5545` | iCalendar parse and encode |
| `hop_top_vstar::codec::rfc6350` | vCard parse and encode |
| `hop_top_vstar::codec::stream` | Streaming parse and encode over `Read` / `Write` |
| `hop_top_vstar::canonical` | Canonical byte form |
| `hop_top_vstar::hashing` | `X-VSTAR-HASH` compute and verify |
| `hop_top_vstar::validate` | Diagnostics with stable codes |
| `hop_top_vstar::rrule` | Recurrence parse and bounded expansion |
| `hop_top_vstar::duration` | ISO 8601 durations and alarm triggers |
| `hop_top_vstar::ext` | `X-*` extension scoping |
| `hop_top_vstar::diff` | Structural diff |
| `hop_top_vstar::supersession` | Append-only state transitions |
| `hop_top_vstar::helpers` | Constructors and typed accessors |
| `hop_top_vstar::generated` | Registry-rendered diagnostic tables |

### Hashing

`set_x_vstar` stamps a component with its own content hash;
`verify_x_vstar` recomputes and reports whether the stamp still holds.

```rust
use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::hashing;
# let ics = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//example//EN\r\n\
#            BEGIN:VTODO\r\nUID:todo-1\r\nDTSTAMP:20260101T000000Z\r\n\
#            DUE:20260102T000000Z\r\nSUMMARY:Ship the port\r\n\
#            END:VTODO\r\nEND:VCALENDAR\r\n";

let mut cal = rfc5545::parse(ics.as_bytes())?;
let todo = &mut cal.components[0];

hashing::set_x_vstar(todo);
// sha256:fbe32617ca289df0b02c8589379f9f252cd630837b3086333ee755620628c9f3
println!("{}", hashing::get_x_vstar(todo).unwrap());

let (ok, want, got) = hashing::verify_x_vstar(todo);
assert!(ok);
assert_eq!(want, got);
# Ok::<(), hop_top_vstar::Error>(())
```

### Validate

Diagnostics carry a stable `code` and a dotted `path`. Match on the
code; the `message` is prose and rewords between versions.
`severity_of` answers for a code without a document in hand.

```rust
use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::validate::{severity_of, validate, Severity};
# let ics = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//example//EN\r\n\
#            BEGIN:VTODO\r\nUID:todo-1\r\nDTSTAMP:20260101T000000Z\r\n\
#            DUE:20260102T000000Z\r\nSUMMARY:Ship the port\r\n\
#            END:VTODO\r\nEND:VCALENDAR\r\n";

let cal = rfc5545::parse(ics.as_bytes())?;
for d in validate(&cal) {
    println!("{} {} {}", d.severity, d.code, d.path);
    // error VS003 VCALENDAR.VTODO[uid=todo-1].X-VSTAR-HASH
}

assert_eq!(severity_of("VS003"), Some(Severity::Error));
assert_eq!(severity_of("VS999"), None);
# Ok::<(), hop_top_vstar::Error>(())
```

### Recurrence

Expansion is always bounded: `occurrences` takes a limit and reports
whether the series ended within it, so an unbounded rule cannot hang a
caller.

```rust
use hop_top_vstar::rrule::{occurrences, parse_rrule};
use hop_top_vstar::{format_time, parse_time};

let rule = parse_rrule("FREQ=DAILY;COUNT=3")?;
assert_eq!(rule.to_string(), "FREQ=DAILY;COUNT=3");

let dtstart = parse_time("20260401T120000Z").expect("form #2");
let (times, complete) = occurrences(&rule, dtstart, 10)?;

let stamps: Vec<String> = times.iter().map(|t| format_time(*t)).collect();
assert_eq!(
    stamps,
    ["20260401T120000Z", "20260402T120000Z", "20260403T120000Z"]
);
assert!(complete, "COUNT=3 ended inside the limit of 10");
# Ok::<(), hop_top_vstar::Error>(())
```

### Streaming

`VCalendarParser::new` takes any `Read` and yields one top-level
component per iteration in constant memory. It is an `Iterator` whose
item is `Result<Component>`: exhaustion is `None`, never an error.

```rust
use hop_top_vstar::codec::stream::VCalendarParser;
use std::io::Cursor;

let src = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//example//EN\r\n\
           BEGIN:VTODO\r\nUID:todo-1\r\nDTSTAMP:20260101T000000Z\r\nEND:VTODO\r\n\
           BEGIN:VTODO\r\nUID:todo-2\r\nDTSTAMP:20260101T000000Z\r\nEND:VTODO\r\n\
           END:VCALENDAR\r\n";

let mut parser = VCalendarParser::new(Cursor::new(src));
assert_eq!(parser.header().prod_id, "-//example//EN");

for item in &mut parser {
    let todo = item?;
    println!("{}", todo.uid()); // todo-1, then todo-2
}
assert!(parser.next().is_none());
# Ok::<(), hop_top_vstar::Error>(())
```

`VCardParser` has the same shape for vCard streams, and
`VCalendarEncoder` / `VCardEncoder` write the other direction.

### Helpers

Constructors seed `UID`, a `DTSTAMP` of now, and `X-VSTAR-HASH`. The
timestamp comes from the host clock (whole seconds), so the digest
changes with it — check it with `verify_x_vstar` rather than against a
literal.

```rust
use hop_top_vstar::helpers::{new_calendar, new_todo};
use hop_top_vstar::{hashing, parse_time};

let due = parse_time("20260102T000000Z").expect("form #2");
let todo = new_todo("todo-1", due)?;

let (ok, _, _) = hashing::verify_x_vstar(&todo);
assert!(ok, "a fresh component's stamp always verifies");

let mut cal = new_calendar("-//example//EN");
cal.components.push(todo);
assert_eq!(cal.components[0].uid(), "todo-1");
# Ok::<(), hop_top_vstar::Error>(())
```

### Extensions

`X-*` names fall into scopes: `X-VSTAR-*` belongs to the spec,
`X-EXP-*` is experimental, and `X-<SYSTEM>-*` belongs to one consuming
system. `system_name` recovers that system's slug, uppercased.

```rust
use hop_top_vstar::ext::{scope_of, system_name, Scope};

assert_eq!(scope_of("X-VSTAR-HASH"), Scope::VStar);
assert_eq!(scope_of("X-ACME-PRIORITY"), Scope::System);
assert_eq!(system_name("x-acme-priority").as_deref(), Some("ACME"));
assert_eq!(system_name("X-VSTAR-HASH"), None); // owned by the spec
```

### Errors

Every failure is one [`Error`] variant carrying positional context.
`sentinel()` recovers the cross-language class identifier — the same
token every V\* implementation reports for that class — so dispatch on
the sentinel, not on the message.

```rust
use hop_top_vstar::codec::rfc5545;

let err = rfc5545::parse(b"NOT A CALENDAR".as_slice()).unwrap_err();

assert_eq!(err.sentinel(), "ErrMalformed");
// malformed: expected BEGIN:VCALENDAR, got "NOT A CALENDAR"
println!("{err}");
```

The enum is `#[non_exhaustive]`, so a `match` needs a wildcard arm and
a new variant is not a breaking change.

## Conformance

This port is a **round-trip** implementation and self-certifies in
[`VSTAR-CONFORMANCE.md`](VSTAR-CONFORMANCE.md): spec revision, criteria
met, known deviations, and which corpus gates are green.

Its output is checked against the Go reference by the cross-language
parity harness: every implementation runs the same corpus and must
produce a byte-identical document, so agreement is proven rather than
assumed.

```sh
make test-parity   # from the monorepo root
```

## Develop

From the monorepo root:

```sh
make lint-rs test-rs build-rs
```

`src/generated/` is rendered from `spec/registry/` by `make
registry-gen`. Never hand-edit it; `make registry-check` fails on drift.

See [`CONTRIBUTING.md`](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md)
for repo-wide rules and
[`docs/dev/`](https://github.com/hop-top/poly-vstar/blob/main/docs/INDEX.md#for-developers)
for the development loop.

## Links

- [Specification](https://github.com/hop-top/poly-vstar/tree/main/spec) — normative text and the conformance corpus
- [Monorepo](https://github.com/hop-top/poly-vstar) — issues and pull requests
- [API documentation](https://docs.rs/hop-top-vstar) — docs.rs
- [Diagnostic codes](https://github.com/hop-top/poly-vstar/blob/main/docs/validate-codes.md) — the `VS***` catalog
- [API mapping](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/api-mapping.md) — every Go symbol and its Rust name
- [Porting guide](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/porting-guide.md) — writing a sister implementation

## License

MIT. See [`LICENSE`](https://github.com/hop-top/poly-vstar/blob/main/LICENSE)
at the monorepo root.

[`Error`]: https://docs.rs/hop-top-vstar/latest/hop_top_vstar/enum.Error.html
