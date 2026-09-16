# V\*

Calendar/Card-shaped data convention for agentic systems — one spec,
five implementations, byte-identical output.

> **Status:** Active development. Usable today, with some rough edges as features evolve.

[![Latest tag](https://img.shields.io/github/v/tag/hop-top/poly-vstar?filter=vstar/*&label=release&color=00ADD8&sort=semver)](https://github.com/hop-top/poly-vstar/releases)
[![CI (Go)](https://img.shields.io/github/actions/workflow/status/hop-top/poly-vstar/ci-go.yml?branch=main&label=ci%20%28go%29)](https://github.com/hop-top/poly-vstar/actions/workflows/ci-go.yml?query=branch%3Amain)
[![Parity](https://img.shields.io/github/actions/workflow/status/hop-top/poly-vstar/ci-parity.yml?branch=main&label=parity)](https://github.com/hop-top/poly-vstar/actions/workflows/ci-parity.yml?query=branch%3Amain)
[![Spec](https://img.shields.io/badge/spec-draft%20v0.1-blue)](spec/)
[![Code](https://img.shields.io/badge/code-MIT-green)](LICENSE)

V\* represents agentic-system state — worlds, missions, players, turns,
observations, decisions — as **iCalendar (RFC 5545) + vCard (RFC 6350)
components**. Agent work that already has time, identity, and sequence
semantics rides existing calendar, scheduler, and contact tooling
instead of a bespoke protocol.

A generic iCalendar library parses an `.ics` and hands back a tree. It
cannot tell you whether two documents mean the same thing, because the
RFCs let one logical content be written many ways: properties in any
order, parameters in any order, datetimes in local or UTC form, folding
at any column. V\* pins that down with a **canonical form** — one byte
sequence per logical content — and an `X-VSTAR-HASH` over it, so "did
this change?" is a string comparison. On top it adds what agent state
needs and a calendar library does not carry: structural validation
with stable diagnostic codes, RRULE evaluation, and append-only
supersession.

This repository is the polyglot home of V\*: the specification, its
conformance corpus, the Go reference implementation, and four ports.
Nothing at this level is the package you install — that is one tree
down, and this page tells you which.

## Why one repository, five languages

- **One change is one pull request.** Spec text, the fixture under
  [`spec/v0.1/conformance/`](spec/v0.1/conformance/), every
  implementation, and the regenerated `.canonical` / `.hash` files
  land together. The corpus cannot drift from the code that
  reproduces it.
- **Conformance is not coupled to one runtime's release cycle.** Each
  tree releases on its own tag and is republished to its own
  read-only mirror (see [Releases and mirrors](#releases-and-mirrors)).
  The spec is a document with its own licence and its own mirror,
  citable independently of any implementation.
- **Independent implementations get a stable reference point.** Go is
  the reference; the four ports cross-validate against its bytes over
  the shared corpus, and the next port follows the same
  [porting guide](docs/dev/porting-guide.md) instead of negotiating
  for one.

### How the five trees are kept in agreement

Every implementation ships a parity emitter: a program that reads
`spec/` and prints one JSON document describing what it makes of
every fixture. The harness diffs each port's document against the Go
reference's, over the entire corpus, and any difference fails the run:

```sh
make test-parity
```

A green run prints a single `parity: ok` line naming every language in
the roster and the corpus size; a mismatch exits non-zero. CI runs the
same command on every push and pull request that touches `spec/`,
`tools/parity/`, or any language tree, so no port can diverge from the
reference and merge. The emitter contract is
[`tools/parity/README.md`](tools/parity/README.md).

Two more gates keep the shared inputs honest: `make fixtures-verify`
regenerates the corpus and its committed mirror under
[`go/testdata/`](go/testdata/) and fails on drift, and
`make registry-check` fails if any language's generated constants
(diagnostic codes, property allow-list, extension scopes) disagree
with [`spec/registry/`](spec/registry/). `make ci` runs all of it plus
each language's own lint, test, and build.

## Pick a tree

| Language | Install | Runtime floor | Source tree | Read-only mirror |
|----------|---------|---------------|-------------|------------------|
| Go | `go get hop.top/vstar` | Go 1.25 | [`go/`](go/) | [`hop-top/vstar`](https://github.com/hop-top/vstar) |
| TypeScript | `pnpm add @hop-top/vstar` | Node 22 | [`ts/`](ts/) | [`hop-top/vstar-ts`](https://github.com/hop-top/vstar-ts) |
| Python | `pip install hop-top-vstar` | Python 3.11 | [`py/`](py/) | [`hop-top/vstar-py`](https://github.com/hop-top/vstar-py) |
| Rust | `cargo add hop-top-vstar` | Rust 1.98 | [`rs/`](rs/) | [`hop-top/vstar-rs`](https://github.com/hop-top/vstar-rs) |
| PHP | `composer require hop-top/vstar` | PHP 8.2 | [`php/`](php/) | [`hop-top/vstar-php`](https://github.com/hop-top/vstar-php) |

Each tree's README is that package's front page — install, a runnable
example, the API surface. Open the one for your language:
[Go](go/README.md), [TypeScript](ts/README.md), [Python](py/README.md),
[Rust](rs/README.md), [PHP](php/README.md). Go is the reference
implementation; the other four are round-trip ports of it, and each
port records its class, deviations, and green gates in a
`VSTAR-CONFORMANCE.md` beside its source.

The mirror column names where each tree is republished on release.
A mirror is created by the first release of its tree, so a mirror
whose tree has not shipped yet does not exist. Whether or not it does,
the source of truth is here: open issues and pull requests against
this repository, never against a mirror.

## Use this when

- You emit agent state and want it to interop with calendars,
  schedulers, or contact directories with no custom serialization.
- You need several runtimes (Go, TypeScript, Python, Rust, PHP, or a
  port of your own) to produce byte-identical output for the same
  logical content, and a hash that proves they did.
- You want a published spec, conformance fixtures, and a reference
  implementation — not a vendored file inside someone else's runtime.

**Skip this if:** you need a streaming wire protocol, or your data
has no time / identity / sequence semantics.

## How it's organized

| Path | Purpose |
| --- | --- |
| [`spec/`](spec/) | The V\* specification: normative text, the conformance corpus, and the registry tables every port carries. Its own [README](spec/README.md) is the spec's front page. |
| [`go/`](go/) | Go reference implementation (module `hop.top/vstar`), its tests, and the `cmd/fixtures-*` corpus generators. |
| [`go/testdata/`](go/testdata/) | Generated, committed mirror of [`spec/v0.1/conformance/`](spec/v0.1/conformance/), kept in sync by `make fixtures-verify` so the published Go module stays self-contained. Nobody hand-edits it. |
| [`ts/`](ts/), [`py/`](py/), [`rs/`](rs/), [`php/`](php/) | The four ports. Each carries its own manifest, lockfile, toolchain, changelog, and conformance statement. |
| [`tools/parity/`](tools/parity/) | The cross-language parity harness and the emitter contract. |
| [`tools/registry/`](tools/registry/) | Renders [`spec/registry/`](spec/registry/) into per-language constants and [`docs/validate-codes.md`](docs/validate-codes.md), so a table is authored once and never retyped per port. |
| [`docs/`](docs/INDEX.md) | Adopter how-tos, contributor guides, the porting guide, and the diagnostic code catalog — shared across languages. |
| `Makefile`, `mise.toml` | Root build targets and toolchain pins. `make ci` is the full cross-language gate; `make ci-<lang>` is one language's slice of it. |
| `.github/` | CI per language, parity CI, release-please, and publish-on-tag mirroring. |

## Spec

The spec is authored under [`spec/`](spec/) and is the source of
truth. [`spec/v0.1/`](spec/v0.1/) holds the numbered sections — read
[`spec/README.md`](spec/README.md) for the order — and
[`spec/v0.1/conformance/`](spec/v0.1/conformance/) is the canonical
fixture set every implementation must reproduce: same input, same
`.canonical` bytes, same `.hash`. Versions are independent
directories; a breaking change opens a new one.

An implementation is conformant when it meets every MUST in
[`05-conformance.md`](spec/v0.1/05-conformance.md) and reproduces the
whole corpus. [`docs/user/how-to-implement-vstar.md`](docs/user/how-to-implement-vstar.md)
walks through the rules with worked examples.

## Releases and mirrors

Each component releases on its own tag, cut by release-please from
the commits that touch its tree, and is republished on that tag — the
tree alone, promoted to the mirror's root — by
[`publish.yml`](.github/workflows/publish.yml).

| Component | Tree | Tag | Mirror |
|-----------|------|-----|--------|
| Spec | `spec/` | `vstar-v0.1/v*` | [`hop-top/spec-vstar`](https://github.com/hop-top/spec-vstar) |
| Go reference | `go/` | `vstar/v*` | [`hop-top/vstar`](https://github.com/hop-top/vstar) |
| TypeScript port | `ts/` | `vstar-ts/v*` | [`hop-top/vstar-ts`](https://github.com/hop-top/vstar-ts) |
| Python port | `py/` | `vstar-py/v*` | [`hop-top/vstar-py`](https://github.com/hop-top/vstar-py) |
| Rust port | `rs/` | `vstar-rs/v*` | [`hop-top/vstar-rs`](https://github.com/hop-top/vstar-rs) |
| PHP port | `php/` | `vstar-php/v*` | [`hop-top/vstar-php`](https://github.com/hop-top/vstar-php) |

Mirrors are distribution channels, not places to send changes, and
each one comes into existence with its tree's first release. Current
versions per component are in
[`.github/.release-please-manifest.json`](.github/.release-please-manifest.json).

## Where to go next

| You are… | Start here |
|----------|------------|
| **Adding V\* to a project** | The README for your language: [Go](go/README.md), [TypeScript](ts/README.md), [Python](py/README.md), [Rust](rs/README.md), [PHP](php/README.md). Then [`docs/INDEX.md`](docs/INDEX.md) for how-tos and reference. |
| **Building a sister implementation** | [`docs/dev/porting-guide.md`](docs/dev/porting-guide.md) → [`05-conformance.md`](spec/v0.1/05-conformance.md) → [`docs/user/how-to-implement-vstar.md`](docs/user/how-to-implement-vstar.md) |
| **Changing V\* itself** | [`CONTRIBUTING.md`](CONTRIBUTING.md) → [`docs/dev/setup.md`](docs/dev/setup.md) → [`docs/dev/contributing-flow.md`](docs/dev/contributing-flow.md) |
| **Understanding a canonical-form rule** | [spec/03 — canonicalization](spec/v0.1/03-canonicalization.md) |
| **Looking up a diagnostic code** | [`docs/validate-codes.md`](docs/validate-codes.md), generated from [`spec/registry/`](spec/registry/) |
| **Mapping a Go symbol to another language** | [`docs/dev/api-mapping.md`](docs/dev/api-mapping.md) |

## Related projects

- [`hop-top/tlc`](https://github.com/hop-top/tlc) — IDE-agnostic todo
  list with full syncing with any issue tracking tool for tasks
  created remotely.

More coming soon.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for repo-wide rules
(licensing, commit conventions, TDD posture, shared `go/testdata/`
discipline) and [`docs/dev/`](docs/INDEX.md#for-developers) for the
dev loop. `make ci` is the gate a pull request must pass; it needs
every toolchain, so `make ci-<lang>` is the shortcut while you work
in one tree.

Security reports: until `SECURITY.md` lands, see
[`CONTRIBUTING.md`](CONTRIBUTING.md) for the private-report path.

## License

Every language tree is MIT. The spec tree is the one exception, and
it is dual-licensed.

| What | Paths | Licence |
| --- | --- | --- |
| Code — the Go reference, the four ports, build tooling, docs | everything outside `spec/` | MIT, see [`LICENSE`](LICENSE) |
| Spec text — normative Markdown | `spec/v*/**/*.md` | CC-BY-4.0, see [`spec/LICENSE`](spec/LICENSE) |
| Conformance corpus and spec-side scripts | `spec/v*/conformance/**`, `spec/.github/scripts/**` | MIT, see [`spec/LICENSE`](spec/LICENSE) |

Every CC-BY-4.0 file carries an `SPDX-License-Identifier` header, so
the scoping travels with a file copied out of the repository.
