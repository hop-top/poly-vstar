# Contributing to `hop-top-vstar`

This crate is developed in the polyglot monorepo
[hop-top/poly-vstar](https://github.com/hop-top/poly-vstar) under
`rs/`, and republished here — promoted to the repository root — on
each `vstar-rs/v*` tag. This repository is a read-only mirror: a pull
request opened here is overwritten by the next publish.

**Open issues and pull requests against
[hop-top/poly-vstar](https://github.com/hop-top/poly-vstar/issues).**

## Rules that apply everywhere

The repository-wide rules — licensing, Conventional Commits, the
RED / GREEN / REFACTOR posture, the shared conformance corpus — are
in the monorepo's
[CONTRIBUTING.md](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md).
The Rust conventions (rustfmt, clippy with `-D warnings`,
`#![forbid(unsafe_code)]`, the MSRV in `Cargo.toml`, no new
dependency without an explicit reviewer approval) are its
[per-language section](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md#per-language-conventions).
Do not raise `rust-version` without a release note.

## Local loop

From a checkout of the monorepo, `make ci-rs` is this tree's gate
(format check, clippy, tests, build). `make ci` is the full
cross-language gate a pull request must pass, including byte-for-byte
parity of this port's output against the Go reference over the whole
corpus.

Inside this tree alone:

```sh
cargo fmt --check
cargo clippy --all-targets --all-features -- -D warnings
cargo test
cargo build
```

## Never hand-edited

`src/generated/` is rendered from the registry in the monorepo's
`spec/registry/` by `make registry-gen`; `make registry-check` fails
on drift.

## Changing what this port emits

This port is a round-trip implementation measured against the Go
reference. A change to canonical bytes, a hash, a diagnostic code or an
error class is a specification change: it lands in one pull request
with the spec text, the fixture that reaches the new rule, and every
implementation — see the monorepo's
[shared conformance corpus](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md#shared-conformance-corpus)
rules. The parity gate fails any port whose output diverges from the
reference, so a divergence is fixed in the reference or the spec, never
special-cased here.

## License

MIT — see [LICENSE](LICENSE).
