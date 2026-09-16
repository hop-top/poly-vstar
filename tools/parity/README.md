# Cross-language parity harness

Every language implementation of V\* ships a **parity emitter**: a
program that reads the `spec/` tree and prints one JSON document
describing what that implementation makes of every fixture. The
orchestrator, `parity.py`, runs each enabled emitter and diffs its
document against the Go reference. Any difference fails the run.

```sh
make test-parity
```

Go is the reference. It is named by `REFERENCE` in `parity.py` and is
never inferred from what happens to succeed: if the Go emitter fails,
prints invalid JSON, or prints a document that is not shaped like an
emitter's output, the harness exits **2** without comparing anything.
A mismatch between two working emitters exits **1**. Agreement exits
**0**.

`LANGUAGES` in `parity.py` is the roster: every entry is built, run
and compared. All five — `go`, `ts`, `py`, `rs`, `php` — are in it, so
every run is a genuine five-way cross-check and a wrong value has to
be wrong identically in five independent codecs to stay green. See
*Coverage limits*.

There is no separate enable-list. **To quarantine a broken emitter,
delete (or comment out) its `LANGUAGES` entry** — that removes it from
the gate, and the diff shows a language leaving rather than a
set-literal changing. Re-add the entry to bring it back. `REFERENCE`
is the exception: deleting the `go` entry is not quarantine, it leaves
the harness with nothing to compare against, and it exits **2** before
running any emitter.

## Emitter contract

This section is normative. A port implements this document, not the
Go emitter's source.

### Invocation

The emitter takes the absolute path to the `spec/` directory as its
single argument and prints one JSON document to standard output.

```sh
<emitter> /abs/path/to/spec
```

It exits `0` having printed the document, and non-zero with a
diagnostic on standard error if a fixture cannot be read or a
contract it depends on is broken. Nothing but the document is
printed to stdout.

### Top level

The document is an object with exactly these eight keys, all
required, each an object:

| Key | Fixture source |
|-----|----------------|
| `conformance` | `spec/v0.1/conformance/{rfc5545,rfc6350,supersession,malformed}/` |
| `rrule` | `spec/v0.1/conformance/rrule/**` |
| `validate` | `spec/behavior/validate/` |
| `diff` | `spec/behavior/diff/` |
| `supersession` | `spec/behavior/supersession/` (inputs from the conformance corpus) |
| `duration` | `spec/behavior/duration/` |
| `ext` | `spec/behavior/ext/` |
| `time` | `spec/behavior/time/` (registries from the conformance corpus) |

A key the emitter has no cases for is an empty object, never absent
and never null. A key not on this list is an unsupported document.

### Fixture keys

Every fixture key is the input file's path relative to its tree root
— `spec/v0.1/conformance` for `conformance` and `rrule`,
`spec/behavior` for the rest — with the extension dropped, using `/`
as the separator on every platform:

```text
rfc5545/world
malformed/missing_uid
rrule/evaluator/daily_first_5
validate/todo_missing_due
diff/value_changed            (from value_changed.a.ics / .b.ics)
duration/parse                (from parse.json)
ext/scopes                    (from scopes.json)
time/tzid                     (from tzid.json)
```

Fixture keys are discovered by walking the corpus, never hard-coded.
A fixture added to `spec/` must reach every emitter without an
emitter edit.

### Failures

A failure serializes as an object carrying an `error` field whose
value is a **token**: the identifier of the Go sentinel for that
failure class, used as the language-neutral name.

```json
{ "error": "ErrMalformed" }
```

The token names a class, not a message. An implementation satisfies
it by failing in that class, however it words the failure. **No
emitter emits a human-readable string anywhere** — no diagnostic
message, no exception text, no prose. Those reword between versions
without the behavior changing.

The tokens in use:

| Token | Class |
|-------|-------|
| `ErrMalformed` | structurally invalid input |
| `ErrUnclosedBlock` | a `BEGIN` with no matching `END` |
| `ErrUnsupportedVersion` | a `VERSION` outside the supported set |
| `ErrMissingUID` | a component requiring `UID` has none (encoder-only in v0.1) |
| `ErrUnsupportedRRule` | an RRULE feature outside the v0.2 scope |
| `ErrIterationCap` | the evaluator's iteration cap was hit |
| `ErrUnboundedExpansion` | expansion requested of an unbounded series |
| `ErrNoTrigger` | a VALARM with no `TRIGGER` |
| `ErrNoAnchor` | a relative trigger with no anchor to resolve against |
| `Unclassified` | the escape hatch — a failure matching no class above |

Classification is **first-match over a list ordered specific to
general**, not best-match. A failure that is both `ErrNoAnchor` and
`ErrMalformed` reports `ErrNoAnchor`. `ErrMalformed` is always tried
last.

`Unclassified` exists so an unrecognized failure surfaces under a
name no port implements — a loud mismatch — rather than silently
under a wrong token.

### Timestamps

Every timestamp is RFC 5545 form #2: UTC, `Z`-suffixed,
`YYYYMMDDTHHMMSSZ`. The one exception is the `value` column of
`time/tzid`, whose whole subject is form #1 (local, no `Z`) input.

### Determinism

Output must be byte-identical across runs. Object keys are sorted
(JSON encoders that sort keys do this for free; those that do not
must be made to). Every list is built in an order this document
pins. A list is never `null`: an empty result is `[]`, an empty map
is `{}`.

---

### `conformance`

One entry per fixture in `rfc5545/`, `rfc6350/`, `supersession/` and
`malformed/`.

Parseable fixtures — those with `.canonical` and `.hash` siblings:

```json
"rfc5545/one_vtodo": {
  "hash": "sha256:4eeee1227d0055f4804bd74d63136f9cf431713b7b3f59e22e2b40e1a1f30032",
  "canonical_sha256": "9d5014e8f09fbe236d7c63a299d05e3bdb3fe4916ca370477b05676952f91511"
}
```

* `hash` — parse the fixture, canonicalize, hash: the full
  `sha256:<64 hex>` content hash, exactly as the `.hash` sibling
  holds it minus the trailing newline.
* `canonical_sha256` — the SHA-256, lowercase hex without a prefix,
  of the canonical bytes **after folding CRLF to LF**. The canonical
  form per spec/03 is CRLF; the corpus stores it LF and every port
  reads the same LF file, so digesting the LF form keeps line-ending
  handling from producing false mismatches. The digest rather than
  the bytes keeps the document small and diffable — one wrong byte
  shows as a different digest, and the bytes are on disk in the
  `.canonical` sibling for whoever debugs it.

**Self-check (required).** Before emitting, compare the computed
`hash` against the committed `.hash` sibling and abort with a
diagnostic on any mismatch. This is the one family with an
independent expectation on disk, and it is what makes the reference
emitter falsifiable on its own — see *Coverage limits* below.

Fixtures under `malformed/` emit the error shape. The token is
derived from what the implementation actually does, never copied
from the `.error` sibling:

```json
"malformed/unclosed_block": { "error": "ErrUnclosedBlock" }
```

Malformed fixtures are two-stage. Parse the input; if it fails,
classify the parse failure. If it parses — which `missing_uid.vcf`
does, because `ErrMissingUID` is encoder-only in v0.1 — re-encode
every parsed card and classify the encode failure instead. A fixture
where both stages succeed is a fault.

`time/` and `fuzz-seed/` are not part of this family: the former are
VTIMEZONE registries the `time` family consumes, the latter are fuzz
inputs. Neither carries `.canonical`/`.hash`.

### `rrule`

One entry per fixture stem under `rrule/**`, keyed relative to
`spec/v0.1/conformance`. The entry's **keys are flat**, one group per
sidecar the fixture carries, so which contracts a fixture pins is
readable off its key set.

A fixture with an `.expect.json` sidecar must be rejected. Report the
class and stop — nothing else applies to a rule that does not parse:

```json
"rrule/rejected/freq_secondly": { "error": "ErrUnsupportedRRule" }
```

Otherwise the rule must parse, and the entry carries `"parsed":
true` plus one group per remaining sidecar:

| Sidecar | Keys added |
|---------|-----------|
| *(none)* | `"parsed": true` |
| `.formatted` | `"formatted": "FREQ=DAILY"` |
| `.next.json` | `"next": [...]` plus `"next_complete": <bool>` or `"next_error": "<token>"` |
| `.expand.json` | `"expand": [...]`, `"expand_complete": <bool>` — or `"expand_error": "<token>"` |
| `.between.json` | `"between": [...]` — or `"between_error": "<token>"` |
| `.occurrences.json` | `"occurrences": [...]`, `"occurrences_complete": <bool>` — or `"occurrences_error": "<token>"` |

```json
"rrule/evaluator/daily_count_terminates": {
  "next": ["20260402T120000Z", "20260403T120000Z"],
  "next_complete": true,
  "parsed": true
}
```

**Which call each sidecar pins.**

* `.formatted` — parse the rule, re-emit it in the fixed rule-part
  order with defaults elided.
* `.next.json` — step the next-occurrence call from `after`, once per
  entry of the sidecar's `expected` list, feeding each result back as
  the next `after`. **Then run one more step past the end**: that
  step's outcome is the terminal contract — `next_error` when it
  fails, `next_complete` when it returns cleanly (`true` if the
  series terminated, `false` if it yielded another occurrence). The
  extra step is unconditional: without it a series that should
  terminate and one that should hit `ErrIterationCap` would be
  indistinguishable, and a port could pass by stopping early. The
  sidecar's `expected` values are read **only** to learn how many
  steps to take; they are never emitted.
* `.expand.json` — bounded expansion from `dtstart` to the sidecar's
  `limit`. `expand_complete` is true only when the series ended
  within the limit.
* `.between.json` — every occurrence in the half-open window
  `[start, end)`. No completeness key: the window bounds the answer.
* `.occurrences.json` — parse the `.ics`, build a recurrence set from
  its **first component** (`DTSTART`, `RRULE`, `RDATE` merged,
  `EXDATE` removed), expand to `limit`.

For an `.ics` fixture with no `.occurrences.json`, the entry is
`{"parsed": true}` alone.

In every case the emitter reads **inputs** from the sidecar
(`dtstart`, `after`, `start`, `end`, `limit`) and reports **outputs**
from its own implementation. Copying a sidecar's `expected` into the
document would make the harness compare fixtures to themselves.

### `validate`

One entry per `<name>.ics`, the value a list of diagnostics.

```json
"validate/bad_hash": [
  { "code": "VS010", "severity": "error", "path": "VCALENDAR.VJOURNAL[uid=journal-bad-hash].X-VSTAR-HASH" }
]
```

* `code` — the catalog identifier from `spec/registry/`.
* `severity` — `"error"` or `"warning"`.
* `path` — the dotted locator: `VCALENDAR.<TYPE>[uid=<uid>]`, or
  `…[#<n>]` with no UID to key on, plus a trailing `.<PROPERTY>` for
  a property-level finding.

**Sorted by `path`, then `code`.** Check order is an implementation
detail and differs per port; the sorted form is the contract. A clean
document is `[]`.

### `diff`

One entry per `<name>.a.ics` / `<name>.b.ics` pair, keyed on the stem
with the side suffix dropped. The value is a list of per-component
change sets.

```json
"diff/param_changed": [
  {
    "uid": "todo-param",
    "path": "VCALENDAR.VTODO[uid=todo-param]",
    "ops": [
      {
        "op": "change",
        "property": "DUE",
        "before": "20260101T000000",
        "after": "20260101T000000",
        "before_params": [{ "name": "TZID", "value": "America/Montreal" }],
        "after_params": [{ "name": "TZID", "value": "Europe/Paris" }]
      }
    ],
    "subs": []
  }
]
```

* `uid` — lifted out of `path` so the common case needs no path
  parsing; `""` for a positionally-addressed component.
* `op` — `"add"`, `"remove"` or `"change"`, lowercase.
* `before` is `null` on an add, `after` is `null` on a remove.
* `before_params` / `after_params` — the property's parameters in the
  order the property carries them; `[]` when it has none. They travel
  apart from the value because a parameter-only change is a
  `"change"` op whose values are equal.
* `subs` — the same shape recursively for changed sub-components
  (a VALARM inside a VEVENT); `[]` when none. Sub-components that
  record no change are dropped.

**Not sorted.** Components come in pairing order and `ops` sorted by
property name case-insensitively; that ordering is itself the
contract. Re-sorting would hide an ordering divergence rather than
catch it. Unchanged components do not appear; `X-VSTAR-HASH` is
excluded from both sides.

### `supersession`

One entry per `<name>.effective.json`, keyed relative to
`spec/behavior`. The input `.ics` is the conformance corpus fixture
of the same stem under `spec/v0.1/conformance/supersession/`.

```json
"supersession/multi_step": { "todo-multi": "COMPLETED" }
```

A map from component UID to the status the ledger projects.
**A UID absent from the map is not superseded** — supersession is a
projection query, not a validator, so a broken hash with no ledger
entry targeting it yields `{}`.

### `duration`

Two shapes under one key.

`duration/parse` is a list, one row per value, **in the order
`parse.json` lists them**. The emitter reads the `value` column out
of the committed file and computes the rest, so a value added to the
corpus reaches every port without an emitter edit.

```json
"duration/parse": [
  { "value": "-PT15M", "seconds": -900, "negative": true, "error": "" },
  { "value": "P1W2D", "seconds": null, "negative": null, "error": "ErrMalformed" }
]
```

* `seconds` — the whole duration as a signed second count, the one
  representation every target language has; `null` on failure.
* `negative` — the sign flag, separate because a zero-length duration
  written with a leading sign cannot be distinguished by `seconds`
  alone (`-PT0S` normalizes to `"negative": false` — reproduce that,
  it is behavior, not rounding); `null` on failure.
* `error` — the class token, `""` on success.

Every other key is one `<name>.ics`, a list of alarm resolutions
**in document order**: parent components in calendar order, VALARMs
in the order they appear inside their parent.

```json
"duration/relative_start": [
  { "alarm_uid": "alarm-start-explicit", "fires_at": "20260601T084500Z", "error": "" },
  { "alarm_uid": "alarm-no-anchor", "fires_at": "", "error": "ErrNoAnchor" }
]
```

`fires_at` is `""` on failure, `error` is `""` on success. An
explicit `VALUE` parameter is authoritative: a contradicting value is
`ErrMalformed`, never silently re-read as the other form.

### `ext`

One key, `ext/scopes`, a list in the order `scopes.json` lists the
names. The emitter reads the `name` column and classifies each.

```json
"ext/scopes": [
  { "name": "X-VSTAR-HASH", "scope": "vstar", "system": null },
  { "name": "X-AGR-FOO", "scope": "system", "system": "AGR" }
]
```

* `scope` — `"vstar"`, `"system"`, `"experimental"`, `"none"` (not an
  extension) or `"unknown"` (has the `X-` prefix, matches no tier).
  **Lowercase**, so no port needs a case convention of its own.
* `system` — the owning system's slug; `null` for every scope but
  `"system"`.

Classification is case-insensitive. The reservation on `VSTAR` and
`EXP` covers the whole slug segment: `X-VSTARLIKE-FOO` is an ordinary
system extension owned by `VSTARLIKE`.

### `time`

One key, `time/tzid`, a list in the order `tzid.json` lists the rows.
The emitter reads the `calendar`, `tzid` and `value` columns and
resolves each.

```json
"time/tzid": [
  {
    "calendar": "america_montreal",
    "tzid": "America/Montreal",
    "value": "20260315T020000",
    "utc": "20260315T060000Z"
  }
]
```

`calendar` names the VTIMEZONE registry to load from
`spec/v0.1/conformance/time/<calendar>.ics`. `value` is form #1
(local, no `Z`) — the one place in the whole document it is.

**`utc` is `null` for every rejection**, and null is the whole
contract: the API reports a boolean, not an error, so there is no
class to name here. Reject an unknown zone, an empty TZID, a form #2
value (already absolute — resolving it would double-apply an
offset), a date-only value, and anything that is not a form #1
timestamp.

---

## Coverage limits

Be clear about what a green `make test-parity` proves.

**It proves** that every enabled language's emitter produces a
byte-identical document. With `n` languages enabled, that is a real
cross-check: for all of them to agree on a wrong value they would
have to be wrong identically.

**It does not prove the reference is right.** With only Go enabled
there is nothing to compare against, so a wrong value in the Go
emitter is simply the new truth and the harness stays green. This is
not a defect of the design — it is what "reference" means — but it
was a real gap while the ports were unwritten.

TypeScript closed it first, for every family; Python, Rust and PHP
followed. Five independent implementations — different languages,
codecs and evaluators — now produce the same document byte for
byte, so a wrong value has to be wrong identically in all of them to
survive the run.

The `conformance` self-check narrows that gap for the one field with
an independent expectation on disk: the emitted `hash` must equal the
committed `.hash` sibling, which `make fixtures-verify` generates and
CI already guards. A reference emitter that computes the wrong hash
aborts with exit 2 rather than emitting it.

`canonical_sha256` has no such anchor — the corpus commits the
canonical bytes, not a digest of them — so the self-check does not
cover it. It is guarded instead by `make fixtures-verify`, which
regenerates every `.canonical` sibling from the reference and fails
on drift; a Go emitter that canonicalized wrongly would have to fail
that target first.

The remaining families have no anchor at all. Their inputs come from
`spec/behavior/`, which is itself generated from the Go reference, so
comparing emitted values back to those fixtures would be the
reference agreeing with itself. Those families are guarded by the Go
unit tests and by `make fixtures-verify`, and they gain a genuine
cross-check from the second emitter, which TypeScript now supplies.

## Adding a language

1. Write the emitter per the contract above, reading `spec/` and
   printing the document.
2. Add that language's `LANGUAGES` entry in `parity.py` — the build
   command (or `None`), the run command with `{spec}` where the spec
   path goes, the working directory, any environment. The entry is
   what enrolls it; there is no second list to update.
3. Run `make test-parity` and work through the per-key diff until it
   is green.
4. The `ci-parity.yml` path filter already covers `ts/`, `py/`, `rs/`
   and `php/`; add the language's toolchain setup step to that
   workflow, and its cache keyed on the port's lockfile.

### What the existing `build` entries are for

The `build` command exists because a clean checkout is what CI runs
the harness on, and some emitters cannot run from one:

- **PHP** builds with `composer install`. The emitter loads the port
  through composer's autoloader, so `php/vendor/` must exist, and
  nothing else in the harness creates it.
- **Rust** builds the `parity` binary. It carries its own JSON
  reader and writer rather than `serde_json`: the library has no
  JSON need, so a `[dependencies]` entry would burden every
  consumer, and a feature gate is unusable because the harness runs
  `cargo run --quiet --bin parity` with no `--features`.
- **TypeScript** compiles the emitter through its own tsconfig
  project into `parity-dist/`, not `dist/`. `tsconfig.build.json` is
  rooted at `src/` and cannot emit a path outside it, and `files` in
  `package.json` ships the whole of `dist/` — emitting there put the
  emitter plus a second copy of every module into the published
  tarball. Interpreters (tsx, ts-node) were rejected because each
  adds a devDependency CI would have to install before the harness
  could run.
- **Go** and **Python** need no build: `go run` compiles, and the
  Python emitter is standard library like the orchestrator, which is
  also why `ci-parity.yml` has no `setup-python` step.

Every emitter sorts object keys on the way out. Go's `encoding/json`
sorts map keys; `JSON.stringify`, `json_encode` and a hand-rolled
writer do not unless told to, and PHP additionally casts every family
to an object so an empty family renders as `{}` rather than `[]`.
