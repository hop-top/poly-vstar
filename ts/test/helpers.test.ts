// SPDX-License-Identifier: MIT

// The `helpers` surface: constructors, accessors, and the mutators that
// keep X-VSTAR-HASH matching the component's own canonical bytes.
//
// The emitter gate at the foot of this file is the load-bearing half:
// it rebuilds two committed conformance fixtures through the helper API
// and asserts the canonical BYTES and the hash match what the reference
// produced. A port whose constructors drift shows up there, not in a
// unit assertion about its own output.

import { readFileSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { calendar as canonicalCalendar, card as canonicalCard } from "../src/canonical/index.js";
import { VDuration } from "../src/duration/index.js";
import { VstarError } from "../src/errors.js";
import {
  X_VSTAR_HASH_PROPERTY,
  calendar as hashCalendar,
  card as hashCard,
  verifyXVstar,
} from "../src/hashing/index.js";
import {
  RELATED_END,
  RELATED_START,
  addCategory,
  addRelatedTo,
  alarmFiresAt,
  alarmRepeatCycle,
  alarmTrigger,
  categories,
  classOf,
  classOrDefault,
  complete,
  due,
  eventEnd,
  eventStatus,
  incrementSequence,
  journalStatus,
  newAbsoluteAlarm,
  newAlarm,
  newCalendar,
  newCard,
  newEvent,
  newFreeBusy,
  newJournal,
  newRelativeAlarm,
  newTodo,
  percentComplete,
  priority,
  relatedTo,
  removePercentComplete,
  removePriority,
  sequence,
  setCategories,
  setClass,
  setDue,
  setEventStatus,
  setJournalStatus,
  setPercentComplete,
  setPriority,
  setSequence,
  setTodoStatus,
  setTransp,
  todoStatus,
  transp,
  transpOrDefault,
} from "../src/helpers/index.js";
import { parseTime } from "../src/time.js";
import type { Calendar, Card, Component, Property } from "../src/types.js";
import { get, getAll, remove } from "../src/types.js";
import { CONFORMANCE_DIR, assertBytesEqual } from "./fixtures.js";

const DUE_AT = parseTime("20260601T090000Z") ?? 0;
const START_AT = parseTime("20260601T080000Z") ?? 0;
const END_AT = parseTime("20260601T100000Z") ?? 0;

function prop(name: string, value: string): Property {
  return { name, params: [], value };
}

/** The `X-VSTAR-HASH` property of `c`, if it carries one. */
function storedHash(c: Component): string | undefined {
  return get(c, X_VSTAR_HASH_PROPERTY)?.value;
}

describe("constructors", () => {
  it.each([
    ["newTodo", () => newTodo("u", DUE_AT), "VTODO", "DUE"],
    ["newJournal", () => newJournal("u", START_AT), "VJOURNAL", "DTSTART"],
    ["newEvent", () => newEvent("u", START_AT, END_AT), "VEVENT", "DTSTART"],
    ["newFreeBusy", () => newFreeBusy("u", START_AT, END_AT), "VFREEBUSY", "DTSTART"],
    ["newAlarm", () => newAlarm("u", "DISPLAY", "-PT15M"), "VALARM", "TRIGGER"],
  ])("%s sets UID, DTSTAMP and X-VSTAR-HASH", (_name, build, type, anchorProp) => {
    const c = build();
    expect(c.type).toBe(type);
    expect(get(c, "UID")?.value).toBe("u");
    expect(get(c, "DTSTAMP")?.value).toMatch(/^\d{8}T\d{6}Z$/);
    expect(get(c, anchorProp)).toBeDefined();
    expect(storedHash(c)).toMatch(/^sha256:[0-9a-f]{64}$/);
  });

  it.each([
    ["newTodo", () => newTodo("u", DUE_AT)],
    ["newJournal", () => newJournal("u", START_AT)],
    ["newEvent", () => newEvent("u", START_AT, END_AT)],
    ["newFreeBusy", () => newFreeBusy("u", START_AT, END_AT)],
    ["newAlarm", () => newAlarm("u", "DISPLAY", "-PT15M")],
    ["newRelativeAlarm", () => newRelativeAlarm("u", "DISPLAY", new VDuration({ negative: true, minutes: 15 }), RELATED_START)],
    ["newAbsoluteAlarm", () => newAbsoluteAlarm("u", "DISPLAY", START_AT)],
  ])("%s stamps a hash that verifies against its own canonical form", (_name, build) => {
    expect(verifyXVstar(build()).ok).toBe(true);
  });

  it.each([
    ["newTodo", () => newTodo("", DUE_AT)],
    ["newJournal", () => newJournal("", START_AT)],
    ["newEvent", () => newEvent("", START_AT, END_AT)],
    ["newFreeBusy", () => newFreeBusy("", START_AT, END_AT)],
    ["newAlarm", () => newAlarm("", "DISPLAY", "-PT15M")],
    ["newRelativeAlarm", () => newRelativeAlarm("", "DISPLAY", new VDuration(), RELATED_START)],
    ["newAbsoluteAlarm", () => newAbsoluteAlarm("", "DISPLAY", START_AT)],
  ])("%s throws ErrMissingUID on an empty uid", (_name, build) => {
    expect(build).toThrow(VstarError);
    try {
      build();
      expect.unreachable("constructor accepted an empty uid");
    } catch (e) {
      expect((e as VstarError).code).toBe("ErrMissingUID");
    }
  });

  it("newEvent sets both ends", () => {
    const c = newEvent("u", START_AT, END_AT);
    expect(get(c, "DTSTART")?.value).toBe("20260601T080000Z");
    expect(get(c, "DTEND")?.value).toBe("20260601T100000Z");
  });

  it("newCalendar cannot fail and defaults an empty PRODID", () => {
    expect(newCalendar("-//Mine//EN").prodId).toBe("-//Mine//EN");
    expect(newCalendar("").prodId).not.toBe("");
    expect(newCalendar("").components).toEqual([]);
  });

  it("newCard defaults an empty kind to individual and stamps no hash", () => {
    const card = newCard("u", "");
    expect(card.kind).toBe("individual");
    expect(get({ type: "VTODO", props: card.props, sub: [] }, "KIND")?.value).toBe("individual");
    // A Card carries no X-VSTAR-HASH property of its own in v0.1.
    expect(card.props.some((p) => p.name === X_VSTAR_HASH_PROPERTY)).toBe(false);
  });

  it("newCard honours an explicit kind", () => {
    expect(newCard("u", "group").kind).toBe("group");
  });
});

describe("alarms", () => {
  const parentEvent = (): Component => {
    const c = newEvent("e", START_AT, END_AT);
    return c;
  };
  const cal: Calendar = { prodId: "-//V*//T//EN", components: [] };

  it("newRelativeAlarm leaves RELATED=START implicit, the RFC default", () => {
    const a = newRelativeAlarm("a", "DISPLAY", new VDuration({ negative: true, minutes: 15 }), RELATED_START);
    const trigger = get(a, "TRIGGER");
    expect(trigger?.value).toBe("-PT15M");
    expect(trigger?.params).toEqual([]);
  });

  it("newRelativeAlarm emits RELATED=END explicitly", () => {
    const a = newRelativeAlarm("a", "DISPLAY", new VDuration({ negative: true, minutes: 15 }), RELATED_END);
    expect(get(a, "TRIGGER")?.params).toEqual([{ name: "RELATED", value: "END" }]);
  });

  it("newAbsoluteAlarm carries VALUE=DATE-TIME so no consumer has to infer the form", () => {
    const a = newAbsoluteAlarm("a", "DISPLAY", START_AT);
    const trigger = get(a, "TRIGGER");
    expect(trigger?.value).toBe("20260601T080000Z");
    expect(trigger?.params).toEqual([{ name: "VALUE", value: "DATE-TIME" }]);
  });

  it("alarmTrigger decodes what the constructors wrote", () => {
    const rel = alarmTrigger(newRelativeAlarm("a", "DISPLAY", new VDuration({ negative: true, minutes: 15 }), RELATED_START));
    expect(rel.relative).toBe(true);
    expect(rel.duration.toString()).toBe("-PT15M");

    const abs = alarmTrigger(newAbsoluteAlarm("a", "DISPLAY", START_AT));
    expect(abs.relative).toBe(false);
    expect(abs.absolute).toBe(START_AT);
  });

  it("alarmFiresAt offsets a relative trigger from DTSTART", () => {
    const alarm = newRelativeAlarm("a", "DISPLAY", new VDuration({ negative: true, minutes: 15 }), RELATED_START);
    expect(alarmFiresAt(alarm, parentEvent(), cal)).toBe(START_AT - 15 * 60 * 1000);
  });

  it("alarmFiresAt offsets a RELATED=END trigger from the parent's end", () => {
    const alarm = newRelativeAlarm("a", "DISPLAY", new VDuration({ negative: true, minutes: 30 }), RELATED_END);
    expect(alarmFiresAt(alarm, parentEvent(), cal)).toBe(END_AT - 30 * 60 * 1000);
  });

  it("alarmFiresAt returns an absolute trigger's instant directly", () => {
    const alarm = newAbsoluteAlarm("a", "DISPLAY", DUE_AT);
    expect(alarmFiresAt(alarm, parentEvent(), cal)).toBe(DUE_AT);
  });

  it("alarmFiresAt throws ErrNoTrigger for a VALARM with no TRIGGER", () => {
    const alarm = newAlarm("a", "DISPLAY", "-PT15M");
    remove(alarm, "TRIGGER");
    try {
      alarmFiresAt(alarm, parentEvent(), cal);
      expect.unreachable("a TRIGGER-less alarm resolved");
    } catch (e) {
      expect((e as VstarError).code).toBe("ErrNoTrigger");
    }
  });

  it("alarmFiresAt throws ErrNoAnchor when the parent lacks the anchor", () => {
    const alarm = newRelativeAlarm("a", "DISPLAY", new VDuration({ minutes: 15 }), RELATED_START);
    const anchorless: Component = { type: "VEVENT", props: [prop("UID", "e")], sub: [] };
    try {
      alarmFiresAt(alarm, anchorless, cal);
      expect.unreachable("a trigger resolved against a missing anchor");
    } catch (e) {
      expect((e as VstarError).code).toBe("ErrNoAnchor");
    }
  });

  it("alarmRepeatCycle reads DURATION and REPEAT", () => {
    const alarm = newAlarm("a", "DISPLAY", "-PT15M");
    alarm.props.push(prop("DURATION", "PT5M"), prop("REPEAT", "3"));
    const [d, n] = alarmRepeatCycle(alarm);
    expect(d.toString()).toBe("PT5M");
    expect(n).toBe(3);
  });

  it("eventEnd reports DTEND", () => {
    expect(eventEnd(parentEvent(), cal)).toBe(END_AT);
  });

  it("eventEnd computes DTSTART + DURATION when there is no DTEND", () => {
    const c = newEvent("e", START_AT, END_AT);
    remove(c, "DTEND");
    c.props.push(prop("DURATION", "PT90M"));
    expect(eventEnd(c, cal)).toBe(START_AT + 90 * 60 * 1000);
  });

  it("eventEnd does NOT fall back to a VTODO's DUE", () => {
    // The DUE fallback belongs to a RELATED=END trigger's anchor, not
    // to eventEnd, which models only RFC 5545 §3.6.1's two VEVENT
    // forms. alarmFiresAt is where a VTODO's DUE becomes the end.
    expect(eventEnd(newTodo("t", DUE_AT), cal)).toBeUndefined();

    const alarm = newRelativeAlarm("a", "DISPLAY", new VDuration({ negative: true, minutes: 10 }), RELATED_END);
    expect(alarmFiresAt(alarm, newTodo("t", DUE_AT), cal)).toBe(DUE_AT - 10 * 60 * 1000);
  });

  it("eventEnd reports undefined when there is no end anchor at all", () => {
    const c = newEvent("e", START_AT, END_AT);
    remove(c, "DTEND");
    expect(eventEnd(c, cal)).toBeUndefined();
  });
});

describe("categories", () => {
  const todo = (): Component => newTodo("u", DUE_AT);

  it("returns an empty list when CATEGORIES is absent", () => {
    expect(categories(todo())).toEqual([]);
  });

  it("splits on commas and trims adjacent whitespace", () => {
    const c = todo();
    c.props.push(prop("CATEGORIES", " Work , Home "));
    expect(categories(c)).toEqual(["Work", "Home"]);
  });

  it("drops empty tokens from leading, trailing and doubled commas", () => {
    const c = todo();
    c.props.push(prop("CATEGORIES", ",Work,,Home,"));
    expect(categories(c)).toEqual(["Work", "Home"]);
  });

  it("setCategories joins with no space after the comma", () => {
    const c = todo();
    setCategories(c, ["Work", "Home"]);
    expect(get(c, "CATEGORIES")?.value).toBe("Work,Home");
  });

  it("setCategories drops duplicates keeping first-seen order", () => {
    const c = todo();
    setCategories(c, ["Work", "Home", "Work"]);
    expect(get(c, "CATEGORIES")?.value).toBe("Work,Home");
  });

  it("setCategories compares case-sensitively — these are user labels", () => {
    const c = todo();
    setCategories(c, ["Work", "work"]);
    expect(get(c, "CATEGORIES")?.value).toBe("Work,work");
  });

  it("setCategories removes the property on empty input", () => {
    const c = todo();
    setCategories(c, ["Work"]);
    setCategories(c, []);
    expect(get(c, "CATEGORIES")).toBeUndefined();
  });

  it("setCategories refreshes the hash", () => {
    const c = todo();
    setCategories(c, ["Work"]);
    expect(verifyXVstar(c).ok).toBe(true);
  });

  it("addCategory appends when absent and refreshes the hash", () => {
    const c = todo();
    addCategory(c, "Work");
    addCategory(c, "Home");
    expect(categories(c)).toEqual(["Work", "Home"]);
    expect(verifyXVstar(c).ok).toBe(true);
  });

  it("addCategory is a no-op for a duplicate or an empty value", () => {
    const c = todo();
    addCategory(c, "Work");
    const before = storedHash(c);
    addCategory(c, "Work");
    addCategory(c, "");
    expect(categories(c)).toEqual(["Work"]);
    // Nothing changed, so no hash churn either.
    expect(storedHash(c)).toBe(before);
  });
});

describe("relations", () => {
  const todo = (): Component => newTodo("u", DUE_AT);

  it("returns an empty list when there is no RELATED-TO", () => {
    expect(relatedTo(todo())).toEqual([]);
  });

  it("addRelatedTo writes the typed RELTYPE as a parameter", () => {
    const c = todo();
    addRelatedTo(c, "parent-1", "CHILD");
    expect(get(c, "RELATED-TO")?.params).toEqual([{ name: "RELTYPE", value: "CHILD" }]);
    expect(verifyXVstar(c).ok).toBe(true);
  });

  it("addRelatedTo omits the parameter for an empty relType", () => {
    const c = todo();
    addRelatedTo(c, "parent-1", "");
    expect(get(c, "RELATED-TO")?.params).toEqual([]);
    // A consumer reading through relatedTo sees the RFC default.
    expect(relatedTo(c)).toEqual([{ uid: "parent-1", relType: "PARENT" }]);
  });

  it("addRelatedTo appends rather than replacing", () => {
    const c = todo();
    addRelatedTo(c, "a", "CHILD");
    addRelatedTo(c, "b", "SIBLING");
    expect(getAll(c, "RELATED-TO").length).toBe(2);
    expect(relatedTo(c)).toEqual([
      { uid: "a", relType: "CHILD" },
      { uid: "b", relType: "SIBLING" },
    ]);
  });

  it("addRelatedTo is a no-op on an empty uid", () => {
    const c = todo();
    const before = storedHash(c);
    addRelatedTo(c, "", "CHILD");
    expect(getAll(c, "RELATED-TO")).toEqual([]);
    expect(storedHash(c)).toBe(before);
  });

  it("relatedTo folds a registered RELTYPE to canonical case", () => {
    const c = todo();
    c.props.push({ name: "RELATED-TO", params: [{ name: "reltype", value: "child" }], value: "a" });
    expect(relatedTo(c)).toEqual([{ uid: "a", relType: "CHILD" }]);
  });

  it("relatedTo passes an unregistered extension value through verbatim", () => {
    const c = todo();
    addRelatedTo(c, "a", "X-CUSTOM-LINK");
    expect(relatedTo(c)).toEqual([{ uid: "a", relType: "X-CUSTOM-LINK" }]);
  });

  it("relatedTo defaults an absent RELTYPE to PARENT per RFC 5545 §3.2.15", () => {
    const c = todo();
    c.props.push(prop("RELATED-TO", "a"));
    expect(relatedTo(c)).toEqual([{ uid: "a", relType: "PARENT" }]);
  });
});

describe("classification and transparency", () => {
  const event = (): Component => newEvent("u", START_AT, END_AT);
  const journal = (): Component => newJournal("u", START_AT);

  it("classOf reports absence faithfully rather than defaulting", () => {
    expect(classOf(event())).toBeUndefined();
  });

  it("classOrDefault applies the RFC's PUBLIC default", () => {
    expect(classOrDefault(event())).toBe("PUBLIC");
  });

  it("classOrDefault also absorbs an unrecognized value — flagging it is validate's job", () => {
    const c = event();
    c.props.push(prop("CLASS", "SEMI-SECRET"));
    expect(classOf(c)).toBeUndefined();
    expect(classOrDefault(c)).toBe("PUBLIC");
  });

  it("setClass writes on the three types that admit it and refreshes the hash", () => {
    for (const build of [event, journal, () => newTodo("u", DUE_AT)]) {
      const c = build();
      setClass(c, "PRIVATE");
      expect(classOf(c)).toBe("PRIVATE");
      expect(verifyXVstar(c).ok).toBe(true);
    }
  });

  it("setClass is a no-op on a type that does not admit CLASS", () => {
    const c = newAlarm("u", "DISPLAY", "-PT15M");
    const before = storedHash(c);
    setClass(c, "PRIVATE");
    expect(get(c, "CLASS")).toBeUndefined();
    expect(storedHash(c)).toBe(before);
  });

  it("transp reports absence faithfully; transpOrDefault applies OPAQUE", () => {
    expect(transp(event())).toBeUndefined();
    expect(transpOrDefault(event())).toBe("OPAQUE");
  });

  it("setTransp writes on a VEVENT only", () => {
    const e = event();
    setTransp(e, "TRANSPARENT");
    expect(transp(e)).toBe("TRANSPARENT");

    const j = journal();
    const before = storedHash(j);
    setTransp(j, "TRANSPARENT");
    expect(get(j, "TRANSP")).toBeUndefined();
    expect(storedHash(j)).toBe(before);
  });
});

describe("integer-valued properties", () => {
  const todo = (): Component => newTodo("u", DUE_AT);
  const event = (): Component => newEvent("u", START_AT, END_AT);

  it("sequence reports absence rather than the RFC default of zero", () => {
    expect(sequence(todo())).toBeUndefined();
  });

  it("setSequence writes and refreshes the hash", () => {
    const c = todo();
    setSequence(c, 3);
    expect(sequence(c)).toBe(3);
    expect(verifyXVstar(c).ok).toBe(true);
  });

  it("setSequence rejects a negative counter rather than clamping", () => {
    const c = todo();
    setSequence(c, 3);
    setSequence(c, -1);
    expect(sequence(c)).toBe(3);
  });

  it("incrementSequence treats an absent counter as the RFC default of zero", () => {
    const c = todo();
    incrementSequence(c);
    expect(sequence(c)).toBe(1);
    incrementSequence(c);
    expect(sequence(c)).toBe(2);
    expect(verifyXVstar(c).ok).toBe(true);
  });

  it("incrementSequence is a no-op on a type that does not carry SEQUENCE", () => {
    const c = newAlarm("u", "DISPLAY", "-PT15M");
    incrementSequence(c);
    expect(get(c, "SEQUENCE")).toBeUndefined();
  });

  it("sequence rejects a padded or signed wire value", () => {
    for (const raw of ["+3", "03", " 3", "-1", "three", ""]) {
      const c = todo();
      c.props.push(prop("SEQUENCE", raw));
      expect(sequence(c)).toBeUndefined();
    }
  });

  it("priority keeps an explicit zero distinct from absence", () => {
    const c = todo();
    expect(priority(c)).toBeUndefined();
    setPriority(c, 0);
    expect(priority(c)).toBe(0);
  });

  it("setPriority rejects out-of-range input rather than clamping", () => {
    const c = todo();
    setPriority(c, 3);
    setPriority(c, 10);
    setPriority(c, -1);
    expect(priority(c)).toBe(3);
  });

  it("setPriority applies to VEVENT and VTODO only", () => {
    const e = event();
    setPriority(e, 5);
    expect(priority(e)).toBe(5);

    const j = newJournal("u", START_AT);
    setPriority(j, 5);
    expect(get(j, "PRIORITY")).toBeUndefined();
  });

  it("removePriority is the counterpart to an explicit zero", () => {
    const c = todo();
    setPriority(c, 0);
    removePriority(c);
    expect(priority(c)).toBeUndefined();
    expect(verifyXVstar(c).ok).toBe(true);
  });

  it("percentComplete keeps an explicit zero distinct from absence", () => {
    const c = todo();
    expect(percentComplete(c)).toBeUndefined();
    setPercentComplete(c, 0);
    expect(percentComplete(c)).toBe(0);
  });

  it("setPercentComplete rejects out of range rather than asserting the task is done", () => {
    const c = todo();
    setPercentComplete(c, 50);
    setPercentComplete(c, 120);
    expect(percentComplete(c)).toBe(50);
  });

  it("setPercentComplete applies to VTODO only", () => {
    const e = event();
    setPercentComplete(e, 50);
    expect(get(e, "PERCENT-COMPLETE")).toBeUndefined();
  });

  it("removePercentComplete does not gate on type — removal is always safe", () => {
    const c = todo();
    setPercentComplete(c, 50);
    removePercentComplete(c);
    expect(percentComplete(c)).toBeUndefined();
    expect(verifyXVstar(c).ok).toBe(true);
  });
});

describe("status", () => {
  const todo = (): Component => newTodo("u", DUE_AT);
  const event = (): Component => newEvent("u", START_AT, END_AT);
  const journal = (): Component => newJournal("u", START_AT);

  it("setTodoStatus writes on a VTODO and refreshes the hash", () => {
    const c = todo();
    setTodoStatus(c, "IN-PROCESS");
    expect(todoStatus(c)).toBe("IN-PROCESS");
    expect(verifyXVstar(c).ok).toBe(true);
  });

  it("the three vocabularies do not bleed into each other", () => {
    const e = event();
    setTodoStatus(e, "IN-PROCESS");
    expect(get(e, "STATUS")).toBeUndefined();

    const j = journal();
    setEventStatus(j, "CONFIRMED");
    expect(get(j, "STATUS")).toBeUndefined();

    const t = todo();
    setJournalStatus(t, "FINAL");
    expect(get(t, "STATUS")).toBeUndefined();
  });

  it("a getter reports undefined for a value outside its own vocabulary", () => {
    const c = todo();
    setTodoStatus(c, "NEEDS-ACTION");
    // NEEDS-ACTION is not in the VEVENT vocabulary.
    expect(eventStatus(c)).toBeUndefined();
    expect(journalStatus(c)).toBeUndefined();
    expect(todoStatus(c)).toBe("NEEDS-ACTION");
  });

  it("CANCELLED is spelled the same in all three and reads in all three", () => {
    const c = todo();
    setTodoStatus(c, "CANCELLED");
    expect(todoStatus(c)).toBe("CANCELLED");
    expect(eventStatus(c)).toBe("CANCELLED");
    expect(journalStatus(c)).toBe("CANCELLED");
  });

  it("setEventStatus and setJournalStatus write on their own types", () => {
    const e = event();
    setEventStatus(e, "CONFIRMED");
    expect(eventStatus(e)).toBe("CONFIRMED");

    const j = journal();
    setJournalStatus(j, "FINAL");
    expect(journalStatus(j)).toBe("FINAL");
  });

  it("complete writes all three done markers atomically with a matching hash", () => {
    const c = todo();
    complete(c, DUE_AT);
    expect(todoStatus(c)).toBe("COMPLETED");
    expect(get(c, "COMPLETED")?.value).toBe("20260601T090000Z");
    expect(percentComplete(c)).toBe(100);
    expect(verifyXVstar(c).ok).toBe(true);
  });

  it("complete is a no-op on anything but a VTODO", () => {
    const e = event();
    const before = storedHash(e);
    complete(e, DUE_AT);
    expect(get(e, "STATUS")).toBeUndefined();
    expect(storedHash(e)).toBe(before);
  });
});

describe("due", () => {
  const cal: Calendar = { prodId: "-//V*//T//EN", components: [] };

  it("reads what newTodo wrote", () => {
    expect(due(newTodo("u", DUE_AT), cal)).toBe(DUE_AT);
  });

  it("setDue writes UTC form #2 and refreshes the hash", () => {
    const c = newTodo("u", DUE_AT);
    setDue(c, END_AT);
    expect(get(c, "DUE")?.value).toBe("20260601T100000Z");
    expect(due(c, cal)).toBe(END_AT);
    expect(verifyXVstar(c).ok).toBe(true);
  });
});

// ---------------------------------------------------------------------
// The emitter gate
// ---------------------------------------------------------------------

/** Read one conformance-corpus sibling as raw bytes. */
function corpusBytes(family: string, name: string): Uint8Array {
  return new Uint8Array(readFileSync(join(CONFORMANCE_DIR, family, name)));
}

/**
 * Strip `\r` before `\n` — the one licensed comparison transform.
 *
 * The corpus is LF on disk and the canonical form is CRLF (spec rule
 * 1), so one side has to move. It runs on OUR bytes, never on the
 * file's: converting the file's LF up to CRLF would silently repair a
 * bare `\n` our encoder should never have emitted.
 *
 * The hash below is taken over the CRLF bytes, not these.
 */
function crlfToLf(b: Uint8Array): Uint8Array {
  const out = new Uint8Array(b.length);
  let n = 0;
  for (let i = 0; i < b.length; i++) {
    if (b[i] === 0x0d && b[i + 1] === 0x0a) continue;
    out[n++] = b[i] as number;
  }
  return out.subarray(0, n);
}

/** Read one conformance-corpus `.hash` sibling as its single line. */
function corpusHash(family: string, stem: string): string {
  return readFileSync(join(CONFORMANCE_DIR, family, `${stem}.hash`), "utf8").trim();
}

describe("emitter gate — a fixture rebuilt through the helper API", () => {
  it("rfc5545/one_vtodo matches the committed canonical bytes and hash", () => {
    // The constructors stamp DTSTAMP with the wall clock, so the one
    // property the fixture pins to a literal is restamped here. Every
    // other byte — property set, ordering, the X-VSTAR-HASH the
    // constructor wrote and canonicalization then strips — comes out of
    // the helper API untouched.
    const todo = newTodo("abc-123", parseTime("20260504T120000Z") ?? 0);
    todo.props = todo.props.filter((p) => p.name !== "DUE");
    todo.props.push(prop("SUMMARY", "Buy milk"));
    setPriority(todo, 3);
    todo.props = todo.props.map((p) =>
      p.name === "DTSTAMP" ? prop("DTSTAMP", "20260504T120000Z") : p,
    );

    const cal: Calendar = newCalendar("-//V*//OneVTODO//EN");
    cal.components.push(todo);

    assertBytesEqual(
      crlfToLf(canonicalCalendar(cal)),
      corpusBytes("rfc5545", "one_vtodo.canonical"),
      "one_vtodo canonical",
    );
    // The hash covers the CRLF bytes, not the LF-transformed ones.
    expect(hashCalendar(cal)).toBe(corpusHash("rfc5545", "one_vtodo"));
  });

  it("rfc6350/minimal matches the committed canonical bytes and hash", () => {
    // The fixture carries no KIND, and newCard defaults an empty kind
    // to individual — so the rebuild clears the model field and the
    // property the constructor derived from it. That divergence is the
    // constructor's documented default, not a canonicalization bug.
    const card: Card = newCard("urn:uuid:11111111-1111-1111-1111-111111111111", "");
    card.kind = "";
    card.props = card.props.filter((p) => p.name !== "KIND" && p.name !== "VERSION" && p.name !== "UID");
    card.props.push(prop("FN", "Jad Bitar"));

    assertBytesEqual(
      crlfToLf(canonicalCard(card)),
      corpusBytes("rfc6350", "minimal.canonical"),
      "minimal canonical",
    );
    expect(hashCard(card)).toBe(corpusHash("rfc6350", "minimal"));
  });
});
