# Set up the dev environment

Clone the repo, install the toolchains, and run the cross-language
CI gate locally.

## Use this when

- You are about to file your first PR against `hop-top/poly-vstar`.
- You want to confirm your environment matches the one CI uses
  before debugging a failing job.
- You hit `command not found` on `make ci` and need the toolchain.

## Result

After completing this guide, you will:

- Have the pinned Go version active.
- Run `make ci` and see all five implementations green, ending on
  the parity line.
- Know the `ci-<lang>` shortcut for the one language you are
  working in.
- Run `make fixtures-verify` standalone and see no drift against
  the committed corpus.

## Before you begin

You need:

- Git.
- [mise](https://mise.jdx.dev/) installed and on `$PATH` — the
  root `mise.toml` pins the Go toolchain and golangci-lint.
- GNU `make` (the bundled BSD `make` on macOS works too).
- `gofumpt` (`go install mvdan.cc/gofumpt@latest`); golangci-lint
  comes from the `mise.toml` pin.

For the **full** matrix you also need each port's toolchain:
`pnpm` (TypeScript), `uv` (Python), `cargo` (Rust), and
`php` + `composer` (PHP). `mise.toml` pins the first three; PHP is
deliberately not pinned there — mise has no PHP backend that builds
reliably on both macOS and Linux — so install it out of band
(`brew install php@8.2 composer` locally; CI uses `setup-php`). You
do not need all five to contribute to one language — see
[the shortcut](#the-per-language-shortcut).

## Quick version

```sh
git clone git@github.com:hop-top/poly-vstar.git
cd poly-vstar
mise install && mise run install
make ci
```

`make ci` is the full cross-language gate: the shared checks, then
each of the five implementations, then the parity harness that
diffs all five against each other. Green means you're ready to
contribute.

## Steps

### 1. Clone the repository

```sh
git clone git@github.com:hop-top/poly-vstar.git
cd poly-vstar
```

The repo is a multi-tree monorepo, one tree per language:

- [`spec/`](../../spec/) — the specification, authored here.
  `spec/v0.1/*.md` is the prose (CC-BY-4.0);
  `spec/v0.1/conformance/` is the authored conformance corpus and
  `spec/.github/scripts/` its tooling (both MIT).
- `go/` — Go reference implementation (MIT), module
  `hop.top/vstar`, with its `go.mod`, `.golangci.yml` and
  `CHANGELOG.md`.
- `ts/`, `py/`, `rs/`, `php/` — the four ports (MIT), each with its
  own manifest, lockfile and toolchain. All four are cross-validated
  against `go/` by `make test-parity`.
- `tools/parity/` — the cross-language harness. `parity.py`'s
  `LANGUAGES` table is the roster of what gets compared.
- `go/testdata/` — generated mirror of `spec/v0.1/conformance/`,
  produced by `make fixtures-verify` and committed so the
  `hop-top/vstar` mirror of `go/` stays self-contained. Never
  hand-edited.
- `docs/` — contributor guides and reference docs, shared across
  languages.
- `Makefile`, `mise.toml` — root build targets and toolchain pins.
  Every Go target runs through `go -C go`, so you never `cd go`
  yourself.

The split keeps every implementation running against one shared
corpus.

### 2. Install the pinned Go toolchain

```sh
mise install && mise run install
```

`mise` reads `mise.toml`, installs the exact Go version and
golangci-lint the repo builds against, then `mise run install`
downloads the module dependencies (`go -C go mod download`). Confirm:

```sh
mise current go
```

Expected: a single line printing the pinned version (e.g.
`go 1.25.x`).

### 3. Build and test

```sh
make build
make test
```

Both should complete cleanly. `make test` runs with the race
detector (`go -C go test -race ./...`); slow on first run, fast on
subsequent runs once the test cache populates.

### 4. Run the full CI gate

```sh
make ci
```

`ci` runs the whole matrix, in this order:

| Step | What it checks |
|---|---|
| `registry-check` | Generated per-language constants match `spec/registry/`. |
| `fixtures-verify` | Regenerates the corpus under `spec/v0.1/conformance/`, mirrors it into `go/testdata/`, asserts no drift. |
| `ci-go` | `vet` + `fmt-check` + `lint` + `cover` (race-mode, profile at `go/coverage.out`). |
| `ci-ts` | `lint-ts` + `test-ts` + `build-ts`. |
| `ci-py` | `lint-py` + `test-py` + `build-py`. |
| `ci-rs` | `lint-rs` + `test-rs` + `build-rs`. |
| `ci-php` | `lint-php` + `test-php` + `build-php`. |
| `test-parity` | Diffs all five emitters against the Go reference. |

The two shared gates run first because every port reads the tables
and corpus they regenerate — a drift there explains the port
failures that would otherwise follow it. `test-parity` runs last: it
is the only step needing all five toolchains at once, and a port
that cannot build its own suite will not produce a comparable
emitter document either.

A green run ends on the parity line, naming every language in the
roster and the number of cases the corpus held at that commit:

```text
parity: ok go php py rs ts (<n> cases)
```

The count is printed by the run, not recorded here — it grows with
every fixture, and a number written into this file goes stale the
next time one lands.

**Measured wall-clock, warm caches** (Apple Silicon, all toolchains
already installed, build caches populated):

| Command | Wall-clock |
|---|---|
| `make ci` (full matrix) | **33 s** |
| `make ci-go` | 10 s |
| `make ci-ts` | 10 s |
| `make ci-py` | 7 s |
| `make ci-rs` | 14 s |
| `make ci-php` | 9 s |
| `make test-parity` | 5 s |

A cold run is substantially slower — first `cargo build`, first
`pnpm install`, and first `composer install` dominate it.

`lint-spec` (markdownlint-cli2 over `spec/**/*.md`, config at
`spec/.markdownlint-cli2.jsonc`) is a separate target — run it
where markdownlint-cli2 is installed.

A green `make ci` is the gate every PR must pass before review.

### The per-language shortcut

`make ci` needs all five toolchains. When you are working in one
tree, run only that language's slice:

```sh
make ci-go     # vet + fmt-check + lint + cover
make ci-ts     # lint + test + build
make ci-py
make ci-rs
make ci-php
```

Each `ci-<lang>` is exactly the slice `make ci` runs for that
language, so a green shortcut means that leg of the full gate is
green too. It needs only that language's toolchain.

The shortcut is not a substitute for the full gate. It cannot catch
a **cross-language** divergence — two implementations disagreeing
about the same fixture is what `test-parity` exists to find, and it
only runs in `make ci`. Run the full matrix before you open a PR;
use the shortcut in the edit loop.

### 5. Verify the conformance corpus

```sh
make fixtures-verify
```

This regenerates every `.canonical` and `.hash` sibling under
`spec/v0.1/conformance/`, verifies the rrule sidecar contracts,
mirrors the corpus into `go/testdata/`, and diffs both trees. If
either is dirty after the regeneration, the target fails — either
commit the regenerated files (because you intentionally changed
implementation behavior) or revert the implementation change
(because the drift is unintentional).

The same target also keeps the codec fuzz-seed corpora
(`go/codec/rfc5545/testdata/fuzz/` etc.) in sync with the canonical
source under `spec/v0.1/conformance/fuzz-seed/`. Never hand-edit
the go-fuzz corpus — and never hand-edit `go/testdata/`, which is
a generated mirror; fixtures are authored in
`spec/v0.1/conformance/`.

## How CI mirrors this

Each language has its own workflow — `ci-go.yml`, `ci-ts.yml`,
`ci-py.yml`, `ci-rs.yml`, `ci-php.yml` — and `ci-parity.yml` runs
the cross-language harness. Together they are what `make ci` runs
locally, split across jobs so they run in parallel.

Every toolchain is cached, keyed on the lockfile that pins its
dependency tree, so a run that changes no dependencies restores
rather than re-resolves:

| Language | Cache | Key |
|---|---|---|
| Go | `actions/setup-go` `cache: true` | `go/go.sum` |
| TypeScript | `actions/setup-node` `cache: pnpm` | `ts/pnpm-lock.yaml` |
| Python | `astral-sh/setup-uv` `enable-cache` | `py/uv.lock` |
| Rust | `Swatinem/rust-cache` | `rs/Cargo.lock` |
| PHP | `actions/cache` on `~/.composer/cache` | `php/composer.lock` |

PHP is the one spelled out by hand: `shivammathur/setup-php` places
the interpreter, not the dependency tree, and takes no cache input
of its own. It caches composer's *download* cache rather than
`php/vendor` — restoring `vendor/` directly would skip composer's
integrity check, and the install is fast once no network fetches
remain.

Two more things the workflows do that are easy to undo by accident:

- `ci-go.yml` and `ci-parity.yml` set `MISE_DISABLE_TOOLS=go` so the
  `mise.toml` Go pin does not shadow the `stable` / `oldstable`
  matrix `actions/setup-go` installs. The pin is the local floor;
  the matrix is what CI tests.
- `pnpm/action-setup` is given `package_json_file: ts/package.json`.
  It reads `packageManager` from the repository root by default, and
  a polyglot root has no `package.json`, so without it the step
  fails before any TypeScript job runs.

### Release wiring

Releases are cut by release-please from
[`.github/release-please-config.json`](../../.github/release-please-config.json),
one package per tree, and published on tag by the shared
`publish-on-tag` workflow in
[`.github/workflows/publish.yml`](../../.github/workflows/publish.yml),
which subtree-splits each tree to its own read-only mirror. The
constraints that are not obvious from the files:

- **Every seed in `.release-please-manifest.json` is
  prerelease-shaped** (`1.0.0-alpha.0`, never `1.0.0`). Each package
  declares `prerelease`, and the org preflight rejects a
  stable-shaped seed for a prerelease component. release-please bumps
  *from* a seed, so the first cut from an `alpha.0` seed lands at
  `alpha.1` unless a commit carries a `Release-As: 1.0.0-alpha.0`
  footer. In this manifest layout that footer applies to every
  package whose path the commit touches, and only to those — a footer
  on a commit outside every package path is ignored. The bootstrap
  commit therefore touches all six trees.
- **No `extra-files`.** Every strategy rewrites its own manifest
  natively — `go.mod`, `package.json`, `Cargo.toml`, `php/VERSION`.
  Python's strategy only rewrites an `__init__.py` under a directory
  matching the distribution slug, and `hop-top-vstar` is not
  `vstar`, so `py/` reads `__version__` back from installed metadata
  instead of storing it in source.
- **The spec entry says `ecosystem: none` explicitly.** It is a
  payload with no registry: the mirror job runs, no publish job does.
  Omitting the key is not equivalent — an absent entry reads as an
  umbrella tag and the mirror job skips it.
- **The TypeScript entry has a `build-command` and no
  `test-command`.** An override *replaces* the reusable workflow's
  default install-and-test step, so vitest would run against a tree
  with no `node_modules`.
- `dir` is the tree root (`go`, `ts`, `spec`, …), never `.`: the
  split ships only that tree, so a mirror never receives `docs/`, the
  `Makefile` or a sibling language.

### Branch protection

`Parity` is only a gate if `main` requires it. **`main` currently
has no branch protection and no rulesets** (`gh api
repos/hop-top/poly-vstar/branches/main/protection` returns `404
Branch not protected`), so every check below is advisory today.

A repo admin can require all of them with:

```sh
gh api -X PUT repos/hop-top/poly-vstar/branches/main/protection \
  --input - <<'JSON'
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "Parity",
      "Lint (stable)",
      "Test (stable)",
      "Node 22",
      "Python 3.11",
      "Rust 1.98",
      "PHP 8.2"
    ]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": null,
  "restrictions": null
}
JSON
```

The context strings are GitHub *check-run* names — the rendered
`jobs.<id>.name`, not the workflow or job id. The list above pins
each language's floor leg (the oldest supported toolchain line);
the newer legs (`Node 24`, `Python 3.13`, `Rust stable`,
`PHP 8.4`, `Lint (oldstable)`, `Test (oldstable)`) still run and
still report, they are just not required to merge.

Because every workflow is path-filtered, a check that does not run
for a given PR reports nothing. With `strict: true` a required
context that never runs blocks the merge, so add a context here
only when you are ready for that.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `mise: command not found` | mise not installed. | Install per [mise.jdx.dev](https://mise.jdx.dev/). On macOS: `brew install mise`. |
| `mise install` succeeds but `go version` shows the wrong version | `mise activate` not in your shell init. | Add `eval "$(mise activate zsh)"` (or `bash`) to your shell rc, then re-source. |
| `make: command not found` | macOS minimal install. | Install Xcode Command Line Tools: `xcode-select --install`. |
| `golangci-lint: command not found` on `make lint` | mise pin not installed. | `mise install` (the version is pinned in `mise.toml`). |
| `make fixtures-verify` shows drift on a fresh checkout | Implementation behavior diverged on `main`. | Open an issue — fixtures should be clean on `main` per CI. |
| `make ci` passes locally but fails on CI | Different Go version, OS, or linter rule version. | Compare versions: `go version` vs the GitHub Actions log. CI runs `stable` and `oldstable`; the mise pin is the oldest supported line. |
| `make ci` fails on `lint-ts` / `test-py` / … with `command not found` | A port's toolchain is not installed. | Install it (`pnpm`, `uv`, `cargo`, `php`+`composer`), or run `make ci-<lang>` for the one language you are changing. |
| `make test-parity` says a language `differs from go` | Two implementations disagree about a fixture. | The per-key diff names every differing fixture. Fix the port — or, if the reference is wrong, change it and regenerate every emitter. |
| `make test-parity` exits 2 | The harness could not reach a verdict: the Go reference failed, printed invalid JSON, or has no `LANGUAGES` entry. | Fix the reference first. Exit 2 is never a mismatch, and the harness never promotes another language to reference. |

## How it works

`go/` is a Go module with no `hop.top/kit` dependency (see
[CONTRIBUTING.md](../../CONTRIBUTING.md#no-hoptopkit-dependency)). The toolchain is
mise-managed so contributors and CI run identical Go versions
without per-machine drift.

`fixtures-verify` is the single-implementation integrity check: it
ensures the Go reference produces byte-identical canonical bytes
and hashes for every fixture, every commit.

`test-parity` is the cross-implementation one, and the two catch
different things. `fixtures-verify` can only tell you Go still
agrees with itself; a value wrong in the reference stays green
through it forever, because the corpus it diffs against was
generated from that same reference. `test-parity` runs all five
emitters over the same fixtures and diffs their parsed documents,
so a wrong value has to be wrong *identically* in five independent
codecs to survive. It compares parsed values rather than bytes —
reformatting an emitter's JSON output changes nothing.

There is no enable-list: `LANGUAGES` in `tools/parity/parity.py` is
the roster, and a language is compared because it has an entry. To
quarantine a broken emitter, delete or comment out its entry; the
run then prints one fewer language, which is visible in the diff
and in the harness' own output line. The Go reference is the
exception — removing its entry exits 2 rather than promoting
another language. See
[tools/parity/README.md](../../tools/parity/README.md) for the
normative emitter contract.

## Next steps

- [Add a feature track end-to-end](contributing-flow.md) — TDD
  posture, Conventional Commits, spec changes.
- [CONTRIBUTING.md](../../CONTRIBUTING.md) — repo-wide rules
  (licensing, commit style, reviewers).
- [spec/03 — canonicalization](../../spec/v0.1/03-canonicalization.md)
  — read the rules of record before proposing a change to them.
