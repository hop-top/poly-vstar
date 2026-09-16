// SPDX-License-Identifier: MIT

// RFC 5545 §3.3.5 DATE-TIME: form #2 (`YYYYMMDDTHHMMSSZ`, UTC) on the
// wire, and form #1 (`YYYYMMDDTHHMMSS`, local) resolved against a
// VTIMEZONE carried by the document itself.
//
// Everything here works on {@link Instant} — epoch milliseconds — with
// its own formatter and parser. `Date#toISOString()` is deliberately
// unused: it emits the extended form with fractional seconds, throws on
// out-of-range values, and switches to expanded-year notation outside
// 1000–9999. Each of those produces silently divergent canonical bytes.
//
// There is no IANA timezone database here, and there must never be one.
// A local time resolves only against the VTIMEZONE definitions inside
// the calendar being processed, so a self-contained document hashes the
// same on every machine, in every year, whatever tzdata is installed.

import type { Instant } from "./date.js";
import type { Calendar, Component } from "./types.js";
import { filter, get, remove, set } from "./types.js";

/** Form #2 is exactly 16 octets: 8 date + `T` + 6 time + `Z`. */
const FORM_TWO_OCTETS = 16;

/** Form #1 is exactly 15 octets: 8 date + `T` + 6 time, no `Z`. */
const FORM_ONE_OCTETS = 15;

const MS_PER_SECOND = 1000;

/**
 * Render `t` as an RFC 5545 §3.3.5 form #2 string —
 * `YYYYMMDDTHHMMSSZ` in UTC.
 *
 * The zero instant renders as the empty string, which is how the
 * property writers spell "clear the property".
 *
 * Sub-second precision is truncated toward negative infinity: form #2
 * carries only second resolution, and rounding would move an instant
 * across a second boundary.
 */
export function formatTime(t: Instant): string {
  if (t === 0) return "";
  const seconds = Math.floor(t / MS_PER_SECOND);
  const d = new Date(seconds * MS_PER_SECOND);
  return (
    pad(d.getUTCFullYear(), 4) +
    pad(d.getUTCMonth() + 1, 2) +
    pad(d.getUTCDate(), 2) +
    "T" +
    pad(d.getUTCHours(), 2) +
    pad(d.getUTCMinutes(), 2) +
    pad(d.getUTCSeconds(), 2) +
    "Z"
  );
}

function pad(n: number, width: number): string {
  return String(n).padStart(width, "0");
}

/**
 * Parse an RFC 5545 §3.3.5 form #2 string (`YYYYMMDDTHHMMSSZ`) into an
 * {@link Instant}. Returns `undefined` for any other input shape —
 * strict by design.
 *
 * Rejected: form #1 (no zone, and unsupported bare), RFC 3339 /
 * ISO 8601 extended layouts, date-only values, a lowercase `z`, leading
 * or trailing whitespace, any length other than sixteen octets, and
 * impossible calendar dates — February 30th does not roll into March.
 */
export function parseTime(s: string): Instant | undefined {
  if (s.length !== FORM_TWO_OCTETS) return undefined;
  if (s[15] !== "Z") return undefined;
  const fields = parseWallFields(s.slice(0, FORM_ONE_OCTETS));
  if (fields === undefined) return undefined;
  return wallToEpoch(fields);
}

/** The decomposed wall-clock fields of a form #1 value. */
interface Wall {
  readonly year: number;
  readonly month: number;
  readonly day: number;
  readonly hour: number;
  readonly minute: number;
  readonly second: number;
}

/**
 * Decode an RFC 5545 form #1 wire string into its wall-clock fields,
 * validating every field's range and the month's real length. Returns
 * `undefined` on any shape or range violation.
 */
function parseWallFields(s: string): Wall | undefined {
  if (s.length !== FORM_ONE_OCTETS) return undefined;
  if (s[8] !== "T") return undefined;
  if (!/^\d{8}T\d{6}$/.test(s)) return undefined;
  const year = Number(s.slice(0, 4));
  const month = Number(s.slice(4, 6));
  const day = Number(s.slice(6, 8));
  const hour = Number(s.slice(9, 11));
  const minute = Number(s.slice(11, 13));
  const second = Number(s.slice(13, 15));
  if (month < 1 || month > 12) return undefined;
  if (day < 1 || day > daysInMonth(year, month)) return undefined;
  if (hour > 23 || minute > 59 || second > 59) return undefined;
  return { year, month, day, hour, minute, second };
}

/** Days in `month` (1-based) of `year`, honouring the Gregorian leap rule. */
function daysInMonth(year: number, month: number): number {
  if (month === 2) {
    const leap = (year % 4 === 0 && year % 100 !== 0) || year % 400 === 0;
    return leap ? 29 : 28;
  }
  return month === 4 || month === 6 || month === 9 || month === 11 ? 30 : 31;
}

/**
 * The epoch-millisecond value of a wall-clock field set read as UTC.
 *
 * `Date.UTC` maps years 0–99 onto 1900–1999; the explicit
 * `setUTCFullYear` undoes that, so a year-42 timestamp stays in year 42.
 */
function wallToEpoch(w: Wall): Instant {
  const ms = Date.UTC(w.year, w.month - 1, w.day, w.hour, w.minute, w.second);
  if (w.year >= 0 && w.year <= 99) {
    const d = new Date(ms);
    d.setUTCFullYear(w.year);
    return d.getTime();
  }
  return ms;
}

/**
 * Parse an RFC 5545 §3.3.5 form #1 value as a wall-clock time in the
 * zone `tzid` names, where the zone is reconstructed from a VTIMEZONE
 * component inside `cal`.
 *
 * Returns `undefined` — not an error — when:
 *
 * - `tzid` is empty;
 * - `cal` carries no VTIMEZONE whose TZID matches (comparison is
 *   case-sensitive: TZIDs are opaque identifiers per RFC 5545 §3.2.19);
 * - the matching VTIMEZONE falls outside the
 *   {@link https://github.com/hop-top/poly-vstar spec's VTIMEZONE subset} —
 *   multiple STANDARD or DAYLIGHT children, a missing offset or
 *   DTSTART, or an RRULE the subset does not accept;
 * - `s` is not form #1 (15 octets, `T` at index 8, no `Z` suffix).
 *
 * Form #2 input is rejected even with a TZID present: the value is
 * already absolute, and resolving it against a zone would apply an
 * offset twice. Rule 5 falls back to verbatim emit in that case.
 *
 * Resolution failure is a normal outcome, not an error. The canonical
 * layer passes the value and its TZID parameter through unchanged.
 */
export function parseTimeWithTzid(s: string, tzid: string, cal: Calendar): Instant | undefined {
  if (tzid === "") return undefined;
  const wall = parseFormOne(s);
  if (wall === undefined) return undefined;
  const tz = findVTimezone(tzid, cal);
  if (tz === undefined) return undefined;
  const rules = loadTzRules(tz);
  if (rules === undefined) return undefined;
  const offsetSeconds = selectActiveOffset(wall, rules);
  return wallToEpoch(wall) - offsetSeconds * MS_PER_SECOND;
}

/**
 * Decode `s` as form #1, rejecting a `Z` suffix explicitly so a form #2
 * value never reaches the zone arithmetic.
 */
function parseFormOne(s: string): Wall | undefined {
  if (s.length !== FORM_ONE_OCTETS) return undefined;
  const last = s[s.length - 1];
  if (last === "Z" || last === "z") return undefined;
  return parseWallFields(s);
}

/** The subset of a STANDARD/DAYLIGHT child needed to place a transition. */
interface TzRule {
  /** TZOFFSETTO in seconds east of UTC. */
  readonly offsetTo: number;
  /** Whether the child carried an accepted `FREQ=YEARLY` RRULE. */
  readonly yearly: boolean;
  /** BYMONTH (1–12); zero when the child carried no RRULE. */
  readonly month: number;
  /** BYDAY weekday, `SU = 0` per RFC 5545 — not ISO-8601's `MO = 1`. */
  readonly weekday: number;
  /** BYDAY ordinal: positive counts from the start, negative from the end. */
  readonly week: number;
  readonly hour: number;
  readonly minute: number;
  readonly second: number;
}

/** The STANDARD plus optional DAYLIGHT pair extracted from a VTIMEZONE. */
interface TzRuleSet {
  readonly std: TzRule;
  readonly dst: TzRule | undefined;
}

/** The VTIMEZONE in `cal` whose TZID property equals `tzid`. */
function findVTimezone(tzid: string, cal: Calendar): Component | undefined {
  return filter(cal, "VTIMEZONE").find((c) => get(c, "TZID")?.value === tzid);
}

/** Sub-components of `c` whose type matches `name` exactly (uppercase per RFC). */
function subsByType(c: Component, name: string): Component[] {
  return c.sub.filter((s) => String(s.type) === name);
}

/**
 * Extract the rule pair from a VTIMEZONE, or `undefined` for any shape
 * outside the spec's VTIMEZONE subset.
 *
 * Accepted: a single STANDARD (fixed offset), a single STANDARD plus a
 * single DAYLIGHT where both carry an accepted `FREQ=YEARLY` rule, and
 * a DAYLIGHT alone (treated as a fixed offset, since there is no
 * transition to compute).
 */
function loadTzRules(tz: Component): TzRuleSet | undefined {
  const standards = subsByType(tz, "STANDARD");
  const daylights = subsByType(tz, "DAYLIGHT");

  if (standards.length === 0 && daylights.length === 0) return undefined;
  // Split-zone histories are outside the subset.
  if (standards.length > 1 || daylights.length > 1) return undefined;

  if (standards.length === 0) {
    const only = parseTzRule(daylights[0] as Component);
    return only === undefined ? undefined : { std: only, dst: undefined };
  }

  const std = parseTzRule(standards[0] as Component);
  if (std === undefined) return undefined;
  if (daylights.length === 0) return { std, dst: undefined };

  const dst = parseTzRule(daylights[0] as Component);
  if (dst === undefined) return undefined;
  // With both children present the subset requires a yearly rule on each.
  if (!std.yearly || !dst.yearly) return undefined;
  return { std, dst };
}

/**
 * The offset in seconds east of UTC that applies to `wall`.
 *
 * With no DAYLIGHT child the STANDARD offset applies unconditionally.
 * Otherwise both transitions are placed in `wall`'s own year and
 * compared on the same naive timeline — which orders two wall events
 * within one year correctly, and is all the comparison needs.
 */
function selectActiveOffset(wall: Wall, rs: TzRuleSet): number {
  const dst = rs.dst;
  if (dst === undefined) return rs.std.offsetTo;
  const at = wallToEpoch(wall);
  const dstStart = transitionAt(wall.year, dst);
  const stdStart = transitionAt(wall.year, rs.std);
  return at >= dstStart && at < stdStart ? dst.offsetTo : rs.std.offsetTo;
}

/**
 * The naive instant at which `r` becomes active in `year`, expressed on
 * the UTC timeline. The absolute value is meaningless; only the
 * ordering of two such instants from the same year is used.
 */
function transitionAt(year: number, r: TzRule): number {
  const day = nthWeekdayOfMonth(year, r.month, r.weekday, r.week);
  return wallToEpoch({
    year,
    month: r.month,
    day,
    hour: r.hour,
    minute: r.minute,
    second: r.second,
  });
}

/**
 * The day-of-month of the `n`th `weekday` in `(year, month)`. Positive
 * `n` counts from the start (1 = first); negative counts from the end
 * (-1 = last). `n = 0` is rejected at parse time.
 */
function nthWeekdayOfMonth(year: number, month: number, weekday: number, n: number): number {
  if (n > 0) {
    const firstWeekday = new Date(Date.UTC(year, month - 1, 1)).getUTCDay();
    const offset = (weekday - firstWeekday + 7) % 7;
    return 1 + offset + (n - 1) * 7;
  }
  const lastDay = daysInMonth(year, month);
  const lastWeekday = new Date(Date.UTC(year, month - 1, lastDay)).getUTCDay();
  const offset = (lastWeekday - weekday + 7) % 7;
  return lastDay - offset + (n + 1) * 7;
}

/**
 * Read one STANDARD/DAYLIGHT child into a {@link TzRule}, or
 * `undefined` for any field shape outside the subset.
 */
function parseTzRule(c: Component): TzRule | undefined {
  const offsetTo = readOffset(c, "TZOFFSETTO");
  if (offsetTo === undefined) return undefined;
  // TZOFFSETFROM is not used in the arithmetic, but the subset requires
  // it to be present and well-formed — a child missing it is a producer
  // shape this implementation declines to guess at.
  if (readOffset(c, "TZOFFSETFROM") === undefined) return undefined;

  const dtstart = get(c, "DTSTART");
  if (dtstart === undefined) return undefined;
  const wall = parseFormOne(dtstart.value);
  if (wall === undefined) return undefined;

  const rrule = get(c, "RRULE");
  if (rrule === undefined) {
    return {
      offsetTo,
      yearly: false,
      month: 0,
      weekday: 0,
      week: 0,
      hour: wall.hour,
      minute: wall.minute,
      second: wall.second,
    };
  }
  const yearly = parseYearlyRrule(rrule.value);
  if (yearly === undefined) return undefined;
  return {
    offsetTo,
    yearly: true,
    month: yearly.month,
    weekday: yearly.weekday,
    week: yearly.week,
    hour: wall.hour,
    minute: wall.minute,
    second: wall.second,
  };
}

/**
 * Read a `±HHMM` or `±HHMMSS` UTC-offset property as seconds east of
 * UTC. Returns `undefined` on any shape mismatch.
 */
function readOffset(c: Component, name: string): number | undefined {
  const v = c.props.find((p) => p.name.toUpperCase() === name)?.value;
  if (v === undefined) return undefined;
  const m = /^([+-])(\d{2})(\d{2})(\d{2})?$/.exec(v);
  if (m === null) return undefined;
  const sign = m[1] === "-" ? -1 : 1;
  const hh = Number(m[2]);
  const mm = Number(m[3]);
  const ss = m[4] === undefined ? 0 : Number(m[4]);
  return sign * (hh * 3600 + mm * 60 + ss);
}

/** The BYMONTH / BYDAY pair an accepted VTIMEZONE RRULE carries. */
interface YearlyRule {
  readonly month: number;
  readonly weekday: number;
  readonly week: number;
}

/**
 * Accept only the VTIMEZONE RRULE subset: `FREQ=YEARLY` with optional
 * `BYMONTH`, an ordinal `BYDAY`, and a no-op `INTERVAL=1`.
 *
 * Everything else fails closed — UNTIL, COUNT, BYWEEKNO, BYSETPOS,
 * WKST, an unknown key, a `BYDAY` without an ordinal — because applying
 * a partial rule would produce a plausible instant that is wrong.
 *
 * This is deliberately narrow and unrelated to the generic RRULE
 * parsing scope, which lands with the recurrence layer.
 */
function parseYearlyRrule(s: string): YearlyRule | undefined {
  let freqSeen = false;
  let month = 0;
  let weekday = 0;
  let week = 0;

  for (const part of s.split(";")) {
    const eq = part.indexOf("=");
    if (eq < 0) return undefined;
    const key = part.slice(0, eq).toUpperCase();
    const val = part.slice(eq + 1);
    switch (key) {
      case "FREQ":
        if (val.toUpperCase() !== "YEARLY") return undefined;
        freqSeen = true;
        break;
      case "BYMONTH": {
        if (!/^\d+$/.test(val)) return undefined;
        const m = Number(val);
        if (m < 1 || m > 12) return undefined;
        month = m;
        break;
      }
      case "BYDAY": {
        const byday = parseByday(val);
        if (byday === undefined) return undefined;
        week = byday.week;
        weekday = byday.weekday;
        break;
      }
      case "INTERVAL":
        // A no-op INTERVAL=1 is accepted; any other value is rejected.
        if (val !== "1") return undefined;
        break;
      default:
        return undefined;
    }
  }
  return freqSeen ? { month, weekday, week } : undefined;
}

/** Weekday codes, `SU = 0` per RFC 5545 §3.3.10. */
const WEEKDAYS: Readonly<Record<string, number>> = {
  SU: 0,
  MO: 1,
  TU: 2,
  WE: 3,
  TH: 4,
  FR: 5,
  SA: 6,
};

/**
 * Parse one BYDAY entry such as `2SU` or `-1SU`. The ordinal-less forms
 * (`SU`, `0SU`) are rejected: a VTIMEZONE transition needs an explicit
 * nth.
 */
function parseByday(s: string): { week: number; weekday: number } | undefined {
  const m = /^([+-]?\d+)(SU|MO|TU|WE|TH|FR|SA)$/i.exec(s);
  if (m === null) return undefined;
  const week = Number(m[1]);
  if (week === 0) return undefined;
  const weekday = WEEKDAYS[(m[2] as string).toUpperCase()];
  return weekday === undefined ? undefined : { week, weekday };
}

/**
 * Resolve a datetime-bearing property to an {@link Instant}, honouring
 * a `TZID` parameter against `cal`'s VTIMEZONE registry.
 *
 * Returns `undefined` for a missing property, a `VALUE=DATE` property
 * (a calendar date is not an instant — read it with the date-typed
 * accessors), or a value neither form #2 nor a resolvable form #1.
 */
function datetimeProp(c: Component, name: string, cal: Calendar): Instant | undefined {
  const p = get(c, name);
  if (p === undefined) return undefined;
  if (p.params.some((x) => x.name.toUpperCase() === "VALUE" && x.value.toUpperCase() === "DATE")) {
    return undefined;
  }
  const direct = parseTime(p.value);
  if (direct !== undefined) return direct;
  const tzid = p.params.find((x) => x.name.toUpperCase() === "TZID")?.value;
  return tzid === undefined ? undefined : parseTimeWithTzid(p.value, tzid, cal);
}

/**
 * The DTSTART value as an instant.
 *
 * The calendar argument is not optional plumbing: resolving a
 * `TZID`-bearing local time needs the enclosing calendar's VTIMEZONE
 * registry, which lives on the Calendar and not on the Component.
 */
export function dtstart(c: Component, cal: Calendar): Instant | undefined {
  return datetimeProp(c, "DTSTART", cal);
}

/** The DTEND value as an instant. */
export function dtend(c: Component, cal: Calendar): Instant | undefined {
  return datetimeProp(c, "DTEND", cal);
}

/** The VTODO DUE value as an instant. */
export function due(c: Component, cal: Calendar): Instant | undefined {
  return datetimeProp(c, "DUE", cal);
}

/** The COMPLETED value as an instant. */
export function completed(c: Component, cal: Calendar): Instant | undefined {
  return datetimeProp(c, "COMPLETED", cal);
}

/**
 * The DTSTAMP value as an instant.
 *
 * No calendar argument: RFC 5545 §3.8.7.2 requires DTSTAMP to be UTC,
 * so there is never a zone to resolve.
 */
export function dtstamp(c: Component): Instant | undefined {
  return parseTime(get(c, "DTSTAMP")?.value ?? "");
}

/**
 * Write `t` as the named property in UTC form #2, or remove the
 * property entirely when `t` is the zero instant.
 *
 * The written property carries no parameters. Dropping any it had is
 * required, not merely tidy: a stale `TZID` on a value now spelled in
 * UTC would contradict the value, and a stale parameter set would make
 * the canonical bytes depend on the property's edit history.
 */
function setOrClearTime(c: Component, name: string, t: Instant): void {
  const value = formatTime(t);
  if (value === "") {
    remove(c, name);
    return;
  }
  set(c, { name, params: [], value });
}

/** Write DTSTART as a UTC form #2 instant. */
export function setDtstart(c: Component, t: Instant): void {
  setOrClearTime(c, "DTSTART", t);
}

/** Write DTEND as a UTC form #2 instant. */
export function setDtend(c: Component, t: Instant): void {
  setOrClearTime(c, "DTEND", t);
}

/** Write DUE as a UTC form #2 instant. */
export function setDue(c: Component, t: Instant): void {
  setOrClearTime(c, "DUE", t);
}

/** Write COMPLETED as a UTC form #2 instant. */
export function setCompleted(c: Component, t: Instant): void {
  setOrClearTime(c, "COMPLETED", t);
}
