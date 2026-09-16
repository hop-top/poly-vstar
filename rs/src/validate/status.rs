// SPDX-License-Identifier: MIT

//! spec/05 §8 — the `STATUS` value domain.

use super::{diagnostic, Diagnostic};
use crate::generated::codes::STATUS_NOT_IN_VOCABULARY;
use crate::{CompType, Component, EventStatus, JournalStatus, TodoStatus};

/// The `STATUS` values RFC 5545 §3.8.1.11 scopes to `ty`, or `None`
/// when the type admits no `STATUS` vocabulary at all.
///
/// The vocabularies are per-type, not global: `CANCELLED` is the only
/// value all three share, and `DRAFT` — legal iCalendar text, and legal
/// on a VJOURNAL — is a conformance violation on a VEVENT. A type this
/// returns `None` for (VFREEBUSY, VTIMEZONE, VALARM, VCALENDAR) is
/// skipped entirely.
///
/// The values come from the crate's own wire enums rather than from the
/// generated [`STATUS_VOCABULARY`](crate::generated::codes::STATUS_VOCABULARY),
/// deliberately. The enums are what the codec encodes against, so a
/// table built from them cannot disagree with what this library writes
/// — a guarantee a lookup into a generated table would give up. The
/// registry's cross-language copy is reconciled against these enums in
/// `tests/registry_vocabulary.rs`, which is what keeps the five ports
/// agreeing without any of them losing the codec linkage.
fn vocabulary_for(ty: &CompType) -> Option<Vec<&'static str>> {
    if *ty == CompType::EVENT {
        return Some(EventStatus::ALL.iter().map(EventStatus::as_str).collect());
    }
    if *ty == CompType::TODO {
        return Some(TodoStatus::ALL.iter().map(TodoStatus::as_str).collect());
    }
    if *ty == CompType::JOURNAL {
        return Some(
            JournalStatus::ALL
                .iter()
                .map(JournalStatus::as_str)
                .collect(),
        );
    }
    None
}

/// Flag a `STATUS` whose value is outside its own component type's
/// vocabulary.
///
/// Comparison is case-insensitive per RFC 5545 §3.1. An absent `STATUS`
/// is clean — the property is optional on every type that admits it.
pub(super) fn check(c: &Component, path: &str) -> Vec<Diagnostic> {
    let Some(allowed) = vocabulary_for(&c.r#type) else {
        return Vec::new();
    };
    let Some(p) = c.get("STATUS") else {
        return Vec::new();
    };
    if allowed
        .iter()
        .any(|want| p.value.eq_ignore_ascii_case(want))
    {
        return Vec::new();
    }
    vec![diagnostic(
        STATUS_NOT_IN_VOCABULARY,
        format!(
            "STATUS value {} is not valid for {}; allowed: {} (RFC 5545 §3.8.1.11)",
            p.value,
            c.r#type.as_str(),
            allowed.join(", ")
        ),
        format!("{path}.STATUS"),
    )]
}
