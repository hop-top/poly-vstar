#!/usr/bin/env python3
"""Cross-language parity harness for the V* spec corpus.

Every language port ships an emitter that reads ``spec/`` and prints one
JSON document describing what its implementation makes of every fixture.
Go is the reference; every other language in ``LANGUAGES`` is diffed
against it, key by key, and any difference fails the run.

The emitter contract -- every key, every value shape, every ordering
rule -- is documented in ``tools/parity/README.md``. That document is
normative; this script only runs the emitters and compares.

``LANGUAGES`` is the single roster: a language is compared because it
has an entry, not because a second list also names it. To quarantine a
broken emitter, delete (or comment out) its ``LANGUAGES`` entry -- that
is the whole mechanism, and it is deliberately loud, because the diff
shows a language leaving the gate rather than a set-literal changing.
``REFERENCE`` can never be quarantined this way: removing it raises
before any emitter runs.

Exit codes:

* ``0`` -- every language agrees with the reference.
* ``1`` -- at least one language differs from the reference.
* ``2`` -- the harness itself could not produce a verdict: the
  reference emitter failed, emitted invalid JSON, emitted an
  unsupported document, or ``REFERENCE`` has no ``LANGUAGES`` entry. A
  missing reference is never resolved by promoting another language.

Python 3.11 standard library only, matching ``tools/registry/gen.py``:
the harness must run on a bare CI runner before any port's toolchain is
installed.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[2]
SPEC = ROOT / "spec"

# Every top-level key an emitter must produce. A document missing one,
# or carrying one not listed here, is structurally wrong rather than
# merely divergent -- the harness reports it as an unsupported document
# and, for the reference, exits 2.
REQUIRED_KEYS = (
    "conformance",
    "rrule",
    "validate",
    "diff",
    "supersession",
    "duration",
    "ext",
    "time",
)

# LANGUAGES is the roster: every entry is built, run and compared. It
# declares how to build and run each port's emitter.
#
# * ``build`` -- an optional command run before the emitter, for
#   languages that compile ahead of time. ``None`` for interpreted ones.
# * ``run`` -- the emitter command. ``{spec}`` is substituted with the
#   absolute path to ``spec/``.
# * ``cwd`` -- working directory, relative to the repo root.
# * ``env`` -- environment overlaid on the caller's.
#
# Adding a port is adding an entry; quarantining one is deleting its
# entry. There is no second enable-list to keep in step -- one that
# drifted out of step with this table would silently stop comparing a
# language that looks, from this table alone, fully wired up.
LANGUAGES: dict[str, dict[str, Any]] = {
    "go": {
        "build": None,
        # `go run` resolves the package against the module in `cwd`, and
        # the emitter takes the spec dir as its single argument.
        "run": ["go", "run", "./tools/parity", "{spec}"],
        "cwd": "go",
        "env": {},
    },
    "ts": {
        # The emitter is compiled, not interpreted: shipping it as
        # `tsc` output rather than running the .ts directly is what
        # keeps a TypeScript runner (tsx, ts-node) out of the port's
        # devDependencies and off this harness' critical path.
        #
        # `ci:build:parity` installs (--ignore-scripts, as the
        # Makefile's port targets do) and then drives
        # tsconfig.parity.json. The package's own build
        # (tsconfig.build.json) cannot compile the emitter: it is
        # rooted at src/ and tools/parity.ts sits outside that root.
        #
        # The output lands in parity-dist/ rather than dist/ because
        # package.json ships the whole of dist/ -- emitting there would
        # publish the emitter and a duplicate copy of every module to
        # npm. Install is part of the script rather than a second entry
        # here because `build` is one command, run without a shell.
        "build": ["pnpm", "run", "ci:build:parity"],
        "run": ["node", "parity-dist/tools/parity.js", "{spec}"],
        "cwd": "ts",
        "env": {},
    },
    "py": {
        "build": None,
        "run": [sys.executable, "tools/parity.py", "{spec}"],
        "cwd": "py",
        "env": {"PYTHONPATH": str(ROOT / "py" / "src")},
    },
    "rs": {
        "build": ["cargo", "build", "--quiet", "--bin", "parity"],
        "run": ["cargo", "run", "--quiet", "--bin", "parity", "--", "{spec}"],
        "cwd": "rs",
        "env": {},
    },
    "php": {
        # The emitter is interpreted, but it loads the port through
        # composer's autoloader, so `vendor/` has to exist before it
        # runs. Nothing else in the harness creates it -- a clean
        # checkout that has never run the PHP suite has no vendor tree
        # -- so the install is the build step.
        "build": ["composer", "install", "--no-interaction", "--no-progress", "--quiet"],
        "run": ["php", "tools/parity.php", "{spec}"],
        "cwd": "php",
        "env": {},
    },
}

# The reference every other language is diffed against. Never inferred
# from what happens to run: a reference that fails is a hard stop, and
# a REFERENCE with no LANGUAGES entry is a harness fault (exit 2), not
# an invitation to promote whichever port did run.
REFERENCE = "go"


class HarnessError(Exception):
    """A fault that prevents a verdict rather than being one."""


class MismatchError(Exception):
    """At least one language differs from the reference."""


def main() -> int:
    try:
        outputs = collect()
    except HarnessError as exc:
        print(f"parity: {exc}", file=sys.stderr)
        return 2

    reference = outputs[REFERENCE]
    mismatched = []
    for language in sorted(outputs):
        if language == REFERENCE:
            continue
        if outputs[language] != reference:
            mismatched.append(language)
            print(f"parity: {language} differs from {REFERENCE}", file=sys.stderr)
            report_diff(reference, outputs[language], [])

    if mismatched:
        print(
            "parity: "
            + ", ".join(mismatched)
            + f" must match the {REFERENCE} reference; fix the port, "
            "or change the reference and regenerate every emitter",
            file=sys.stderr,
        )
        return 1

    languages = " ".join(sorted(outputs))
    print(f"parity: ok {languages} ({count_cases(reference)} cases)")
    return 0


def collect() -> dict[str, Any]:
    """Run every emitter in LANGUAGES and return its parsed document.

    Raises HarnessError when the harness cannot reach a verdict: a
    reference with no LANGUAGES entry, or a reference emitter that
    failed, emitted invalid JSON or emitted an unsupported document. A
    non-reference emitter failing the same way is also a HarnessError --
    a port whose emitter will not run has not been shown to differ, and
    calling that a mismatch would misreport why the run is red.
    """
    # Quarantining a port is deleting its LANGUAGES entry; deleting the
    # reference's entry is not quarantine, it is a harness with nothing
    # to compare against, so it stops here at exit 2 rather than
    # promoting whichever port happens to remain.
    if REFERENCE not in LANGUAGES:
        raise HarnessError(
            f"reference {REFERENCE!r} has no LANGUAGES entry; "
            "the harness never promotes another language to reference"
        )
    if not SPEC.is_dir():
        raise HarnessError(f"no spec directory at {SPEC}")

    outputs: dict[str, Any] = {}
    # The reference runs first so its failure is reported before any
    # port's, which is the more useful diagnostic when both are broken.
    for language in [REFERENCE] + sorted(set(LANGUAGES) - {REFERENCE}):
        document = run_emitter(language, LANGUAGES[language])
        check_document(language, document)
        outputs[language] = document
    return outputs


def run_emitter(language: str, config: dict[str, Any]) -> Any:
    """Build (if needed) and run one emitter, returning its document."""
    cwd = ROOT / config["cwd"]
    env = os.environ.copy()
    env.update(config["env"])

    if config["build"]:
        build = subprocess.run(  # noqa: S603 - fixed command from LANGUAGES
            config["build"],
            cwd=str(cwd),
            env=env,
            text=True,
            capture_output=True,
            check=False,
        )
        if build.returncode != 0:
            raise HarnessError(
                describe_failure(f"{language} build", config["build"], build)
            )

    command = [arg.format(spec=str(SPEC)) for arg in config["run"]]
    proc = subprocess.run(  # noqa: S603 - fixed command from LANGUAGES
        command,
        cwd=str(cwd),
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if proc.returncode != 0:
        raise HarnessError(describe_failure(f"{language} emitter", command, proc))
    try:
        return json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        raise HarnessError(
            f"{language} emitter printed invalid JSON: {exc}\n{proc.stdout[:2000]}"
        ) from exc


def describe_failure(what: str, command: list[str], proc: subprocess.CompletedProcess) -> str:
    parts = [f"{what} failed (exit {proc.returncode}): {' '.join(command)}"]
    for stream in (proc.stdout, proc.stderr):
        if stream and stream.strip():
            parts.append(stream.rstrip())
    return "\n".join(parts)


def check_document(language: str, document: Any) -> None:
    """Reject a document that is not shaped like an emitter's output."""
    if not isinstance(document, dict):
        raise HarnessError(
            f"{language} emitter printed a {type(document).__name__}, expected an object"
        )
    missing = [key for key in REQUIRED_KEYS if key not in document]
    extra = sorted(set(document) - set(REQUIRED_KEYS))
    problems = []
    if missing:
        problems.append("missing key(s): " + ", ".join(missing))
    if extra:
        problems.append("unsupported key(s): " + ", ".join(extra))
    for key in REQUIRED_KEYS:
        if key in document and not isinstance(document[key], dict):
            problems.append(f"{key} is a {type(document[key]).__name__}, expected an object")
    if problems:
        raise HarnessError(
            f"{language} emitter printed an unsupported document: " + "; ".join(problems)
        )


def report_diff(reference: Any, candidate: Any, path: list[str]) -> None:
    """Print the differing keys, deepest-first, one line each.

    Every differing key is reported rather than only the first: a port
    under development usually differs in a family at a time, and seeing
    the whole family is what makes the next fix obvious.
    """
    where = ".".join(path) or "<root>"
    if type(reference) is not type(candidate):
        print(
            f"  {where}: {REFERENCE} has {type(reference).__name__}, "
            f"candidate has {type(candidate).__name__}",
            file=sys.stderr,
        )
        return
    if isinstance(reference, dict):
        for key in sorted(set(reference) | set(candidate)):
            if key not in candidate:
                print(f"  {'.'.join(path + [key])}: missing from candidate", file=sys.stderr)
            elif key not in reference:
                print(f"  {'.'.join(path + [key])}: absent from {REFERENCE}", file=sys.stderr)
            elif reference[key] != candidate[key]:
                report_diff(reference[key], candidate[key], path + [key])
        return
    if isinstance(reference, list):
        if len(reference) != len(candidate):
            print(
                f"  {where}: {len(reference)} item(s) in {REFERENCE}, "
                f"{len(candidate)} in candidate",
                file=sys.stderr,
            )
            return
        for index, (left, right) in enumerate(zip(reference, candidate)):
            if left != right:
                report_diff(left, right, path + [str(index)])
        return
    print(f"  {where}: {reference!r} != {candidate!r}", file=sys.stderr)


# The flat tables: one fixture file holding many independent rows, each
# of which is its own case. Counting these files as one case apiece
# would understate what the harness actually compares.
FLAT_TABLES = ("ext/scopes", "time/tzid", "duration/parse")


def count_cases(document: dict[str, Any]) -> int:
    """Count the cases the document covers.

    One case per fixture key, except the flat tables, which contribute
    one case per row.
    """
    total = 0
    for family in REQUIRED_KEYS:
        for key, value in document[family].items():
            total += len(value) if key in FLAT_TABLES else 1
    return total


if __name__ == "__main__":
    raise SystemExit(main())
