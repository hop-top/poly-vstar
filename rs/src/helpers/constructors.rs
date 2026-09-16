// SPDX-License-Identifier: MIT

//! The component and container constructors.

use crate::enums::Kind;
use crate::error::{Error, Result};
use crate::hashing;
use crate::model::{Calendar, Card, Component, Property};
use crate::time::format_time;
use crate::CompType;
use chrono::{DateTime, Utc};

/// The `PRODID` emitted when a caller passes an empty string to
/// [`new_calendar`]. `PRODID` survives canonicalization and is hashed,
/// so the default is version-free and language-free: the same literal
/// in every port, stable across releases. Callers wanting a custom
/// identifier supply their own.
pub(super) const DEFAULT_PROD_ID: &str = "-//hop-top//vstar//EN";

/// The wall-clock instant, truncated to seconds — V\* carries no
/// sub-second precision.
///
/// Read from [`std::time::SystemTime`] rather than `chrono::Utc::now`
/// because the crate builds `chrono` without its `clock` feature. This
/// is a *clock*, not a timezone database: the spec's "no IANA timezone
/// database" rule forbids resolving a named zone from the host, which
/// reading the epoch second count does not do.
pub(super) fn now() -> DateTime<Utc> {
    let secs = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_or(0, |d| d.as_secs() as i64);
    DateTime::<Utc>::from_timestamp(secs, 0).unwrap_or(DateTime::<Utc>::UNIX_EPOCH)
}

/// Seeds `c` with `UID` and a fresh `DTSTAMP` of now, in UTC.
pub(super) fn stamp_uid(mut c: Component, uid: &str) -> Component {
    c.set(Property::new("UID", uid));
    c.set(Property::new("DTSTAMP", format_time(now())));
    c
}

/// Rejects an empty UID with [`Error::MissingUid`].
pub(super) fn require_uid(uid: &str, what: &str) -> Result<()> {
    if uid.is_empty() {
        return Err(Error::MissingUid(format!("helpers::{what}: empty UID")));
    }
    Ok(())
}

/// A fresh VTODO with `UID`, `DTSTAMP` of now, and `DUE` set.
/// `X-VSTAR-HASH` is computed and stored last.
///
/// Returns [`Error::MissingUid`] when `uid` is empty.
pub fn new_todo(uid: &str, due: DateTime<Utc>) -> Result<Component> {
    require_uid(uid, "new_todo")?;
    let mut c = stamp_uid(Component::new(CompType::TODO), uid);
    c.set_due(due);
    hashing::set_x_vstar(&mut c);
    Ok(c)
}

/// A fresh VJOURNAL with `UID`, `DTSTAMP` of now, and `DTSTART` set.
/// `X-VSTAR-HASH` is computed and stored last.
pub fn new_journal(uid: &str, dtstart: DateTime<Utc>) -> Result<Component> {
    require_uid(uid, "new_journal")?;
    let mut c = stamp_uid(Component::new(CompType::JOURNAL), uid);
    c.set_dtstart(dtstart);
    hashing::set_x_vstar(&mut c);
    Ok(c)
}

/// A fresh VEVENT with `UID`, `DTSTAMP` of now, `DTSTART` and `DTEND`.
/// `X-VSTAR-HASH` is computed and stored last.
pub fn new_event(uid: &str, dtstart: DateTime<Utc>, dtend: DateTime<Utc>) -> Result<Component> {
    require_uid(uid, "new_event")?;
    let mut c = stamp_uid(Component::new(CompType::EVENT), uid);
    c.set_dtstart(dtstart);
    c.set_dtend(dtend);
    hashing::set_x_vstar(&mut c);
    Ok(c)
}

/// A fresh VFREEBUSY with `UID`, `DTSTAMP` of now, `DTSTART` and
/// `DTEND`. `X-VSTAR-HASH` is computed and stored last.
pub fn new_free_busy(uid: &str, dtstart: DateTime<Utc>, dtend: DateTime<Utc>) -> Result<Component> {
    require_uid(uid, "new_free_busy")?;
    let mut c = stamp_uid(Component::new(CompType::FREE_BUSY), uid);
    c.set_dtstart(dtstart);
    c.set_dtend(dtend);
    hashing::set_x_vstar(&mut c);
    Ok(c)
}

/// A fresh VALARM with `UID`, `DTSTAMP` of now, `ACTION` and a
/// `TRIGGER` taken as an unvalidated wire string. `X-VSTAR-HASH` is
/// computed and stored last.
///
/// VALARM is the one component type whose RFC 5545 schema does not
/// require `UID`, but V\* requires one on every persisted component
/// (spec/02), so an empty `uid` is still [`Error::MissingUid`].
///
/// For a trigger that cannot encode a malformed duration, use
/// [`new_relative_alarm`](super::new_relative_alarm) or
/// [`new_absolute_alarm`](super::new_absolute_alarm).
pub fn new_alarm(uid: &str, action: &str, trigger: &str) -> Result<Component> {
    require_uid(uid, "new_alarm")?;
    let mut c = stamp_uid(Component::new(CompType::ALARM), uid);
    c.set(Property::new("ACTION", action));
    c.set(Property::new("TRIGGER", trigger));
    hashing::set_x_vstar(&mut c);
    Ok(c)
}

/// A fresh VCALENDAR carrying `prod_id`. An empty string resolves to
/// the crate default.
///
/// Cannot fail — there is no input it can reject.
pub fn new_calendar(prod_id: &str) -> Calendar {
    Calendar {
        prod_id: if prod_id.is_empty() {
            DEFAULT_PROD_ID.to_owned()
        } else {
            prod_id.to_owned()
        },
        components: Vec::new(),
    }
}

/// A fresh VCARD with `VERSION`, `KIND` and `UID` properties set.
///
/// An absent `kind` defaults to [`Kind::Individual`] — and note that
/// this **writes a `KIND` property**, which `rfc6350/minimal` does not
/// carry. Code rebuilding that fixture through this constructor has to
/// clear the property afterwards.
///
/// Cards are not subject to the `X-VSTAR-HASH` discipline at the
/// constructor layer: [`hashing::card`] exists for callers
/// needing a card-level digest, but [`Card`] has no `X-VSTAR-HASH`
/// property of its own and this constructor stamps none.
///
/// Cannot fail — `Card` accepts an empty UID at this layer; the RFC
/// 6350 *encoder* is what refuses one.
pub fn new_card(uid: &str, kind: Option<Kind>) -> Card {
    let kind = kind.unwrap_or(Kind::Individual);
    let mut card = Card {
        uid: uid.to_owned(),
        kind: Some(kind),
        props: Vec::new(),
    };
    card.set(Property::new("VERSION", "4.0"));
    card.set(Property::new("KIND", kind.as_str()));
    card.set(Property::new("UID", uid));
    card
}
