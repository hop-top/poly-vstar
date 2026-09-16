# Contributing to `hop-top/vstar`

This package is developed in the polyglot monorepo
[hop-top/poly-vstar](https://github.com/hop-top/poly-vstar) under
`php/`, and republished here — promoted to the repository root — on
each `vstar-php/v*` tag. Packagist reads the package from this
repository, but it is a read-only mirror: a pull request opened here is
overwritten by the next publish.

**Open issues and pull requests against
[hop-top/poly-vstar](https://github.com/hop-top/poly-vstar/issues).**

## Rules that apply everywhere

The repository-wide rules — licensing, Conventional Commits, the
RED / GREEN / REFACTOR posture, the shared conformance corpus — are
in the monorepo's
[CONTRIBUTING.md](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md).
The PHP conventions (PSR-12 via php-cs-fixer, PHPStan at level 9,
PHPUnit, `declare(strict_types=1)`) are its
[per-language section](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md#per-language-conventions).
The floor is PHP 8.2 and CI tests 8.2 and 8.4: use no syntax newer
than 8.2, and resolve `composer.json` against the platform floor in its
`config.platform.php`, not against your local runtime.

## Local loop

From a checkout of the monorepo, `make ci-php` is this tree's gate
(static analysis, style check, tests, manifest validation). `make ci`
is the full cross-language gate a pull request must pass, including
byte-for-byte parity of this port's output against the Go reference
over the whole corpus.

Inside this tree alone:

```sh
composer install
vendor/bin/phpstan analyse
vendor/bin/php-cs-fixer check
vendor/bin/phpunit
composer validate --strict
```

## Never hand-edited

`src/Generated/` is rendered from the registry in the monorepo's
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
