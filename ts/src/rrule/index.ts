// SPDX-License-Identifier: MIT

/**
 * `@hop-top/vstar/rrule` — RFC 5545 §3.3.10 recurrence: parsing,
 * validation, the fixed wire form, forward evaluation, bounded and
 * lazy expansion, recurrence sets, and RECURRENCE-ID.
 *
 * The accepted scope is fixed by spec/v1.0/03 §RRULE parsing scope:
 * `FREQ` of `MINUTELY`, `HOURLY`, `DAILY`, `WEEKLY`, `MONTHLY` or
 * `YEARLY`, with `INTERVAL`, `UNTIL` (UTC form #2 only), `COUNT`, every
 * `BY-*` clause, and `WKST`. `FREQ=SECONDLY` and `RSCALE` parse as
 * recognizable and are reported `ErrUnsupportedRRule`.
 *
 * Parser scope equals evaluator scope: everything the parser accepts,
 * the evaluator evaluates.
 *
 * All arithmetic is pure UTC epoch-millisecond arithmetic. There is no
 * IANA timezone database here and there must never be one — zone
 * resolution against a document's own VTIMEZONE happens a layer below,
 * so every instant reaching this module is already absolute.
 */

export { MAX_ITERATIONS, all, between, nextOccurrence, occurrences } from "./expand.js";
export { Rule, parseRRule, validateRRule } from "./rule.js";
export {
  RecurrenceId,
  RuleSet,
  formatDateTimeList,
  parseDateTimeList,
  parseRecurrenceId,
  ruleSetFromComponent,
} from "./set.js";
export { WEEKDAYS, toIsoWeekday, weekdayNumber } from "./types.js";
export type { ByDay, Expansion, Freq, RecurrenceRange, RuleFields, Weekday } from "./types.js";
