// SPDX-License-Identifier: MIT

// The typed form of an RFC 5545 §3.3.10 RRULE value, for the scope
// spec/v1.0/03 §RRULE parsing scope fixes.

import type { Instant } from "../date.js";

/**
 * The RRULE `FREQ` value.
 *
 * `"INVALID"` is the unset marker — a {@link Rule} literal built
 * without an explicit `freq` carries it, and a successful parse never
 * produces it, because a missing `FREQ` is `ErrMalformed`.
 *
 * `SECONDLY` is deliberately absent: it is a syntactically valid RFC
 * value that this scope defers, so it is rejected at parse with
 * `ErrUnsupportedRRule` rather than modelled.
 */
export type Freq = "INVALID" | "MINUTELY" | "HOURLY" | "DAILY" | "WEEKLY" | "MONTHLY" | "YEARLY";

/**
 * An RFC 5545 §3.3.10 weekday symbol.
 *
 * Numbering is `SU = 0` through `SA = 6` per the RFC — see
 * {@link toIsoWeekday}, which is the one place the conversion to
 * ISO-8601's `MO = 1` should live.
 */
export type Weekday = "SU" | "MO" | "TU" | "WE" | "TH" | "FR" | "SA";

/** The weekday symbols in RFC order, indexed by their RFC number. */
export const WEEKDAYS: readonly Weekday[] = ["SU", "MO", "TU", "WE", "TH", "FR", "SA"];

/**
 * The RFC 5545 §3.3.10 number of a weekday symbol: `SU = 0` … `SA = 6`.
 *
 * This is the numbering every calculation in this module uses, and it
 * matches `Date#getUTCDay()` exactly — which is why no conversion
 * appears at the arithmetic sites.
 */
export function weekdayNumber(w: Weekday): number {
  return WEEKDAYS.indexOf(w);
}

/**
 * The ISO-8601 number of a weekday symbol: `MO = 1` … `SU = 7`.
 *
 * The explicit counterpart to {@link weekdayNumber}, exposed so callers
 * that need the ISO spelling convert at one named boundary instead of
 * reusing a platform weekday number and inheriting a one-off bug.
 */
export function toIsoWeekday(w: Weekday): number {
  const n = weekdayNumber(w);
  return n === 0 ? 7 : n;
}

/**
 * One entry in a `BYDAY` list: an optional ordinal plus a weekday.
 *
 * `ordinal === 0` means "every weekday of this kind in the containing
 * FREQ period" — `BYDAY=MO` under `FREQ=MONTHLY` is every Monday of the
 * month. A non-zero ordinal in -53..-1 or 1..53 picks the nth, counting
 * from the start when positive and from the end when negative. The
 * explicit `0` prefix is invalid per RFC and rejected at parse.
 */
export interface ByDay {
  readonly ordinal: number;
  readonly weekday: Weekday;
}

/**
 * The structured form of an RRULE value.
 *
 * Absent list rule-parts are empty arrays; absent `until` is
 * `undefined` and absent `count` is `0`. `interval` defaults to 1 and
 * `weekStart` to `MO`, both applied at parse.
 *
 * Field order here is for reading; rule-part order on the wire is
 * irrelevant on parse and fixed on emit — see {@link Rule.toString}.
 */
export interface RuleFields {
  readonly freq: Freq;
  readonly interval: number;
  /** The `UNTIL` instant (form #2, UTC), or `undefined` when absent. */
  readonly until: Instant | undefined;
  /** The `COUNT` value, or `0` when absent. Exclusive with `until`. */
  readonly count: number;
  readonly byDay: readonly ByDay[];
  readonly byMonth: readonly number[];
  readonly byMonthDay: readonly number[];
  readonly byHour: readonly number[];
  readonly byMinute: readonly number[];
  readonly bySecond: readonly number[];
  readonly byYearDay: readonly number[];
  readonly byWeekNo: readonly number[];
  readonly bySetPos: readonly number[];
  readonly weekStart: Weekday;
}

/**
 * The RECURRENCE-ID `RANGE` parameter (RFC 5545 §3.2.13).
 *
 * The empty string is the default — the RFC expresses "this instance
 * only" by omitting the parameter entirely, so the default has no wire
 * token of its own and emitting one would change the bytes.
 */
export type RecurrenceRange = "" | "THISANDFUTURE";

/** The result of a bounded expansion: the occurrences and how it ended. */
export interface Expansion {
  /** The occurrences produced, in chronological order. */
  readonly times: readonly Instant[];
  /**
   * `true` when the series itself ended within the bound, `false` when
   * the bound truncated it.
   *
   * The distinction is required by spec §Expansion and is why this is a
   * record rather than a bare array: a caller that stops after N steps
   * otherwise never learns whether N was the whole series.
   */
  readonly complete: boolean;
}
