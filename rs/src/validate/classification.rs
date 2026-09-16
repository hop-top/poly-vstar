// SPDX-License-Identifier: MIT

//! spec/05 §8 — the `CLASS` and `TRANSP` value domains.

use super::{diagnostic, Diagnostic};
use crate::generated::codes::{CLASS_NOT_IN_VOCABULARY, TRANSP_NOT_IN_VOCABULARY};
use crate::{Class, Component, Transp};

/// The closed-vocabulary property names this rule checks.
const PROP_CLASS: &str = "CLASS";
const PROP_TRANSP: &str = "TRANSP";

/// The `CLASS` values RFC 5545 §3.8.1.3 allows.
///
/// Built from the crate's own wire enum rather than the generated
/// [`CLASS_VOCABULARY`](crate::generated::codes::CLASS_VOCABULARY), for
/// the reason `status` gives: the enum is what the codec encodes
/// against, so a table built from it cannot disagree with what this
/// library writes. The registry's copy is reconciled against the enum
/// in `tests/registry_vocabulary.rs`.
fn class_vocabulary() -> Vec<&'static str> {
    Class::ALL.iter().map(Class::as_str).collect()
}

/// The `TRANSP` values RFC 5545 §3.8.2.7 allows; see
/// [`class_vocabulary`] for why the source is the enum.
fn transp_vocabulary() -> Vec<&'static str> {
    Transp::ALL.iter().map(Transp::as_str).collect()
}

/// Flag a `CLASS` outside the RFC 5545 §3.8.1.3 vocabulary and a
/// `TRANSP` outside §3.8.2.7's, one finding each.
///
/// Comparison is case-insensitive (spec/05 §8, RFC 5545 §3.1). The
/// value is checked on any component carrying the property — there is
/// no type gating, because V\* diagnoses no scope rule for any
/// property. An `X-` or IANA token on `CLASS`, which the RFC's ABNF
/// admits, is still a finding: spec/05 §8 binds the value to the three
/// registered names.
pub(super) fn check(c: &Component, path: &str) -> Vec<Diagnostic> {
    let mut out = Vec::new();
    out.extend(vocabulary_diagnostic(
        c,
        path,
        PROP_CLASS,
        &class_vocabulary(),
        CLASS_NOT_IN_VOCABULARY,
        "RFC 5545 §3.8.1.3",
    ));
    out.extend(vocabulary_diagnostic(
        c,
        path,
        PROP_TRANSP,
        &transp_vocabulary(),
        TRANSP_NOT_IN_VOCABULARY,
        "RFC 5545 §3.8.2.7",
    ));
    out
}

/// The `STATUS`-shaped diagnostic for property `name` on `c` when its
/// value is outside `allowed`; `None` when the property is absent or
/// its value is allowed.
fn vocabulary_diagnostic(
    c: &Component,
    path: &str,
    name: &str,
    allowed: &[&str],
    code: &'static str,
    reference: &str,
) -> Option<Diagnostic> {
    let p = c.get(name)?;
    if allowed
        .iter()
        .any(|want| p.value.eq_ignore_ascii_case(want))
    {
        return None;
    }
    Some(diagnostic(
        code,
        format!(
            "{name} value {} is not valid; allowed: {} ({reference})",
            p.value,
            allowed.join(", ")
        ),
        format!("{path}.{name}"),
    ))
}
