// SPDX-License-Identifier: MIT

//! V\*'s append-only state-change discipline, from spec/02 §"Status
//! supersession (append-only ledger)".
//!
//! V\* ledgers are append-only: original components MUST NOT be mutated
//! after they are appended. A status change is a fresh VJOURNAL
//! "supersession" entry that references the target via `RELATED-TO` and
//! carries `CATEGORIES:status-supersession`, an
//! `X-VSTAR-EFFECTIVE-STATUS`, and its own `X-VSTAR-HASH`.
//!
//! Two primitives:
//!
//! - [`supersedes`] constructs that journal, refusing a target whose
//!   stored hash does not verify ([`Error::TargetCorrupted`]).
//! - [`superseded`] queries a ledger for the latest supersession
//!   pointing at a component.
//!
//! V\* itself defines only the encoding; full ledger projection
//! (state-from-log) is the consumer's job per spec/02.

use crate::error::{Error, Result};
use crate::hashing;
use crate::model::{Component, Property};
use crate::time::{format_time, parse_time};
use crate::CompType;
use chrono::{DateTime, Utc};

/// The `CATEGORIES` wire string marking a VJOURNAL as a supersession
/// entry per spec/02.
///
/// Consumers projecting a ledger MUST match on this exact value,
/// case-insensitively, to identify state-change journals.
pub const CATEGORY_STATUS_SUPERSESSION: &str = "status-supersession";

/// The property name carrying the new status on a supersession
/// VJOURNAL per spec/02.
///
/// The value is opaque to V\*: the spec example shows VTODO statuses
/// but applications MAY use any vocabulary their domain demands.
///
/// # Why this lives here
///
/// Like [`hashing::X_VSTAR_HASH_PROPERTY`], the constant stays in its
/// own module and is **not** hoisted to the crate root.
/// `X-VSTAR-EFFECTIVE-STATUS` is meaningful only inside the
/// supersession pattern, and a root-level export invites callers to
/// write it onto components directly — precisely the mutation the
/// append-only discipline forbids.
pub const PROP_EFFECTIVE_STATUS: &str = "X-VSTAR-EFFECTIVE-STATUS";

/// The literal prefix every supersession UID carries. Consumers MAY
/// filter on it to enumerate supersession journals without parsing
/// `CATEGORIES`, but matching on [`CATEGORY_STATUS_SUPERSESSION`] is
/// the spec-blessed path.
const UID_PREFIX: &str = "journal:status:";

/// Constructs a fresh supersession VJOURNAL superseding `target` with
/// `status` at `t`. The target is **not** mutated.
///
/// Properties on the returned component:
///
/// - `UID` — `journal:status:<target uid>:<form #2 t>`
/// - `DTSTAMP` — `t` in form #2
/// - `RELATED-TO` — the target's UID
/// - `CATEGORIES` — [`CATEGORY_STATUS_SUPERSESSION`]
/// - [`PROP_EFFECTIVE_STATUS`] — `status`, verbatim
/// - `X-VSTAR-HASH` — computed last, so it covers everything above
///
/// # The integrity check
///
/// When `target` carries an `X-VSTAR-HASH`, this recomputes the
/// canonical hash and compares. A mismatch returns
/// [`Error::TargetCorrupted`] and nothing else is touched: writing a
/// supersession record against a component mutated since it was hashed
/// would silently attach the new status to different content.
///
/// A target with no `X-VSTAR-HASH` carries no integrity claim, so there
/// is nothing to verify and the call proceeds.
///
/// This returning a `Result` rather than an [`Option`] is contract: a
/// port that returns a nullable component here has thrown away *which*
/// failure occurred. Its counterpart [`superseded`] is the optional
/// shape, and the asymmetry is deliberate.
pub fn supersedes(target: &Component, status: &str, t: DateTime<Utc>) -> Result<Component> {
    if hashing::get_x_vstar(target).is_some() {
        let (ok, want, got) = hashing::verify_x_vstar(target);
        if !ok {
            return Err(Error::TargetCorrupted(format!(
                "target {:?} hash {got} does not match canonical form {want}",
                target.uid()
            )));
        }
    }

    let stamp = format_time(t);
    let mut c = Component::new(CompType::JOURNAL);
    c.set(Property::new(
        "UID",
        format!("{UID_PREFIX}{}:{stamp}", target.uid()),
    ));
    c.set(Property::new("DTSTAMP", &stamp));
    c.set(Property::new("RELATED-TO", target.uid()));
    c.set(Property::new("CATEGORIES", CATEGORY_STATUS_SUPERSESSION));
    c.set(Property::new(PROP_EFFECTIVE_STATUS, status));

    // The hash refresh MUST be the last mutation: `set_x_vstar` strips
    // any existing value before computing, so the stored hash covers
    // every property set above.
    hashing::set_x_vstar(&mut c);
    Ok(c)
}

/// The effective status `ledger` projects onto `c`, or `None` when
/// nothing supersedes it.
///
/// A match is a VJOURNAL whose `RELATED-TO` equals `c`'s UID, whose
/// `CATEGORIES` contains [`CATEGORY_STATUS_SUPERSESSION`], and which
/// carries [`PROP_EFFECTIVE_STATUS`]. The latest entry by parsed
/// `DTSTAMP` wins; on a tie the one later in the ledger wins, since the
/// scan is stable and in order.
///
/// `None` when the ledger is empty, when `c` has no UID to match
/// against, when nothing points at it, or when matching entries carry
/// no effective-status property.
///
/// # Not a validator
///
/// An entry whose `DTSTAMP` will not parse sorts to the zero time and
/// is effectively skipped under "latest wins". Asking "has this been
/// superseded?" has exactly two honest answers and "no" is not a
/// failure, so this returns an [`Option`] and never an error — the
/// counterpart asymmetry to [`supersedes`].
pub fn superseded(c: &Component, ledger: &[Component]) -> Option<String> {
    let target_uid = c.uid();
    if target_uid.is_empty() {
        return None;
    }

    let mut best: Option<(DateTime<Utc>, String)> = None;
    for entry in ledger {
        if entry.r#type != CompType::JOURNAL {
            continue;
        }
        if entry.get("RELATED-TO").map(|p| p.value.as_str()) != Some(target_uid) {
            continue;
        }
        if !categories_contain_supersession(entry) {
            continue;
        }
        let Some(status) = entry.get(PROP_EFFECTIVE_STATUS) else {
            continue;
        };

        let when = entry_dtstamp(entry);
        // `>=` picks the later ledger position on a tie, since the scan
        // runs in order.
        if best.as_ref().is_none_or(|(b, _)| when >= *b) {
            best = Some((when, status.value.clone()));
        }
    }
    best.map(|(_, status)| status)
}

/// Reports whether `c` carries a `CATEGORIES` property containing
/// [`CATEGORY_STATUS_SUPERSESSION`].
///
/// RFC 5545 §3.8.1.2 makes `CATEGORIES` comma-delimited, so each token
/// is trimmed and compared case-insensitively rather than
/// substring-matched against the raw value — otherwise a label like
/// `status-supersession-deferred` would falsely match.
fn categories_contain_supersession(c: &Component) -> bool {
    c.get_all("CATEGORIES").into_iter().any(|p| {
        p.value.split(',').any(|tok| {
            tok.trim()
                .eq_ignore_ascii_case(CATEGORY_STATUS_SUPERSESSION)
        })
    })
}

/// The parsed `DTSTAMP` of `c`, or the Unix epoch when it is missing or
/// unparseable — "sorts to the start", so ledger noise is demoted
/// rather than fatal.
fn entry_dtstamp(c: &Component) -> DateTime<Utc> {
    parse_time(c.dtstamp_raw()).unwrap_or(DateTime::<Utc>::UNIX_EPOCH)
}
