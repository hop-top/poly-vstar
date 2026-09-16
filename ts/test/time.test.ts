// SPDX-License-Identifier: MIT

// `formatTime`, `parseTime` and `parseTimeWithTzid`, plus the
// `spec/behavior/time/tzid.json` gate.
//
// `parseTimeWithTzid` resolves ONLY against the VTIMEZONE components in
// the supplied calendar. No `Intl`, no IANA database: the fixtures name
// a real zone precisely because a port reaching for the system database
// passes them and fails every other zone.

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { parse as parseIcs } from "../src/codec/rfc5545/index.js";
import { formatTime, parseTime, parseTimeWithTzid } from "../src/time.js";
import type { Calendar, Component } from "../src/types.js";
import { type TzidCase, loadBehaviorJson } from "./behavior.js";
import { CONFORMANCE_DIR } from "./fixtures.js";

/** Load `spec/v0.1/conformance/time/<name>.ics` as a parsed calendar. */
function timeCalendar(name: string): Calendar {
  return parseIcs(new Uint8Array(readFileSync(join(CONFORMANCE_DIR, "time", `${name}.ics`))));
}

describe("formatTime", () => {
  it("emits RFC 5545 form #2", () => {
    expect(formatTime(Date.UTC(2026, 4, 4, 18, 30, 45))).toBe("20260504T183045Z");
  });

  it("emits the empty string for the zero instant", () => {
    expect(formatTime(0)).toBe("");
  });

  it("zero-pads every field", () => {
    expect(formatTime(Date.UTC(2026, 0, 2, 3, 4, 5))).toBe("20260102T030405Z");
  });

  it("truncates sub-second precision rather than emitting it", () => {
    expect(formatTime(Date.UTC(2026, 4, 4, 18, 30, 45) + 999)).toBe("20260504T183045Z");
  });

  it("truncates toward negative infinity for pre-epoch instants", () => {
    expect(formatTime(Date.UTC(1969, 11, 31, 23, 59, 59))).toBe("19691231T235959Z");
  });

  it("never emits the extended ISO form or expanded years", () => {
    expect(formatTime(Date.UTC(875, 0, 1))).toBe("08750101T000000Z");
    expect(formatTime(Date.UTC(2026, 4, 4))).not.toContain("-");
    expect(formatTime(Date.UTC(2026, 4, 4))).not.toContain(".");
  });
});

describe("parseTime", () => {
  it("parses RFC 5545 form #2", () => {
    expect(parseTime("20260504T183045Z")).toBe(Date.UTC(2026, 4, 4, 18, 30, 45));
  });

  it.each([
    ["20260504T183045", "form #1 (local) has no zone"],
    ["2026-05-04T18:30:45Z", "RFC 3339 / ISO 8601 extended"],
    ["20260504", "date only"],
    ["20260504T183045z", "lowercase z"],
    ["20260504T183045Z ", "trailing whitespace"],
    [" 20260504T183045Z", "leading whitespace"],
    ["", "empty"],
    ["20260504T183045ZZ", "extra octets"],
    ["20260231T000000Z", "February 31st"],
    ["20260504T250000Z", "hour 25"],
    ["20260504T186045Z", "minute 60"],
    ["20261304T183045Z", "month 13"],
    ["2026050AT183045Z", "non-digit"],
  ])("rejects %s (%s)", (input) => {
    expect(parseTime(input)).toBeUndefined();
  });

  it("round-trips through formatTime", () => {
    for (const s of ["20260504T183045Z", "19691231T235959Z", "19700101T000001Z", "20991231T235959Z"]) {
      expect(formatTime(parseTime(s) as number)).toBe(s);
    }
  });

  it("parses the epoch instant, which formatTime then spells as the sentinel", () => {
    // `0` is this port's "no instant" sentinel, inherited from the model
    // layer: `dateOf(0)` is the zero VDate and the setters read `0` as
    // "clear the property". The Unix epoch therefore cannot round-trip —
    // it parses to `0` and formats back to the empty string.
    //
    // The Go reference does not share the collision: its zero time is
    // year 1, so 1970-01-01T00:00:00Z is an ordinary instant there.
    // No corpus or behavior fixture carries the epoch, so the two agree
    // on every byte the gates compare. Narrowing the gap would mean
    // changing the model layer's sentinel, which is not this layer's to
    // change.
    expect(parseTime("19700101T000000Z")).toBe(0);
    expect(formatTime(0)).toBe("");
  });
});

describe("parseTimeWithTzid — spec/behavior/time/tzid.json", () => {
  const cases = loadBehaviorJson<TzidCase[]>("time", "tzid.json");

  it("covers the whole table", () => {
    expect(cases.length).toBeGreaterThan(0);
  });

  for (const [i, c] of cases.entries()) {
    const label = `${c.calendar} ${JSON.stringify(c.tzid)} ${JSON.stringify(c.value)}`;
    it(`[${i}] ${label} → ${c.utc ?? "not ok"}`, () => {
      const got = parseTimeWithTzid(c.value, c.tzid, timeCalendar(c.calendar));
      if (c.utc === null) {
        expect(got).toBeUndefined();
        return;
      }
      expect(got).toBeDefined();
      expect(formatTime(got as number)).toBe(c.utc);
    });
  }
});

describe("parseTimeWithTzid — the VTIMEZONE subset", () => {
  // STANDARD and DAYLIGHT are sub-component types the RFC 5545 parser
  // produces; CompType is open, so they need no cast.
  const sub = (type: string, props: [string, string][]): Component => ({
    type,
    props: props.map(([name, value]) => ({ name, params: [], value })),
    sub: [],
  });
  const std = (props: [string, string][]): Component => sub("STANDARD", props);
  const dst = (props: [string, string][]): Component => sub("DAYLIGHT", props);
  const zone = (children: Component[], tzid = "Z"): Calendar => ({
    prodId: "-//V*//T//EN",
    components: [
      { type: "VTIMEZONE", props: [{ name: "TZID", params: [], value: tzid }], sub: children },
    ],
  });

  const FIXED: [string, string][] = [
    ["DTSTART", "19700101T000000"],
    ["TZOFFSETFROM", "+0200"],
    ["TZOFFSETTO", "+0200"],
  ];

  it("accepts a STANDARD-only fixed-offset zone", () => {
    const got = parseTimeWithTzid("20260504T120000", "Z", zone([std(FIXED)]));
    expect(formatTime(got as number)).toBe("20260504T100000Z");
  });

  it("accepts an offset with seconds (±HHMMSS)", () => {
    const got = parseTimeWithTzid(
      "20260504T120000",
      "Z",
      zone([
        std([
          ["DTSTART", "19700101T000000"],
          ["TZOFFSETFROM", "-000130"],
          ["TZOFFSETTO", "-000130"],
        ]),
      ]),
    );
    expect(formatTime(got as number)).toBe("20260504T120130Z");
  });

  it("accepts a DAYLIGHT-only zone as a fixed offset", () => {
    const got = parseTimeWithTzid("20260504T120000", "Z", zone([dst(FIXED)]));
    expect(formatTime(got as number)).toBe("20260504T100000Z");
  });

  it.each([
    ["no children", []],
    ["two STANDARD children", [std(FIXED), std(FIXED)]],
    ["two DAYLIGHT children", [std(FIXED), dst(FIXED), dst(FIXED)]],
    [
      "missing TZOFFSETTO",
      [
        std([
          ["DTSTART", "19700101T000000"],
          ["TZOFFSETFROM", "+0000"],
        ]),
      ],
    ],
    [
      "missing TZOFFSETFROM",
      [
        std([
          ["DTSTART", "19700101T000000"],
          ["TZOFFSETTO", "+0000"],
        ]),
      ],
    ],
    [
      "missing DTSTART",
      [
        std([
          ["TZOFFSETFROM", "+0000"],
          ["TZOFFSETTO", "+0000"],
        ]),
      ],
    ],
    [
      "malformed TZOFFSETTO",
      [
        std([
          ["DTSTART", "19700101T000000"],
          ["TZOFFSETFROM", "+0000"],
          ["TZOFFSETTO", "0200"],
        ]),
      ],
    ],
  ])("rejects a zone with %s", (_label, children) => {
    expect(parseTimeWithTzid("20260504T120000", "Z", zone(children as Component[]))).toBeUndefined();
  });

  const withRule = (rule: string): Calendar =>
    zone([
      std([
        ["DTSTART", "19701101T020000"],
        ["TZOFFSETFROM", "-0400"],
        ["TZOFFSETTO", "-0500"],
        ["RRULE", "FREQ=YEARLY;BYMONTH=11;BYDAY=1SU"],
      ]),
      dst([
        ["DTSTART", "19700308T020000"],
        ["TZOFFSETFROM", "-0500"],
        ["TZOFFSETTO", "-0400"],
        ["RRULE", rule],
      ]),
    ]);

  it("accepts FREQ=YEARLY with BYMONTH and an ordinal BYDAY", () => {
    expect(
      parseTimeWithTzid("20260704T120000", "Z", withRule("FREQ=YEARLY;BYMONTH=3;BYDAY=2SU")),
    ).toBeDefined();
  });

  it("accepts a no-op INTERVAL=1", () => {
    expect(
      parseTimeWithTzid(
        "20260704T120000",
        "Z",
        withRule("FREQ=YEARLY;INTERVAL=1;BYMONTH=3;BYDAY=2SU"),
      ),
    ).toBeDefined();
  });

  it("accepts a negative BYDAY ordinal (last Sunday)", () => {
    // Last Sunday of March 2026 is the 29th; a 12:00 wall time on the
    // 30th is therefore inside DST (-0400 → 16:00Z).
    const got = parseTimeWithTzid(
      "20260330T120000",
      "Z",
      withRule("FREQ=YEARLY;BYMONTH=3;BYDAY=-1SU"),
    );
    expect(formatTime(got as number)).toBe("20260330T160000Z");
  });

  it.each([
    ["FREQ=MONTHLY;BYMONTH=3;BYDAY=2SU", "not FREQ=YEARLY"],
    ["FREQ=YEARLY;BYMONTH=3;BYDAY=2SU;COUNT=5", "COUNT"],
    ["FREQ=YEARLY;BYMONTH=3;BYDAY=2SU;UNTIL=20300101T000000Z", "UNTIL"],
    ["FREQ=YEARLY;BYMONTH=3;BYDAY=2SU;BYSETPOS=1", "BYSETPOS"],
    ["FREQ=YEARLY;BYMONTH=3;BYDAY=2SU;WKST=MO", "WKST"],
    ["FREQ=YEARLY;BYMONTH=3;BYDAY=SU", "BYDAY with no ordinal"],
    ["FREQ=YEARLY;BYMONTH=3;BYDAY=0SU", "BYDAY ordinal zero"],
    ["FREQ=YEARLY;BYMONTH=13;BYDAY=2SU", "BYMONTH out of range"],
    ["FREQ=YEARLY;BYMONTH=3;BYDAY=2SU;INTERVAL=2", "INTERVAL other than 1"],
    ["BYMONTH=3;BYDAY=2SU", "missing FREQ"],
    ["FREQ=YEARLY;X-WHAT=1", "unknown rule part"],
    ["FREQ=YEARLY;BROKEN", "a part with no '='"],
  ])("rejects RRULE %s (%s)", (rule) => {
    expect(parseTimeWithTzid("20260704T120000", "Z", withRule(rule))).toBeUndefined();
  });

  it("rejects a mismatched TZID rather than falling back to any zone", () => {
    expect(parseTimeWithTzid("20260504T120000", "Other", zone([std(FIXED)]))).toBeUndefined();
  });

  it("matches TZID case-sensitively", () => {
    expect(parseTimeWithTzid("20260504T120000", "z", zone([std(FIXED)]))).toBeUndefined();
  });
});
