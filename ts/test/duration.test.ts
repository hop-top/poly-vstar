// SPDX-License-Identifier: MIT

// The RFC 5545 §3.3.6 DURATION value type and the §3.8.6.3 TRIGGER
// property, gated by `spec/behavior/duration/`.

import { describe, expect, it } from "vitest";

import { parse as parseIcs } from "../src/codec/rfc5545/index.js";
import {
  RELATED_END,
  RELATED_START,
  alarmRepeatCycle,
  alarmTrigger,
  eventEnd,
  fromSigned,
  parse as parseDuration,
  parseTrigger,
  valid,
} from "../src/duration/index.js";
import { VstarError } from "../src/errors.js";
import { formatTime, parseTime } from "../src/time.js";
import type { Calendar, Component, Property } from "../src/types.js";
import { type DurationParseCase, type TriggerCase, loadBehaviorInput, loadBehaviorJson } from "./behavior.js";

const MS = 1000;

describe("duration parse — spec/behavior/duration/parse.json", () => {
  const cases = loadBehaviorJson<DurationParseCase[]>("duration", "parse.json");

  it("covers the whole table", () => {
    expect(cases.length).toBeGreaterThan(0);
  });

  for (const [i, c] of cases.entries()) {
    it(`[${i}] ${JSON.stringify(c.value)}`, () => {
      if (c.error !== undefined) {
        expect(() => parseDuration(c.value)).toThrow(VstarError);
        try {
          parseDuration(c.value);
        } catch (e) {
          expect((e as VstarError).code).toBe(c.error);
        }
        expect(valid(c.value)).toBe(false);
        return;
      }
      const d = parseDuration(c.value);
      expect(valid(c.value)).toBe(true);
      expect(d.signed() / MS).toBe(c.seconds);
      expect(d.isNegative()).toBe(c.negative);
    });
  }
});

describe("VDuration.toString", () => {
  it.each([
    "P1W",
    "P26W",
    "P7D",
    "PT1H",
    "PT15M",
    "PT30S",
    "P1DT2H30M45S",
    "PT1H30M",
    "-PT15M",
    "-P1DT2H",
    "PT0S",
    "P0D",
    "-P1D",
    "PT5M",
  ])("round-trips %s byte for byte", (s) => {
    expect((parseDuration(s)).toString()).toBe(s);
  });

  it("drops an explicit leading plus, the one intentional normalization", () => {
    expect((parseDuration("+PT15M")).toString()).toBe("PT15M");
  });

  it("renders a negative zero as positive", () => {
    expect((parseDuration("-PT0S")).toString()).toBe("PT0S");
  });
});

describe("dayForm — P0D versus PT0S", () => {
  it("records the authored day form", () => {
    expect(parseDuration("P0D").dayForm).toBe(true);
    expect(parseDuration("PT0S").dayForm).toBe(false);
  });

  it("reproduces the authored spelling", () => {
    expect((parseDuration("P0D")).toString()).toBe("P0D");
    expect((parseDuration("PT0S")).toString()).toBe("PT0S");
  });

  it("affects formatting only", () => {
    const a = parseDuration("P0D");
    const b = parseDuration("PT0S");
    expect(a.signed()).toBe(b.signed());
    expect(a.isNegative()).toBe(b.isNegative());
    const anchor = Date.UTC(2026, 5, 1, 9);
    expect(a.addTo(anchor)).toBe(b.addTo(anchor));
  });

  it("sets dayForm on a day value with a time part", () => {
    expect(parseDuration("P1DT2H").dayForm).toBe(true);
  });
});

describe("VDuration.addTo", () => {
  const anchor = Date.UTC(2026, 5, 1, 9, 0, 0);

  it("adds a time part as elapsed time", () => {
    expect(parseDuration("PT90M").addTo(anchor)).toBe(anchor + 90 * 60 * MS);
  });

  it("subtracts a negative duration", () => {
    expect(parseDuration("-PT15M").addTo(anchor)).toBe(anchor - 15 * 60 * MS);
  });

  it("advances days by calendar date", () => {
    expect(formatTime(parseDuration("P1D").addTo(anchor))).toBe("20260602T090000Z");
  });

  it("advances weeks by seven calendar days", () => {
    expect(formatTime(parseDuration("P1W").addTo(anchor))).toBe("20260608T090000Z");
  });

  it("moves both the calendar and the elapsed part backwards when negative", () => {
    expect(formatTime(parseDuration("-P1DT2H").addTo(anchor))).toBe("20260531T070000Z");
  });
});

describe("VDuration.signed and isNegative", () => {
  it("reports milliseconds with days as 24h and weeks as 7 days", () => {
    expect(parseDuration("P1W").signed()).toBe(7 * 24 * 3600 * MS);
    expect(parseDuration("P1D").signed()).toBe(24 * 3600 * MS);
  });

  it("never reports a zero-length duration as negative", () => {
    expect(parseDuration("-PT0S").isNegative()).toBe(false);
    expect(parseDuration("-P0D").isNegative()).toBe(false);
  });
});

describe("fromSigned", () => {
  it("expresses the value in hours, minutes and seconds only", () => {
    expect((fromSigned(24 * 3600 * MS)).toString()).toBe("PT24H");
    expect((fromSigned(90 * 60 * MS)).toString()).toBe("PT1H30M");
  });

  it("truncates sub-second precision", () => {
    expect((fromSigned(1500)).toString()).toBe("PT1S");
  });

  it("carries the sign", () => {
    expect((fromSigned(-900 * MS)).toString()).toBe("-PT15M");
    expect(fromSigned(-900 * MS).isNegative()).toBe(true);
  });

  it("renders zero as PT0S", () => {
    expect((fromSigned(0)).toString()).toBe("PT0S");
  });
});

describe("Related", () => {
  it("spells the RFC wire values", () => {
    expect(RELATED_START).toBe("START");
    expect(RELATED_END).toBe("END");
  });
});

/** Every VALARM in `cal`, in document order, with its parent. */
function alarms(cal: Calendar): { parent: Component; alarm: Component; uid: string }[] {
  const out: { parent: Component; alarm: Component; uid: string }[] = [];
  for (const parent of cal.components) {
    for (const alarm of parent.sub) {
      if (alarm.type !== "VALARM") continue;
      const uid = alarm.props.find((p) => p.name.toUpperCase() === "UID")?.value ?? "";
      out.push({ parent, alarm, uid });
    }
  }
  return out;
}

const TRIGGER_FIXTURES = [
  "absolute",
  "missing_anchor",
  "missing_trigger",
  "relative_end",
  "relative_start",
  "value_contradiction",
  "vtodo_due_anchor",
] as const;

describe.each(TRIGGER_FIXTURES)("trigger behavior — %s", (stem) => {
  const cal = parseIcs(loadBehaviorInput("duration", `${stem}.ics`));
  const expected = loadBehaviorJson<TriggerCase[]>("duration", `${stem}.trigger.json`);
  const found = alarms(cal);

  it("has one entry per VALARM, in document order", () => {
    expect(found.map((a) => a.uid)).toEqual(expected.map((e) => e.alarm_uid));
  });

  for (const [i, want] of expected.entries()) {
    it(`${want.alarm_uid} → ${want.fires_at ?? want.error}`, () => {
      const entry = found[i];
      expect(entry).toBeDefined();
      const { parent, alarm } = entry as { parent: Component; alarm: Component };

      if (want.error !== undefined) {
        let caught: unknown;
        try {
          alarmTrigger(alarm).resolve(parent, cal);
        } catch (e) {
          caught = e;
        }
        expect(caught).toBeInstanceOf(VstarError);
        expect((caught as VstarError).code).toBe(want.error);
        return;
      }
      const at = alarmTrigger(alarm).resolve(parent, cal);
      expect(formatTime(at)).toBe(want.fires_at);
    });
  }
});

describe("parseTrigger", () => {
  const trigger = (value: string, params: Property["params"] = []): Property => ({
    name: "TRIGGER",
    params,
    value,
  });

  it("reads VALUE=DURATION as relative", () => {
    const t = parseTrigger(trigger("-PT15M", [{ name: "VALUE", value: "DURATION" }]));
    expect(t.relative).toBe(true);
    expect((t.duration).toString()).toBe("-PT15M");
  });

  it("reads VALUE=DATE-TIME as absolute", () => {
    const t = parseTrigger(trigger("20260531T220000Z", [{ name: "VALUE", value: "DATE-TIME" }]));
    expect(t.relative).toBe(false);
    expect(formatTime(t.absolute)).toBe("20260531T220000Z");
  });

  it("infers relative from a duration-shaped value", () => {
    expect(parseTrigger(trigger("-PT30M")).relative).toBe(true);
  });

  it("infers absolute from a form #2 instant", () => {
    expect(parseTrigger(trigger("20260531T230000Z")).relative).toBe(false);
  });

  it("treats an explicit VALUE as authoritative, not a hint", () => {
    expect(() => parseTrigger(trigger("-PT15M", [{ name: "VALUE", value: "DATE-TIME" }]))).toThrow(
      VstarError,
    );
    expect(() =>
      parseTrigger(trigger("20260531T220000Z", [{ name: "VALUE", value: "DURATION" }])),
    ).toThrow(VstarError);
  });

  it("rejects an unsupported VALUE", () => {
    expect(() => parseTrigger(trigger("-PT15M", [{ name: "VALUE", value: "TEXT" }]))).toThrow(
      VstarError,
    );
  });

  it("rejects a value that is neither form", () => {
    expect(() => parseTrigger(trigger("tomorrow"))).toThrow(VstarError);
  });

  it("honours RELATED on a relative trigger, case-insensitively", () => {
    expect(parseTrigger(trigger("-PT10M", [{ name: "related", value: "end" }])).related).toBe(
      RELATED_END,
    );
    expect(parseTrigger(trigger("-PT10M", [{ name: "RELATED", value: "START" }])).related).toBe(
      RELATED_START,
    );
  });

  it("defaults RELATED to START", () => {
    expect(parseTrigger(trigger("-PT10M")).related).toBe(RELATED_START);
  });

  it("rejects RELATED on an absolute trigger", () => {
    expect(() =>
      parseTrigger(trigger("20260531T220000Z", [{ name: "RELATED", value: "END" }])),
    ).toThrow(VstarError);
  });

  it("rejects an unknown RELATED value", () => {
    expect(() => parseTrigger(trigger("-PT10M", [{ name: "RELATED", value: "MIDDLE" }]))).toThrow(
      VstarError,
    );
  });
});

describe("Trigger.toProperty", () => {
  it("omits the default RELATED=START", () => {
    const p = parseTrigger({ name: "TRIGGER", params: [], value: "-PT15M" }).toProperty();
    expect(p).toEqual({ name: "TRIGGER", params: [], value: "-PT15M" });
  });

  it("writes RELATED=END explicitly", () => {
    const p = parseTrigger({
      name: "TRIGGER",
      params: [{ name: "RELATED", value: "END" }],
      value: "-PT10M",
    }).toProperty();
    expect(p).toEqual({
      name: "TRIGGER",
      params: [{ name: "RELATED", value: "END" }],
      value: "-PT10M",
    });
  });

  it("writes an absolute trigger with VALUE=DATE-TIME", () => {
    const p = parseTrigger({
      name: "TRIGGER",
      params: [],
      value: "20260531T220000Z",
    }).toProperty();
    expect(p).toEqual({
      name: "TRIGGER",
      params: [{ name: "VALUE", value: "DATE-TIME" }],
      value: "20260531T220000Z",
    });
  });
});

describe("alarmTrigger and ErrNoTrigger", () => {
  it("throws ErrNoTrigger for a VALARM with no TRIGGER", () => {
    const alarm: Component = { type: "VALARM", props: [], sub: [] };
    try {
      alarmTrigger(alarm);
      expect.unreachable();
    } catch (e) {
      expect((e as VstarError).code).toBe("ErrNoTrigger");
    }
  });
});

describe("eventEnd", () => {
  const empty: Calendar = { prodId: "-//V*//T//EN", components: [] };
  const event = (props: [string, string][]): Component => ({
    type: "VEVENT",
    props: props.map(([name, value]) => ({ name, params: [], value })),
    sub: [],
  });

  it("prefers DTEND when both forms are present", () => {
    const c = event([
      ["DTSTART", "20260601T090000Z"],
      ["DTEND", "20260601T170000Z"],
      ["DURATION", "P1D"],
    ]);
    expect(formatTime(eventEnd(c, empty) as number)).toBe("20260601T170000Z");
  });

  it("falls back to DTSTART plus DURATION", () => {
    const c = event([
      ["DTSTART", "20260601T090000Z"],
      ["DURATION", "P1D"],
    ]);
    expect(formatTime(eventEnd(c, empty) as number)).toBe("20260602T090000Z");
  });

  it("reports nothing when neither form is available", () => {
    expect(eventEnd(event([["DTSTART", "20260601T090000Z"]]), empty)).toBeUndefined();
  });

  it("reports nothing when the DURATION form has no DTSTART", () => {
    expect(eventEnd(event([["DURATION", "P1D"]]), empty)).toBeUndefined();
  });

  it("reports nothing when the DURATION value is malformed", () => {
    const c = event([
      ["DTSTART", "20260601T090000Z"],
      ["DURATION", "P1W2D"],
    ]);
    expect(eventEnd(c, empty)).toBeUndefined();
  });
});

describe("alarmRepeatCycle", () => {
  const alarm = (props: [string, string][]): Component => ({
    type: "VALARM",
    props: props.map(([name, value]) => ({ name, params: [], value })),
    sub: [],
  });

  it("reads the DURATION/REPEAT pair", () => {
    const [d, n] = alarmRepeatCycle(
      alarm([
        ["DURATION", "PT5M"],
        ["REPEAT", "2"],
      ]),
    );
    expect((d).toString()).toBe("PT5M");
    expect(n).toBe(2);
  });

  it("reports a zero cycle when neither property is present", () => {
    const [d, n] = alarmRepeatCycle(alarm([]));
    expect((d).toString()).toBe("PT0S");
    expect(n).toBe(0);
  });

  it.each([
    [[["DURATION", "PT5M"]], "DURATION without REPEAT"],
    [[["REPEAT", "2"]], "REPEAT without DURATION"],
    [
      [
        ["DURATION", "nonsense"],
        ["REPEAT", "2"],
      ],
      "malformed DURATION",
    ],
    [
      [
        ["DURATION", "PT5M"],
        ["REPEAT", "two"],
      ],
      "non-integer REPEAT",
    ],
    [
      [
        ["DURATION", "PT5M"],
        ["REPEAT", "-1"],
      ],
      "negative REPEAT",
    ],
  ])("rejects %#: %s", (props) => {
    expect(() => alarmRepeatCycle(alarm(props as [string, string][]))).toThrow(VstarError);
  });
});

describe("resolve against a TZID-bearing anchor", () => {
  it("resolves DTSTART through the calendar's VTIMEZONE registry", () => {
    const cal = parseIcs(
      new TextEncoder().encode(
        [
          "BEGIN:VCALENDAR",
          "VERSION:2.0",
          "PRODID:-//V*//T//EN",
          "BEGIN:VTIMEZONE",
          "TZID:Fixed/Plus02",
          "BEGIN:STANDARD",
          "DTSTART:19700101T000000",
          "TZOFFSETFROM:+0200",
          "TZOFFSETTO:+0200",
          "END:STANDARD",
          "END:VTIMEZONE",
          "BEGIN:VEVENT",
          "UID:e",
          "DTSTAMP:20260504T120000Z",
          "DTSTART;TZID=Fixed/Plus02:20260601T110000",
          "BEGIN:VALARM",
          "ACTION:DISPLAY",
          "TRIGGER:-PT15M",
          "END:VALARM",
          "END:VEVENT",
          "END:VCALENDAR",
          "",
        ].join("\r\n"),
      ),
    );
    const evt = cal.components.find((c) => c.type === "VEVENT") as Component;
    const alarm = evt.sub[0] as Component;
    expect(formatTime(alarmTrigger(alarm).resolve(evt, cal))).toBe("20260601T084500Z");
  });
});

describe("parseTime is the one instant parser", () => {
  it("rejects what the trigger reader also rejects", () => {
    expect(parseTime("20260531T2200Z")).toBeUndefined();
  });
});
