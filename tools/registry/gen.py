#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Render language constants and docs from the V* machine-readable registry.

The registry under ``spec/registry/`` is the single source of truth for
the tables every language port must carry identically: the diagnostic
code catalog, the RFC standard-property allow-list, the ``X-*``
extension scope tiers, and the enumerated value vocabularies.

Retyping those tables per language is how ports drift. This generator
renders them instead, from one input, into every target at once.

Usage::

    python3 tools/registry/gen.py gen            # write the outputs
    python3 tools/registry/gen.py gen --check    # fail on drift, write nothing
    python3 tools/registry/gen.py gen --strict   # write, fail if anything changed

``gen`` names every path it had to rewrite. Rewriting a generated file
restores a hand-edit, leaving the working tree clean, so ``git diff``
alone cannot see that someone edited the output instead of the registry;
the report is what keeps that edit visible.

Standard library only, by design: the generator has to run in every
port's CI, including ones with no Python dependency story.
"""

from __future__ import annotations

import argparse
import difflib
import json
import pathlib
import re
import sys
from typing import Any

# Repository root, two levels above tools/registry/gen.py.
ROOT = pathlib.Path(__file__).resolve().parents[2]
REGISTRY = ROOT / "spec" / "registry"
TEMPLATE = pathlib.Path(__file__).resolve().parent / "validate-codes.head.md"

# The banner every generated artifact carries. Rendered with each
# language's comment marker; the wording is identical everywhere so a
# grep for it finds the whole generated surface in any port.
BANNER_LINES = [
    "GENERATED — do not edit.",
    "Source: spec/registry/ · Generator: tools/registry/gen.py",
    "Run `make registry-gen` after editing the registry JSON.",
]

SEVERITY_GO = {"error": "SeverityError", "warning": "SeverityWarning"}

# Human-facing labels for the registry's RFC grouping keys.
RFC_LABELS = {
    "rfc5545": "RFC 5545 (iCalendar) §3.7-§3.8.",
    "rfc6350": "RFC 6350 (vCard) §6.",
}


# --------------------------------------------------------------------
# Registry loading
# --------------------------------------------------------------------


def load(name: str) -> Any:
    """Read one registry file, failing loudly with its path on error."""
    path = REGISTRY / name
    try:
        with path.open(encoding="utf-8") as handle:
            return json.load(handle)
    except FileNotFoundError:
        sys.exit(f"registry file missing: {path.relative_to(ROOT)}")
    except json.JSONDecodeError as err:
        sys.exit(f"registry file is not valid JSON: {path.relative_to(ROOT)}: {err}")


class Registry:
    """The whole registry, validated once at load time."""

    def __init__(self) -> None:
        self.codes = sorted(load("diagnostic-codes.json"), key=lambda c: c["code"])
        self.properties = load("standard-properties.json")
        self.scopes = load("extension-scopes.json")
        self.vocabulary = load("status-vocabulary.json")
        self._validate()

    def _validate(self) -> None:
        seen = set()
        for entry in self.codes:
            for field in ("code", "severity", "section", "rule"):
                if field not in entry:
                    sys.exit(f"diagnostic-codes.json: entry missing {field!r}: {entry}")
            code = entry["code"]
            if code in seen:
                sys.exit(f"diagnostic-codes.json: duplicate code {code}")
            seen.add(code)
            if entry["severity"] not in SEVERITY_GO:
                sys.exit(
                    f"diagnostic-codes.json: {code} has unknown severity "
                    f"{entry['severity']!r}; expected error or warning"
                )

        for key in ("CLASS", "TRANSP", "RELTYPE"):
            if key not in self.vocabulary:
                sys.exit(f"status-vocabulary.json: missing {key!r}")
            if not isinstance(self.vocabulary[key], list) or not self.vocabulary[key]:
                sys.exit(f"status-vocabulary.json: {key} must be a non-empty list")
        status = self.vocabulary.get("STATUS")
        if not isinstance(status, dict) or not status:
            sys.exit(
                "status-vocabulary.json: STATUS must be a non-empty object keyed "
                "by component type"
            )
        for comp, values in status.items():
            if not isinstance(values, list) or not values:
                sys.exit(
                    f"status-vocabulary.json: STATUS.{comp} must be a non-empty list"
                )

    # The STATUS vocabularies are per component type, so a renderer
    # needs the component order pinned as well as the values. Sorted
    # so every language emits the same order from the same input.
    @property
    def status_types(self) -> list[str]:
        """The component types that admit a STATUS, sorted."""
        return sorted(self.vocabulary["STATUS"])

    def status_values(self, comp: str) -> list[str]:
        """The STATUS values the registry scopes to ``comp``, in registry order.

        Registry order, not sorted: the vocabularies are RFC-ordered
        lists (``NEEDS-ACTION`` before ``COMPLETED``), and that order is
        visible to users through the VS044 diagnostic message, which
        joins the allowed values. Sorting here would reword every port's
        message and break the behavior fixtures.
        """
        return list(self.vocabulary["STATUS"][comp])

    def flat_vocabulary(self, key: str) -> list[str]:
        """One of the flat vocabularies (CLASS, TRANSP, RELTYPE), in order."""
        return list(self.vocabulary[key])

    @property
    def all_properties(self) -> list[str]:
        """Every standard property name, both RFCs, sorted."""
        return sorted(
            {name for group in self.properties.values() for name in group}
        )

    def sections(self) -> dict[str, list[dict[str, Any]]]:
        """Codes grouped by spec/05 criterion, each group sorted by code."""
        out: dict[str, list[dict[str, Any]]] = {}
        for entry in self.codes:
            out.setdefault(str(entry["section"]), []).append(entry)
        return out


# --------------------------------------------------------------------
# Rendering helpers
# --------------------------------------------------------------------


def banner(prefix: str, open_marker: str = "", close_marker: str = "") -> str:
    """Render the generated-file banner with a language's comment style."""
    lines = []
    if open_marker:
        lines.append(open_marker)
    lines.extend(f"{prefix}{line}".rstrip() for line in BANNER_LINES)
    if close_marker:
        lines.append(close_marker)
    return "\n".join(lines)


def go_ident(code: str) -> str:
    """Map a code to its exported Go constant name.

    The names predate the registry and are part of the published Go API,
    so they are pinned here rather than derived: a derivation would have
    to reproduce every historical spelling choice exactly, and a silent
    mismatch would be an API break.
    """
    return GO_NAMES[code]


GO_NAMES = {
    "VS001": "CodeMissingUID",
    "VS002": "CodeMissingDTSTAMP",
    "VS003": "CodeMissingXVSTARHash",
    "VS010": "CodeBadXVSTARHash",
    "VS020": "CodeUnknownProperty",
    "VS030": "CodeSupersessionMissingProps",
    "VS031": "CodeSupersessionOrphan",
    "VS040": "CodeVTODOMissingDue",
    "VS041": "CodeVEVENTMissingDTSTART",
    "VS042": "CodeVFREEBUSYMissingTimes",
    "VS043": "CodeVCARDMissingRequired",
    "VS044": "CodeStatusNotInVocabulary",
    "VS050": "CodeRRuleUnsupported",
    "VS051": "CodeRRuleMalformed",
    "VS052": "CodeMalformedDuration",
}


def wrap_comment(text: str, prefix: str, width: int = 68) -> list[str]:
    """Wrap ``text`` into comment lines of at most ``width`` columns.

    ``textwrap`` is not used: it counts a leading tab as one column, so
    tab-indented Go comments come out over-long under gofumpt's view of
    the line. Counting the prefix ourselves keeps the arithmetic honest.
    """
    lines: list[str] = []
    current = ""
    for word in text.split():
        candidate = f"{current} {word}" if current else word
        if current and len(prefix) + len(candidate) > width:
            lines.append(prefix + current)
            current = word
        else:
            current = candidate
    if current:
        lines.append(prefix + current)
    return lines


# A Markdown inline link: [text](target).
_MD_LINK = re.compile(r"\[([^\]]+)\]\([^)]*\)")


def plain(text: str) -> str:
    """Reduce a Markdown rule to prose fit for a source comment.

    Backticks and link targets are Markdown's business; a Go doc comment
    or a PHPDoc block wants the sentence, not the URL. Link text is kept
    so the cross-reference still reads, only the destination is dropped.
    """
    text = _MD_LINK.sub(r"\1", text)
    return text.replace("`", "")


def align_map(entries: list[tuple[str, str]], indent: str = "\t") -> list[str]:
    """Render ``key: value,`` lines with gofmt's column alignment.

    gofmt aligns the values of a composite literal across each run of
    adjacent lines, restarting at a blank line or a comment. Emitting the
    alignment here — rather than shelling out to gofumpt — keeps `gen`
    and `gen --check` byte-identical without a toolchain dependency.
    """
    width = max((len(k) for k, _ in entries), default=0)
    return [f"{indent}{k + ':':<{width + 1}} {v}," for k, v in entries]


def go_string(value: str) -> str:
    """Render a Go/TS/PHP double-quoted string literal."""
    return json.dumps(value, ensure_ascii=False)


# --------------------------------------------------------------------
# Renderers, one per output
# --------------------------------------------------------------------


def render_go(reg: Registry) -> str:
    out = ["// SPDX-License-Identifier: MIT", "", banner("// "), "", "package validate", ""]

    out.append("// Diagnostic codes. Each is stable across minor versions: a")
    out.append("// code never changes meaning and is never recycled. See")
    out.append("// docs/validate-codes.md for the full catalog.")
    out.append("const (")
    for i, entry in enumerate(reg.codes):
        if i:
            out.append("")
        name = go_ident(entry["code"])
        for line in wrap_comment(
            f"{name} — {plain(entry['rule'])} (spec/05 criterion {entry['section']})",
            "\t// ",
        ):
            out.append(line)
        out.append(f"\t{name} = {go_string(entry['code'])}")
    out.append(")")
    out.append("")

    out.append("// codeSeverities maps every diagnostic code to the severity the")
    out.append("// registry assigns it. Emitters use the constants directly; this")
    out.append("// map exists so tooling can classify a code it did not emit.")
    out.append("var codeSeverities = map[string]Severity{")
    out.extend(
        align_map(
            [
                (go_ident(e["code"]), SEVERITY_GO[e["severity"]])
                for e in reg.codes
            ]
        )
    )
    out.append("}")
    out.append("")

    out.append("// SeverityOf reports the severity the registry assigns to code,")
    out.append("// and whether the code is known at all. An unknown code yields")
    out.append("// (SeverityError, false) — callers must check ok before acting.")
    out.append("func SeverityOf(code string) (Severity, bool) {")
    out.append("\ts, ok := codeSeverities[code]")
    out.append("\treturn s, ok")
    out.append("}")
    out.append("")

    out.append("// Codes returns every diagnostic code the package defines, sorted.")
    out.append("// The slice is freshly allocated on each call, so callers may")
    out.append("// retain or mutate it.")
    out.append("func Codes() []string {")
    out.append("\treturn []string{")
    for entry in reg.codes:
        out.append(f"\t\t{go_ident(entry['code'])},")
    out.append("\t}")
    out.append("}")
    out.append("")

    out.append("// standardProperties is the allow-list of property names defined")
    out.append("// by RFC 5545 (iCalendar) §3.7-§3.8 and RFC 6350 (vCard) §6. A")
    out.append("// name that is absent here AND does not begin with \"X-\" earns a")
    out.append("// SeverityWarning VS020 from spec/05 criterion 3.")
    out.append("//")
    out.append("// Keys are upper-cased; lookup is case-insensitive at the call")
    out.append("// site (see isStandardProperty).")
    out.append("var standardProperties = map[string]struct{}{")
    for rfc in sorted(reg.properties):
        out.append(f"\t// {RFC_LABELS.get(rfc, rfc.upper())}")
        out.extend(
            align_map([(go_string(n), "{}") for n in sorted(reg.properties[rfc])])
        )
    out.append("}")
    out.append("")

    out.append("// StandardPropertyCount returns the number of property names in")
    out.append("// the standard allow-list. Exported for diagnostic and test code")
    out.append("// — not part of the validator hot path.")
    out.append("func StandardPropertyCount() int {")
    out.append("\treturn len(standardProperties)")
    out.append("}")
    out.append("")

    out.append("// StatusVocabulary is the registry's STATUS value vocabulary,")
    out.append("// keyed by the component type RFC 5545 §3.8.1.11 scopes it to.")
    out.append("// A type absent from the map admits no STATUS vocabulary.")
    out.append("//")
    out.append("// This is the cross-language table, not the Go wire constants.")
    out.append("// The validator keeps deriving its table from the constants the")
    out.append("// codec encodes against — see status_vocabulary.go — and a test")
    out.append("// asserts the two agree. That way the table cannot drift from")
    out.append("// the codec, and neither can drift from the other ports.")
    out.append("var StatusVocabulary = map[string][]string{")
    out.extend(
        align_map(
            [
                (
                    go_string(comp),
                    "{" + ", ".join(go_string(v) for v in reg.status_values(comp)) + "}",
                )
                for comp in reg.status_types
            ]
        )
    )
    out.append("}")
    out.append("")

    for name, key, doc in (
        (
            "ClassVocabulary",
            "CLASS",
            "ClassVocabulary is the registry's CLASS value vocabulary, "
            "RFC 5545 §3.8.1.3.",
        ),
        (
            "TranspVocabulary",
            "TRANSP",
            "TranspVocabulary is the registry's TRANSP value vocabulary, "
            "RFC 5545 §3.8.2.7.",
        ),
        (
            "RelTypeVocabulary",
            "RELTYPE",
            "RelTypeVocabulary is the registry's registered RELTYPE "
            "vocabulary, RFC 5545 §3.2.15 plus RFC 9253 §11.4. The "
            "vocabulary is registered, not closed: RELTYPE admits IANA "
            "and X- values outside it.",
        ),
    ):
        out.extend(wrap_comment(doc, "// "))
        # One value per line rather than a single literal: RELTYPE's
        # twelve values run well past a readable column, and gofumpt
        # leaves a long one-liner alone rather than splitting it.
        out.append(f"var {name} = []string{{")
        for value in reg.flat_vocabulary(key):
            out.append(f"\t{go_string(value)},")
        out.append("}")
        out.append("")

    return "\n".join(out).rstrip("\n") + "\n"


def md_table(headers: list[str], rows: list[list[str]]) -> str:
    """Render a Markdown table, padding columns to a common width."""
    widths = [len(h) for h in headers]
    for row in rows:
        for i, cell in enumerate(row):
            widths[i] = max(widths[i], len(cell))
    out = ["| " + " | ".join(h.ljust(widths[i]) for i, h in enumerate(headers)) + " |"]
    out.append("|" + "|".join("-" * (w + 2) for w in widths) + "|")
    for row in rows:
        out.append(
            "| " + " | ".join(cell.ljust(widths[i]) for i, cell in enumerate(row)) + " |"
        )
    return "\n".join(out)


def render_docs(reg: Registry) -> str:
    template = TEMPLATE.read_text(encoding="utf-8")
    sections = reg.sections()

    for section, entries in sections.items():
        rows = [
            [e["code"], e["severity"].capitalize(), e["rule"]]
            for e in entries
        ]
        table = md_table(["Code", "Severity", "Rule"], rows)
        placeholder = "{{CODES_TABLE:%s}}" % section
        if placeholder not in template:
            sys.exit(
                f"{TEMPLATE.name}: no {placeholder} for section {section}; "
                "add the section heading and placeholder to the template"
            )
        template = template.replace(placeholder, table)

    status = reg.vocabulary["STATUS"]
    rows = [
        [f"`{comp}`", ", ".join(f"`{v}`" for v in status[comp])]
        for comp in sorted(status)
    ]
    template = template.replace(
        "{{STATUS_VOCABULARY_TABLE}}",
        md_table(["Component", "Allowed `STATUS` values"], rows),
    )

    leftovers = [line for line in template.splitlines() if "{{" in line]
    if leftovers:
        sys.exit(f"{TEMPLATE.name}: unrendered placeholders: {leftovers}")

    return template


def render_ts(reg: Registry) -> str:
    out = [banner(" * ", "/**", " */"), ""]
    out.append("export const CODES = {")
    for entry in reg.codes:
        out.append(f"  /** {plain(entry['rule'])} */")
        out.append(f"  {go_ident(entry['code'])}: {go_string(entry['code'])},")
    out.append("} as const;")
    out.append("")
    out.append("export type Code = (typeof CODES)[keyof typeof CODES];")
    out.append("")
    out.append("export type Severity = \"error\" | \"warning\";")
    out.append("")
    out.append("export const CODE_SEVERITIES: Readonly<Record<Code, Severity>> = {")
    for entry in reg.codes:
        out.append(f"  {go_string(entry['code'])}: {go_string(entry['severity'])},")
    out.append("};")
    out.append("")
    out.append("export const STANDARD_PROPERTIES: readonly string[] = [")
    for name in reg.all_properties:
        out.append(f"  {go_string(name)},")
    out.append("];")
    out.append("")
    out.append("/**")
    out.append(" * The registry's STATUS value vocabulary, keyed by the component")
    out.append(" * type RFC 5545 §3.8.1.11 scopes it to. A type absent from this")
    out.append(" * record admits no STATUS vocabulary.")
    out.append(" *")
    out.append(" * This is the cross-language table, not the port's wire constants.")
    out.append(" * The validator keeps deriving its table from the constants the")
    out.append(" * codec encodes against, and a test asserts the two agree — so the")
    out.append(" * table cannot drift from the codec, nor from the other ports.")
    out.append(" */")
    out.append(
        "export const STATUS_VOCABULARY: Readonly<Record<string, readonly string[]>> = {"
    )
    for comp in reg.status_types:
        values = ", ".join(go_string(v) for v in reg.status_values(comp))
        out.append(f"  {go_string(comp)}: [{values}],")
    out.append("};")
    for name, key, doc in TS_FLAT_VOCABULARIES:
        out.append("")
        if len(doc) + 7 <= 76:
            out.append(f"/** {doc} */")
        else:
            out.append("/**")
            out.extend(wrap_comment(doc, " * ", width=76))
            out.append(" */")
        out.append(f"export const {name}: readonly string[] = [")
        for value in reg.flat_vocabulary(key):
            out.append(f"  {go_string(value)},")
        out.append("];")
    return "\n".join(out) + "\n"


# The flat vocabularies, with the one-line doc each language repeats.
TS_FLAT_VOCABULARIES = [
    ("CLASS_VOCABULARY", "CLASS", "The registry's CLASS vocabulary, RFC 5545 §3.8.1.3."),
    ("TRANSP_VOCABULARY", "TRANSP", "The registry's TRANSP vocabulary, RFC 5545 §3.8.2.7."),
    (
        "RELTYPE_VOCABULARY",
        "RELTYPE",
        "The registry's registered RELTYPE vocabulary, RFC 5545 §3.2.15 plus "
        "RFC 9253 §11.4. Registered, not closed: RELTYPE admits IANA and X- "
        "values outside it.",
    ),
]


def render_py(reg: Registry) -> str:
    out = [banner("# "), "", "from __future__ import annotations", "", "from typing import Final", ""]
    out.append("")
    for entry in reg.codes:
        out.append(f"#: {plain(entry['rule'])}")
        name = go_ident(entry["code"]).removeprefix("Code")
        out.append(f"{_snake(name).upper()}: Final[str] = {go_string(entry['code'])}")
        out.append("")
    out.append("CODE_SEVERITIES: Final[dict[str, str]] = {")
    for entry in reg.codes:
        out.append(f"    {go_string(entry['code'])}: {go_string(entry['severity'])},")
    out.append("}")
    out.append("")
    out.append("STANDARD_PROPERTIES: Final[frozenset[str]] = frozenset(")
    out.append("    (")
    for name in reg.all_properties:
        out.append(f"        {go_string(name)},")
    out.append("    )")
    out.append(")")
    out.append("")
    out.append("#: The registry's STATUS vocabulary, keyed by the component type")
    out.append("#: RFC 5545 §3.8.1.11 scopes it to. A type absent from this mapping")
    out.append("#: admits no STATUS vocabulary.")
    out.append("#:")
    out.append("#: This is the cross-language table, not the port's wire constants.")
    out.append("#: The validator keeps deriving its table from the constants the")
    out.append("#: codec encodes against, and a test asserts the two agree — so the")
    out.append("#: table cannot drift from the codec, nor from the other ports.")
    out.append("STATUS_VOCABULARY: Final[dict[str, tuple[str, ...]]] = {")
    for comp in reg.status_types:
        values = ", ".join(go_string(v) for v in reg.status_values(comp))
        out.append(f"    {go_string(comp)}: ({values},),")
    out.append("}")
    for name, key, doc in TS_FLAT_VOCABULARIES:
        out.append("")
        out.extend(wrap_comment(doc, "#: "))
        out.append(f"{name}: Final[tuple[str, ...]] = (")
        for value in reg.flat_vocabulary(key):
            out.append(f"    {go_string(value)},")
        out.append(")")
    return "\n".join(out) + "\n"


def render_rs(reg: Registry) -> str:
    out = [banner("// "), ""]
    for entry in reg.codes:
        name = _snake(go_ident(entry["code"]).removeprefix("Code")).upper()
        out.append(f"/// {plain(entry['rule'])}")
        out.append(f"pub const {name}: &str = {go_string(entry['code'])};")
        out.append("")
    out.append("/// Every diagnostic code, sorted by code.")
    out.append(f"pub const CODES: [&str; {len(reg.codes)}] = [")
    for entry in reg.codes:
        out.append(f"    {go_string(entry['code'])},")
    out.append("];")
    out.append("")
    out.append("/// Severity for each code, index-aligned with `CODES`.")
    out.append(f"pub const CODE_SEVERITIES: [(&str, &str); {len(reg.codes)}] = [")
    for entry in reg.codes:
        out.append(
            f"    ({go_string(entry['code'])}, {go_string(entry['severity'])}),"
        )
    out.append("];")
    out.append("")
    out.append("/// RFC 5545 / RFC 6350 standard property allow-list.")
    out.append(
        f"pub const STANDARD_PROPERTIES: [&str; {len(reg.all_properties)}] = ["
    )
    for name in reg.all_properties:
        out.append(f"    {go_string(name)},")
    out.append("];")
    out.append("")
    out.append("/// The registry's `STATUS` vocabulary, paired with the component")
    out.append("/// type RFC 5545 §3.8.1.11 scopes it to. A type absent from this")
    out.append("/// table admits no `STATUS` vocabulary.")
    out.append("///")
    out.append("/// This is the cross-language table, not the crate's wire enums.")
    out.append("/// The validator keeps deriving its table from the enums the codec")
    out.append("/// encodes against, and a test asserts the two agree — so the table")
    out.append("/// cannot drift from the codec, nor from the other ports.")
    out.append(
        f"pub const STATUS_VOCABULARY: [(&str, &[&str]); {len(reg.status_types)}] = ["
    )
    for comp in reg.status_types:
        values = ", ".join(go_string(v) for v in reg.status_values(comp))
        out.append(f"    ({go_string(comp)}, &[{values}]),")
    out.append("];")
    for name, key, doc in TS_FLAT_VOCABULARIES:
        values = reg.flat_vocabulary(key)
        out.append("")
        out.extend(wrap_comment(doc, "/// "))
        out.append(f"pub const {name}: [&str; {len(values)}] = [")
        for value in values:
            out.append(f"    {go_string(value)},")
        out.append("];")
    return "\n".join(out) + "\n"


def render_php(reg: Registry) -> str:
    out = ["<?php", "", "declare(strict_types=1);", "", banner("// "), "", "namespace HopTop\\Vstar\\Generated;", ""]
    out.append("final class Codes")
    out.append("{")
    for entry in reg.codes:
        name = _snake(go_ident(entry["code"]).removeprefix("Code")).upper()
        out.append(f"    /** {plain(entry['rule'])} */")
        out.append(f"    public const {name} = {go_string(entry['code'])};")
        out.append("")
    out.append("    /** @var array<string, string> */")
    out.append("    public const SEVERITIES = [")
    for entry in reg.codes:
        out.append(
            f"        {go_string(entry['code'])} => {go_string(entry['severity'])},"
        )
    out.append("    ];")
    out.append("")
    out.append("    /** @var list<string> */")
    out.append("    public const STANDARD_PROPERTIES = [")
    for name in reg.all_properties:
        out.append(f"        {go_string(name)},")
    out.append("    ];")
    out.append("")
    out.append("    /**")
    out.append("     * The registry's STATUS vocabulary, keyed by the component type")
    out.append("     * RFC 5545 §3.8.1.11 scopes it to. A type absent from this table")
    out.append("     * admits no STATUS vocabulary.")
    out.append("     *")
    out.append("     * This is the cross-language table, not the port's backed enums.")
    out.append("     * The validator keeps deriving its table from the enums the codec")
    out.append("     * encodes against, and a test asserts the two agree -- so the")
    out.append("     * table cannot drift from the codec, nor from the other ports.")
    out.append("     *")
    out.append("     * @var array<string, list<string>>")
    out.append("     */")
    out.append("    public const STATUS_VOCABULARY = [")
    for comp in reg.status_types:
        values = ", ".join(go_string(v) for v in reg.status_values(comp))
        out.append(f"        {go_string(comp)} => [{values}],")
    out.append("    ];")
    for name, key, doc in TS_FLAT_VOCABULARIES:
        out.append("")
        out.append("    /**")
        for line in wrap_comment(plain(doc), "     * ", width=72):
            out.append(line)
        out.append("     *")
        out.append("     * @var list<string>")
        out.append("     */")
        out.append(f"    public const {name} = [")
        for value in reg.flat_vocabulary(key):
            out.append(f"        {go_string(value)},")
        out.append("    ];")
    out.append("}")
    return "\n".join(out) + "\n"


def _snake(name: str) -> str:
    """CamelCase to snake_case, keeping acronym runs together."""
    out: list[str] = []
    for i, ch in enumerate(name):
        if ch.isupper() and i and not (name[i - 1].isupper() and (i + 1 == len(name) or name[i + 1].isupper())):
            out.append("_")
        out.append(ch)
    return "".join(out).lower()


OUTPUTS = {
    "go/validate/codes_gen.go": render_go,
    "docs/validate-codes.md": render_docs,
    "ts/src/generated/codes.ts": render_ts,
    "py/src/vstar/_generated/codes.py": render_py,
    "rs/src/generated/codes.rs": render_rs,
    "php/src/Generated/Codes.php": render_php,
}


# --------------------------------------------------------------------
# Commands
# --------------------------------------------------------------------


def cmd_gen(check: bool, strict: bool = False) -> int:
    reg = Registry()
    rendered = {path: fn(reg) for path, fn in sorted(OUTPUTS.items())}

    if not check:
        # Diff before writing. Rewriting a generated file restores a
        # hand-edit, so the working tree goes clean again and `git diff`
        # cannot see that someone edited the output instead of the
        # registry. Report the paths that actually changed and exit
        # non-zero, so the edit stays visible — the same contract
        # fixtures-gen-behavior and fixtures-verify already carry.
        changed = []
        for rel, content in rendered.items():
            target = ROOT / rel
            before = target.read_text(encoding="utf-8") if target.exists() else None
            if before != content:
                changed.append(rel)
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(content, encoding="utf-8")
        if changed:
            print(
                f"registry-gen: {len(changed)} generated file(s) differed from "
                "what was on disk and were rewritten:",
                file=sys.stderr,
            )
            for rel in changed:
                print(f"  {rel}", file=sys.stderr)
            print(
                "These files are generated: change spec/registry/ or the "
                "generator,\nthen commit the regenerated files alongside the "
                "change.",
                file=sys.stderr,
            )
            # --strict is what a gate runs: writing is still useful
            # there (the diff lands in the tree for inspection), but a
            # rewrite has to fail the build. Plain `gen` is the
            # authoring entrypoint and stays exit 0, because writing is
            # the whole point of running it.
            return 1 if strict else 0
        print(f"registry-gen: {len(rendered)} generated files match spec/registry/")
        return 0

    drift = []
    for rel, want in rendered.items():
        target = ROOT / rel
        got = target.read_text(encoding="utf-8") if target.exists() else None
        if got == want:
            continue
        drift.append(rel)
        if got is None:
            print(f"registry-check: {rel} is missing", file=sys.stderr)
            continue
        print(f"registry-check: {rel} differs from the registry:", file=sys.stderr)
        diff = difflib.unified_diff(
            got.splitlines(keepends=True),
            want.splitlines(keepends=True),
            fromfile=f"{rel} (on disk)",
            tofile=f"{rel} (from registry)",
            n=2,
        )
        sys.stderr.writelines(diff)
        sys.stderr.write("\n")

    if drift:
        print(
            "registry-check: drift in "
            + ", ".join(drift)
            + "\nRun 'make registry-gen' and commit the result, or revert the "
            "edit to the generated file and change spec/registry/ instead.",
            file=sys.stderr,
        )
        return 1

    print(f"registry-check: {len(rendered)} generated files match spec/registry/")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="gen.py",
        description="Render language constants and docs from spec/registry/.",
    )
    sub = parser.add_subparsers(dest="command", required=True)
    gen = sub.add_parser("gen", help="render the generated files")
    gen.add_argument(
        "--check",
        action="store_true",
        help="do not write; exit 1 when any output differs from the registry",
    )
    gen.add_argument(
        "--strict",
        action="store_true",
        help="write, but exit 1 when any output had to be rewritten",
    )
    args = parser.parse_args(argv)
    if args.command == "gen":
        if args.check and args.strict:
            parser.error("--check and --strict are mutually exclusive")
        return cmd_gen(args.check, args.strict)
    parser.error(f"unknown command {args.command!r}")
    return 2


if __name__ == "__main__":
    sys.exit(main())
