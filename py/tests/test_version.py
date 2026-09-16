# SPDX-License-Identifier: MIT

from __future__ import annotations

import re

import vstar


def test_version_is_pep440() -> None:
    assert re.fullmatch(r"\d+\.\d+\.\d+(?:[ab]|rc)?\d*(?:\.dev\d+)?", vstar.__version__)


def test_version_is_exported() -> None:
    assert "__version__" in vstar.__all__
