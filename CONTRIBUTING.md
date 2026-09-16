# Contributing to vstar

Thanks for considering a contribution. This document covers the
shared rules across the specification in [`spec/`](spec/), the Go
reference implementation in `go/`, and the four ports in `ts/`,
`py/`, `rs/` and `php/`. Per-language details live alongside the code
(see [`go/README.md`](go/README.md) and the per-language conventions
[below](#per-language-conventions)).

## What you're contributing to

- [`spec/`](spec/) — V\* specification text and the conformance
  corpus it defines. Authored here; spec changes are PRs against
  this repository.
- [`go/`](go/) — Go reference implementation, MIT.
- [`ts/`](ts/) — TypeScript port, MIT.
- [`py/`](py/) — Python port, MIT.
- [`rs/`](rs/) — Rust port, MIT.
- [`php/`](php/) — PHP port, MIT.
- [`go/testdata/`](go/testdata/) — generated mirror of
  [`spec/v0.1/conformance/`](spec/v0.1/conformance/), committed so the
  published Go module stays self-contained.
- [`docs/`](docs/) — contributor guides and reference docs.

The repository layout is described in
[`docs/dev/setup.md`](docs/dev/setup.md); the rules it implements
are normative in the spec.

## Licensing

- Code (`go/`, `ts/`, `py/`, `rs/`, `php/`) ships under **MIT**.
  Every source file carries the SPDX header in its language's comment
  syntax; the Go form is:

  ```go
  // SPDX-License-Identifier: MIT
  ```

- Spec **prose** — `spec/v*/**/*.md`, excluding the conformance
  corpus and `spec/v*/CHANGELOG.md` — ships under **CC-BY-4.0**;
  see [`spec/LICENSE`](spec/LICENSE). Each numbered section and
  `spec/v*/README.md` opens with
  `<!-- SPDX-License-Identifier: CC-BY-4.0 -->` — an HTML comment,
  so it never renders. Everything else under `spec/` ships under
  **MIT**: the conformance corpus (`spec/v*/conformance/**`, its
  READMEs and notes included), the behavior fixtures
  (`spec/behavior/**`), the registry tables (`spec/registry/**`),
  the release tooling (`spec/.github/scripts/**`) and its output,
  and the tree's front matter — so an implementation can vendor the
  fixtures and tables on the same terms as the code that consumes
  them. MIT files under `spec/` carry **no** SPDX header: fixtures
  are byte-exact test data, JSON has no comment syntax, and the
  scripts and Markdown follow suit; a headerless file there is MIT,
  not unscoped.
- **Inbound = outbound.** By opening a PR you agree your
  contribution is licensed under the same terms as the file you're
  touching — the default convention for public repositories, and
  the one [GitHub's Terms of Service
  §D.6](https://docs.github.com/site-policy/github-terms/github-terms-of-service#6-contributions-under-repository-license)
  states for contributions to a public repo. **No DCO sign-off, no
  CLA** — at least until V\* enters a foundation and the rules
  change.

## Commit conventions

We use [Conventional Commits](https://www.conventionalcommits.org/).
Allowed types:

| Type | Use for |
|------|---------|
| `feat` | new user-visible functionality |
| `fix` | bug fix |
| `docs` | docs-only change (README, guides, this file) |
| `style` | formatting (no code change) |
| `refactor` | restructure without behaviour change |
| `perf` | performance improvement |
| `test` | adding or revising tests |
| `build` | toolchain, deps, build scripts |
| `ci` | GitHub Actions, CI infrastructure |
| `chore` | release plumbing, housekeeping |

Optional scope is the area, e.g. `feat(rfc5545): ...`,
`fix(canonical): ...`, `ci: ...`. Use the imperative mood
("add", not "added"). Keep the subject under 72 chars.

Commit often, then squash before merge if the history is noisy.

## TDD posture (RED / GREEN / REFACTOR)

Every behavioural change lands as a sequence of TDD-phase commits:

1. **RED** — write the failing test first. Commit message ends in
   `— RED`. Tree fails CI.
2. **GREEN** — minimum implementation to pass. Commit message ends
   in `— GREEN`. Tree passes CI.
3. **REFACTOR** — tidy the code without expanding scope. Commit
   message ends in `— REFACTOR`. Tree still passes.

Pure config or docs changes (no test-able behaviour) skip the
RED commit and go straight to GREEN. The smoke test in the
scaffolding track demonstrates the full RED → GREEN flow.

## Architecture decisions

Anything that constrains a future contributor — the canonical form,
the hash discipline, a public error sentinel, a parser scope
boundary, a wire format — is a spec rule, not an implementation
detail. Land the [`spec/`](spec/) change in the same PR as the code
that depends on it, so the public record carries the rule itself.

Standing choices this repo takes as given, each with the
alternative it rejected:

- Monorepo layout — the specification in `spec/`, one tree per
  language (`go/`, `ts/`, `py/`, `rs/`, `php/`), docs at the root.
  Each tree is republished to its own read-only mirror by subtree
  split (`.github/workflows/publish.yml`), so a tree is promoted to
  a repository root on release. Nothing under `spec/` or a language
  tree may link parent-relative to `docs/`, `go/` or a sibling —
  those links are dead at the mirror root; use absolute URLs.
  `spec/LICENSE` names paths relative to `spec/`, not to this repo,
  for the same reason.
- License — MIT for every language tree, chosen over Apache-2.0
  with the trade on the record: Apache-2.0's express patent grant
  (§3) was given up, and MIT carries no contribution clause, which
  is why inbound = outbound rests on GitHub's ToS rather than on a
  licence term.
- `hop.top/vstar` takes no `hop.top/kit` dependency (next section).
- The Go reference is the reference. `make test-parity` diffs every
  port against it and never promotes another language when it fails
  (exit 2, not a comparison); a value wrong in Go is corrected in Go
  and every emitter regenerated. See
  [`tools/parity/README.md`](tools/parity/README.md).
- Tables every language must carry identically live once, in
  [`spec/registry/`](spec/registry/), and are rendered by
  `make registry-gen`. Rejected: one hand-maintained copy per port,
  which is how a hand-edited catalog once listed codes out of order.
  One exception is deliberate: each validator builds its `STATUS`
  vocabulary from its own wire enums — the values its codec encodes
  against — and reconciles them against the generated table in a
  test, rather than reading the table. A lookup would let validator
  and codec drift apart; the test keeps five ports agreeing without
  giving that linkage up.
- Datetimes resolve only against the document's own `VTIMEZONE`,
  never a system or bundled tz database, so the same document hashes
  the same on every machine in every year. Normative in spec/03; the
  per-language forbidden list is in the
  [porting guide](docs/dev/porting-guide.md#no-iana-timezone-database).
- Generators diff before they write and name every path they
  rewrote. A generator that writes unconditionally repairs a
  hand-edit on its way to a green build and leaves `git diff` clean,
  so the edit — and any bug it was trying to fix — vanishes
  unnoticed. All three generators in this repo had that defect once.
- The recurrence iteration bound is a published constant
  (`rrule.MaxIterations`), not a per-call option: only unsatisfiable
  rules reach it, and a larger budget fails identically at higher
  cost. Hitting it is reported as `ErrIterationCap`, never as
  termination.
- `duration` keeps the authored units (weeks, days, time fields,
  sign) rather than collapsing to a nanosecond count. A calendar day
  is 23/24/25 h across a DST transition, so a count cannot hold `P1D`
  apart from `PT24H`, and re-serialising one would rewrite the wire
  form and shift alarms that cross a transition.

## No `hop.top/kit` dependency

`hop.top/vstar` is a library. It depends only on the Go standard
library and selected `golang.org/x/...` packages. **No
`hop.top/kit` import.** PRs that introduce a non-stdlib
non-`golang.org/x` dependency require an explicit reviewer approval
and a note in this section.

## Shared conformance corpus

Conformance fixtures are authored in
[`spec/v0.1/conformance/`](spec/v0.1/conformance/) — one source for
both the spec text and the fixtures that pin it.
[`go/testdata/`](go/testdata/) is a generated mirror of that
directory, committed so the `hop-top/vstar` module mirror (which
publishes `go/` alone) stays self-contained. **Never hand-edit
`go/testdata/`**: `make fixtures-verify` rewrites it from the corpus
and reports any path it had to change. The report is the only
evidence — restoring a hand-edited mirror file leaves the tree
clean, so `git diff` alone cannot see that an edit was lost.

`make fixtures-verify` regenerates every `.canonical` and `.hash`
sibling in the corpus, verifies the rrule sidecar contracts, mirrors
the corpus into `go/testdata/`, syncs the fuzz seeds into the
go-fuzz corpus directories, and then fails on any drift in either
tree. `go/testdata/fuzz/` is the root package's own go-fuzz corpus,
not spec content, and survives the mirror untouched.

A fixture change now lands as one PR in this repository:

1. Add or modify a fixture under `spec/v0.1/conformance/`.
2. Update the relevant section of [`spec/v0.1/`](spec/v0.1/) to
   describe the new behaviour.
3. Update every implementation — the Go reference and all four ports
   — to handle it; add tests that consume the new fixture. A fixture
   the ports do not agree on fails `make test-parity`.
4. Run `make fixtures-verify` and commit the regenerated corpus
   siblings and the `go/testdata/` mirror alongside the change.

Splitting these across PRs leaves implementations divergent
from the spec — refused at review.

## Per-language conventions

- **Spec**: markdown under `spec/` is linted by `make lint-spec`
  (markdownlint-cli2, config at `spec/.markdownlint-cli2.jsonc`). A
  commit touching `spec/vX.Y/` must stay within that one version
  directory and must not carry breaking-change syntax — CI enforces
  both. `spec/` is republished with itself as the repository root
  (see [Architecture decisions](#architecture-decisions)): link out
  of it with absolute URLs only, and point contributors at this
  repository rather than at root `make` targets that do not exist
  on the mirror.
- **Go**: see [`go/README.md`](go/README.md) for the module and
  [`docs/dev/setup.md`](docs/dev/setup.md) for build/test/lint
  invocations (root `Makefile`, `go -C go`), the toolchain pins
  (`mise.toml`), formatter (gofumpt), and linter set (golangci-lint,
  config in `go/.golangci.yml`).
- **TypeScript**: see [`ts/README.md`](ts/README.md) for the package.
  pnpm is the package manager (pinned by `packageManager` in
  `ts/package.json`); vitest runs the suite, eslint with
  typescript-eslint lints it, and `tsc` typechecks under `strict` with
  `NodeNext` module resolution — so relative imports carry their `.js`
  suffix, which is what the emitted ESM actually needs at runtime.
  Invocations are the root `Makefile`'s `lint-ts`, `test-ts` and
  `build-ts`. `ts/src/generated/` is rendered from `spec/registry/` by
  `make registry-gen`; never hand-edit it.
- **Python**: see [`py/README.md`](py/README.md) for the distribution.
  uv is the package and environment manager (`py/uv.lock` is
  committed); pytest runs the suite, ruff lints and formats, and mypy
  typechecks under `strict`. The distribution is `hop-top-vstar` and
  the import name is `vstar`.
- **Rust**: see [`rs/README.md`](rs/README.md) for the crate. rustfmt
  and clippy (`--all-targets --all-features -D warnings`) are both
  gates. The MSRV is `rust-version` in `rs/Cargo.toml`; do not raise it
  without a release note.
- **PHP**: see [`php/README.md`](php/README.md) for the package. PSR-12
  via php-cs-fixer, PHPStan at level 9, PHPUnit for the suite. The
  floor is PHP 8.2 and CI tests 8.2 and 8.4, so resolve `composer.json`
  against the `config.platform.php` floor rather than the local
  runtime.

Each port follows the same rules as TypeScript above: the root
`Makefile`'s `lint-`, `test-` and `build-` targets are the invocations,
and each port's generated tree is rendered from `spec/registry/` by
`make registry-gen` and must never be hand-edited.

## Filing an issue

GitHub issues are the right place for spec ambiguities, bugs,
and feature requests. For private security reports, see
`SECURITY.md` (TBD; until then, contact the repository owners
directly).

## Reviewers

Repository maintainers review every PR; releases and spec-facing
changes need a maintainer approval.
