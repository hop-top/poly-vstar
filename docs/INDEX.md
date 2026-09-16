# V* documentation

Navigation hub for the `docs/` tree. Use the section that matches what
you came here to do.

## For adopters

You are importing `hop.top/vstar` or `@hop-top/vstar` into your
project, OR building a sister implementation (Racket, Python, …) that
must agree byte-for-byte with the Go reference.

- [Quickstart](user/quickstart.md) — parse your first VCALENDAR in
  five minutes.
- [TypeScript package](../ts/README.md) — `@hop-top/vstar`: install,
  subpath exports, and the API surface for Node 22+. Its
  [conformance statement](../ts/VSTAR-CONFORMANCE.md) records which
  gates are green.
- [Python package](../py/README.md) — `hop-top-vstar`: install, module
  layout, and the API surface for Python 3.11+
  ([conformance](../py/VSTAR-CONFORMANCE.md)).
- [Rust crate](../rs/README.md) — `hop-top-vstar` on crates.io, with
  [docs.rs](https://docs.rs/hop-top-vstar) for the generated API
  reference ([conformance](../rs/VSTAR-CONFORMANCE.md)).
- [PHP package](../php/README.md) — `hop-top/vstar` on Packagist:
  install, namespace layout, and the API surface for PHP 8.2+
  ([conformance](../php/VSTAR-CONFORMANCE.md)).
- [How-to: validate and hash a Calendar](user/how-to-validate-and-hash.md) —
  compute `X-VSTAR-HASH`, run `validate.Validate`, interpret diagnostic
  codes.
- [How-to: parse and evaluate RRULE](user/how-to-recurrence.md) —
  v0.2 recurrence parser + `NextOccurrence` evaluator with the
  common patterns (DAILY, BYDAY, BYSETPOS).
- [How-to: build a sister vstar implementation](user/how-to-implement-vstar.md) —
  cross-validate against the Go reference using the conformance
  corpus and `X-VSTAR-HASH` algorithm.

## Reference

Audience-agnostic — any reader can link in.

- [Diagnostic code catalog](validate-codes.md) — every
  `validate.Diagnostic.Code` the Go reference emits, generated from
  [`spec/registry/`](../spec/registry/). The catalog is the count;
  restating a range here only goes stale.
- [Specification](../spec/) — V* normative text, CC-BY-4.0, authored in
  this repo.
- [Conformance corpus](../spec/v0.1/conformance/) — the canonical
  fixtures shared by every implementation;
  [`../go/testdata/`](../go/testdata/) is its generated mirror.

## For developers

You are contributing to vstar itself (the Go reference, one of the four
ports, or spec edits).

- [Setup the dev environment](dev/setup.md) — clone, mise install,
  `make ci`, `make fixtures-verify`.
- [Add a feature track end-to-end](dev/contributing-flow.md) —
  TDD posture, Conventional Commits, spec changes, conformance
  fixtures.
- [API mapping](dev/api-mapping.md) — every exported Go symbol and its
  name in TypeScript, Python, Rust and PHP; the twelve error
  sentinels; the `(value, ok)` versus `error` distinction; time
  representation per language.
- [Porting guide](dev/porting-guide.md) — the six-layer build order
  for a language port, the corpus gate each layer must pass, and the
  byte-level rules that make four implementations agree.
- [First publish of the four ports](dev/first-publish.md) — the
  one-time manual bootstrap on npm, PyPI, crates.io and Packagist,
  and the order the release PRs must be merged in.

## Rules of record

Every binding choice — canonical-form rules, datetime resolution,
the VTIMEZONE subset, the RRULE parsing scope, extension namespace
constraints — is stated normatively in the spec. Read it when you
need the rule behind a canonical-form behaviour or a parser scope
boundary.

- [spec/03 — canonicalization](../spec/v0.1/03-canonicalization.md)
  — rules 1–10, datetime resolution, VTIMEZONE subset, RRULE
  parsing scope.
- [spec/04 — extensions](../spec/v0.1/04-extensions.md)
  — `X-*` namespace tiers.

## Release process

Releases are cut automatically by [release-please](https://github.com/googleapis/release-please);
see [`.github/release-please-config.json`](../.github/release-please-config.json) and
[`go/CHANGELOG.md`](../go/CHANGELOG.md) for the Go component's history.

The exception is the very first publish of each port, which no
registry credential in CI is allowed to perform:
[First publish of the four ports](dev/first-publish.md).
