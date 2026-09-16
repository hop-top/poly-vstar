// SPDX-License-Identifier: MIT

// Recurrence sets (RFC 5545 §3.8.5) and RECURRENCE-ID (§3.8.4.4).
//
// The evaluation order in spec §Recurrence sets is load-bearing:
// DTSTART first, the RRULE expands from it, RDATE merges in, and EXDATE
// removes LAST — so an instant named by both an RDATE and an EXDATE
// stays excluded. Applying EXDATE before RDATE would resurrect it,
// which is why `set/exdate_after_rdate` exists.

import type { Instant } from "../date.js";
import { VstarError } from "../errors.js";
import { formatTime, parseTime } from "../time.js";
import type { Component, Property } from "../types.js";
import { get } from "../types.js";
import type { Expansion, RecurrenceRange } from "./types.js";
import { Rule, parseRRule } from "./rule.js";
import { checkExpandable, iterationCap, walk } from "./expand.js";

/** Throw `ErrMalformed` with the given explanation. */
function malformed(message: string): never {
  throw new VstarError("ErrMalformed", `rrule: ${message}`);
}

/** Throw `ErrUnsupportedRRule` with the given explanation. */
function unsupported(message: string): never {
  throw new VstarError("ErrUnsupportedRRule", `rrule: ${message}`);
}

/** Sort ascending and drop duplicate instants, returning a new array. */
function sortDedupe(times: readonly Instant[]): Instant[] {
  const sorted = [...times].sort((a, b) => a - b);
  return sorted.filter((t, i) => i === 0 || t !== sorted[i - 1]);
}

/**
 * A complete recurrence definition for one component: the DTSTART
 * anchor, an optional RRULE, and the explicit RDATE additions and
 * EXDATE removals RFC 5545 §3.8.5 layers on top.
 *
 * A set with RDATE and no RRULE is legal and finite. A set with
 * neither is a single non-recurring occurrence at DTSTART.
 */
export class RuleSet {
  /** The component's DTSTART: the anchor, and occurrence #1. */
  readonly dtstart: Instant;
  /** The recurrence rule, or `undefined` for an RDATE-only set. */
  readonly rrule: Rule | undefined;
  /** Explicit additional occurrences, sorted and de-duplicated. */
  readonly rdate: readonly Instant[];
  /** Explicit exclusions; one matching nothing is silently ignored. */
  readonly exdate: readonly Instant[];

  constructor(fields: {
    dtstart: Instant;
    rrule?: Rule | undefined;
    rdate?: readonly Instant[];
    exdate?: readonly Instant[];
  }) {
    this.dtstart = fields.dtstart;
    this.rrule = fields.rrule;
    this.rdate = sortDedupe(fields.rdate ?? []);
    this.exdate = sortDedupe(fields.exdate ?? []);
  }

  /**
   * DTSTART plus every RDATE, sorted and de-duplicated — the
   * occurrences that exist independently of any rule.
   */
  private explicit(): Instant[] {
    return sortDedupe([this.dtstart, ...this.rdate]);
  }

  /**
   * Up to `limit` occurrences of the set, with the same `complete`
   * contract as the package-level {@link occurrences}.
   *
   * EXDATE removals do not consume limit slots: the limit bounds
   * *returned* occurrences, so a set whose first hundred rule
   * occurrences are all excluded still yields the hundred-and-first.
   */
  occurrences(limit: number): Expansion {
    if (limit < 0) {
      throw new VstarError(
        "ErrUnboundedExpansion",
        `rrule: RuleSet.times: limit must be >= 0, got ${limit}`,
      );
    }
    if (this.rrule !== undefined) checkExpandable(this.rrule);
    if (limit === 0) return { times: [], complete: false };

    const explicit = this.explicit();
    const out: Instant[] = [];
    let ei = 0;
    let truncated = false;

    // EXDATE is consulted here, at emit — after the rule stream and the
    // explicit stream have been merged. That is what makes the removal
    // last, and an RDATE cannot undo it.
    const emit = (t: Instant): boolean => {
      if (this.exdate.includes(t)) return true;
      if (out.length === limit) return false;
      out.push(t);
      return true;
    };

    if (this.rrule !== undefined) {
      const result = walk(this.rrule, this.dtstart, (occ) => {
        // Drain every explicit occurrence sorting before this one, so
        // the merged stream stays chronological.
        while (ei < explicit.length && (explicit[ei] as Instant) < occ) {
          if (!emit(explicit[ei] as Instant)) {
            truncated = true;
            return false;
          }
          ei++;
        }
        // The same instant from both streams is one occurrence.
        if (ei < explicit.length && explicit[ei] === occ) ei++;
        if (!emit(occ)) {
          truncated = true;
          return false;
        }
        return true;
      });
      if (result.capped) throw iterationCap("RuleSet.times", this.rrule);
      if (truncated) return { times: out, complete: false };
    }

    for (; ei < explicit.length; ei++) {
      if (!emit(explicit[ei] as Instant)) return { times: out, complete: false };
    }
    return { times: out, complete: true };
  }

  /**
   * Every occurrence of the set in the half-open window
   * `[start, end)`, with RDATE merged and EXDATE removed last.
   */
  between(start: Instant, end: Instant): Instant[] {
    if (end === undefined || end === null || Number.isNaN(end)) {
      throw new VstarError("ErrUnboundedExpansion", "rrule: RuleSet.between: end must be present");
    }
    if (!(end > start)) {
      throw new VstarError(
        "ErrUnboundedExpansion",
        `rrule: RuleSet.between: end ${end} must be after start ${start}`,
      );
    }

    const merged: Instant[] = [];
    if (this.rrule !== undefined) {
      checkExpandable(this.rrule);
      const result = walk(this.rrule, this.dtstart, (occ) => {
        if (occ >= end) return false;
        merged.push(occ);
        return true;
      });
      if (result.capped) throw iterationCap("RuleSet.between", this.rrule);
    }
    merged.push(...this.explicit());

    const windowed = merged.filter((t) => t >= start && t < end && !this.exdate.includes(t));
    return sortDedupe(windowed);
  }
}

/**
 * Build a {@link RuleSet} from a component's DTSTART, RRULE, RDATE and
 * EXDATE properties.
 *
 * EXDATE and RDATE may each appear several times and may each carry
 * several comma-separated values; every value accumulates.
 *
 * v0.1 recurrence sets are UTC form #2 only, so a `VALUE=DATE` or
 * `TZID` EXDATE/RDATE is `ErrUnsupportedRRule`. Failing closed is
 * deliberate: silently dropping an unparseable EXDATE would surface an
 * occurrence the producer explicitly cancelled.
 */
export function ruleSetFromComponent(c: Component): RuleSet {
  let dtstart = 0;
  let rrule: Rule | undefined;
  const rdate: Instant[] = [];
  const exdate: Instant[] = [];

  const start = get(c, "DTSTART");
  if (start !== undefined) {
    const t = parseTime(start.value);
    if (t === undefined) {
      malformed(`DTSTART ${JSON.stringify(start.value)} is not RFC 5545 form #2`);
    }
    dtstart = t;
  }

  for (const p of c.props) {
    switch (p.name.toUpperCase()) {
      case "RRULE":
        rrule = parseRRule(p.value);
        break;
      case "RDATE":
        rdate.push(...parseDateListProperty(p));
        break;
      case "EXDATE":
        exdate.push(...parseDateListProperty(p));
        break;
      default:
        break;
    }
  }

  return new RuleSet({ dtstart, rrule, rdate, exdate });
}

/** Validate an EXDATE/RDATE property's parameters, then parse its list. */
function parseDateListProperty(p: Property): Instant[] {
  const name = p.name.toUpperCase();
  for (const param of p.params) {
    switch (param.name.toUpperCase()) {
      case "VALUE":
        if (param.value.toUpperCase() !== "DATE-TIME") {
          // A date-only value would need a time-of-day guessed for it.
          unsupported(`${name} VALUE=${param.value} is outside the supported value types (DATE-TIME only)`);
        }
        break;
      case "TZID":
        // EXDATE and RDATE are not on the datetime-resolution
        // allow-list, so a zoned value arrives here unresolved and
        // would need the calendar's VTIMEZONE registry — which a
        // component-scoped constructor cannot reach.
        unsupported(`${name} TZID=${param.value} requires VTIMEZONE resolution unavailable at component scope`);
        break;
      default:
        break;
    }
  }
  return parseDateTimeList(p.value);
}

/**
 * Parse a comma-separated list of RFC 5545 form #2 (UTC) datetimes —
 * the value form of a DATE-TIME-valued EXDATE or RDATE.
 *
 * The result is sorted and de-duplicated, so callers get a canonical
 * set whatever order the producer wrote.
 */
export function parseDateTimeList(s: string): Instant[] {
  if (s === "") malformed("empty date-time list");
  const out: Instant[] = [];
  for (const raw of s.split(",")) {
    const t = parseTime(raw);
    if (t === undefined) {
      malformed(`${JSON.stringify(raw)} is not an RFC 5545 form #2 date-time`);
    }
    out.push(t);
  }
  return sortDedupe(out);
}

/**
 * Render instants as an EXDATE/RDATE property value: comma-separated
 * UTC form #2, sorted and de-duplicated so identical logical content
 * yields identical bytes. An empty input renders as the empty string.
 */
export function formatDateTimeList(times: readonly Instant[]): string {
  return sortDedupe(times).map(formatTime).join(",");
}

/**
 * The typed form of a RECURRENCE-ID property: the instant identifying
 * which instance of a series a component overrides, plus RANGE.
 *
 * This is parsing and typed access only. Applying overrides — taking a
 * base component plus its RECURRENCE-ID siblings and producing the
 * effective series — needs component-level semantics that sit above
 * this layer and are outside v0.1 scope.
 */
export class RecurrenceId {
  /**
   * The identified instance's original start instant — what the base
   * series expands to for it, not the overriding component's own
   * (possibly moved) DTSTART.
   */
  readonly time: Instant;
  /** The RANGE parameter; the empty string when absent. */
  readonly range: RecurrenceRange;

  constructor(time: Instant, range: RecurrenceRange) {
    this.time = time;
    this.range = range;
  }

  /**
   * Render back to wire form. RANGE is emitted only for
   * `THISANDFUTURE` — the default is expressed by omitting it, so
   * emitting a token for it would change the bytes.
   */
  toProperty(): Property {
    const p: Property = { name: "RECURRENCE-ID", params: [], value: formatTime(this.time) };
    if (this.range !== "") p.params.push({ name: "RANGE", value: this.range });
    return p;
  }
}

/**
 * Extract a {@link RecurrenceId} from a RECURRENCE-ID property.
 *
 * `ErrMalformed` for a property that is not RECURRENCE-ID, a value
 * outside form #2, or a RANGE other than `THISANDFUTURE` — RFC 5545
 * §3.2.13 defines exactly the one token. `ErrUnsupportedRRule` for
 * `VALUE=DATE` or `TZID`, for the same reasons as EXDATE and RDATE.
 */
export function parseRecurrenceId(p: Property): RecurrenceId {
  if (p.name.toUpperCase() !== "RECURRENCE-ID") {
    malformed(`property ${JSON.stringify(p.name)} is not RECURRENCE-ID`);
  }
  let range: RecurrenceRange = "";
  for (const param of p.params) {
    switch (param.name.toUpperCase()) {
      case "RANGE":
        if (param.value.toUpperCase() !== "THISANDFUTURE") {
          malformed(
            `RECURRENCE-ID RANGE=${JSON.stringify(param.value)} invalid (RFC 5545 §3.2.13 defines THISANDFUTURE only)`,
          );
        }
        range = "THISANDFUTURE";
        break;
      case "VALUE":
        if (param.value.toUpperCase() !== "DATE-TIME") {
          unsupported(
            `RECURRENCE-ID VALUE=${param.value} is outside the supported value types (DATE-TIME only)`,
          );
        }
        break;
      case "TZID":
        unsupported(
          `RECURRENCE-ID TZID=${param.value} requires VTIMEZONE resolution unavailable at property scope`,
        );
        break;
      default:
        break;
    }
  }
  const t = parseTime(p.value);
  if (t === undefined) {
    malformed(`RECURRENCE-ID ${JSON.stringify(p.value)} is not an RFC 5545 form #2 date-time`);
  }
  return new RecurrenceId(t, range);
}
