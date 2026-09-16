// SPDX-License-Identifier: MIT

//! The RFC 5545 §3.8.1.2 `CATEGORIES` property.

use crate::hashing;
use crate::model::{Component, Property};

/// The wire name.
const CATEGORIES: &str = "CATEGORIES";

/// The comma-separated `CATEGORIES` values of `c`.
///
/// Whitespace adjacent to commas is trimmed and empty tokens — from a
/// leading or trailing comma, or an `a,,b` run — are dropped. Returns
/// an empty vector when the property is absent or holds no non-empty
/// token.
///
/// Readers preserve user input as written; comparison and dedupe
/// semantics live in [`set_categories`] and [`add_category`].
pub fn categories(c: &Component) -> Vec<String> {
    let Some(p) = c.get(CATEGORIES) else {
        return Vec::new();
    };
    p.value
        .split(',')
        .map(str::trim)
        .filter(|t| !t.is_empty())
        .map(str::to_owned)
        .collect()
}

/// Replaces `CATEGORIES` with the comma-joined `values` — no space
/// after the comma, which is the canonical wire form.
///
/// Empty input removes the property. Duplicates are dropped preserving
/// first-seen order, and comparison is **case-sensitive**: per the RFC
/// these are user-facing labels, not registry tokens, so `Work` and
/// `work` are distinct.
///
/// Refreshes `X-VSTAR-HASH` last.
pub fn set_categories(c: &mut Component, values: &[String]) {
    let deduped = dedupe_preserving_order(values);
    if deduped.is_empty() {
        c.remove(CATEGORIES);
    } else {
        c.set(Property::new(CATEGORIES, deduped.join(",")));
    }
    hashing::set_x_vstar(c);
}

/// Appends one category if not already present (case-sensitively).
///
/// An empty `value` is a no-op with no hash refresh, since nothing
/// changed. Refreshes `X-VSTAR-HASH` last otherwise.
pub fn add_category(c: &mut Component, value: &str) {
    if value.is_empty() {
        return;
    }
    let mut current = categories(c);
    if current.iter().any(|existing| existing == value) {
        return;
    }
    current.push(value.to_owned());
    c.set(Property::new(CATEGORIES, current.join(",")));
    hashing::set_x_vstar(c);
}

/// Trims each value, drops empties, and removes duplicates while
/// preserving first-seen order.
fn dedupe_preserving_order(values: &[String]) -> Vec<String> {
    let mut seen = Vec::with_capacity(values.len());
    for v in values {
        let v = v.trim();
        if v.is_empty() || seen.iter().any(|s: &String| s == v) {
            continue;
        }
        seen.push(v.to_owned());
    }
    seen
}
