# V\*

Calendar/Card-shaped data convention for agentic systems.

> **Status:** Active development. Usable today, with some rough edges as features evolve.

[![Latest tag](https://img.shields.io/github/v/tag/hop-top/poly-vstar?filter=vstar/*&label=release&color=00ADD8&sort=semver)](https://github.com/hop-top/poly-vstar/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/hop-top/poly-vstar/ci-go.yml?branch=main&label=ci)](https://github.com/hop-top/poly-vstar/actions/workflows/ci-go.yml?query=branch%3Amain)
[![Spec](https://img.shields.io/badge/spec-draft%20v0.1-blue)](spec/)
[![Code](https://img.shields.io/badge/code-MIT-green)](LICENSE)

V\* represents agentic-system state — worlds, missions, players, turns,
observations, decisions — as **iCalendar (RFC 5545) + vCard (RFC 6350)
components**. Agent work that already has time, identity, and sequence
semantics gets to ride existing calendar/scheduler/contact tooling
instead of a bespoke protocol.

This repository is the polyglot home of the spec and its reference
implementations. The spec is authored here under [`spec/`](spec/); each
language lives in its own tree and is republished to a per-language
read-only mirror on release.

## Install

| Language | Install | Tree | Mirror |
|----------|---------|------|--------|
| Go | `go get hop.top/vstar` | [`go/`](go/) | [`hop-top/vstar`](https://github.com/hop-top/vstar) |
| TypeScript | `pnpm add @hop-top/vstar` | [`ts/`](ts/) | [`hop-top/vstar-ts`](https://github.com/hop-top/vstar-ts) |
| Python | `pip install hop-top-vstar` | [`py/`](py/) | [`hop-top/vstar-py`](https://github.com/hop-top/vstar-py) |
| Rust | `cargo add hop-top-vstar` | [`rs/`](rs/) | [`hop-top/vstar-rs`](https://github.com/hop-top/vstar-rs) |
| PHP | `composer require hop-top/vstar` | [`php/`](php/) | [`hop-top/vstar-php`](https://github.com/hop-top/vstar-php) |

## Quick start

Parse a VCALENDAR in Go (the other four languages are in
[Per-language usage](#per-language-usage)):

```go
import (
    "strings"

    "hop.top/vstar/codec/rfc5545"
)

cal, err := rfc5545.Parse(strings.NewReader(input))
```

Full walkthrough (parse + hash + walk components):
[`docs/user/quickstart.md`](docs/user/quickstart.md).

## Use this when

- You emit agent state and want it to interop with calendars,
  schedulers, or contact directories with no custom serialization.
- You need a stable reference point so multiple runtimes (Go, TS,
  Python, Racket, …) can produce byte-identical output.
- You want a published spec, conformance fixtures, and a reference
  implementation — not a vendored file inside someone else's runtime.

**Skip this if:** you need a streaming wire protocol, or your data
has no time / identity / sequence semantics.

## Per-language usage

Each implementation has its own README with install and idiomatic
examples:

- [Go](go/README.md) — `hop.top/vstar`, the reference implementation
  every other language cross-validates against.
- [TypeScript](ts/README.md) — `@hop-top/vstar`, ESM-only, Node 22+.
- [Python](py/README.md) — `hop-top-vstar`, pure standard library,
  3.11+.
- [Rust](rs/README.md) — `hop-top-vstar`, `#![forbid(unsafe_code)]`,
  MSRV 1.98.
- [PHP](php/README.md) — `hop-top/vstar`, PSR-4 under `HopTop\Vstar\`,
  PHP 8.2+.

Every port is verified byte-identical to the Go reference over the
shared corpus by `make test-parity`.

## How it's organized

| Path | Purpose |
| --- | --- |
| `spec/` | The V\* specification — normative text, conformance corpus, and spec-side CI; published on `vstar-v0.1/v*` tags to the read-only mirror [`hop-top/spec-vstar`](https://github.com/hop-top/spec-vstar) |
| `go/` | Go reference implementation (module `hop.top/vstar`), its tests, lint config and changelog |
| `go/testdata/` | Generated, committed mirror of [`spec/v0.1/conformance/`](spec/v0.1/conformance/), kept in sync by `make fixtures-verify` so the published Go module stays self-contained |
| `go/cmd/fixtures-*` | Corpus generators and the `fixtures-verify` drift gate |
| `docs/` | Adopter guides, contributor guides, diagnostic code catalog — shared across languages |
| `Makefile`, `mise.toml` | Root build targets and toolchain pins; every Go target runs through `go -C go` |
| `.github/` | CI, release-please, publish-on-tag mirroring |

## Spec

The spec is authored here under [`spec/`](spec/) and is the source of
truth; its [`spec/v0.1/conformance/`](spec/v0.1/conformance/) corpus is
the canonical conformance set. [`go/testdata/`](go/testdata/) is a
generated mirror of it — `make fixtures-verify` regenerates both trees
and fails on any drift, so the two MUST stay byte-identical. Nobody
hand-edits `go/testdata/`.

| Layer | State |
|-------|-------|
| Spec ([`spec/`](spec/)) | Draft v0.1 |
| Go reference (`hop.top/vstar`) | `go/`, tagged `vstar/v*` |
| TypeScript port (`@hop-top/vstar`) | [`ts/`](ts/), tagged `vstar-ts/v*` |
| Python port (`hop-top-vstar`) | [`py/`](py/), tagged `vstar-py/v*` |
| Rust port (`hop-top-vstar`) | [`rs/`](rs/), tagged `vstar-rs/v*` |
| PHP port (`hop-top/vstar`) | [`php/`](php/), tagged `vstar-php/v*` |
| Racket emitter (in `hop-top/agr`) | Downstream, cross-validates against Go |

## Where to go next

| You are… | Start here |
|----------|------------|
| **Adopting `hop.top/vstar`** | [`go/README.md`](go/README.md) → [`docs/user/quickstart.md`](docs/user/quickstart.md) → [`docs/INDEX.md`](docs/INDEX.md) for how-tos and reference |
| **Building a sister implementation** (TS / Python / …) | [`05-conformance.md`](spec/v0.1/05-conformance.md) + [`conformance/`](spec/v0.1/conformance/) + [`docs/user/how-to-implement-vstar.md`](docs/user/how-to-implement-vstar.md) |
| **Changing vstar itself** | [`CONTRIBUTING.md`](CONTRIBUTING.md) → [`docs/dev/setup.md`](docs/dev/setup.md) → [`docs/dev/contributing-flow.md`](docs/dev/contributing-flow.md) |
| **Understanding a canonical-form rule** | [spec/03 — canonicalization](spec/v0.1/03-canonicalization.md) |

## Why this exists as its own repo

- The convention is reusable — any agent-protocol or orchestration
  system can emit V\*.
- Conformance must not be coupled to one runtime's release cycle.
- Independent implementations need a stable reference point — five
  already do, and the next one should not have to negotiate for it.
- Foundation conversations are easier with an independent spec.

## Related projects

- **AGR** — downstream consumer. Its compiler converts
  Racket-described worlds into V\* component sequences. AGR-internal
  extensions live in `X-AGR-*`.
- **AGNTCY TS SDK** — candidate consumer; agents can emit V\* as
  their canonical action log. Not yet wired.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for repo-wide rules
(licensing, commit conventions, TDD posture, shared `go/testdata/`
discipline) and [`docs/dev/`](docs/dev/) for the dev loop.

Security reports: until `SECURITY.md` lands, see
[`CONTRIBUTING.md`](CONTRIBUTING.md) for the private-report path.

## License

The repository is MIT, except the spec text.

| What | Paths | Licence |
| --- | --- | --- |
| Code — reference implementations, build tooling, docs | everything outside `spec/` | MIT, see [`LICENSE`](LICENSE) |
| Spec text — normative Markdown | `spec/v*/**/*.md` | CC-BY-4.0, see [`spec/LICENSE`](spec/LICENSE) |
| Conformance corpus and spec-side scripts | `spec/v*/conformance/**`, `spec/.github/scripts/**` | MIT, see [`spec/LICENSE`](spec/LICENSE) |

Every CC-BY-4.0 file carries an `SPDX-License-Identifier` header, so
the scoping travels with a file copied out of the repository.
