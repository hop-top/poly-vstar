// SPDX-License-Identifier: MIT

//! `CLASS` (RFC 5545 §3.8.1.3) and `TRANSP` (§3.8.2.7).
//!
//! Each has a plain getter reporting absence, an `_or_default` getter
//! applying the RFC default, and a setter. The default is applied by
//! the `_or_default` form and by nothing else.
//!
//! An absent `CLASS` reports `None` rather than `Some(Public)`, even
//! though the RFC assigns `PUBLIC` by default, for two reasons. First,
//! `Option` means "present and recognized" everywhere else in this
//! module; one getter returning `Some` for an absent property would
//! make the whole convention untrustworthy. Second, a caller that must
//! distinguish "explicitly PUBLIC" from "unset" — round-trip fidelity,
//! diffing, supersession projection — cannot recover the distinction
//! once a getter has folded it away, whereas a caller wanting the
//! effective value gets it from [`class_or_default`] at zero cost.

use crate::enums::{Class, Transp};
use crate::hashing;
use crate::model::{Component, Property};
use crate::CompType;

/// The `CLASS` wire name.
const CLASS: &str = "CLASS";

/// The `TRANSP` wire name.
const TRANSP: &str = "TRANSP";

/// Reports whether `t` admits a `CLASS` property per RFC 5545 §3.8.1.3.
fn class_applies(t: &CompType) -> bool {
    *t == CompType::EVENT || *t == CompType::TODO || *t == CompType::JOURNAL
}

/// The parsed `CLASS` of `c`, or `None` when the property is absent or
/// carries an unrecognized value.
///
/// Spelled `class_of`, not `class`, in every port: `class` is reserved
/// in TypeScript, Python and PHP, and in Rust it would shadow the
/// [`Class`] type.
pub fn class_of(c: &Component) -> Option<Class> {
    c.get(CLASS).and_then(|p| Class::parse(&p.value))
}

/// The effective `CLASS` of `c`, applying the RFC 5545 §3.8.1.3 default
/// of `PUBLIC` when the property is absent or unrecognized.
///
/// An unrecognized value falls back to the default rather than being
/// surfaced: the RFC treats an unknown token as equivalent to `PRIVATE`
/// only for IANA / `X-` registrations it cannot resolve, and V\* does
/// not model those. Flagging the value is the validate layer's job, not
/// this accessor's.
pub fn class_or_default(c: &Component) -> Class {
    class_of(c).unwrap_or(Class::Public)
}

/// Writes `CLASS` and refreshes `X-VSTAR-HASH` last. No-op when `c`'s
/// type does not admit `CLASS`.
pub fn set_class(c: &mut Component, v: Class) {
    if !class_applies(&c.r#type) {
        return;
    }
    c.set(Property::new(CLASS, v.as_str()));
    hashing::set_x_vstar(c);
}

/// The parsed `TRANSP` of `c`, or `None` when absent or unrecognized.
/// See the module docs for why absence is not reported as the RFC
/// default here.
pub fn transp(c: &Component) -> Option<Transp> {
    c.get(TRANSP).and_then(|p| Transp::parse(&p.value))
}

/// The effective `TRANSP` of `c`, applying the RFC 5545 §3.8.2.7
/// default of `OPAQUE` — the event consumes free/busy time — when the
/// property is absent or unrecognized.
pub fn transp_or_default(c: &Component) -> Transp {
    transp(c).unwrap_or(Transp::Opaque)
}

/// Writes `TRANSP` and refreshes `X-VSTAR-HASH` last. No-op when `c` is
/// not a VEVENT: `TRANSP` is VEVENT-only per RFC 5545 §3.8.2.7.
pub fn set_transp(c: &mut Component, v: Transp) {
    if c.r#type != CompType::EVENT {
        return;
    }
    c.set(Property::new(TRANSP, v.as_str()));
    hashing::set_x_vstar(c);
}
