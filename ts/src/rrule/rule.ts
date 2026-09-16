// SPDX-License-Identifier: MIT

// The `Rule` class: the parsed fields, plus the wire form spec/v0.1/03
// §RRULE wire form fixes for emitters.

import type { Property } from "../types.js";
import { formatTime } from "../time.js";
import type { ByDay, Freq, RuleFields, Weekday } from "./types.js";
import type { Instant } from "../date.js";
import { parseRuleFields } from "./parse.js";

/**
 * The rule-part order every emitter uses, from spec §RRULE wire form.
 *
 * RFC 5545 §3.3.10 imposes no order — a producer may emit rule-parts
 * any way it likes and the value means the same thing. The spec fixes
 * one so identical logical content produces byte-identical output, and
 * it is the order the RFC's own `recur` ABNF lists: FREQ and its
 * modifiers, the termination bound, then the BY-* filters from coarsest
 * to finest, BYSETPOS last because it applies last, and WKST last of
 * all because it modifies the whole rule rather than filtering it.
 */
const BY_LIST_ORDER = [
  "BYMONTH",
  "BYWEEKNO",
  "BYYEARDAY",
  "BYMONTHDAY",
  "BYDAY",
  "BYHOUR",
  "BYMINUTE",
  "BYSECOND",
  "BYSETPOS",
] as const;

/**
 * A parsed RRULE.
 *
 * The fields are read-only: a rule is a value, and mutating one after
 * an evaluator has read it would make the `complete` flag and the
 * iteration bound mean different things mid-expansion.
 */
export class Rule implements RuleFields {
  readonly freq: Freq;
  readonly interval: number;
  readonly until: Instant | undefined;
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

  constructor(fields: RuleFields) {
    this.freq = fields.freq;
    this.interval = fields.interval;
    this.until = fields.until;
    this.count = fields.count;
    this.byDay = fields.byDay;
    this.byMonth = fields.byMonth;
    this.byMonthDay = fields.byMonthDay;
    this.byHour = fields.byHour;
    this.byMinute = fields.byMinute;
    this.bySecond = fields.bySecond;
    this.byYearDay = fields.byYearDay;
    this.byWeekNo = fields.byWeekNo;
    this.bySetPos = fields.bySetPos;
    this.weekStart = fields.weekStart;
  }

  /**
   * Render the rule as an RFC 5545 §3.3.10 RRULE property value, with
   * no `RRULE:` prefix.
   *
   * Three properties are contract, not implementation detail:
   *
   * - **Fixed rule-part order**, per spec §RRULE wire form.
   * - **Defaults elided** — `INTERVAL=1` and `WKST=MO` are omitted, so
   *   two rules differing only in whether the producer spelled out a
   *   default render identically.
   * - **List order preserved** — `BYDAY=WE,MO` stays `BYDAY=WE,MO`.
   *   RFC 5545 gives BY-* lists no ordering semantics, so sorting them
   *   would rewrite the producer's content while looking tidier.
   *   Callers wanting order-insensitive equality compare parsed rules,
   *   not strings.
   *
   * Parsing the result and re-emitting it is idempotent.
   *
   * A rule with no `FREQ` renders as the empty string rather than a
   * partial value that would fail to re-parse.
   */
  toString(): string {
    if (this.freq === "INVALID") return "";

    const parts = [`FREQ=${this.freq}`];
    if (this.interval > 1) parts.push(`INTERVAL=${this.interval}`);
    // UNTIL and COUNT are mutually exclusive; emit whichever is set.
    if (this.until !== undefined) parts.push(`UNTIL=${formatTime(this.until)}`);
    if (this.count > 0) parts.push(`COUNT=${this.count}`);

    for (const name of BY_LIST_ORDER) {
      const rendered = name === "BYDAY" ? renderByDay(this.byDay) : renderInts(this.listFor(name));
      if (rendered !== "") parts.push(`${name}=${rendered}`);
    }

    if (this.weekStart !== "MO") parts.push(`WKST=${this.weekStart}`);
    return parts.join(";");
  }

  /**
   * Render the rule as a complete {@link Property} ready to attach to a
   * component. A rule that cannot produce a valid value yields the
   * empty property rather than a broken one.
   */
  toProperty(): Property {
    const value = this.toString();
    if (value === "") return { name: "", params: [], value: "" };
    return { name: "RRULE", params: [], value };
  }

  /** The integer list behind one BY-* rule-part name. */
  private listFor(name: (typeof BY_LIST_ORDER)[number]): readonly number[] {
    switch (name) {
      case "BYMONTH":
        return this.byMonth;
      case "BYWEEKNO":
        return this.byWeekNo;
      case "BYYEARDAY":
        return this.byYearDay;
      case "BYMONTHDAY":
        return this.byMonthDay;
      case "BYHOUR":
        return this.byHour;
      case "BYMINUTE":
        return this.byMinute;
      case "BYSECOND":
        return this.bySecond;
      case "BYSETPOS":
        return this.bySetPos;
      // BYDAY is rendered by renderByDay and never reaches here.
      case "BYDAY":
        return [];
    }
  }
}

/** A comma-separated integer list, in the order the rule holds it. */
function renderInts(values: readonly number[]): string {
  return values.join(",");
}

/**
 * A `BYDAY` list. A zero ordinal renders with no numeric prefix — the
 * explicit `0` prefix is invalid per RFC 5545, so "every weekday of
 * this kind" is spelled by omission.
 */
function renderByDay(values: readonly ByDay[]): string {
  return values.map((bd) => (bd.ordinal === 0 ? bd.weekday : `${bd.ordinal}${bd.weekday}`)).join(",");
}

/**
 * Parse an RRULE property value into a {@link Rule}.
 *
 * Throws {@link VstarError} with code `ErrMalformed` for a syntactic
 * failure and `ErrUnsupportedRRule` for a feature this scope defers —
 * the two are different answers and the corpus asserts which.
 */
export function parseRRule(s: string): Rule {
  return new Rule(parseRuleFields(s));
}

/**
 * Check that `s` would parse cleanly, discarding the result.
 *
 * Returns nothing and throws exactly what {@link parseRRule} would.
 * Deliberately not a boolean: the *identity* of the failure is the
 * payload, and the `rrule/rejected/` fixtures assert it.
 */
export function validateRRule(s: string): void {
  parseRuleFields(s);
}
