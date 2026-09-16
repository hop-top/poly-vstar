// SPDX-License-Identifier: MIT

//! RFC 5545 §3.3.10 recurrence: the RRULE parser, the wire-form
//! emitter, the forward evaluator, and the recurrence set that layers
//! `RDATE` and `EXDATE` on top of a rule.
//!
//! # Scope
//!
//! The accepted surface is fixed by `spec/v1.0/03-canonicalization.md`
//! §RRULE parsing scope, and the parser's scope equals the evaluator's:
//! everything that parses, evaluates.
//!
//! Accepted: `FREQ` (`MINUTELY`, `HOURLY`, `DAILY`, `WEEKLY`, `MONTHLY`,
//! `YEARLY`), `INTERVAL`, `UNTIL` (form #2 / UTC only), `COUNT`,
//! `BYDAY`, `BYMONTH`, `BYMONTHDAY`, `BYHOUR`, `BYMINUTE`, `BYSECOND`,
//! `BYYEARDAY` and `BYWEEKNO` (`FREQ=YEARLY` only), `BYSETPOS`, `WKST`.
//!
//! Deferred, and reported as [`Error::UnsupportedRRule`]:
//! `FREQ=SECONDLY` and `RSCALE` (RFC 7529).
//! Everything else outside the list is [`Error::Malformed`] — the two
//! are different answers and the corpus asserts which.
//!
//! # Time
//!
//! Arithmetic is pure UTC [`chrono`]: no `chrono-tz`, no system
//! database. `UNTIL`, `RDATE` and `EXDATE` are UTC form #2 by spec, so
//! there is no zone to carry and no DST rule to consult.
//!
//! # Bounding an expansion
//!
//! A rule with neither `UNTIL` nor `COUNT` is infinite, and the three
//! ways to bound it are the three entry points: a count
//! ([`occurrences`]), a window ([`between`]), or laziness ([`all`],
//! which the consumer stops). [`MAX_ITERATIONS`] guards a rule whose
//! `BY-*` clauses can never match; reaching it is
//! [`Error::IterationCap`], never an empty or completed result.
//!
//! ```
//! use hop_top_vstar::rrule::{occurrences, parse_rrule};
//!
//! let rule = parse_rrule("FREQ=WEEKLY;BYDAY=MO,WE;COUNT=4")?;
//! assert_eq!(rule.to_string(), "FREQ=WEEKLY;COUNT=4;BYDAY=MO,WE");
//!
//! let dtstart = hop_top_vstar::parse_time("20260406T090000Z").expect("a Monday");
//! let (times, complete) = occurrences(&rule, dtstart, 10)?;
//! assert_eq!(times.len(), 4);
//! assert!(complete, "COUNT=4 ended inside the limit of 10");
//! # Ok::<(), hop_top_vstar::Error>(())
//! ```

mod evaluate;
mod expand;
mod format;
mod parse;
mod set;
mod types;

pub use evaluate::{all, between, next_occurrence, occurrences, MAX_ITERATIONS};
pub use parse::{parse_rrule, validate_rrule};
pub use set::{
    format_date_time_list, parse_date_time_list, parse_recurrence_id, Range, RecurrenceId, RuleSet,
};
pub use types::{ByDay, Freq, Rule, Weekday};
