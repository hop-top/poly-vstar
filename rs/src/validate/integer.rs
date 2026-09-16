// SPDX-License-Identifier: MIT

//! spec/05 §8 — the bounded integer value domains, and the
//! canonical-decimal form they and `REPEAT` share.

use super::{diagnostic, Diagnostic};
use crate::generated::codes::INTEGER_OUT_OF_DOMAIN;
use crate::Component;

/// One bounded integer property: its name, the RFC 5545 section that
/// bounds it, and its upper bound as a digit string (`None` when
/// unbounded). The lower bound is always 0, which the canonical form
/// already enforces — it admits no sign.
struct IntegerDomain {
    name: &'static str,
    section: &'static str,
    max: Option<&'static str>,
}

/// The table the rule checks, in the order the codes catalog lists the
/// properties.
const INTEGER_DOMAINS: [IntegerDomain; 3] = [
    IntegerDomain {
        name: "PRIORITY",
        section: "§3.8.1.9",
        max: Some("9"),
    },
    IntegerDomain {
        name: "PERCENT-COMPLETE",
        section: "§3.8.1.8",
        max: Some("100"),
    },
    IntegerDomain {
        name: "SEQUENCE",
        section: "§3.8.7.4",
        max: None,
    },
];

/// Whether `s` is a canonical non-negative decimal per spec/05 §8:
/// digits only, no sign, no whitespace, and no leading zero unless `s`
/// is exactly `0`. Equivalent to the regular expression
/// `^(0|[1-9][0-9]*)$`.
///
/// The check is textual on purpose. It never converts `s` to a machine
/// integer, so a value past every integer width (a `SEQUENCE` of
/// 2^64) is well-formed — the spec bounds `SEQUENCE` below, never
/// above.
pub(super) fn is_canonical_decimal(s: &str) -> bool {
    let bytes = s.as_bytes();
    let Some(first) = bytes.first() else {
        return false;
    };
    if !bytes.iter().all(u8::is_ascii_digit) {
        return false;
    }
    bytes == b"0" || *first != b'0'
}

/// Whether the canonical decimal `v` is numerically greater than the
/// canonical decimal `max`. Both must already satisfy
/// [`is_canonical_decimal`]: with no leading zeros, a longer string is
/// a larger number and equal lengths compare lexically.
fn exceeds_digit_string(v: &str, max: &str) -> bool {
    if v.len() != max.len() {
        return v.len() > max.len();
    }
    v > max
}

/// One finding per bounded integer property whose value is not a
/// canonical decimal inside its RFC 5545 domain.
///
/// The value is checked wherever the property appears; the rule does
/// not gate on component type (`PERCENT-COMPLETE` on a VEVENT is
/// bounded, not flagged for scope). One diagnostic per offending
/// property, at the property's path, like the `STATUS` and duration
/// rules.
pub(super) fn check(c: &Component, path: &str) -> Vec<Diagnostic> {
    let mut out = Vec::new();
    for p in &c.props {
        let Some(dom) = INTEGER_DOMAINS
            .iter()
            .find(|dom| p.name.eq_ignore_ascii_case(dom.name))
        else {
            continue;
        };
        if !is_canonical_decimal(&p.value) {
            out.push(diagnostic(
                INTEGER_OUT_OF_DOMAIN,
                format!(
                    "{} is not a canonical non-negative decimal (RFC 5545 {}, spec/05 §8): {}",
                    dom.name, dom.section, p.value
                ),
                format!("{path}.{}", dom.name),
            ));
            continue;
        }
        if let Some(max) = dom.max {
            if exceeds_digit_string(&p.value, max) {
                out.push(diagnostic(
                    INTEGER_OUT_OF_DOMAIN,
                    format!(
                        "{} value {} is outside 0–{max} (RFC 5545 {})",
                        dom.name, p.value, dom.section
                    ),
                    format!("{path}.{}", dom.name),
                ));
            }
        }
    }
    out
}
