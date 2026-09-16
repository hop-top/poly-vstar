// SPDX-License-Identifier: MIT

// RFC 5545 §3.3.10 RRULE value parsing, for the scope
// spec/v1.0/03 §RRULE parsing scope fixes.
//
// Two failure classes, and the fixtures under `rrule/rejected/`
// distinguish them:
//
// - `ErrMalformed` — syntactically wrong: unknown rule-part, missing
//   FREQ, a zero ordinal, INTERVAL below 1, UNTIL and COUNT together,
//   UNTIL outside form #2, a value out of its RFC range.
// - `ErrUnsupportedRRule` — syntactically fine but deferred by this
//   scope: FREQ=SECONDLY, RSCALE.
//
// Getting the two the wrong way round passes "it failed" and fails the
// corpus, which asserts *which* failure.

import { VstarError } from "../errors.js";
import { parseTime } from "../time.js";
import type { ByDay, Freq, RuleFields, Weekday } from "./types.js";
import { WEEKDAYS } from "./types.js";

/** Throw `ErrMalformed` with the given explanation. */
function malformed(message: string): never {
  throw new VstarError("ErrMalformed", `rrule: ${message}`);
}

/** Throw `ErrUnsupportedRRule` with the given explanation. */
function unsupported(message: string): never {
  throw new VstarError("ErrUnsupportedRRule", `rrule: ${message}`);
}

/** The mutable accumulator the rule-part handlers fill in. */
interface Draft {
  freq: Freq;
  interval: number;
  until: number | undefined;
  count: number;
  byDay: ByDay[];
  byMonth: number[];
  byMonthDay: number[];
  byHour: number[];
  byMinute: number[];
  bySecond: number[];
  byYearDay: number[];
  byWeekNo: number[];
  bySetPos: number[];
  weekStart: Weekday;
}

/** A fresh draft carrying the RFC defaults: `INTERVAL=1`, `WKST=MO`. */
function newDraft(): Draft {
  return {
    freq: "INVALID",
    interval: 1,
    until: undefined,
    count: 0,
    byDay: [],
    byMonth: [],
    byMonthDay: [],
    byHour: [],
    byMinute: [],
    bySecond: [],
    byYearDay: [],
    byWeekNo: [],
    bySetPos: [],
    weekStart: "MO",
  };
}

/**
 * Decompose an RRULE property value — no `RRULE:` prefix — into its
 * fields, applying the RFC defaults and enforcing every cross-field
 * invariant.
 *
 * Keys and values are matched case-sensitively in their RFC wire
 * spelling, matching the strict posture {@link parseTime} takes. The
 * order of rule-parts is irrelevant.
 */
export function parseRuleFields(s: string): RuleFields {
  if (s === "") malformed("empty input");

  const draft = newDraft();
  const seen = new Set<string>();

  for (const part of s.split(";")) {
    const eq = part.indexOf("=");
    // An empty key (`=DAILY`) is as malformed as a missing separator.
    if (eq <= 0) malformed(`malformed rule-part ${JSON.stringify(part)}`);
    const key = part.slice(0, eq);
    const value = part.slice(eq + 1);
    if (seen.has(key)) malformed(`duplicate rule-part ${JSON.stringify(key)}`);
    seen.add(key);
    applyRulePart(draft, key, value);
  }

  validateAfterParse(draft);
  return draft;
}

/** Dispatch one `KEY=VALUE` pair to its field handler. */
function applyRulePart(d: Draft, key: string, value: string): void {
  switch (key) {
    case "FREQ":
      d.freq = parseFreq(value);
      return;
    case "INTERVAL":
      d.interval = parseBoundedInt("INTERVAL", value, 1, Number.MAX_SAFE_INTEGER);
      return;
    case "UNTIL": {
      const t = parseTime(value);
      if (t === undefined) {
        malformed(`UNTIL must be RFC 5545 form #2 (UTC, Z-suffixed), got ${JSON.stringify(value)}`);
      }
      d.until = t;
      return;
    }
    case "COUNT":
      d.count = parseBoundedInt("COUNT", value, 1, Number.MAX_SAFE_INTEGER);
      return;
    case "BYDAY":
      d.byDay = parseByDayList(value);
      return;
    case "BYMONTH":
      d.byMonth = parseIntList("BYMONTH", value, 1, 12);
      return;
    case "BYMONTHDAY":
      d.byMonthDay = parseSignedIntList("BYMONTHDAY", value, 1, 31);
      return;
    case "BYHOUR":
      d.byHour = parseIntList("BYHOUR", value, 0, 23);
      return;
    case "BYMINUTE":
      d.byMinute = parseIntList("BYMINUTE", value, 0, 59);
      return;
    case "BYSECOND":
      // 60 is retained for leap seconds per RFC 5545 §3.3.10.
      d.bySecond = parseIntList("BYSECOND", value, 0, 60);
      return;
    case "BYYEARDAY":
      d.byYearDay = parseSignedIntList("BYYEARDAY", value, 1, 366);
      return;
    case "BYWEEKNO":
      d.byWeekNo = parseSignedIntList("BYWEEKNO", value, 1, 53);
      return;
    case "BYSETPOS":
      d.bySetPos = parseSignedIntList("BYSETPOS", value, 1, 366);
      return;
    case "WKST":
      d.weekStart = parseWeekday(value);
      return;
    case "RSCALE":
      // RFC 7529, non-Gregorian calendars — deferred indefinitely.
      unsupported(`rule-part ${key}: outside the RRULE parsing scope`);
      return;
    default:
      malformed(`unknown rule-part ${JSON.stringify(key)}`);
  }
}

/** The `FREQ` token, rejecting the one deferred frequency by name. */
function parseFreq(v: string): Freq {
  switch (v) {
    case "MINUTELY":
    case "HOURLY":
    case "DAILY":
    case "WEEKLY":
    case "MONTHLY":
    case "YEARLY":
      return v;
    case "SECONDLY":
      // Syntactically valid, deliberately deferred: extreme expansion.
      unsupported(`FREQ=${v}: outside the RRULE parsing scope`);
      break;
    default:
      malformed(`invalid FREQ value ${JSON.stringify(v)}`);
  }
}

/**
 * A base-10 integer, strictly spelled.
 *
 * `Number()` is deliberately not used: it accepts `"0x10"`, `"1e3"`,
 * `" 5 "` and `""`, each of which would turn a malformed rule-part into
 * a plausible value.
 */
function strictInt(name: string, raw: string): number {
  if (!/^[+-]?\d+$/.test(raw)) {
    malformed(`${name} non-integer ${JSON.stringify(raw)}`);
  }
  const n = Number(raw);
  if (!Number.isSafeInteger(n)) malformed(`${name} out of range ${JSON.stringify(raw)}`);
  return n;
}

/** A single integer constrained to `[lo, hi]`. */
function parseBoundedInt(name: string, v: string, lo: number, hi: number): number {
  const n = strictInt(name, v);
  if (n < lo || n > hi) malformed(`${name} must be >= ${lo}, got ${n}`);
  return n;
}

/** A comma-separated list of integers, each in `[lo, hi]`. */
function parseIntList(name: string, v: string, lo: number, hi: number): number[] {
  if (v === "") malformed(`${name} empty`);
  return v.split(",").map((raw) => {
    const n = strictInt(name, raw);
    if (n < lo || n > hi) malformed(`${name} ${n} out of range ${lo}..${hi}`);
    return n;
  });
}

/**
 * A comma-separated list of signed integers where each `n` satisfies
 * `lo <= |n| <= hi` and `n !== 0`.
 *
 * The two-sided range is RFC 5545 §3.3.10's "from the start (positive)
 * or from the end (negative)" pattern, shared by BYMONTHDAY,
 * BYYEARDAY, BYWEEKNO and BYSETPOS. Zero is rejected for all four: it
 * would name neither end.
 */
function parseSignedIntList(name: string, v: string, lo: number, hi: number): number[] {
  if (v === "") malformed(`${name} empty`);
  return v.split(",").map((raw) => {
    const n = strictInt(name, raw);
    if (n === 0) malformed(`${name} 0 invalid (RFC 5545 §3.3.10)`);
    const abs = Math.abs(n);
    if (abs < lo || abs > hi) {
      malformed(`${name} ${n} out of range -${hi}..-${lo} or ${lo}..${hi}`);
    }
    return n;
  });
}

/** A `BYDAY` list: `[<ordinal>]<weekday>` entries, authored order kept. */
function parseByDayList(v: string): ByDay[] {
  if (v === "") malformed("BYDAY empty");
  return v.split(",").map(parseByDayEntry);
}

/** One `BYDAY` entry. The weekday is the final two characters. */
function parseByDayEntry(s: string): ByDay {
  if (s.length < 2) malformed(`BYDAY entry ${JSON.stringify(s)} too short`);
  const weekday = parseWeekday(s.slice(-2));
  const prefix = s.slice(0, -2);
  if (prefix === "") return { ordinal: 0, weekday };
  const n = strictInt("BYDAY ordinal", prefix);
  // The explicit "0" prefix is invalid per RFC 5545 §3.3.10 — it is
  // spelled by omitting the ordinal, not by writing zero.
  if (n === 0) malformed("BYDAY ordinal 0 invalid (RFC 5545 §3.3.10)");
  if (n < -53 || n > 53) malformed(`BYDAY ordinal ${n} out of range -53..53`);
  return { ordinal: n, weekday };
}

/** A two-letter weekday symbol. */
function parseWeekday(s: string): Weekday {
  const found = WEEKDAYS.find((w) => w === s);
  if (found === undefined) malformed(`invalid weekday ${JSON.stringify(s)}`);
  return found;
}

/**
 * The cross-field invariants, run once every rule-part is consumed —
 * rule-part order is irrelevant per RFC, so none of these can be
 * checked while parsing.
 */
function validateAfterParse(d: Draft): void {
  if (d.freq === "INVALID") malformed("FREQ is required");
  if (d.until !== undefined && d.count > 0) {
    malformed("UNTIL and COUNT are mutually exclusive");
  }
  if (d.byYearDay.length > 0 && d.freq !== "YEARLY") {
    malformed(`BYYEARDAY requires FREQ=YEARLY (RFC 5545 §3.3.10), got FREQ=${d.freq}`);
  }
  if (d.byWeekNo.length > 0 && d.freq !== "YEARLY") {
    malformed(`BYWEEKNO requires FREQ=YEARLY (RFC 5545 §3.3.10), got FREQ=${d.freq}`);
  }
  if (d.bySetPos.length > 0 && !hasOtherBy(d)) {
    malformed("BYSETPOS requires at least one other BY-* rule-part (RFC 5545 §3.3.10)");
  }
}

/**
 * Whether any BY-* clause other than BYSETPOS is present — the
 * precondition RFC 5545 §3.3.10 puts on BYSETPOS ("MUST only be used
 * in conjunction with another BYxxx rule part").
 */
function hasOtherBy(d: Draft): boolean {
  return (
    d.byDay.length > 0 ||
    d.byMonth.length > 0 ||
    d.byMonthDay.length > 0 ||
    d.byHour.length > 0 ||
    d.byMinute.length > 0 ||
    d.bySecond.length > 0 ||
    d.byYearDay.length > 0 ||
    d.byWeekNo.length > 0
  );
}
