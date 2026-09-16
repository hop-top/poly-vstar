// SPDX-License-Identifier: MIT

//! spec/02, spec/05 §1 — the required common properties.

use super::{diagnostic, Diagnostic, X_VSTAR_HASH};
use crate::generated::codes::{MISSING_DTSTAMP, MISSING_UID, MISSING_XVSTAR_HASH};
use crate::Component;

/// One error per missing required common property.
///
/// `UID`, `DTSTAMP` and `X-VSTAR-HASH` are checked in that order so the
/// diagnostic stream is deterministic. Each finding's path names the
/// absent property, not just the component: the caller is being told
/// what to add, and `...[uid=foo]` alone would not say.
pub(super) fn check(c: &Component, path: &str) -> Vec<Diagnostic> {
    let mut out = Vec::new();
    if c.get("UID").is_none() {
        out.push(diagnostic(
            MISSING_UID,
            "required common property UID is missing (spec/02)".to_owned(),
            format!("{path}.UID"),
        ));
    }
    if c.get("DTSTAMP").is_none() {
        out.push(diagnostic(
            MISSING_DTSTAMP,
            "required common property DTSTAMP is missing (spec/02)".to_owned(),
            format!("{path}.DTSTAMP"),
        ));
    }
    if c.get(X_VSTAR_HASH).is_none() {
        out.push(diagnostic(
            MISSING_XVSTAR_HASH,
            format!("required common property {X_VSTAR_HASH} is missing (spec/02)"),
            format!("{path}.{X_VSTAR_HASH}"),
        ));
    }
    out
}
