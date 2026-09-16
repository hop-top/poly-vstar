// SPDX-License-Identifier: MIT

// The `diff` surface — semantic equality and structural diff — gated by
// `spec/behavior/diff/`.
//
// The parity contract deliberately does NOT sort diff output as a
// whole: components come back in pairing order and each component's
// ops are sorted by property name case-insensitively. Both halves of
// that are asserted here against the fixtures, so a port that
// "normalizes" the component list fails rather than passing quietly.

import { describe, expect, it } from "vitest";

import { parse as parseIcs } from "../src/codec/rfc5545/index.js";
import {
  OP_ADDED,
  OP_CHANGED,
  OP_REMOVED,
  calendarEqual,
  cardEqual,
  componentDiffToString,
  componentEqual,
  diffOpToString,
  isEmpty,
  ofCalendar,
  ofCard,
  ofComponent,
  propertyEqual,
  type ComponentDiff,
  type PropertyDiff,
} from "../src/diff/index.js";
import type { Calendar, Card, Component, Property } from "../src/types.js";
import { type DiffCase, type DiffOpCase, behaviorStems, loadBehaviorInput } from "./behavior.js";
import { loadBehaviorJson } from "./behavior.js";

function prop(name: string, value: string, params: Property["params"] = []): Property {
  return { name, params, value };
}

/** Project a {@link ComponentDiff} tree onto the fixture's shape. */
function project(ds: readonly ComponentDiff[]): DiffCase[] {
  return ds.map((d) => {
    const subs = d.subDiffs.filter((s) => !isEmpty(s));
    const out: DiffCase = {
      uid: uidFromPath(d.path),
      path: d.path,
      ops: d.properties.map(projectOp),
    };
    return subs.length === 0 ? out : { ...out, subs: project(subs) };
  });
}

function projectOp(pd: PropertyDiff): DiffOpCase {
  const base = { op: pd.op, property: pd.property.name };
  switch (pd.op) {
    case OP_ADDED:
      return withParams({ ...base, before: null, after: pd.property.value }, undefined, pd.property);
    case OP_REMOVED:
      return withParams({ ...base, before: pd.property.value, after: null }, pd.property, undefined);
    case OP_CHANGED:
      return withParams(
        { ...base, before: pd.old?.value ?? null, after: pd.property.value },
        pd.old,
        pd.property,
      );
  }
}

/**
 * Attach the parameter lists the fixture carries. The fixture omits an
 * empty list entirely (`omitempty` on the Go side), so an absent key
 * and an empty list are the same statement and neither is emitted.
 */
function withParams(
  base: Omit<DiffOpCase, "before_params" | "after_params">,
  before: Property | undefined,
  after: Property | undefined,
): DiffOpCase {
  const out: DiffOpCase = { ...base };
  const b = before?.params ?? [];
  const a = after?.params ?? [];
  return {
    ...out,
    ...(b.length > 0 ? { before_params: b.map((p) => ({ name: p.name, value: p.value })) } : {}),
    ...(a.length > 0 ? { after_params: a.map((p) => ({ name: p.name, value: p.value })) } : {}),
  };
}

/** Lift the uid out of `VCALENDAR.VTODO[uid=todo-1]`; `""` for a positional path. */
function uidFromPath(path: string): string {
  const marker = "[uid=";
  const i = path.lastIndexOf(marker);
  if (i < 0 || !path.endsWith("]")) return "";
  return path.slice(i + marker.length, -1);
}

describe("diff — spec/behavior/diff/", () => {
  const stems = behaviorStems("diff", ".diff.json");

  it("walks the whole family", () => {
    expect(stems.length).toBe(7);
    expect(stems).toEqual([
      "component_added",
      "component_removed",
      "identical",
      "param_changed",
      "property_added",
      "property_removed",
      "value_changed",
    ]);
  });

  for (const stem of stems) {
    it(`${stem} reproduces the fixture exactly`, () => {
      const a = parseIcs(loadBehaviorInput("diff", `${stem}.a.ics`));
      const b = parseIcs(loadBehaviorInput("diff", `${stem}.b.ics`));
      const want = loadBehaviorJson<DiffCase[]>("diff", `${stem}.diff.json`);
      expect(project(ofCalendar(a, b))).toEqual(want);
    });
  }

  it("reports two equal documents as no entry at all", () => {
    const a = parseIcs(loadBehaviorInput("diff", "identical.a.ics"));
    const b = parseIcs(loadBehaviorInput("diff", "identical.b.ics"));
    // Not "an entry with no ops" — the empty list is the whole answer.
    expect(ofCalendar(a, b)).toEqual([]);
    expect(loadBehaviorJson<DiffCase[]>("diff", "identical.diff.json")).toEqual([]);
  });

  it("keeps ops in case-insensitive property order, which is the contract", () => {
    for (const stem of stems) {
      const want = loadBehaviorJson<DiffCase[]>("diff", `${stem}.diff.json`);
      for (const entry of want) {
        const names = entry.ops.map((o) => o.property.toUpperCase());
        expect(names).toEqual([...names].sort());
      }
    }
  });

  it("keeps components in pairing order, which is also the contract", () => {
    // component_added pairs todo-keep before todo-new because the UID
    // sort runs inside a type bucket. A port sorting the output by,
    // say, op kind or path length would still produce these two
    // entries and would still be wrong.
    const a = parseIcs(loadBehaviorInput("diff", "component_added.a.ics"));
    const b = parseIcs(loadBehaviorInput("diff", "component_added.b.ics"));
    const want = loadBehaviorJson<DiffCase[]>("diff", "component_added.diff.json");
    expect(ofCalendar(a, b).map((d) => d.path)).toEqual(want.map((d) => d.path));
  });
});

describe("param-only changes carry their parameters apart from the value", () => {
  it("reports equal values and differing params as a change", () => {
    const a = parseIcs(loadBehaviorInput("diff", "param_changed.a.ics"));
    const b = parseIcs(loadBehaviorInput("diff", "param_changed.b.ics"));
    const got = ofCalendar(a, b);
    const due = got[0]?.properties.find((p) => p.property.name.toUpperCase() === "DUE");
    expect(due?.op).toBe(OP_CHANGED);
    expect(due?.old?.value).toBe(due?.property.value);
    expect(due?.old?.params).not.toEqual(due?.property.params);
  });
});

describe("X-VSTAR-HASH is excluded from both sides", () => {
  const withHash = (hash: string): Component => ({
    type: "VTODO",
    props: [
      prop("UID", "u"),
      prop("DTSTAMP", "20260504T120000Z"),
      prop("X-VSTAR-HASH", hash),
    ],
    sub: [],
  });

  it("does not report a restamped hash as a content change", () => {
    const d = ofComponent(withHash("sha256:aaa"), withHash("sha256:bbb"));
    expect(isEmpty(d)).toBe(true);
  });

  it("excludes it case-insensitively", () => {
    const lower: Component = {
      type: "VTODO",
      props: [prop("UID", "u"), prop("DTSTAMP", "20260504T120000Z"), prop("x-vstar-hash", "sha256:zzz")],
      sub: [],
    };
    expect(isEmpty(ofComponent(withHash("sha256:aaa"), lower))).toBe(true);
  });
});

describe("the equal family", () => {
  const todo = (): Component => ({
    type: "VTODO",
    props: [prop("UID", "u"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "s")],
    sub: [],
  });

  it("ignores property order — equality is via canonical bytes", () => {
    const a = todo();
    const b: Component = { ...todo(), props: [...todo().props].reverse() };
    expect(componentEqual(a, b)).toBe(true);
  });

  it("ignores parameter order", () => {
    const a: Component = {
      type: "VTODO",
      props: [
        prop("UID", "u"),
        prop("DTSTAMP", "20260504T120000Z"),
        prop("ATTENDEE", "mailto:a@example", [
          { name: "CN", value: "A" },
          { name: "ROLE", value: "CHAIR" },
        ]),
      ],
      sub: [],
    };
    const b: Component = {
      ...a,
      props: [
        prop("UID", "u"),
        prop("DTSTAMP", "20260504T120000Z"),
        prop("ATTENDEE", "mailto:a@example", [
          { name: "ROLE", value: "CHAIR" },
          { name: "CN", value: "A" },
        ]),
      ],
    };
    expect(componentEqual(a, b)).toBe(true);
  });

  it("reports a real value change as unequal", () => {
    const b = todo();
    (b.props[2] as Property).value = "t";
    expect(componentEqual(todo(), b)).toBe(false);
  });

  it("compares cards", () => {
    const card = (fn: string): Card => ({ uid: "u", kind: "individual", props: [prop("FN", fn)] });
    expect(cardEqual(card("Jad"), card("Jad"))).toBe(true);
    expect(cardEqual(card("Jad"), card("Other"))).toBe(false);
  });

  it("compares calendars, absorbing top-level component reordering", () => {
    const one = todo();
    const two: Component = { ...todo(), props: [prop("UID", "v"), prop("DTSTAMP", "20260504T120000Z")] };
    const a: Calendar = { prodId: "-//V*//T//EN", components: [one, two] };
    const b: Calendar = { prodId: "-//V*//T//EN", components: [two, one] };
    expect(calendarEqual(a, b)).toBe(true);
  });

  it("compares properties case-insensitively by name, case-sensitively by value", () => {
    expect(propertyEqual(prop("summary", "s"), prop("SUMMARY", "s"))).toBe(true);
    expect(propertyEqual(prop("SUMMARY", "s"), prop("SUMMARY", "S"))).toBe(false);
  });
});

describe("ofCard", () => {
  const card = (props: Property[]): Card => ({ uid: "u", kind: "individual", props });

  it("diffs properties and always reports no sub-diffs", () => {
    const d = ofCard(card([prop("FN", "Jad")]), card([prop("FN", "Other")]));
    expect(d.path).toBe("");
    expect(d.subDiffs).toEqual([]);
    expect(d.properties.map((p) => p.op)).toEqual([OP_CHANGED]);
  });
});

describe("sub-component pairing", () => {
  const alarm = (uid: string | undefined, action: string): Component => ({
    type: "VALARM",
    props: uid === undefined ? [prop("ACTION", action)] : [prop("UID", uid), prop("ACTION", action)],
    sub: [],
  });
  const event = (subs: Component[]): Component => ({
    type: "VEVENT",
    props: [prop("UID", "e"), prop("DTSTAMP", "20260504T120000Z")],
    sub: subs,
  });

  it("pairs UID-bearing sub-components by (type, uid)", () => {
    const d = ofComponent(event([alarm("a1", "DISPLAY")]), event([alarm("a1", "AUDIO")]));
    expect(d.subDiffs.map((s) => s.path)).toEqual(["VALARM[uid=a1]"]);
    expect(d.subDiffs[0]?.properties.map((p) => p.op)).toEqual([OP_CHANGED]);
  });

  it("pairs UID-less sub-components positionally", () => {
    const d = ofComponent(event([alarm(undefined, "DISPLAY")]), event([alarm(undefined, "AUDIO")]));
    expect(d.subDiffs.map((s) => s.path)).toEqual(["VALARM[#0]"]);
  });

  it("surfaces a reordered UID-less pair as add plus remove, the documented v0.1 limit", () => {
    const a = event([alarm(undefined, "DISPLAY"), alarm(undefined, "AUDIO")]);
    const b = event([alarm(undefined, "AUDIO"), alarm(undefined, "DISPLAY")]);
    const d = ofComponent(a, b);
    // Both positions changed, rather than one move being recognized.
    expect(d.subDiffs.map((s) => s.path)).toEqual(["VALARM[#0]", "VALARM[#1]"]);
  });
});

describe("multi-valued properties pair in order", () => {
  const withAttendees = (...values: string[]): Component => ({
    type: "VEVENT",
    props: [prop("UID", "e"), prop("DTSTAMP", "20260504T120000Z"), ...values.map((v) => prop("ATTENDEE", v))],
    sub: [],
  });

  it("reports the surplus on the b side as adds", () => {
    const d = ofComponent(withAttendees("mailto:a@x"), withAttendees("mailto:a@x", "mailto:b@x"));
    expect(d.properties.map((p) => p.op)).toEqual([OP_ADDED]);
    expect(d.properties[0]?.property.value).toBe("mailto:b@x");
  });

  it("reports the surplus on the a side as removes", () => {
    const d = ofComponent(withAttendees("mailto:a@x", "mailto:b@x"), withAttendees("mailto:a@x"));
    expect(d.properties.map((p) => p.op)).toEqual([OP_REMOVED]);
    expect(d.properties[0]?.property.value).toBe("mailto:b@x");
  });
});

describe("isEmpty", () => {
  it("is false when a nested sub-diff records a change and this level does not", () => {
    const d: ComponentDiff = {
      path: "",
      properties: [],
      subDiffs: [
        {
          path: "VALARM[#0]",
          properties: [{ op: OP_ADDED, property: prop("ACTION", "DISPLAY"), old: undefined }],
          subDiffs: [],
        },
      ],
    };
    expect(isEmpty(d)).toBe(false);
  });

  it("is true for a tree of empty sub-diffs", () => {
    const d: ComponentDiff = {
      path: "",
      properties: [],
      subDiffs: [{ path: "VALARM[#0]", properties: [], subDiffs: [] }],
    };
    expect(isEmpty(d)).toBe(true);
  });
});

describe("componentDiffToString", () => {
  it("renders an empty diff as the empty string", () => {
    expect(componentDiffToString({ path: "x", properties: [], subDiffs: [] })).toBe("");
  });

  it("renders the three op markers and the path header", () => {
    const d: ComponentDiff = {
      path: "VCALENDAR.VTODO[uid=t]",
      properties: [
        { op: OP_ADDED, property: prop("DUE", "20260101T000000Z"), old: undefined },
        { op: OP_CHANGED, property: prop("SUMMARY", "new"), old: prop("SUMMARY", "old") },
        { op: OP_REMOVED, property: prop("PRIORITY", "3"), old: undefined },
      ],
      subDiffs: [],
    };
    expect(componentDiffToString(d)).toBe(
      "--- VCALENDAR.VTODO[uid=t]\n" +
        "+ DUE:20260101T000000Z\n" +
        "~ SUMMARY: old -> new\n" +
        "- PRIORITY:3\n",
    );
  });

  it("indents a sub-diff two spaces and gives it its own header", () => {
    const d: ComponentDiff = {
      path: "VCALENDAR.VEVENT[uid=e]",
      properties: [],
      subDiffs: [
        {
          path: "VCALENDAR.VEVENT[uid=e].VALARM[#0]",
          properties: [{ op: OP_ADDED, property: prop("ACTION", "DISPLAY"), old: undefined }],
          subDiffs: [],
        },
      ],
    };
    expect(componentDiffToString(d)).toBe(
      "--- VCALENDAR.VEVENT[uid=e]\n" +
        "  --- VCALENDAR.VEVENT[uid=e].VALARM[#0]\n" +
        "  + ACTION:DISPLAY\n",
    );
  });

  it("sorts a rendered property's parameters so output is deterministic", () => {
    const d: ComponentDiff = {
      path: "p",
      properties: [
        {
          op: OP_ADDED,
          property: prop("DUE", "20260101T000000", [
            { name: "VALUE", value: "DATE-TIME" },
            { name: "TZID", value: "America/Montreal" },
          ]),
          old: undefined,
        },
      ],
      subDiffs: [],
    };
    expect(componentDiffToString(d)).toContain("+ DUE;TZID=America/Montreal;VALUE=DATE-TIME:");
  });
});

describe("diffOpToString", () => {
  it.each([
    [OP_ADDED, "Added"],
    [OP_REMOVED, "Removed"],
    [OP_CHANGED, "Changed"],
  ])("renders %s as the reference's display spelling %s", (op, want) => {
    expect(diffOpToString(op)).toBe(want);
  });
});
