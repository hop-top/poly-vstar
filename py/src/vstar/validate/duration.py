# SPDX-License-Identifier: MIT

"""spec/03 rule 12, spec/05 criterion 7 — DURATION well-formedness."""

from __future__ import annotations

import re
from typing import Final

from .. import duration as _duration
from .._generated.codes import MALFORMED_DURATION
from ..types import Component, Property
from ._internal import Diagnostic, diagnostic, equal_fold

__all__ = ["check_duration"]

_DURATION: Final[str] = "DURATION"
_TRIGGER: Final[str] = "TRIGGER"
_REPEAT: Final[str] = "REPEAT"

#: A non-negative decimal integer, anchored.
#:
#: Deliberately a pattern rather than :func:`int`, which accepts
#: surrounding whitespace, a leading ``+``, and underscore separators —
#: none of which RFC 5545 §3.8.6.2 admits on the wire.
_NON_NEGATIVE_INT: Final[re.Pattern[str]] = re.compile(r"\A[0-9]+\Z")


def check_duration(c: Component, path: str) -> list[Diagnostic]:
    """One diagnostic per duration-bearing property whose value is malformed.

    TRIGGER goes through :func:`vstar.duration.parse_trigger` rather
    than a bare duration parse so the two value forms and the RELATED /
    VALUE parameter rules are enforced together — an absolute trigger is
    not a duration, and rejecting it as one would be wrong. DURATION and
    REPEAT are checked directly: each has exactly one legal shape.
    """
    out: list[Diagnostic] = []
    for p in c.props:
        if equal_fold(p.name, _TRIGGER):
            out.extend(_check_trigger(p, path))
        elif equal_fold(p.name, _DURATION):
            out.extend(_check_duration_property(p, path))
        elif equal_fold(p.name, _REPEAT):
            out.extend(_check_repeat(p, path))
    return out


def _check_trigger(p: Property, path: str) -> list[Diagnostic]:
    """Flag a TRIGGER whose value or parameters do not parse."""
    try:
        _duration.parse_trigger(p)
    except Exception as e:
        return [
            diagnostic(
                MALFORMED_DURATION,
                f"TRIGGER is malformed (RFC 5545 §3.8.6.3): {e}",
                f"{path}.{_TRIGGER}",
            )
        ]
    return []


def _check_duration_property(p: Property, path: str) -> list[Diagnostic]:
    """Flag a DURATION whose value does not parse."""
    try:
        _duration.parse(p.value)
    except Exception as e:
        return [
            diagnostic(
                MALFORMED_DURATION,
                f"DURATION is malformed (RFC 5545 §3.3.6): {e}",
                f"{path}.{_DURATION}",
            )
        ]
    return []


def _check_repeat(p: Property, path: str) -> list[Diagnostic]:
    """Flag a REPEAT that is not a non-negative decimal integer."""
    if _NON_NEGATIVE_INT.match(p.value):
        return []
    return [
        diagnostic(
            MALFORMED_DURATION,
            f"REPEAT is not a non-negative integer (RFC 5545 §3.8.6.2): {p.value}",
            f"{path}.{_REPEAT}",
        )
    ]
