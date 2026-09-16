# Contributing to `hop.top/vstar`

This module is developed in the polyglot monorepo
[hop-top/poly-vstar](https://github.com/hop-top/poly-vstar) under
`go/`, and republished here — promoted to the repository root — on
each `vstar/v*` tag. This repository is a read-only mirror: a pull
request opened here is overwritten by the next publish.

**Open issues and pull requests against
[hop-top/poly-vstar](https://github.com/hop-top/poly-vstar/issues).**

## Rules that apply everywhere

The repository-wide rules — licensing, Conventional Commits, the
RED / GREEN / REFACTOR posture, the shared conformance corpus — are
in the monorepo's
[CONTRIBUTING.md](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md).
The Go conventions (gofumpt, golangci-lint, the `go 1.25` floor, no
dependency beyond the standard library and `golang.org/x/text`) are its
[per-language section](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md#per-language-conventions).

## Local loop

From a checkout of the monorepo, `make ci-go` is this tree's gate
(vet, format check, lint, tests with `-race` and coverage). `make ci`
is the full cross-language gate a pull request must pass, including
parity of this module's output — which every port is measured
against — over the whole corpus.

Inside this tree alone:

```sh
go vet ./...
go test ./...
```

## Never hand-edited

- `validate/codes_gen.go` is rendered from the registry in the
  monorepo's `spec/registry/` by `make registry-gen`;
  `make registry-check` fails on drift.
- `testdata/` is the generated mirror of the conformance corpus, kept
  in sync by `make fixtures-verify`, so this module tests without the
  monorepo present.

## Changing what this module emits

This module is the V\* reference implementation. A change to canonical
bytes, a hash, a diagnostic code or an error class is a specification
change: it lands in one pull request with the spec text, the fixture
that reaches the new rule, and every port — see the monorepo's
[shared conformance corpus](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md#shared-conformance-corpus)
rules. A value wrong here is corrected here and every port regenerated,
never the other way round.

## License

MIT — see [LICENSE](LICENSE).
