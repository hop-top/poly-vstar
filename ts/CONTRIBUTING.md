# Contributing to `@hop-top/vstar`

This package is developed in the polyglot monorepo
[hop-top/poly-vstar](https://github.com/hop-top/poly-vstar) under
`ts/`, and republished here — promoted to the repository root — on
each `vstar-ts/v*` tag. This repository is a read-only mirror: a pull
request opened here is overwritten by the next publish.

**Open issues and pull requests against
[hop-top/poly-vstar](https://github.com/hop-top/poly-vstar/issues).**

## Rules that apply everywhere

The repository-wide rules — licensing, Conventional Commits, the
RED / GREEN / REFACTOR posture, the shared conformance corpus — are
in the monorepo's
[CONTRIBUTING.md](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md).
The TypeScript conventions (pnpm, vitest, eslint with
typescript-eslint, `tsc` under `strict` with `NodeNext` resolution, the
Node 22 floor) are its
[per-language section](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md#per-language-conventions).

## Local loop

From a checkout of the monorepo, `make ci-ts` is this tree's gate
(lint, test, build). `make ci` is the full cross-language gate a pull
request must pass, including byte-for-byte parity of this port's
output against the Go reference over the whole corpus.

Inside this tree alone:

```sh
pnpm install --ignore-scripts
pnpm lint
pnpm test
pnpm run ci:build
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
