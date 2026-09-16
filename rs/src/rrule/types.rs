// SPDX-License-Identifier: MIT

//! The structured form of an RRULE value: [`Rule`] and the three small
//! types its fields are made of.

use std::fmt;

use chrono::{DateTime, Utc};

/// The `FREQ` rule-part.
///
/// `SECONDLY` is deliberately absent: it is outside the scope, and the
/// parser reports it as
/// [`Error::UnsupportedRRule`](crate::Error::UnsupportedRRule) rather
/// than modelling it as a variant nothing can evaluate.
///
/// There is no `Invalid` variant either. Go needs one because its zero
/// value is a `Rule` with no `Freq`; Rust makes `Freq` a required field
/// of [`Rule`], so "unset" is unrepresentable and the check Go performs
/// at every entry point does not exist here.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Freq {
    /// `FREQ=MINUTELY`.
    Minutely,
    /// `FREQ=HOURLY`.
    Hourly,
    /// `FREQ=DAILY`.
    Daily,
    /// `FREQ=WEEKLY`.
    Weekly,
    /// `FREQ=MONTHLY`.
    Monthly,
    /// `FREQ=YEARLY`.
    Yearly,
}

impl Freq {
    /// The RFC 5545 wire token.
    pub fn as_str(&self) -> &'static str {
        match self {
            Freq::Minutely => "MINUTELY",
            Freq::Hourly => "HOURLY",
            Freq::Daily => "DAILY",
            Freq::Weekly => "WEEKLY",
            Freq::Monthly => "MONTHLY",
            Freq::Yearly => "YEARLY",
        }
    }
}

impl fmt::Display for Freq {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// An RFC 5545 §3.3.10 weekday.
///
/// The discriminants are the RFC's own numbering, `SU = 0` through
/// `SA = 6`. That is **not** ISO-8601's `MO = 1`, and the conversion to
/// a platform weekday lives in exactly one place — [`Weekday::to_chrono`]
/// — so a caller cannot reuse a platform number by accident.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord)]
#[repr(u8)]
pub enum Weekday {
    /// Sunday, `SU`.
    Su = 0,
    /// Monday, `MO`.
    Mo = 1,
    /// Tuesday, `TU`.
    Tu = 2,
    /// Wednesday, `WE`.
    We = 3,
    /// Thursday, `TH`.
    Th = 4,
    /// Friday, `FR`.
    Fr = 5,
    /// Saturday, `SA`.
    Sa = 6,
}

impl Weekday {
    /// The two-letter RFC 5545 wire symbol.
    pub fn as_str(&self) -> &'static str {
        match self {
            Weekday::Su => "SU",
            Weekday::Mo => "MO",
            Weekday::Tu => "TU",
            Weekday::We => "WE",
            Weekday::Th => "TH",
            Weekday::Fr => "FR",
            Weekday::Sa => "SA",
        }
    }

    /// The explicit conversion to [`chrono::Weekday`].
    ///
    /// This is the only place the RFC's `SU = 0` numbering meets a
    /// platform type. Everything inside this module compares
    /// `rrule::Weekday` values directly.
    pub fn to_chrono(&self) -> chrono::Weekday {
        match self {
            Weekday::Su => chrono::Weekday::Sun,
            Weekday::Mo => chrono::Weekday::Mon,
            Weekday::Tu => chrono::Weekday::Tue,
            Weekday::We => chrono::Weekday::Wed,
            Weekday::Th => chrono::Weekday::Thu,
            Weekday::Fr => chrono::Weekday::Fri,
            Weekday::Sa => chrono::Weekday::Sat,
        }
    }

    /// The inverse of [`Weekday::to_chrono`], used where the evaluator
    /// asks a date what weekday it falls on.
    pub(crate) fn from_chrono(w: chrono::Weekday) -> Self {
        match w {
            chrono::Weekday::Sun => Weekday::Su,
            chrono::Weekday::Mon => Weekday::Mo,
            chrono::Weekday::Tue => Weekday::Tu,
            chrono::Weekday::Wed => Weekday::We,
            chrono::Weekday::Thu => Weekday::Th,
            chrono::Weekday::Fri => Weekday::Fr,
            chrono::Weekday::Sat => Weekday::Sa,
        }
    }

    /// The RFC's 0-based index, `SU = 0`.
    pub(crate) fn index(&self) -> i64 {
        *self as u8 as i64
    }
}

impl fmt::Display for Weekday {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// One entry of a `BYDAY` list: an optional ordinal plus a weekday.
///
/// `ordinal == 0` means "every weekday of this kind in the containing
/// `FREQ` period" — `BYDAY=MO` under `FREQ=MONTHLY` is every Monday of
/// the month. A non-zero ordinal in `-53..=-1` or `1..=53` picks the
/// Nth, counting from the end when negative. An explicit `0` prefix on
/// the wire is invalid per RFC 5545 and the parser rejects it.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct ByDay {
    /// The `BYDAY` prefix integer, or `0` when the wire form omitted it.
    pub ordinal: i32,
    /// The weekday symbol.
    pub weekday: Weekday,
}

impl ByDay {
    /// A `BYDAY` entry. Pass `0` for the ordinal to mean "every one".
    pub fn new(ordinal: i32, weekday: Weekday) -> Self {
        Self { ordinal, weekday }
    }
}

impl fmt::Display for ByDay {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.ordinal != 0 {
            write!(f, "{}", self.ordinal)?;
        }
        f.write_str(self.weekday.as_str())
    }
}

/// An RFC 5545 §3.3.10 recurrence rule.
///
/// Field order here is the wire order [`Display`](fmt::Display) emits;
/// rule-part order on the wire is irrelevant on parse.
///
/// List fields are empty when their rule-part is absent. `until` and
/// `count` are mutually exclusive — a value carrying both is
/// [`Error::Malformed`](crate::Error::Malformed) — and `Option` makes
/// "absent" distinct from `COUNT=0`, which the RFC does not allow
/// anyway.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Rule {
    /// `FREQ`. Required; there is no unset state.
    pub freq: Freq,
    /// `INTERVAL`, defaulting to 1. Always `>= 1`.
    pub interval: u32,
    /// `UNTIL`, a UTC form #2 instant. Inclusive per RFC 5545.
    pub until: Option<DateTime<Utc>>,
    /// `COUNT`, the number of occurrences. Always `>= 1` when present.
    pub count: Option<u32>,
    /// `BYMONTH` (1..=12).
    pub by_month: Vec<i32>,
    /// `BYWEEKNO` (-53..=-1, 1..=53). `FREQ=YEARLY` only.
    pub by_week_no: Vec<i32>,
    /// `BYYEARDAY` (-366..=-1, 1..=366). `FREQ=YEARLY` only.
    pub by_year_day: Vec<i32>,
    /// `BYMONTHDAY` (-31..=-1, 1..=31).
    pub by_month_day: Vec<i32>,
    /// `BYDAY`.
    pub by_day: Vec<ByDay>,
    /// `BYHOUR` (0..=23).
    pub by_hour: Vec<i32>,
    /// `BYMINUTE` (0..=59).
    pub by_minute: Vec<i32>,
    /// `BYSECOND` (0..=60; 60 is retained for leap seconds).
    pub by_second: Vec<i32>,
    /// `BYSETPOS` (-366..=-1, 1..=366). Requires another `BY-*` clause.
    pub by_set_pos: Vec<i32>,
    /// `WKST`, defaulting to [`Weekday::Mo`].
    pub week_start: Weekday,
}

impl Rule {
    /// A rule with the given `FREQ` and every other part at its RFC
    /// default: `INTERVAL=1`, `WKST=MO`, no bound, no `BY-*` clause.
    pub fn new(freq: Freq) -> Self {
        Self {
            freq,
            interval: 1,
            until: None,
            count: None,
            by_month: Vec::new(),
            by_week_no: Vec::new(),
            by_year_day: Vec::new(),
            by_month_day: Vec::new(),
            by_day: Vec::new(),
            by_hour: Vec::new(),
            by_minute: Vec::new(),
            by_second: Vec::new(),
            by_set_pos: Vec::new(),
            week_start: Weekday::Mo,
        }
    }

    /// Whether any `BY-*` clause other than `BYSETPOS` is present — the
    /// precondition RFC 5545 §3.3.10 puts on `BYSETPOS`.
    pub(crate) fn has_other_by(&self) -> bool {
        !self.by_day.is_empty()
            || !self.by_month.is_empty()
            || !self.by_month_day.is_empty()
            || !self.by_hour.is_empty()
            || !self.by_minute.is_empty()
            || !self.by_second.is_empty()
            || !self.by_year_day.is_empty()
            || !self.by_week_no.is_empty()
    }
}
