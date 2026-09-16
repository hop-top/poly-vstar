// SPDX-License-Identifier: MIT

// The forward evaluator: one FREQ period expanded into concrete
// occurrences via the BY-* filters, then stepped.
//
// Every calculation here is pure UTC epoch-millisecond arithmetic on
// {@link Instant}. There is no IANA timezone database and there must
// never be one — no `Intl`, no zone lookups. A V* implementation
// resolves local times only against the VTIMEZONE definitions inside
// the document it is processing, which happens a layer below this one;
// by the time a rule reaches the evaluator every instant is absolute.
//
// Weekday numbering is `SU = 0` per RFC 5545 §3.3.10, which is what
// `Date#getUTCDay()` returns — so the numbering matches and no
// conversion appears at the arithmetic sites. ISO-8601's `MO = 1` is
// reached only through `toIsoWeekday`, at the API boundary.

import type { Instant } from "../date.js";
import type { ByDay, RuleFields } from "./types.js";
import { weekdayNumber } from "./types.js";

const MS_PER_SECOND = 1000;
const MS_PER_MINUTE = 60 * MS_PER_SECOND;
const MS_PER_HOUR = 60 * MS_PER_MINUTE;
const MS_PER_DAY = 24 * MS_PER_HOUR;

/**
 * The evaluator's iteration bound: the number of consecutive empty
 * FREQ periods walked before the search is abandoned.
 *
 * 100000 is large enough for every realistic recurrence — a yearly rule
 * steps once per year, so the bound covers ~100k years of those and
 * ~273 years of daily ones — and small enough to fail fast on a rule
 * that never yields, such as `FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30`.
 *
 * The bound is a starvation guard, not a total-occurrence limit: the
 * counter resets whenever a period produces an occurrence, so a rule
 * that fires regularly runs as long as the caller wants.
 *
 * It counts FREQ periods, so under `FREQ=MINUTELY` at `INTERVAL=1` it
 * spans about 69 days: a satisfiable rule whose limits admit nothing
 * for longer (`FREQ=MINUTELY;BYMONTH=1` evaluated from February)
 * reports `ErrIterationCap` rather than the eventual occurrence.
 * Skipping the base to the next window a limit admits would lift that,
 * and is a follow-up for every FREQ, not a MINUTELY concern.
 *
 * It is exported so a caller that catches `ErrIterationCap` can state
 * the bound that was hit, and deliberately not configurable: the only
 * inputs that reach it are unsatisfiable rules, for which a larger
 * budget merely costs more before failing identically.
 */
export const MAX_ITERATIONS = 100000;

/** The wall-clock fields of an instant, read as UTC. */
interface Wall {
  year: number;
  /** 1-based, per the API mapping — never a zero-based month index. */
  month: number;
  day: number;
  hour: number;
  minute: number;
  second: number;
}

/** Decompose an instant into its UTC wall-clock fields. */
export function toWall(t: Instant): Wall {
  const d = new Date(t);
  return {
    year: d.getUTCFullYear(),
    month: d.getUTCMonth() + 1,
    day: d.getUTCDate(),
    hour: d.getUTCHours(),
    minute: d.getUTCMinutes(),
    second: d.getUTCSeconds(),
  };
}

/**
 * Compose UTC wall-clock fields back into an instant.
 *
 * `Date.UTC` maps years 0–99 onto 1900–1999; the explicit
 * `setUTCFullYear` undoes that, so a year-42 rule stays in year 42.
 */
export function fromWall(w: Wall): Instant {
  const ms = Date.UTC(w.year, w.month - 1, w.day, w.hour, w.minute, w.second);
  if (w.year >= 0 && w.year <= 99) {
    const d = new Date(ms);
    d.setUTCFullYear(w.year);
    return d.getTime();
  }
  return ms;
}

/** The RFC 5545 weekday number (`SU = 0`) of an instant. */
function weekdayOf(t: Instant): number {
  return new Date(t).getUTCDay();
}

/** Days in a 1-based `month` of `year`, honouring the Gregorian leap rule. */
function daysInMonth(year: number, month: number): number {
  if (month === 2) {
    const leap = (year % 4 === 0 && year % 100 !== 0) || year % 400 === 0;
    return leap ? 29 : 28;
  }
  return month === 4 || month === 6 || month === 9 || month === 11 ? 30 : 31;
}

/** 365 or 366, for the given Gregorian year. */
function daysInYear(year: number): number {
  return (Date.UTC(year + 1, 0, 1) - Date.UTC(year, 0, 1)) / MS_PER_DAY;
}

/** Midnight UTC on the 1-based `doy`th day of `year`. */
function dayOfYearToInstant(year: number, doy: number): Instant {
  return Date.UTC(year, 0, 1) + (doy - 1) * MS_PER_DAY;
}

/** Midnight UTC on the date `t` falls on. */
function startOfDay(t: Instant): Instant {
  const w = toWall(t);
  return fromWall({ year: w.year, month: w.month, day: w.day, hour: 0, minute: 0, second: 0 });
}

/**
 * The start of the week containing `t`, anchored on `wkst`.
 *
 * WKST-awareness lives here and nowhere else: every week boundary the
 * evaluator draws — weekly expansion, BYWEEKNO numbering — comes
 * through this function, so a rule's week start is honoured uniformly.
 */
function startOfWeek(t: Instant, wkst: number): Instant {
  const delta = (weekdayOf(t) - wkst + 7) % 7;
  return startOfDay(t) - delta * MS_PER_DAY;
}

/**
 * The WKST-anchored start of week `n` of `year`.
 *
 * Week 1 is the week containing January 4th — the ISO 8601 rule,
 * generalized to an arbitrary week start.
 */
function startOfWeekN(year: number, n: number, wkst: number): Instant {
  return startOfWeek(Date.UTC(year, 0, 4), wkst) + 7 * (n - 1) * MS_PER_DAY;
}

/**
 * The number of WKST-anchored weeks in `year` under the same "week 1
 * contains January 4th" rule.
 *
 * Most years have 52; a 53rd exists when the trailing days of December
 * still belong to a week of the current year rather than to week 1 of
 * the next.
 */
function weeksInYear(year: number, wkst: number): number {
  const first = startOfWeek(Date.UTC(year, 0, 4), wkst);
  const next = startOfWeek(Date.UTC(year + 1, 0, 4), wkst);
  return Math.round((next - first) / MS_PER_DAY / 7);
}

// ── Period stepping ───────────────────────────────────────────────

/**
 * Step `current` forward by one `FREQ × INTERVAL` period, or return
 * `undefined` when the frequency is not one this evaluator steps.
 *
 * That `undefined` is a *termination* signal — there are no more
 * periods — and is deliberately distinct from exhausting
 * {@link MAX_ITERATIONS}, which means periods remained but the budget
 * did not and reports `ErrIterationCap`.
 */
export function advance(rule: RuleFields, current: Instant): Instant | undefined {
  const w = toWall(current);
  switch (rule.freq) {
    case "MINUTELY":
      return current + rule.interval * MS_PER_MINUTE;
    case "HOURLY":
      return current + rule.interval * MS_PER_HOUR;
    case "DAILY":
      return current + rule.interval * MS_PER_DAY;
    case "WEEKLY":
      return current + 7 * rule.interval * MS_PER_DAY;
    case "MONTHLY":
      // Land on day 1 of the target month rather than adding months to
      // the current day: adding a month to January 31st would overflow
      // into March, and the expansion below reconstructs the real day
      // from BYMONTHDAY or from dtstart anyway.
      return fromWall({ ...w, day: 1, month: w.month + rule.interval });
    case "YEARLY":
      // Same reasoning as MONTHLY, one level up: landing on January 1st
      // stops a February 29th rule sliding into March on a non-leap
      // year, because expandYearly takes the month from dtstart.
      return fromWall({ ...w, year: w.year + rule.interval, month: 1, day: 1 });
    case "INVALID":
      return undefined;
  }
}

// ── BY-* matching ─────────────────────────────────────────────────

/** Whether `t`'s month satisfies BYMONTH (vacuously true when unset). */
function matchesByMonth(rule: RuleFields, t: Instant): boolean {
  if (rule.byMonth.length === 0) return true;
  return rule.byMonth.includes(toWall(t).month);
}

/**
 * Whether `t`'s hour satisfies BYHOUR (vacuously true when unset).
 *
 * The same shape as {@link matchesByMonth}; used where the RFC 5545
 * §3.3.10 table says BYHOUR limits (HOURLY and finer).
 */
function matchesByHour(rule: RuleFields, t: Instant): boolean {
  if (rule.byHour.length === 0) return true;
  return rule.byHour.includes(toWall(t).hour);
}

/**
 * Whether `t`'s minute satisfies BYMINUTE (vacuously true when unset).
 *
 * The BYMINUTE twin of {@link matchesByHour}; used where the table says
 * BYMINUTE limits (MINUTELY).
 */
function matchesByMinute(rule: RuleFields, t: Instant): boolean {
  if (rule.byMinute.length === 0) return true;
  return rule.byMinute.includes(toWall(t).minute);
}

/**
 * Whether `t`'s day satisfies BYMONTHDAY, resolving negative entries
 * against the real length of `t`'s own month — so `-1` is the last day
 * of whichever month this is, 28 or 31.
 */
function matchesByMonthDay(rule: RuleFields, t: Instant): boolean {
  if (rule.byMonthDay.length === 0) return true;
  const { year, month, day } = toWall(t);
  const dim = daysInMonth(year, month);
  return rule.byMonthDay.some((md) => (md > 0 ? md === day : dim + md + 1 === day));
}

/**
 * Whether `t`'s weekday appears in BYDAY, ignoring ordinals.
 *
 * Ordinals are meaningful only inside a MONTHLY or YEARLY period, where
 * {@link matchesByDayOrdinal} honours them.
 */
function matchesByDay(rule: RuleFields, t: Instant): boolean {
  if (rule.byDay.length === 0) return true;
  const wd = weekdayOf(t);
  return rule.byDay.some((bd) => weekdayNumber(bd.weekday) === wd);
}

/**
 * The BYDAY check for MONTHLY and YEARLY periods, where `2MO` means
 * "the second Monday of this month" and `-1FR` "the last Friday".
 */
function matchesByDayOrdinal(rule: RuleFields, t: Instant): boolean {
  if (rule.byDay.length === 0) return true;
  const wd = weekdayOf(t);
  const { year, month, day } = toWall(t);
  const dim = daysInMonth(year, month);
  return rule.byDay.some((bd) => matchesOneByDay(bd, wd, day, year, month, dim));
}

/** Whether one BYDAY entry matches a day of the month. */
function matchesOneByDay(
  bd: ByDay,
  weekday: number,
  day: number,
  year: number,
  month: number,
  dim: number,
): boolean {
  if (weekdayNumber(bd.weekday) !== weekday) return false;
  if (bd.ordinal === 0) return true;
  if (bd.ordinal > 0) {
    // Days 1–7 hold the first of each weekday, 8–14 the second, …
    return bd.ordinal === Math.floor((day - 1) / 7) + 1;
  }
  const last = lastWeekdayOfMonth(year, month, dim, weekday);
  return bd.ordinal === -(Math.floor((last - day) / 7) + 1);
}

/** The day-of-month of the last `weekday` in the given month. */
function lastWeekdayOfMonth(year: number, month: number, dim: number, weekday: number): number {
  const lastWd = weekdayOf(Date.UTC(year, month - 1, dim));
  return dim - ((lastWd - weekday + 7) % 7);
}

// ── Period expansion ──────────────────────────────────────────────

/** A BY-* list if non-empty, else a single-element list of the default. */
function byOrDefault(list: readonly number[], fallback: number): readonly number[] {
  return list.length === 0 ? [fallback] : list;
}

/**
 * The cartesian product of BYHOUR × BYMINUTE × BYSECOND on the date of
 * `day`. A time field with no BY-* clause is taken from `dtstart`,
 * which is the RFC's anchor for everything a rule does not constrain.
 */
function crossTimeOfDay(rule: RuleFields, day: Instant, dtstart: Instant): Instant[] {
  const d = toWall(day);
  const s = toWall(dtstart);
  const out: Instant[] = [];
  for (const hour of byOrDefault(rule.byHour, s.hour)) {
    for (const minute of byOrDefault(rule.byMinute, s.minute)) {
      for (const second of byOrDefault(rule.bySecond, s.second)) {
        out.push(fromWall({ year: d.year, month: d.month, day: d.day, hour, minute, second }));
      }
    }
  }
  return out;
}

/**
 * The days-of-month to consider in a month of `dim` days.
 *
 * With BYMONTHDAY, each entry resolved (negative from the end). With
 * BYDAY but no BYMONTHDAY, every day — the BYDAY filter narrows them.
 * Otherwise the single day dtstart falls on, which is how an
 * unqualified monthly rule fires. A dtstart day past the end of a
 * shorter month yields nothing, which is RFC 5545's explicit
 * skip-don't-clamp behaviour: a January 31st monthly rule has no
 * February occurrence rather than a February 28th one.
 */
function monthDayCandidates(rule: RuleFields, dim: number, dtstartDay: number): number[] {
  if (rule.byMonthDay.length > 0) {
    return rule.byMonthDay.map((md) => (md > 0 ? md : dim + md + 1));
  }
  if (rule.byDay.length > 0) {
    return Array.from({ length: dim }, (_, i) => i + 1);
  }
  return dtstartDay > dim ? [] : [dtstartDay];
}

/** Every occurrence in the days of one month that pass the filters. */
function expandMonthDays(rule: RuleFields, year: number, month: number, dtstart: Instant): Instant[] {
  const dim = daysInMonth(year, month);
  const out: Instant[] = [];
  for (const d of monthDayCandidates(rule, dim, toWall(dtstart).day)) {
    // An out-of-range candidate is dropped, not clamped: BYMONTHDAY=29
    // simply has no occurrence in a non-leap February.
    if (d < 1 || d > dim) continue;
    const day = fromWall({ year, month, day: d, hour: 0, minute: 0, second: 0 });
    if (!matchesByDayOrdinal(rule, day)) continue;
    out.push(...crossTimeOfDay(rule, day, dtstart));
  }
  return out;
}

/**
 * One base minute: BYMONTH/BYMONTHDAY/BYDAY/BYHOUR/BYMINUTE limit;
 * BYSECOND expand (RFC 5545 §3.3.10 table, MINUTELY column). A base
 * that fails any limit yields nothing; one that passes emits one
 * occurrence per BYSECOND entry (the base's second when unset) on the
 * base's date, hour and minute — the {@link expandHourly} shape.
 *
 * No `dtstart` parameter: the base carries dtstart's second through
 * the minute walk, and an {@link Instant} has no sub-second field for
 * dtstart to anchor.
 */
function expandMinutely(rule: RuleFields, base: Instant): Instant[] {
  if (
    !matchesByMonth(rule, base) ||
    !matchesByMonthDay(rule, base) ||
    !matchesByDay(rule, base) ||
    !matchesByHour(rule, base) ||
    !matchesByMinute(rule, base)
  ) {
    return [];
  }
  const b = toWall(base);
  const out: Instant[] = [];
  for (const second of byOrDefault(rule.bySecond, b.second)) {
    out.push(fromWall({ year: b.year, month: b.month, day: b.day, hour: b.hour, minute: b.minute, second }));
  }
  return out;
}

/**
 * One base hour: BYMONTH/BYMONTHDAY/BYDAY/BYHOUR limit; BYMINUTE×BYSECOND
 * expand (RFC 5545 §3.3.10 table, HOURLY column). A base whose hour is
 * not in BYHOUR yields nothing; one that passes emits one occurrence
 * per BYMINUTE×BYSECOND pair on the base's date and hour.
 */
function expandHourly(rule: RuleFields, base: Instant, dtstart: Instant): Instant[] {
  if (
    !matchesByMonth(rule, base) ||
    !matchesByMonthDay(rule, base) ||
    !matchesByDay(rule, base) ||
    !matchesByHour(rule, base)
  ) {
    return [];
  }
  const b = toWall(base);
  const s = toWall(dtstart);
  const out: Instant[] = [];
  for (const minute of byOrDefault(rule.byMinute, b.minute)) {
    for (const second of byOrDefault(rule.bySecond, s.second)) {
      out.push(fromWall({ year: b.year, month: b.month, day: b.day, hour: b.hour, minute, second }));
    }
  }
  return out;
}

/** One base day, with the time-of-day clauses crossed over it. */
function expandDaily(rule: RuleFields, base: Instant, dtstart: Instant): Instant[] {
  if (!matchesByMonth(rule, base) || !matchesByMonthDay(rule, base) || !matchesByDay(rule, base)) {
    return [];
  }
  return crossTimeOfDay(rule, base, dtstart);
}

/**
 * With BYDAY, the seven days of the WKST-anchored week, filtered.
 * Without it, only dtstart's own weekday — which is the base itself,
 * since the weekly step preserves the weekday.
 */
function expandWeekly(rule: RuleFields, base: Instant, dtstart: Instant): Instant[] {
  if (rule.byDay.length === 0) {
    if (!matchesByMonth(rule, base) || !matchesByMonthDay(rule, base)) return [];
    return crossTimeOfDay(rule, base, dtstart);
  }
  const weekStart = startOfWeek(base, weekdayNumber(rule.weekStart));
  const out: Instant[] = [];
  for (let i = 0; i < 7; i++) {
    const day = weekStart + i * MS_PER_DAY;
    if (!matchesByDay(rule, day) || !matchesByMonth(rule, day) || !matchesByMonthDay(rule, day)) {
      continue;
    }
    out.push(...crossTimeOfDay(rule, day, dtstart));
  }
  return out;
}

/** Every matching day of the base month. */
function expandMonthly(rule: RuleFields, base: Instant, dtstart: Instant): Instant[] {
  if (!matchesByMonth(rule, base)) return [];
  const { year, month } = toWall(base);
  return expandMonthDays(rule, year, month, dtstart);
}

/**
 * A year, expanded by whichever clause governs it: BYYEARDAY picks
 * days of the year, BYWEEKNO picks whole weeks, and otherwise the
 * BYMONTH months — or dtstart's own month — expand as monthly ones.
 */
function expandYearly(rule: RuleFields, base: Instant, dtstart: Instant): Instant[] {
  const year = toWall(base).year;
  if (rule.byYearDay.length > 0) return expandByYearDay(rule, year, dtstart);
  if (rule.byWeekNo.length > 0) return expandByWeekNo(rule, year, dtstart);

  // The yearly step resets the base to January to dodge day overflow,
  // so the anchor month has to come from dtstart, not from the base.
  const months = rule.byMonth.length > 0 ? rule.byMonth : [toWall(dtstart).month];
  const out: Instant[] = [];
  for (const month of months) {
    if (month < 1 || month > 12) continue;
    out.push(...expandMonthDays(rule, year, month, dtstart));
  }
  return out;
}

/**
 * The BYYEARDAY days of a year, with BYMONTH and BYDAY narrowing them.
 * Negative entries count back from year-end, and an entry the year does
 * not have — day 366 of a non-leap year — is silently dropped.
 */
function expandByYearDay(rule: RuleFields, year: number, dtstart: Instant): Instant[] {
  const total = daysInYear(year);
  const out: Instant[] = [];
  for (const yd of rule.byYearDay) {
    const doy = yd > 0 ? yd : total + yd + 1;
    if (doy < 1 || doy > total) continue;
    const day = dayOfYearToInstant(year, doy);
    if (!matchesByMonth(rule, day) || !matchesByDay(rule, day)) continue;
    out.push(...crossTimeOfDay(rule, day, dtstart));
  }
  return out;
}

/**
 * All seven days of each BYWEEKNO week, with BYMONTH and BYDAY
 * narrowing them. Negative entries count weeks from year-end, and a
 * week the year does not have — 53 in a 52-week year — is dropped.
 */
function expandByWeekNo(rule: RuleFields, year: number, dtstart: Instant): Instant[] {
  const wkst = weekdayNumber(rule.weekStart);
  const total = weeksInYear(year, wkst);
  const out: Instant[] = [];
  for (const wn of rule.byWeekNo) {
    const n = wn > 0 ? wn : total + wn + 1;
    if (n < 1 || n > total) continue;
    const weekStart = startOfWeekN(year, n, wkst);
    for (let i = 0; i < 7; i++) {
      const day = weekStart + i * MS_PER_DAY;
      if (!matchesByMonth(rule, day) || !matchesByDay(rule, day)) continue;
      out.push(...crossTimeOfDay(rule, day, dtstart));
    }
  }
  return out;
}

/** Expand one FREQ period into its concrete occurrences, unsorted. */
function expand(rule: RuleFields, base: Instant, dtstart: Instant): Instant[] {
  switch (rule.freq) {
    case "MINUTELY":
      return expandMinutely(rule, base);
    case "HOURLY":
      return expandHourly(rule, base, dtstart);
    case "DAILY":
      return expandDaily(rule, base, dtstart);
    case "WEEKLY":
      return expandWeekly(rule, base, dtstart);
    case "MONTHLY":
      return expandMonthly(rule, base, dtstart);
    case "YEARLY":
      return expandYearly(rule, base, dtstart);
    case "INVALID":
      return [];
  }
}

/**
 * Filter a sorted occurrence list down to the 1-based positions
 * BYSETPOS names. Positive entries index from the start, negative from
 * the end (`-1` is the last). Out-of-range entries are dropped per RFC
 * 5545 §3.3.10, and the result is de-duplicated — two entries may
 * resolve to the same occurrence — and left in chronological order.
 */
function applyBySetPos(occs: readonly Instant[], setPos: readonly number[]): Instant[] {
  if (occs.length === 0) return [];
  const picked = new Set<number>();
  for (const p of setPos) {
    const idx = p > 0 ? p - 1 : occs.length + p;
    if (idx < 0 || idx >= occs.length) continue;
    picked.add(idx);
  }
  return occs.filter((_, i) => picked.has(i));
}

/**
 * One FREQ period, expanded, sorted, and positionally filtered.
 *
 * BYSETPOS is applied here, after every other BY-* clause and after the
 * sort, because RFC 5545 §3.3.10 defines it as a filter over the fully
 * expanded set of the period — `-1` means "the last of whatever the
 * other clauses produced", so the ordering of these three steps is the
 * semantics.
 */
export function periodOccurrences(rule: RuleFields, base: Instant, dtstart: Instant): Instant[] {
  const occs = expand(rule, base, dtstart).sort((a, b) => a - b);
  return rule.bySetPos.length > 0 ? applyBySetPos(occs, rule.bySetPos) : occs;
}
