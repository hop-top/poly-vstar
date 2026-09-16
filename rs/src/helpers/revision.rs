// SPDX-License-Identifier: MIT

//! The integer-valued properties: `SEQUENCE` (RFC 5545 §3.8.7.4),
//! `PRIORITY` (§3.8.1.9) and `PERCENT-COMPLETE` (§3.8.1.8).
//!
//! Every getter reports **absence** rather than defaulting to zero: an
//! explicit `PRIORITY:0` is the RFC's "undefined priority" and is a
//! different fact from a component carrying no `PRIORITY` at all. The
//! removers are how a caller moves from the former to the latter.

use crate::hashing;
use crate::model::{Component, Property};
use crate::CompType;

/// The `SEQUENCE` wire name.
const SEQUENCE: &str = "SEQUENCE";

/// The `PRIORITY` wire name.
const PRIORITY: &str = "PRIORITY";

/// The `PERCENT-COMPLETE` wire name.
pub(super) const PERCENT_COMPLETE: &str = "PERCENT-COMPLETE";

/// The RFC 5545 §3.8.1.9 range. `0` is a legal *value* meaning
/// "undefined" — it is not a sentinel for absence.
const PRIORITY_RANGE: std::ops::RangeInclusive<i32> = 0..=9;

/// The RFC 5545 §3.8.1.8 range.
const PERCENT_RANGE: std::ops::RangeInclusive<i32> = 0..=100;

/// The top of [`PERCENT_RANGE`], which
/// [`complete`](super::complete) writes.
pub(super) const PERCENT_MAX: i32 = 100;

/// Reads `s` as a base-10 non-negative integer with no decoration.
///
/// Rejects a leading `+`, a leading zero, and surrounding whitespace:
/// the RFC value type here is `integer` with no permitted padding, so a
/// sloppy wire value is unparseable rather than coerced. `"+3"`, `"03"`
/// and `" 3"` must not silently round-trip as `3`.
fn parse_uint(s: &str) -> Option<i32> {
    let n: i32 = s.parse().ok()?;
    (n >= 0 && n.to_string() == s).then_some(n)
}

/// Reports whether `SEQUENCE` applies to `t` — VEVENT, VTODO and
/// VJOURNAL per RFC 5545 §3.8.7.4.
fn carries_sequence(t: &CompType) -> bool {
    *t == CompType::EVENT || *t == CompType::TODO || *t == CompType::JOURNAL
}

/// Reports whether `PRIORITY` applies to `t` — VEVENT and VTODO per
/// RFC 5545 §3.8.1.9.
fn carries_priority(t: &CompType) -> bool {
    *t == CompType::EVENT || *t == CompType::TODO
}

/// The parsed `SEQUENCE` revision counter of `c`.
///
/// `None` when the property is absent or holds a value that is not a
/// canonical non-negative integer. A caller wanting the RFC default
/// reads absence as revision 0 (§3.8.7.4); this reports what the wire
/// actually holds, and type applicability is the mutators' concern.
pub fn sequence(c: &Component) -> Option<i32> {
    c.get(SEQUENCE).and_then(|p| parse_uint(&p.value))
}

/// Writes the `SEQUENCE` revision counter and refreshes
/// `X-VSTAR-HASH` last.
///
/// No-op when `c`'s type does not carry `SEQUENCE` or when `n` is
/// negative. A negative revision counter is a caller bug, and silently
/// rewriting it to 0 would advertise a revision the caller never
/// intended.
pub fn set_sequence(c: &mut Component, n: i32) {
    if !carries_sequence(&c.r#type) || n < 0 {
        return;
    }
    c.set(Property::new(SEQUENCE, n.to_string()));
    hashing::set_x_vstar(c);
}

/// Bumps `SEQUENCE` by one and refreshes `X-VSTAR-HASH` last.
///
/// "Bump the revision" is the actual use case, and doing it as a
/// read-then-write at the call site leaves a window in which a
/// concurrent mutator can interleave. An absent or unparseable
/// `SEQUENCE` is treated as the RFC default of 0, so the first
/// increment yields 1. No-op when `c`'s type does not carry it.
pub fn increment_sequence(c: &mut Component) {
    if !carries_sequence(&c.r#type) {
        return;
    }
    let next = sequence(c).unwrap_or(0) + 1;
    c.set(Property::new(SEQUENCE, next.to_string()));
    hashing::set_x_vstar(c);
}

/// The parsed `PRIORITY` of `c`, or `None` when absent or outside the
/// RFC 5545 §3.8.1.9 range 0-9.
///
/// An explicit `PRIORITY:0` returns `Some(0)` — the RFC's "undefined
/// priority" — whereas a component with none returns `None`. Use
/// [`remove_priority`] to move from the former to the latter.
pub fn priority(c: &Component) -> Option<i32> {
    c.get(PRIORITY)
        .and_then(|p| parse_uint(&p.value))
        .filter(|n| PRIORITY_RANGE.contains(n))
}

/// Writes `PRIORITY` and refreshes `X-VSTAR-HASH` last.
///
/// No-op when `c`'s type does not carry `PRIORITY` or when `n` is
/// outside 0-9. Out-of-range input is rejected, not clamped: clamping
/// an off-by-one 10 to a legitimate-looking lowest priority of 9 would
/// hide the bug in data that later round-trips cleanly. Rejection also
/// leaves any existing value untouched.
///
/// Passing 0 is not rejection — it writes the RFC's explicit
/// "undefined" marker.
pub fn set_priority(c: &mut Component, n: i32) {
    if !carries_priority(&c.r#type) || !PRIORITY_RANGE.contains(&n) {
        return;
    }
    c.set(Property::new(PRIORITY, n.to_string()));
    hashing::set_x_vstar(c);
}

/// Deletes `PRIORITY` and refreshes `X-VSTAR-HASH` last.
///
/// The counterpart to `set_priority(c, 0)`: removal means "no priority
/// stated", where 0 means "priority explicitly undefined". Unlike the
/// setter this does not gate on type — removing a property that should
/// not be there is always safe.
pub fn remove_priority(c: &mut Component) {
    c.remove(PRIORITY);
    hashing::set_x_vstar(c);
}

/// The parsed `PERCENT-COMPLETE` of `c`, or `None` when absent or
/// outside the RFC 5545 §3.8.1.8 range 0-100.
///
/// As with [`priority`], an explicit 0 — "started, nothing done" — is
/// distinct from absence and returns `Some(0)`.
pub fn percent_complete(c: &Component) -> Option<i32> {
    c.get(PERCENT_COMPLETE)
        .and_then(|p| parse_uint(&p.value))
        .filter(|n| PERCENT_RANGE.contains(n))
}

/// Writes `PERCENT-COMPLETE` and refreshes `X-VSTAR-HASH` last.
///
/// No-op when `c` is not a VTODO (§3.8.1.8 scopes the property there)
/// or when `n` is outside 0-100. Out-of-range input is rejected rather
/// than clamped, for the same reason as [`set_priority`]: clamping 120
/// to 100 would silently assert the task is finished.
///
/// Setting 100 does not by itself mark a VTODO done — use
/// [`complete`](super::complete), which also writes `STATUS` and
/// `COMPLETED`.
pub fn set_percent_complete(c: &mut Component, n: i32) {
    if c.r#type != CompType::TODO || !PERCENT_RANGE.contains(&n) {
        return;
    }
    c.set(Property::new(PERCENT_COMPLETE, n.to_string()));
    hashing::set_x_vstar(c);
}

/// Deletes `PERCENT-COMPLETE` and refreshes `X-VSTAR-HASH` last. Does
/// not gate on type — removal is always safe.
pub fn remove_percent_complete(c: &mut Component) {
    c.remove(PERCENT_COMPLETE);
    hashing::set_x_vstar(c);
}
