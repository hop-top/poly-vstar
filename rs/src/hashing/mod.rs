// SPDX-License-Identifier: MIT

//! SHA-256 content hashes of V\* objects, in the `sha256:<hex>` form
//! the specification mandates.
//!
//! Hashes are computed over the canonical byte form, so two
//! implementations that agree on canonical bytes produce identical
//! hashes. That is the whole point: the hash is the cheap,
//! transportable proof that two documents are the same logical content,
//! and it is only as good as the byte agreement underneath it.
//!
//! The `sha256:` prefix is part of the value, not decoration. It exists
//! so a future `sha3-256:` or `blake3:` is expressible without
//! ambiguity; v0.1 emits only `sha256:`.
//!
//! # Hash exclusion
//!
//! Rule 7 excludes `X-VSTAR-HASH` from the bytes its own value is
//! computed over — otherwise the stored hash would feed back into its
//! own digest. The canonical layer strips it per its own contract, and
//! these functions strip it again. The redundancy is deliberate: it
//! documents the invariant at the API boundary, so a reader of this
//! module does not have to go and confirm the canonical layer's
//! behaviour.
//!
//! Every function here is pure except [`set_x_vstar`], the one writer.

use crate::canonical;
use crate::model::{Calendar, Card, Component, Property};
use sha2::{Digest, Sha256};

/// The property name V\* uses to carry the content hash.
///
/// This constant lives in `hashing` and stays there. It is not hoisted
/// to the crate root in any port, even though the root is where
/// [`Property`] lives and the constant names a property: the string is
/// meaningful only in company with the hash functions that write and
/// read it, and the canonical layer's rule-7 exclusion of that property
/// is a hashing concern. Callers write `hashing::X_VSTAR_HASH_PROPERTY`
/// in every language.
pub const X_VSTAR_HASH_PROPERTY: &str = "X-VSTAR-HASH";

/// The algorithm tag every v0.1 hash carries.
const SHA256_PREFIX: &str = "sha256:";

/// The `sha256:<hex>` digest of the canonical byte form of `c`.
///
/// This delegates to the context-free canonical form, not the
/// context-taking one. A component carrying `TZID`-tagged datetimes
/// therefore hashes over wire-form bytes: two timezone spellings of the
/// same logical instant hash differently. For calendar-aware hashing
/// that resolves `TZID`s against a VTIMEZONE registry, hash the whole
/// calendar with [`calendar`].
///
/// The asymmetry mirrors the canonical layer's own, and it is correct:
/// a component without a parent calendar has no registry to consult.
pub fn component(c: &Component) -> String {
    digest(&canonical::component(&strip_component(c)))
}

/// The `sha256:<hex>` digest of the canonical byte form of `cal`.
///
/// `X-VSTAR-HASH` is stripped at every depth — top-level components and
/// their sub-components alike — before the canonical pass.
/// `TZID`-tagged datetimes resolve against the calendar's own VTIMEZONE
/// registry, so this is the entry point whose result is stable across
/// producers that spell the same instant differently.
pub fn calendar(cal: &Calendar) -> String {
    let stripped = Calendar {
        prod_id: cal.prod_id.clone(),
        components: cal.components.iter().map(strip_component).collect(),
    };
    digest(&canonical::calendar(&stripped))
}

/// The `sha256:<hex>` digest of the canonical byte form of `c`.
pub fn card(c: &Card) -> String {
    let stripped = Card {
        uid: c.uid.clone(),
        kind: c.kind,
        props: filter_out_hash(&c.props),
    };
    digest(&canonical::card(&stripped))
}

/// Computes [`component`] and writes the result to `c` as the
/// `X-VSTAR-HASH` property, replacing any existing value rather than
/// duplicating it.
///
/// This mutates `c` — it is the one function here that does. The hash
/// is computed over the stripped bytes, so calling it repeatedly on the
/// same logical component is idempotent: the second call computes the
/// same hash and rewrites the same value.
pub fn set_x_vstar(c: &mut Component) {
    let h = component(c);
    c.set(Property::new(X_VSTAR_HASH_PROPERTY, h));
}

/// The stored `X-VSTAR-HASH` value, or `None` when the property is
/// absent.
///
/// The stored value's format is not validated here. A caller wanting to
/// confirm both shape and freshness uses [`verify_x_vstar`].
pub fn get_x_vstar(c: &Component) -> Option<&str> {
    c.get(X_VSTAR_HASH_PROPERTY).map(|p| p.value.as_str())
}

/// Recomputes `c`'s hash and compares it against the stored
/// `X-VSTAR-HASH`, returning `(ok, want, got)`.
///
/// - `ok` — whether a hash is stored AND equals the recomputed one
///   exactly.
/// - `want` — the recomputed, correct hash. Always populated.
/// - `got` — the stored hash; the empty string when none is stored.
///
/// All three are reported regardless of the outcome, so a caller can
/// say *what* differed rather than only *that* something did — the
/// difference between a usable corruption report and a shrug. This is
/// why the return is a triple and not an `Option`.
pub fn verify_x_vstar(c: &Component) -> (bool, String, String) {
    let want = component(c);
    match get_x_vstar(c) {
        None => (false, want, String::new()),
        Some(got) => (got == want, want, got.to_owned()),
    }
}

/// `sha256:` plus the lowercase hex digest of `b`.
fn digest(b: &[u8]) -> String {
    let sum = Sha256::digest(b);
    let mut out = String::with_capacity(SHA256_PREFIX.len() + sum.len() * 2);
    out.push_str(SHA256_PREFIX);
    for byte in sum {
        // `{:02x}` is lowercase hex, which the spec fixes: a
        // capitalized digest is a different string and fails every
        // cross-implementation comparison.
        out.push_str(&format!("{byte:02x}"));
    }
    out
}

/// A copy of `c` with `X-VSTAR-HASH` removed from its properties and
/// from every nested sub-component.
///
/// Building a fresh component rather than editing in place is what
/// keeps the hash functions pure: a caller never observes the strip.
fn strip_component(c: &Component) -> Component {
    Component {
        r#type: c.r#type.clone(),
        props: filter_out_hash(&c.props),
        sub: c.sub.iter().map(strip_component).collect(),
    }
}

/// A copy of `props` without any `X-VSTAR-HASH` property.
fn filter_out_hash(props: &[Property]) -> Vec<Property> {
    props
        .iter()
        .filter(|p| !p.name.eq_ignore_ascii_case(X_VSTAR_HASH_PROPERTY))
        .cloned()
        .collect()
}

#[cfg(test)]
mod tests {
    use super::{digest, filter_out_hash, strip_component, X_VSTAR_HASH_PROPERTY};
    use crate::model::{Component, Property};
    use crate::CompType;

    #[test]
    fn the_digest_is_prefixed_lowercase_hex() {
        let h = digest(b"");
        assert_eq!(
            h, "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
            "the empty-input digest is the published SHA-256 test vector"
        );
    }

    #[test]
    fn the_strip_is_recursive_and_case_insensitive() {
        let mut inner = Component::new(CompType::from_wire("VALARM"));
        inner.add(Property::new("x-vstar-hash", "sha256:0"));
        inner.add(Property::new("ACTION", "DISPLAY"));
        let mut outer = Component::new(CompType::from_wire("VEVENT"));
        outer.add(Property::new(X_VSTAR_HASH_PROPERTY, "sha256:1"));
        outer.sub.push(inner);

        let stripped = strip_component(&outer);
        assert!(stripped.get(X_VSTAR_HASH_PROPERTY).is_none());
        assert!(stripped.sub[0].get(X_VSTAR_HASH_PROPERTY).is_none());
        assert_eq!(stripped.sub[0].props.len(), 1, "other properties survive");
    }

    #[test]
    fn filtering_preserves_order_of_the_survivors() {
        let props = vec![
            Property::new("A", "1"),
            Property::new(X_VSTAR_HASH_PROPERTY, "sha256:0"),
            Property::new("B", "2"),
        ];
        let out = filter_out_hash(&props);
        assert_eq!(out.len(), 2);
        assert_eq!(out[0].name, "A");
        assert_eq!(out[1].name, "B");
    }
}
