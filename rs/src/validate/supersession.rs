// SPDX-License-Identifier: MIT

//! spec/02, spec/05 §4 — supersession discipline.

use super::{diagnostic, Diagnostic};
use crate::generated::codes::{SUPERSESSION_MISSING_PROPS, SUPERSESSION_ORPHAN};
use crate::supersession::{CATEGORY_STATUS_SUPERSESSION, PROP_EFFECTIVE_STATUS};
use crate::{CompType, Component};

/// Flag a supersession VJOURNAL that violates spec/05 §4.
///
/// `ledger` supplies the cross-component context: the orphan rule
/// resolves `RELATED-TO` against every component in the same calendar.
/// When it is `None` the caller is the single-component entry point,
/// which sees one component and so cannot make the claim "this target
/// does not exist" — the orphan rule is skipped rather than guessed.
/// The missing-property rule still fires either way: it needs nothing
/// but the component itself.
pub(super) fn check(c: &Component, ledger: Option<&[Component]>, path: &str) -> Vec<Diagnostic> {
    if c.r#type != CompType::JOURNAL || !categories_contain_supersession(c) {
        return Vec::new();
    }

    let mut out = Vec::new();
    let related = c.get("RELATED-TO");

    if !is_present(related.map(|p| p.value.as_str())) {
        out.push(diagnostic(
            SUPERSESSION_MISSING_PROPS,
            "supersession VJOURNAL missing RELATED-TO (spec/02, spec/05 §4)".to_owned(),
            format!("{path}.RELATED-TO"),
        ));
    }

    if !is_present(c.get(PROP_EFFECTIVE_STATUS).map(|p| p.value.as_str())) {
        out.push(diagnostic(
            SUPERSESSION_MISSING_PROPS,
            format!("supersession VJOURNAL missing {PROP_EFFECTIVE_STATUS} (spec/02, spec/05 §4)"),
            format!("{path}.{PROP_EFFECTIVE_STATUS}"),
        ));
    }

    if let (Some(rel), Some(ledger)) = (related, ledger) {
        let target = rel.value.trim();
        if !target.is_empty() && !ledger_contains_uid(ledger, target) {
            out.push(diagnostic(
                SUPERSESSION_ORPHAN,
                format!(
                    "supersession VJOURNAL RELATED-TO={target} has no matching component in calendar (spec/02, spec/05 §4)"
                ),
                format!("{path}.RELATED-TO"),
            ));
        }
    }

    out
}

/// Whether a property is present *and* carries a non-blank value.
///
/// A present-but-empty `RELATED-TO` is as broken as an absent one — it
/// names no target — so both route to the same finding.
fn is_present(value: Option<&str>) -> bool {
    value.is_some_and(|v| !v.trim().is_empty())
}

/// Whether `c` carries a `CATEGORIES` value containing the supersession
/// category, comma-separated and case-insensitive.
fn categories_contain_supersession(c: &Component) -> bool {
    c.get_all("CATEGORIES").iter().any(|p| {
        p.value.split(',').any(|tok| {
            tok.trim()
                .eq_ignore_ascii_case(CATEGORY_STATUS_SUPERSESSION)
        })
    })
}

/// Whether any component in `ledger` has `uid` as its UID.
///
/// Comparison is case-sensitive per RFC 5545 §3.8.4.7: a UID is an
/// opaque identifier, not a human-facing name.
fn ledger_contains_uid(ledger: &[Component], uid: &str) -> bool {
    ledger.iter().any(|c| c.uid() == uid)
}
