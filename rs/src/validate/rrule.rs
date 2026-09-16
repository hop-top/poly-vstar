// SPDX-License-Identifier: MIT

//! spec/05 §6 — RRULE conformance.

use super::{diagnostic, Diagnostic};
use crate::generated::codes::{R_RULE_MALFORMED, R_RULE_UNSUPPORTED};
use crate::rrule::validate_rrule;
use crate::{Component, Error};

/// The RFC 5545 §3.8.5.3 property name.
const PROP_RRULE: &str = "RRULE";

/// One finding per `RRULE` property whose value fails validation.
///
/// The split is by failure class, not by message text:
///
/// - [`Error::UnsupportedRRule`] → unsupported (warning). The rule is
///   well-formed iCalendar that sits outside the RRULE parsing scope.
/// - [`Error::Malformed`] → malformed (error). The rule is not
///   well-formed at all.
///
/// Any other class routes to malformed conservatively, so an
/// unanticipated failure surfaces as a finding rather than vanishing.
pub(super) fn check(c: &Component, path: &str) -> Vec<Diagnostic> {
    let mut out = Vec::new();
    for p in &c.props {
        if !p.name.eq_ignore_ascii_case(PROP_RRULE) {
            continue;
        }
        let Err(err) = validate_rrule(&p.value) else {
            continue;
        };
        let (code, message) = match err {
            Error::UnsupportedRRule(_) => (
                R_RULE_UNSUPPORTED,
                format!(
                    "RRULE uses a feature outside the RRULE parsing scope (spec/03 §RRULE parsing scope): {err}"
                ),
            ),
            Error::Malformed(_) => (
                R_RULE_MALFORMED,
                format!("RRULE is malformed (RFC 5545 §3.3.10): {err}"),
            ),
            other => (
                R_RULE_MALFORMED,
                format!("RRULE failed validation: {other}"),
            ),
        };
        out.push(diagnostic(code, message, format!("{path}.{PROP_RRULE}")));
    }
    out
}
