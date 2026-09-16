// SPDX-License-Identifier: MIT

import { describe, expect, it } from "vitest";

import type { Calendar, Card, Component, RelType, VDate } from "../src/index.js";
import {
  CLASS_PUBLIC,
  COMP_TODO,
  DEFAULT_REL_TYPE,
  EVENT_TENTATIVE,
  JOURNAL_DRAFT,
  KIND_GROUP,
  REL_DEPENDS_ON,
  TODO_NEEDS_ACTION,
  TRANSP_OPAQUE,
  VALUE_DATE,
  VALUE_PARAM,
  VstarError,
  append,
  completedDate,
  dateOf,
  dtstartDate,
  filter,
  find,
  formatDate,
  isDateOnly,
  parseDate,
  parseRelType,
  propertyEqual,
  relTypeEqualFold,
  setDtstartDate,
  setDueDate,
} from "../src/index.js";

describe("wire-string enums", () => {
  it("carry the normative wire values", () => {
    expect(COMP_TODO).toBe("VTODO");
    expect(KIND_GROUP).toBe("group");
    expect(TODO_NEEDS_ACTION).toBe("NEEDS-ACTION");
    expect(EVENT_TENTATIVE).toBe("TENTATIVE");
    expect(JOURNAL_DRAFT).toBe("DRAFT");
    expect(CLASS_PUBLIC).toBe("PUBLIC");
    expect(TRANSP_OPAQUE).toBe("OPAQUE");
    expect(VALUE_PARAM).toBe("VALUE");
    expect(VALUE_DATE).toBe("DATE");
  });
});

describe("RelType is an open enum", () => {
  it("folds a registered value case-insensitively", () => {
    expect(parseRelType("depends-on")).toEqual([REL_DEPENDS_ON, true]);
  });

  it("returns PARENT with ok=true for the empty string", () => {
    expect(parseRelType("")).toEqual([DEFAULT_REL_TYPE, true]);
  });

  it("returns an unregistered value verbatim with ok=false", () => {
    expect(parseRelType("X-CUSTOM")).toEqual(["X-CUSTOM" as RelType, false]);
  });

  it("compares case-insensitively", () => {
    expect(relTypeEqualFold(REL_DEPENDS_ON, "depends-on")).toBe(true);
    expect(relTypeEqualFold(REL_DEPENDS_ON, "CHILD")).toBe(false);
  });
});

describe("VDate", () => {
  it("parses a strict YYYYMMDD value", () => {
    expect(parseDate("20260515")).toEqual({ year: 2026, month: 5, day: 15 });
  });

  it("uses a 1-based month, not a zero-based index", () => {
    expect(parseDate("20260101")?.month).toBe(1);
  });

  it("rejects everything that is not exactly eight digits", () => {
    for (const bad of ["2026-05-15", "20260515T000000Z", "202605", "", "2026051X", " 20260515"]) {
      expect(parseDate(bad), bad).toBeUndefined();
    }
  });

  it("rejects impossible calendar dates with no roll-over", () => {
    for (const bad of ["20260230", "20261301", "20260100", "20260229"]) {
      expect(parseDate(bad), bad).toBeUndefined();
    }
  });

  it("formats back to the eight-octet wire form", () => {
    expect(formatDate({ year: 2026, month: 5, day: 15 })).toBe("20260515");
    expect(formatDate({ year: 1, month: 2, day: 3 })).toBe("00010203");
  });

  it("formats out-of-range fields as the empty string", () => {
    expect(formatDate({ year: 2026, month: 13, day: 1 })).toBe("");
    expect(formatDate({ year: 10000, month: 1, day: 1 })).toBe("");
  });

  it("dateOf reads the calendar date off an instant", () => {
    const midnight = Date.UTC(2026, 4, 15, 0, 0, 0);
    expect(dateOf(midnight)).toEqual({ year: 2026, month: 5, day: 15 });
  });
});

describe("Component date accessors", () => {
  const allDay = (): Component => ({
    type: "VTODO",
    props: [
      { name: "UID", params: [], value: "t" },
      { name: "DUE", params: [{ name: "VALUE", value: "DATE" }], value: "20260515" },
      { name: "DTSTART", params: [], value: "20260515T090000Z" },
    ],
    sub: [],
  });

  it("isDateOnly branches on VALUE=DATE", () => {
    const c = allDay();
    expect(isDateOnly(c, "DUE")).toBe(true);
    expect(isDateOnly(c, "DTSTART")).toBe(false);
    expect(isDateOnly(c, "MISSING")).toBe(false);
  });

  it("refuses to promote an untagged eight-digit DATE-TIME to a VDate", () => {
    const c: Component = {
      type: "VTODO",
      props: [{ name: "DTSTART", params: [], value: "20260515" }],
      sub: [],
    };
    expect(dtstartDate(c)).toBeUndefined();
  });

  it("returns undefined for an absent property", () => {
    expect(completedDate(allDay())).toBeUndefined();
  });

  it("setters write VALUE=DATE and drop stale parameters", () => {
    const c: Component = {
      type: "VTODO",
      props: [{ name: "DTSTART", params: [{ name: "TZID", value: "X" }], value: "20260515T090000" }],
      sub: [],
    };
    setDtstartDate(c, { year: 2026, month: 6, day: 1 });
    expect(c.props[0]).toEqual({
      name: "DTSTART",
      params: [{ name: "VALUE", value: "DATE" }],
      value: "20260601",
    });
  });

  it("the zero date clears the property", () => {
    const c = allDay();
    setDueDate(c, { year: 0, month: 0, day: 0 } as VDate);
    expect(c.props.some((p) => p.name === "DUE")).toBe(false);
  });
});

describe("Calendar accessors", () => {
  const cal = (): Calendar => ({
    prodId: "-//test//EN",
    components: [
      { type: "VTODO", props: [{ name: "UID", params: [], value: "a" }], sub: [] },
      { type: "VEVENT", props: [{ name: "UID", params: [], value: "b" }], sub: [] },
    ],
  });

  it("find matches UID case-sensitively", () => {
    expect(find(cal(), "a")?.type).toBe("VTODO");
    expect(find(cal(), "A")).toBeUndefined();
  });

  it("filter selects by component type", () => {
    expect(filter(cal(), "VEVENT").length).toBe(1);
    expect(filter(cal(), "VJOURNAL")).toEqual([]);
  });

  it("append adds to the component list", () => {
    const c = cal();
    append(c, { type: "VJOURNAL", props: [], sub: [] });
    expect(c.components.length).toBe(3);
  });
});

describe("propertyEqual", () => {
  it("compares names case-insensitively and values case-sensitively", () => {
    expect(
      propertyEqual(
        { name: "summary", params: [], value: "A" },
        { name: "SUMMARY", params: [], value: "A" },
      ),
    ).toBe(true);
    expect(
      propertyEqual(
        { name: "SUMMARY", params: [], value: "a" },
        { name: "SUMMARY", params: [], value: "A" },
      ),
    ).toBe(false);
  });

  it("normalizes parameter order before comparing", () => {
    expect(
      propertyEqual(
        {
          name: "X",
          params: [
            { name: "B", value: "2" },
            { name: "A", value: "1" },
          ],
          value: "v",
        },
        {
          name: "X",
          params: [
            { name: "A", value: "1" },
            { name: "B", value: "2" },
          ],
          value: "v",
        },
      ),
    ).toBe(true);
  });

  it("does not mutate its inputs", () => {
    const a = {
      name: "X",
      params: [
        { name: "B", value: "2" },
        { name: "A", value: "1" },
      ],
      value: "v",
    };
    propertyEqual(a, a);
    expect(a.params.map((p) => p.name)).toEqual(["B", "A"]);
  });
});

describe("Card accessors", () => {
  const card = (): Card => ({
    uid: "u",
    kind: KIND_GROUP,
    props: [
      { name: "FN", params: [], value: "A" },
      { name: "MEMBER", params: [], value: "m1" },
      { name: "MEMBER", params: [], value: "m2" },
    ],
  });

  it("cardGet returns the first match, cardGetAll returns every match", async () => {
    const { cardGet, cardGetAll } = await import("../src/index.js");
    expect(cardGet(card(), "fn")?.value).toBe("A");
    expect(cardGetAll(card(), "MEMBER").length).toBe(2);
  });

  it("cardSet replaces every match with one copy, cardRemove deletes all", async () => {
    const { cardRemove, cardSet } = await import("../src/index.js");
    const c = card();
    cardSet(c, { name: "MEMBER", params: [], value: "only" });
    expect(c.props.filter((p) => p.name === "MEMBER").length).toBe(1);
    cardRemove(c, "member");
    expect(c.props.some((p) => p.name === "MEMBER")).toBe(false);
  });
});

describe("VstarError", () => {
  it("is an Error carrying a recoverable sentinel identifier", () => {
    const e = new VstarError("ErrMalformed", "bad input");
    expect(e).toBeInstanceOf(Error);
    expect(e.code).toBe("ErrMalformed");
    expect(e.name).toBe("VstarError");
    expect(e.message).toContain("bad input");
  });

  it("carries positional context when known", () => {
    const e = new VstarError("ErrMalformed", "bad", { line: 7 });
    expect(e.line).toBe(7);
  });

  it("declares all twelve sentinel identifiers", async () => {
    const { VSTAR_ERROR_CODES } = await import("../src/index.js");
    expect([...VSTAR_ERROR_CODES].sort()).toEqual(
      [
        "ErrAlreadyClosed",
        "ErrHeaderLocked",
        "ErrIterationCap",
        "ErrMalformed",
        "ErrMissingUID",
        "ErrNoAnchor",
        "ErrNoTrigger",
        "ErrTargetCorrupted",
        "ErrUnboundedExpansion",
        "ErrUnclosedBlock",
        "ErrUnsupportedRRule",
        "ErrUnsupportedVersion",
      ].sort(),
    );
  });
});
