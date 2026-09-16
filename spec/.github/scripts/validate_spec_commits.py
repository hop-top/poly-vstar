#!/usr/bin/env python3
"""Enforce per-spec-version commit isolation and stable-version invariants.

Rules:
  A. A single commit MUST NOT touch files in more than one spec/vX.Y/ directory.
     One shape is exempt: a whole-directory rename. When the commit removes
     spec/vA.B/ entirely and every path it held reappears at the same
     relative path under one other directory spec/vC.D/, the commit counts
     as touching vC.D only — the directory changed its name. Files may be
     edited on the way (a squashed rename-and-revise lands as a delete plus
     an add); what matters is that nothing is left behind and nothing moved
     to a different relative path. A partial move or a reshuffle counts both
     versions and fails.
  B. A commit touching spec/vX.Y/ MUST NOT carry breaking-change syntax
     (Conventional Commits `!:` or a `BREAKING CHANGE:` trailer). Breaking
     changes mean a new spec-version directory (spec/vX.Y+1/ or vX+1.0/),
     not a bump within an existing one.

Inputs (env):
  BASE_SHA — pull_request.base.sha
  HEAD_SHA — pull_request.head.sha

Both are git SHAs (40-hex). The script validates that before passing them
to any subprocess invocation.
"""
from __future__ import annotations

import os
import re
import subprocess
import sys

SHA_RE = re.compile(r"^[0-9a-f]{7,40}$")
SPEC_VERSION_RE = re.compile(r"^spec/(v\d+\.\d+)/")
# Conventional Commits header: <type>(<scope>)?<!>?: <subject>
# We only need to detect the optional `!` before the colon.
BREAKING_HEADER_RE = re.compile(r"^[a-z]+(?:\([^)]+\))?!:")
BREAKING_TRAILER_RE = re.compile(r"^BREAKING[ -]CHANGE:", re.MULTILINE)


def fail(msg: str) -> None:
    print(f"::error::{msg}", file=sys.stderr)


def require_sha(name: str, value: str) -> str:
    if not SHA_RE.match(value):
        print(f"::error::env {name} is not a git SHA: {value!r}", file=sys.stderr)
        sys.exit(2)
    return value


def git(args: list[str]) -> str:
    result = subprocess.run(
        ["git", *args],
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout


def spec_version(path: str) -> str | None:
    m = SPEC_VERSION_RE.match(path)
    return m.group(1) if m else None


def relative_to_version(path: str, version: str) -> str:
    return path[len(f"spec/{version}/"):]


def directory_exists(sha: str, version: str) -> bool:
    try:
        return bool(git(["ls-tree", "-d", sha, f"spec/{version}"]).strip())
    except subprocess.CalledProcessError:
        return False  # no such commit (a root commit has no parent)


def touched_spec_versions(sha: str) -> set[str]:
    """Return the spec versions a commit touches, per rule A.

    Every path an entry names charges its version. Exact renames
    (`-M100%`) surface as `R100<TAB>old<TAB>new` and charge both ends;
    an edited-and-moved file surfaces as a delete plus an add. Then the
    whole-directory exemption: a version whose directory the commit
    removes, and whose every removed path reappears at the same relative
    path under exactly one other version, is dropped from the set — the
    directory was renamed, whatever else the commit did to its files.
    """
    entries = git(
        ["diff-tree", "--no-commit-id", "--name-status", "-r", "-M100%", sha],
    ).splitlines()
    versions: set[str] = set()
    removed: dict[str, set[str]] = {}  # version -> relative paths it lost
    present: dict[str, set[str]] = {}  # version -> relative paths it gained
    for entry in entries:
        fields = entry.split("\t")
        status = fields[0]
        if status.startswith("R") and len(fields) == 3:
            pairs = [("D", fields[1]), ("A", fields[2])]
        else:
            pairs = [(status[:1], path) for path in fields[1:]]
        for kind, path in pairs:
            v = spec_version(path)
            if not v:
                continue
            versions.add(v)
            if kind == "D":
                removed.setdefault(v, set()).add(relative_to_version(path, v))
            elif kind == "A":
                present.setdefault(v, set()).add(relative_to_version(path, v))

    for old_v, lost in removed.items():
        # The directory must vanish in this commit: present before, gone
        # after. Anything left behind means a partial move.
        if directory_exists(sha, old_v) or not directory_exists(f"{sha}^", old_v):
            continue
        destinations = [
            new_v for new_v, gained in present.items()
            if new_v != old_v and lost <= gained
        ]
        if len(destinations) == 1:
            versions.discard(old_v)
    return versions


def main() -> int:
    base = require_sha("BASE_SHA", os.environ.get("BASE_SHA", ""))
    head = require_sha("HEAD_SHA", os.environ.get("HEAD_SHA", ""))

    rev_range = f"{base}..{head}"
    shas = git(["rev-list", rev_range]).split()
    if not shas:
        print("no commits in range; nothing to validate")
        return 0

    violations = 0
    for sha in reversed(shas):  # oldest first
        spec_versions = touched_spec_versions(sha)

        if not spec_versions:
            continue  # commit doesn't touch any spec version; out of scope

        # Rule A — single commit, multiple spec versions
        if len(spec_versions) > 1:
            joined = ", ".join(sorted(spec_versions))
            fail(
                f"commit {sha[:12]} touches multiple spec versions ({joined}). "
                "Each commit MUST stay within one spec/vX.Y/ directory."
            )
            violations += 1

        # Rule B — breaking-change syntax forbidden when touching any spec/vX.Y/
        msg = git(["log", "-1", "--format=%B", sha])
        header = msg.split("\n", 1)[0]
        is_breaking = bool(BREAKING_HEADER_RE.match(header)) or bool(
            BREAKING_TRAILER_RE.search(msg),
        )
        if is_breaking:
            joined = ", ".join(sorted(spec_versions))
            fail(
                f"commit {sha[:12]} declares a breaking change while touching "
                f"{joined}. Breaking changes spawn a new spec/vN.M/ directory; "
                "they do NOT bump within an existing version."
            )
            violations += 1

    if violations:
        print(f"{violations} commit-rule violation(s)", file=sys.stderr)
        return 1
    print(f"validated {len(shas)} commit(s); no violations")
    return 0


if __name__ == "__main__":
    sys.exit(main())
