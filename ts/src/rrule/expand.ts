// SPDX-License-Identifier: MIT

// The four expansion entry points — `nextOccurrence`, `all`,
// `occurrences`, `between` — plus the shared walk behind them.
//
// Spec §Expansion names three bounding strategies and this module
// offers all three: a count (`occurrences`), a window (`between`), and
// laziness (`all`, which the consumer stops). An unbounded request
// through any of the bounded two is `ErrUnboundedExpansion`, not an
// empty result.

import type { Instant } from "../date.js";
import { VstarError } from "../errors.js";
import type { Expansion, RuleFields } from "./types.js";
import { MAX_ITERATIONS, advance, periodOccurrences } from "./evaluate.js";

export { MAX_ITERATIONS };

/**
 * Reject a rule the evaluator cannot walk.
 *
 * `parseRRule` guarantees both conditions, so this only fires for a
 * rule literal assembled by hand — which is exactly why it reports
 * `ErrUnsupportedRRule` rather than trusting the value.
 */
export function checkExpandable(rule: RuleFields): void {
  if (rule.freq === "INVALID") {
    throw new VstarError("ErrUnsupportedRRule", "rrule: FREQ is required");
  }
  if (rule.interval < 1) {
    throw new VstarError("ErrUnsupportedRRule", `rrule: INTERVAL must be >= 1, got ${rule.interval}`);
  }
}

/** The `ErrIterationCap` failure, naming the bound that was hit. */
export function iterationCap(op: string, rule: RuleFields): VstarError {
  return new VstarError(
    "ErrIterationCap",
    `rrule: ${op}: no occurrence within ${MAX_ITERATIONS} consecutive ${rule.freq} periods`,
  );
}

/** How a {@link walk} ended. */
export interface WalkResult {
  /**
   * `true` when {@link MAX_ITERATIONS} consecutive periods produced
   * nothing and the search was abandoned. This is NOT termination — the
   * rule may well have occurrences beyond the budget.
   */
  readonly capped: boolean;
}

/**
 * Yield every occurrence of `rule` from `dtstart`, chronologically,
 * into `yield_`, stopping when it returns `false`.
 *
 * The iteration bound is a starvation guard: the empty-period counter
 * resets whenever a period yields, so a regularly-firing rule runs as
 * long as the caller wants while one whose BY-* clauses can never match
 * still stops.
 *
 * An unwalkable rule yields nothing and is not a cap hit; callers that
 * need that reported run {@link checkExpandable} first.
 */
export function walk(rule: RuleFields, dtstart: Instant, yield_: (t: Instant) => boolean): WalkResult {
  try {
    checkExpandable(rule);
  } catch {
    return { capped: false };
  }

  let emitted = 0;
  let empty = 0;
  let current: Instant | undefined = dtstart;

  while (empty < MAX_ITERATIONS) {
    let produced = false;
    for (const occ of periodOccurrences(rule, current, dtstart)) {
      // A period can reach back before dtstart — a weekly expansion
      // covers the whole week dtstart falls in — and the series starts
      // at dtstart, never earlier.
      if (occ < dtstart) continue;
      // UNTIL is inclusive per RFC 5545 §3.3.10.
      if (rule.until !== undefined && occ > rule.until) return { capped: false };
      produced = true;
      emitted++;
      if (!yield_(occ)) return { capped: false };
      if (rule.count > 0 && emitted >= rule.count) return { capped: false };
    }
    empty = produced ? 0 : empty + 1;
    current = advance(rule, current);
    if (current === undefined) return { capped: false };
  }
  return { capped: true };
}

/**
 * The next occurrence strictly after `after`, or `undefined` when the
 * rule has terminated.
 *
 * Termination — UNTIL passed, COUNT exhausted, no further period — is
 * `undefined` and is normal completion. Reaching the iteration bound is
 * `ErrIterationCap` and is not: the rule has not necessarily ended, the
 * evaluator stopped looking. A caller that treats the two alike
 * silently drops occurrences, which is why they are different answers.
 *
 * The first occurrence of a rule is `dtstart` itself whenever it
 * satisfies the BY-* filters, per RFC 5545 — pass `after = dtstart` to
 * step past it.
 */
export function nextOccurrence(rule: RuleFields, dtstart: Instant, after: Instant): Instant | undefined {
  checkExpandable(rule);

  let found: Instant | undefined;
  const result = walk(rule, dtstart, (occ) => {
    if (occ > after) {
      found = occ;
      return false;
    }
    return true;
  });
  if (found !== undefined) return found;
  if (result.capped) throw iterationCap("nextOccurrence", rule);
  return undefined;
}

/**
 * Every occurrence of `rule` from `dtstart`, lazily.
 *
 * A rule with neither UNTIL nor COUNT is infinite, and iterating this
 * without breaking will not return — that is the documented behaviour
 * of a lazy sequence and the reason it exists. Use {@link occurrences}
 * or {@link between} when a bounded result is what you want.
 *
 * The sequence also ends at the iteration bound. A generator has
 * nowhere to put an error, so `all` cannot tell that apart from
 * termination; the bounded entry points report it as
 * `ErrIterationCap`.
 */
export function* all(rule: RuleFields, dtstart: Instant): Generator<Instant, void, undefined> {
  // The walk is push-shaped and a generator is pull-shaped, so the
  // occurrences of one period are buffered and drained rather than
  // re-entering the walk per value.
  try {
    checkExpandable(rule);
  } catch {
    return;
  }

  let emitted = 0;
  let empty = 0;
  let current: Instant | undefined = dtstart;

  while (empty < MAX_ITERATIONS) {
    let produced = false;
    for (const occ of periodOccurrences(rule, current, dtstart)) {
      if (occ < dtstart) continue;
      if (rule.until !== undefined && occ > rule.until) return;
      produced = true;
      emitted++;
      yield occ;
      if (rule.count > 0 && emitted >= rule.count) return;
    }
    empty = produced ? 0 : empty + 1;
    current = advance(rule, current);
    if (current === undefined) return;
  }
}

/**
 * Up to `limit` occurrences, with the flag that says which way the
 * expansion stopped.
 *
 * `complete` is `true` when the rule itself terminated within the limit
 * — the returned list is the entire series — and `false` when the limit
 * truncated it. Reporting that distinction is required by spec
 * §Expansion: a caller that stops after N steps otherwise never learns
 * whether N was the whole series or merely the first N.
 *
 * A limit of `0` returns no occurrences and `complete: false` — no
 * occurrences, and no claim that the series ended. A negative limit is
 * `ErrUnboundedExpansion`.
 */
export function occurrences(rule: RuleFields, dtstart: Instant, limit: number): Expansion {
  if (limit < 0) {
    throw new VstarError(
      "ErrUnboundedExpansion",
      `rrule: occurrences: limit must be >= 0, got ${limit}`,
    );
  }
  checkExpandable(rule);
  if (limit === 0) return { times: [], complete: false };

  const out: Instant[] = [];
  let truncated = false;
  const result = walk(rule, dtstart, (occ) => {
    if (out.length === limit) {
      // The walk produced one more than asked for, so the series
      // definitively continues past the limit.
      truncated = true;
      return false;
    }
    out.push(occ);
    return true;
  });
  if (result.capped) throw iterationCap("occurrences", rule);
  return { times: out, complete: !truncated };
}

/**
 * Every occurrence in the half-open window `[start, end)` — `start`
 * inclusive, `end` exclusive.
 *
 * The window bounds the result, so this terminates even for a rule with
 * neither UNTIL nor COUNT. It does not bound the *search*: a rule that
 * never yields never reaches `end`, so the iteration bound stops it and
 * the failure is `ErrIterationCap`.
 *
 * A missing `end`, or an `end` not strictly after `start`, is
 * `ErrUnboundedExpansion` rather than a silent empty result — an
 * unbounded window is an infinite expansion request.
 */
export function between(rule: RuleFields, dtstart: Instant, start: Instant, end: Instant): Instant[] {
  if (end === undefined || end === null || Number.isNaN(end)) {
    throw new VstarError(
      "ErrUnboundedExpansion",
      "rrule: between: end must be present (an open-ended window is an infinite expansion; use all)",
    );
  }
  if (!(end > start)) {
    throw new VstarError("ErrUnboundedExpansion", `rrule: between: end ${end} must be after start ${start}`);
  }
  checkExpandable(rule);

  const out: Instant[] = [];
  const result = walk(rule, dtstart, (occ) => {
    if (occ >= end) return false;
    if (occ >= start) out.push(occ);
    return true;
  });
  if (result.capped) throw iterationCap("between", rule);
  return out;
}
