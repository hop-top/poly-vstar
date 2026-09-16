#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Guard the release-version literals and the spec version paths.

release-please owns every version in this repository: the manifest,
each package's native version file, and -- through ``extra-files`` with
an ``x-release-please-version`` annotation -- the few literals that live
outside those files. A version typed anywhere else is a literal nobody
rewrites, and it drifts on the first release after it is written.

Rules, each with its own message and a non-zero exit on any finding:

1. ``annotated``  -- every line carrying ``x-release-please-version``
   sits under exactly one release-please package path, holds exactly
   one version literal, and that literal equals the manifest's version
   for the package. Outside every package a line that names the
   annotation but carries no version literal is prose about the
   mechanism, not an annotation, and is ignored.
2. ``configured`` -- the set of annotated files equals the set of
   ``extra-files`` entries across packages. A configured file with no
   annotation, or an annotated file missing from the config, is a
   half-wired mechanism.
3. ``literal``    -- outside the manifest, changelogs, lockfiles, the
   natively rewritten files and the allow-list below, no un-annotated
   line contains a literal equal to a manifest value, or a literal in
   the prerelease channel shape (``X.Y.Z-alpha.N`` or the PEP 440
   ``X.Y.ZaN``).
4. ``spec-path``  -- every ``spec/vX.Y`` path, and every corpus-walker
   spelling of one (a quoted ``vX.Y`` token joined to a quoted
   ``conformance``), names a version whose ``spec/vX.Y`` key is in the
   manifest.
5. ``prodid``     -- no language-tagged, version-bearing ``PRODID``
   literal (``vstar-<lang> vX.Y.Z``): PRODID is hashed, so the default
   is version-free and identical in every port.

Allow-listed: ``spec/.github/scripts/**`` (release tooling whose inputs
are release titles, so its tests spell versions by construction) and
this tool's own test file (synthetic trees seeded with literals).

Usage::

    python3 tools/version-check/check.py            # check the repository
    python3 tools/version-check/check.py --root DIR # check another tree

Standard library only, the shape of ``tools/registry/gen.py``, so it
runs before any port's toolchain is installed.
"""

from __future__ import annotations

import argparse
import fnmatch
import json
import pathlib
import re
import subprocess
import sys

ANNOTATION = "x-release-please-version"
MANIFEST = ".github/.release-please-manifest.json"
CONFIG = ".github/release-please-config.json"

# The generic updater's own version regex (release-please
# src/updaters/generic.ts): what it would rewrite on an annotated line.
VERSION_RE = re.compile(
    r"(?<![\w.-])(?P<major>\d+)\.(?P<minor>\d+)\.(?P<patch>\d+)"
    r"(-(?P<pre>[\w.]+))?(\+(?P<build>[-\w.]+))?(?![\w-])",
)
# The prerelease channel shape and its PEP 440 spelling.
CHANNEL_RE = re.compile(r"(?<![\w.-])\d+\.\d+\.\d+-(alpha|beta|rc)\.\d+(?![\w-])")
PEP440_RE = re.compile(r"(?<![\w.-])\d+\.\d+\.\d+(a|b|rc)\d+(?![\w-])")
# A spec version directory, with or without a trailing slash.
SPEC_PATH_RE = re.compile(r"spec/v(\d+\.\d+)(?![\w.])")
# A corpus walker's spelling: a quoted vX.Y token joined, within a short
# span, to a quoted "conformance" -- covers `"v1.0", "conformance"`,
# `"v1.0" / "conformance"`, `.join("v1.0").join("conformance")` and
# `'v1.0' . DIRECTORY_SEPARATOR . 'conformance'`.
WALKER_RE = re.compile(r"""["']v(\d+\.\d+)["'][^\n"']{0,40}["']conformance["']""")
# A language-tagged, version-bearing PRODID.
PRODID_RE = re.compile(r"vstar-(go|ts|py|rs|php) v\d")

# Files release-please rewrites natively, by release type.
NATIVE_FILES = {
    "go": (),
    "node": ("package.json",),
    "python": ("pyproject.toml",),
    "rust": ("Cargo.toml",),
    "php": ("VERSION",),
    "simple": ("version.txt",),
}
LOCKFILES = ("Cargo.lock", "composer.lock", "pnpm-lock.yaml", "uv.lock")
ALLOW_LIST = (
    # Release tooling whose inputs are release PR titles; its tests
    # spell versions on purpose.
    "spec/.github/scripts/*",
    # This guard's own tests seed synthetic trees with literals.
    "tools/version-check/test_check.py",
)


class Finding:
    __slots__ = ("path", "line", "rule", "detail")

    def __init__(self, path: str, line: int, rule: str, detail: str) -> None:
        self.path, self.line, self.rule, self.detail = path, line, rule, detail

    def __str__(self) -> str:
        return f"{self.path}:{self.line}: {self.rule}: {self.detail}"


class Repo:
    """One tree: the manifest, the config and the tracked text files."""

    def __init__(self, root: pathlib.Path) -> None:
        self.root = root
        self.manifest: dict[str, str] = self._load(MANIFEST)
        self.config: dict = self._load(CONFIG)
        self.packages: dict[str, dict] = self.config.get("packages", {})
        self.files = self._tracked()

    def _load(self, rel: str):
        path = self.root / rel
        try:
            with path.open(encoding="utf-8") as handle:
                return json.load(handle)
        except FileNotFoundError:
            sys.exit(f"version-check: missing {rel}")
        except json.JSONDecodeError as err:
            sys.exit(f"version-check: {rel} is not valid JSON: {err}")

    def _tracked(self) -> list[str]:
        result = subprocess.run(
            ["git", "-C", str(self.root), "ls-files", "-z"],
            check=True, capture_output=True, text=True,
        )
        return [f for f in result.stdout.split("\0") if f]

    def package_of(self, path: str) -> str | None:
        """The package whose path contains ``path``; the longest wins."""
        best = None
        for pkg in self.packages:
            prefix = pkg.rstrip("/") + "/"
            if path.startswith(prefix) and (best is None or len(pkg) > len(best)):
                best = pkg
        return best

    def native_files(self) -> set[str]:
        out: set[str] = set()
        for pkg, cfg in self.packages.items():
            for name in NATIVE_FILES.get(cfg.get("release-type", ""), ()):
                out.add(f"{pkg.rstrip('/')}/{name}")
        return out

    def configured_extra_files(self) -> set[str]:
        out: set[str] = set()
        for pkg, cfg in self.packages.items():
            for entry in cfg.get("extra-files", ()):
                rel = entry if isinstance(entry, str) else entry.get("path", "")
                if rel:
                    out.add(f"{pkg.rstrip('/')}/{rel}")
        return out

    def spec_versions(self) -> set[str]:
        return {m.group(1) for m in (SPEC_PATH_RE.match(k) for k in self.manifest) if m}

    def scannable(self, path: str) -> bool:
        """Whether rules 3-5 read this file at all."""
        base = path.rsplit("/", 1)[-1]
        if base == "CHANGELOG.md" or base in LOCKFILES:
            return False
        if path == MANIFEST or path in self.native_files():
            return False
        return not any(fnmatch.fnmatch(path, pat) for pat in ALLOW_LIST)

    def lines(self, path: str) -> list[str] | None:
        try:
            return (self.root / path).read_text(encoding="utf-8").splitlines()
        except (UnicodeDecodeError, FileNotFoundError, IsADirectoryError):
            return None


# --------------------------------------------------------------------
# Rules
# --------------------------------------------------------------------


def rule_annotated(repo: Repo, path: str, n: int, line: str) -> list[Finding]:
    """Rule 1: an annotated line holds its package's manifest version."""
    pkg = repo.package_of(path)
    if pkg is None:
        return [Finding(path, n, "annotated",
                        "annotation outside every release-please package path; "
                        "release-please cannot rewrite it")]
    literals = [m.group(0) for m in VERSION_RE.finditer(line)]
    if len(literals) != 1:
        return [Finding(path, n, "annotated",
                        f"expected exactly one version literal on the annotated line, "
                        f"found {len(literals)}")]
    want = repo.manifest.get(pkg)
    if literals[0] != want:
        return [Finding(path, n, "annotated",
                        f"{literals[0]} != manifest[{pkg}] = {want}")]
    return []


def rule_configured(repo: Repo, annotated: set[str]) -> list[Finding]:
    """Rule 2: annotated files and extra-files entries are one set."""
    out: list[Finding] = []
    configured = repo.configured_extra_files()
    for path in sorted(configured - annotated):
        out.append(Finding(path, 0, "configured",
                           f"listed in extra-files but carries no {ANNOTATION} line"))
    for path in sorted(annotated - configured):
        out.append(Finding(path, 0, "configured",
                           f"carries {ANNOTATION} but is not an extra-files entry"))
    return out


def rule_literal(repo: Repo, path: str, n: int, line: str) -> list[Finding]:
    """Rule 3: no stray release-version literal on an un-annotated line."""
    out: list[Finding] = []
    for m in VERSION_RE.finditer(line):
        if m.group(0) in repo.manifest.values():
            out.append(Finding(path, n, "literal",
                               f"{m.group(0)} equals a manifest version and is not "
                               f"annotated with {ANNOTATION}"))
    for m in CHANNEL_RE.finditer(line):
        if m.group(0) not in repo.manifest.values():
            out.append(Finding(path, n, "literal",
                               f"{m.group(0)} is a hard-coded prerelease version"))
    for m in PEP440_RE.finditer(line):
        out.append(Finding(path, n, "literal",
                           f"{m.group(0)} is a hard-coded prerelease version (PEP 440)"))
    return out


def rule_spec_path(repo: Repo, path: str, n: int, line: str, known: set[str]) -> list[Finding]:
    """Rule 4: every spec/vX.Y path names a manifest spec version."""
    out: list[Finding] = []
    seen: set[str] = set()
    for m in list(SPEC_PATH_RE.finditer(line)) + list(WALKER_RE.finditer(line)):
        v = m.group(1)
        if v in known or v in seen:
            continue
        seen.add(v)
        out.append(Finding(path, n, "spec-path",
                           f"spec/v{v} is not a manifest package "
                           f"(known: {', '.join('spec/v' + k for k in sorted(known)) or 'none'})"))
    return out


def rule_prodid(path: str, n: int, line: str) -> list[Finding]:
    """Rule 5: no language-tagged, version-bearing PRODID literal."""
    m = PRODID_RE.search(line)
    if not m:
        return []
    return [Finding(path, n, "prodid",
                    f"{m.group(0)}... is a language-tagged, version-bearing PRODID; "
                    "the default is -//hop-top//vstar//EN in every port")]


# --------------------------------------------------------------------
# Driver
# --------------------------------------------------------------------


def check(root: pathlib.Path) -> tuple[list[Finding], int, int]:
    repo = Repo(root)
    known = repo.spec_versions()
    findings: list[Finding] = []
    annotated_files: set[str] = set()
    annotated_lines = 0
    scanned = 0
    for path in repo.files:
        if not repo.scannable(path):
            continue
        lines = repo.lines(path)
        if lines is None:
            continue
        scanned += 1
        in_package = repo.package_of(path) is not None
        for n, line in enumerate(lines, 1):
            if ANNOTATION in line:
                if not in_package and not VERSION_RE.search(line):
                    continue  # prose naming the annotation, nothing to rewrite
                annotated_files.add(path)
                annotated_lines += 1
                findings += rule_annotated(repo, path, n, line)
                continue
            findings += rule_literal(repo, path, n, line)
            findings += rule_spec_path(repo, path, n, line, known)
            findings += rule_prodid(path, n, line)
    findings += rule_configured(repo, annotated_files)
    findings.sort(key=lambda f: (f.path, f.line, f.rule))
    return findings, annotated_lines, scanned


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n", 1)[0])
    parser.add_argument(
        "--root", type=pathlib.Path,
        default=pathlib.Path(__file__).resolve().parents[2],
        help="repository root (default: two levels above this script)",
    )
    args = parser.parse_args(argv)
    findings, annotated, scanned = check(args.root.resolve())
    for f in findings:
        print(f)
    if findings:
        print(f"version-check: {len(findings)} finding(s)", file=sys.stderr)
        return 1
    print(f"version-check: ok ({annotated} annotated, {scanned} files scanned)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
