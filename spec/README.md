# V\* specification

The normative text, conformance corpus and registry tables for V\*:
agentic-system state — worlds, missions, players, turns, observations,
decisions — carried as RFC 5545 iCalendar and RFC 6350 vCard
components, with a canonical form and a content hash that make two
documents comparable byte-for-byte.

**Status:** Draft
> [!IMPORTANT]
> **Source and mirror.** This tree is authored under `spec/` in
> [`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar) and is
> republished, with `spec/` promoted to the repository root, to
> [`hop-top/spec-vstar`](https://github.com/hop-top/spec-vstar) on every
> `vstar-spec/v*` tag. The mirror comes into existence with the first
> release and is read-only; edits there are overwritten on the next
> publish. File issues and pull requests against
> [`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar/issues).
>
> **Pin a version directory.** Cite and implement against `v0.1/`, not
> `main`. Versions are independent directories and a breaking change
> opens a new one; see [Versions](#versions).

This directory ships no code and nothing to install. It is a document
with three parts every implementation consumes: the numbered sections
under `v0.1/`, the fixtures under `v0.1/conformance/` and `behavior/`,
and the enumerations under `registry/`.

## What the specification pins down

RFC 5545 and RFC 6350 let one logical content be written many ways:
properties in any order, parameters in any order, datetimes in local
or UTC form, lines folded at any column. A generic parser accepts all
of them and cannot say whether two documents mean the same thing.

V\* removes the choice. [`03-canonicalization.md`](v0.1/03-canonicalization.md)
defines one byte sequence per logical content — folding, for
instance, happens at a 75-**octet** boundary, and a multi-byte UTF-8
sequence that straddles it is split — and
[`X-VSTAR-HASH`](v0.1/03-canonicalization.md#hashing) is `sha256:`
followed by the hex SHA-256 of that canonical form with the hash
property itself removed. Equality is then a string comparison, and
the conformance corpus asserts the bytes.

Around the canonical form the specification fixes the mapping of
agentic concepts to component types, the common properties every
component carries (`UID`, `DTSTAMP`, `X-VSTAR-HASH`), the append-only
supersession ledger for state changes, the `X-*` extension namespace,
and what "conformant" means for a document and for an implementation.

## Versions

| Version | Directory | Release tag | Change history |
|---------|-----------|-------------|----------------|
| v0.1 | [`v0.1/`](v0.1/) | `vstar-v0.1/v*` | [`v0.1/CHANGELOG.md`](v0.1/CHANGELOG.md) |

Versions are independent directories. A breaking change opens a new
version directory; it never bumps within an existing one. CI enforces
both halves of that rule with
[`.github/scripts/validate_spec_commits.py`](.github/scripts/validate_spec_commits.py):
a commit may touch only one `vX.Y/` directory, and a commit touching
one may not carry Conventional Commits breaking-change syntax.

The `**Status:**` line at the top of this file and of each version's
documents is written by release automation
([`.github/scripts/update_status_lines.py`](.github/scripts/update_status_lines.py))
from the release channel: `alpha` is Draft, `beta` is Pre-release,
`rc` is Release Candidate, and an unsuffixed version is General
Availability.

## How to read v0.1

The sections are numbered and normative in this order. Each states
its own status at the top.

1. [`01-overview.md`](v0.1/01-overview.md) — design principles,
   [scope](v0.1/01-overview.md#scope) and
   [non-goals](v0.1/01-overview.md#non-goals).
2. [`02-component-mapping.md`](v0.1/02-component-mapping.md) —
   agentic concept to RFC 5545 / RFC 6350 component; required and
   recommended properties; `RELTYPE`; the supersession ledger.
3. [`03-canonicalization.md`](v0.1/03-canonicalization.md) — the
   byte-for-byte rules, datetime and `TZID` resolution, the RRULE
   parsing scope and wire form, `TRIGGER` conventions, hashing.
4. [`04-extensions.md`](v0.1/04-extensions.md) — `X-*` namespace
   tiers, the promotion path, compatibility rules.
5. [`05-conformance.md`](v0.1/05-conformance.md) — conformance
   criteria, implementation classes, failure classes,
   self-certification.

[`v0.1/README.md`](v0.1/README.md) is the version's own front matter.
The corpus and the registry, below, complete the version.

## Conformance

An implementation is conformant when it meets every MUST in
[`05-conformance.md`](v0.1/05-conformance.md) and reproduces the
corpus: same input, same `.canonical` bytes, same `.hash`.

- **Implementation classes.** An implementation is an *emitter*, a
  *consumer*, or a *round-trip* (both, with byte-identical re-emit of
  what it emitted) — see
  [Implementation classes](v0.1/05-conformance.md#implementation-classes).
- **Failure classes.** Rejected or abandoned input is reported by
  class, and each class has one token — `ErrMalformed`,
  `ErrUnsupportedRRule`, and the rest — that every implementation
  exposes under exactly that spelling, whatever its host language
  calls the error value. The table in
  [Failure classes](v0.1/05-conformance.md#failure-classes) is the
  complete set for v0.1; the corpus names classes by token.
- **Self-certification.** Until a formal suite exists, an
  implementation publishes a `VSTAR-CONFORMANCE.md` stating the spec
  revision it targets, its class, its deviations and its green gates —
  see [Self-certification](v0.1/05-conformance.md#self-certification).

### The corpus

Two fixture trees, both read by every implementation:

| Tree | Contract | What a fixture states |
|------|----------|-----------------------|
| [`v0.1/conformance/`](v0.1/conformance/) | Wire | What parses, which canonical bytes it produces, which hash; which malformed inputs fail and in which class; how an RRULE parses, formats, expands and is rejected; `VTIMEZONE` inputs for `TZID` resolution; fuzz seeds. |
| [`behavior/`](behavior/) | Behavior | Which diagnostics a document raises, what a diff between two documents reports, which status a supersession ledger projects, when an alarm fires, how an extension name classifies, how a local timestamp resolves against a zone. |

Byte identity against the corpus is necessary, not sufficient: a
fixture pins only the rules it happens to reach, and some rules cannot
be reached through the wire format at all. Each tree's README —
[`v0.1/conformance/README.md`](v0.1/conformance/README.md) and
[`behavior/README.md`](behavior/README.md) — records the file
conventions, the sort orders that are part of the contract, and what
the fixtures cannot verify, which an implementation pins with tests of
its own.

The fixtures are files; this tree carries no runner. In the source
repository `make fixtures-verify` regenerates both trees from the Go
reference and fails on drift, and `make test-parity` diffs every
implementation's reading of the corpus against the reference's — the
emitter contract is
[`tools/parity/README.md`](https://github.com/hop-top/poly-vstar/blob/main/tools/parity/README.md).

### The registry

[`registry/`](registry/) holds the enumerations the specification
fixes as JSON tables, authored once and never retyped per language:
`diagnostic-codes.json` (the stable validation codes, their rule and
severity), `extension-scopes.json` (the `X-*` tiers of
`04-extensions.md`), `standard-properties.json` (the RFC 5545 and
RFC 6350 property allow-lists) and `status-vocabulary.json` (the
`STATUS`, `CLASS`, `TRANSP` and `RELTYPE` vocabularies). The source
repository renders them into each implementation's constants and into
the diagnostic code catalog,
[`docs/validate-codes.md`](https://github.com/hop-top/poly-vstar/blob/main/docs/validate-codes.md),
and `make registry-check` fails CI when a rendering disagrees with
the table.

This is not an extension registry. Registering `X-<SYSTEM>-*` names is
deferred to a later version — see
[Registration](v0.1/04-extensions.md#registration).

## Implementations

Every implementation lives in the source repository, one tree per
language, and is republished on release to its own read-only mirror.
A mirror is a distribution channel, not a place to send changes.

| Language | Source tree | Read-only mirror | Status |
|----------|-------------|------------------|--------|
| Go (`hop.top/vstar`) | [`go/`](https://github.com/hop-top/poly-vstar/tree/main/go) | [`hop-top/vstar`](https://github.com/hop-top/vstar) | Reference implementation; every other language is cross-validated against its bytes |
| TypeScript (`@hop-top/vstar`) | [`ts/`](https://github.com/hop-top/poly-vstar/tree/main/ts) | [`hop-top/vstar-ts`](https://github.com/hop-top/vstar-ts) | Round-trip port, cross-validated against the Go reference by parity CI |
| Python (`hop-top-vstar`) | [`py/`](https://github.com/hop-top/poly-vstar/tree/main/py) | [`hop-top/vstar-py`](https://github.com/hop-top/vstar-py) | Round-trip port, cross-validated against the Go reference by parity CI |
| Rust (`hop-top-vstar`) | [`rs/`](https://github.com/hop-top/poly-vstar/tree/main/rs) | [`hop-top/vstar-rs`](https://github.com/hop-top/vstar-rs) | Round-trip port, cross-validated against the Go reference by parity CI |
| PHP (`hop-top/vstar`) | [`php/`](https://github.com/hop-top/poly-vstar/tree/main/php) | [`hop-top/vstar-php`](https://github.com/hop-top/vstar-php) | Round-trip port, cross-validated against the Go reference by parity CI |

The Go tree publishes no `VSTAR-CONFORMANCE.md`: its emitted bytes
define conformance rather than attest to it. Each port publishes one
beside its source, and any deviation it records is confined to API
surface — none changes emitted bytes, or the parity gate would fail.

Building a new implementation: the
[how-to-implement guide](https://github.com/hop-top/poly-vstar/blob/main/docs/user/how-to-implement-vstar.md)
walks through the rules with worked examples, and the
[porting guide](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/porting-guide.md)
covers the harness, the reference-bytes tables and the gates a port
must turn green.

## What is not in scope

[`01-overview.md`](v0.1/01-overview.md#scope) draws the line:
orchestration semantics, projection and state-from-log algorithms,
storage, and any transport or wire format beyond the RFC 5545 / 6350
text form belong to consuming systems. Two more things this document
does not carry:

- **API shape** — package layout, type names, builder helpers — is
  each implementation's own; the mapping between them is the source
  repository's
  [`docs/dev/api-mapping.md`](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/api-mapping.md).
- **Design rationale** — the specification states the rules, not the
  deliberation behind them.

## Changing the specification

Spec text, corpus and implementations share one repository, so a
change to a rule is one pull request against
[`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar): the
section under `v0.1/`, the fixture that reaches the new rule, the
implementation change, and the regenerated `.canonical` / `.hash`
siblings. [`v0.1/conformance/README.md`](v0.1/conformance/README.md#change-policy)
states the change policy and the mutation proof a new fixture needs;
[`CONTRIBUTING.md`](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md)
has the repository-wide rules and the local check commands.

Prose in this tree is linted with markdownlint on its own rules,
[`.markdownlint-cli2.jsonc`](.markdownlint-cli2.jsonc); the source
repository runs it as `make lint-spec`. Nothing under `spec/` links
parent-relative to the rest of the source repository, because those
links are dead at the mirror root.

## License

This tree is dual-licensed; paths are relative to this directory and
hold in both the source repository and the mirror. See
[`LICENSE`](LICENSE).

| What | Paths | Licence |
|------|-------|---------|
| Spec text — the normative Markdown under a version directory | `v*/**/*.md`, excluding `v*/conformance/**` and `v*/CHANGELOG.md` | CC-BY-4.0 |
| Conformance corpus, behavior fixtures, registry tables, release tooling and its changelog output, and this tree's front matter | `v*/conformance/**` (its READMEs and notes included), `behavior/**`, `registry/**`, `.github/scripts/**`, `v*/CHANGELOG.md`, `README.md`, `.markdownlint-cli2.jsonc` | MIT |

Every CC-BY-4.0 file carries an `SPDX-License-Identifier` header, so
the scoping survives a file being copied out of the tree. The MIT
files carry none — fixtures are byte-exact test data and JSON has no
comment syntax — so a file here with no header is MIT, not unscoped.
Everything outside this tree — the implementations and the
contributor documentation — is MIT under the source repository's own
licence.
