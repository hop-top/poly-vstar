// SPDX-License-Identifier: MIT

//! spec/05 §2 — `X-VSTAR-HASH` integrity.

use super::{diagnostic, Diagnostic, X_VSTAR_HASH};
use crate::generated::codes::BAD_XVSTAR_HASH;
use crate::{hashing, Component};

/// Recompute `c`'s content hash and flag a stored `X-VSTAR-HASH` that
/// is present but does not match.
///
/// An absent `X-VSTAR-HASH` is deliberately **not** flagged here — that
/// case belongs to the required-common rule. The split keeps the
/// diagnostic surface unambiguous: present-but-wrong is a different bug
/// from absent, and collapsing them would tell a caller with a
/// corrupted hash to "add" a property they already have.
pub(super) fn check(c: &Component, path: &str) -> Vec<Diagnostic> {
    if c.get(X_VSTAR_HASH).is_none() {
        return Vec::new();
    }
    let (ok, want, got) = hashing::verify_x_vstar(c);
    if ok {
        return Vec::new();
    }
    vec![diagnostic(
        BAD_XVSTAR_HASH,
        format!(
            "{X_VSTAR_HASH} does not match recomputed canonical hash; want={want} got={got} (spec/05 §2)"
        ),
        format!("{path}.{X_VSTAR_HASH}"),
    )]
}
