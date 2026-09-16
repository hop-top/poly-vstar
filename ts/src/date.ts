// SPDX-License-Identifier: MIT

import type { Component } from "./types.js";
import { get, remove, set } from "./types.js";

/**
 * An instant, as epoch milliseconds in UTC.
 *
 * V* handles instants at second resolution and formats them as RFC 5545
 * form #2 (`YYYYMMDDTHHMMSSZ`). The port carries its own formatter and
 * parser over this integer — never `Date#toISOString()` semantics,
 * which emit the extended form with fractional seconds, throw on
 * out-of-range values, and use expanded-year notation outside
 * 1000–9999.
 */
export type Instant = number;

/** The exact wire width of an RFC 5545 §3.3.4 DATE. */
const DATE_OCTETS = 8;

/**
 * An RFC 5545 §3.3.4 DATE value: a calendar date with no time and no
 * time zone.
 *
 * Named `VDate` rather than `Date` because `Date` is a built-in — see
 * the API mapping.
 *
 * ## Why a distinct type
 *
 * DATE and DATE-TIME are semantically different, not two spellings of
 * one thing. `DUE;VALUE=DATE:20260515` means "due on the 15th, as
 * reckoned by whoever reads it"; `DUE:20260515T000000Z` means "due at
 * one specific instant". A task due on the 15th is not late at
 * 00:00:01Z; a task due at midnight UTC is.
 *
 * An instant cannot carry that distinction — a date-only value stored
 * as one is indistinguishable from a midnight instant, and forgetting
 * the out-of-band flag is silent data corruption rather than a type
 * error. `VDate` has no clock fields at all, so the distinction cannot
 * be lost by accident.
 *
 * `month` is **1-based**, not a zero-based `Date#getMonth()` index.
 *
 * The zero `VDate` (all fields 0) is the "no date" sentinel: it formats
 * as the empty string, and the date-typed setters treat it as "clear
 * the property".
 */
export interface VDate {
  year: number;
  month: number;
  day: number;
}

/** The zero {@link VDate} — the "no date" sentinel. */
const ZERO_DATE: VDate = { year: 0, month: 0, day: 0 };

/** Report whether `d` is the zero {@link VDate}. */
export function isZero(d: VDate): boolean {
  return d.year === 0 && d.month === 0 && d.day === 0;
}

/**
 * The calendar date of `t`, reckoned in UTC.
 *
 * `dateOf` of the zero instant sentinel (`0`) returns the zero
 * {@link VDate}, mirroring the Go reference's zero-time handling.
 */
export function dateOf(t: Instant): VDate {
  if (t === 0) return { ...ZERO_DATE };
  const d = new Date(t);
  return { year: d.getUTCFullYear(), month: d.getUTCMonth() + 1, day: d.getUTCDate() };
}

/**
 * `d` as midnight UTC, for callers handing the value to time-based
 * arithmetic.
 *
 * The conversion is lossy by design and one-way: the result no longer
 * records that its source was date-only. Do not round-trip a `VDate`
 * through an instant to store it — use the date-typed accessors, which
 * preserve the DATE value type on the wire.
 */
export function toInstant(d: VDate): Instant {
  if (isZero(d)) return 0;
  return Date.UTC(d.year, d.month - 1, d.day);
}

/** The RFC 5545 §3.3.4 wire form of `d`, or `""` for the zero date. */
export function dateToString(d: VDate): string {
  return formatDate(d);
}

/**
 * Render `d` as an RFC 5545 §3.3.4 DATE string — `YYYYMMDD`,
 * zero-padded to eight octets.
 *
 * The zero date renders as the empty string, which is how the
 * date-typed writers spell "clear the property".
 *
 * Out-of-range field values (month 13, day 32, a year outside
 * 0000–9999) render as the empty string rather than an impossible wire
 * form: the DATE production is a fixed-width four-digit year, and
 * emitting torn data would defeat the strictness the reader enforces.
 */
export function formatDate(d: VDate): string {
  if (isZero(d)) return "";
  if (
    !Number.isInteger(d.year) ||
    !Number.isInteger(d.month) ||
    !Number.isInteger(d.day) ||
    d.year < 0 ||
    d.year > 9999 ||
    d.month < 1 ||
    d.month > 12 ||
    d.day < 1 ||
    d.day > 31
  ) {
    return "";
  }
  return (
    String(d.year).padStart(4, "0") +
    String(d.month).padStart(2, "0") +
    String(d.day).padStart(2, "0")
  );
}

/**
 * Parse an RFC 5545 §3.3.4 DATE string (`YYYYMMDD`). Returns
 * `undefined` for any other input shape — strict by design.
 *
 * Rejected: DATE-TIME forms, ISO 8601 extended layouts (`2026-05-15`),
 * impossible calendar dates (Feb 30, month 13, day 0, Feb 29 in a
 * non-leap year) with no silent roll-over, and any input that is not
 * exactly eight ASCII digits.
 */
export function parseDate(s: string): VDate | undefined {
  if (s.length !== DATE_OCTETS) return undefined;
  if (!/^\d{8}$/.test(s)) return undefined;
  const year = Number(s.slice(0, 4));
  const month = Number(s.slice(4, 6));
  const day = Number(s.slice(6, 8));
  if (month < 1 || month > 12 || day < 1 || day > daysInMonth(year, month)) return undefined;
  return { year, month, day };
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
 * The RFC 5545 §3.2.20 parameter name that declares a property's value
 * type explicitly.
 */
export const VALUE_PARAM = "VALUE";

/**
 * The RFC 5545 §3.2.20 VALUE parameter value selecting the DATE value
 * type (§3.3.4).
 *
 * The parameter is REQUIRED on any date-only DTSTART/DTEND/DUE/
 * COMPLETED: the default value type for those properties is DATE-TIME,
 * so an untagged eight-octet value is a malformed DATE-TIME, not a
 * DATE.
 */
export const VALUE_DATE = "DATE";

/**
 * Report whether the named property is present and carries
 * `VALUE=DATE` — i.e. whether its value is a calendar date rather than
 * an instant. This is the branch point for callers that do not know
 * the wire form up front. Returns `false` for an absent property.
 */
export function isDateOnly(c: Component, name: string): boolean {
  const p = get(c, name);
  if (p === undefined) return false;
  const v = p.params.find((x) => x.name.toUpperCase() === VALUE_PARAM)?.value;
  return v !== undefined && v.toUpperCase() === VALUE_DATE;
}

/**
 * Parse a date-bearing property's value. Returns `undefined` for a
 * missing property, one that does not declare `VALUE=DATE`, or a
 * malformed DATE value.
 *
 * The `VALUE=DATE` requirement is deliberate, not merely defensive: an
 * untagged `20260515` declares itself DATE-TIME by default and is
 * simply torn data. Promoting it to a `VDate` would be the silent
 * coercion the parsers exist to prevent.
 */
function dateProp(c: Component, name: string): VDate | undefined {
  if (!isDateOnly(c, name)) return undefined;
  const p = get(c, name);
  return p === undefined ? undefined : parseDate(p.value);
}

/**
 * The DTSTART value as a calendar date when the property carries
 * `VALUE=DATE` (an all-day event or task). A midnight DATE-TIME does
 * not surface here.
 */
export function dtstartDate(c: Component): VDate | undefined {
  return dateProp(c, "DTSTART");
}

/**
 * The DTEND value as a calendar date.
 *
 * Note RFC 5545 §3.6.1: for an all-day event DTEND is EXCLUSIVE — a
 * one-day event on the 15th has `DTEND;VALUE=DATE:20260516`. This
 * accessor reports the wire value as written and does not adjust it.
 */
export function dtendDate(c: Component): VDate | undefined {
  return dateProp(c, "DTEND");
}

/** The VTODO DUE value as a calendar date — the all-day-task reader. */
export function dueDate(c: Component): VDate | undefined {
  return dateProp(c, "DUE");
}

/**
 * The VTODO COMPLETED value as a calendar date.
 *
 * RFC 5545 §3.8.2.1 defines COMPLETED as DATE-TIME only, so a
 * `VALUE=DATE` COMPLETED is non-conforming input. The accessor exists
 * for symmetry and to let readers recover such a value rather than
 * lose it.
 */
export function completedDate(c: Component): VDate | undefined {
  return dateProp(c, "COMPLETED");
}

/**
 * Write a date-only value for the named property, or remove the
 * property entirely when `d` is the zero date.
 *
 * The written property carries exactly one parameter, `VALUE=DATE`,
 * and nothing else. Dropping pre-existing parameters is required here,
 * not merely tidy: a stale TZID from a prior local-time form would be
 * meaningless on a DATE (RFC 5545 §3.2.19 scopes TZID to DATE-TIME and
 * TIME values), and a stale parameter set would make the canonical
 * bytes depend on the property's edit history.
 */
function setOrClearDate(c: Component, name: string, d: VDate): void {
  if (isZero(d)) {
    remove(c, name);
    return;
  }
  set(c, {
    name,
    params: [{ name: VALUE_PARAM, value: VALUE_DATE }],
    value: formatDate(d),
  });
}

/**
 * Write an all-day DTSTART: the RFC 5545 §3.3.4 DATE form plus the
 * required `VALUE=DATE` parameter. The zero date removes the property.
 */
export function setDtstartDate(c: Component, d: VDate): void {
  setOrClearDate(c, "DTSTART", d);
}

/**
 * Write an all-day DTEND.
 *
 * Per RFC 5545 §3.6.1 the all-day DTEND is EXCLUSIVE: to express a
 * one-day event on the 15th, pass the 16th. This setter writes what it
 * is given and does not adjust.
 */
export function setDtendDate(c: Component, d: VDate): void {
  setOrClearDate(c, "DTEND", d);
}

/** Write an all-day DUE — the all-day-task writer. */
export function setDueDate(c: Component, d: VDate): void {
  setOrClearDate(c, "DUE", d);
}

/**
 * Write a date-only COMPLETED.
 *
 * RFC 5545 §3.8.2.1 mandates DATE-TIME for COMPLETED, so this emits
 * non-conforming output. It exists for symmetry.
 */
export function setCompletedDate(c: Component, d: VDate): void {
  setOrClearDate(c, "COMPLETED", d);
}
