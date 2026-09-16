# First publish of the four ports

Claim `@hop-top/vstar` on npm, `hop-top-vstar` on PyPI and crates.io,
and `hop-top/vstar` on Packagist — then hand the registries back to
CI forever.

## Use this when

- No version of a port has ever been published, and CI's publish job
  cannot create the name for you.
- `publish.yml` failed on a port's very first tag with a 404, a
  "package not found", or a scope/permission error that looks wrong
  given the secret is set.

This is a one-time procedure per registry. Once a name exists, every
later version ships through `publish.yml` with no human in the loop.

## Result

After completing this guide:

- Each of the four package names exists on its registry, owned by the
  maintainer account.
- npm publishes via OIDC from this repo's `publish.yml`, with sigstore
  provenance and no token.
- PyPI, crates.io and Packagist publish from CI using per-project
  credentials in the org secrets.
- The four port mirrors exist under `hop-top/`, created by the mirror
  job on each port's first tag.

## Why the first publish is manual

Every registry draws the same line: a scoped or project-scoped
credential may publish a **new version of an existing package**, but
may not **create a package that does not exist yet**. The tokens CI
holds are deliberately on the weak side of that line, so the blast
radius of a leaked CI secret is "one more version of something we
already ship", never "a brand-new package under our name".

So the first publish needs a stronger credential, used once,
interactively, from the maintainer's machine — and then downgraded.

## ⚠️ What is irreversible

**A version number, once published, can never be reused.** npm, PyPI
and crates.io all refuse to re-upload a version even after a yank,
unpublish or delete. If `1.0.0-alpha.1` goes out with a broken build,
that number is burned; the fix ships as `1.0.0-alpha.2`.

Concretely, the points of no return are:

| Step | Irreversible from |
|---|---|
| `pnpm publish` (npm) | the moment the upload returns 200 |
| `twine upload` (PyPI) | the moment the upload returns 200 |
| `cargo publish` (crates.io) | the moment the upload returns 200 |
| Packagist submit | reversible — a package can be deleted and re-submitted |
| Merging a release PR | reversible until the tag fires `publish.yml` |

Everything before the upload — building, packing, inspecting the
tarball — is free. Do all of it first.

## Before you begin

You need:

- `make ci` green on `main` (below).
- An npm account with publish rights on the `@hop-top` scope and 2FA
  enabled.
- A PyPI account that owns, or can claim, `hop-top-vstar`.
- A crates.io account with a **verified email** (crates.io rejects
  publishes from unverified accounts) and an unrestricted API token
  held outside CI.
- A Packagist account matching the `PACKAGIST_USERNAME` org secret.
- Toolchains from `mise install` — `pnpm`, `uv`, `cargo`.

Account state is the one thing this runbook cannot verify for you.
Confirm each login before starting; a half-finished publish is worse
than a late one.

## Preflight

Run both. Neither changes anything.

```sh
make ci
```

The full matrix: registry check, fixture drift, all five language
gates, and the parity harness. It must end on the parity line with no
diff. A port that is not byte-identical to the Go reference has no
business on a registry.

```sh
gh secret list --org hop-top
```

Expected, all with `ALL` visibility:
`NPM_REGISTRY_TOKEN`, `PYPI_REGISTRY_TOKEN`, `CARGO_REGISTRY_TOKEN`,
`PACKAGIST_USERNAME`, `PACKAGIST_TOKEN`, `GH_MIRROR_PAT`,
`RELEASE_BOT_APP_ID`, `RELEASE_BOT_PRIVATE_KEY`.

These are what CI uses *after* the bootstrap. If one is missing, fix
it before publishing anything by hand — otherwise the first CI
publish fails on a package you have already claimed, and you burn a
version number diagnosing it.

Confirm the names are still unclaimed:

```sh
curl -sI https://registry.npmjs.org/@hop-top%2fvstar | head -1
curl -sI https://pypi.org/pypi/hop-top-vstar/json         | head -1
curl -sI https://repo.packagist.org/p2/hop-top/vstar.json | head -1
```

A 404 on each means the name is free. For crates.io use the web UI —
the unauthenticated API answers 403 regardless of whether the crate
exists, so it is not a usable signal.

## Who does what

| Registry | Human does once | CI does forever after |
|---|---|---|
| npm | `npm login`, `pnpm publish`, bind trusted publisher | `publish-ts` publishes via OIDC |
| PyPI | account-token `twine upload`, then swap to a project token | `publish-py` uploads with `PYPI_REGISTRY_TOKEN` |
| crates.io | `cargo publish` with an unrestricted token | `publish-rs` publishes with `CARGO_REGISTRY_TOKEN` |
| Packagist | submit the mirror URL (after its first mirror push) | `publish-php` pings Packagist to re-index |
| Go | nothing | the module proxy serves the tag off the mirror |

The split matters: CI never creates a package, and you never publish
a routine version. If you find yourself running `pnpm publish` for a
second time, something upstream is broken — fix that instead.

## npm — `@hop-top/vstar`

```sh
cd ts
npm login
pnpm install --ignore-scripts
pnpm build
pnpm pack                       # inspect the tarball before it is final
pnpm publish --access public
```

`--access public` is required only for this first upload; npm defaults
new scoped packages to restricted. After this, access is a server-side
property of the package. (`ts/package.json` also sets
`publishConfig.access = public`, so the flag is belt-and-braces.)

`pnpm build` runs `tsc --build tsconfig.build.json` and populates
`dist/`, which is what `files` ships. Publishing without it produces a
tarball with no `dist/` — and that version number is then spent.

Then bind trusted publishing immediately:

```sh
npm trust github @hop-top/vstar \
  --repo hop-top/poly-vstar \
  --file publish.yml \
  --allow-publish --yes

npm trust list @hop-top/vstar
```

Expect `type: github`, repo `hop-top/poly-vstar`, `file: publish.yml`.

Four things bite here:

- **`--file` takes a bare filename**, not a path. `.github/workflows/publish.yml`
  fails with "workflow must be just a file not a path".
- **Bind `publish.yml`, the caller — never the reusable `publish-ts.yml`.**
  GitHub's OIDC `workflow_ref` claim names the workflow that was
  triggered, which is this repo's `publish.yml`.
- **Bind `hop-top/poly-vstar`, not the `vstar-ts` mirror.** The publish
  job checks out *this* repo at the tag and publishes from `ts/`; the
  mirror is a downstream copy that never publishes.
- **`E409 Conflict` means the binding already exists.** Confirm with
  `npm trust list`; do not retry.

`E401` → run `npm login` again. `EOTP` → append `--otp <code>`.

Once bound, `NPM_REGISTRY_TOKEN` becomes a dormant fallback: OIDC wins
when both are configured. Leave the secret in place.

> **Inconsistency worth knowing about.** `ts/package.json` declares
> `repository.url` as `hop-top/vstar-ts` (the mirror), while the
> publish runs from `hop-top/poly-vstar` (the source). Today this is
> cosmetic: the CI publish is token-authenticated and passes no
> `--provenance`, so no sigstore bundle is generated and npm never
> validates `repository` against the publishing repo. The npm page
> will simply link at the mirror.
>
> It stops being cosmetic the moment provenance is switched on: npm
> validates the bundle against `repository`, and a mismatch is
> rejected with `E422 ... Error verifying sigstore provenance
> bundle`. Before enabling provenance, point `repository.url` at
> `git+https://github.com/hop-top/poly-vstar.git` and keep
> `"directory": "ts"`.

## PyPI — `hop-top-vstar`

Project-scoped tokens cannot create a project; by definition they act
only on projects that already exist. Bootstrap with an account-scoped
token, then throw it away.

1. Mint a token with scope **Entire account** at
   <https://pypi.org/manage/account/token/>.

2. Build and upload:

   ```sh
   cd py
   uv build
   uv run twine upload dist/*
   ```

   `uv build` uses the hatchling backend declared in `py/pyproject.toml`
   and writes both an sdist and a wheel to `py/dist/`. Upload both.

3. **Delete the account-scoped token immediately.** An account-wide
   token can publish to every project you own; it exists for one
   upload and no longer.

4. Mint a token scoped to the `hop-top-vstar` project and set it as
   the `PYPI_REGISTRY_TOKEN` org secret (or confirm the existing
   secret is already project-scoped to it).

CI auth for this repo is **`pypi-auth: token`**, set on the `vstar-py`
entry in `.github/workflows/publish.yml`. That means step 4 is the
whole CI story — no GitHub Environment, no pending trusted publisher
to pre-register on pypi.org. Do not add an OIDC binding on the
assumption that it is the better path here; the token path is the one
this repo is wired for, and switching requires editing `publish.yml`
and creating the `pypi` Environment.

## crates.io — `hop-top-vstar`

Post-2023 crates.io tokens carry crate-name scoping. A scoped token
cannot publish a name it does not already include, so the first
publish needs an unrestricted token or a local `cargo login` session.

```sh
cd rs
cargo publish --dry-run     # packages and compiles, uploads nothing
cargo publish
```

`cargo publish` runs the build itself and refuses on a dirty tree, so
commit or clean first. Use the unrestricted bootstrap token from your
keyring — **never** put an unrestricted token in CI secrets: anyone
who can open a pull request against this repo can propose a workflow
that reads them, and an unrestricted token can publish to any crate
name on the account.

After the crate exists, mint a token scoped to `hop-top-vstar` and set
it as `CARGO_REGISTRY_TOKEN`.

crates.io also rejects publishes from accounts with an unverified
email, with a message that does not obviously say so. Verify the email
before the upload, not after.

## Packagist — `hop-top/vstar`

Packagist is last, and not by preference: it indexes a **git repository
URL**, and the URL it needs is the `hop-top/vstar-php` mirror, which
does not exist until the mirror job has run at least once. So the PHP
sequence is inverted relative to the other three.

1. Merge the `vstar-php` release PR. The tag fires `publish.yml`,
   whose mirror job creates `hop-top/vstar-php` (via `GH_MIRROR_PAT`)
   and pushes the `php/` subtree to it.

2. The `publish-php` job then tries to notify Packagist. On this first
   run the package is not registered yet, so treat a failure there as
   expected.

3. Submit <https://github.com/hop-top/vstar-php> at
   <https://packagist.org/packages/submit>, signed in as the account
   behind `PACKAGIST_USERNAME`.

4. Re-run the failed `publish-php` job, or simply let the next release
   pick it up — from here CI's notify step keeps Packagist in sync.

Unlike the other three, this one is recoverable: a Packagist package
can be deleted and re-submitted.

## The release PRs, and why order matters

release-please keeps one open pull request per component, each titled
`chore(release): <component> <version>`. As of this writing there are
six open — `vstar` (Go), `vstar-ts`, `vstar-py`, `vstar-rs`,
`vstar-php`, and `vstar-spec` (the spec tree). Read the actual titles
before you start; the set and the version they propose both move as
commits land:

```sh
gh pr list --repo hop-top/poly-vstar --label status:release-pending
```

**Merge them one at a time. Wait for `publish.yml` to finish before
merging the next.**

The reason is `.github/.release-please-manifest.json`. It is a single
JSON object holding every component's current version, and all six
branches propose an edit to it. Merging one advances the file on
`main`; every sibling branch is now based on a manifest that no longer
exists, and GitHub marks them `CONFLICTING`. Merging in parallel does
not avoid this — it just means you discover five broken branches at
once instead of fixing one at a time.

This is a property of a shared manifest, not a race you can win by
merging faster. release-please will not rebase a stale branch on its
own; the manifest key it proposes "remained the same", so it sees
nothing to do.

When a sibling does go stale, rebase it by hand — the conflict is one
line of JSON and the merge is mechanical:

```sh
git fetch origin
git checkout -b rebase-<component> \
  origin/release-please--branches--main--components--<component>
git rebase origin/main
# Conflict in .github/.release-please-manifest.json:
# keep BOTH sides' keys in one object.
git add .github/.release-please-manifest.json
git rebase --continue
git push origin \
  rebase-<component>:release-please--branches--main--components--<component> \
  --force-with-lease
```

Then merge that PR and move to the next. Closing and reopening release
PRs to force a refresh is the slower path and burns PR numbers.

### Suggested order

1. **`vstar-rs`** — self-contained; `cargo publish` either works or
   fails loudly, and nothing downstream depends on it.
2. **`vstar-py`** — validates that the project-scoped
   `PYPI_REGISTRY_TOKEN` works on the project you just created.
3. **`vstar-ts`** — first exercise of the OIDC binding; the provenance
   gap noted above surfaces here if it is going to.
4. **`vstar-php`** — last, because Packagist registration depends on
   the mirror this merge creates.

The Go (`vstar`) and spec (`vstar-spec`) components need no registry
bootstrap and can be merged whenever; they still take a turn in the
one-at-a-time queue because they touch the same manifest.

## After each merge

Watch the tag's workflow before merging the next PR:

```sh
gh run list --repo hop-top/poly-vstar --workflow publish.yml --limit 5
gh run watch --repo hop-top/poly-vstar <run-id>
```

Green means: the port published, the mirror exists, and the manifest
on `main` has advanced. Only then move on.

If a publish job fails, note that **re-running it uses the workflow
file as it existed at the tag**, not the current `main`. A fix to
`publish.yml` on `main` has no effect on a re-run of an old tag — the
fix has to land and then be picked up by a new tag.

## Stop points

Stop and reassess — do not push through — if any of these happen:

- `make ci` is not green. A parity failure means the ports disagree;
  publishing that is worse than publishing nothing.
- A registry reports the name as already taken by someone else.
- A publish returns a success you did not expect (for example, an
  upload that "worked" before you ran the build). Check what actually
  shipped before publishing anything else.
- Two release PRs merged before you noticed. Do not merge a third;
  rebase the remaining branches first.
- The npm binding does not appear in `npm trust list`. Fix it before
  the next tag, or CI falls back to the token path and hits the 2FA
  wall.

## See also

- [Setup the dev environment](setup.md) — toolchains and `make ci`.
- [Add a feature track end-to-end](contributing-flow.md) —
  Conventional Commits, which is what feeds release-please's version
  bumps.
