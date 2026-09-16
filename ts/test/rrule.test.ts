// SPDX-License-Identifier: MIT

// The layer-(c) gate: every sidecar under
// `spec/v1.0/conformance/rrule/` decides one call, and this file makes
// each of them a test.
//
// The tree is walked rather than enumerated by name, so a fixture added
// to the corpus becomes a test here without an edit. Each `.rrule`
// input is classified by which siblings it carries:
//
// - `.expect.json`      → `validateRRule` must throw the named sentinel.
// - `.formatted`        → `Rule#toString()` must equal the file.
// - `.next.json`        → `nextOccurrence` stepped `expected.length`
//                         times, then once more past the end.
// - `.expand.json`      → `occurrences(rule, dtstart, limit)`.
// - `.between.json`     → `between(rule, dtstart, start, end)`.
// - no sibling at all   → the fixture pins only "this parses".
//
// `.ics` inputs go through `ruleSetFromComponent` and carry either
// `.expect.json` or `.times.json`.

import { describe, expect, it } from "vitest";

import { VstarError } from "../src/index.js";
import { parse as parseIcs } from "../src/codec/rfc5545/index.js";
import { formatTime, parseTime } from "../src/time.js";
import type { Instant } from "../src/date.js";
import {
  MAX_ITERATIONS,
  all,
  between,
  formatDateTimeList,
  nextOccurrence,
  occurrences,
  parseDateTimeList,
  parseRecurrenceId,
  parseRRule,
  ruleSetFromComponent,
  validateRRule,
} from "../src/rrule/index.js";
import { loadRruleFixtures, loadSetFixtures, type OutcomeSpec, type SetFixture } from "./fixtures.js";

const rruleFixtures = loadRruleFixtures();
const setFixtures = loadSetFixtures();

/** The failure class a thrown {@link VstarError} names, or `undefined`. */
function capture(fn: () => unknown): unknown {
  try {
    fn();
  } catch (e) {
    return e;
  }
  return undefined;
}

/** Assert `fn` throws a {@link VstarError} whose `code` is `want`. */
function expectSentinel(want: string, fn: () => unknown, label: string): void {
  const err = capture(fn);
  expect(err, `${label}: call succeeded, want ${want}`).toBeInstanceOf(VstarError);
  expect((err as VstarError).code, label).toBe(want);
}

/** A sidecar timestamp as an {@link Instant}, failing loudly on a bad field. */
function instant(raw: string, label: string): Instant {
  const t = parseTime(raw);
  expect(t, `${label}: ${raw} is not RFC 5545 form #2`).toBeDefined();
  return t as Instant;
}

/** Render instants back to form #2 so failures read as timestamps. */
function shown(times: readonly Instant[]): string[] {
  return times.map(formatTime);
}

// ── Corpus coverage ───────────────────────────────────────────────

describe("rrule corpus", () => {
  it("finds the fixture tree", () => {
    expect(rruleFixtures.length).toBeGreaterThan(0);
    expect(setFixtures.length).toBeGreaterThan(0);
  });

  it("covers every subdirectory the conformance README names", () => {
    const dirs = new Set(rruleFixtures.map((f) => f.dir));
    for (const want of ["happy", "bounds", "by-clauses", "rejected", "format", "evaluator", "expansion"]) {
      expect(dirs, `no fixtures under rrule/${want}/`).toContain(want);
    }
  });
});

// ── rejected/ — the named sentinel ────────────────────────────────

describe("rrule rejected", () => {
  for (const f of rruleFixtures.filter((x) => x.sentinel !== undefined)) {
    it(`${f.id} is rejected with ${f.sentinel}`, () => {
      expectSentinel(f.sentinel as string, () => validateRRule(f.value), f.id);
      // parseRRule and validateRRule are the same answer by contract:
      // validateRRule parses and discards.
      expectSentinel(f.sentinel as string, () => parseRRule(f.value), f.id);
    });
  }
});

// ── happy/, bounds/, by-clauses/ — "this parses", and nothing more ──

describe("rrule accepted", () => {
  const bare = rruleFixtures.filter((f) => f.sentinel === undefined && !f.hasAnySidecar);

  it("finds the sidecar-less fixtures", () => {
    expect(bare.length).toBeGreaterThan(0);
  });

  for (const f of bare) {
    it(`${f.id} parses`, () => {
      const rule = parseRRule(f.value);
      // The fixture pins exactly this much: the value is in scope.
      // FREQ is the one field a successful parse always sets.
      expect(rule.freq, f.id).not.toBe("INVALID");
      expect(() => {
        validateRRule(f.value);
      }, f.id).not.toThrow();
    });
  }
});

// ── format/ — the fixed wire form ─────────────────────────────────

describe("rrule format", () => {
  const formatted = rruleFixtures.filter((f) => f.formatted !== undefined);

  it("finds the .formatted fixtures", () => {
    expect(formatted.length).toBeGreaterThan(0);
  });

  for (const f of formatted) {
    it(`${f.id} re-emits as its .formatted sibling`, () => {
      const rule = parseRRule(f.value);
      expect(rule.toString(), f.id).toBe(f.formatted);
    });

    it(`${f.id} is idempotent under parse then format`, () => {
      const once = parseRRule(f.value).toString();
      const twice = parseRRule(once).toString();
      expect(twice, `${f.id}: re-parsing the wire form changed it`).toBe(once);
    });
  }

  // Every parseable fixture in the corpus, not only those with a
  // `.formatted` sibling, must survive the round trip — the spec makes
  // idempotence a property of the wire form, not of two fixtures.
  for (const f of rruleFixtures.filter((x) => x.sentinel === undefined)) {
    it(`${f.id} round-trips through the wire form`, () => {
      const once = parseRRule(f.value).toString();
      expect(parseRRule(once).toString(), f.id).toBe(once);
    });
  }

  it("keeps BYDAY list values in authored order", () => {
    // Authored descending; RFC 5545 gives BY-* lists no ordering
    // semantics, so a sorting emitter would silently rewrite content.
    expect(parseRRule("FREQ=WEEKLY;BYDAY=WE,MO").toString()).toBe("FREQ=WEEKLY;BYDAY=WE,MO");
    expect(parseRRule("FREQ=MONTHLY;BYMONTHDAY=-1,15").toString()).toBe("FREQ=MONTHLY;BYMONTHDAY=-1,15");
  });

  it("elides INTERVAL=1 and WKST=MO", () => {
    expect(parseRRule("FREQ=DAILY;INTERVAL=1;WKST=MO").toString()).toBe("FREQ=DAILY");
    expect(parseRRule("FREQ=DAILY;INTERVAL=2").toString()).toBe("FREQ=DAILY;INTERVAL=2");
    expect(parseRRule("FREQ=WEEKLY;WKST=SU").toString()).toBe("FREQ=WEEKLY;WKST=SU");
  });

  it("emits rule-parts in the spec's fixed order", () => {
    const scrambled =
      "WKST=SU;BYSETPOS=1;BYSECOND=0;BYMINUTE=0;BYHOUR=9;BYDAY=MO;" +
      "BYMONTHDAY=1;BYYEARDAY=5;BYWEEKNO=2;BYMONTH=3;COUNT=4;INTERVAL=2;FREQ=YEARLY";
    expect(parseRRule(scrambled).toString()).toBe(
      "FREQ=YEARLY;INTERVAL=2;COUNT=4;BYMONTH=3;BYWEEKNO=2;BYYEARDAY=5;" +
        "BYMONTHDAY=1;BYDAY=MO;BYHOUR=9;BYMINUTE=0;BYSECOND=0;BYSETPOS=1;WKST=SU",
    );
  });

  it("renders a Property carrying the wire form", () => {
    const p = parseRRule("FREQ=DAILY;INTERVAL=1").toProperty();
    expect(p.name).toBe("RRULE");
    expect(p.value).toBe("FREQ=DAILY");
    expect(p.params).toEqual([]);
  });
});

// ── evaluator/ — .next.json ───────────────────────────────────────

describe("rrule evaluator", () => {
  const stepped = rruleFixtures.filter((f) => f.next !== undefined);

  it("finds the .next.json fixtures", () => {
    expect(stepped.length).toBeGreaterThan(0);
  });

  for (const f of stepped) {
    const spec = f.next as OutcomeSpec;
    it(`${f.id} steps through its expected occurrences`, () => {
      const rule = parseRRule(f.value);
      const dtstart = instant(spec.dtstart as string, `${f.id}.dtstart`);
      let after = instant(spec.after as string, `${f.id}.after`);

      const expected = spec.expected ?? [];
      for (const [i, want] of expected.entries()) {
        const got = nextOccurrence(rule, dtstart, after);
        expect(got, `${f.id}: step ${i} returned undefined, want ${want}`).toBeDefined();
        expect(formatTime(got as Instant), `${f.id}: step ${i}`).toBe(want);
        after = got as Instant;
      }

      // The step past the end is what tells termination apart from the
      // iteration cap, and the sidecar says which answer is right.
      //
      // When it names an `error`, that step MUST fail with it —
      // neither yield another occurrence nor report ordinary
      // termination. When it does not, the step is only decidable for
      // a rule that bounds itself: an unbounded `FREQ=DAILY` genuinely
      // has a next occurrence forever, so asserting `undefined` there
      // would assert a falsehood rather than catch a port that stops
      // early.
      if (spec.error !== undefined) {
        expectSentinel(
          spec.error,
          () => nextOccurrence(rule, dtstart, after),
          `${f.id}: step past the end`,
        );
      } else if (rule.count > 0 || rule.until !== undefined) {
        expect(nextOccurrence(rule, dtstart, after), `${f.id}: step past the end`).toBeUndefined();
      } else {
        // Unbounded: the series must continue rather than quietly stop,
        // which is the other way a port can be wrong here.
        expect(nextOccurrence(rule, dtstart, after), `${f.id}: step past the end`).toBeDefined();
      }
    });
  }

  it("distinguishes the iteration cap from termination", () => {
    // Unsatisfiable by construction — February never has a 30th — so
    // the evaluator must give up loudly rather than report completion.
    const rule = parseRRule("FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30");
    const dtstart = instant("20260101T090000Z", "cap.dtstart");
    expectSentinel("ErrIterationCap", () => nextOccurrence(rule, dtstart, dtstart), "cap");
    expectSentinel("ErrIterationCap", () => occurrences(rule, dtstart, 5), "cap occurrences");

    // A terminating rule is the contrasting case: undefined, no throw.
    const bounded = parseRRule("FREQ=DAILY;COUNT=1");
    expect(nextOccurrence(bounded, dtstart, dtstart)).toBeUndefined();
  });

  it("publishes a finite iteration bound", () => {
    expect(MAX_ITERATIONS).toBe(100000);
  });

  it("skips BYMONTHDAY=29 in a non-leap February and honours -1", () => {
    const skip = parseRRule("FREQ=MONTHLY;BYMONTHDAY=29");
    const dtstart = instant("20260129T120000Z", "skip.dtstart");
    // 2026 is not a leap year: February is skipped entirely.
    expect(shown(occurrences(skip, dtstart, 3).times)).toEqual([
      "20260129T120000Z",
      "20260329T120000Z",
      "20260429T120000Z",
    ]);

    const last = parseRRule("FREQ=MONTHLY;BYMONTHDAY=-1");
    const jan = instant("20260131T120000Z", "last.dtstart");
    expect(shown(occurrences(last, jan, 3).times)).toEqual([
      "20260131T120000Z",
      "20260228T120000Z",
      "20260331T120000Z",
    ]);
  });

  it("anchors week boundaries on WKST", () => {
    // The same BYDAY set under two week starts: with WKST=SU the
    // Sunday belongs to the week that opens on it, with WKST=MO it
    // closes the previous one, so INTERVAL=2 selects different days.
    const dtstart = instant("20260105T090000Z", "wkst.dtstart"); // a Monday
    const su = parseRRule("FREQ=WEEKLY;INTERVAL=2;BYDAY=SU,MO;WKST=SU");
    const mo = parseRRule("FREQ=WEEKLY;INTERVAL=2;BYDAY=SU,MO;WKST=MO");
    expect(shown(occurrences(su, dtstart, 4).times)).not.toEqual(
      shown(occurrences(mo, dtstart, 4).times),
    );
  });

  it("rejects a rule the evaluator cannot walk", () => {
    const dtstart = instant("20260101T000000Z", "unsupported.dtstart");
    const broken = { ...parseRRule("FREQ=DAILY"), interval: 0 };
    expectSentinel("ErrUnsupportedRRule", () => nextOccurrence(broken, dtstart, dtstart), "interval 0");
    expectSentinel("ErrUnsupportedRRule", () => occurrences(broken, dtstart, 5), "interval 0 occurrences");
  });
});

// ── expansion/ — .expand.json and .between.json ───────────────────

describe("rrule expansion", () => {
  const expanded = rruleFixtures.filter((f) => f.expand !== undefined);
  const windowed = rruleFixtures.filter((f) => f.between !== undefined);

  it("finds the expansion fixtures", () => {
    expect(expanded.length).toBeGreaterThan(0);
    expect(windowed.length).toBeGreaterThan(0);
  });

  for (const f of expanded) {
    const spec = f.expand as OutcomeSpec;
    it(`${f.id} expands to its .expand.json sidecar`, () => {
      const rule = parseRRule(f.value);
      const dtstart = instant(spec.dtstart as string, `${f.id}.dtstart`);
      if (spec.error !== undefined) {
        expectSentinel(spec.error, () => occurrences(rule, dtstart, spec.limit ?? 0), f.id);
        return;
      }
      const got = occurrences(rule, dtstart, spec.limit ?? 0);
      expect(shown(got.times), f.id).toEqual(spec.expected ?? []);
      expect(got.complete, `${f.id}: complete flag`).toBe(spec.complete ?? false);
    });
  }

  for (const f of windowed) {
    const spec = f.between as OutcomeSpec;
    it(`${f.id} windows to its .between.json sidecar`, () => {
      const rule = parseRRule(f.value);
      const dtstart = instant(spec.dtstart as string, `${f.id}.dtstart`);
      const start = instant(spec.start as string, `${f.id}.start`);
      const end = instant(spec.end as string, `${f.id}.end`);
      if (spec.error !== undefined) {
        expectSentinel(spec.error, () => between(rule, dtstart, start, end), f.id);
        return;
      }
      expect(shown(between(rule, dtstart, start, end)), f.id).toEqual(spec.expected ?? []);
    });
  }

  it("treats the window as half-open", () => {
    const rule = parseRRule("FREQ=DAILY");
    const dtstart = instant("20260401T120000Z", "half-open.dtstart");
    const got = between(rule, dtstart, dtstart, instant("20260403T120000Z", "half-open.end"));
    // start is included, end is not.
    expect(shown(got)).toEqual(["20260401T120000Z", "20260402T120000Z"]);
  });

  it("refuses an unbounded window", () => {
    const rule = parseRRule("FREQ=DAILY");
    const dtstart = instant("20260401T120000Z", "unbounded.dtstart");
    expectSentinel(
      "ErrUnboundedExpansion",
      () => between(rule, dtstart, dtstart, undefined as unknown as Instant),
      "missing end",
    );
    expectSentinel("ErrUnboundedExpansion", () => between(rule, dtstart, dtstart, dtstart), "zero window");
    expectSentinel(
      "ErrUnboundedExpansion",
      () => between(rule, dtstart, dtstart, instant("20260331T120000Z", "inverted.end")),
      "inverted window",
    );
  });

  it("refuses a negative limit", () => {
    const rule = parseRRule("FREQ=DAILY");
    const dtstart = instant("20260401T120000Z", "negative.dtstart");
    expectSentinel("ErrUnboundedExpansion", () => occurrences(rule, dtstart, -1), "negative limit");
  });

  it("reports a zero limit as neither complete nor truncated", () => {
    const rule = parseRRule("FREQ=DAILY;COUNT=2");
    const dtstart = instant("20260401T120000Z", "zero.dtstart");
    const got = occurrences(rule, dtstart, 0);
    expect(got.times).toEqual([]);
    expect(got.complete).toBe(false);
  });

  it("yields lazily from all()", () => {
    // An unbounded rule: a materializing implementation never returns.
    const rule = parseRRule("FREQ=DAILY");
    const dtstart = instant("20260401T120000Z", "all.dtstart");
    const seen: Instant[] = [];
    for (const t of all(rule, dtstart)) {
      seen.push(t);
      if (seen.length === 3) break;
    }
    expect(shown(seen)).toEqual(["20260401T120000Z", "20260402T120000Z", "20260403T120000Z"]);
  });

  it("ends all() when the rule terminates", () => {
    const rule = parseRRule("FREQ=DAILY;COUNT=2");
    const dtstart = instant("20260401T120000Z", "all-count.dtstart");
    expect(shown([...all(rule, dtstart)])).toEqual(["20260401T120000Z", "20260402T120000Z"]);
  });
});

// ── set/ — .ics recurrence sets ───────────────────────────────────

describe("rrule sets", () => {
  for (const f of setFixtures.filter((x) => x.sentinel !== undefined)) {
    it(`${f.id} is rejected with ${f.sentinel}`, () => {
      expectSentinel(f.sentinel as string, () => ruleSetFromComponent(firstComponent(f)), f.id);
    });
  }

  for (const f of setFixtures.filter((x) => x.occurrences !== undefined)) {
    const spec = f.occurrences as OutcomeSpec;
    it(`${f.id} expands to its .occurrences.json sidecar`, () => {
      const set = ruleSetFromComponent(firstComponent(f));
      if (spec.error !== undefined) {
        expectSentinel(spec.error, () => set.occurrences(spec.limit ?? 0), f.id);
        return;
      }
      const got = set.occurrences(spec.limit ?? 0);
      expect(shown(got.times), f.id).toEqual(spec.expected ?? []);
      expect(got.complete, `${f.id}: complete flag`).toBe(spec.complete ?? false);
    });
  }

  it("removes EXDATE after merging RDATE", () => {
    // The ordering is load-bearing: an instant named by BOTH an RDATE
    // and an EXDATE stays excluded, because EXDATE is applied last.
    const ics = [
      "BEGIN:VCALENDAR",
      "VERSION:2.0",
      "PRODID:-//V*//test//EN",
      "BEGIN:VEVENT",
      "UID:order",
      "DTSTAMP:20260101T000000Z",
      "DTSTART:20260401T120000Z",
      "RDATE:20260410T120000Z",
      "EXDATE:20260410T120000Z",
      "END:VEVENT",
      "END:VCALENDAR",
      "",
    ].join("\r\n");
    const set = ruleSetFromComponent(parseIcs(new TextEncoder().encode(ics)).components[0]!);
    expect(shown(set.occurrences(10).times)).toEqual(["20260401T120000Z"]);
  });

  it("sorts and de-duplicates the merged result", () => {
    const ics = [
      "BEGIN:VCALENDAR",
      "VERSION:2.0",
      "PRODID:-//V*//test//EN",
      "BEGIN:VEVENT",
      "UID:dedupe",
      "DTSTAMP:20260101T000000Z",
      "DTSTART:20260401T120000Z",
      "RRULE:FREQ=DAILY;COUNT=2",
      "RDATE:20260402T120000Z,20260401T120000Z,20260403T120000Z",
      "END:VEVENT",
      "END:VCALENDAR",
      "",
    ].join("\r\n");
    const set = ruleSetFromComponent(parseIcs(new TextEncoder().encode(ics)).components[0]!);
    expect(shown(set.occurrences(10).times)).toEqual([
      "20260401T120000Z",
      "20260402T120000Z",
      "20260403T120000Z",
    ]);
  });

  it("windows a set", () => {
    const ics = [
      "BEGIN:VCALENDAR",
      "VERSION:2.0",
      "PRODID:-//V*//test//EN",
      "BEGIN:VEVENT",
      "UID:window",
      "DTSTAMP:20260101T000000Z",
      "DTSTART:20260401T120000Z",
      "RRULE:FREQ=DAILY",
      "EXDATE:20260403T120000Z",
      "END:VEVENT",
      "END:VCALENDAR",
      "",
    ].join("\r\n");
    const set = ruleSetFromComponent(parseIcs(new TextEncoder().encode(ics)).components[0]!);
    const got = set.between(
      instant("20260402T120000Z", "set-window.start"),
      instant("20260405T120000Z", "set-window.end"),
    );
    expect(shown(got)).toEqual(["20260402T120000Z", "20260404T120000Z"]);
  });

  it("refuses VALUE=DATE and TZID on EXDATE and RDATE", () => {
    for (const line of [
      "EXDATE;VALUE=DATE:20260402",
      "RDATE;VALUE=DATE:20260402",
      "EXDATE;TZID=America/New_York:20260402T080000",
      "RDATE;TZID=America/New_York:20260402T080000",
    ]) {
      const ics = [
        "BEGIN:VCALENDAR",
        "VERSION:2.0",
        "PRODID:-//V*//test//EN",
        "BEGIN:VEVENT",
        "UID:scope",
        "DTSTAMP:20260101T000000Z",
        "DTSTART:20260401T120000Z",
        line,
        "END:VEVENT",
        "END:VCALENDAR",
        "",
      ].join("\r\n");
      const comp = parseIcs(new TextEncoder().encode(ics)).components[0]!;
      expectSentinel("ErrUnsupportedRRule", () => ruleSetFromComponent(comp), line);
    }
  });
});

/** The first component of a `.ics` set fixture. */
function firstComponent(f: SetFixture) {
  const cal = parseIcs(f.input);
  const first = cal.components[0];
  expect(first, `${f.id}: calendar has no components`).toBeDefined();
  return first!;
}

// ── Date-time lists and RECURRENCE-ID ─────────────────────────────

describe("rrule date-time lists", () => {
  it("parses a list, sorted and de-duplicated", () => {
    const got = parseDateTimeList("20260403T120000Z,20260401T120000Z,20260403T120000Z");
    expect(shown(got)).toEqual(["20260401T120000Z", "20260403T120000Z"]);
  });

  it("rejects an empty list and a non-form-#2 value", () => {
    expectSentinel("ErrMalformed", () => parseDateTimeList(""), "empty list");
    expectSentinel("ErrMalformed", () => parseDateTimeList("20260401"), "date-only");
    expectSentinel("ErrMalformed", () => parseDateTimeList("20260401T120000"), "form #1");
  });

  it("formats a list sorted and de-duplicated", () => {
    const a = instant("20260403T120000Z", "fmt.a");
    const b = instant("20260401T120000Z", "fmt.b");
    expect(formatDateTimeList([a, b, a])).toBe("20260401T120000Z,20260403T120000Z");
    expect(formatDateTimeList([])).toBe("");
  });
});

describe("rrule recurrence id", () => {
  it("parses a bare RECURRENCE-ID as this-instance", () => {
    const rid = parseRecurrenceId({ name: "RECURRENCE-ID", params: [], value: "20260401T120000Z" });
    expect(formatTime(rid.time)).toBe("20260401T120000Z");
    expect(rid.range).toBe("");
  });

  it("parses RANGE=THISANDFUTURE", () => {
    const rid = parseRecurrenceId({
      name: "RECURRENCE-ID",
      params: [{ name: "RANGE", value: "THISANDFUTURE" }],
      value: "20260401T120000Z",
    });
    expect(rid.range).toBe("THISANDFUTURE");
  });

  it("round-trips through a Property, omitting the default RANGE", () => {
    const bare = parseRecurrenceId({ name: "RECURRENCE-ID", params: [], value: "20260401T120000Z" });
    expect(bare.toProperty()).toEqual({
      name: "RECURRENCE-ID",
      params: [],
      value: "20260401T120000Z",
    });
    const future = parseRecurrenceId({
      name: "RECURRENCE-ID",
      params: [{ name: "RANGE", value: "THISANDFUTURE" }],
      value: "20260401T120000Z",
    });
    expect(future.toProperty().params).toEqual([{ name: "RANGE", value: "THISANDFUTURE" }]);
  });

  it("rejects a wrong name, a bad RANGE and a non-form-#2 value", () => {
    expectSentinel(
      "ErrMalformed",
      () => parseRecurrenceId({ name: "DTSTART", params: [], value: "20260401T120000Z" }),
      "wrong name",
    );
    expectSentinel(
      "ErrMalformed",
      () =>
        parseRecurrenceId({
          name: "RECURRENCE-ID",
          params: [{ name: "RANGE", value: "THISONLY" }],
          value: "20260401T120000Z",
        }),
      "bad RANGE",
    );
    expectSentinel(
      "ErrMalformed",
      () => parseRecurrenceId({ name: "RECURRENCE-ID", params: [], value: "20260401" }),
      "date-only value",
    );
  });

  it("rejects VALUE=DATE and TZID as out of scope", () => {
    expectSentinel(
      "ErrUnsupportedRRule",
      () =>
        parseRecurrenceId({
          name: "RECURRENCE-ID",
          params: [{ name: "VALUE", value: "DATE" }],
          value: "20260401",
        }),
      "VALUE=DATE",
    );
    expectSentinel(
      "ErrUnsupportedRRule",
      () =>
        parseRecurrenceId({
          name: "RECURRENCE-ID",
          params: [{ name: "TZID", value: "America/New_York" }],
          value: "20260401T080000",
        }),
      "TZID",
    );
  });
});

// ── Parsing scope — the hard errors spelled out ───────────────────

describe("rrule parsing scope", () => {
  it("defers FREQ=SECONDLY and RSCALE", () => {
    for (const v of ["FREQ=SECONDLY", "FREQ=YEARLY;RSCALE=HEBREW"]) {
      expectSentinel("ErrUnsupportedRRule", () => parseRRule(v), v);
      expectSentinel("ErrUnsupportedRRule", () => validateRRule(v), v);
    }
  });

  it("names the RRULE parsing scope, never a spec version, in the deferral messages", () => {
    const secondly = capture(() => parseRRule("FREQ=SECONDLY")) as VstarError;
    expect(secondly.message).toBe("ErrUnsupportedRRule: rrule: FREQ=SECONDLY: outside the RRULE parsing scope");
    const rscale = capture(() => parseRRule("FREQ=YEARLY;RSCALE=HEBREW")) as VstarError;
    expect(rscale.message).toBe("ErrUnsupportedRRule: rrule: rule-part RSCALE: outside the RRULE parsing scope");
  });

  it("accepts FREQ=MINUTELY", () => {
    expect(parseRRule("FREQ=MINUTELY").freq).toBe("MINUTELY");
  });

  it("rejects the hard errors as malformed", () => {
    for (const v of [
      "",
      "INTERVAL=2",
      "FREQ=DAILY;FOO=BAR",
      "FREQ=DAILY;INTERVAL=0",
      "FREQ=DAILY;INTERVAL=-1",
      "FREQ=MONTHLY;BYMONTHDAY=0",
      "FREQ=MONTHLY;BYDAY=0SU",
      "FREQ=DAILY;UNTIL=20261231T235959Z;COUNT=10",
      "FREQ=DAILY;UNTIL=20261231T235959",
      "FREQ=DAILY;COUNT=0",
      "FREQ=MONTHLY;BYSETPOS=-1",
      "FREQ=MONTHLY;BYWEEKNO=20",
      "FREQ=MONTHLY;BYYEARDAY=100",
      "FREQ=DAILY;FREQ=WEEKLY",
      "FREQ=DAILY;BYMONTH=13",
      "FREQ=DAILY;BYHOUR=24",
      "FREQ=DAILY;BYSECOND=61",
      "FREQ=DAILY;WKST=XX",
      "FREQ=NEVER",
      "FREQ",
      "=DAILY",
    ]) {
      expectSentinel("ErrMalformed", () => parseRRule(v), JSON.stringify(v));
    }
  });

  it("applies the RFC defaults on parse", () => {
    const rule = parseRRule("FREQ=DAILY");
    expect(rule.interval).toBe(1);
    expect(rule.weekStart).toBe("MO");
    expect(rule.count).toBe(0);
    expect(rule.until).toBeUndefined();
  });

  it("accepts BYSETPOS alongside another BY-* clause", () => {
    expect(() => parseRRule("FREQ=MONTHLY;BYDAY=MO;BYSETPOS=-1")).not.toThrow();
  });
});
