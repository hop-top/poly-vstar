// SPDX-License-Identifier: MIT

//! The forward evaluator and the three bounding strategies.
//!
//! One walk sits under all of them. It steps `FREQ` period by `FREQ`
//! period, expands each into its occurrences, and hands them to a
//! consumer that decides when to stop — by count, by window, or by
//! simply not asking for more.

use chrono::{DateTime, Utc};

use crate::rrule::expand::{advance, period_occurrences};
use crate::rrule::types::Rule;
use crate::{Error, Result};

/// The evaluator's iteration bound: how many consecutive `FREQ` periods
/// may yield nothing before the search is abandoned.
///
/// It is a starvation guard, not a total-occurrence limit — the counter
/// resets whenever a period produces something, so a rule that fires
/// regularly runs as long as the caller wants while one whose `BY-*`
/// clauses can never match still stops.
///
/// 100 000 covers every realistic recurrence (about 273 years of a daily
/// rule, 100 000 years of a yearly one) and fails fast on a rule that
/// never yields. Reaching it is [`Error::IterationCap`], never an empty
/// or completed result: the value is implementation-defined per spec,
/// but the distinction is not.
///
/// The bound counts `FREQ` periods, so under `FREQ=MINUTELY` at
/// `INTERVAL=1` it spans about 69 days: a satisfiable rule whose limits
/// admit nothing for longer (`FREQ=MINUTELY;BYMONTH=1` evaluated from
/// February) reports [`Error::IterationCap`] rather than the eventual
/// occurrence. Skipping the base to the next window a limit admits would
/// lift that, and is a follow-up for every `FREQ`, not a `MINUTELY`
/// concern.
///
/// The bound is deliberately not configurable. The only inputs that
/// reach it are unsatisfiable rules, for which a larger budget costs
/// more before failing identically; a caller needing a tighter bound
/// should put a deadline around the call.
pub const MAX_ITERATIONS: usize = 100_000;

/// The next occurrence strictly after `after`, computed relative to
/// `dtstart`.
///
/// Three outcomes, and telling them apart is the point:
///
/// - `Ok(Some(t))` — the next occurrence.
/// - `Ok(None)` — the rule terminated: `UNTIL` passed or `COUNT` was
///   exhausted. Normal completion.
/// - `Err(`[`Error::IterationCap`]`)` — the evaluator stepped through
///   [`MAX_ITERATIONS`] empty periods and gave up. The rule has **not**
///   necessarily ended; treating this as completion silently drops
///   occurrences.
///
/// The first occurrence of a rule is `dtstart` itself when dtstart
/// satisfies the `BY-*` filters, per RFC 5545 — pass `after = dtstart`
/// to step past it.
///
/// ```
/// use hop_top_vstar::{parse_time, rrule::{next_occurrence, parse_rrule}};
///
/// let rule = parse_rrule("FREQ=DAILY;COUNT=2")?;
/// let dtstart = parse_time("20260401T120000Z").expect("instant");
///
/// let second = next_occurrence(&rule, dtstart, dtstart)?.expect("one more");
/// assert_eq!(hop_top_vstar::format_time(second), "20260402T120000Z");
/// assert_eq!(next_occurrence(&rule, dtstart, second)?, None, "COUNT=2 is spent");
/// # Ok::<(), hop_top_vstar::Error>(())
/// ```
pub fn next_occurrence(
    rule: &Rule,
    dtstart: DateTime<Utc>,
    after: DateTime<Utc>,
) -> Result<Option<DateTime<Utc>>> {
    let mut found = None;
    let capped = walk(rule, dtstart, |occ| {
        if occ > after {
            found = Some(occ);
            return false;
        }
        true
    });
    match (found, capped) {
        (Some(t), _) => Ok(Some(t)),
        (None, true) => Err(iteration_cap("next_occurrence", rule)),
        (None, false) => Ok(None),
    }
}

/// Every occurrence of `rule` from `dtstart`, lazily.
///
/// This is the third bounding strategy — the consumer stops — and the
/// only one that works on an unbounded rule without choosing a bound up
/// front. A rule with neither `UNTIL` nor `COUNT` yields forever;
/// collecting it without a `take` will not return.
///
/// There is no error channel here, which is why the iteration cap is
/// reachable only through [`next_occurrence`], [`occurrences`] and
/// [`between`]: an `all` consumer bounds the work itself, so an
/// unsatisfiable rule ends the iterator rather than reporting.
///
/// ```
/// use hop_top_vstar::{parse_time, rrule::{all, parse_rrule}};
///
/// let rule = parse_rrule("FREQ=DAILY")?;  // unbounded
/// let dtstart = parse_time("20260401T120000Z").expect("instant");
/// assert_eq!(all(&rule, dtstart).take(3).count(), 3);
/// # Ok::<(), hop_top_vstar::Error>(())
/// ```
pub fn all(rule: &Rule, dtstart: DateTime<Utc>) -> impl Iterator<Item = DateTime<Utc>> + '_ {
    Walker::new(rule, dtstart)
}

/// Up to `limit` occurrences, plus whether the series ended inside it.
///
/// `complete` is the signal a bare stepping loop cannot give: `true`
/// means the rule terminated and the slice is the entire series; `false`
/// means the limit truncated it and more exist. A limit of 0 returns no
/// occurrences and `false` — no claim either way.
///
/// Fails with [`Error::IterationCap`] when the evaluator abandoned the
/// search, which is distinct from a completed empty series.
pub fn occurrences(
    rule: &Rule,
    dtstart: DateTime<Utc>,
    limit: usize,
) -> Result<(Vec<DateTime<Utc>>, bool)> {
    if limit == 0 {
        return Ok((Vec::new(), false));
    }
    let mut out = Vec::with_capacity(limit);
    let mut truncated = false;
    let capped = walk(rule, dtstart, |occ| {
        if out.len() == limit {
            // The walk produced one past the limit, so the series
            // definitively continues.
            truncated = true;
            return false;
        }
        out.push(occ);
        true
    });
    if capped {
        return Err(iteration_cap("occurrences", rule));
    }
    Ok((out, !truncated))
}

/// Every occurrence in the half-open window `[start, end)`.
///
/// The window bounds the result, not the search: a rule that never
/// yields never reaches `end`, so the iteration bound stops it and this
/// reports [`Error::IterationCap`].
///
/// `end` must be strictly after `start`. A zero-width or inverted window
/// is [`Error::UnboundedExpansion`] rather than an empty result — an
/// open-ended window is an infinite expansion request, and [`all`] is
/// the answer to that.
pub fn between(
    rule: &Rule,
    dtstart: DateTime<Utc>,
    start: DateTime<Utc>,
    end: DateTime<Utc>,
) -> Result<Vec<DateTime<Utc>>> {
    if end <= start {
        return Err(Error::UnboundedExpansion(format!(
            "rrule: between: end {} must be after start {}",
            crate::format_time(end),
            crate::format_time(start)
        )));
    }
    let mut out = Vec::new();
    let capped = walk(rule, dtstart, |occ| {
        if occ >= end {
            return false;
        }
        if occ >= start {
            out.push(occ);
        }
        true
    });
    if capped {
        return Err(iteration_cap("between", rule));
    }
    Ok(out)
}

/// Drives the walk, handing each occurrence to `yield_fn` until it says
/// stop or the rule ends.
///
/// Returns `true` when the walk was abandoned at the iteration bound,
/// which is the one outcome the caller must not confuse with
/// termination.
pub(crate) fn walk<F>(rule: &Rule, dtstart: DateTime<Utc>, mut yield_fn: F) -> bool
where
    F: FnMut(DateTime<Utc>) -> bool,
{
    let mut walker = Walker::new(rule, dtstart);
    for occ in walker.by_ref() {
        if !yield_fn(occ) {
            return false;
        }
    }
    walker.capped
}

/// The occurrence iterator every entry point is built on.
///
/// It holds one period's expansion at a time, so a `BYHOUR`-heavy rule
/// does not materialize a year of instants to yield the first.
struct Walker<'a> {
    rule: &'a Rule,
    dtstart: DateTime<Utc>,
    /// The period being drained.
    current: Option<DateTime<Utc>>,
    /// The remaining occurrences of that period, in reverse order so
    /// popping is O(1).
    pending: Vec<DateTime<Utc>>,
    /// How many occurrences have been emitted, for `COUNT`.
    emitted: u32,
    /// Consecutive empty periods, for the iteration bound.
    empty: usize,
    /// Whether the walk ended at the bound rather than at termination.
    capped: bool,
    /// Whether the rule has terminated; once set, nothing more is
    /// yielded.
    done: bool,
}

impl<'a> Walker<'a> {
    fn new(rule: &'a Rule, dtstart: DateTime<Utc>) -> Self {
        Self {
            rule,
            dtstart,
            current: Some(dtstart),
            pending: Vec::new(),
            emitted: 0,
            empty: 0,
            capped: false,
            done: false,
        }
    }
}

impl Iterator for Walker<'_> {
    type Item = DateTime<Utc>;

    fn next(&mut self) -> Option<DateTime<Utc>> {
        loop {
            if self.done {
                return None;
            }
            if let Some(occ) = self.pending.pop() {
                // UNTIL is inclusive per RFC 5545 §3.3.10.
                if self.rule.until.is_some_and(|u| occ > u) {
                    self.done = true;
                    return None;
                }
                self.emitted += 1;
                if self.rule.count.is_some_and(|c| self.emitted >= c) {
                    self.done = true;
                }
                return Some(occ);
            }

            if self.empty >= MAX_ITERATIONS {
                self.capped = true;
                self.done = true;
                return None;
            }
            let Some(base) = self.current else {
                self.done = true;
                return None;
            };

            // Occurrences before DTSTART are not part of the series: a
            // period straddling dtstart can expand to earlier instants.
            let mut occs: Vec<_> = period_occurrences(self.rule, base, self.dtstart)
                .into_iter()
                .filter(|t| *t >= self.dtstart)
                .collect();
            if occs.is_empty() {
                self.empty += 1;
            } else {
                self.empty = 0;
            }
            occs.reverse();
            self.pending = occs;
            self.current = advance(self.rule, base);
        }
    }
}

fn iteration_cap(op: &str, rule: &Rule) -> Error {
    Error::IterationCap(format!(
        "rrule: {op}: no occurrence within {MAX_ITERATIONS} consecutive {} periods",
        rule.freq
    ))
}

#[cfg(test)]
mod tests {
    use super::{all, between, next_occurrence, occurrences};
    use crate::rrule::parse::parse_rrule;
    use crate::{format_time, parse_time};

    fn t(s: &str) -> chrono::DateTime<chrono::Utc> {
        parse_time(s).expect("instant")
    }

    #[test]
    fn dtstart_is_occurrence_one() {
        let rule = parse_rrule("FREQ=DAILY;COUNT=3").expect("parses");
        let (times, complete) = occurrences(&rule, t("20260401T120000Z"), 10).expect("expands");
        assert_eq!(format_time(times[0]), "20260401T120000Z");
        assert_eq!(times.len(), 3);
        assert!(complete);
    }

    #[test]
    fn until_is_inclusive() {
        let rule = parse_rrule("FREQ=DAILY;UNTIL=20260403T120000Z").expect("parses");
        let (times, complete) = occurrences(&rule, t("20260401T120000Z"), 10).expect("expands");
        assert_eq!(times.len(), 3, "01, 02 and 03 — UNTIL includes its instant");
        assert!(complete);
    }

    #[test]
    fn a_window_is_half_open() {
        let rule = parse_rrule("FREQ=DAILY").expect("parses");
        let got = between(
            &rule,
            t("20260401T120000Z"),
            t("20260402T120000Z"),
            t("20260404T120000Z"),
        )
        .expect("window");
        assert_eq!(got.len(), 2, "start is included, end is not");
        assert_eq!(format_time(got[0]), "20260402T120000Z");
    }

    #[test]
    fn the_cap_is_not_termination() {
        let rule = parse_rrule("FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30").expect("parses");
        let dtstart = t("20260101T090000Z");
        assert_eq!(
            occurrences(&rule, dtstart, 1).expect_err("caps").sentinel(),
            "ErrIterationCap"
        );
        assert_eq!(
            next_occurrence(&rule, dtstart, dtstart)
                .expect_err("caps")
                .sentinel(),
            "ErrIterationCap"
        );
        // `all` has no error channel, so the same rule simply ends.
        assert_eq!(all(&rule, dtstart).count(), 0);
    }
}
