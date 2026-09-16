// SPDX-License-Identifier: MIT

// The `validate` surface — V*'s semantic conformance checks — gated by
// `spec/behavior/validate/*.diagnostics.json`.
//
// Every `.ics` under `spec/behavior/validate/` has a same-stem
// `.diagnostics.json` sibling naming the exact findings the reference
// emits. The tree is walked rather than enumerated, so a fixture added
// to the reference becomes a case here without a test edit.
//
// Messages are deliberately absent from the fixtures: `Diagnostic.message`
// is human-readable and not part of the contract (see
// docs/validate-codes.md §Stability). Comparison is on
// `(code, severity, path)` only, sorted by `(path, code)`.

import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { parse as parseIcs } from "../src/codec/rfc5545/index.js";
import { CODES, CODE_SEVERITIES, type Code } from "../src/generated/codes.js";
import { COMP_ALARM, COMP_TODO } from "../src/types.js";
import type { Calendar, Component, Property } from "../src/types.js";
import {
  codes,
  severityOf,
  standardPropertyCount,
  validate,
  validateComponent,
  type Diagnostic,
} from "../src/validate/index.js";
import { BEHAVIOR_DIR, behaviorStems } from "./behavior.js";
import { CONFORMANCE_DIR } from "./fixtures.js";

/** The fields the fixtures pin. `message` is not one of them. */
interface DiagnosticRow {
  readonly code: string;
  readonly severity: string;
  readonly path: string;
}

/** `spec/behavior/validate/` — the layer-(d) gate. */
const VALIDATE_DIR = join(BEHAVIOR_DIR, "validate");

/** Sort key: `(path, code)`, per the fixture-comparison contract. */
function sortRows(rows: readonly DiagnosticRow[]): DiagnosticRow[] {
  return [...rows].sort((a, b) =>
    a.path < b.path ? -1 : a.path > b.path ? 1 : a.code < b.code ? -1 : a.code > b.code ? 1 : 0,
  );
}

/** A `Diagnostic` reduced to the three fields the fixtures compare. */
function toRow(d: Diagnostic): DiagnosticRow {
  return { code: d.code, severity: d.severity, path: d.path };
}

/** Parse `<stem>.ics` from `spec/behavior/validate/`. */
function loadCalendar(stem: string): Calendar {
  return parseIcs(new Uint8Array(readFileSync(join(VALIDATE_DIR, `${stem}.ics`))));
}

/** Read `<stem>.diagnostics.json`. */
function loadExpected(stem: string): DiagnosticRow[] {
  return JSON.parse(readFileSync(join(VALIDATE_DIR, `${stem}.diagnostics.json`), "utf8")) as DiagnosticRow[];
}

const STEMS = behaviorStems("validate", ".ics");

describe("validate — behavior fixtures", () => {
  it("the gate directory is not empty", () => {
    expect(STEMS.length).toBeGreaterThan(0);
  });

  for (const stem of STEMS) {
    it(stem, () => {
      const got = validate(loadCalendar(stem)).map(toRow);
      expect(sortRows(got)).toEqual(sortRows(loadExpected(stem)));
    });
  }
});

describe("validate — registry coverage", () => {
  it("every registry code is emitted by at least one fixture", () => {
    const emitted = new Set<string>();
    for (const stem of STEMS) {
      for (const d of validate(loadCalendar(stem))) emitted.add(d.code);
    }
    const missing = Object.values(CODES).filter((c) => !emitted.has(c));
    expect(missing).toEqual([]);
  });

  it("every fixture row's severity matches the registry", () => {
    for (const stem of STEMS) {
      for (const row of loadExpected(stem)) {
        expect(CODE_SEVERITIES[row.code as Code]).toBe(row.severity);
      }
    }
  });

  it("every emitted diagnostic's severity matches the registry", () => {
    for (const stem of STEMS) {
      for (const d of validate(loadCalendar(stem))) {
        expect(d.severity).toBe(CODE_SEVERITIES[d.code]);
      }
    }
  });
});

describe("validate — accessors", () => {
  it("codes() lists every registry code", () => {
    expect([...codes()].sort()).toEqual(Object.values(CODES).slice().sort());
  });

  it("severityOf() answers from the registry", () => {
    for (const code of Object.values(CODES)) {
      expect(severityOf(code)).toBe(CODE_SEVERITIES[code]);
    }
  });

  it("severityOf() is undefined for an unknown code", () => {
    expect(severityOf("NOT-A-CODE")).toBeUndefined();
  });

  it("standardPropertyCount() is positive", () => {
    expect(standardPropertyCount()).toBeGreaterThan(0);
  });
});

describe("validateComponent", () => {
  /** The lone component of a single-component fixture. */
  function soleComponent(stem: string): Component {
    const cal = loadCalendar(stem);
    expect(cal.components.length).toBe(1);
    return cal.components[0] as Component;
  }

  it("paths omit the VCALENDAR prefix", () => {
    const got = validateComponent(soleComponent("missing_dtstamp")).map(toRow);
    expect(got).toEqual([
      {
        code: CODES.CodeMissingDTSTAMP,
        severity: CODE_SEVERITIES[CODES.CodeMissingDTSTAMP],
        path: "VJOURNAL[uid=journal-no-dtstamp].DTSTAMP",
      },
    ]);
  });

  it("still emits the component-local supersession diagnostic", () => {
    const got = validateComponent(soleComponent("supersession_missing_props")).map((d) => d.code);
    expect(got).toContain(CODES.CodeSupersessionMissingProps);
  });

  it("skips the orphan-supersession diagnostic — it has no ledger", () => {
    // The orphan fixture's RELATED-TO resolves to nothing even inside
    // its own calendar; `validate` flags it. `validateComponent` sees
    // one component and cannot resolve anything, so it must stay
    // silent rather than guess.
    const comp = soleComponent("supersession_orphan");
    expect(validate(loadCalendar("supersession_orphan")).map((d) => d.code)).toContain(
      CODES.CodeSupersessionOrphan,
    );
    expect(validateComponent(comp).map((d) => d.code)).not.toContain(CODES.CodeSupersessionOrphan);
  });
});

describe("validate — corpus is clean of the status-vocabulary diagnostic", () => {
  // Mirrors the Go corpus walk: every VCALENDAR fixture in the
  // conformance corpus carries only STATUS values inside its own
  // component type's vocabulary. A hit here means the port's
  // status scoping is wrong, not the fixture.
  for (const family of ["rfc5545", "supersession"]) {
    const dir = join(CONFORMANCE_DIR, family);
    for (const name of readdirSync(dir).filter((n) => n.endsWith(".ics")).sort()) {
      it(`${family}/${name}`, () => {
        const cal = parseIcs(new Uint8Array(readFileSync(join(dir, name))));
        const hits = validate(cal).filter((d) => d.code === CODES.CodeStatusNotInVocabulary);
        expect(hits.map((d) => d.path)).toEqual([]);
      });
    }
  }

  it("a journal-only STATUS on a VEVENT is flagged", () => {
    // The cross-type shape the rule exists for: DRAFT is legal
    // iCalendar text, and legal for a VJOURNAL, but not for a VEVENT.
    const cal = loadCalendar("clean_vevent");
    const comp = cal.components[0] as Component;
    const status: Property = { name: "STATUS", params: [], value: "DRAFT" };
    comp.props = [...comp.props.filter((p) => p.name.toUpperCase() !== "STATUS"), status];
    expect(validate(cal).map((d) => d.code)).toContain(CODES.CodeStatusNotInVocabulary);
  });
});

describe("no hand-written diagnostic-code literals", () => {
  // The registry is authoritative: `spec/registry/` renders into
  // `src/generated/`, and hand-written source must reference the
  // generated constant by name. A literal spelled out by hand is a
  // second source of truth that drifts silently when the registry
  // changes. `make registry-check` guards the generated file; this
  // guards everything else.
  const NEEDLE = ["VS", "0"].join("");

  /** Every `.ts` file under `dir`, recursively. */
  function walk(dir: string): string[] {
    const out: string[] = [];
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const path = join(dir, entry.name);
      if (entry.isDirectory()) out.push(...walk(path));
      else if (entry.name.endsWith(".ts")) out.push(path);
    }
    return out;
  }

  const SRC = join(BEHAVIOR_DIR, "..", "..", "ts", "src");

  it("src/generated/ is the sole home of the code literals", () => {
    const offenders: string[] = [];
    for (const path of walk(SRC)) {
      if (path.includes(`${join("src", "generated")}`)) continue;
      const text = readFileSync(path, "utf8");
      for (const [i, line] of text.split("\n").entries()) {
        if (new RegExp(`${NEEDLE}\\d\\d`).test(line)) offenders.push(`${path}:${i + 1}: ${line.trim()}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("the guard actually fires — the generated module does contain them", () => {
    const generated = readFileSync(join(SRC, "generated", "codes.ts"), "utf8");
    expect(new RegExp(`${NEEDLE}\\d\\d`).test(generated)).toBe(true);
  });
});

// Rules the shared fixtures reach only weakly. Every `.ics` under
// spec/behavior/validate/ spells its property names in uppercase and
// reaches VS052 only through DURATION — so a port could fold neither of
// those cases correctly and still pass the whole behavior gate.
//
// The positional-path case is no longer in that category:
// missing_uid_positional carries two UID-less VTODOs, so the shared
// gate now pins [#0] against [#1] for every port. The local test is
// kept as the narrower, faster signal — it fails on the rule alone,
// without a fixture round-trip in the way.
//
// The Go reference pins these with unit tests of its own
// (validate_test.go, required_test.go, duration_test.go). These are the
// TypeScript counterparts; the gap was found independently by the
// Python, Rust and PHP validate lanes, each of which now carries its
// own equivalents.
describe("validate — rules the behavior fixtures reach only weakly", () => {
  const prop = (name: string, value: string): Property => ({ name, params: [], value });

  it("gives UID-less components of one type distinct positional paths", () => {
    const cal: Calendar = {
      prodId: "-//V*//Test//EN",
      components: [
        { type: COMP_TODO, props: [], sub: [] },
        { type: COMP_TODO, props: [], sub: [] },
      ],
    };
    const paths = validate(cal).map((d) => d.path);
    expect(paths.some((p) => p.startsWith("VCALENDAR.VTODO[#0]"))).toBe(true);
    expect(paths.some((p) => p.startsWith("VCALENDAR.VTODO[#1]"))).toBe(true);
  });

  it("matches property names case-insensitively", () => {
    // RFC 5545 §3.1 — names are case-insensitive, so lowercase
    // spellings still satisfy the required-property rules and must not
    // be reported as unknown.
    const c: Component = {
      type: COMP_TODO,
      props: [
        prop("uid", "todo-1"),
        prop("dtstamp", "20260504T120000Z"),
        prop("due", "20260601T000000Z"),
        prop("x-vstar-hash", "sha256:abc"),
      ],
      sub: [],
    };
    const got = validateComponent(c).map((d) => d.code);
    expect(got).not.toContain(CODES.CodeMissingUID);
    expect(got).not.toContain(CODES.CodeUnknownProperty);
  });

  it("flags a REPEAT that is not a non-negative integer", () => {
    const c: Component = {
      type: COMP_ALARM,
      props: [
        prop("ACTION", "DISPLAY"),
        prop("TRIGGER", "-PT15M"),
        prop("DURATION", "PT5M"),
        prop("REPEAT", "many"),
      ],
      sub: [],
    };
    expect(validateComponent(c).map((d) => d.code)).toContain(CODES.CodeMalformedDuration);
  });
});
