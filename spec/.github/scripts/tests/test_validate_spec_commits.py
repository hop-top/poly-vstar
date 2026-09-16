"""Tests for .github/scripts/validate_spec_commits.py.

Each test sets up a temp git repo with synthesized commits and invokes the
script via subprocess, capturing exit code + stderr. The script reads BASE_SHA
and HEAD_SHA from env.
"""
from __future__ import annotations

import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[3]
SCRIPT = REPO_ROOT / ".github" / "scripts" / "validate_spec_commits.py"


def _git(cwd: Path, *args: str, env: dict[str, str] | None = None) -> str:
    full_env = os.environ.copy()
    # Stable identity so SHAs are deterministic within a test.
    full_env.update({
        "GIT_AUTHOR_NAME": "Test",
        "GIT_AUTHOR_EMAIL": "test@example.com",
        "GIT_COMMITTER_NAME": "Test",
        "GIT_COMMITTER_EMAIL": "test@example.com",
        "GIT_AUTHOR_DATE": "2026-01-01T00:00:00Z",
        "GIT_COMMITTER_DATE": "2026-01-01T00:00:00Z",
    })
    if env:
        full_env.update(env)
    result = subprocess.run(
        ["git", *args], cwd=str(cwd), env=full_env,
        check=True, capture_output=True, text=True,
    )
    return result.stdout.strip()


def _commit(cwd: Path, files: dict[str, str], message: str) -> str:
    for rel, content in files.items():
        p = cwd / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(content)
    _git(cwd, "add", "-A")
    _git(cwd, "commit", "-m", message)
    return _git(cwd, "rev-parse", "HEAD")


def _run_script(repo: Path, base: str, head: str) -> subprocess.CompletedProcess:
    env = os.environ.copy()
    env["BASE_SHA"] = base
    env["HEAD_SHA"] = head
    return subprocess.run(
        [sys.executable, str(SCRIPT)],
        cwd=str(repo), env=env,
        capture_output=True, text=True,
    )


class ValidateSpecCommitsTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = Path(tempfile.mkdtemp(prefix="vsc-"))
        _git(self.tmp, "init", "--initial-branch=main", "--quiet")
        # Seed an initial commit so we always have a base.
        self.base = _commit(self.tmp, {"README.md": "seed\n"}, "chore: seed")

    def tearDown(self) -> None:
        shutil.rmtree(self.tmp, ignore_errors=True)

    # --- Rule A: per-spec-version isolation -------------------------------

    def test_single_commit_single_spec_dir_passes(self) -> None:
        head = _commit(self.tmp, {"spec/v0.1/x.md": "x\n"}, "feat: x")
        r = _run_script(self.tmp, self.base, head)
        self.assertEqual(r.returncode, 0, r.stderr)

    def test_single_commit_two_spec_dirs_fails(self) -> None:
        head = _commit(
            self.tmp,
            {"spec/v0.1/x.md": "x\n", "spec/v0.2/y.md": "y\n"},
            "feat: x",
        )
        r = _run_script(self.tmp, self.base, head)
        self.assertEqual(r.returncode, 1)
        self.assertIn("multiple spec versions", r.stderr)
        self.assertIn("v0.1", r.stderr)
        self.assertIn("v0.2", r.stderr)

    def test_commit_touching_only_root_files_passes(self) -> None:
        head = _commit(self.tmp, {"README.md": "seed\nedited\n"}, "docs: edit")
        r = _run_script(self.tmp, self.base, head)
        self.assertEqual(r.returncode, 0, r.stderr)

    # --- Rule B: breaking-change syntax forbidden in spec dirs ------------

    def test_breaking_bang_in_spec_dir_fails(self) -> None:
        head = _commit(self.tmp, {"spec/v0.1/x.md": "x\n"}, "feat!: drop field")
        r = _run_script(self.tmp, self.base, head)
        self.assertEqual(r.returncode, 1)
        self.assertIn("breaking change", r.stderr)
        self.assertIn("v0.1", r.stderr)

    def test_breaking_trailer_in_spec_dir_fails(self) -> None:
        head = _commit(
            self.tmp,
            {"spec/v0.1/x.md": "x\n"},
            "feat: x\n\nBREAKING CHANGE: removed Y\n",
        )
        r = _run_script(self.tmp, self.base, head)
        self.assertEqual(r.returncode, 1)
        self.assertIn("breaking change", r.stderr)

    def test_breaking_trailer_hyphen_in_spec_dir_fails(self) -> None:
        head = _commit(
            self.tmp,
            {"spec/v0.1/x.md": "x\n"},
            "feat: x\n\nBREAKING-CHANGE: removed Y\n",
        )
        r = _run_script(self.tmp, self.base, head)
        self.assertEqual(r.returncode, 1)

    def test_breaking_bang_touching_root_only_passes(self) -> None:
        head = _commit(self.tmp, {"README.md": "seed\nedit\n"}, "chore!: x")
        r = _run_script(self.tmp, self.base, head)
        self.assertEqual(r.returncode, 0, r.stderr)

    def test_non_breaking_note_passes(self) -> None:
        head = _commit(
            self.tmp,
            {"spec/v0.1/x.md": "x\n"},
            "feat: x\n\nNote: nothing breaking here\n",
        )
        r = _run_script(self.tmp, self.base, head)
        self.assertEqual(r.returncode, 0, r.stderr)

    # --- Range edge cases -------------------------------------------------

    def test_empty_range_passes(self) -> None:
        r = _run_script(self.tmp, self.base, self.base)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("no commits", r.stdout)

    def test_multi_commit_range_one_violates(self) -> None:
        good = _commit(self.tmp, {"spec/v0.1/a.md": "a\n"}, "feat: a")
        bad = _commit(self.tmp, {"spec/v0.1/b.md": "b\n"}, "feat!: b")
        ok = _commit(self.tmp, {"spec/v0.1/c.md": "c\n"}, "fix: c")
        r = _run_script(self.tmp, self.base, ok)
        self.assertEqual(r.returncode, 1)
        # Only the breaking commit should produce a violation.
        self.assertEqual(r.stderr.count("breaking change"), 1)

    def test_multi_commit_range_multiple_violate(self) -> None:
        a = _commit(self.tmp, {"spec/v0.1/a.md": "a\n"}, "feat!: a")
        b = _commit(
            self.tmp,
            {"spec/v0.1/b.md": "b\n", "spec/v0.2/b.md": "b\n"},
            "feat: b",
        )
        r = _run_script(self.tmp, self.base, b)
        self.assertEqual(r.returncode, 1)
        self.assertIn("2 commit-rule violation", r.stderr)

    # --- Input validation -------------------------------------------------

    def test_rejects_non_sha_input(self) -> None:
        r = _run_script(self.tmp, "not-a-sha", self.base)
        self.assertEqual(r.returncode, 2)
        self.assertIn("not a git SHA", r.stderr)


if __name__ == "__main__":
    unittest.main()
