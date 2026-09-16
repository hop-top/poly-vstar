// SPDX-License-Identifier: MIT

//! The RRULE wire form.
//!
//! Canonical form preserves an `RRULE` value byte-for-byte (spec rule
//! 8), so this is the emitter's contract, not the canonical layer's:
//! two rules with the same logical content produce the same bytes only
//! because the emitter fixes an order and elides the defaults.
//!
//! The order is RFC 5545 §3.3.10's own `recur` ABNF order:
//!
//! ```text
//! FREQ, INTERVAL, UNTIL, COUNT, BYMONTH, BYWEEKNO, BYYEARDAY,
//! BYMONTHDAY, BYDAY, BYHOUR, BYMINUTE, BYSECOND, BYSETPOS, WKST
//! ```
//!
//! `INTERVAL=1` and `WKST=MO` are the RFC defaults and are omitted.
//!
//! **List values keep their authored order.** RFC 5545 gives `BY-*`
//! lists no ordering semantics, so sorting them would change bytes the
//! producer chose and break the round-trip. Callers wanting
//! order-insensitive equality compare parsed [`Rule`]s, not strings.

use std::fmt;

use crate::rrule::types::{ByDay, Rule, Weekday};
use crate::{format_time, Property};

impl fmt::Display for Rule {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "FREQ={}", self.freq)?;

        // INTERVAL — elided at its RFC default of 1.
        if self.interval > 1 {
            write!(f, ";INTERVAL={}", self.interval)?;
        }
        // UNTIL and COUNT are mutually exclusive; emit whichever is set.
        if let Some(until) = self.until {
            write!(f, ";UNTIL={}", format_time(until))?;
        }
        if let Some(count) = self.count {
            write!(f, ";COUNT={count}")?;
        }

        write_int_list(f, "BYMONTH", &self.by_month)?;
        write_int_list(f, "BYWEEKNO", &self.by_week_no)?;
        write_int_list(f, "BYYEARDAY", &self.by_year_day)?;
        write_int_list(f, "BYMONTHDAY", &self.by_month_day)?;
        write_by_day_list(f, &self.by_day)?;
        write_int_list(f, "BYHOUR", &self.by_hour)?;
        write_int_list(f, "BYMINUTE", &self.by_minute)?;
        write_int_list(f, "BYSECOND", &self.by_second)?;
        write_int_list(f, "BYSETPOS", &self.by_set_pos)?;

        // WKST — elided at its RFC default of MO.
        if self.week_start != Weekday::Mo {
            write!(f, ";WKST={}", self.week_start)?;
        }
        Ok(())
    }
}

impl Rule {
    /// The rule as a complete `RRULE` [`Property`], ready to attach to a
    /// component.
    ///
    /// ```
    /// use hop_top_vstar::rrule::parse_rrule;
    ///
    /// let p = parse_rrule("FREQ=DAILY;INTERVAL=2")?.to_property();
    /// assert_eq!(p.name, "RRULE");
    /// assert_eq!(p.value, "FREQ=DAILY;INTERVAL=2");
    /// assert!(p.params.is_empty());
    /// # Ok::<(), hop_top_vstar::Error>(())
    /// ```
    pub fn to_property(&self) -> Property {
        Property::new("RRULE", self.to_string())
    }
}

/// Writes `;NAME=v1,v2` for a non-empty list, in the order held.
fn write_int_list(f: &mut fmt::Formatter<'_>, name: &str, vals: &[i32]) -> fmt::Result {
    if vals.is_empty() {
        return Ok(());
    }
    write!(f, ";{name}=")?;
    for (i, v) in vals.iter().enumerate() {
        if i > 0 {
            f.write_str(",")?;
        }
        write!(f, "{v}")?;
    }
    Ok(())
}

/// Writes `;BYDAY=[<ordinal>]<weekday>,…`. A zero ordinal renders with
/// no prefix — the explicit `0` prefix is invalid per RFC 5545.
fn write_by_day_list(f: &mut fmt::Formatter<'_>, vals: &[ByDay]) -> fmt::Result {
    if vals.is_empty() {
        return Ok(());
    }
    f.write_str(";BYDAY=")?;
    for (i, bd) in vals.iter().enumerate() {
        if i > 0 {
            f.write_str(",")?;
        }
        write!(f, "{bd}")?;
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use crate::rrule::parse::parse_rrule;
    use crate::rrule::types::{Freq, Rule, Weekday};

    #[test]
    fn elides_the_rfc_defaults() {
        assert_eq!(Rule::new(Freq::Daily).to_string(), "FREQ=DAILY");
        let mut r = Rule::new(Freq::Daily);
        r.interval = 2;
        r.week_start = Weekday::Su;
        assert_eq!(r.to_string(), "FREQ=DAILY;INTERVAL=2;WKST=SU");
    }

    #[test]
    fn keeps_list_order_as_authored() {
        let r = parse_rrule("FREQ=WEEKLY;BYDAY=SA,MO,WE").expect("parses");
        assert_eq!(r.to_string(), "FREQ=WEEKLY;BYDAY=SA,MO,WE");
    }

    #[test]
    fn re_emitting_is_idempotent() {
        for src in [
            "FREQ=DAILY",
            "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE",
            "FREQ=MONTHLY;BYMONTHDAY=-1;WKST=SU",
            "FREQ=DAILY;UNTIL=20260401T120000Z",
        ] {
            let once = parse_rrule(src).expect("parses").to_string();
            let twice = parse_rrule(&once).expect("re-parses").to_string();
            assert_eq!(once, twice, "{src}");
        }
    }
}
