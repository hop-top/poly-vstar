"""Tests for .github/scripts/update_status_lines.py.

Each test sets up a temp git working tree + temp bare remote, drops the
script's expected spec layout into the worktree, and invokes the script
via subprocess with PR_TITLE / PR_BRANCH in env.
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
SCRIPT = REPO_ROOT / ".github" / "scripts" / "update_status_lines.py"


def _git(cwd: Path, *args: str) -> str:
    env = os.environ.copy()
    env.update({
        "GIT_AUTHOR_NAME": "Test",
        "GIT_AUTHOR_EMAIL": "test@example.com",
        "GIT_COMMITTER_NAME": "Test",
        "GIT_COMMITTER_EMAIL": "test@example.com",
        "GIT_AUTHOR_DATE": "2026-01-01T00:00:00Z",
        "GIT_COMMITTER_DATE": "2026-01-01T00:00:00Z",
    })
    result = subprocess.run(
        ["git", *args], cwd=str(cwd), env=env,
        check=True, capture_output=True, text=True,
    )
    return result.stdout.strip()


def _setup_worktree(tmp: Path) -> tuple[Path, Path, str]:
    """Returns (worktree, bare_remote, head_sha)."""
    remote = tmp / "remote.git"
    work = tmp / "work"
    work.mkdir()
    subprocess.run(
        ["git", "init", "--bare", "--quiet", str(remote)],
        check=True, capture_output=True,
    )
    _git(work, "init", "--initial-branch=main", "--quiet")
    _git(work, "remote", "add", "origin", str(remote))
    # Seed a minimal spec tree plus a monorepo-root README that is not a
    # spec document and must stay untouched.
    (work / "spec" / "v1.0").mkdir(parents=True)
    (work / "spec" / "v1.0" / "envelope.md").write_text(
        "# vstar v1.0 — Envelope\n\n"
        "**Status:** Draft  \n"
        "**License:** CC-BY-4.0\n\n"
        "Body text.\n",
    )
    (work / "spec" / "v1.0" / "events.md").write_text(
        "# vstar v1.0 — Events\n\n"
        "**Status:** Draft\n"
        "**License:** CC-BY-4.0\n\n"
        "Body text.\n",
    )
    (work / "spec" / "v1.0" / "README.md").write_text(
        "# vstar v1.0\n\n"
        "**Status:** Draft  \n"
        "**Last updated:** 2026-05-28\n",
    )
    (work / "spec" / "README.md").write_text(
        "# V* specification\n\nNo status line here.\n",
    )
    (work / "README.md").write_text(
        "# vstar\n\nNo status line here.\n",
    )
    # The component → package-path map the script reads.
    (work / ".github").mkdir()
    (work / ".github" / "release-please-config.json").write_text(
        '{"packages": {"spec/v1.0": {"component": "vstar-spec"}, '
        '"go": {"component": "vstar"}}}\n',
    )
    _git(work, "add", "-A")
    _git(work, "commit", "-m", "chore: seed")
    head = _git(work, "rev-parse", "HEAD")
    # Push so the bare remote has a `main` ref, and create the PR branch.
    _git(work, "push", "-u", "origin", "main")
    return work, remote, head


def _run_script(
    work: Path, pr_title: str, pr_branch: str,
) -> subprocess.CompletedProcess:
    env = os.environ.copy()
    env["PR_TITLE"] = pr_title
    env["PR_BRANCH"] = pr_branch
    return subprocess.run(
        [sys.executable, str(SCRIPT)],
        cwd=str(work), env=env,
        capture_output=True, text=True,
    )


def _status_lines(work: Path) -> dict[str, str | None]:
    out: dict[str, str | None] = {}
    for rel in [
        "spec/v1.0/envelope.md",
        "spec/v1.0/events.md",
        "spec/v1.0/README.md",
        "spec/README.md",
        "README.md",
    ]:
        p = work / rel
        if not p.is_file():
            out[rel] = None
            continue
        for line in p.read_text().splitlines()[:15]:
            if line.startswith("**Status:**"):
                out[rel] = line.strip()
                break
        else:
            out[rel] = None
    return out


class UpdateStatusLinesTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = Path(tempfile.mkdtemp(prefix="usl-"))
        self.work, self.remote, self.head = _setup_worktree(self.tmp)
        # Branch the PR off main so we can push to it.
        _git(self.work, "checkout", "-b", "release-pr")
        _git(self.work, "push", "-u", "origin", "release-pr")

    def tearDown(self) -> None:
        shutil.rmtree(self.tmp, ignore_errors=True)

    # --- channel mapping --------------------------------------------------

    def test_alpha_maps_to_draft(self) -> None:
        r = _run_script(
            self.work, "chore(release): vstar-spec 0.1.0-alpha.0", "release-pr",
        )
        self.assertEqual(r.returncode, 0, r.stderr)
        for line in _status_lines(self.work).values():
            if line is not None:
                self.assertEqual(line, "**Status:** Draft")

    def test_beta_maps_to_pre_release(self) -> None:
        r = _run_script(
            self.work, "chore(release): vstar-spec 0.1.0-beta.2", "release-pr",
        )
        self.assertEqual(r.returncode, 0, r.stderr)
        lines = _status_lines(self.work)
        self.assertEqual(lines["spec/v1.0/envelope.md"], "**Status:** Pre-release")

    def test_rc_maps_to_release_candidate(self) -> None:
        r = _run_script(
            self.work, "chore(release): vstar-spec 0.1.0-rc.0", "release-pr",
        )
        self.assertEqual(r.returncode, 0, r.stderr)
        lines = _status_lines(self.work)
        self.assertEqual(
            lines["spec/v1.0/envelope.md"], "**Status:** Release Candidate",
        )

    def test_stable_maps_to_general_availability(self) -> None:
        r = _run_script(
            self.work, "chore(release): vstar-spec 0.1.0", "release-pr",
        )
        self.assertEqual(r.returncode, 0, r.stderr)
        lines = _status_lines(self.work)
        self.assertEqual(
            lines["spec/v1.0/envelope.md"], "**Status:** General Availability",
        )

    # --- input validation -------------------------------------------------

    def test_malformed_title_exits_2(self) -> None:
        r = _run_script(self.work, "feat: not a release", "release-pr")
        self.assertEqual(r.returncode, 2)
        self.assertIn("PR_TITLE", r.stderr)

    def test_spec_component_without_a_config_mapping_exits_2(self) -> None:
        # The spec directory comes from the release-please config, never
        # from the component's spelling; with no mapping there is nothing
        # safe to patch.
        cfg = self.work / ".github" / "release-please-config.json"
        cfg.write_text('{"packages": {"go": {"component": "vstar"}}}\n')
        _git(self.work, "commit", "-q", "-am", "drop the spec mapping")
        r = _run_script(
            self.work, "chore(release): vstar-spec 1.0.0-alpha.0", "release-pr",
        )
        self.assertEqual(r.returncode, 2)
        self.assertIn("maps to 0 package path", r.stderr)

    def test_other_component_title_is_a_no_op(self) -> None:
        # A release PR for a package component of the monorepo is a
        # well-formed release-please title that simply has no status
        # lines to patch: exit 0, say so, change nothing.
        before = _status_lines(self.work)
        for title in (
            "chore(release): vstar 1.0.0-alpha.0",
            "chore(release): vstar-py 1.0.0-alpha.0",
            "chore(release): vstar-ts 1.0.0",
        ):
            r = _run_script(self.work, title, "release-pr")
            self.assertEqual(r.returncode, 0, r.stderr)
            self.assertIn("no status lines to patch", r.stdout)
        self.assertEqual(_status_lines(self.work), before)

    def test_branch_with_double_dot_exits_2(self) -> None:
        r = _run_script(
            self.work, "chore(release): vstar-spec 0.1.0-alpha.0", "foo/../bar",
        )
        self.assertEqual(r.returncode, 2)

    def test_branch_with_space_exits_2(self) -> None:
        r = _run_script(
            self.work, "chore(release): vstar-spec 0.1.0-alpha.0", "foo bar",
        )
        self.assertEqual(r.returncode, 2)

    def test_branch_with_semicolon_exits_2(self) -> None:
        r = _run_script(
            self.work, "chore(release): vstar-spec 0.1.0-alpha.0", "main; rm",
        )
        self.assertEqual(r.returncode, 2)

    def test_branch_with_newline_exits_2(self) -> None:
        r = _run_script(
            self.work, "chore(release): vstar-spec 0.1.0-alpha.0", "foo\nbar",
        )
        self.assertEqual(r.returncode, 2)

    # --- patch behavior ---------------------------------------------------

    def test_file_without_status_line_unchanged(self) -> None:
        before = (self.work / "README.md").read_text()
        r = _run_script(
            self.work, "chore(release): vstar-spec 0.1.0-alpha.0", "release-pr",
        )
        self.assertEqual(r.returncode, 0, r.stderr)
        after = (self.work / "README.md").read_text()
        self.assertEqual(before, after)

    def test_status_below_head_unchanged(self) -> None:
        # Add a file with **Status:** below the head window.
        late = self.work / "spec" / "v1.0" / "buried.md"
        late.write_text(
            "# Buried\n\n" + ("filler\n" * 20) + "**Status:** Draft\n",
        )
        _git(self.work, "add", "-A")
        _git(self.work, "commit", "-m", "chore: filler")
        r = _run_script(
            self.work, "chore(release): vstar-spec 0.1.0-rc.0", "release-pr",
        )
        self.assertEqual(r.returncode, 0, r.stderr)
        # The buried status line should remain Draft (out of head window).
        self.assertIn("**Status:** Draft", late.read_text())

    def test_no_change_no_commit(self) -> None:
        # First run patches files; second run on the same target should be
        # idempotent (no further commit).
        _run_script(
            self.work, "chore(release): vstar-spec 0.1.0-alpha.0", "release-pr",
        )
        head_before = _git(self.work, "rev-parse", "HEAD")
        r = _run_script(
            self.work, "chore(release): vstar-spec 0.1.0-alpha.0", "release-pr",
        )
        self.assertEqual(r.returncode, 0, r.stderr)
        head_after = _git(self.work, "rev-parse", "HEAD")
        self.assertEqual(head_before, head_after)
        self.assertIn("no status lines to update", r.stdout)

    # --- push integration -------------------------------------------------

    def test_push_to_remote(self) -> None:
        r = _run_script(
            self.work, "chore(release): vstar-spec 0.1.0-rc.0", "release-pr",
        )
        self.assertEqual(r.returncode, 0, r.stderr)
        # Confirm push reached the bare remote.
        log = subprocess.run(
            ["git", "log", "release-pr", "--oneline", "-1"],
            cwd=str(self.remote), check=True, capture_output=True, text=True,
        ).stdout
        self.assertIn("sync spec status lines", log)


if __name__ == "__main__":
    unittest.main()
