// SPDX-License-Identifier: MIT

//! The RFC 5545 §3.3.4 DATE value.
//!
//! Rust keeps the Go spelling `Date` where TypeScript, Python and PHP
//! rename to `VDate`: `Date` is a built-in in all three, and the crate
//! root has no such collision — `chrono::NaiveDate` is imported under
//! its own path.

use chrono::{DateTime, Datelike, TimeZone, Utc};
use std::fmt;

/// The exact wire width of an RFC 5545 §3.3.4 DATE.
const DATE_OCTETS: usize = 8;

/// An RFC 5545 §3.3.4 DATE value: a calendar date with no time and no
/// time zone.
///
/// # Why a distinct type
///
/// DATE and DATE-TIME are semantically different, not two spellings of
/// one thing. `DUE;VALUE=DATE:20260515` means "due on the 15th, as
/// reckoned by whoever reads it"; `DUE:20260515T000000Z` means "due at
/// one specific instant, the stroke of midnight UTC". A task due on the
/// 15th is not late at 00:00:01Z; a task due at midnight UTC is.
///
/// A `DateTime` cannot carry that distinction — every one has clock
/// fields and an offset, so a date-only value stored in one is
/// indistinguishable from a midnight instant, and the caller is left
/// consulting an out-of-band flag. `Date` has no clock and no zone
/// fields at all, so the distinction cannot be lost by accident.
///
/// The zero `Date` is the "no date" sentinel: it formats as the empty
/// string, and the date-typed setters read it as "clear the property".
///
/// `month` is 1-based, matching Go's `time.Month`. It is **not** a
/// zero-based month index.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct Date {
    /// Four-digit year, 0–9999 on the wire.
    pub year: i32,
    /// Month, 1-based: January is 1.
    pub month: u32,
    /// Day of month, 1-based.
    pub day: u32,
}

impl Date {
    /// Builds a date from its fields without validating them.
    ///
    /// [`format_date`] rejects out-of-range fields at emit time rather
    /// than normalizing them, so an impossible value cannot reach the
    /// wire.
    pub const fn new(year: i32, month: u32, day: u32) -> Self {
        Date { year, month, day }
    }

    /// Reports whether this is the zero `Date` — the "no date" sentinel.
    pub const fn is_zero(&self) -> bool {
        self.year == 0 && self.month == 0 && self.day == 0
    }

    /// This date as midnight UTC, for handing to time-based arithmetic.
    ///
    /// The conversion is lossy by design and one-way: the returned value
    /// no longer records that its source was date-only. Do not round-trip
    /// a `Date` through this to store it.
    ///
    /// The zero `Date`, and any out-of-range field combination, yields
    /// the Unix epoch rather than panicking.
    pub fn to_datetime(&self) -> DateTime<Utc> {
        Utc.with_ymd_and_hms(self.year, self.month, self.day, 0, 0, 0)
            .single()
            .unwrap_or_else(|| DateTime::from_timestamp(0, 0).expect("epoch is representable"))
    }
}

impl fmt::Display for Date {
    /// The RFC 5545 §3.3.4 wire form, or `""` for the zero `Date`.
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&format_date(*self))
    }
}

/// The calendar date of `t`, as observed in UTC.
///
/// Go's `DateOf` reads the date in the value's own `Location`; a
/// `DateTime<Utc>` has exactly one, so the two agree.
pub fn date_of(t: DateTime<Utc>) -> Date {
    Date {
        year: t.year(),
        month: t.month(),
        day: t.day(),
    }
}

/// Renders `d` as an RFC 5545 §3.3.4 DATE string — `YYYYMMDD`,
/// zero-padded to eight octets.
///
/// The zero `Date` renders as the empty string, so the date-typed
/// property writers can use it to mean "clear the property".
///
/// Out-of-range field values (month 13, day 32, a year outside 0000–9999)
/// render as the empty string rather than an impossible wire form: the
/// DATE production is a fixed-width four-digit year, and emitting torn
/// data would defeat the strictness [`parse_date`] enforces. Note this
/// deliberately does **not** normalize — Feb 30 does not become Mar 2,
/// because that would hide a producer bug rather than surface it.
pub fn format_date(d: Date) -> String {
    if d.is_zero() {
        return String::new();
    }
    if !(0..=9999).contains(&d.year) || !(1..=12).contains(&d.month) || !(1..=31).contains(&d.day) {
        return String::new();
    }
    format!("{:04}{:02}{:02}", d.year, d.month, d.day)
}

/// Parses an RFC 5545 §3.3.4 DATE string (`YYYYMMDD`).
///
/// Strict by design. Specifically rejected:
///
/// - DATE-TIME forms (`YYYYMMDDTHHMMSS`, `YYYYMMDDTHHMMSSZ`).
/// - ISO 8601 extended layouts (`2026-05-15`).
/// - Impossible calendar dates (Feb 30, month 13, day 0, Feb 29 in a
///   non-leap year) — no silent roll-over.
/// - Empty strings, surrounding whitespace, extra octets, and any input
///   not exactly eight octets long.
///
/// The `None` IS the error signal; no sentinel is returned.
pub fn parse_date(s: &str) -> Option<Date> {
    if s.len() != DATE_OCTETS {
        return None;
    }
    let b = s.as_bytes();
    if !b.iter().all(u8::is_ascii_digit) {
        return None;
    }
    let num = |lo: usize, hi: usize| -> u32 {
        s[lo..hi]
            .bytes()
            .fold(0u32, |acc, c| acc * 10 + u32::from(c - b'0'))
    };
    let (year, month, day) = (num(0, 4), num(4, 6), num(6, 8));

    // chrono validates field ranges and rejects impossible dates without
    // rolling them over, which is exactly the posture the doc comment
    // promises.
    let year = i32::try_from(year).ok()?;
    chrono::NaiveDate::from_ymd_opt(year, month, day)?;

    Some(Date { year, month, day })
}
