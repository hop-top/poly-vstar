// SPDX-License-Identifier: MIT

//! The RFC 5545 §3.3.6 DURATION value type and the §3.8.6.3 TRIGGER
//! property on VALARM, including relative triggers and their `RELATED`
//! anchor.
//!
//! # Why a struct and not a `chrono::Duration`
//!
//! [`Duration`] preserves the units the producer authored — weeks,
//! days, hours, minutes, seconds — rather than collapsing them. A plain
//! elapsed-time count cannot represent "one day" distinctly from "24
//! hours", but RFC 5545 draws that distinction deliberately: a calendar
//! day is 23, 24 or 25 hours across a UTC-offset transition.
//! Re-serializing an elapsed count would silently rewrite `P1D` as
//! `PT24H` — and canonical form preserves DURATION values verbatim
//! (spec rule 12), so that rewrite changes the hash.
//!
//! Both views are available: [`Duration::signed`] reports the nominal
//! length as a [`chrono::Duration`] (days as 24h, weeks as 7 days), and
//! [`Duration::add_to`] anchors the value against a real instant,
//! advancing calendar days and weeks by date.
//!
//! Rust keeps the Go spelling `Duration` where TypeScript, Python and
//! PHP rename to `VDuration`: the module path (`duration::Duration`)
//! disambiguates, and `chrono::Duration` is imported under its own
//! path.

mod trigger;

use crate::{Error, Result};
use chrono::Duration as ChronoDuration;
use std::fmt;

pub use trigger::{alarm_repeat_cycle, alarm_trigger, event_end, parse_trigger, Related, Trigger};

/// An RFC 5545 §3.3.6 DURATION value in the units its producer
/// authored.
///
/// The grammar admits either a week form (weeks alone) or a
/// day-and-time form (days plus an optional hour/minute/second part);
/// the two never mix. The sign applies to the WHOLE duration, not to
/// any single field — that is what makes `-PT15M` mean "15 minutes
/// before" on a VALARM `TRIGGER`.
///
/// A duration with no non-zero unit is a valid, positive, zero-length
/// value that renders as `PT0S`, so [`Duration::default`] is a usable
/// wire value rather than an empty string or a bare `P`.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Hash)]
pub struct Duration {
    /// Whether the whole duration is subtractive.
    ///
    /// This is the authored sign flag, not the answer to "is this
    /// subtractive" — a zero-length duration can carry the flag and
    /// still not be negative. Callers want [`Duration::is_negative`].
    pub negative: bool,
    /// The `nW` count. When non-zero every other unit field is zero.
    pub weeks: i64,
    /// The `nD` count.
    pub days: i64,
    /// The `nH` count from the time part.
    pub hours: i64,
    /// The `nM` count from the time part.
    pub minutes: i64,
    /// The `nS` count from the time part.
    pub seconds: i64,
    /// Whether the value was authored in the day form.
    ///
    /// With a zero day count (`P0D`) every unit field is zero, which is
    /// indistinguishable from the zero duration — so this flag is what
    /// lets [`fmt::Display`] reproduce `P0D` rather than the canonical
    /// zero spelling `PT0S`. It is load-bearing precisely because
    /// canonical form preserves DURATION verbatim (spec rule 12).
    ///
    /// It affects formatting only: [`signed`](Duration::signed),
    /// [`is_negative`](Duration::is_negative) and
    /// [`add_to`](Duration::add_to) ignore it, and the two values are
    /// numerically equal.
    pub day_form: bool,
}

impl Duration {
    /// Whether every unit field is zero.
    const fn is_zero_length(&self) -> bool {
        self.weeks == 0
            && self.days == 0
            && self.hours == 0
            && self.minutes == 0
            && self.seconds == 0
    }

    /// The nominal length, negated when the duration is negative. Days
    /// count as 24 hours and weeks as 7 days.
    ///
    /// Exact for time-only values and for any anchor in a fixed-offset
    /// zone, UTC included. Use [`add_to`](Duration::add_to) when a real
    /// anchor is available and a day value might cross a transition.
    pub fn signed(&self) -> ChronoDuration {
        let total = ChronoDuration::weeks(self.weeks)
            + ChronoDuration::days(self.days)
            + ChronoDuration::hours(self.hours)
            + ChronoDuration::minutes(self.minutes)
            + ChronoDuration::seconds(self.seconds);
        if self.negative {
            -total
        } else {
            total
        }
    }

    /// Whether the duration is subtractive.
    ///
    /// A zero-length duration is never negative, however it was
    /// authored — `-PT0S` is zero. This, not the
    /// [`negative`](Duration::negative) field, is what
    /// `spec/behavior/duration/parse.json`'s `negative` column asserts.
    pub const fn is_negative(&self) -> bool {
        self.negative && !self.is_zero_length()
    }

    /// Advances `t` by this duration, honouring calendar semantics:
    /// weeks and days move by calendar date, the hour/minute/second
    /// part is added as elapsed time.
    ///
    /// Every V\* instant is UTC, where a calendar day is always 24
    /// hours, so the two paths coincide here. They are kept separate
    /// anyway because the distinction is the reason the authored units
    /// are preserved at all, and collapsing them here invites the
    /// collapse back into [`fmt::Display`].
    pub fn add_to(&self, t: chrono::DateTime<chrono::Utc>) -> chrono::DateTime<chrono::Utc> {
        let sign: i64 = if self.negative { -1 } else { 1 };
        let calendar = ChronoDuration::days(sign * (self.weeks * 7 + self.days));
        let clock =
            ChronoDuration::seconds(sign * (self.hours * 3600 + self.minutes * 60 + self.seconds));
        t + calendar + clock
    }
}

impl fmt::Display for Duration {
    /// Renders as an RFC 5545 §3.3.6 DURATION value, preserving the
    /// units the value carries.
    ///
    /// [`parse`] and this round-trip byte for byte, with one
    /// intentional normalization: an explicit `+` is dropped, since a
    /// positive duration is the default.
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.negative && !self.is_zero_length() {
            f.write_str("-")?;
        }
        f.write_str("P")?;

        if self.weeks != 0 {
            return write!(f, "{}W", self.weeks);
        }
        // A wholly zero duration has no unit to render, so it takes the
        // canonical zero spelling rather than a bare `P`, which parse
        // rejects.
        if self.is_zero_length() && !self.day_form {
            return f.write_str("T0S");
        }
        if self.days != 0 || self.day_form {
            write!(f, "{}D", self.days)?;
        }
        if self.hours == 0 && self.minutes == 0 && self.seconds == 0 {
            return Ok(());
        }
        f.write_str("T")?;
        if self.hours != 0 {
            write!(f, "{}H", self.hours)?;
        }
        if self.minutes != 0 {
            write!(f, "{}M", self.minutes)?;
        }
        if self.seconds != 0 {
            write!(f, "{}S", self.seconds)?;
        }
        Ok(())
    }
}

/// Builds an [`Error::Malformed`] with the module's message prefix.
fn malformed(message: impl AsRef<str>) -> Error {
    Error::Malformed(format!("duration: {}", message.as_ref()))
}

/// Decodes an RFC 5545 §3.3.6 DURATION value — no property name, no
/// parameters, e.g. `-PT15M`.
///
/// The grammar accepted is exactly:
///
/// ```text
/// dur-value  = ["+" / "-"] "P" (dur-date / dur-time / dur-week)
/// dur-date   = dur-day [dur-time]
/// dur-time   = "T" (dur-hour / dur-minute / dur-second)
/// dur-week   = 1*DIGIT "W"
/// dur-hour   = 1*DIGIT "H" [dur-minute]
/// dur-minute = 1*DIGIT "M" [dur-second]
/// dur-second = 1*DIGIT "S"
/// dur-day    = 1*DIGIT "D"
/// ```
///
/// Parsing is strict, matching [`crate::parse_time`]'s posture. Yields
/// [`Error::Malformed`] on: empty input, a missing `P`, lowercase
/// designators, weeks mixed with any other unit, time units outside the
/// `T` part, units out of RFC order, repeated units, digits with no
/// unit, an empty `T` part, ISO 8601 years or months (`P1Y`, `P1M` —
/// not in RFC 5545), fractional values, per-component signs, and any
/// whitespace.
pub fn parse(s: &str) -> Result<Duration> {
    if s.is_empty() {
        return Err(malformed("empty input"));
    }

    let mut d = Duration::default();
    let rest = match s.as_bytes()[0] {
        b'+' => &s[1..],
        b'-' => {
            d.negative = true;
            &s[1..]
        }
        _ => s,
    };
    let Some(body) = rest.strip_prefix('P') else {
        return Err(malformed(format!("{s:?} is missing its \"P\" designator")));
    };
    if body.is_empty() {
        return Err(malformed(format!("{s:?} has no value after \"P\"")));
    }

    // A bare time part: "PT…".
    if let Some(time_part) = body.strip_prefix('T') {
        parse_time_part(time_part, &mut d, s)?;
        return Ok(d);
    }

    let (date_part, time_part, has_time) = match body.split_once('T') {
        Some((date, time)) => (date, time, true),
        None => (body, "", false),
    };

    let (n, unit, remainder) = next_field(date_part, s)?;
    match unit {
        b'W' => {
            if !remainder.is_empty() {
                return Err(malformed(format!("{s:?} mixes weeks with other units")));
            }
            if has_time {
                return Err(malformed(format!("{s:?} mixes weeks with a time part")));
            }
            d.weeks = n;
        }
        b'D' => {
            if !remainder.is_empty() {
                return Err(malformed(format!(
                    "{s:?} has trailing input {remainder:?} after the day value"
                )));
            }
            d.days = n;
            d.day_form = true;
        }
        other => {
            return Err(malformed(format!(
                "{s:?} uses unit {:?} outside a time part (RFC 5545 has no years or months)",
                char::from(other)
            )))
        }
    }

    if has_time {
        parse_time_part(time_part, &mut d, s)?;
    }
    Ok(d)
}

/// Decodes the segment after `T` into the hour, minute and second
/// fields. Units appear at most once and in RFC order (H, then M, then
/// S).
fn parse_time_part(s: &str, d: &mut Duration, orig: &str) -> Result<()> {
    if s.is_empty() {
        return Err(malformed(format!("{orig:?} has an empty time part")));
    }
    // `order` tracks how far through H→M→S we have advanced, so a
    // repeated or out-of-order unit is rejected rather than silently
    // overwriting.
    let mut order = 0;
    let mut rest = s;
    while !rest.is_empty() {
        let (n, unit, remainder) = next_field(rest, orig)?;
        let rank = match unit {
            b'H' => {
                d.hours = n;
                1
            }
            b'M' => {
                d.minutes = n;
                2
            }
            b'S' => {
                d.seconds = n;
                3
            }
            other => {
                return Err(malformed(format!(
                    "{orig:?} uses unknown time unit {:?}",
                    char::from(other)
                )))
            }
        };
        if rank <= order {
            return Err(malformed(format!(
                "{orig:?} repeats or misorders time unit {:?}",
                char::from(unit)
            )));
        }
        order = rank;
        rest = remainder;
    }
    Ok(())
}

/// Consumes one `1*DIGIT UNIT` field off the front of `s`, returning
/// the value, the unit octet and the unconsumed remainder.
fn next_field<'a>(s: &'a str, orig: &str) -> Result<(i64, u8, &'a str)> {
    let b = s.as_bytes();
    let digits = b.iter().take_while(|c| c.is_ascii_digit()).count();
    if digits == 0 {
        return Err(malformed(format!("{orig:?} has a unit with no digits")));
    }
    if digits == b.len() {
        return Err(malformed(format!("{orig:?} has digits with no unit")));
    }
    let n = s[..digits].parse::<i64>().map_err(|_| {
        malformed(format!(
            "{orig:?} has an out-of-range value {:?}",
            &s[..digits]
        ))
    })?;
    Ok((n, b[digits], &s[digits + 1..]))
}

/// Reports whether `s` is a well-formed RFC 5545 §3.3.6 DURATION value.
///
/// Equivalent to discarding [`parse`]'s result, offered so a caller
/// testing a wire string need not construct an unused [`Duration`].
pub fn valid(s: &str) -> bool {
    parse(s).is_ok()
}

/// Converts a [`chrono::Duration`] into a [`Duration`] expressed in
/// hours, minutes and seconds. Sub-second precision is truncated: RFC
/// 5545 durations have second resolution.
///
/// The result never uses the week or day units — an elapsed-time count
/// carries no calendar information, so emitting `P1D` from 24 hours
/// would invent a distinction the input never made. A caller meaning
/// calendar days builds the [`Duration`] directly.
pub fn from_signed(td: ChronoDuration) -> Duration {
    let negative = td < ChronoDuration::zero();
    let mut secs = td.num_seconds().abs();
    let hours = secs / 3600;
    secs -= hours * 3600;
    let minutes = secs / 60;
    secs -= minutes * 60;
    Duration {
        negative,
        hours,
        minutes,
        seconds: secs,
        ..Duration::default()
    }
}

#[cfg(test)]
mod tests {
    use super::{from_signed, parse, valid, Duration};
    use chrono::Duration as ChronoDuration;

    #[test]
    fn the_sign_flag_and_is_negative_differ_on_a_zero_length_value() {
        let d = parse("-PT0S").expect("parses");
        assert!(d.negative, "the authored sign flag is preserved");
        assert!(!d.is_negative(), "a zero-length duration is never negative");
        assert_eq!(d.to_string(), "PT0S");
    }

    #[test]
    fn the_week_form_never_mixes_with_other_units() {
        assert!(parse("P1W").is_ok());
        assert!(parse("P1W2D").is_err());
        assert!(parse("P1WT1H").is_err());
    }

    #[test]
    fn from_signed_truncates_toward_zero() {
        assert_eq!(
            from_signed(ChronoDuration::milliseconds(1_999)).to_string(),
            "PT1S"
        );
        assert_eq!(
            from_signed(ChronoDuration::milliseconds(-1_999)).to_string(),
            "-PT1S"
        );
    }

    #[test]
    fn valid_agrees_with_parse() {
        for s in ["P1D", "PT0S", "", "P", "pt1h", "P1W"] {
            assert_eq!(valid(s), parse(s).is_ok(), "{s:?}");
        }
    }

    #[test]
    fn the_default_is_a_positive_zero() {
        let d = Duration::default();
        assert!(!d.is_negative());
        assert_eq!(d.to_string(), "PT0S");
    }
}
