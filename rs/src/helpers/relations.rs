// SPDX-License-Identifier: MIT

//! The RFC 5545 §3.8.4.5 `RELATED-TO` property and its §3.2.15
//! `RELTYPE` parameter.

use crate::enums::{parse_rel_type, RelType, DEFAULT_REL_TYPE};
use crate::hashing;
use crate::model::{Component, Param, Property};

/// The wire name.
const RELATED_TO: &str = "RELATED-TO";

/// The parameter name. Its default when omitted is `PARENT`.
const RELTYPE: &str = "RELTYPE";

/// One parsed `RELATED-TO` property: the referenced UID and its
/// relationship type.
///
/// `rel_type` carries a registered value from the [`RelType`]
/// vocabulary folded to canonical case, or an unregistered value — an
/// `X-` extension, say — verbatim. Validating it against the spec
/// vocabulary is the validate layer's job; this one accepts any
/// non-empty value.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RelatedRef {
    /// The referenced component's UID — the property value.
    pub uid: String,
    /// The `RELTYPE`, defaulted to `PARENT` when the parameter is
    /// absent.
    pub rel_type: RelType,
}

/// Every `RELATED-TO` property on `c`, parsed, in property order.
///
/// `RELTYPE` is read case-insensitively per RFC 5545 §3.2 and folded to
/// its canonical spelling; an absent parameter defaults to `PARENT` per
/// §3.2.15. Unregistered values pass through verbatim.
///
/// Returns an empty vector when there are no `RELATED-TO` properties.
pub fn related_to(c: &Component) -> Vec<RelatedRef> {
    c.get_all(RELATED_TO)
        .into_iter()
        .map(|p| {
            let rel_type = p
                .params
                .iter()
                .find(|par| par.name.eq_ignore_ascii_case(RELTYPE) && !par.value.is_empty())
                .map_or_else(|| DEFAULT_REL_TYPE, |par| parse_rel_type(&par.value).0);
            RelatedRef {
                uid: p.value.clone(),
                rel_type,
            }
        })
        .collect()
}

/// Appends a `RELATED-TO` property with value `uid` and the given
/// `rel_type`.
///
/// An empty `rel_type` omits the parameter entirely — consumers then
/// see the RFC default of `PARENT` through [`related_to`]. Any value is
/// accepted, including `X-` extensions.
///
/// This **appends**; it does not replace an existing `RELATED-TO`, so a
/// component can carry several relationships. No-op when `uid` is
/// empty. Refreshes `X-VSTAR-HASH` last.
pub fn add_related_to(c: &mut Component, uid: &str, rel_type: RelType) {
    if uid.is_empty() {
        return;
    }
    let mut prop = Property::new(RELATED_TO, uid);
    if !rel_type.as_str().is_empty() {
        prop.params.push(Param::new(RELTYPE, rel_type.as_str()));
    }
    c.add(prop);
    hashing::set_x_vstar(c);
}
