# Add a feature track end-to-end

Land a new feature in the Go reference (`go/`) from RED test through GREEN
implementation, spec change (when architectural), and conformance fixture
updates.

## Use this when

- You are about to ship a non-trivial change to `go/` —
  anything that adds a public symbol, changes wire-form behavior,
  or touches the canonical/hash discipline.
- Your change crosses the spec/implementation boundary (touches
  `spec/` as well as the Go reference or the conformance corpus).
- You need to land a spec change alongside a code PR.

## Result

After completing this guide, you will:

- Have a feature branch with TDD-phase commits (RED → GREEN →
  REFACTOR).
- Pass `make ci` and `make fixtures-verify` locally.
- Land the change as a single PR covering spec, code, and fixtures
  together.

## Before you begin

You need:

- A working dev environment per [setup](setup.md).
- A clear statement of the user-visible behavior change. If you
  cannot describe it in one sentence, the scope is wrong.
- For architectural changes: a draft of the spec wording. See
  [spec/03](../../spec/v1.0/03-canonicalization.md)
  for the register the rules are written in.

## Quick version

```sh
git checkout -b feat/<area>-<short-summary>

# RED commit
go test ./<pkg> # fails
git add <pkg>/<feature>_test.go
git commit -m "test(<area>): <feature> — RED"

# GREEN commit
go test ./<pkg> # passes
git add <pkg>/<feature>.go
git commit -m "feat(<area>): <feature> — GREEN"

# REFACTOR commit (if cleanup needed)
git commit -am "refactor(<area>): tidy <feature> — REFACTOR"

# Coordinated update if fixtures or spec move
# author the fixture under spec/v1.0/conformance/, then:
make fixtures-verify  # regenerates the corpus + the go/testdata/ mirror
git add spec/ go/testdata/
git commit -m "feat(<area>): regenerate corpus"

make ci
gh pr create --base main
```

## Steps

### 1. Choose a Conventional Commit type

`vstar` uses [Conventional Commits](https://www.conventionalcommits.org/).
Allowed types (per [CONTRIBUTING.md](../../CONTRIBUTING.md)):

| Type | Use for |
|---|---|
| `feat` | New user-visible functionality. |
| `fix` | Bug fix. |
| `docs` | Docs-only change. |
| `style` | Formatting only, no code change. |
| `refactor` | Restructure without behavior change. |
| `perf` | Performance improvement. |
| `test` | Adding or revising tests. |
| `build` | Toolchain, deps, build scripts. |
| `ci` | GitHub Actions, CI infrastructure. |
| `chore` | Release plumbing, housekeeping. |

Optional scope is the area: `feat(rfc5545): ...`, `fix(canonical): ...`,
`ci(go): ...`. Use the imperative mood ("add", not "added"). Keep
the subject under 72 chars.

### 2. Write the RED test first

Every behavioral change lands as a sequence of TDD-phase commits.
The first commit is the failing test:

```sh
git add <pkg>/<feature>_test.go
git commit -m "test(<area>): <feature> — RED"
```

The commit message ends in `— RED`. The tree fails CI on this
commit by design. CI does not gate intermediate commits — it gates
the merge.

If the change is pure config or docs (no testable behavior), skip
RED and go straight to GREEN.

### 3. Implement the minimum to pass — GREEN

```sh
git add <pkg>/<feature>.go
git commit -m "feat(<area>): <feature> — GREEN"
```

Resist scope creep. Only add code the RED test exercises. Anything
else lands as a separate sequence (or a follow-up REFACTOR commit).

Run the package's tests to confirm green:

```sh
go test ./<pkg>
```

### 4. REFACTOR (optional)

If the GREEN implementation is messy — repeated branches, awkward
naming, helpers that want extracting — clean up in a third commit:

```sh
git commit -am "refactor(<area>): tidy <feature> — REFACTOR"
```

Tree must still pass. Commit message ends in `— REFACTOR`.

### 5. Land the spec change for architectural changes

Significant design choices — anything that constrains the canonical
form, the hash discipline, a public error sentinel, or a parser
scope boundary — are stated normatively under
[`spec/`](../../spec/). Edit the spec text in the same PR as the
code that depends on it, and write the rule the way
[spec/03](../../spec/v1.0/03-canonicalization.md)
already does: what an implementation MUST do, the accepted and
rejected shapes, and the fallback when input is out of scope.

If your change is purely additive (new helper, new optional flag,
new fixture without semantic shift), no spec change is required.

### 6. Update the conformance corpus

If your change affects canonical bytes or hash output for any
existing fixture, regenerate the corpus:

```sh
make fixtures-verify
```

The target regenerates every `.canonical` and `.hash` under
`spec/v1.0/conformance/`, mirrors the corpus into `go/testdata/`,
and diffs both trees. If either is dirty, the target fails —
commit the regenerated files in the same PR:

```sh
git add spec/v1.0/conformance/ go/testdata/ \
  go/codec/rfc5545/testdata/fuzz/ go/codec/rfc6350/testdata/fuzz/
git commit -m "feat(<area>): regenerate corpus for <change>"
```

If the change is intentional, the regeneration commit is part of
the PR. If unintentional, revert your implementation change — you
broke a canonical-form invariant.

### 7. Coordinate spec + fixture + implementation

Spec text, corpus and implementations all live in this repo, so
this is one PR. It must move all three together:

1. Add or modify the fixture under `spec/v1.0/conformance/` — the
   authored corpus.
2. Update the relevant section of the spec under `spec/v1.0/` to
   describe the new behavior.
3. Update each implementation — the Go reference and all four ports —
   to handle it; add tests that consume the new fixture. For an
   `rfc5545/` or `rfc6350/` fixture, record the Go encoder's bytes in
   each port's reference table (see the porting guide's
   [reference-bytes gate](porting-guide.md#pin-the-reference-encoders-bytes-not-just-the-round-trip));
   the port suites fail by name until the row exists.

Then run `make fixtures-verify` and commit the regenerated
`go/testdata/` mirror alongside.

A fixture earns its place by reaching a rule nothing else reaches.
Prove it by mutation: disable the rule in the reference and confirm
`make fixtures-verify` rewrites exactly that fixture's `.canonical`
and `.hash` siblings and no others. Every corpus blind spot found so
far — NFC, binary `ATTACH`, UTF-8 UID order, the fold boundary,
continuation width, TEXT escaping, authored list order — was a rule
every committed fixture satisfied by accident, so four ports could
skip it and stay byte-identical.

Splitting these across PRs leaves implementations divergent from
the spec — refused at review.

### 8. Run the full CI gate locally

```sh
make ci
```

Green locally is a prerequisite — CI runs the same target.

### 9. Open the PR

```sh
gh pr create --base main \
  --title "feat(<area>): <one-line summary>" \
  --body "$(cat <<'EOF'
## Summary

<2-3 bullets covering the user-visible change>

## Test plan

- [ ] make ci
- [ ] make fixtures-verify
- [ ] <any feature-specific exercise>

## Spec

Spec sections touched: <spec/v1.0/NN-*.md> (or: none — additive change).
EOF
)"
```

Reviewers: repository maintainers, per
[CONTRIBUTING.md](../../CONTRIBUTING.md).

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `make fixtures-verify` shows drift you didn't expect | Implementation behavior accidentally changed canonical bytes. | `git diff spec/v1.0/conformance/ go/testdata/` to see the byte-level difference; revert the implementation change unless the drift is the point of the PR. |
| `make fixtures-verify` names rewritten mirror paths but `git diff` is clean | Someone hand-edited `go/testdata/` and the mirror restored it — the tree is clean *because* the edit is gone. | Make the edit under `spec/v1.0/conformance/` instead and rerun; the report is the only trace a restored edit leaves. |
| RED commit fails CI and you cannot push | CI is running per-commit, not per-merge. | Check `.github/workflows/ci-go.yml` — vstar's CI gates the merge, not intermediate commits. If your fork enforces per-commit, squash before pushing. |
| Conventional Commit linter rejects your subject | Subject exceeds 72 chars or uses past tense. | Rewrite imperative + concise: "add X" not "added X with a long explanation". |
| Reviewer asks for a spec change on a change you thought was additive | The change touches the canonical/hash discipline or adds a public sentinel. | Land the spec change — public sentinels and canonical-form rules are architectural. See [spec/03 §RRULE parsing scope](../../spec/v1.0/03-canonicalization.md#rrule-parsing-scope) for how a scope boundary is stated. |
| `gofumpt` fails fmt-check | Formatter version mismatch with CI. | Match CI's gofumpt or run `make fmt` and commit. |

## How it works

The TDD posture (RED/GREEN/REFACTOR) is named in commit messages
so reviewers and `git log --oneline` readers can see the discipline
without opening diffs. Squash-on-merge collapses the trio to a
single commit on `main`, but the original sequence is preserved on
the feature branch.

`make fixtures-verify` is the cross-cutting integrity check.
Implementations may freely refactor as long as canonical bytes
stay byte-identical; any drift surfaces as a hard CI failure with
the diff visible. This is what keeps the Go reference, the four
ports and downstream cross-validators agreeing forever.

Two structural facts about the Go tests that are easy to get wrong
from inside `go/`:

- Tests that need a fixture parsed through `codec/rfc5545` live in
  the external package `vstar_test` (`go/time_fixture_test.go` and
  its siblings). `codec/rfc5545` imports the root package, so an
  internal test cannot import it back without a cycle; everything
  under test is exported, so nothing is lost.
- They read `go/testdata/`, never `spec/`. The published module is
  the `go/` subtree alone, with no sibling `spec/`, so a test that
  reached across would pass here and fail for anyone running
  `go test hop.top/vstar/...` from the module proxy. When a test
  parses a fixture rather than building a literal, also assert the
  parsed shape: fields that never reach the behaviour under test
  (`TZNAME`, `LAST-MODIFIED` on a VTIMEZONE) can vanish from the
  fixture with every behavioural assertion still green.

Spec changes are the mechanism for moving past "is this a refactor
or a decision?" Anything that constrains a future contributor —
wire formats, error sentinels, package boundaries, scope cuts — is
stated in the spec. Read
[spec/03 §RRULE parsing scope](../../spec/v1.0/03-canonicalization.md#rrule-parsing-scope)
for an example of a scope boundary with explicit accepted,
malformed, and deferred lists.

## Next steps

- [Set up the dev environment](setup.md) — toolchain prerequisites
  if you skipped past it.
- [CONTRIBUTING.md](../../CONTRIBUTING.md) — full repo-wide rules
  (licensing, reviewers, security disclosure).
- [spec/03 — canonicalization](../../spec/v1.0/03-canonicalization.md)
  — the register to model your own spec wording on.
