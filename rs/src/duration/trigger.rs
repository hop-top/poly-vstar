// SPDX-License-Identifier: MIT

//! The RFC 5545 §3.8.6.3 TRIGGER property on VALARM, its `RELATED`
//! anchor, and the §3.8.6.2 DURATION/REPEAT pair.

use super::{malformed, parse, valid, Duration};
use crate::model::{Calendar, Component, Param, Property};
use crate::time::{format_time, parse_time};
use crate::{Error, Result};
use chrono::{DateTime, Utc};
use std::fmt;

/// Property names this module reads off components.
const PROP_TRIGGER: &str = "TRIGGER";
const PROP_DURATION: &str = "DURATION";
const PROP_REPEAT: &str = "REPEAT";

/// RFC 5545 §3.2 parameter names, and the `VALUE` arguments TRIGGER
/// uses.
const PARAM_RELATED: &str = "RELATED";
const PARAM_VALUE: &str = "VALUE";
const VALUE_DURATION: &str = "DURATION";
const VALUE_DATE_TIME: &str = "DATE-TIME";

/// Which end of the parent component a relative `TRIGGER` is measured
/// from, per the RFC 5545 §3.2.14 `RELATED` parameter.
///
/// [`Related::Start`] is the default variant because it is also the RFC
/// default when the parameter is absent.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Hash)]
pub enum Related {
    /// Anchor the trigger to the parent's start (`DTSTART`).
    #[default]
    Start,
    /// Anchor the trigger to the parent's end — `DTEND`, else
    /// `DTSTART` + `DURATION`, else a VTODO's `DUE`.
    End,
}

impl Related {
    /// The RFC wire spelling.
    pub const fn as_str(&self) -> &'static str {
        match self {
            Related::Start => "START",
            Related::End => "END",
        }
    }
}

impl fmt::Display for Related {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// A parsed RFC 5545 §3.8.6.3 TRIGGER property: either a relative
/// offset from one end of the parent component, or an absolute instant.
///
/// Exactly one form is populated. When [`relative`](Trigger::relative)
/// is true, [`duration`](Trigger::duration) and
/// [`related`](Trigger::related) carry the offset and its anchor and
/// [`absolute`](Trigger::absolute) is `None`; otherwise `absolute`
/// carries the instant and `duration` is zero-length.
///
/// `absolute` is an `Option` rather than an in-band "zero instant"
/// sentinel. Go spells the absent state as the zero `time.Time`, which
/// is a real instant in year 1 that a caller can reach by accident; an
/// `Option` makes the absence a type-level fact.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Trigger {
    /// Which of the two forms this is.
    pub relative: bool,
    /// The offset, meaningful only when `relative` is true. A negative
    /// duration fires before the anchor — the common case.
    pub duration: Duration,
    /// The anchor, meaningful only when `relative` is true.
    pub related: Related,
    /// The firing instant, meaningful only when `relative` is false.
    pub absolute: Option<DateTime<Utc>>,
}

impl Default for Trigger {
    fn default() -> Self {
        Trigger {
            relative: false,
            duration: Duration::default(),
            related: Related::Start,
            absolute: None,
        }
    }
}

impl Trigger {
    /// Renders back to the wire property.
    ///
    /// A relative trigger emits its duration as the value, adding
    /// `RELATED=END` only when the anchor is the end — `RELATED=START`
    /// is the RFC default and is left implicit. An absolute trigger
    /// emits the UTC form #2 instant and carries `VALUE=DATE-TIME`
    /// explicitly, so a consumer never has to infer the form.
    pub fn to_property(&self) -> Property {
        if !self.relative {
            return Property {
                name: PROP_TRIGGER.to_owned(),
                params: vec![Param::new(PARAM_VALUE, VALUE_DATE_TIME)],
                value: self.absolute.map(format_time).unwrap_or_default(),
            };
        }
        Property {
            name: PROP_TRIGGER.to_owned(),
            params: match self.related {
                Related::End => vec![Param::new(PARAM_RELATED, Related::End.as_str())],
                Related::Start => Vec::new(),
            },
            value: self.duration.to_string(),
        }
    }

    /// The instant at which this trigger fires.
    ///
    /// An absolute trigger returns its instant and ignores both
    /// arguments. A relative trigger resolves its anchor from `parent`
    /// — `DTSTART` for [`Related::Start`]; for [`Related::End`] a
    /// VTODO's `DUE`, else the end [`event_end`] computes — then
    /// offsets it.
    ///
    /// `cal` supplies the VTIMEZONE registry used to resolve a
    /// `TZID`-bearing anchor.
    ///
    /// Yields [`Error::NoAnchor`] when the anchor the `RELATED`
    /// parameter selects is absent or unparseable. That is reported
    /// rather than silently resolving against a zero instant, which
    /// would place every such alarm at the epoch.
    pub fn resolve(&self, parent: &Component, cal: &Calendar) -> Result<DateTime<Utc>> {
        if !self.relative {
            return self.absolute.ok_or_else(|| {
                Error::NoAnchor("an absolute trigger carries no instant".to_owned())
            });
        }
        let at = self.anchor(parent, cal).ok_or_else(|| {
            Error::NoAnchor(format!(
                "{} has no {} anchor for a relative trigger",
                parent.r#type.as_str(),
                self.related
            ))
        })?;
        Ok(self.duration.add_to(at))
    }

    /// The parent instant this trigger is measured from.
    fn anchor(&self, parent: &Component, cal: &Calendar) -> Option<DateTime<Utc>> {
        if self.related == Related::Start {
            return parent.dtstart(cal);
        }
        // RELATED=END: a VTODO ends at DUE; everything else at DTEND or
        // DTSTART + DURATION.
        if parent.r#type.eq_fold("VTODO") {
            if let Some(at) = parent.due(cal) {
                return Some(at);
            }
        }
        event_end(parent, cal)
    }
}

/// Decodes a TRIGGER property.
///
/// The form is chosen as follows:
///
/// - `VALUE=DURATION`, or no `VALUE` parameter with a value that parses
///   as a duration → relative.
/// - `VALUE=DATE-TIME`, or no `VALUE` parameter with a value that
///   parses as an RFC 5545 form #2 instant → absolute.
///
/// An explicit `VALUE` parameter is **authoritative**: a value that
/// contradicts it yields [`Error::Malformed`] rather than being
/// silently re-read as the other form. With no `VALUE` parameter the
/// two shapes are unambiguous, so the value itself decides — producers
/// in the wild routinely omit the parameter.
///
/// `RELATED` is honoured on relative triggers only; RFC 5545 §3.2.14
/// scopes it to DURATION-valued triggers, so `RELATED` on an absolute
/// trigger is rejected. Parameter names and values match
/// case-insensitively per RFC 5545 §3.2.
pub fn parse_trigger(p: &Property) -> Result<Trigger> {
    let declared = p.param(PARAM_VALUE).map(|prm| prm.value.as_str());

    let mut t = match declared {
        Some(v) if v.eq_ignore_ascii_case(VALUE_DURATION) => {
            let d = parse(&p.value).map_err(|e| {
                Error::Malformed(format!(
                    "trigger: VALUE=DURATION but value is not a duration ({e})"
                ))
            })?;
            Trigger {
                relative: true,
                duration: d,
                ..Trigger::default()
            }
        }
        Some(v) if v.eq_ignore_ascii_case(VALUE_DATE_TIME) => {
            let at = parse_time(&p.value).ok_or_else(|| {
                Error::Malformed(format!(
                    "trigger: VALUE=DATE-TIME but value {:?} is not an RFC 5545 form #2 instant",
                    p.value
                ))
            })?;
            Trigger {
                absolute: Some(at),
                ..Trigger::default()
            }
        }
        Some(v) => {
            return Err(Error::Malformed(format!(
                "trigger: unsupported VALUE={v} (want DURATION or DATE-TIME)"
            )))
        }
        // No VALUE parameter — infer from the value's own shape.
        None => match parse(&p.value) {
            Ok(d) => Trigger {
                relative: true,
                duration: d,
                ..Trigger::default()
            },
            Err(_) => {
                let at = parse_time(&p.value).ok_or_else(|| {
                    Error::Malformed(format!(
                        "trigger: value {:?} is neither a DURATION nor a DATE-TIME",
                        p.value
                    ))
                })?;
                Trigger {
                    absolute: Some(at),
                    ..Trigger::default()
                }
            }
        },
    };

    let Some(related) = p.param(PARAM_RELATED) else {
        return Ok(t);
    };
    if !t.relative {
        return Err(Error::Malformed(
            "trigger: RELATED is meaningful only on a relative trigger (RFC 5545 §3.2.14)"
                .to_owned(),
        ));
    }
    t.related = if related.value.eq_ignore_ascii_case("START") {
        Related::Start
    } else if related.value.eq_ignore_ascii_case("END") {
        Related::End
    } else {
        return Err(Error::Malformed(format!(
            "trigger: unknown RELATED={} (want START or END)",
            related.value
        )));
    };
    Ok(t)
}

/// Reads and parses the TRIGGER property of a VALARM.
///
/// Yields [`Error::NoTrigger`] when the property is absent: RFC 5545
/// §3.6.6 makes TRIGGER mandatory on VALARM, so this is a producer bug
/// rather than an absent optional.
pub fn alarm_trigger(alarm: &Component) -> Result<Trigger> {
    let p = alarm
        .get(PROP_TRIGGER)
        .ok_or_else(|| Error::NoTrigger("VALARM has no TRIGGER property".to_owned()))?;
    parse_trigger(p)
}

/// The end instant of a component that expresses it either as `DTEND`
/// or as `DTSTART` plus a `DURATION`, per RFC 5545 §3.6.1 (which allows
/// exactly one of the two on a VEVENT).
///
/// `DTEND` wins when both are present — it is the explicit statement.
///
/// `None` when neither form is available, when `DTSTART` is missing for
/// the `DURATION` form, or when the `DURATION` value is malformed.
pub fn event_end(c: &Component, cal: &Calendar) -> Option<DateTime<Utc>> {
    if let Some(end) = c.dtend(cal) {
        return Some(end);
    }
    let p = c.get(PROP_DURATION)?;
    if !valid(&p.value) {
        return None;
    }
    let start = c.dtstart(cal)?;
    Some(parse(&p.value).ok()?.add_to(start))
}

/// The VALARM `DURATION`/`REPEAT` pair from RFC 5545 §3.8.6.2 and
/// §3.8.6.3: the interval between repetitions and how many additional
/// times the alarm repeats after its initial trigger.
///
/// The two properties travel together — the RFC requires that if one is
/// present the other must be. Returns a zero-length duration and zero
/// when neither is present; yields [`Error::Malformed`] when only one
/// is, when the `DURATION` value is invalid, or when `REPEAT` is not a
/// non-negative integer.
pub fn alarm_repeat_cycle(alarm: &Component) -> Result<(Duration, i32)> {
    let dur = alarm.get(PROP_DURATION);
    let rep = alarm.get(PROP_REPEAT);

    let (Some(dur), Some(rep)) = (dur, rep) else {
        return match (dur, rep) {
            (None, None) => Ok((Duration::default(), 0)),
            (Some(_), None) => Err(malformed(
                "VALARM has DURATION without REPEAT (RFC 5545 §3.8.6.2)",
            )),
            _ => Err(malformed(
                "VALARM has REPEAT without DURATION (RFC 5545 §3.8.6.2)",
            )),
        };
    };

    let d = parse(&dur.value).map_err(|e| malformed(format!("VALARM DURATION: {e}")))?;
    let repeat: i32 = rep
        .value
        .parse()
        .map_err(|_| malformed(format!("VALARM REPEAT {:?} is not an integer", rep.value)))?;
    if repeat < 0 {
        return Err(malformed(format!("VALARM REPEAT {repeat} is negative")));
    }
    Ok((d, repeat))
}

#[cfg(test)]
mod tests {
    use super::{parse_trigger, Related, Trigger};
    use crate::model::{Param, Property};

    #[test]
    fn related_defaults_to_start() {
        assert_eq!(Related::default(), Related::Start);
        assert_eq!(Trigger::default().related, Related::Start);
        assert_eq!(Related::Start.to_string(), "START");
        assert_eq!(Related::End.to_string(), "END");
    }

    #[test]
    fn an_absolute_trigger_carries_no_related() {
        let p = Property {
            name: "TRIGGER".to_owned(),
            params: vec![Param::new("RELATED", "END")],
            value: "20260601T090000Z".to_owned(),
        };
        assert!(parse_trigger(&p).is_err());
    }

    #[test]
    fn the_absent_instant_is_an_option_not_a_sentinel() {
        // A default Trigger has no instant at all, rather than one in
        // year 1 that a caller can reach by accident.
        assert_eq!(Trigger::default().absolute, None);
    }
}
