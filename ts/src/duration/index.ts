// SPDX-License-Identifier: MIT

/**
 * The RFC 5545 §3.3.6 DURATION value type and the §3.8.6.3 TRIGGER
 * property on VALARM, including relative triggers and their RELATED
 * anchor.
 *
 * ## Why a class and not a millisecond count
 *
 * {@link VDuration} preserves the units the producer authored — weeks,
 * days, hours, minutes, seconds — rather than collapsing them. A plain
 * millisecond count cannot represent "one day" distinctly from "24
 * hours", but RFC 5545 draws that distinction deliberately: a calendar
 * day is 23, 24 or 25 hours across a UTC-offset transition.
 * Re-serializing a millisecond count would silently rewrite `P1D` as
 * `PT24H` — and canonical form preserves DURATION values verbatim
 * (spec rule 12), so that rewrite changes the hash.
 *
 * Both views are available: {@link VDuration.signed} reports the
 * nominal length in milliseconds (days as 24h, weeks as 7 days), and
 * {@link VDuration.addTo} anchors the value against a real instant,
 * advancing calendar days and weeks by date.
 *
 * The type is named `VDuration` rather than `Duration` because
 * `Temporal.Duration` is a near-universal built-in and the V* type is
 * not interchangeable with it.
 */

import { VstarError } from "../errors.js";
import { formatTime, parseTime } from "../time.js";
import { dtend, dtstart, due } from "../time.js";
import type { Instant } from "../date.js";
import type { Calendar, Component, Property } from "../types.js";
import { get } from "../types.js";

export { version } from "../version.js";

const MS_PER_SECOND = 1000;
const MS_PER_MINUTE = 60 * MS_PER_SECOND;
const MS_PER_HOUR = 60 * MS_PER_MINUTE;
const MS_PER_DAY = 24 * MS_PER_HOUR;

/** Property names read off components, spelled once. */
const PROP_TRIGGER = "TRIGGER";
const PROP_DURATION = "DURATION";
const PROP_REPEAT = "REPEAT";

/** RFC 5545 §3.2 parameter names and the VALUE arguments TRIGGER uses. */
const PARAM_RELATED = "RELATED";
const PARAM_VALUE = "VALUE";
const VALUE_DURATION = "DURATION";
const VALUE_DATE_TIME = "DATE-TIME";

/**
 * Which end of the parent component a relative TRIGGER is measured
 * from, per the RFC 5545 §3.2.14 RELATED parameter.
 *
 * The wire spelling is the value, so a `Related` stringifies to exactly
 * what the property carries.
 */
export type Related = "START" | "END";

/**
 * Anchor the trigger to the parent's start (DTSTART). This is the RFC
 * default when RELATED is absent.
 */
export const RELATED_START = "START" as const satisfies Related;

/**
 * Anchor the trigger to the parent's end — DTEND, else
 * DTSTART + DURATION, else a VTODO's DUE.
 */
export const RELATED_END = "END" as const satisfies Related;

/** The fields {@link VDuration} carries, before the behaviour. */
interface DurationFields {
  negative: boolean;
  weeks: number;
  days: number;
  hours: number;
  minutes: number;
  seconds: number;
  dayForm: boolean;
}

/**
 * An RFC 5545 §3.3.6 DURATION value in the units its producer authored.
 *
 * The grammar admits either a week form (weeks alone) or a
 * day-and-time form (days plus an optional hour/minute/second part);
 * the two never mix. The sign applies to the WHOLE duration, not to any
 * single field — that is what makes `-PT15M` mean "15 minutes before"
 * on a VALARM TRIGGER.
 *
 * A duration with no non-zero unit is a valid, positive, zero-length
 * value that renders as `PT0S`.
 */
export class VDuration {
  /** Whether the whole duration is subtractive. */
  readonly negative: boolean;
  /** The `nW` count. When non-zero every other unit field is zero. */
  readonly weeks: number;
  /** The `nD` count. */
  readonly days: number;
  /** The `nH` count from the time part. */
  readonly hours: number;
  /** The `nM` count from the time part. */
  readonly minutes: number;
  /** The `nS` count from the time part. */
  readonly seconds: number;
  /**
   * Whether the value was authored in the day form.
   *
   * With a zero day count (`P0D`) every unit field is zero, which is
   * indistinguishable from the zero duration — so this flag is what
   * lets {@link toString} reproduce `P0D` rather than the canonical
   * zero spelling `PT0S`. It is load-bearing precisely because
   * canonical form preserves DURATION verbatim.
   *
   * It affects formatting only: {@link signed}, {@link isNegative} and
   * {@link addTo} ignore it, and the two values are numerically equal.
   */
  readonly dayForm: boolean;

  constructor(fields: Partial<DurationFields> = {}) {
    this.negative = fields.negative ?? false;
    this.weeks = fields.weeks ?? 0;
    this.days = fields.days ?? 0;
    this.hours = fields.hours ?? 0;
    this.minutes = fields.minutes ?? 0;
    this.seconds = fields.seconds ?? 0;
    this.dayForm = fields.dayForm ?? false;
  }

  /** Whether every unit field is zero. */
  private get zeroLength(): boolean {
    return (
      this.weeks === 0 &&
      this.days === 0 &&
      this.hours === 0 &&
      this.minutes === 0 &&
      this.seconds === 0
    );
  }

  /**
   * Render as an RFC 5545 §3.3.6 DURATION value, preserving the units
   * the value carries.
   *
   * {@link parse} and this round-trip byte for byte, with one
   * intentional normalization: an explicit `+` is dropped, since a
   * positive duration is the default.
   */
  toString(): string {
    let out = this.negative && !this.zeroLength ? "-P" : "P";
    if (this.weeks !== 0) return `${out}${this.weeks}W`;

    // A wholly zero duration has no unit to render, so it takes the
    // canonical zero spelling rather than a bare `P`, which parse rejects.
    if (this.zeroLength && !this.dayForm) return `${out}T0S`;

    if (this.days !== 0 || this.dayForm) out += `${this.days}D`;
    const hasTime = this.hours !== 0 || this.minutes !== 0 || this.seconds !== 0;
    if (!hasTime) return out;

    out += "T";
    if (this.hours !== 0) out += `${this.hours}H`;
    if (this.minutes !== 0) out += `${this.minutes}M`;
    if (this.seconds !== 0) out += `${this.seconds}S`;
    return out;
  }

  /**
   * The nominal length in milliseconds, negated when the duration is
   * negative. Days count as 24 hours and weeks as 7 days.
   *
   * Exact for time-only values and for any anchor in a fixed-offset
   * zone, UTC included. Use {@link addTo} when a real anchor is
   * available and a day value might cross a transition.
   */
  signed(): number {
    const total =
      this.weeks * 7 * MS_PER_DAY +
      this.days * MS_PER_DAY +
      this.hours * MS_PER_HOUR +
      this.minutes * MS_PER_MINUTE +
      this.seconds * MS_PER_SECOND;
    // `-0` is avoided deliberately: it compares equal to `0` under `==`
    // and `===` but not under `Object.is`, so a negated zero-length
    // duration would fail an identity-based assertion while being the
    // same value. Go's int arithmetic has no such distinction.
    if (total === 0) return 0;
    return this.negative ? -total : total;
  }

  /**
   * Whether the duration is subtractive. A zero-length duration is
   * never negative, however it was authored — `-PT0S` is zero.
   */
  isNegative(): boolean {
    return this.negative && !this.zeroLength;
  }

  /**
   * Advance `t` by this duration, honouring calendar semantics: weeks
   * and days move by calendar date, the hour/minute/second part is
   * added as elapsed time.
   *
   * Every V* instant is UTC, where a calendar day is always 24 hours,
   * so the two paths coincide here. They are kept separate anyway
   * because the distinction is the reason the authored units are
   * preserved at all, and a port that collapses them invites the
   * collapse back into {@link toString}.
   */
  addTo(t: Instant): Instant {
    const sign = this.negative ? -1 : 1;
    const days = sign * (this.weeks * 7 + this.days);
    const clock =
      this.hours * MS_PER_HOUR + this.minutes * MS_PER_MINUTE + this.seconds * MS_PER_SECOND;
    return t + days * MS_PER_DAY + sign * clock;
  }
}

/**
 * Decode an RFC 5545 §3.3.6 DURATION value — no property name, no
 * parameters, e.g. `-PT15M`.
 *
 * The grammar accepted is exactly:
 *
 * ```
 * dur-value  = ["+" / "-"] "P" (dur-date / dur-time / dur-week)
 * dur-date   = dur-day [dur-time]
 * dur-time   = "T" (dur-hour / dur-minute / dur-second)
 * dur-week   = 1*DIGIT "W"
 * dur-hour   = 1*DIGIT "H" [dur-minute]
 * dur-minute = 1*DIGIT "M" [dur-second]
 * dur-second = 1*DIGIT "S"
 * dur-day    = 1*DIGIT "D"
 * ```
 *
 * Parsing is strict, matching {@link parseTime}'s posture. Throws
 * `ErrMalformed` on: empty input, a missing `P`, lowercase designators,
 * weeks mixed with any other unit, time units outside the `T` part,
 * units out of RFC order, repeated units, digits with no unit, an empty
 * `T` part, ISO 8601 years or months (`P1Y`, `P1M` — not in RFC 5545),
 * fractional values, per-component signs, and any whitespace.
 */
export function parse(s: string): VDuration {
  if (s === "") throw malformed("empty input");

  let rest = s;
  let negative = false;
  if (rest[0] === "+") {
    rest = rest.slice(1);
  } else if (rest[0] === "-") {
    negative = true;
    rest = rest.slice(1);
  }
  if (rest[0] !== "P") throw malformed(`${JSON.stringify(s)} is missing its "P" designator`);
  rest = rest.slice(1);
  if (rest === "") throw malformed(`${JSON.stringify(s)} has no value after "P"`);

  const fields: DurationFields = {
    negative,
    weeks: 0,
    days: 0,
    hours: 0,
    minutes: 0,
    seconds: 0,
    dayForm: false,
  };

  // A bare time part: "PT…".
  if (rest[0] === "T") {
    parseTimePart(rest.slice(1), fields, s);
    return new VDuration(fields);
  }

  const cut = rest.indexOf("T");
  const datePart = cut < 0 ? rest : rest.slice(0, cut);
  const timePart = cut < 0 ? "" : rest.slice(cut + 1);
  const hasTime = cut >= 0;

  const first = nextField(datePart, s);
  switch (first.unit) {
    case "W":
      if (first.rest !== "") throw malformed(`${JSON.stringify(s)} mixes weeks with other units`);
      if (hasTime) throw malformed(`${JSON.stringify(s)} mixes weeks with a time part`);
      fields.weeks = first.n;
      break;
    case "D":
      if (first.rest !== "") {
        throw malformed(
          `${JSON.stringify(s)} has trailing input ${JSON.stringify(first.rest)} after the day value`,
        );
      }
      fields.days = first.n;
      fields.dayForm = true;
      break;
    default:
      throw malformed(
        `${JSON.stringify(s)} uses unit ${JSON.stringify(first.unit)} outside a time part ` +
          "(RFC 5545 has no years or months)",
      );
  }

  if (hasTime) parseTimePart(timePart, fields, s);
  return new VDuration(fields);
}

/**
 * Decode the segment after `T` into the hour, minute and second fields.
 * Units appear at most once and in RFC order (H, then M, then S).
 */
function parseTimePart(s: string, fields: DurationFields, orig: string): void {
  if (s === "") throw malformed(`${JSON.stringify(orig)} has an empty time part`);
  // `rank` tracks how far through H→M→S we have advanced, so a repeated
  // or out-of-order unit is rejected rather than silently overwriting.
  let order = 0;
  let rest = s;
  while (rest !== "") {
    const field = nextField(rest, orig);
    let rank: number;
    switch (field.unit) {
      case "H":
        rank = 1;
        fields.hours = field.n;
        break;
      case "M":
        rank = 2;
        fields.minutes = field.n;
        break;
      case "S":
        rank = 3;
        fields.seconds = field.n;
        break;
      default:
        throw malformed(
          `${JSON.stringify(orig)} uses unknown time unit ${JSON.stringify(field.unit)}`,
        );
    }
    if (rank <= order) {
      throw malformed(
        `${JSON.stringify(orig)} repeats or misorders time unit ${JSON.stringify(field.unit)}`,
      );
    }
    order = rank;
    rest = field.rest;
  }
}

/** One `1*DIGIT UNIT` field consumed off the front of `s`. */
interface Field {
  readonly n: number;
  readonly unit: string;
  readonly rest: string;
}

/** Consume one `1*DIGIT UNIT` field, or throw `ErrMalformed`. */
function nextField(s: string, orig: string): Field {
  let i = 0;
  while (i < s.length && s[i]! >= "0" && s[i]! <= "9") i++;
  if (i === 0) throw malformed(`${JSON.stringify(orig)} has a unit with no digits`);
  if (i === s.length) throw malformed(`${JSON.stringify(orig)} has digits with no unit`);
  return { n: Number(s.slice(0, i)), unit: s[i] as string, rest: s.slice(i + 1) };
}

function malformed(message: string): VstarError {
  return new VstarError("ErrMalformed", `duration: ${message}`);
}

/**
 * Whether `s` is a well-formed RFC 5545 §3.3.6 DURATION value.
 * Equivalent to discarding {@link parse}'s result, offered so a caller
 * testing a wire string need not catch.
 */
export function valid(s: string): boolean {
  try {
    parse(s);
    return true;
  } catch {
    return false;
  }
}

/**
 * Convert a signed millisecond count into a {@link VDuration} expressed
 * in hours, minutes and seconds. Sub-second precision is truncated:
 * RFC 5545 durations have second resolution.
 *
 * The result never uses the week or day units — a millisecond count
 * carries no calendar information, so emitting `P1D` from 24 hours
 * would invent a distinction the input never made. A caller meaning
 * calendar days constructs the {@link VDuration} directly.
 */
export function fromSigned(ms: number): VDuration {
  const negative = ms < 0;
  let rest = Math.trunc(Math.abs(ms) / MS_PER_SECOND) * MS_PER_SECOND;
  const hours = Math.trunc(rest / MS_PER_HOUR);
  rest -= hours * MS_PER_HOUR;
  const minutes = Math.trunc(rest / MS_PER_MINUTE);
  rest -= minutes * MS_PER_MINUTE;
  const seconds = Math.trunc(rest / MS_PER_SECOND);
  return new VDuration({ negative, hours, minutes, seconds });
}

/**
 * A parsed RFC 5545 §3.8.6.3 TRIGGER property: either a relative offset
 * from one end of the parent component, or an absolute instant.
 *
 * Exactly one form is populated. When {@link relative} is true,
 * {@link duration} and {@link related} carry the offset and its anchor
 * and {@link absolute} is the zero instant; otherwise {@link absolute}
 * carries the instant and {@link duration} is zero-length.
 */
export class Trigger {
  /** Which of the two forms this is. */
  readonly relative: boolean;
  /**
   * The offset, meaningful only when {@link relative} is true. A
   * negative duration fires before the anchor — the common case.
   */
  readonly duration: VDuration;
  /** The anchor, meaningful only when {@link relative} is true. */
  readonly related: Related;
  /** The firing instant, meaningful only when {@link relative} is false. */
  readonly absolute: Instant;

  constructor(fields: {
    relative?: boolean;
    duration?: VDuration;
    related?: Related;
    absolute?: Instant;
  }) {
    this.relative = fields.relative ?? false;
    this.duration = fields.duration ?? new VDuration();
    this.related = fields.related ?? RELATED_START;
    this.absolute = fields.absolute ?? 0;
  }

  /**
   * Render back to the wire property.
   *
   * A relative trigger emits its duration as the value, adding
   * `RELATED=END` only when the anchor is the end — `RELATED=START` is
   * the RFC default and is left implicit. An absolute trigger emits the
   * UTC form #2 instant and carries `VALUE=DATE-TIME` explicitly, so a
   * consumer never has to infer the form.
   */
  toProperty(): Property {
    if (!this.relative) {
      return {
        name: PROP_TRIGGER,
        params: [{ name: PARAM_VALUE, value: VALUE_DATE_TIME }],
        value: formatTime(this.absolute),
      };
    }
    return {
      name: PROP_TRIGGER,
      params: this.related === RELATED_END ? [{ name: PARAM_RELATED, value: RELATED_END }] : [],
      value: this.duration.toString(),
    };
  }

  /**
   * The instant at which this trigger fires.
   *
   * An absolute trigger returns its instant and ignores both arguments.
   * A relative trigger resolves its anchor from `parent` — DTSTART for
   * `RELATED=START`; for `RELATED=END` a VTODO's DUE, else the end
   * {@link eventEnd} computes — then offsets it.
   *
   * `cal` supplies the VTIMEZONE registry used to resolve a
   * `TZID`-bearing anchor.
   *
   * Throws `ErrNoAnchor` when the anchor the RELATED parameter selects
   * is absent or unparseable. That is reported rather than silently
   * resolving against the zero instant, which would place every such
   * alarm at the epoch.
   */
  resolve(parent: Component, cal: Calendar): Instant {
    if (!this.relative) return this.absolute;
    const at = this.anchor(parent, cal);
    if (at === undefined) {
      throw new VstarError(
        "ErrNoAnchor",
        `duration: ${parent.type} has no ${this.related} anchor for a relative trigger`,
      );
    }
    return this.duration.addTo(at);
  }

  /** The parent instant this trigger is measured from. */
  private anchor(parent: Component, cal: Calendar): Instant | undefined {
    if (this.related === RELATED_START) return dtstart(parent, cal);
    // RELATED=END: a VTODO ends at DUE; everything else at DTEND or
    // DTSTART + DURATION.
    if (parent.type === "VTODO") {
      const at = due(parent, cal);
      if (at !== undefined) return at;
    }
    return eventEnd(parent, cal);
  }
}

/**
 * Decode a TRIGGER property.
 *
 * The form is chosen as follows:
 *
 * - `VALUE=DURATION`, or no VALUE parameter with a value that parses as
 *   a duration → relative.
 * - `VALUE=DATE-TIME`, or no VALUE parameter with a value that parses
 *   as an RFC 5545 form #2 instant → absolute.
 *
 * An explicit VALUE parameter is **authoritative**: a value that
 * contradicts it throws `ErrMalformed` rather than being silently
 * re-read as the other form. With no VALUE parameter the two shapes are
 * unambiguous, so the value itself decides — producers in the wild
 * routinely omit the parameter.
 *
 * RELATED is honoured on relative triggers only; RFC 5545 §3.2.14
 * scopes it to DURATION-valued triggers, so RELATED on an absolute
 * trigger is rejected. Parameter names and values match
 * case-insensitively per RFC 5545 §3.2.
 */
export function parseTrigger(p: Property): Trigger {
  const declared = paramValue(p, PARAM_VALUE);
  let t: Trigger;

  if (declared !== undefined && equalFold(declared, VALUE_DURATION)) {
    t = new Trigger({ relative: true, duration: parseOrRethrow(p.value, "VALUE=DURATION") });
  } else if (declared !== undefined && equalFold(declared, VALUE_DATE_TIME)) {
    const at = parseTime(p.value);
    if (at === undefined) {
      throw new VstarError(
        "ErrMalformed",
        `trigger: VALUE=DATE-TIME but value ${JSON.stringify(p.value)} is not an RFC 5545 form #2 instant`,
      );
    }
    t = new Trigger({ absolute: at });
  } else if (declared !== undefined) {
    throw new VstarError(
      "ErrMalformed",
      `trigger: unsupported VALUE=${declared} (want DURATION or DATE-TIME)`,
    );
  } else {
    // No VALUE parameter — infer from the value's own shape.
    if (valid(p.value)) {
      t = new Trigger({ relative: true, duration: parse(p.value) });
    } else {
      const at = parseTime(p.value);
      if (at === undefined) {
        throw new VstarError(
          "ErrMalformed",
          `trigger: value ${JSON.stringify(p.value)} is neither a DURATION nor a DATE-TIME`,
        );
      }
      t = new Trigger({ absolute: at });
    }
  }

  const related = paramValue(p, PARAM_RELATED);
  if (related === undefined) return t;
  if (!t.relative) {
    throw new VstarError(
      "ErrMalformed",
      "trigger: RELATED is meaningful only on a relative trigger (RFC 5545 §3.2.14)",
    );
  }
  if (equalFold(related, RELATED_START)) {
    return new Trigger({ relative: true, duration: t.duration, related: RELATED_START });
  }
  if (equalFold(related, RELATED_END)) {
    return new Trigger({ relative: true, duration: t.duration, related: RELATED_END });
  }
  throw new VstarError("ErrMalformed", `trigger: unknown RELATED=${related} (want START or END)`);
}

/** Parse a duration value, re-wording the failure for the VALUE context. */
function parseOrRethrow(value: string, context: string): VDuration {
  try {
    return parse(value);
  } catch (cause) {
    throw new VstarError("ErrMalformed", `trigger: ${context} but value is not a duration`, {
      cause,
    });
  }
}

/**
 * Read and parse the TRIGGER property of a VALARM.
 *
 * Throws `ErrNoTrigger` when the property is absent: RFC 5545 §3.6.6
 * makes TRIGGER mandatory on VALARM, so this is a producer bug rather
 * than an absent optional.
 */
export function alarmTrigger(alarm: Component): Trigger {
  const p = get(alarm, PROP_TRIGGER);
  if (p === undefined) {
    throw new VstarError("ErrNoTrigger", "duration: VALARM has no TRIGGER property");
  }
  return parseTrigger(p);
}

/**
 * The end instant of a component that expresses it either as DTEND or
 * as DTSTART plus a DURATION, per RFC 5545 §3.6.1 (which allows exactly
 * one of the two on a VEVENT).
 *
 * DTEND wins when both are present — it is the explicit statement.
 *
 * Returns `undefined` when neither form is available, when DTSTART is
 * missing for the DURATION form, or when the DURATION value is
 * malformed.
 */
export function eventEnd(c: Component, cal: Calendar): Instant | undefined {
  const end = dtend(c, cal);
  if (end !== undefined) return end;
  const p = get(c, PROP_DURATION);
  if (p === undefined) return undefined;
  if (!valid(p.value)) return undefined;
  const start = dtstart(c, cal);
  if (start === undefined) return undefined;
  return parse(p.value).addTo(start);
}

/**
 * The VALARM DURATION/REPEAT pair from RFC 5545 §3.8.6.2 and §3.8.6.3:
 * the interval between repetitions and how many additional times the
 * alarm repeats after its initial trigger.
 *
 * The two properties travel together — the RFC requires that if one is
 * present the other must be. Returns a zero-length duration and zero
 * when neither is present; throws `ErrMalformed` when only one is, when
 * the DURATION value is invalid, or when REPEAT is not a non-negative
 * integer.
 */
export function alarmRepeatCycle(alarm: Component): [VDuration, number] {
  const durProp = get(alarm, PROP_DURATION);
  const repProp = get(alarm, PROP_REPEAT);

  if (durProp === undefined && repProp === undefined) return [new VDuration(), 0];
  if (durProp === undefined) {
    throw new VstarError(
      "ErrMalformed",
      "duration: VALARM has REPEAT without DURATION (RFC 5545 §3.8.6.2)",
    );
  }
  if (repProp === undefined) {
    throw new VstarError(
      "ErrMalformed",
      "duration: VALARM has DURATION without REPEAT (RFC 5545 §3.8.6.2)",
    );
  }

  const d = parse(durProp.value);
  if (!/^\d+$/.test(repProp.value)) {
    throw new VstarError(
      "ErrMalformed",
      `duration: VALARM REPEAT ${JSON.stringify(repProp.value)} is not a non-negative integer`,
    );
  }
  return [d, Number(repProp.value)];
}

/** The value of the first parameter named `name`, case-insensitively. */
function paramValue(p: Property, name: string): string | undefined {
  return p.params.find((par) => equalFold(par.name, name))?.value;
}

/** ASCII-case-insensitive equality, per RFC 5545 §3.2. */
function equalFold(a: string, b: string): boolean {
  return a.toUpperCase() === b.toUpperCase();
}
