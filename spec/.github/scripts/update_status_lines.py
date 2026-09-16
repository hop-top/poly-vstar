#!/usr/bin/env python3
"""Patch `**Status:**` lines in spec markdown files based on the release-please
PR title's version suffix.

Triggered by .github/workflows/release-status-line.yml when a release-please
PR is labeled `status:release-pending`. Runs from the repository root; the
spec tree it patches lives at spec/.

Mapping:
  -alpha.N → Draft
  -beta.N  → Pre-release
  -rc.N    → Release Candidate
  unsuffixed → General Availability

Inputs (env, strictly validated):
  PR_TITLE  — e.g. `chore(release): vstar-spec 1.0.0-alpha.0`
  PR_BRANCH — head ref to push to (must match BRANCH_RE)
"""
from __future__ import annotations

import json
import os
import re
import subprocess
import sys
from pathlib import Path

# The specification tree, relative to the repository root the workflow
# checks out. Every version directory (spec/vX.Y) and the spec's own
# README live under it.
SPEC_ROOT = Path("spec")

# Strict parse of release-please's pull-request-title-pattern.
# Config sets `chore(release):${component} ${version}` (no scope, single space).
# Component: the spec component, named `vstar-spec` in the config; its
# package path (the spec version directory) is looked up there.
# Version shape: 0.1.0, 0.1.0-alpha.0, 0.2.3-rc.4, etc.
TITLE_RE = re.compile(
    r"^chore\(release\):\s*"
    r"(?P<component>vstar-spec)\s+"
    r"(?P<version>\d+\.\d+\.\d+(?:-(?P<channel>alpha|beta|rc)\.\d+)?)\s*$",
)

# The same title pattern for any other release-please component of the
# monorepo (`vstar`, `vstar-ts`, `vstar-py`, ...). Those carry no
# status lines; a release PR for one of them is a no-op here, not an
# error — the error path is reserved for titles that are not
# release-please titles at all.
OTHER_COMPONENT_TITLE_RE = re.compile(
    r"^chore\(release\):\s*"
    r"(?P<component>[A-Za-z0-9._-]+)\s+"
    r"\d+\.\d+\.\d+(?:-(?:alpha|beta|rc)\.\d+)?\s*$",
)

# Git branch names are limited by git's refname rules; we apply a strict
# allow-list rather than relying on git/shell quoting. Additional checks
# reject path-traversal patterns ('..') and leading/trailing punctuation.
BRANCH_RE = re.compile(r"^[A-Za-z0-9._/-]{1,255}$")


def _branch_is_safe(b: str) -> bool:
    if not BRANCH_RE.match(b):
        return False
    if ".." in b:
        return False
    if b.startswith(("/", "-", ".")) or b.endswith(("/", ".lock", ".")):
        return False
    return True

# Match a top-of-file status line. Tolerate trailing whitespace markdown
# uses for hard breaks ("  ").
STATUS_RE = re.compile(r"^\*\*Status:\*\*\s+.+?\s*$", re.MULTILINE)
STATUS_HEAD_LINES = 15  # only patch lines in the head of the file

CHANNEL_LABEL = {
    "alpha": "Draft",
    "beta": "Pre-release",
    "rc": "Release Candidate",
    None: "General Availability",
}


def die(msg: str, code: int = 2) -> None:
    print(f"::error::{msg}", file=sys.stderr)
    sys.exit(code)


def run(argv: list[str]) -> str:
    return subprocess.run(
        argv, check=True, capture_output=True, text=True,
    ).stdout


def parse_title(title: str) -> tuple[str, str, str]:
    m = TITLE_RE.match(title)
    if not m:
        die(f"PR_TITLE does not match release-please pattern: {title!r}")
    component = m.group("component")
    version = m.group("version")
    channel = m.group("channel")  # alpha | beta | rc | None
    return component, version, channel


def validate_branch(branch: str) -> str:
    if not _branch_is_safe(branch):
        die(f"PR_BRANCH is not a safe branch name: {branch!r}")
    return branch


CONFIG_PATH = Path(".github/release-please-config.json")


def spec_dir_for(component: str) -> Path:
    # release-please's own config is the component → package-path map;
    # the spec component's package path is the version directory whose
    # status lines this script patches. Nothing is derived from the
    # component's spelling.
    try:
        packages = json.loads(CONFIG_PATH.read_text(encoding="utf-8"))["packages"]
    except (OSError, ValueError, KeyError) as exc:
        die(f"cannot read packages from {CONFIG_PATH}: {exc}")
    paths = [p for p, cfg in packages.items() if cfg.get("component") == component]
    if len(paths) != 1:
        die(
            f"component {component!r} maps to {len(paths)} package path(s) "
            f"in {CONFIG_PATH}; expected exactly one",
        )
    p = Path(paths[0])
    if p.parts[:1] != (SPEC_ROOT.name,) or not p.is_dir():
        die(f"spec directory does not exist or is outside {SPEC_ROOT}/: {p}")
    return p


def patch_file(path: Path, new_status: str) -> bool:
    text = path.read_text(encoding="utf-8")
    lines = text.split("\n")
    head = "\n".join(lines[:STATUS_HEAD_LINES])
    tail = "\n".join(lines[STATUS_HEAD_LINES:])

    def repl(m: re.Match[str]) -> str:
        return f"**Status:** {new_status}"

    new_head, n = STATUS_RE.subn(repl, head)
    if n == 0:
        return False
    new_text = new_head + ("\n" + tail if tail else "")
    if new_text == text:
        return False
    path.write_text(new_text, encoding="utf-8")
    return True


def main() -> int:
    title = os.environ.get("PR_TITLE", "")
    branch = validate_branch(os.environ.get("PR_BRANCH", ""))

    if not TITLE_RE.match(title) and OTHER_COMPONENT_TITLE_RE.match(title):
        other = OTHER_COMPONENT_TITLE_RE.match(title)
        assert other is not None
        print(
            f"::notice::{other.group('component')} is not a spec component; "
            "no status lines to patch",
        )
        return 0

    component, version, channel = parse_title(title)
    new_status = CHANNEL_LABEL[channel]
    print(f"version={version} channel={channel or 'stable'} → status={new_status!r}")

    spec_dir = spec_dir_for(component)
    md_files = sorted(spec_dir.rglob("*.md"))
    # Also patch the spec tree's own README if it carries a status line
    # (rare). The monorepo root README is not a spec document and is
    # deliberately left alone.
    spec_readme = SPEC_ROOT / "README.md"
    if spec_readme.is_file():
        md_files.append(spec_readme)

    patched: list[Path] = []
    for f in md_files:
        if patch_file(f, new_status):
            patched.append(f)

    if not patched:
        print("no status lines to update")
        return 0

    print("patched:")
    for f in patched:
        print(f"  {f}")

    # Commit and push. argv-list form, no shell.
    run(["git", "config", "user.name", "release-please-status-bot"])
    run(["git", "config", "user.email",
         "release-please-status-bot@users.noreply.github.com"])
    run(["git", "add", "--", *[str(f) for f in patched]])
    run(["git", "commit", "-m", "chore(release): sync spec status lines"])
    run(["git", "push", "origin", f"HEAD:refs/heads/{branch}"])
    print("pushed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
