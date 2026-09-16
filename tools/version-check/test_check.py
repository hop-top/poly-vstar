# SPDX-License-Identifier: MIT
"""Tests for tools/version-check/check.py.

Each test builds a small tree in a temporary git repository -- a
manifest, a config and a few tracked files -- and runs the checker on
it through its ``--root`` flag, asserting on exit code and findings.
The literals below are the checker's inputs by construction; the real
tree allow-lists this file for that reason.
"""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().with_name("check.py")

MANIFEST = {"go": "1.0.0-alpha.0", "ts": "1.0.0-alpha.0", "py": "1.0.0-alpha.0",
            "spec/v1.0": "1.0.0-alpha.0"}


def _config(extra: dict[str, list[str]] | None = None) -> str:
    import json
    packages = {
        "go": {"release-type": "go", "component": "vstar"},
        "ts": {"release-type": "node", "component": "vstar-ts"},
        "py": {"release-type": "python", "component": "vstar-py"},
        "spec/v1.0": {"release-type": "simple", "component": "vstar-spec"},
    }
    for pkg, files in (extra or {}).items():
        packages[pkg]["extra-files"] = files
    return json.dumps({"packages": packages})


class VersionCheckTests(unittest.TestCase):
    def setUp(self) -> None:
        import json
        self.tmp = Path(tempfile.mkdtemp(prefix="vc-"))
        subprocess.run(["git", "init", "--quiet", str(self.tmp)], check=True)
        self.write(".github/.release-please-manifest.json", json.dumps(MANIFEST))
        self.write(".github/release-please-config.json", _config())

    def tearDown(self) -> None:
        shutil.rmtree(self.tmp, ignore_errors=True)

    def write(self, rel: str, content: str) -> None:
        path = self.tmp / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")

    def run_check(self) -> subprocess.CompletedProcess:
        subprocess.run(["git", "-C", str(self.tmp), "add", "-A"], check=True)
        env = os.environ.copy()
        env["PYTHONIOENCODING"] = "utf-8"
        return subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(self.tmp)],
            capture_output=True, text=True, env=env,
        )

    # --- ok path ------------------------------------------------------------

    def test_ok(self) -> None:
        self.write(".github/release-please-config.json", _config({"ts": ["src/version.ts"]}))
        self.write("ts/src/version.ts",
                   'export const version = "1.0.0-alpha.0"; // x-release-please-version\n')
        self.write("docs/a.md", "See spec/v1.0/03.md; pin X.Y.Z-alpha.N.\n")
        self.write("ts/CHANGELOG.md", "## 1.0.0-alpha.0\n")
        r = self.run_check()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("version-check: ok (1 annotated,", r.stdout)

    # --- rule 1: annotated => current -------------------------------------

    def test_annotated_literal_must_equal_manifest(self) -> None:
        self.write(".github/release-please-config.json", _config({"ts": ["src/version.ts"]}))
        self.write("ts/src/version.ts",
                   'export const version = "1.0.0-alpha.9"; // x-release-please-version\n')
        r = self.run_check()
        self.assertEqual(r.returncode, 1)
        self.assertIn("ts/src/version.ts:1: annotated: 1.0.0-alpha.9 != manifest[ts] = 1.0.0-alpha.0", r.stdout)

    def test_annotated_line_with_a_second_pep440_token_fails(self) -> None:
        # The updater rewrites the first SemVer match only; a PEP 440
        # spelling elsewhere on the line would go stale unseen.
        self.write(".github/release-please-config.json", _config({"py": ["README.md"]}))
        self.write("py/README.md",
                   "pin `==1.0.0-alpha.0` (pip normalizes it to `1.0.0a0`). "
                   "<!-- x-release-please-version -->\n")
        r = self.run_check()
        self.assertEqual(r.returncode, 1)
        self.assertIn("py/README.md:1: annotated: expected exactly one version literal "
                      "on the annotated line, found 2", r.stdout)

    def test_annotation_outside_a_package_fails(self) -> None:
        self.write("docs/a.md", "pin 1.0.0-alpha.0 <!-- x-release-please-version -->\n")
        r = self.run_check()
        self.assertEqual(r.returncode, 1)
        self.assertIn("docs/a.md:1: annotated: annotation outside every release-please package path", r.stdout)

    def test_prose_naming_the_annotation_outside_a_package_is_ignored(self) -> None:
        self.write("docs/a.md", "each carries `x-release-please-version` on the literal's line\n")
        r = self.run_check()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("version-check: ok (0 annotated,", r.stdout)

    # --- rule 2: annotated <=> configured ---------------------------------

    def test_configured_file_without_annotation_fails(self) -> None:
        self.write(".github/release-please-config.json", _config({"py": ["README.md"]}))
        self.write("py/README.md", "no annotation here\n")
        r = self.run_check()
        self.assertEqual(r.returncode, 1)
        self.assertIn("py/README.md:0: configured: listed in extra-files but carries no x-release-please-version line", r.stdout)

    def test_annotated_file_not_configured_fails(self) -> None:
        self.write("py/README.md", "pin (`==1.0.0-alpha.0`) <!-- x-release-please-version -->\n")
        r = self.run_check()
        self.assertEqual(r.returncode, 1)
        self.assertIn("py/README.md:0: configured: carries x-release-please-version but is not an extra-files entry", r.stdout)

    # --- rule 3: no stray literals ----------------------------------------

    def test_manifest_value_literal_fails(self) -> None:
        self.write("docs/a.md", "Install 1.0.0-alpha.0 today.\n")
        r = self.run_check()
        self.assertEqual(r.returncode, 1)
        self.assertIn("docs/a.md:1: literal: 1.0.0-alpha.0 equals a manifest version", r.stdout)

    def test_channel_shape_literal_fails(self) -> None:
        self.write("docs/a.md", "pin 1.0.0-alpha.7 here\n")
        r = self.run_check()
        self.assertEqual(r.returncode, 1)
        self.assertIn("docs/a.md:1: literal: 1.0.0-alpha.7 is a hard-coded prerelease version", r.stdout)

    def test_pep440_literal_fails(self) -> None:
        self.write("py/tests/test_version.py", "# e.g. 1.0.0a1\n")
        r = self.run_check()
        self.assertEqual(r.returncode, 1)
        self.assertIn("py/tests/test_version.py:1: literal: 1.0.0a1 is a hard-coded prerelease version (PEP 440)", r.stdout)

    def test_excluded_files_are_not_scanned(self) -> None:
        self.write("ts/CHANGELOG.md", "## 1.0.0-alpha.0\n")
        self.write("ts/package.json", '{"version": "1.0.0-alpha.0"}\n')
        self.write("py/pyproject.toml", 'version = "1.0.0-alpha.0"\n')
        self.write("spec/v1.0/version.txt", "1.0.0-alpha.0\n")
        self.write("py/uv.lock", 'version = "1.0.0-alpha.0"\n')
        self.write("spec/.github/scripts/x.py", 'TITLE = "chore(release): vstar 1.0.0-alpha.1"\n')
        r = self.run_check()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    # --- rule 4: spec paths name a manifest version -----------------------

    def test_spec_path_with_unknown_version_fails(self) -> None:
        self.write("docs/a.md", "see spec/v0.8/03.md and `spec/v0.9`\n")
        r = self.run_check()
        self.assertEqual(r.returncode, 1)
        self.assertIn("docs/a.md:1: spec-path: spec/v0.8 is not a manifest package (known: spec/v1.0)", r.stdout)
        self.assertIn("docs/a.md:1: spec-path: spec/v0.9 is not a manifest package", r.stdout)

    def test_walker_spellings_are_checked(self) -> None:
        self.write("ts/test/fixtures.ts", 'join(here, "spec", "v0.9", "conformance")\n')
        self.write("py/tests/_fixtures.py", 'SPEC_DIR / "v0.9" / "conformance"\n')
        self.write("rs/tests/support/mod.rs", '.join("v0.9").join("conformance")\n')
        self.write("php/tests/Corpus.php", "'v0.9' . DIRECTORY_SEPARATOR . 'conformance'\n")
        self.write("go/x.go", 'filepath.Join(spec, "v1.0", "conformance")\n')
        r = self.run_check()
        self.assertEqual(r.returncode, 1)
        for path in ("ts/test/fixtures.ts", "py/tests/_fixtures.py",
                     "rs/tests/support/mod.rs", "php/tests/Corpus.php"):
            self.assertIn(f"{path}:1: spec-path: spec/v0.9 is not a manifest package", r.stdout)
        self.assertNotIn("go/x.go", r.stdout)

    # --- rule 5: no language-tagged PRODID --------------------------------

    def test_language_tagged_prodid_fails(self) -> None:
        self.write("go/helpers/c.go", 'const defaultProdID = "-//hop-top//vstar-go v9.9.9//EN"\n')
        self.write("go/ok.go", 'const defaultProdID = "-//hop-top//vstar//EN"\n')
        r = self.run_check()
        self.assertEqual(r.returncode, 1)
        self.assertIn("go/helpers/c.go:1: prodid: vstar-go v9... is a language-tagged, version-bearing PRODID", r.stdout)
        self.assertNotIn("go/ok.go", r.stdout)


if __name__ == "__main__":
    unittest.main()
