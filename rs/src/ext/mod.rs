// SPDX-License-Identifier: MIT

//! Predicates and accessors for the V\* `X-*` extension namespace
//! defined in spec/04 (Extension Discipline).
//!
//! V\* extensions live in three tiers:
//!
//! - `X-VSTAR-*` — cross-system extensions on a stabilization track.
//! - `X-<SYSTEM>-*` — one specific consuming system (`X-AGR-INTENT`).
//! - `X-EXP-*` — experimental / unstable; no guarantees.
//!
//! The promotion path runs `X-EXP-FOO` → `X-<SYSTEM>-FOO` →
//! `X-VSTAR-FOO`: an experimental property graduates when one system
//! commits to it, and promotes once two independent systems implement
//! compatible semantics. Removing an extension is a breaking change for
//! consumers; the pattern is promote-then-replace, never rename.

use crate::model::{Component, Property};
use std::fmt;

/// The classification of an extension name under spec/04.
///
/// [`Scope::None`] is the zero value — the name is not an `X-*`
/// extension at all — and [`Scope::Unknown`] is the catch-all for a name
/// that carries the `X-` prefix but matches no sanctioned tier.
///
/// # Display versus the fixture token
///
/// [`fmt::Display`] renders the reference's capitalized spelling
/// (`"VStar"`, `"System"`, …), matching Go's `Scope.String()`. The
/// `spec/behavior/ext/scopes.json` fixture records that value
/// *lowercased* — the generator writes
/// `strings.ToLower(ScopeOf(name).String())` — so a gate comparing
/// against the fixture lowercases this rendering rather than reaching
/// for a second spelling of the same value.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Scope {
    /// The name is not an `X-*` extension at all.
    ///
    /// The zero value, matching Go's `ScopeNone`.
    #[default]
    None,
    /// `X-VSTAR-*` — a cross-system V\* extension on the stabilization
    /// track. `X-VSTAR-HASH` (spec/02) is the only mandatory member in
    /// v0.1.
    VStar,
    /// `X-<SYSTEM>-*` — an extension owned by one consuming system.
    ///
    /// `SYSTEM` is any slug other than `VSTAR` (reserved for
    /// [`Scope::VStar`]) and `EXP` (reserved for
    /// [`Scope::Experimental`]).
    System,
    /// `X-EXP-*` — an unstable experimental extension. Senders MUST NOT
    /// depend on receivers honoring these (spec/04).
    Experimental,
    /// Has the `X-` prefix but matches no sanctioned tier: `X-` (no
    /// slug), `X-VSTAR-` (nothing after the prefix), `X-FOO` (no name
    /// after the system slug). Treat as opaque; receivers MUST still
    /// ignore it per RFC 5545 compatibility rules.
    Unknown,
}

impl fmt::Display for Scope {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(match self {
            Scope::None => "None",
            Scope::VStar => "VStar",
            Scope::System => "System",
            Scope::Experimental => "Experimental",
            Scope::Unknown => "Unknown",
        })
    }
}

/// Reports whether `name` carries the `X-` prefix per RFC 5545 §3.8.8 /
/// spec/04. Comparison is case-insensitive, so both `X-FOO` and `x-foo`
/// are extensions.
///
/// The hyphen is required: `"X"` is a regular IANA-style identifier
/// while `"X-"` is an (ill-formed) extension. Pair this with
/// [`scope_of`] to classify a name.
pub fn is_extension(name: &str) -> bool {
    let b = name.as_bytes();
    b.len() >= 2 && (b[0] == b'X' || b[0] == b'x') && b[1] == b'-'
}

/// Splits `SLUG-REST` at the first hyphen, or `None` when there is none.
fn cut_slug(s: &str) -> Option<(&str, &str)> {
    let i = s.find('-')?;
    Some((&s[..i], &s[i + 1..]))
}

/// Classifies `name` into one of the five [`Scope`] values per spec/04,
/// case-insensitively.
///
/// The decision tree:
///
/// - no `X-` prefix → [`Scope::None`]
/// - `X-VSTAR-<NAME>`, `NAME` non-empty → [`Scope::VStar`]
/// - `X-EXP-<NAME>`, `NAME` non-empty → [`Scope::Experimental`]
/// - `X-<SYSTEM>-<NAME>`, both non-empty and `SYSTEM` neither `VSTAR`
///   nor `EXP` → [`Scope::System`]
/// - anything else with the prefix → [`Scope::Unknown`]
///
/// The reservation on `VSTAR` and `EXP` covers the whole slug segment,
/// not a prefix of it: `X-VSTARLIKE-FOO` is an ordinary system
/// extension owned by `VSTARLIKE`.
///
/// The name is `scope_of`, not `scope`, because Go cannot share an
/// identifier between a type and a function in one package. Rust has no
/// such constraint but keeps the spelling so the four ports and the
/// reference read alike.
pub fn scope_of(name: &str) -> Scope {
    if !is_extension(name) {
        return Scope::None;
    }
    let rest = &name[2..];
    if rest.is_empty() {
        return Scope::Unknown;
    }
    let Some((slug, suffix)) = cut_slug(rest) else {
        // "X-FOO" — no second hyphen, so no name after the slug.
        return Scope::Unknown;
    };
    if suffix.is_empty() {
        return Scope::Unknown;
    }
    match slug.to_ascii_uppercase().as_str() {
        "VSTAR" => Scope::VStar,
        "EXP" => Scope::Experimental,
        _ => Scope::System,
    }
}

/// The owning system's slug for an `X-<SYSTEM>-<NAME>` extension,
/// uppercased so callers compare without re-normalizing, or `None` for
/// any name that is not [`Scope::System`]-scoped.
///
/// That exclusion is deliberate and covers non-extensions, `X-VSTAR-*`
/// (the V\* spec owns it), `X-EXP-*` (no owner at all) and malformed
/// names lacking the `<SYSTEM>-<NAME>` structure. The question this
/// answers is "which system owns this property?", which only a
/// system-scoped name has an answer to.
///
/// Returns an owned [`String`] rather than a borrow: the slug is
/// uppercased, so there is no substring of the input to hand back. See
/// the note in the crate's porting notes — `docs/dev/api-mapping.md`
/// spells the Rust return as `Option<&str>`, which cannot express an
/// uppercased result.
pub fn system_name(name: &str) -> Option<String> {
    if !is_extension(name) {
        return None;
    }
    let (slug, suffix) = cut_slug(&name[2..])?;
    if slug.is_empty() || suffix.is_empty() {
        return None;
    }
    let upper = slug.to_ascii_uppercase();
    if upper == "VSTAR" || upper == "EXP" {
        return None;
    }
    Some(upper)
}

/// Every property on `c` whose name classifies into `scope`, in the
/// component's own property order — **no sort**.
///
/// `scope_of(name) == Scope::None` selects every non-extension property
/// (`UID`, `DTSTART`, …), which is the useful shape for diffing the V\*
/// core surface.
///
/// The function does not recurse into [`Component::sub`]; a caller
/// wanting the whole tree walks the sub-components itself.
pub fn extensions_by_scope(c: &Component, scope: Scope) -> Vec<&Property> {
    c.props
        .iter()
        .filter(|p| scope_of(&p.name) == scope)
        .collect()
}
