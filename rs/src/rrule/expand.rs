// SPDX-License-Identifier: MIT

//! Per-period expansion: one `FREQ` period in, its concrete occurrences
//! out.
//!
//! This is the kernel the iteration strategies above it share. It knows
//! nothing about `UNTIL`, `COUNT`, limits or windows — those bound the
//! walk, not the expansion.
//!
//! `dtstart` is the fallback for every field no `BY-*` clause
//! constrains: a `FREQ=MONTHLY` rule with no `BYMONTHDAY` fires on
//! dtstart's day-of-month, and a rule with no `BYHOUR` fires at
//! dtstart's time of day.

use chrono::{DateTime, Datelike, Duration, NaiveDate, TimeZone, Timelike, Utc};

use crate::rrule::types::{Freq, Rule, Weekday};

/// Expands one `FREQ` period into its occurrences, sorted and with
/// `BYSETPOS` applied.
///
/// Ordering happens before `BYSETPOS` because the positional filter
/// indexes the chronologically ordered set, per RFC 5545 §3.3.10.
pub(crate) fn period_occurrences(
    rule: &Rule,
    base: DateTime<Utc>,
    dtstart: DateTime<Utc>,
) -> Vec<DateTime<Utc>> {
    let mut occs = expand(rule, base, dtstart);
    occs.sort_unstable();
    if !rule.by_set_pos.is_empty() {
        occs = apply_by_set_pos(&occs, &rule.by_set_pos);
    }
    occs
}

/// Steps `current` forward by one `FREQ × INTERVAL` period.
///
/// `MONTHLY` and `YEARLY` land on the first day of the target period
/// rather than adding to the current day: adding a month to January 31
/// would overflow into March, and the expansion below reconstructs the
/// real day from `BY-*` or from dtstart anyway.
pub(crate) fn advance(rule: &Rule, current: DateTime<Utc>) -> Option<DateTime<Utc>> {
    let n = i64::from(rule.interval);
    match rule.freq {
        Freq::Hourly => current.checked_add_signed(Duration::hours(n)),
        Freq::Daily => current.checked_add_signed(Duration::days(n)),
        Freq::Weekly => current.checked_add_signed(Duration::days(7 * n)),
        Freq::Monthly => {
            let months = i64::from(current.year()) * 12 + i64::from(current.month0()) + n;
            let year = i32::try_from(months.div_euclid(12)).ok()?;
            let month = u32::try_from(months.rem_euclid(12)).ok()? + 1;
            at(year, month, 1, current)
        }
        Freq::Yearly => at(
            current.year().checked_add(i32::try_from(n).ok()?)?,
            1,
            1,
            current,
        ),
    }
}

/// A date at `clock`'s time of day, or `None` when the date does not
/// exist.
fn at(year: i32, month: u32, day: u32, clock: DateTime<Utc>) -> Option<DateTime<Utc>> {
    NaiveDate::from_ymd_opt(year, month, day)?
        .and_hms_opt(clock.hour(), clock.minute(), clock.second())
        .map(|dt| Utc.from_utc_datetime(&dt))
}

/// A UTC instant from date parts and time parts, or `None` when either
/// is out of range.
fn instant(year: i32, month: u32, day: u32, h: u32, m: u32, s: u32) -> Option<DateTime<Utc>> {
    // A `BYSECOND=60` leap second has no chrono representation as an
    // ordinary time, so it folds into the next minute's :00 the way the
    // reference's time arithmetic does.
    NaiveDate::from_ymd_opt(year, month, day)?
        .and_hms_opt(h, m, s.min(59))
        .map(|dt| Utc.from_utc_datetime(&dt))
}

fn expand(rule: &Rule, base: DateTime<Utc>, dtstart: DateTime<Utc>) -> Vec<DateTime<Utc>> {
    match rule.freq {
        Freq::Hourly => expand_hourly(rule, base, dtstart),
        Freq::Daily => expand_daily(rule, base, dtstart),
        Freq::Weekly => expand_weekly(rule, base, dtstart),
        Freq::Monthly => expand_monthly(rule, base, dtstart),
        Freq::Yearly => expand_yearly(rule, base, dtstart),
    }
}

/// One occurrence per hour, at base's minute and second unless
/// `BYMINUTE`/`BYSECOND` say otherwise. `BYHOUR` is a no-op here — every
/// hour fires — while `BYMONTH`, `BYMONTHDAY` and `BYDAY` filter.
fn expand_hourly(rule: &Rule, base: DateTime<Utc>, _dtstart: DateTime<Utc>) -> Vec<DateTime<Utc>> {
    if !matches_by_month(rule, base)
        || !matches_by_month_day(rule, base)
        || !matches_by_day(rule, base)
    {
        return Vec::new();
    }
    let minutes = or_default(&rule.by_minute, base.minute());
    let seconds = or_default(&rule.by_second, base.second());
    let mut out = Vec::with_capacity(minutes.len() * seconds.len());
    for mn in &minutes {
        for sc in &seconds {
            out.extend(instant(
                base.year(),
                base.month(),
                base.day(),
                base.hour(),
                *mn,
                *sc,
            ));
        }
    }
    out
}

/// One base day; the time-of-day clauses cross-product over it.
fn expand_daily(rule: &Rule, base: DateTime<Utc>, dtstart: DateTime<Utc>) -> Vec<DateTime<Utc>> {
    if !matches_by_month(rule, base)
        || !matches_by_month_day(rule, base)
        || !matches_by_day(rule, base)
    {
        return Vec::new();
    }
    cross_time_of_day(rule, base.date_naive(), dtstart)
}

/// With `BYDAY`, every day of the WKST-anchored week is a candidate;
/// without it, the rule fires on dtstart's weekday — which is `base`
/// itself, since the weekly step preserves weekday.
fn expand_weekly(rule: &Rule, base: DateTime<Utc>, dtstart: DateTime<Utc>) -> Vec<DateTime<Utc>> {
    if rule.by_day.is_empty() {
        if !matches_by_month(rule, base) || !matches_by_month_day(rule, base) {
            return Vec::new();
        }
        return cross_time_of_day(rule, base.date_naive(), dtstart);
    }
    let week_start = start_of_week(base.date_naive(), rule.week_start);
    let mut out = Vec::new();
    for i in 0..7 {
        let Some(day) = week_start.checked_add_signed(Duration::days(i)) else {
            continue;
        };
        if !matches_by_day_date(rule, day)
            || !matches_by_month_date(rule, day)
            || !matches_by_month_day_date(rule, day)
        {
            continue;
        }
        out.extend(cross_time_of_day(rule, day, dtstart));
    }
    out
}

/// Every candidate day of the base month, filtered by `BYDAY` with its
/// ordinals honoured.
fn expand_monthly(rule: &Rule, base: DateTime<Utc>, dtstart: DateTime<Utc>) -> Vec<DateTime<Utc>> {
    if !matches_by_month(rule, base) {
        return Vec::new();
    }
    expand_month(rule, base.year(), base.month(), dtstart)
}

/// One month of a `MONTHLY` or `YEARLY` expansion.
fn expand_month(rule: &Rule, year: i32, month: u32, dtstart: DateTime<Utc>) -> Vec<DateTime<Utc>> {
    let dim = days_in_month(year, month);
    let mut out = Vec::new();
    for d in month_day_candidates(rule, dim, dtstart.day()) {
        if d < 1 || d > dim {
            continue;
        }
        let Ok(d) = u32::try_from(d) else { continue };
        let Some(day) = NaiveDate::from_ymd_opt(year, month, d) else {
            continue;
        };
        if !matches_by_day_ordinal(rule, day, dim) {
            continue;
        }
        out.extend(cross_time_of_day(rule, day, dtstart));
    }
    out
}

/// The year, through whichever door the rule opens: `BYYEARDAY`,
/// `BYWEEKNO`, or month-by-month.
fn expand_yearly(rule: &Rule, base: DateTime<Utc>, dtstart: DateTime<Utc>) -> Vec<DateTime<Utc>> {
    let year = base.year();
    if !rule.by_year_day.is_empty() {
        return expand_by_year_day(rule, year, dtstart);
    }
    if !rule.by_week_no.is_empty() {
        return expand_by_week_no(rule, year, dtstart);
    }
    // The yearly step resets base to January to dodge day-overflow, so
    // the anchor month is dtstart's, never base's.
    let months = if rule.by_month.is_empty() {
        vec![i32::try_from(dtstart.month()).unwrap_or(1)]
    } else {
        rule.by_month.clone()
    };
    let mut out = Vec::new();
    for m in months {
        let Ok(month) = u32::try_from(m) else {
            continue;
        };
        if !(1..=12).contains(&month) {
            continue;
        }
        out.extend(expand_month(rule, year, month, dtstart));
    }
    out
}

/// `BYYEARDAY`: specific days of the year, with `BYMONTH` and `BYDAY` as
/// secondary filters. An entry the year does not have — day 366 of a
/// common year — is dropped silently, per RFC 5545.
fn expand_by_year_day(rule: &Rule, year: i32, dtstart: DateTime<Utc>) -> Vec<DateTime<Utc>> {
    let total = days_in_year(year);
    let mut out = Vec::new();
    for yd in &rule.by_year_day {
        let doy = if *yd > 0 { *yd } else { total + yd + 1 };
        if doy < 1 || doy > total {
            continue;
        }
        let Some(day) = NaiveDate::from_yo_opt(year, u32::try_from(doy).unwrap_or(1)) else {
            continue;
        };
        if !matches_by_month_date(rule, day) || !matches_by_day_date(rule, day) {
            continue;
        }
        out.extend(cross_time_of_day(rule, day, dtstart));
    }
    out
}

/// `BYWEEKNO`: all seven days of each named week, filtered by `BYDAY`
/// and `BYMONTH`. Week 1 is the WKST-anchored week containing January 4
/// — the ISO 8601 rule generalized to an arbitrary WKST.
fn expand_by_week_no(rule: &Rule, year: i32, dtstart: DateTime<Utc>) -> Vec<DateTime<Utc>> {
    let total = weeks_in_year(year, rule.week_start);
    let mut out = Vec::new();
    for wn in &rule.by_week_no {
        let n = if *wn > 0 { *wn } else { total + wn + 1 };
        if n < 1 || n > total {
            continue;
        }
        let Some(week_start) = start_of_week_n(year, n, rule.week_start) else {
            continue;
        };
        for i in 0..7 {
            let Some(day) = week_start.checked_add_signed(Duration::days(i)) else {
                continue;
            };
            if !matches_by_month_date(rule, day) || !matches_by_day_date(rule, day) {
                continue;
            }
            out.extend(cross_time_of_day(rule, day, dtstart));
        }
    }
    out
}

/// The WKST-anchored start of week `n` of `year`.
fn start_of_week_n(year: i32, n: i32, wkst: Weekday) -> Option<NaiveDate> {
    let jan4 = NaiveDate::from_ymd_opt(year, 1, 4)?;
    start_of_week(jan4, wkst).checked_add_signed(Duration::days(7 * i64::from(n - 1)))
}

/// How many WKST-anchored weeks `year` holds under the "week 1 contains
/// January 4" rule — 52, or 53 when the trailing days of December still
/// belong to this year's numbering.
fn weeks_in_year(year: i32, wkst: Weekday) -> i32 {
    let this = NaiveDate::from_ymd_opt(year, 1, 4).map(|d| start_of_week(d, wkst));
    let next = NaiveDate::from_ymd_opt(year + 1, 1, 4).map(|d| start_of_week(d, wkst));
    match (this, next) {
        (Some(a), Some(b)) => i32::try_from((b - a).num_days() / 7).unwrap_or(52),
        _ => 52,
    }
}

/// Filters a chronologically sorted set down to the 1-based positions
/// `BYSETPOS` names. Positive indexes from the start, negative from the
/// end (`-1` is last). Out-of-range entries are dropped per RFC 5545,
/// and the result stays in chronological order however the entries were
/// written.
fn apply_by_set_pos(occs: &[DateTime<Utc>], setpos: &[i32]) -> Vec<DateTime<Utc>> {
    if occs.is_empty() {
        return Vec::new();
    }
    let len = i64::try_from(occs.len()).unwrap_or(i64::MAX);
    let mut picked = vec![false; occs.len()];
    for p in setpos {
        let idx = if *p > 0 {
            i64::from(*p) - 1
        } else {
            len + i64::from(*p)
        };
        if idx < 0 || idx >= len {
            continue;
        }
        if let Ok(i) = usize::try_from(idx) {
            picked[i] = true;
        }
    }
    occs.iter()
        .zip(&picked)
        .filter_map(|(t, keep)| keep.then_some(*t))
        .collect()
}

/// The `BYHOUR × BYMINUTE × BYSECOND` product over one date. An absent
/// clause takes its component from dtstart.
fn cross_time_of_day(rule: &Rule, date: NaiveDate, dtstart: DateTime<Utc>) -> Vec<DateTime<Utc>> {
    let hours = or_default(&rule.by_hour, dtstart.hour());
    let minutes = or_default(&rule.by_minute, dtstart.minute());
    let seconds = or_default(&rule.by_second, dtstart.second());
    let mut out = Vec::with_capacity(hours.len() * minutes.len() * seconds.len());
    for h in &hours {
        for m in &minutes {
            for s in &seconds {
                out.extend(instant(date.year(), date.month(), date.day(), *h, *m, *s));
            }
        }
    }
    out
}

/// The clause's values, or a single-element fallback when it is absent.
fn or_default(list: &[i32], fallback: u32) -> Vec<u32> {
    if list.is_empty() {
        return vec![fallback];
    }
    list.iter().filter_map(|n| u32::try_from(*n).ok()).collect()
}

fn matches_by_month(rule: &Rule, t: DateTime<Utc>) -> bool {
    matches_by_month_date(rule, t.date_naive())
}

fn matches_by_month_date(rule: &Rule, d: NaiveDate) -> bool {
    rule.by_month.is_empty()
        || rule
            .by_month
            .iter()
            .any(|m| u32::try_from(*m) == Ok(d.month()))
}

fn matches_by_month_day(rule: &Rule, t: DateTime<Utc>) -> bool {
    matches_by_month_day_date(rule, t.date_naive())
}

/// `BYMONTHDAY`, with negatives resolved against the real month length
/// so `-1` is the last day whatever that is.
fn matches_by_month_day_date(rule: &Rule, d: NaiveDate) -> bool {
    if rule.by_month_day.is_empty() {
        return true;
    }
    let dim = days_in_month(d.year(), d.month());
    let day = i32::try_from(d.day()).unwrap_or(0);
    rule.by_month_day
        .iter()
        .any(|md| (*md > 0 && *md == day) || (*md < 0 && dim + md + 1 == day))
}

/// `BYDAY` ignoring ordinals — the check for the periods where an
/// ordinal has no meaning (`HOURLY`, `DAILY`, `WEEKLY`, and the
/// week/year-day arms of `YEARLY`).
fn matches_by_day(rule: &Rule, t: DateTime<Utc>) -> bool {
    matches_by_day_date(rule, t.date_naive())
}

fn matches_by_day_date(rule: &Rule, d: NaiveDate) -> bool {
    if rule.by_day.is_empty() {
        return true;
    }
    let wd = Weekday::from_chrono(d.weekday());
    rule.by_day.iter().any(|bd| bd.weekday == wd)
}

/// `BYDAY` with ordinals honoured — the `MONTHLY` and month-wise
/// `YEARLY` check, where `2MO` is the second Monday and `-1FR` the last
/// Friday.
fn matches_by_day_ordinal(rule: &Rule, d: NaiveDate, dim: i32) -> bool {
    if rule.by_day.is_empty() {
        return true;
    }
    let wd = Weekday::from_chrono(d.weekday());
    let day = i32::try_from(d.day()).unwrap_or(0);
    rule.by_day.iter().any(|bd| {
        if bd.weekday != wd {
            return false;
        }
        if bd.ordinal == 0 {
            return true;
        }
        if bd.ordinal > 0 {
            return bd.ordinal == (day - 1) / 7 + 1;
        }
        // Counting back from the month's end: how many weekdays of this
        // kind follow this one?
        let last = last_weekday_of_month(d.year(), d.month(), dim, wd);
        bd.ordinal == -((last - day) / 7 + 1)
    })
}

/// The day-of-month of the last `wd` in the month.
fn last_weekday_of_month(year: i32, month: u32, dim: i32, wd: Weekday) -> i32 {
    let Some(last) = NaiveDate::from_ymd_opt(year, month, u32::try_from(dim).unwrap_or(28)) else {
        return 0;
    };
    let delta = (Weekday::from_chrono(last.weekday()).index() - wd.index()).rem_euclid(7);
    dim - i32::try_from(delta).unwrap_or(0)
}

/// The days-of-month to consider for one month.
///
/// `BYMONTHDAY` names them outright. Failing that, an ordinal-bearing
/// `BYDAY` means every day is a candidate and the `BYDAY` check narrows.
/// Failing both, dtstart's day is the single candidate — and when the
/// month is too short for it, the period yields nothing, which is RFC
/// 5545's explicit "skip, do not clamp".
fn month_day_candidates(rule: &Rule, dim: i32, dtstart_day: u32) -> Vec<i32> {
    if !rule.by_month_day.is_empty() {
        return rule
            .by_month_day
            .iter()
            .map(|md| if *md > 0 { *md } else { dim + md + 1 })
            .collect();
    }
    if !rule.by_day.is_empty() {
        return (1..=dim).collect();
    }
    let day = i32::try_from(dtstart_day).unwrap_or(1);
    if day > dim {
        return Vec::new();
    }
    vec![day]
}

/// The WKST-anchored start of the week containing `d`.
fn start_of_week(d: NaiveDate, wkst: Weekday) -> NaiveDate {
    let delta = (Weekday::from_chrono(d.weekday()).index() - wkst.index()).rem_euclid(7);
    d - Duration::days(delta)
}

/// The number of days in a calendar month.
fn days_in_month(year: i32, month: u32) -> i32 {
    let (ny, nm) = if month == 12 {
        (year + 1, 1)
    } else {
        (year, month + 1)
    };
    match (
        NaiveDate::from_ymd_opt(year, month, 1),
        NaiveDate::from_ymd_opt(ny, nm, 1),
    ) {
        (Some(a), Some(b)) => i32::try_from((b - a).num_days()).unwrap_or(30),
        _ => 30,
    }
}

/// 365 or 366.
fn days_in_year(year: i32) -> i32 {
    match (
        NaiveDate::from_ymd_opt(year, 1, 1),
        NaiveDate::from_ymd_opt(year + 1, 1, 1),
    ) {
        (Some(a), Some(b)) => i32::try_from((b - a).num_days()).unwrap_or(365),
        _ => 365,
    }
}

#[cfg(test)]
mod tests {
    use super::{days_in_month, days_in_year, last_weekday_of_month, start_of_week, weeks_in_year};
    use crate::rrule::types::Weekday;
    use chrono::NaiveDate;

    #[test]
    fn month_lengths_account_for_leap_years() {
        assert_eq!(days_in_month(2026, 2), 28);
        assert_eq!(days_in_month(2028, 2), 29);
        assert_eq!(days_in_month(2026, 12), 31);
        assert_eq!(days_in_year(2026), 365);
        assert_eq!(days_in_year(2028), 366);
    }

    #[test]
    fn week_start_anchors_on_wkst() {
        // 2026-01-01 is a Thursday.
        let thu = NaiveDate::from_ymd_opt(2026, 1, 1).expect("date");
        assert_eq!(
            start_of_week(thu, Weekday::Mo),
            NaiveDate::from_ymd_opt(2025, 12, 29).expect("the Monday before")
        );
        assert_eq!(
            start_of_week(thu, Weekday::Su),
            NaiveDate::from_ymd_opt(2025, 12, 28).expect("the Sunday before")
        );
    }

    #[test]
    fn finds_the_last_weekday_of_a_month() {
        // January 2026 ends on Saturday the 31st, so the last Friday is
        // the 30th and the last Sunday the 25th.
        assert_eq!(last_weekday_of_month(2026, 1, 31, Weekday::Sa), 31);
        assert_eq!(last_weekday_of_month(2026, 1, 31, Weekday::Fr), 30);
        assert_eq!(last_weekday_of_month(2026, 1, 31, Weekday::Su), 25);
    }

    #[test]
    fn counts_the_weeks_in_a_year() {
        assert!((52..=53).contains(&weeks_in_year(2026, Weekday::Mo)));
        assert_eq!(weeks_in_year(2026, Weekday::Mo), 53);
    }
}
