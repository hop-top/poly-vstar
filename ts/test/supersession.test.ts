// SPDX-License-Identifier: MIT

// The `supersession` surface — V*'s append-only state-change discipline
// — gated by `spec/behavior/supersession/*.effective.json` over the
// conformance corpus inputs.

import { readdirSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { component as canonicalComponent } from "../src/canonical/index.js";
import { parse as parseIcs } from "../src/codec/rfc5545/index.js";
import { VstarError } from "../src/errors.js";
import { getXVstar, setXVstar, verifyXVstar } from "../src/hashing/index.js";
import {
  CATEGORY_STATUS_SUPERSESSION,
  PROP_EFFECTIVE_STATUS,
  supersedes,
  superseded,
} from "../src/supersession/index.js";
import { formatTime, parseTime } from "../src/time.js";
import type { Calendar, Component, Property } from "../src/types.js";
import { behaviorStems, loadBehaviorJson, type EffectiveStatusCase } from "./behavior.js";
import { CONFORMANCE_DIR } from "./fixtures.js";
import { readFileSync } from "node:fs";
import { get, uid as componentUid } from "../src/types.js";

function prop(name: string, value: string): Property {
  return { name, params: [], value };
}

/** The conformance calendar supplying a behavior-family supersession case. */
function loadConformanceCalendar(stem: string): Calendar {
  return parseIcs(
    new Uint8Array(readFileSync(join(CONFORMANCE_DIR, "supersession", `${stem}.ics`))),
  );
}

/** A hashed VTODO, the shape `supersedes` is normally handed. */
function hashedTodo(uid: string, summary = "Ship the port"): Component {
  const c: Component = {
    type: "VTODO",
    props: [prop("UID", uid), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", summary)],
    sub: [],
  };
  setXVstar(c);
  return c;
}

const AT = parseTime("20260504T143000Z") ?? 0;

describe("supersession effective status — spec/behavior/supersession/", () => {
  const stems = behaviorStems("supersession", ".effective.json");

  it("walks the whole family", () => {
    expect(stems.length).toBe(5);
    expect(stems).toEqual([
      "corrupt_mutated",
      "cross_component",
      "effective_status_vevent",
      "linear",
      "multi_step",
    ]);
  });

  it("keys the table by the conformance corpus's own file names", () => {
    // The family mints no new .ics: it projects the existing
    // spec/v1.0/conformance/supersession/ inputs, so a port already
    // loading that corpus gets this table for free.
    const corpus = new Set(
      readdirSync(join(CONFORMANCE_DIR, "supersession"))
        .filter((n) => n.endsWith(".ics"))
        .map((n) => n.slice(0, -".ics".length)),
    );
    for (const stem of stems) expect(corpus.has(stem)).toBe(true);
  });

  for (const stem of behaviorStems("supersession", ".effective.json")) {
    it(`${stem} projects the fixture's table`, () => {
      const cal = loadConformanceCalendar(stem);
      const want = loadBehaviorJson<EffectiveStatusCase>("supersession", `${stem}.effective.json`);

      const got: Record<string, string> = {};
      for (const c of cal.components) {
        const status = superseded(c, cal.components);
        if (status === undefined) continue;
        const u = componentUid(c);
        expect(u).not.toBe("");
        got[u] = status;
      }
      expect(got).toEqual(want);
    });
  }

  it("projects nothing for a corrupt target — supersession is a query, not a validator", () => {
    // corrupt_mutated's hash is broken, but no supersession entry
    // targets it, so the projection is empty. The hash violation is the
    // validate family's business.
    const cal = loadConformanceCalendar("corrupt_mutated");
    const target = cal.components[0] as Component;
    expect(verifyXVstar(target).ok).toBe(false);
    expect(superseded(target, cal.components)).toBeUndefined();
  });

  it("lets the later DTSTAMP win when two entries target one component", () => {
    const cal = loadConformanceCalendar("multi_step");
    const target = cal.components.find((c) => c.type === "VTODO") as Component;
    // The ledger carries IN-PROCESS at 14:30 and COMPLETED at 16:45.
    expect(superseded(target, cal.components)).toBe("COMPLETED");
  });
});

describe("supersedes", () => {
  it("writes the five properties in order, hash last", () => {
    const target = hashedTodo("todo-1");
    const j = supersedes(target, "COMPLETED", AT);

    expect(j.type).toBe("VJOURNAL");
    expect(j.props.map((p) => p.name)).toEqual([
      "UID",
      "DTSTAMP",
      "RELATED-TO",
      "CATEGORIES",
      PROP_EFFECTIVE_STATUS,
      "X-VSTAR-HASH",
    ]);
  });

  it("derives a deterministic UID from the target and the instant", () => {
    const j = supersedes(hashedTodo("todo-1"), "COMPLETED", AT);
    expect(get(j, "UID")?.value).toBe(`journal:status:todo-1:${formatTime(AT)}`);
  });

  it("carries the spec-blessed category and the new status verbatim", () => {
    const j = supersedes(hashedTodo("todo-1"), "COMPLETED", AT);
    expect(get(j, "CATEGORIES")?.value).toBe(CATEGORY_STATUS_SUPERSESSION);
    expect(get(j, PROP_EFFECTIVE_STATUS)?.value).toBe("COMPLETED");
    expect(get(j, "RELATED-TO")?.value).toBe("todo-1");
  });

  it("treats the status as opaque — any vocabulary is admitted", () => {
    const j = supersedes(hashedTodo("todo-1"), "SHIPPED-TO-PROD", AT);
    expect(get(j, PROP_EFFECTIVE_STATUS)?.value).toBe("SHIPPED-TO-PROD");
  });

  it("stamps a hash covering everything it just wrote", () => {
    const j = supersedes(hashedTodo("todo-1"), "COMPLETED", AT);
    expect(verifyXVstar(j).ok).toBe(true);
  });

  it("does not mutate the target — the ledger is append-only", () => {
    const target = hashedTodo("todo-1");
    const before = canonicalComponent(target);
    supersedes(target, "COMPLETED", AT);
    expect(canonicalComponent(target)).toEqual(before);
  });

  it("throws ErrTargetCorrupted when the target's stored hash fails verification", () => {
    const cal = loadConformanceCalendar("corrupt_mutated");
    const target = cal.components[0] as Component;
    expect(() => supersedes(target, "COMPLETED", AT)).toThrow(VstarError);
    try {
      supersedes(target, "COMPLETED", AT);
      expect.unreachable("supersedes accepted a corrupted target");
    } catch (e) {
      expect((e as VstarError).code).toBe("ErrTargetCorrupted");
    }
  });

  it("refuses a target mutated after it was hashed", () => {
    const target = hashedTodo("todo-1");
    // The mutation the append-only discipline forbids: change content,
    // leave the stored hash behind.
    (target.props[2] as Property).value = "MUTATED";
    try {
      supersedes(target, "COMPLETED", AT);
      expect.unreachable("supersedes accepted a mutated target");
    } catch (e) {
      expect((e as VstarError).code).toBe("ErrTargetCorrupted");
    }
  });

  it("accepts a target carrying no hash — it makes no integrity claim", () => {
    const target: Component = {
      type: "VTODO",
      props: [prop("UID", "todo-nohash"), prop("DTSTAMP", "20260504T120000Z")],
      sub: [],
    };
    expect(getXVstar(target)).toBeUndefined();
    const j = supersedes(target, "COMPLETED", AT);
    expect(get(j, "RELATED-TO")?.value).toBe("todo-nohash");
  });
});

describe("superseded", () => {
  const journal = (
    uid: string,
    relatedTo: string,
    status: string,
    dtstamp: string,
    categories = CATEGORY_STATUS_SUPERSESSION,
  ): Component => ({
    type: "VJOURNAL",
    props: [
      prop("UID", uid),
      prop("DTSTAMP", dtstamp),
      prop("RELATED-TO", relatedTo),
      prop("CATEGORIES", categories),
      prop(PROP_EFFECTIVE_STATUS, status),
    ],
    sub: [],
  });

  const target = hashedTodo("todo-1");

  it("returns undefined for an empty ledger", () => {
    expect(superseded(target, [])).toBeUndefined();
  });

  it("returns undefined when the component has no UID", () => {
    const anon: Component = { type: "VTODO", props: [prop("DTSTAMP", "20260504T120000Z")], sub: [] };
    expect(superseded(anon, [journal("j", "", "COMPLETED", "20260504T143000Z")])).toBeUndefined();
  });

  it("ignores an entry whose RELATED-TO names a different component", () => {
    expect(
      superseded(target, [journal("j", "todo-other", "COMPLETED", "20260504T143000Z")]),
    ).toBeUndefined();
  });

  it("ignores an entry that is not a VJOURNAL", () => {
    const notJournal: Component = { ...journal("j", "todo-1", "COMPLETED", "20260504T143000Z"), type: "VTODO" };
    expect(superseded(target, [notJournal])).toBeUndefined();
  });

  it("ignores an entry lacking the supersession category", () => {
    expect(
      superseded(target, [journal("j", "todo-1", "COMPLETED", "20260504T143000Z", "notes")]),
    ).toBeUndefined();
  });

  it("matches the category case-insensitively and inside a comma list", () => {
    expect(
      superseded(target, [
        journal("j", "todo-1", "COMPLETED", "20260504T143000Z", "notes,STATUS-SUPERSESSION,other"),
      ]),
    ).toBe("COMPLETED");
  });

  it("does not match a category that merely starts with the token", () => {
    expect(
      superseded(target, [
        journal("j", "todo-1", "COMPLETED", "20260504T143000Z", "status-supersession-deferred"),
      ]),
    ).toBeUndefined();
  });

  it("ignores a matching entry that carries no effective status", () => {
    const noStatus = journal("j", "todo-1", "COMPLETED", "20260504T143000Z");
    noStatus.props = noStatus.props.filter((p) => p.name !== PROP_EFFECTIVE_STATUS);
    expect(superseded(target, [noStatus])).toBeUndefined();
  });

  it("takes the latest DTSTAMP, whatever the ledger order", () => {
    const early = journal("j1", "todo-1", "IN-PROCESS", "20260504T143000Z");
    const late = journal("j2", "todo-1", "COMPLETED", "20260504T164500Z");
    expect(superseded(target, [early, late])).toBe("COMPLETED");
    expect(superseded(target, [late, early])).toBe("COMPLETED");
  });

  it("breaks a DTSTAMP tie in favour of the later ledger position", () => {
    const a = journal("j1", "todo-1", "IN-PROCESS", "20260504T143000Z");
    const b = journal("j2", "todo-1", "COMPLETED", "20260504T143000Z");
    expect(superseded(target, [a, b])).toBe("COMPLETED");
    expect(superseded(target, [b, a])).toBe("IN-PROCESS");
  });

  it("demotes an unparseable DTSTAMP to the epoch rather than raising", () => {
    const broken = journal("j1", "todo-1", "IN-PROCESS", "not-a-timestamp");
    const good = journal("j2", "todo-1", "COMPLETED", "20260504T143000Z");
    expect(superseded(target, [broken, good])).toBe("COMPLETED");
    // Still found when it is the only entry — a query, not a validator.
    expect(superseded(target, [broken])).toBe("IN-PROCESS");
  });
});
