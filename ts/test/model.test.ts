// SPDX-License-Identifier: MIT

import { describe, expect, expectTypeOf, it } from "vitest";

import type { Calendar, Card, CompType, Component, Kind, RelType, VDate } from "../src/index.js";
import {
  CLASS_PUBLIC,
  COMP_ALARM,
  COMP_CALENDAR,
  COMP_EVENT,
  COMP_FREE_BUSY,
  COMP_JOURNAL,
  COMP_TIMEZONE,
  COMP_TODO,
  DEFAULT_REL_TYPE,
  EVENT_TENTATIVE,
  JOURNAL_DRAFT,
  KIND_GROUP,
  KIND_INDIVIDUAL,
  KIND_ORG,
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
import { parse as parseCalendar } from "../src/codec/rfc5545/index.js";
import { parse as parseCards } from "../src/codec/rfc6350/index.js";
import { loadFixtures } from "./fixtures.js";

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

// CompType and Kind are open like RelType: the constants name the
// registered vocabulary, they do not bound it. The `expectTypeOf`
// assertions are checked by `tsc`, not at run time; the runtime
// assertions cover the parser end of the same contract.
describe("CompType is an open enum", () => {
  type KnownCompType =
    | "VCALENDAR"
    | "VTODO"
    | "VJOURNAL"
    | "VEVENT"
    | "VFREEBUSY"
    | "VTIMEZONE"
    | "VALARM";

  it("admits any wire token, without a cast", () => {
    const standard: CompType = "STANDARD";
    const fromWire = (name: string): CompType => name.toUpperCase();
    expect(standard).toBe("STANDARD");
    expect(fromWire("daylight")).toBe("DAYLIGHT");
    expectTypeOf<string>().toMatchTypeOf<CompType>();
  });

  it("keeps the seven registered values as distinct literals", () => {
    // A plain `string` would collapse the literals: Extract<string, K>
    // is never. The branded tail keeps them as constituents.
    expectTypeOf<Extract<CompType, KnownCompType>>().toEqualTypeOf<KnownCompType>();
    expectTypeOf(COMP_CALENDAR).toEqualTypeOf<"VCALENDAR">();
    expectTypeOf(COMP_TODO).toEqualTypeOf<"VTODO">();
    expectTypeOf(COMP_JOURNAL).toEqualTypeOf<"VJOURNAL">();
    expectTypeOf(COMP_EVENT).toEqualTypeOf<"VEVENT">();
    expectTypeOf(COMP_FREE_BUSY).toEqualTypeOf<"VFREEBUSY">();
    expectTypeOf(COMP_TIMEZONE).toEqualTypeOf<"VTIMEZONE">();
    expectTypeOf(COMP_ALARM).toEqualTypeOf<"VALARM">();
  });

  it("carries STANDARD and DAYLIGHT sub-components through the parser", () => {
    const fixture = loadFixtures("rfc5545", ".ics").find((f) => f.stem === "nested_vtimezone");
    expect(fixture).toBeDefined();
    const cal = parseCalendar(fixture!.input);
    const tz = cal.components.find((c) => c.type === COMP_TIMEZONE);
    expect(tz).toBeDefined();
    const want: CompType[] = ["STANDARD", "DAYLIGHT"];
    expect(tz!.sub.map((s) => s.type)).toEqual(want);
  });
});

describe("Kind is an open enum", () => {
  type KnownKind = "individual" | "org" | "group" | "";

  it("admits any wire token, without a cast", () => {
    const robot: Kind = "x-robot";
    expect(robot).toBe("x-robot");
    expectTypeOf<string>().toMatchTypeOf<Kind>();
  });

  it("keeps the registered values and the absent marker as distinct literals", () => {
    expectTypeOf<Extract<Kind, KnownKind>>().toEqualTypeOf<KnownKind>();
    expectTypeOf(KIND_INDIVIDUAL).toEqualTypeOf<"individual">();
    expectTypeOf(KIND_ORG).toEqualTypeOf<"org">();
    expectTypeOf(KIND_GROUP).toEqualTypeOf<"group">();
  });

  it("carries an x-name KIND through the parser", () => {
    const vcf = "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:k\r\nKIND:X-Robot\r\nEND:VCARD\r\n";
    const cards = parseCards(new TextEncoder().encode(vcf));
    const want: Kind = "x-robot";
    expect(cards[0]?.kind).toBe(want);
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
