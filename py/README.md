# hop-top-vstar

Canonical calendar and contact interchange for agentic systems —
RFC 5545 and RFC 6350, with byte-stable output and a content hash.

[![PyPI](https://img.shields.io/pypi/v/hop-top-vstar?label=pypi)](https://pypi.org/project/hop-top-vstar/)
[![CI](https://img.shields.io/github/actions/workflow/status/hop-top/poly-vstar/ci-py.yml?branch=main&label=ci)](https://github.com/hop-top/poly-vstar/actions/workflows/ci-py.yml?query=branch%3Amain)
[![Types](https://img.shields.io/badge/types-py.typed-blue)](https://peps.python.org/pep-0561/)
[![Spec](https://img.shields.io/badge/spec-draft%20v0.1-blue)](../spec/)
[![License](https://img.shields.io/badge/license-MIT-green)](../LICENSE)

> **Read-only mirror.** This package is developed in the polyglot
> monorepo [`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar)
> under `py/` and republished on release to
> [`hop-top/vstar-py`](https://github.com/hop-top/vstar-py). Open issues
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

- **Byte-identical output across languages.** This port is verified
  byte-for-byte against the Go reference over the whole shared
  conformance corpus — not merely self-consistent. The same calendar
  hashed by the Python, Go and TypeScript ports yields the same
  `sha256:` string. See [Conformance](#conformance).
- **Content addressing.** A stable hash over canonical bytes, so
  documents can be deduplicated, cached, or compared across services.
- **Append-only state.** Supersession chains rather than mutation, so
  history survives.

**When not to use this:** if you need a full calendaring client —
timezone database management, free/busy scheduling across attendees,
CalDAV sync, or broad support for RFC 5545's long tail — use a general
iCalendar library such as `icalendar`. V\* deliberately implements a
bounded subset chosen for machine-generated state, and its `RRULE`
scope excludes `FREQ=SECONDLY`/`MINUTELY` and `RSCALE`. If you simply
want to read someone else's calendar file, this is more machinery than
you need.

## Install

```sh
pip install hop-top-vstar
```

```sh
uv add hop-top-vstar
```

Requires **Python 3.11 or newer**. Pure Python, no dependencies. The
distribution is `hop-top-vstar`; the import name is `vstar`. Type hints
ship inline and are advertised by `py.typed`, so mypy and pyright see
them with no `types-*` companion.

## Usage

Parse, hash, canonicalize — the most common path:

```python
from vstar.canonical import calendar as canonical_calendar
from vstar.codec.rfc5545 import parse
from vstar.hashing import calendar as hash_calendar

ics = (
    "BEGIN:VCALENDAR\r\n"
    "VERSION:2.0\r\n"
    "PRODID:-//example//EN\r\n"
    "BEGIN:VTODO\r\n"
    "UID:todo-1\r\n"
    "DTSTAMP:20260101T000000Z\r\n"
    "DUE:20260102T000000Z\r\n"
    "SUMMARY:Ship the port\r\n"
    "END:VTODO\r\n"
    "END:VCALENDAR\r\n"
)

cal = parse(ics)

print(hash_calendar(cal))
# sha256:e551d17793785d5876edc6e33bca47f2aae73eb9feac49678d76159a78c91b18

# Canonical form is a BYTE sequence; compare it as one.
data = canonical_calendar(cal)
print(type(data).__name__, len(data))
# bytes 175
```

That `sha256:e551d177…` is the same string the Go and TypeScript ports
print for the same input. The hash is the cross-language contract.

## API

Each area is its own subpackage, so an application imports only what it
uses. The root package re-exports the data model, the error classes,
the time helpers, and the `canonical`, `diff`, `duration`, `ext`,
`hashing`, `helpers` and `supersession` namespaces.

| Import | Area |
| --- | --- |
| `vstar` | Data model, error classes, time helpers, plus the namespaces below |
| `vstar.codec.rfc5545` | iCalendar parse and serialize |
| `vstar.codec.rfc6350` | vCard parse and serialize |
| `vstar.codec.stream` | Incremental stream decoding |
| `vstar.canonical` | Canonical byte form |
| `vstar.hashing` | `X-VSTAR-HASH` compute and verify |
| `vstar.validate` | Diagnostics with stable codes |
| `vstar.rrule` | Recurrence parse and bounded expansion |
| `vstar.duration` | ISO 8601 durations and alarm triggers |
| `vstar.ext` | `X-*` extension namespaces |
| `vstar.diff` | Structural diff |
| `vstar.supersession` | Append-only state transitions |
| `vstar.helpers` | Convenience constructors and accessors |

### Validate

Diagnostics carry a stable `code` and a dotted `path`. Match on the
code; the message is prose and rewords between versions.

```python
from vstar.validate import validate

for d in validate(cal):
    print(d.severity, d.code, d.path)
# error VS003 VCALENDAR.VTODO[uid=todo-1].X-VSTAR-HASH
```

### Recurrence

Expansion is always bounded: `occurrences` takes a limit and reports
whether the series ended within it, so an unbounded rule cannot hang a
caller.

```python
from vstar import format_time, parse_time
from vstar.rrule import occurrences, parse_rrule

rule = parse_rrule("FREQ=DAILY;COUNT=3")
dtstart = parse_time("20260401T120000Z")

times, complete = occurrences(rule, dtstart, 10)
print([format_time(t) for t in times], complete)
# ['20260401T120000Z', '20260402T120000Z', '20260403T120000Z'] True
```

### Hashing and tamper detection

`set_x_vstar` stamps the hash onto a component; `verify_x_vstar`
recomputes it and reports whether the content still matches.

```python
from vstar import Property, parse_time
from vstar.helpers import new_todo, set_todo_status, todo_status
from vstar.hashing import set_x_vstar, verify_x_vstar
from vstar.types import TodoStatus

todo = new_todo("todo-9", parse_time("20260501T090000Z"))
set_todo_status(todo, TodoStatus.IN_PROCESS)
print(todo.type, todo.uid(), todo.get("DUE").value, todo_status(todo))
# VTODO todo-9 20260501T090000Z IN-PROCESS

set_x_vstar(todo)
ok, want, got = verify_x_vstar(todo)
print(ok)
# True

todo.set(Property(name="SUMMARY", value="edited"))
print(verify_x_vstar(todo)[0])
# False
```

### Diff

`of_calendar` pairs components across two calendars and reports the
property-level changes, with paths that locate each change site.

```python
from vstar.codec.rfc5545 import parse
from vstar.diff import of_calendar


def todo_cal(summary: str):
    return parse(
        "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//example//EN\r\n"
        "BEGIN:VTODO\r\nUID:todo-1\r\nDTSTAMP:20260101T000000Z\r\n"
        f"DUE:20260102T000000Z\r\nSUMMARY:{summary}\r\n"
        "END:VTODO\r\nEND:VCALENDAR\r\n"
    )


before = todo_cal("Ship the port")
after = todo_cal("Ship the Python port")

for d in of_calendar(before, after):
    print(d.path)
    for pd in d.properties:
        print(" ", pd.op, pd.property.name, pd.old.value, "->", pd.property.value)
# VCALENDAR.VTODO[uid=todo-1]
#    Changed SUMMARY Ship the port -> Ship the Python port
```

`pd.op` is a `DiffOp` enum member; it renders as `Changed` because
`__str__` carries the reference's display spelling. Compare against
`DiffOp.CHANGED`, not against the string.

### Errors

Every failure raises a subclass of `VstarError`. Catch by class, or
dispatch on `sentinel` — the Go identifier, spelled identically in
every V\* implementation.

```python
from vstar import Malformed, VstarError
from vstar.codec.rfc5545 import parse

try:
    parse(b"BEGIN:VCALENDAR\r\nnot a content line\r\n")
except Malformed as e:
    print(e.sentinel)
    # ErrMalformed
except VstarError as e:
    if e.sentinel == "ErrUnclosedBlock":
        ...
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
make lint-py test-py build-py
```

[uv](https://docs.astral.sh/uv/) drives the environment; ruff lints and
formats, mypy typechecks under `strict`, and pytest runs the suite.

`src/vstar/_generated/` is rendered from `spec/registry/` by `make
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
