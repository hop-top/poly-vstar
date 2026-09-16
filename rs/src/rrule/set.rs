// SPDX-License-Identifier: MIT

//! Recurrence sets: a `DTSTART` anchor, an optional `RRULE`, the
//! explicit `RDATE` additions and the `EXDATE` removals RFC 5545 §3.8.5
//! layers on top — plus `RECURRENCE-ID`, which names one instance of
//! such a series.
//!
//! # Evaluation order
//!
//! Per spec §Recurrence sets, and the order is load-bearing:
//!
//! 1. `DTSTART` is occurrence #1.
//! 2. The `RRULE`, if any, expands from `DTSTART`.
//! 3. `RDATE` values are merged in.
//! 4. `EXDATE` values are removed — **last**, so an excluded instant
//!    stays excluded even when an `RDATE` names it.
//! 5. The result is sorted and de-duplicated by instant.
//!
//! Applying `EXDATE` before `RDATE` makes step 3 resurrect what step 4
//! was asked to cancel, which is the one ordering bug a set can have.
//!
//! # Value types
//!
//! `EXDATE` and `RDATE` are UTC form #2 only. A `VALUE=DATE` or
//! `TZID` value is [`Error::UnsupportedRRule`]: resolving a date-only
//! value would mean inventing a time of day, and a zoned one would need
//! the calendar's VTIMEZONE registry that a component-scoped constructor
//! cannot reach. Failing closed is deliberate — silently dropping an
//! unparseable `EXDATE` would surface an occurrence the producer
//! explicitly cancelled.

use std::fmt;

use chrono::{DateTime, Utc};

use crate::rrule::evaluate::walk;
use crate::rrule::parse::parse_rrule;
use crate::rrule::types::Rule;
use crate::{format_time, parse_time, Component, Error, Param, Property, Result};

/// A complete recurrence definition for one component.
///
/// A set with no `rrule` is legal: an `RDATE`-only series is finite and
/// explicitly enumerated, and a set with neither is a single occurrence
/// at `dtstart`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RuleSet {
    /// The component's `DTSTART`. It anchors the expansion and is itself
    /// occurrence #1 unless an `EXDATE` removes it.
    pub dtstart: DateTime<Utc>,
    /// The recurrence rule, or `None` for an `RDATE`-only set.
    pub rrule: Option<Rule>,
    /// Explicit additional occurrences (RFC 5545 §3.8.5.2).
    pub rdate: Vec<DateTime<Utc>>,
    /// Explicit exclusions (RFC 5545 §3.8.5.1). An entry matching no
    /// occurrence is ignored, per the RFC.
    pub exdate: Vec<DateTime<Utc>>,
}

impl RuleSet {
    /// Builds a set from a component's `DTSTART`, `RRULE`, `RDATE` and
    /// `EXDATE` properties.
    ///
    /// `EXDATE` and `RDATE` may each appear several times and may each
    /// carry several comma-separated values; every value accumulates.
    ///
    /// ```
    /// use hop_top_vstar::{CompType, Component, Property, rrule::RuleSet};
    ///
    /// let mut c = Component::new(CompType::event());
    /// c.set(Property::new("UID", "e-1"));
    /// c.set(Property::new("DTSTART", "20260401T120000Z"));
    /// c.set(Property::new("RRULE", "FREQ=DAILY;COUNT=3"));
    /// c.set(Property::new("EXDATE", "20260402T120000Z"));
    ///
    /// let set = RuleSet::from_component(&c)?;
    /// let (times, complete) = set.occurrences(10)?;
    /// assert_eq!(times.len(), 2, "one of the three is excluded");
    /// assert!(complete);
    /// # Ok::<(), hop_top_vstar::Error>(())
    /// ```
    pub fn from_component(c: &Component) -> Result<Self> {
        let dtstart = match c.get("DTSTART") {
            Some(p) => parse_time(&p.value).ok_or_else(|| {
                Error::Malformed(format!(
                    "rrule: DTSTART {:?} is not RFC 5545 form #2",
                    p.value
                ))
            })?,
            None => DateTime::<Utc>::MIN_UTC,
        };

        let mut set = Self {
            dtstart,
            rrule: None,
            rdate: Vec::new(),
            exdate: Vec::new(),
        };
        for p in &c.props {
            match p.name.to_ascii_uppercase().as_str() {
                "RRULE" => set.rrule = Some(parse_rrule(&p.value)?),
                "RDATE" => set.rdate.extend(parse_date_list_property(p)?),
                "EXDATE" => set.exdate.extend(parse_date_list_property(p)?),
                _ => {}
            }
        }
        sort_dedupe(&mut set.rdate);
        sort_dedupe(&mut set.exdate);
        Ok(set)
    }

    /// Up to `limit` occurrences of the set, with the same
    /// `complete`-flag contract as the free
    /// [`occurrences`](super::occurrences).
    ///
    /// `EXDATE` removals do **not** consume limit slots: the limit
    /// bounds returned occurrences, so a set whose first hundred rule
    /// occurrences are all excluded still yields the hundred-and-first.
    pub fn occurrences(&self, limit: usize) -> Result<(Vec<DateTime<Utc>>, bool)> {
        if limit == 0 {
            return Ok((Vec::new(), false));
        }
        let explicit = self.explicit_occurrences();
        let mut out: Vec<DateTime<Utc>> = Vec::with_capacity(limit);
        let mut ei = 0usize;
        let mut truncated = false;

        // Emitting is where EXDATE applies — last, after the rule and
        // the RDATE merge have both had their say.
        macro_rules! emit {
            ($t:expr) => {{
                let t = $t;
                if self.exdate.contains(&t) {
                    true
                } else if out.len() == limit {
                    false
                } else {
                    out.push(t);
                    true
                }
            }};
        }

        if let Some(rule) = &self.rrule {
            let capped = walk(rule, self.dtstart, |occ| {
                // Drain every explicit occurrence that sorts before this
                // one, so the merged stream stays chronological.
                while ei < explicit.len() && explicit[ei] < occ {
                    if !emit!(explicit[ei]) {
                        truncated = true;
                        return false;
                    }
                    ei += 1;
                }
                if ei < explicit.len() && explicit[ei] == occ {
                    ei += 1; // the same instant; emitted once, below
                }
                if !emit!(occ) {
                    truncated = true;
                    return false;
                }
                true
            });
            if capped {
                return Err(Error::IterationCap(format!(
                    "rrule: set occurrences: no occurrence within {} consecutive {} periods",
                    super::MAX_ITERATIONS,
                    rule.freq
                )));
            }
            if truncated {
                return Ok((out, false));
            }
        }
        while ei < explicit.len() {
            if !emit!(explicit[ei]) {
                return Ok((out, false));
            }
            ei += 1;
        }
        Ok((out, true))
    }

    /// Every occurrence of the set in the half-open window
    /// `[start, end)`.
    ///
    /// Same window rules as the free [`between`](super::between): `end`
    /// must be strictly after `start`, and a rule that never yields is
    /// [`Error::IterationCap`].
    pub fn between(&self, start: DateTime<Utc>, end: DateTime<Utc>) -> Result<Vec<DateTime<Utc>>> {
        if end <= start {
            return Err(Error::UnboundedExpansion(format!(
                "rrule: set between: end {} must be after start {}",
                format_time(end),
                format_time(start)
            )));
        }
        let mut merged = Vec::new();
        if let Some(rule) = &self.rrule {
            let capped = walk(rule, self.dtstart, |occ| {
                if occ >= end {
                    return false;
                }
                merged.push(occ);
                true
            });
            if capped {
                return Err(Error::IterationCap(format!(
                    "rrule: set between: no occurrence within {} consecutive {} periods",
                    super::MAX_ITERATIONS,
                    rule.freq
                )));
            }
        }
        merged.extend(self.explicit_occurrences());

        // EXDATE last, then sort and de-duplicate.
        merged.retain(|t| *t >= start && *t < end && !self.exdate.contains(t));
        sort_dedupe(&mut merged);
        Ok(merged)
    }

    /// `DTSTART` plus every `RDATE`, sorted and de-duplicated — the
    /// occurrences that exist independently of any rule.
    fn explicit_occurrences(&self) -> Vec<DateTime<Utc>> {
        let mut out = Vec::with_capacity(self.rdate.len() + 1);
        if self.dtstart != DateTime::<Utc>::MIN_UTC {
            out.push(self.dtstart);
        }
        out.extend_from_slice(&self.rdate);
        sort_dedupe(&mut out);
        out
    }
}

/// Sorts by instant and removes duplicates, in place.
fn sort_dedupe(ts: &mut Vec<DateTime<Utc>>) {
    ts.sort_unstable();
    ts.dedup();
}

/// Validates an `EXDATE`/`RDATE` property's parameters, then parses its
/// multi-valued list.
fn parse_date_list_property(p: &Property) -> Result<Vec<DateTime<Utc>>> {
    let name = p.name.to_ascii_uppercase();
    for param in &p.params {
        match param.name.to_ascii_uppercase().as_str() {
            "VALUE" if !param.value.eq_ignore_ascii_case("DATE-TIME") => {
                return Err(Error::UnsupportedRRule(format!(
                    "rrule: {name} VALUE={} is outside the supported value types (DATE-TIME only)",
                    param.value
                )))
            }
            "TZID" => {
                return Err(Error::UnsupportedRRule(format!(
                    "rrule: {name} TZID={} requires VTIMEZONE resolution unavailable at component scope",
                    param.value
                )))
            }
            _ => {}
        }
    }
    parse_date_time_list(&p.value)
}

/// Parses a comma-separated list of RFC 5545 form #2 datetimes — the
/// value form of a DATE-TIME-valued `EXDATE` or `RDATE`.
///
/// The result is sorted and de-duplicated, so callers get a canonical
/// set whatever order the producer wrote.
///
/// A date-only (`VALUE=DATE`) value is [`Error::Malformed`] here: form
/// #2 is the only shape this parses, and the value-type rejection with
/// its own sentinel happens one level up, where the parameter is
/// visible.
pub fn parse_date_time_list(s: &str) -> Result<Vec<DateTime<Utc>>> {
    if s.is_empty() {
        return Err(Error::Malformed("rrule: empty date-time list".to_owned()));
    }
    let mut out = s
        .split(',')
        .map(|raw| {
            parse_time(raw).ok_or_else(|| {
                Error::Malformed(format!(
                    "rrule: {raw:?} is not an RFC 5545 form #2 date-time"
                ))
            })
        })
        .collect::<Result<Vec<_>>>()?;
    sort_dedupe(&mut out);
    Ok(out)
}

/// Renders instants as an `EXDATE`/`RDATE` value: comma-separated UTC
/// form #2, sorted and de-duplicated so identical logical content yields
/// identical bytes. Empty input renders as the empty string.
pub fn format_date_time_list(times: &[DateTime<Utc>]) -> String {
    let mut sorted = times.to_vec();
    sort_dedupe(&mut sorted);
    sorted
        .iter()
        .map(|t| format_time(*t))
        .collect::<Vec<_>>()
        .join(",")
}

// ── RECURRENCE-ID ─────────────────────────────────────────────────────

/// The `RECURRENCE-ID` `RANGE` parameter (RFC 5545 §3.2.13).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum Range {
    /// The default when `RANGE` is absent: the id names exactly one
    /// instance.
    #[default]
    ThisInstance,
    /// `RANGE=THISANDFUTURE`: the override applies to the named instance
    /// and every later one.
    ThisAndFuture,
}

impl fmt::Display for Range {
    /// [`Range::ThisInstance`] renders as the empty string: the RFC
    /// default is expressed by omitting the parameter entirely.
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(match self {
            Range::ThisInstance => "",
            Range::ThisAndFuture => "THISANDFUTURE",
        })
    }
}

/// The typed form of an RFC 5545 §3.8.4.4 `RECURRENCE-ID`: which
/// instance of a series a component overrides, plus the `RANGE`.
///
/// This is parsing and typed access only. *Applying* overrides — taking
/// a base component plus its `RECURRENCE-ID` siblings and producing the
/// effective series — needs component-level semantics that sit above
/// this module and is out of the spec's scope.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct RecurrenceId {
    /// The identified instance's **original** start instant — what the
    /// base series expands to for it, not the overriding component's own
    /// (possibly moved) `DTSTART`.
    pub time: DateTime<Utc>,
    /// The `RANGE` parameter; [`Range::ThisInstance`] when absent.
    pub range: Range,
}

impl RecurrenceId {
    /// Renders back to wire form. The `RANGE` parameter appears only for
    /// [`Range::ThisAndFuture`].
    pub fn to_property(&self) -> Property {
        let p = Property::new("RECURRENCE-ID", format_time(self.time));
        match self.range {
            Range::ThisInstance => p,
            Range::ThisAndFuture => Property {
                params: vec![Param::new("RANGE", self.range.to_string())],
                ..p
            },
        }
    }
}

/// Extracts a [`RecurrenceId`] from a `RECURRENCE-ID` property.
///
/// [`Error::Malformed`] when the property is not `RECURRENCE-ID`, when
/// its value is not form #2, or when `RANGE` carries anything but
/// `THISANDFUTURE`. A `TZID` or `VALUE=DATE` id is
/// [`Error::UnsupportedRRule`], for the same reason as `EXDATE`/`RDATE`.
pub fn parse_recurrence_id(p: &Property) -> Result<RecurrenceId> {
    if !p.name.eq_ignore_ascii_case("RECURRENCE-ID") {
        return Err(Error::Malformed(format!(
            "rrule: property {:?} is not RECURRENCE-ID",
            p.name
        )));
    }
    let mut range = Range::ThisInstance;
    for param in &p.params {
        match param.name.to_ascii_uppercase().as_str() {
            "RANGE" => {
                if !param.value.eq_ignore_ascii_case("THISANDFUTURE") {
                    return Err(Error::Malformed(format!(
                        "rrule: RECURRENCE-ID RANGE={:?} invalid (RFC 5545 §3.2.13 defines THISANDFUTURE only)",
                        param.value
                    )));
                }
                range = Range::ThisAndFuture;
            }
            "VALUE" if !param.value.eq_ignore_ascii_case("DATE-TIME") => {
                return Err(Error::UnsupportedRRule(format!(
                    "rrule: RECURRENCE-ID VALUE={} is outside the supported value types (DATE-TIME only)",
                    param.value
                )))
            }
            "TZID" => {
                return Err(Error::UnsupportedRRule(format!(
                    "rrule: RECURRENCE-ID TZID={} requires VTIMEZONE resolution unavailable at property scope",
                    param.value
                )))
            }
            _ => {}
        }
    }
    let time = parse_time(&p.value).ok_or_else(|| {
        Error::Malformed(format!(
            "rrule: RECURRENCE-ID {:?} is not an RFC 5545 form #2 date-time",
            p.value
        ))
    })?;
    Ok(RecurrenceId { time, range })
}

#[cfg(test)]
mod tests {
    use super::{format_date_time_list, parse_date_time_list, RuleSet};
    use crate::rrule::parse::parse_rrule;
    use crate::{format_time, parse_time};

    fn t(s: &str) -> chrono::DateTime<chrono::Utc> {
        parse_time(s).expect("instant")
    }

    #[test]
    fn exdate_wins_over_a_coinciding_rdate() {
        let set = RuleSet {
            dtstart: t("20260401T120000Z"),
            rrule: None,
            rdate: vec![t("20260410T120000Z")],
            exdate: vec![t("20260410T120000Z")],
        };
        let (times, complete) = set.occurrences(10).expect("expands");
        assert_eq!(times.len(), 1, "only DTSTART survives");
        assert!(complete);
    }

    #[test]
    fn an_rdate_only_set_is_finite() {
        let set = RuleSet {
            dtstart: t("20260401T120000Z"),
            rrule: None,
            rdate: vec![t("20260403T120000Z"), t("20260402T120000Z")],
            exdate: Vec::new(),
        };
        let (times, complete) = set.occurrences(10).expect("expands");
        assert_eq!(
            times.iter().map(|t| format_time(*t)).collect::<Vec<_>>(),
            vec!["20260401T120000Z", "20260402T120000Z", "20260403T120000Z"]
        );
        assert!(complete);
    }

    #[test]
    fn exclusions_do_not_consume_limit_slots() {
        let set = RuleSet {
            dtstart: t("20260401T120000Z"),
            rrule: Some(parse_rrule("FREQ=DAILY;COUNT=5").expect("parses")),
            rdate: Vec::new(),
            exdate: vec![t("20260401T120000Z"), t("20260402T120000Z")],
        };
        let (times, _) = set.occurrences(2).expect("expands");
        assert_eq!(
            times.iter().map(|t| format_time(*t)).collect::<Vec<_>>(),
            vec!["20260403T120000Z", "20260404T120000Z"]
        );
    }

    #[test]
    fn date_time_lists_are_canonical_in_both_directions() {
        let parsed = parse_date_time_list("20260403T120000Z,20260401T120000Z").expect("parses");
        assert_eq!(
            format_date_time_list(&parsed),
            "20260401T120000Z,20260403T120000Z"
        );
    }
}
