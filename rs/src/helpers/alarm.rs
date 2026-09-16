// SPDX-License-Identifier: MIT

//! Typed VALARM constructors and the alarm-firing resolver.
//!
//! [`new_alarm`](super::new_alarm) takes its trigger as an unvalidated
//! wire string; the two constructors here take typed values that cannot
//! encode a malformed duration or an ambiguous instant.

use super::constructors::{require_uid, stamp_uid};
use crate::duration::{alarm_trigger, Duration, Related, Trigger};
use crate::error::Result;
use crate::hashing;
use crate::model::{Calendar, Component, Property};
use crate::CompType;
use chrono::{DateTime, Utc};

/// The RFC 5545 §3.8.6.1 property name.
const ACTION: &str = "ACTION";

/// A fresh VALARM whose `TRIGGER` is a relative offset from one end of
/// its parent — the overwhelmingly common alarm shape, and the one
/// Apple Reminders and Thunderbird emit.
///
/// A negative offset fires *before* the anchor, so a fifteen-minute
/// warning is:
///
/// ```
/// use hop_top_vstar::duration::{from_signed, Related};
/// use hop_top_vstar::helpers::new_relative_alarm;
///
/// let alarm = new_relative_alarm(
///     "alarm-1",
///     "DISPLAY",
///     from_signed(chrono::Duration::minutes(-15)),
///     Related::Start,
/// )?;
/// assert_eq!(alarm.get("TRIGGER").map(|p| p.value.as_str()), Some("-PT15M"));
/// # Ok::<(), hop_top_vstar::Error>(())
/// ```
///
/// [`Related::Start`] is the RFC default and stays implicit in the
/// emitted property; [`Related::End`] emits `RELATED=END`.
///
/// `UID`, `DTSTAMP` of now and `ACTION` are set as for
/// [`new_alarm`](super::new_alarm), and `X-VSTAR-HASH` is computed
/// last. Returns [`Error::MissingUid`](crate::Error::MissingUid) when
/// `uid` is empty.
pub fn new_relative_alarm(
    uid: &str,
    action: &str,
    offset: Duration,
    related: Related,
) -> Result<Component> {
    require_uid(uid, "new_relative_alarm")?;
    let trigger = Trigger {
        relative: true,
        duration: offset,
        related,
        absolute: None,
    };
    Ok(alarm_with_trigger(uid, action, trigger.to_property()))
}

/// A fresh VALARM whose `TRIGGER` is a fixed instant rather than an
/// offset.
///
/// The emitted property carries `VALUE=DATE-TIME` explicitly so a
/// consumer never infers the form, and the instant is written in UTC
/// form #2.
///
/// `UID`, `DTSTAMP` of now and `ACTION` are set as for
/// [`new_alarm`](super::new_alarm), and `X-VSTAR-HASH` is computed
/// last. Returns [`Error::MissingUid`](crate::Error::MissingUid) when
/// `uid` is empty.
pub fn new_absolute_alarm(uid: &str, action: &str, at: DateTime<Utc>) -> Result<Component> {
    require_uid(uid, "new_absolute_alarm")?;
    let trigger = Trigger {
        relative: false,
        duration: Duration::default(),
        related: Related::Start,
        absolute: Some(at),
    };
    Ok(alarm_with_trigger(uid, action, trigger.to_property()))
}

/// Assembles a VALARM around an already-rendered `TRIGGER`, applying
/// the shared UID/DTSTAMP/ACTION seeding and the hash-last discipline.
fn alarm_with_trigger(uid: &str, action: &str, trigger: Property) -> Component {
    let mut c = stamp_uid(Component::new(CompType::ALARM), uid);
    c.set(Property::new(ACTION, action));
    c.set(trigger);
    hashing::set_x_vstar(&mut c);
    c
}

/// The instant at which `alarm` fires, given the `parent` component it
/// hangs off and that parent's `cal`.
///
/// A relative trigger is offset from the anchor its `RELATED` parameter
/// selects — `DTSTART` for `START`, and for `END` the parent's `DTEND`,
/// `DTSTART` plus `DURATION`, or a VTODO's `DUE`. Calendar days and
/// weeks advance by date, so an offset crossing a DST transition keeps
/// its wall-clock meaning. An absolute trigger returns its instant
/// directly.
///
/// `cal` supplies the VTIMEZONE registry for TZID-bearing anchors; pass
/// the default [`Calendar`] when the parent's times are plain UTC.
///
/// # Errors
///
/// [`Error::NoTrigger`](crate::Error::NoTrigger) when the VALARM has no
/// `TRIGGER`, [`Error::NoAnchor`](crate::Error::NoAnchor) when the
/// parent lacks the required anchor, and
/// [`Error::Malformed`](crate::Error::Malformed) when the `TRIGGER`
/// value does not parse.
pub fn alarm_fires_at(
    alarm: &Component,
    parent: &Component,
    cal: &Calendar,
) -> Result<DateTime<Utc>> {
    alarm_trigger(alarm)?.resolve(parent, cal)
}
