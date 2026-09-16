# V\* specification

> [!IMPORTANT]
> **Read-only mirror.** This tree is published from
> [`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar)
> `spec/` on each `vstar-v0.1/v*` tag. Edits here are overwritten on
> the next publish. File issues and pull requests against
> [`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar/issues).
>
> **Pin a version.** Depend on a specific spec version (`v0.1`,
> `v0.2`, …) rather than tracking `main`. See
> [`v0.1/CHANGELOG.md`](v0.1/CHANGELOG.md) for the v0.1 change history.

Language-agnostic specification for V\*: agentic-system state —
worlds, missions, players, turns, observations, decisions —
represented as **iCalendar (RFC 5545) + vCard (RFC 6350)
components**. Agent work that already has time, identity, and
sequence semantics rides existing calendar/scheduler/contact tooling
instead of a bespoke protocol.

> **Status:** Active development. Usable today, with some rough edges as features evolve.

This directory ships **no code**. It is the specification: the
normative Markdown text plus the cross-language conformance corpus
every implementation must reproduce byte-for-byte. Implementations
consume it from here:

Every implementation below lives in the source repository
[`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar),
one tree per language. Each tree is republished on release to its own
read-only mirror; the mirror is a distribution channel, not a place to
send changes.

| Language | Source tree | Read-only mirror | Status |
|----------|-------------|------------------|--------|
| Go (`hop.top/vstar`) | [`go/`](https://github.com/hop-top/poly-vstar/tree/main/go) | [`hop-top/vstar`](https://github.com/hop-top/vstar) | Reference implementation; every other language is cross-validated against its bytes |
| TypeScript (`@hop-top/vstar`) | [`ts/`](https://github.com/hop-top/poly-vstar/tree/main/ts) | [`hop-top/vstar-ts`](https://github.com/hop-top/vstar-ts) | Round-trip port, cross-validated against the Go reference by parity CI |
| Python (`hop-top-vstar`) | [`py/`](https://github.com/hop-top/poly-vstar/tree/main/py) | [`hop-top/vstar-py`](https://github.com/hop-top/vstar-py) | Round-trip port, cross-validated against the Go reference by parity CI |
| Rust (`hop-top-vstar`) | [`rs/`](https://github.com/hop-top/poly-vstar/tree/main/rs) | [`hop-top/vstar-rs`](https://github.com/hop-top/vstar-rs) | Round-trip port, cross-validated against the Go reference by parity CI |
| PHP (`hop-top/vstar`) | [`php/`](https://github.com/hop-top/poly-vstar/tree/main/php) | [`hop-top/vstar-php`](https://github.com/hop-top/vstar-php) | Round-trip port, cross-validated against the Go reference by parity CI |
| Racket | [`hop-top/agr`](https://github.com/hop-top/agr) | — | Downstream emitter; cross-validates against the Go reference |

Each port states its own class, deviations and green gates in a
`VSTAR-CONFORMANCE.md` beside its source — see
[Self-certification](v0.1/05-conformance.md#self-certification).

## Spec versions

| Version | Status | Source of truth |
|---------|--------|-----------------|
| [`v0.1`](v0.1/) | Draft | [`v0.1/`](v0.1/) — five numbered sections, read in order |

Versions are independent directories. Breaking changes open a new
version directory; they never bump within an existing one.

## How to read the spec

1. Design principles and scope: [`v0.1/01-overview.md`](v0.1/01-overview.md).
2. The core mapping table: [`v0.1/02-component-mapping.md`](v0.1/02-component-mapping.md).
3. Byte-for-byte canonical form and hashing: [`v0.1/03-canonicalization.md`](v0.1/03-canonicalization.md).
4. `X-*` extension discipline: [`v0.1/04-extensions.md`](v0.1/04-extensions.md).
5. What "conformant" means: [`v0.1/05-conformance.md`](v0.1/05-conformance.md).
6. Conformance corpus: [`v0.1/conformance/`](v0.1/conformance/).

## Implementing V\*

An implementation is conformant when it meets every MUST in
[`05-conformance.md`](v0.1/05-conformance.md) and reproduces
every fixture under [`conformance/`](v0.1/conformance/) —
same input, same `.canonical` bytes, same `.hash`. The
[how-to-implement guide](https://github.com/hop-top/poly-vstar/blob/main/docs/user/how-to-implement-vstar.md)
walks through the rules with worked examples.

## What's NOT in scope

- **API shape** — package layout, type names, builder helpers — see
  each implementation tree.
- **Design rationale** — the spec states the rules, not the
  deliberation behind them.
- **Transport, storage, sync** — V\* is a data convention, not a
  wire protocol.

## Contributing

Spec text, corpus and implementations live in one repository —
[`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar) — so a
fixture change is one pull request there: the spec text under `spec/`,
the fixture under `spec/v0.1/conformance/`, the implementation change,
and the regenerated `.canonical` / `.hash` files. Its
[`CONTRIBUTING.md`](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md)
has the local check commands; see also
[`v0.1/conformance/README.md`](v0.1/conformance/README.md).

## License

CC-BY-4.0 for the spec text (`v*/**/*.md`); MIT for the conformance
corpus (`v*/conformance/**`) and the release tooling
(`.github/scripts/**`). See [`LICENSE`](LICENSE).
