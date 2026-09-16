// SPDX-License-Identifier: MIT

//! RFC 5545 §3.3.5 DATE-TIME: form #2 (`YYYYMMDDTHHMMSSZ`, UTC) on the
//! wire, and form #1 (`YYYYMMDDTHHMMSS`, local) resolved against a
//! VTIMEZONE carried by the document itself.
//!
//! # No IANA timezone database
//!
//! A V\* implementation resolves local times **only** against the
//! VTIMEZONE definitions inside the document being processed. It never
//! consults a system or bundled database — no `chrono-tz`, no
//! `tzfile`, no `iana-time-zone`. Offsets are reconstructed as
//! [`chrono::FixedOffset`] values read straight off the document's
//! `TZOFFSETTO` properties.
//!
//! The reason is determinism. A calendar carrying its own VTIMEZONE
//! hashes the same on every machine, in every year, regardless of which
//! tzdata release is installed. The moment an implementation reaches
//! for the system database, two conformant implementations can produce
//! different canonical bytes for the same document — the exact failure
//! the specification exists to prevent.
//!
//! `spec/behavior/time/tzid.json` is the gate, and it is a trap by
//! design: it names a real IANA zone, so a port resolving it from the
//! system database passes the resolvable rows and fails every zone the
//! corpus does not carry.

use crate::model::{Calendar, Component};
use chrono::{
    DateTime, Datelike, Duration, FixedOffset, NaiveDate, NaiveDateTime, TimeZone, Timelike, Utc,
};

/// Form #2 is exactly 16 octets: 8 date + `T` + 6 time + `Z`.
const FORM_TWO_OCTETS: usize = 16;

/// Form #1 is exactly 15 octets: 8 date + `T` + 6 time, no `Z`.
const FORM_ONE_OCTETS: usize = 15;

/// Seconds in one hour, for offset arithmetic.
const SECONDS_PER_HOUR: i32 = 3600;

/// Seconds in one minute.
const SECONDS_PER_MINUTE: i32 = 60;

/// Renders `t` as an RFC 5545 §3.3.5 form #2 string —
/// `YYYYMMDDTHHMMSSZ` in UTC.
///
/// The format pattern is explicit rather than delegated to
/// `to_rfc3339` or any ISO-8601 convenience: form #2 carries no
/// separators, no fractional seconds and no expanded-year notation, and
/// each of those differences produces silently divergent canonical
/// bytes.
///
/// Sub-second precision is truncated, since form #2 carries only second
/// resolution and rounding would move an instant across a boundary.
pub fn format_time(t: DateTime<Utc>) -> String {
    t.format("%Y%m%dT%H%M%SZ").to_string()
}

/// Parses an RFC 5545 §3.3.5 form #2 string (`YYYYMMDDTHHMMSSZ`).
///
/// Strict by design, mirroring [`crate::parse_date`]. Rejected: form #1
/// (no zone), RFC 3339 / ISO 8601 extended layouts, date-only values, a
/// lowercase `z`, leading or trailing whitespace, any length other than
/// sixteen octets, and impossible calendar dates — February 30th does
/// not roll into March.
///
/// The `None` IS the error signal; no sentinel is returned.
pub fn parse_time(s: &str) -> Option<DateTime<Utc>> {
    if s.len() != FORM_TWO_OCTETS || !s.ends_with('Z') {
        return None;
    }
    let naive = parse_wall(&s[..FORM_ONE_OCTETS])?;
    Some(Utc.from_utc_datetime(&naive))
}

/// Decodes an RFC 5545 form #1 wire string into its wall-clock fields.
///
/// Every field's range and the month's real length are validated by
/// `chrono`, which rejects an impossible date rather than rolling it
/// over.
fn parse_wall(s: &str) -> Option<NaiveDateTime> {
    if s.len() != FORM_ONE_OCTETS {
        return None;
    }
    let b = s.as_bytes();
    if b[8] != b'T' {
        return None;
    }
    if !b[..8].iter().all(u8::is_ascii_digit) || !b[9..].iter().all(u8::is_ascii_digit) {
        return None;
    }
    let num = |lo: usize, hi: usize| -> u32 {
        s[lo..hi]
            .bytes()
            .fold(0u32, |acc, c| acc * 10 + u32::from(c - b'0'))
    };
    let year = i32::try_from(num(0, 4)).ok()?;
    let date = NaiveDate::from_ymd_opt(year, num(4, 6), num(6, 8))?;
    date.and_hms_opt(num(9, 11), num(11, 13), num(13, 15))
}

/// Decodes `s` as form #1, rejecting a `Z` suffix explicitly so a form
/// #2 value never reaches the zone arithmetic.
fn parse_form_one(s: &str) -> Option<NaiveDateTime> {
    if s.ends_with('Z') || s.ends_with('z') {
        return None;
    }
    parse_wall(s)
}

/// Parses an RFC 5545 §3.3.5 form #1 value as a wall-clock time in the
/// zone `tzid` names, where the zone is reconstructed from a VTIMEZONE
/// component inside `cal`.
///
/// Returns `None` — not an error — when:
///
/// - `tzid` is empty;
/// - `cal` carries no VTIMEZONE whose `TZID` matches (comparison is
///   **case-sensitive**: TZIDs are opaque identifiers per RFC 5545
///   §3.2.19);
/// - the matching VTIMEZONE falls outside the spec's VTIMEZONE subset —
///   multiple `STANDARD` or `DAYLIGHT` children, a missing offset or
///   `DTSTART`, or an `RRULE` the subset does not accept;
/// - `s` is not form #1.
///
/// Form #2 input is rejected even with a `TZID` present: the value is
/// already absolute, and resolving it against a zone would apply an
/// offset twice. Rule 5 falls back to verbatim emit in that case.
///
/// Resolution failure is a normal outcome, not an error. The canonical
/// layer passes the value and its `TZID` parameter through unchanged.
pub fn parse_time_with_tzid(s: &str, tzid: &str, cal: &Calendar) -> Option<DateTime<Utc>> {
    if tzid.is_empty() {
        return None;
    }
    let wall = parse_form_one(s)?;
    let tz = find_vtimezone(tzid, cal)?;
    let rules = load_tz_rules(tz)?;
    let offset = FixedOffset::east_opt(select_active_offset(wall, &rules))?;
    // `from_local_datetime` is ambiguous across a fall-back transition
    // and absent across a spring-forward gap. The offset here is
    // already the one that applies, so the conversion is a plain
    // subtraction and cannot be either: it is done arithmetically so a
    // gap or overlap value resolves rather than yielding `None`, which
    // is what the tzid.json rows at the 2026 transitions assert.
    let seconds = i64::from(offset.local_minus_utc());
    Some(Utc.from_utc_datetime(&(wall - Duration::seconds(seconds))))
}

/// The VTIMEZONE in `cal` whose `TZID` property equals `tzid`.
fn find_vtimezone<'a>(tzid: &str, cal: &'a Calendar) -> Option<&'a Component> {
    cal.components
        .iter()
        .filter(|c| c.r#type.eq_fold("VTIMEZONE"))
        .find(|c| c.get("TZID").is_some_and(|p| p.value == tzid))
}

/// The subset of a `STANDARD`/`DAYLIGHT` child needed to place a
/// transition.
struct TzRule {
    /// `TZOFFSETTO` in seconds east of UTC.
    offset_to: i32,
    /// Whether the child carried an accepted `FREQ=YEARLY` RRULE.
    yearly: bool,
    /// `BYMONTH` (1–12); zero when the child carried no RRULE.
    month: u32,
    /// `BYDAY` weekday, `SU = 0` per RFC 5545 — not ISO-8601's `MO = 1`.
    weekday: u32,
    /// `BYDAY` ordinal: positive counts from the start, negative from
    /// the end.
    week: i32,
    hour: u32,
    minute: u32,
    second: u32,
}

/// The `STANDARD` plus optional `DAYLIGHT` pair extracted from a
/// VTIMEZONE.
struct TzRuleSet {
    std: TzRule,
    dst: Option<TzRule>,
}

/// Sub-components of `c` whose type matches `name`, case-insensitively.
fn subs_by_type<'a>(c: &'a Component, name: &str) -> Vec<&'a Component> {
    c.sub.iter().filter(|s| s.r#type.eq_fold(name)).collect()
}

/// Extracts the rule pair from a VTIMEZONE, or `None` for any shape
/// outside the spec's VTIMEZONE subset.
///
/// Accepted: a single `STANDARD` (fixed offset); a single `STANDARD`
/// plus a single `DAYLIGHT` where both carry an accepted `FREQ=YEARLY`
/// rule; and a `DAYLIGHT` alone, treated as a fixed offset since there
/// is no transition to compute.
///
/// Everything else fails closed. A partially applied rule would produce
/// a plausible instant that is wrong, which is worse than a fallback
/// the consumer can see.
fn load_tz_rules(tz: &Component) -> Option<TzRuleSet> {
    let standards = subs_by_type(tz, "STANDARD");
    let daylights = subs_by_type(tz, "DAYLIGHT");

    if standards.is_empty() && daylights.is_empty() {
        return None;
    }
    // Split-zone histories are outside the subset.
    if standards.len() > 1 || daylights.len() > 1 {
        return None;
    }

    let Some(std_child) = standards.first() else {
        let only = parse_tz_rule(daylights[0])?;
        return Some(TzRuleSet {
            std: only,
            dst: None,
        });
    };

    let std = parse_tz_rule(std_child)?;
    let Some(dst_child) = daylights.first() else {
        return Some(TzRuleSet { std, dst: None });
    };
    let dst = parse_tz_rule(dst_child)?;
    // With both children present the subset requires a yearly rule on
    // each.
    if !std.yearly || !dst.yearly {
        return None;
    }
    Some(TzRuleSet {
        std,
        dst: Some(dst),
    })
}

/// The offset in seconds east of UTC that applies to `wall`.
///
/// With no `DAYLIGHT` child the `STANDARD` offset applies
/// unconditionally. Otherwise both transitions are placed in `wall`'s
/// own year and compared on the same naive timeline — which orders two
/// wall events within one year correctly, and is all the comparison
/// needs.
fn select_active_offset(wall: NaiveDateTime, rs: &TzRuleSet) -> i32 {
    let Some(dst) = rs.dst.as_ref() else {
        return rs.std.offset_to;
    };
    let year = wall.year();
    let (Some(dst_start), Some(std_start)) =
        (transition_at(year, dst), transition_at(year, &rs.std))
    else {
        return rs.std.offset_to;
    };
    if wall >= dst_start && wall < std_start {
        dst.offset_to
    } else {
        rs.std.offset_to
    }
}

/// The naive instant at which `r` becomes active in `year`.
///
/// The absolute value is meaningless; only the ordering of two such
/// instants from the same year is used.
fn transition_at(year: i32, r: &TzRule) -> Option<NaiveDateTime> {
    let day = nth_weekday_of_month(year, r.month, r.weekday, r.week)?;
    NaiveDate::from_ymd_opt(year, r.month, day)?.and_hms_opt(r.hour, r.minute, r.second)
}

/// The day-of-month of the `n`th `weekday` in `(year, month)`.
///
/// Positive `n` counts from the start (1 = first); negative counts from
/// the end (-1 = last). `n = 0` is rejected at parse time.
fn nth_weekday_of_month(year: i32, month: u32, weekday: u32, n: i32) -> Option<u32> {
    // `chrono`'s `num_days_from_sunday` is `SU = 0`, matching RFC 5545
    // §3.3.10 and the `weekday` field, so no conversion is needed.
    if n > 0 {
        let first = NaiveDate::from_ymd_opt(year, month, 1)?;
        let offset = (weekday + 7 - first.weekday().num_days_from_sunday()) % 7;
        let day = 1 + offset + u32::try_from(n - 1).ok()? * 7;
        return days_in_month(year, month)
            .filter(|last| day <= *last)
            .map(|_| day);
    }
    let last = days_in_month(year, month)?;
    let last_date = NaiveDate::from_ymd_opt(year, month, last)?;
    let offset = (last_date.weekday().num_days_from_sunday() + 7 - weekday) % 7;
    let back = offset + u32::try_from(-(n + 1)).ok()? * 7;
    (back < last).then(|| last - back)
}

/// The number of days in `(year, month)`, honouring the Gregorian leap
/// rule.
fn days_in_month(year: i32, month: u32) -> Option<u32> {
    let (next_y, next_m) = if month == 12 {
        (year + 1, 1)
    } else {
        (year, month + 1)
    };
    let first = NaiveDate::from_ymd_opt(year, month, 1)?;
    let next = NaiveDate::from_ymd_opt(next_y, next_m, 1)?;
    u32::try_from((next - first).num_days()).ok()
}

/// Reads one `STANDARD`/`DAYLIGHT` child into a [`TzRule`], or `None`
/// for any field shape outside the subset.
fn parse_tz_rule(c: &Component) -> Option<TzRule> {
    let offset_to = read_offset(c, "TZOFFSETTO")?;
    // TZOFFSETFROM is not used in the arithmetic, but the subset
    // requires it to be present and well-formed — a child missing it is
    // a producer shape this implementation declines to guess at.
    read_offset(c, "TZOFFSETFROM")?;

    let wall = parse_form_one(&c.get("DTSTART")?.value)?;
    let (hour, minute, second) = (wall.hour(), wall.minute(), wall.second());

    let Some(rrule) = c.get("RRULE") else {
        return Some(TzRule {
            offset_to,
            yearly: false,
            month: 0,
            weekday: 0,
            week: 0,
            hour,
            minute,
            second,
        });
    };
    let yearly = parse_yearly_rrule(&rrule.value)?;
    Some(TzRule {
        offset_to,
        yearly: true,
        month: yearly.month,
        weekday: yearly.weekday,
        week: yearly.week,
        hour,
        minute,
        second,
    })
}

/// Reads a `±HHMM` or `±HHMMSS` UTC-offset property as seconds east of
/// UTC. Returns `None` on any shape mismatch.
fn read_offset(c: &Component, name: &str) -> Option<i32> {
    let v = &c.get(name)?.value;
    let b = v.as_bytes();
    if b.len() != 5 && b.len() != 7 {
        return None;
    }
    let sign = match b[0] {
        b'+' => 1,
        b'-' => -1,
        _ => return None,
    };
    if !b[1..].iter().all(u8::is_ascii_digit) {
        return None;
    }
    let num = |lo: usize, hi: usize| -> i32 {
        v[lo..hi]
            .bytes()
            .fold(0i32, |acc, c| acc * 10 + i32::from(c - b'0'))
    };
    let hh = num(1, 3);
    let mm = num(3, 5);
    let ss = if b.len() == 7 { num(5, 7) } else { 0 };
    Some(sign * (hh * SECONDS_PER_HOUR + mm * SECONDS_PER_MINUTE + ss))
}

/// The `BYMONTH` / `BYDAY` pair an accepted VTIMEZONE RRULE carries.
struct YearlyRule {
    month: u32,
    weekday: u32,
    week: i32,
}

/// Accepts only the VTIMEZONE RRULE subset: `FREQ=YEARLY` with optional
/// `BYMONTH`, an ordinal `BYDAY`, and a no-op `INTERVAL=1`.
///
/// Everything else fails closed — `UNTIL`, `COUNT`, `BYWEEKNO`,
/// `BYSETPOS`, `WKST`, an unknown key, a `BYDAY` without an ordinal.
///
/// This is deliberately narrow and unrelated to the generic RRULE
/// parsing scope, which lands with the recurrence layer.
fn parse_yearly_rrule(s: &str) -> Option<YearlyRule> {
    let mut freq_seen = false;
    let mut out = YearlyRule {
        month: 0,
        weekday: 0,
        week: 0,
    };

    for part in s.split(';') {
        let (key, val) = part.split_once('=')?;
        match key.to_ascii_uppercase().as_str() {
            "FREQ" => {
                if !val.eq_ignore_ascii_case("YEARLY") {
                    return None;
                }
                freq_seen = true;
            }
            "BYMONTH" => {
                let m: u32 = val.parse().ok()?;
                if !(1..=12).contains(&m) {
                    return None;
                }
                out.month = m;
            }
            "BYDAY" => {
                let (week, weekday) = parse_byday(val)?;
                out.week = week;
                out.weekday = weekday;
            }
            // A no-op INTERVAL=1 is accepted; any other value is not.
            "INTERVAL" if val == "1" => {}
            _ => return None,
        }
    }
    freq_seen.then_some(out)
}

/// Parses one `BYDAY` entry such as `2SU` or `-1SU`.
///
/// The ordinal-less forms (`SU`, `0SU`) are rejected: a VTIMEZONE
/// transition needs an explicit nth.
fn parse_byday(s: &str) -> Option<(i32, u32)> {
    if s.len() < 3 {
        return None;
    }
    let (ord, day) = s.split_at(s.len() - 2);
    let weekday = match day.to_ascii_uppercase().as_str() {
        "SU" => 0,
        "MO" => 1,
        "TU" => 2,
        "WE" => 3,
        "TH" => 4,
        "FR" => 5,
        "SA" => 6,
        _ => return None,
    };
    let week: i32 = ord.parse().ok()?;
    (week != 0).then_some((week, weekday))
}

/// The rule-5 allow-list: property names whose values are RFC 5545
/// §3.3.5 DATE-TIME and may carry a `TZID` the canonical form resolves.
///
/// `DTSTAMP` is here even though RFC 5545 §3.8.7.2 requires it to be
/// UTC: resolving defensively catches a non-conforming producer rather
/// than emitting a TZID-tagged `DTSTAMP` unchanged.
pub(crate) const DATETIME_PROPERTIES: [&str; 8] = [
    "DTSTAMP",
    "DTSTART",
    "DTEND",
    "DUE",
    "COMPLETED",
    "RECURRENCE-ID",
    "CREATED",
    "LAST-MODIFIED",
];

/// Reports whether `name` is on the rule-5 allow-list.
pub(crate) fn is_datetime_property(name: &str) -> bool {
    DATETIME_PROPERTIES
        .iter()
        .any(|p| name.eq_ignore_ascii_case(p))
}

/// Resolves a datetime-bearing property to an instant, honouring a
/// `TZID` parameter against `cal`'s VTIMEZONE registry.
///
/// `None` for a missing property, a `VALUE=DATE` property (a calendar
/// date is not an instant — read it with the date-typed accessors), or
/// a value neither form #2 nor a resolvable form #1.
fn datetime_prop(c: &Component, name: &str, cal: &Calendar) -> Option<DateTime<Utc>> {
    let p = c.get(name)?;
    if p.has_value_date() {
        return None;
    }
    if let Some(direct) = parse_time(&p.value) {
        return Some(direct);
    }
    let tzid = p.param("TZID")?;
    parse_time_with_tzid(&p.value, &tzid.value, cal)
}

impl Component {
    /// The `DTSTART` value as an instant.
    ///
    /// The calendar argument is not optional plumbing: resolving a
    /// `TZID`-bearing local time needs the enclosing calendar's
    /// VTIMEZONE registry, which lives on the [`Calendar`] and not on
    /// the [`Component`].
    pub fn dtstart(&self, cal: &Calendar) -> Option<DateTime<Utc>> {
        datetime_prop(self, "DTSTART", cal)
    }

    /// The `DTEND` value as an instant.
    pub fn dtend(&self, cal: &Calendar) -> Option<DateTime<Utc>> {
        datetime_prop(self, "DTEND", cal)
    }

    /// The VTODO `DUE` value as an instant.
    pub fn due(&self, cal: &Calendar) -> Option<DateTime<Utc>> {
        datetime_prop(self, "DUE", cal)
    }

    /// The `COMPLETED` value as an instant.
    pub fn completed(&self, cal: &Calendar) -> Option<DateTime<Utc>> {
        datetime_prop(self, "COMPLETED", cal)
    }

    /// The `DTSTAMP` value as an instant.
    ///
    /// No calendar argument: RFC 5545 §3.8.7.2 requires `DTSTAMP` to be
    /// UTC, so there is never a zone to resolve.
    pub fn dtstamp(&self) -> Option<DateTime<Utc>> {
        parse_time(self.dtstamp_raw())
    }

    /// Writes `t` as the named property in UTC form #2, or removes the
    /// property entirely when the rendered value is empty.
    ///
    /// The written property carries no parameters. Dropping any it had
    /// is required, not merely tidy: a stale `TZID` on a value now
    /// spelled in UTC would contradict the value, and a stale parameter
    /// set would make the canonical bytes depend on the property's edit
    /// history.
    fn set_time(&mut self, name: &str, t: DateTime<Utc>) {
        self.set(crate::model::Property::new(name, format_time(t)));
    }

    /// Writes `DTSTART` as a UTC form #2 instant.
    pub fn set_dtstart(&mut self, t: DateTime<Utc>) {
        self.set_time("DTSTART", t);
    }

    /// Writes `DTEND` as a UTC form #2 instant.
    pub fn set_dtend(&mut self, t: DateTime<Utc>) {
        self.set_time("DTEND", t);
    }

    /// Writes `DUE` as a UTC form #2 instant.
    pub fn set_due(&mut self, t: DateTime<Utc>) {
        self.set_time("DUE", t);
    }

    /// Writes `COMPLETED` as a UTC form #2 instant.
    pub fn set_completed(&mut self, t: DateTime<Utc>) {
        self.set_time("COMPLETED", t);
    }
}

#[cfg(test)]
mod tests {
    use super::{days_in_month, nth_weekday_of_month, parse_byday, read_offset};
    use crate::model::{Component, Property};

    #[test]
    fn nth_weekday_counts_from_both_ends() {
        // March 2026: the 1st is a Sunday, so 2SU is the 8th.
        assert_eq!(nth_weekday_of_month(2026, 3, 0, 2), Some(8));
        // November 2026: the 1st is a Sunday, so 1SU is the 1st.
        assert_eq!(nth_weekday_of_month(2026, 11, 0, 1), Some(1));
        // The last Sunday of November 2026 is the 29th.
        assert_eq!(nth_weekday_of_month(2026, 11, 0, -1), Some(29));
    }

    #[test]
    fn days_in_month_honours_the_leap_rule() {
        assert_eq!(days_in_month(2024, 2), Some(29));
        assert_eq!(days_in_month(2026, 2), Some(28));
        assert_eq!(days_in_month(2100, 2), Some(28));
        assert_eq!(days_in_month(2000, 2), Some(29));
        assert_eq!(days_in_month(2026, 12), Some(31));
    }

    #[test]
    fn byday_requires_an_explicit_ordinal() {
        assert_eq!(parse_byday("2SU"), Some((2, 0)));
        assert_eq!(parse_byday("-1SU"), Some((-1, 0)));
        assert_eq!(parse_byday("SU"), None);
        assert_eq!(parse_byday("0SU"), None);
        assert_eq!(parse_byday("1XX"), None);
    }

    #[test]
    fn offsets_accept_both_wire_widths() {
        let mut c = Component::default();
        c.add(Property::new("A", "-0500"));
        c.add(Property::new("B", "+0530"));
        c.add(Property::new("C", "-000044"));
        c.add(Property::new("D", "0500"));
        assert_eq!(read_offset(&c, "A"), Some(-18_000));
        assert_eq!(read_offset(&c, "B"), Some(19_800));
        assert_eq!(read_offset(&c, "C"), Some(-44));
        assert_eq!(read_offset(&c, "D"), None);
        assert_eq!(read_offset(&c, "MISSING"), None);
    }
}
