#!/usr/bin/env python3
"""Enforce per-spec-version commit isolation and stable-version invariants.

Rules:
  A. A single commit MUST NOT touch files in more than one spec/vX.Y/ directory.
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
        files = git(["diff-tree", "--no-commit-id", "--name-only", "-r", sha]).split()
        spec_versions = set()
        for f in files:
            m = SPEC_VERSION_RE.match(f)
            if m:
                spec_versions.add(m.group(1))

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
