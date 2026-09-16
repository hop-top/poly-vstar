// SPDX-License-Identifier: MIT

//! The RFC 5545 §3.8.1.11 `STATUS` property.
//!
//! The RFC scopes a distinct value vocabulary to each of VEVENT, VTODO
//! and VJOURNAL while sharing one property name, so there are three
//! getter/setter pairs here and deliberately **no** unifying `Status`
//! type — a port that adds one has diverged.
//!
//! The getters do not gate on the component type: each answers "does
//! this component carry a <TYPE>-shaped STATUS?", so a VTODO's
//! `NEEDS-ACTION` reads as absent through [`event_status`]. The setters
//! do gate, which is what keeps the three vocabularies from bleeding
//! into each other.

use crate::enums::{EventStatus, JournalStatus, TodoStatus};
use crate::hashing;
use crate::model::{Component, Property};
use crate::CompType;
use chrono::{DateTime, Utc};

/// The wire name, shared by all three vocabularies.
const STATUS: &str = "STATUS";

/// The parsed VTODO `STATUS` of `c`, or `None` when the property is
/// absent or carries a value outside the VTODO vocabulary.
///
/// Spelled `todo_status` rather than `status`: in Go the bare name is
/// historical, and every port spells all three pairs symmetrically so
/// no caller reaches for `status` expecting it to work on a VEVENT.
pub fn todo_status(c: &Component) -> Option<TodoStatus> {
    c.get(STATUS).and_then(|p| TodoStatus::parse(&p.value))
}

/// Writes the VTODO `STATUS` and refreshes `X-VSTAR-HASH` last.
///
/// No-op when `c` is not a VTODO — the four [`TodoStatus`] values are
/// VTODO-specific; VEVENT and VJOURNAL carry their own vocabularies.
pub fn set_todo_status(c: &mut Component, s: TodoStatus) {
    if c.r#type != CompType::TODO {
        return;
    }
    c.set(Property::new(STATUS, s.as_str()));
    hashing::set_x_vstar(c);
}

/// The parsed VEVENT `STATUS` of `c`, or `None` when absent or outside
/// the VEVENT vocabulary — including values legal for another component
/// type.
pub fn event_status(c: &Component) -> Option<EventStatus> {
    c.get(STATUS).and_then(|p| EventStatus::parse(&p.value))
}

/// Writes the VEVENT `STATUS` and refreshes `X-VSTAR-HASH` last. No-op
/// when `c` is not a VEVENT.
pub fn set_event_status(c: &mut Component, s: EventStatus) {
    if c.r#type != CompType::EVENT {
        return;
    }
    c.set(Property::new(STATUS, s.as_str()));
    hashing::set_x_vstar(c);
}

/// The parsed VJOURNAL `STATUS` of `c`, or `None` when absent or
/// outside the VJOURNAL vocabulary.
pub fn journal_status(c: &Component) -> Option<JournalStatus> {
    c.get(STATUS).and_then(|p| JournalStatus::parse(&p.value))
}

/// Writes the VJOURNAL `STATUS` and refreshes `X-VSTAR-HASH` last.
/// No-op when `c` is not a VJOURNAL.
pub fn set_journal_status(c: &mut Component, s: JournalStatus) {
    if c.r#type != CompType::JOURNAL {
        return;
    }
    c.set(Property::new(STATUS, s.as_str()));
    hashing::set_x_vstar(c);
}

/// Finalizes a VTODO atomically: `STATUS=COMPLETED`, `COMPLETED=t` in
/// UTC form #2, `PERCENT-COMPLETE=100`, and `X-VSTAR-HASH` refreshed
/// last.
///
/// One call yields the full set of "done" markers with a stored hash
/// that matches; doing it as three separate mutations leaves a window
/// in which the component is half-finished. No-op when `c` is not a
/// VTODO.
pub fn complete(c: &mut Component, t: DateTime<Utc>) {
    if c.r#type != CompType::TODO {
        return;
    }
    c.set(Property::new(STATUS, TodoStatus::Completed.as_str()));
    c.set_completed(t);
    c.set(Property::new(
        super::revision::PERCENT_COMPLETE,
        super::revision::PERCENT_MAX.to_string(),
    ));
    hashing::set_x_vstar(c);
}
