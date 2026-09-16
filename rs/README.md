# hop-top-vstar

Canonical calendar and contact interchange for agentic systems —
RFC 5545 and RFC 6350, with byte-stable output and a content hash.

[![crates.io](https://img.shields.io/crates/v/hop-top-vstar?label=crates.io)](https://crates.io/crates/hop-top-vstar)
[![docs.rs](https://img.shields.io/docsrs/hop-top-vstar?label=docs.rs)](https://docs.rs/hop-top-vstar)
[![CI](https://img.shields.io/github/actions/workflow/status/hop-top/poly-vstar/ci-rs.yml?branch=main&label=ci)](https://github.com/hop-top/poly-vstar/actions/workflows/ci-rs.yml?query=branch%3Amain)
[![MSRV](https://img.shields.io/badge/msrv-1.98-blue)](https://releases.rs/)
[![Spec](https://img.shields.io/badge/spec-draft%20v0.1-blue)](../spec/)
[![License](https://img.shields.io/badge/license-MIT-green)](../LICENSE)

> **Read-only mirror.** This crate is developed in the polyglot
> monorepo [`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar)
> under `rs/` and republished on release to
> [`hop-top/vstar-rs`](https://github.com/hop-top/vstar-rs). Open issues
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
  The quickstart below prints the same `sha256:e551d177…` the
  TypeScript, Go, Python and PHP ports print for the same input.
- **Append-only state.** Supersession chains rather than mutation, so
  history survives.

Rust-specific reasons to pick this crate: it is
`#![forbid(unsafe_code)]`, it pulls three dependencies (`sha2`,
`chrono` with `default-features = false`, `unicode-normalization`) and
no runtime, and every failure is one non-exhaustive `Error` enum whose
variants carry positional context.

**When not to use this:** if you need a full calendaring client —
timezone database management, free/busy scheduling across attendees,
CalDAV sync, or broad support for RFC 5545's long tail — use a general
iCalendar library. V\* deliberately implements a bounded subset chosen
for machine-generated state, and its `RRULE` scope excludes
`FREQ=SECONDLY`/`MINUTELY` and `RSCALE`. If you simply want to read
someone else's calendar file, this is more machinery than you need.

## Install

```sh
cargo add hop-top-vstar
```

The crate is `hop-top-vstar`; the library is `hop_top_vstar`. MSRV is
**Rust 1.98**. Full API documentation lives at
[docs.rs/hop-top-vstar](https://docs.rs/hop-top-vstar).

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

## API

| Module | Area |
| --- | --- |
| `hop_top_vstar` | Data model, `Error` sentinels, time and date helpers |
| `hop_top_vstar::codec::rfc5545` | iCalendar parse and encode |
| `hop_top_vstar::codec::rfc6350` | vCard parse and encode |
| `hop_top_vstar::codec::stream` | Incremental stream decoding |
| `hop_top_vstar::canonical` | Canonical byte form |
| `hop_top_vstar::hashing` | `X-VSTAR-HASH` compute and verify |
| `hop_top_vstar::validate` | Diagnostics with stable codes |
| `hop_top_vstar::rrule` | Recurrence parse and bounded expansion |
| `hop_top_vstar::duration` | ISO 8601 durations and alarm triggers |
| `hop_top_vstar::ext` | `X-*` extension namespaces |
| `hop_top_vstar::diff` | Structural diff |
| `hop_top_vstar::supersession` | Append-only state transitions |
| `hop_top_vstar::helpers` | Convenience constructors and accessors |
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

```rust
use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::validate::validate;
# let ics = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//example//EN\r\n\
#            BEGIN:VTODO\r\nUID:todo-1\r\nDTSTAMP:20260101T000000Z\r\n\
#            DUE:20260102T000000Z\r\nSUMMARY:Ship the port\r\n\
#            END:VTODO\r\nEND:VCALENDAR\r\n";

let cal = rfc5545::parse(ics.as_bytes())?;
for d in validate(&cal) {
    println!("{} {} {}", d.severity.as_str(), d.code, d.path);
    // error VS003 VCALENDAR.VTODO[uid=todo-1].X-VSTAR-HASH
}
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
make lint-rs test-rs build-rs
```

`src/generated/` is rendered from `spec/registry/` by `make
registry-gen`. Never hand-edit it; `make registry-check` fails on drift.

See [`CONTRIBUTING.md`](../CONTRIBUTING.md) for repo-wide rules and
[`docs/dev/`](../docs/dev/) for the development loop.

## Links

- [Specification](../spec/) — normative text and the conformance corpus
- [Monorepo](https://github.com/hop-top/poly-vstar) — issues and pull requests
- [API documentation](https://docs.rs/hop-top-vstar) — docs.rs
- [Porting guide](../docs/dev/porting-guide.md) — writing a sister implementation
- [Diagnostic codes](../docs/validate-codes.md) — the `VS***` catalog

## License

MIT. See [`LICENSE`](../LICENSE) at the repository root.

[`Error`]: https://docs.rs/hop-top-vstar/latest/hop_top_vstar/enum.Error.html
