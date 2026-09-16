// SPDX-License-Identifier: MIT

//! spec/05 §7 — DURATION well-formedness.

use super::integer::is_canonical_decimal;
use super::{diagnostic, Diagnostic};
use crate::duration;
use crate::generated::codes::MALFORMED_DURATION;
use crate::Component;

/// The duration-bearing property names this rule checks.
const PROP_DURATION: &str = "DURATION";
const PROP_TRIGGER: &str = "TRIGGER";
const PROP_REPEAT: &str = "REPEAT";

/// One finding per duration-bearing property whose value is malformed.
///
/// `TRIGGER` is routed through [`duration::parse_trigger`] rather than
/// the bare value parser so the `RELATED` / `VALUE` parameter rules are
/// enforced with the value: an absolute trigger carrying `RELATED`, or
/// a `VALUE` that contradicts the value it labels, is malformed even
/// though the value itself parses. `DURATION` and `REPEAT` are checked
/// directly — each has only the one legal form.
///
/// `REPEAT` takes the same textual canonical-decimal rule spec/05 §8
/// applies to the bounded integers: a sign or a leading zero is
/// malformed even though the number is in range. It stays under the
/// duration code because a published code never changes meaning.
pub(super) fn check(c: &Component, path: &str) -> Vec<Diagnostic> {
    let mut out = Vec::new();
    for p in &c.props {
        if p.name.eq_ignore_ascii_case(PROP_TRIGGER) {
            if let Err(err) = duration::parse_trigger(p) {
                out.push(diagnostic(
                    MALFORMED_DURATION,
                    format!("TRIGGER is malformed (RFC 5545 §3.8.6.3): {err}"),
                    format!("{path}.{PROP_TRIGGER}"),
                ));
            }
        } else if p.name.eq_ignore_ascii_case(PROP_DURATION) {
            if let Err(err) = duration::parse(&p.value) {
                out.push(diagnostic(
                    MALFORMED_DURATION,
                    format!("DURATION is malformed (RFC 5545 §3.3.6): {err}"),
                    format!("{path}.{PROP_DURATION}"),
                ));
            }
        } else if p.name.eq_ignore_ascii_case(PROP_REPEAT) && !is_canonical_decimal(&p.value) {
            out.push(diagnostic(
                MALFORMED_DURATION,
                format!(
                    "REPEAT is not a canonical non-negative integer (RFC 5545 §3.8.6.2, spec/05 §8): {}",
                    p.value
                ),
                format!("{path}.{PROP_REPEAT}"),
            ));
        }
    }
    out
}
