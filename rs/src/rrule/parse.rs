// SPDX-License-Identifier: MIT

//! The RRULE value parser.
//!
//! Two failure classes, and which one a given input earns is contract:
//!
//! - [`Error::Malformed`] for the hard-error list in spec §RRULE
//!   parsing scope — unknown rule-part, missing `FREQ`, `BYMONTHDAY=0`,
//!   a `BYDAY` ordinal of 0, a non-positive `INTERVAL`, `UNTIL` and
//!   `COUNT` together, `UNTIL` in form #1 or #3.
//! - [`Error::UnsupportedRRule`] for what is syntactically fine but
//!   deferred: `FREQ=SECONDLY`, `RSCALE`.
//!
//! The parser is permissive about unsatisfiable `BY-*` combinations by
//! design: `FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30` parses, and surfaces
//! at evaluation as [`Error::IterationCap`].

use crate::rrule::types::{ByDay, Freq, Rule, Weekday};
use crate::{parse_time, Error, Result};

/// Parses an RRULE property value — the text after `RRULE:`, with no
/// property name and no parameters.
///
/// Rule-part order is irrelevant. Defaults are applied for the absent
/// parts: `INTERVAL=1` and `WKST=MO`.
///
/// ```
/// use hop_top_vstar::rrule::{parse_rrule, Freq, Weekday};
///
/// let rule = parse_rrule("FREQ=MONTHLY;BYDAY=-1FR")?;
/// assert_eq!(rule.freq, Freq::Monthly);
/// assert_eq!(rule.by_day[0].ordinal, -1);
/// assert_eq!(rule.by_day[0].weekday, Weekday::Fr);
/// assert_eq!(rule.interval, 1, "the RFC default");
/// # Ok::<(), hop_top_vstar::Error>(())
/// ```
pub fn parse_rrule(s: &str) -> Result<Rule> {
    if s.is_empty() {
        return Err(Error::Malformed("rrule: empty input".to_owned()));
    }

    let mut draft = Draft::default();
    let mut seen: Vec<&str> = Vec::new();
    for part in s.split(';') {
        let (key, value) = split_rule_part(part)
            .ok_or_else(|| Error::Malformed(format!("rrule: malformed rule-part {part:?}")))?;
        if seen.contains(&key) {
            return Err(Error::Malformed(format!(
                "rrule: duplicate rule-part {key:?}"
            )));
        }
        seen.push(key);
        draft.apply(key, value)?;
    }
    draft.finish()
}

/// Parses `s` and discards the result, reporting only whether it is
/// acceptable.
///
/// The failure *identity* is the payload — `ErrMalformed` and
/// `ErrUnsupportedRRule` are different answers, and the `rrule/rejected/`
/// fixtures assert which — so this returns `Result<()>` rather than a
/// boolean.
pub fn validate_rrule(s: &str) -> Result<()> {
    parse_rrule(s).map(|_| ())
}

/// Splits `KEY=VALUE`. An empty key is a malformed part; an empty value
/// is left to the per-key handler, which rejects it where a value is
/// required.
fn split_rule_part(p: &str) -> Option<(&str, &str)> {
    let idx = p.find('=')?;
    if idx == 0 {
        return None;
    }
    Some((&p[..idx], &p[idx + 1..]))
}

/// A rule under construction.
///
/// `freq` is the one field [`Rule`] cannot default, so the draft holds
/// it as an `Option` and [`Draft::finish`] turns its absence into the
/// `ErrMalformed` the spec requires.
#[derive(Default)]
struct Draft {
    freq: Option<Freq>,
    interval: Option<u32>,
    until: Option<chrono::DateTime<chrono::Utc>>,
    count: Option<u32>,
    by_month: Vec<i32>,
    by_week_no: Vec<i32>,
    by_year_day: Vec<i32>,
    by_month_day: Vec<i32>,
    by_day: Vec<ByDay>,
    by_hour: Vec<i32>,
    by_minute: Vec<i32>,
    by_second: Vec<i32>,
    by_set_pos: Vec<i32>,
    week_start: Option<Weekday>,
}

impl Draft {
    fn apply(&mut self, key: &str, value: &str) -> Result<()> {
        match key {
            "FREQ" => self.freq = Some(parse_freq(value)?),
            "INTERVAL" => self.interval = Some(parse_interval(value)?),
            "UNTIL" => self.until = Some(parse_until(value)?),
            "COUNT" => self.count = Some(parse_count(value)?),
            "BYDAY" => self.by_day = parse_by_day(value)?,
            "BYMONTH" => self.by_month = int_list("BYMONTH", value, 1, 12)?,
            "BYMONTHDAY" => self.by_month_day = signed_list("BYMONTHDAY", value, 1, 31)?,
            "BYHOUR" => self.by_hour = int_list("BYHOUR", value, 0, 23)?,
            "BYMINUTE" => self.by_minute = int_list("BYMINUTE", value, 0, 59)?,
            "BYSECOND" => self.by_second = int_list("BYSECOND", value, 0, 60)?,
            "BYYEARDAY" => self.by_year_day = signed_list("BYYEARDAY", value, 1, 366)?,
            "BYWEEKNO" => self.by_week_no = signed_list("BYWEEKNO", value, 1, 53)?,
            "BYSETPOS" => self.by_set_pos = signed_list("BYSETPOS", value, 1, 366)?,
            "WKST" => self.week_start = Some(parse_weekday(value)?),

            // Deferred by spec §RRULE parsing scope — syntactically
            // valid, and a different answer from "unknown".
            "RSCALE" => {
                return Err(Error::UnsupportedRRule(format!(
                    "rrule: rule-part {key}: outside the RRULE parsing scope"
                )))
            }

            _ => {
                return Err(Error::Malformed(format!(
                    "rrule: unknown rule-part {key:?}"
                )))
            }
        }
        Ok(())
    }

    /// Applies the cross-field invariants, which can only run once every
    /// rule-part has been consumed: rule-part order is irrelevant, so
    /// `BYSETPOS=1;BYDAY=MO` and `BYDAY=MO;BYSETPOS=1` must agree.
    fn finish(self) -> Result<Rule> {
        let freq = self
            .freq
            .ok_or_else(|| Error::Malformed("rrule: FREQ is required".to_owned()))?;

        if self.until.is_some() && self.count.is_some() {
            return Err(Error::Malformed(
                "rrule: UNTIL and COUNT are mutually exclusive".to_owned(),
            ));
        }
        if !self.by_year_day.is_empty() && freq != Freq::Yearly {
            return Err(Error::Malformed(format!(
                "rrule: BYYEARDAY requires FREQ=YEARLY (RFC 5545 §3.3.10), got FREQ={freq}"
            )));
        }
        if !self.by_week_no.is_empty() && freq != Freq::Yearly {
            return Err(Error::Malformed(format!(
                "rrule: BYWEEKNO requires FREQ=YEARLY (RFC 5545 §3.3.10), got FREQ={freq}"
            )));
        }

        let rule = Rule {
            freq,
            interval: self.interval.unwrap_or(1),
            until: self.until,
            count: self.count,
            by_month: self.by_month,
            by_week_no: self.by_week_no,
            by_year_day: self.by_year_day,
            by_month_day: self.by_month_day,
            by_day: self.by_day,
            by_hour: self.by_hour,
            by_minute: self.by_minute,
            by_second: self.by_second,
            by_set_pos: self.by_set_pos,
            week_start: self.week_start.unwrap_or(Weekday::Mo),
        };
        if !rule.by_set_pos.is_empty() && !rule.has_other_by() {
            return Err(Error::Malformed(
                "rrule: BYSETPOS requires at least one other BY-* rule-part (RFC 5545 §3.3.10)"
                    .to_owned(),
            ));
        }
        Ok(rule)
    }
}

fn parse_freq(v: &str) -> Result<Freq> {
    match v {
        "MINUTELY" => Ok(Freq::Minutely),
        "HOURLY" => Ok(Freq::Hourly),
        "DAILY" => Ok(Freq::Daily),
        "WEEKLY" => Ok(Freq::Weekly),
        "MONTHLY" => Ok(Freq::Monthly),
        "YEARLY" => Ok(Freq::Yearly),
        "SECONDLY" => Err(Error::UnsupportedRRule(format!(
            "rrule: FREQ={v}: outside the RRULE parsing scope"
        ))),
        _ => Err(Error::Malformed(format!("rrule: invalid FREQ value {v:?}"))),
    }
}

fn parse_interval(v: &str) -> Result<u32> {
    let n: i64 = v
        .parse()
        .map_err(|_| Error::Malformed(format!("rrule: INTERVAL non-integer {v:?}")))?;
    u32::try_from(n)
        .ok()
        .filter(|n| *n >= 1)
        .ok_or_else(|| Error::Malformed(format!("rrule: INTERVAL must be >= 1, got {n}")))
}

fn parse_until(v: &str) -> Result<chrono::DateTime<chrono::Utc>> {
    parse_time(v).ok_or_else(|| {
        Error::Malformed(format!(
            "rrule: UNTIL must be RFC 5545 form #2 (UTC, Z-suffixed), got {v:?}"
        ))
    })
}

fn parse_count(v: &str) -> Result<u32> {
    let n: i64 = v
        .parse()
        .map_err(|_| Error::Malformed(format!("rrule: COUNT non-integer {v:?}")))?;
    u32::try_from(n)
        .ok()
        .filter(|n| *n >= 1)
        .ok_or_else(|| Error::Malformed(format!("rrule: COUNT must be >= 1, got {n}")))
}

/// Parses a `BYDAY` list, preserving the authored order.
fn parse_by_day(v: &str) -> Result<Vec<ByDay>> {
    if v.is_empty() {
        return Err(Error::Malformed("rrule: BYDAY empty".to_owned()));
    }
    v.split(',').map(parse_by_day_entry).collect()
}

/// Parses one `[<ordinal>]<weekday>` entry.
///
/// The weekday is the last two bytes and the ordinal is everything
/// before it. Both halves are ASCII by construction — a non-ASCII entry
/// fails the weekday lookup — so byte slicing cannot split a character.
fn parse_by_day_entry(s: &str) -> Result<ByDay> {
    if s.len() < 2 || !s.is_ascii() {
        return Err(Error::Malformed(format!(
            "rrule: BYDAY entry {s:?} invalid"
        )));
    }
    let (prefix, wd) = s.split_at(s.len() - 2);
    let weekday = parse_weekday(wd)?;
    if prefix.is_empty() {
        return Ok(ByDay::new(0, weekday));
    }
    let n: i32 = prefix
        .parse()
        .map_err(|_| Error::Malformed(format!("rrule: BYDAY ordinal {prefix:?} non-integer")))?;
    if n == 0 {
        return Err(Error::Malformed(
            "rrule: BYDAY ordinal 0 invalid (RFC 5545 §3.3.10)".to_owned(),
        ));
    }
    if !(-53..=53).contains(&n) {
        return Err(Error::Malformed(format!(
            "rrule: BYDAY ordinal {n} out of range -53..53"
        )));
    }
    Ok(ByDay::new(n, weekday))
}

fn parse_weekday(s: &str) -> Result<Weekday> {
    match s {
        "SU" => Ok(Weekday::Su),
        "MO" => Ok(Weekday::Mo),
        "TU" => Ok(Weekday::Tu),
        "WE" => Ok(Weekday::We),
        "TH" => Ok(Weekday::Th),
        "FR" => Ok(Weekday::Fr),
        "SA" => Ok(Weekday::Sa),
        _ => Err(Error::Malformed(format!("rrule: invalid weekday {s:?}"))),
    }
}

/// A comma-separated list of integers, each in `lo..=hi`. Authored order
/// is preserved: RFC 5545 gives `BY-*` lists no ordering semantics, and
/// the wire form must round-trip what the producer wrote.
fn int_list(name: &str, v: &str, lo: i32, hi: i32) -> Result<Vec<i32>> {
    if v.is_empty() {
        return Err(Error::Malformed(format!("rrule: {name} empty")));
    }
    v.split(',')
        .map(|raw| {
            let n: i32 = raw
                .parse()
                .map_err(|_| Error::Malformed(format!("rrule: {name} non-integer {raw:?}")))?;
            if n < lo || n > hi {
                return Err(Error::Malformed(format!(
                    "rrule: {name} {n} out of range {lo}..{hi}"
                )));
            }
            Ok(n)
        })
        .collect()
}

/// A comma-separated list of signed integers where each `n` satisfies
/// `lo <= |n| <= hi` and `n != 0` — the "from-start or from-end" shape
/// `BYMONTHDAY`, `BYYEARDAY`, `BYWEEKNO` and `BYSETPOS` share.
fn signed_list(name: &str, v: &str, lo: i32, hi: i32) -> Result<Vec<i32>> {
    if v.is_empty() {
        return Err(Error::Malformed(format!("rrule: {name} empty")));
    }
    v.split(',')
        .map(|raw| {
            let n: i32 = raw
                .parse()
                .map_err(|_| Error::Malformed(format!("rrule: {name} non-integer {raw:?}")))?;
            if n == 0 {
                return Err(Error::Malformed(format!(
                    "rrule: {name} 0 invalid (RFC 5545 §3.3.10)"
                )));
            }
            let abs = n.abs();
            if abs < lo || abs > hi {
                return Err(Error::Malformed(format!(
                    "rrule: {name} {n} out of range -{hi}..-{lo} or {lo}..{hi}"
                )));
            }
            Ok(n)
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::{parse_rrule, validate_rrule};
    use crate::rrule::types::{Freq, Weekday};

    #[test]
    fn applies_the_rfc_defaults() {
        let r = parse_rrule("FREQ=DAILY").expect("parses");
        assert_eq!(r.interval, 1);
        assert_eq!(r.week_start, Weekday::Mo);
        assert!(r.until.is_none() && r.count.is_none());
        assert!(r.by_day.is_empty());
    }

    #[test]
    fn rule_part_order_is_irrelevant() {
        let a = parse_rrule("FREQ=WEEKLY;BYSETPOS=1;BYDAY=MO").expect("parses");
        let b = parse_rrule("BYDAY=MO;BYSETPOS=1;FREQ=WEEKLY").expect("parses");
        assert_eq!(a, b);
    }

    #[test]
    fn a_duplicate_rule_part_is_malformed() {
        let e = parse_rrule("FREQ=DAILY;COUNT=1;COUNT=2").expect_err("rejected");
        assert_eq!(e.sentinel(), "ErrMalformed");
    }

    #[test]
    fn an_unsatisfiable_combination_still_parses() {
        let r = parse_rrule("FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30").expect("parses");
        assert_eq!(r.freq, Freq::Yearly);
    }

    #[test]
    fn validate_agrees_with_parse() {
        assert!(validate_rrule("FREQ=DAILY").is_ok());
        assert_eq!(
            validate_rrule("FREQ=SECONDLY")
                .expect_err("rejected")
                .sentinel(),
            "ErrUnsupportedRRule"
        );
    }
}
