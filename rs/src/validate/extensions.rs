// SPDX-License-Identifier: MIT

//! spec/05 §3, spec/04 — extension namespace compliance.

use super::{diagnostic, Diagnostic};
use crate::ext;
use crate::generated::codes::{STANDARD_PROPERTIES, UNKNOWN_PROPERTY};
use crate::Component;

/// One warning per property whose name is neither on the RFC
/// allow-list nor prefixed `X-`.
///
/// The `X-` classification is delegated to [`ext::is_extension`] so
/// there is one answer to "what counts as an extension"; the allow-list
/// stays here because it answers the orthogonal question "is this a
/// known RFC 5545/6350 property?", which `ext` has no business knowing.
pub(super) fn check(c: &Component, path: &str) -> Vec<Diagnostic> {
    let mut out = Vec::new();
    for p in &c.props {
        if is_standard(&p.name) || ext::is_extension(&p.name) {
            continue;
        }
        out.push(diagnostic(
            UNKNOWN_PROPERTY,
            format!(
                "property {} is not a known RFC 5545/6350 property and does not use the X- extension prefix (spec/05 §3, spec/04)",
                p.name
            ),
            format!("{path}.{}", p.name),
        ));
    }
    out
}

/// How many properties the generated RFC allow-list carries.
pub(super) fn standard_property_count() -> usize {
    STANDARD_PROPERTIES.len()
}

/// Whether `name` is on the generated RFC 5545/6350 allow-list.
///
/// Comparison is case-insensitive per RFC 5545 §3.1. The scan is linear
/// rather than a binary search on purpose: the generated table's order
/// is the generator's business, and a lookup that silently depends on
/// it would start returning wrong answers the day the registry emits
/// the list in a different order.
fn is_standard(name: &str) -> bool {
    STANDARD_PROPERTIES
        .iter()
        .any(|known| known.eq_ignore_ascii_case(name))
}
