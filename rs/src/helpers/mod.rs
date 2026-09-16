// SPDX-License-Identifier: MIT

//! Convenience constructors and high-level mutators sitting above the
//! bare property API.
//!
//! Every helper that mutates state refreshes `X-VSTAR-HASH` **last**
//! (via [`hashing::set_x_vstar`](crate::hashing::set_x_vstar)), so
//! callers receive components whose stored hash matches their canonical
//! bytes.
//!
//! # Free functions, not methods
//!
//! These are module-level functions taking `&Component` /
//! `&mut Component` rather than inherent methods, mirroring the Go
//! reference's package-level shape:
//!
//! ```
//! # use hop_top_vstar::{CompType, Component, Property, TodoStatus};
//! # use hop_top_vstar::helpers::set_todo_status;
//! let mut c = Component::new(CompType::TODO);
//! c.set(Property::new("UID", "task-1"));
//! set_todo_status(&mut c, TodoStatus::Completed);
//! ```
//!
//! # No-op on mismatch
//!
//! A mutator whose guards fail — the wrong component type, an
//! out-of-range value, an unrecognized enum — does nothing at all: no
//! error, no half-applied mutation, and no hash churn. Out-of-range
//! input is **rejected, never clamped**: clamping a `PERCENT-COMPLETE`
//! of 120 down to 100 would silently assert the task is finished, and
//! clamping a `PRIORITY` of 10 to 9 would turn an off-by-one into
//! legitimate-looking data.

mod alarm;
mod categories;
mod classification;
mod constructors;
mod relations;
mod revision;
mod status;
mod time;

pub use alarm::{alarm_fires_at, new_absolute_alarm, new_relative_alarm};
pub use categories::{add_category, categories, set_categories};
pub use classification::{
    class_of, class_or_default, set_class, set_transp, transp, transp_or_default,
};
pub use constructors::{
    new_alarm, new_calendar, new_card, new_event, new_free_busy, new_journal, new_todo,
};
pub use relations::{add_related_to, related_to, RelatedRef};
pub use revision::{
    increment_sequence, percent_complete, priority, remove_percent_complete, remove_priority,
    sequence, set_percent_complete, set_priority, set_sequence,
};
pub use status::{
    complete, event_status, journal_status, set_event_status, set_journal_status, set_todo_status,
    todo_status,
};
pub use time::{due, set_due};
