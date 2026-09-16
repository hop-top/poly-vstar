// SPDX-License-Identifier: MIT

//! The VTODO `DUE` property, wrapped so the reader and the writer sit
//! side by side in one module.

use crate::hashing;
use crate::model::{Calendar, Component};
use chrono::{DateTime, Utc};

/// The `DUE` instant of `c`, resolved against `cal`'s VTIMEZONE
/// registry when the property carries a `TZID`.
///
/// A thin wrapper over [`Component::due`] that funnels callers through
/// this module so [`set_due`] has a matching reader.
pub fn due(c: &Component, cal: &Calendar) -> Option<DateTime<Utc>> {
    c.due(cal)
}

/// Writes `DUE` in UTC form #2 and refreshes `X-VSTAR-HASH` last.
pub fn set_due(c: &mut Component, t: DateTime<Utc>) {
    c.set_due(t);
    hashing::set_x_vstar(c);
}
