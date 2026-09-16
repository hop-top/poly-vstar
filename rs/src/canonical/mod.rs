// SPDX-License-Identifier: MIT

//! The deterministic canonical byte form of V\* objects, per spec rules
//! 1–12.
//!
//! Two V\* documents containing the same logical content MUST produce
//! identical canonical bytes. That is the invariant the whole
//! specification exists to hold, and it is what makes an
//! `X-VSTAR-HASH` comparable across implementations.
//!
//! # The output is bytes
//!
//! Every function here returns `Vec<u8>`, not `String`. The
//! distinction is load-bearing twice over: the canonical form's final
//! CRLF is part of the value, and — decisively for Rust — folding
//! counts octets, so a multi-byte UTF-8 sequence straddling the
//! 75-octet boundary is **split across the fold** (spec rule 3). A
//! `String` cannot hold a split sequence, so the only way to produce
//! one from a `String` pipeline would be to retreat the cut to a
//! character boundary — which moves every subsequent fold point and
//! changes both the canonical bytes and the hash. The corpus fixture
//! `rfc5545/fold_split_utf8.canonical` is not valid UTF-8, by design.
//!
//! # Transform order
//!
//! The transforms are applied to one component in the order the spec
//! fixes:
//!
//! 1. **Select** — drop `X-VSTAR-HASH` (rule 7); reduce `ATTACH` to URI
//!    form by stripping `VALUE=BINARY` and `ENCODING=BASE64` (rule 10).
//! 2. **Resolve datetimes** on the rule-5 allow-list against the
//!    calendar's VTIMEZONE registry, re-emitting as UTC form #2 and
//!    dropping the `TZID` where resolution succeeds. A `VALUE=DATE`
//!    property is governed by rule 11 and is never resolved; `RRULE`
//!    (rule 8) and `DURATION` (rule 12) pass through verbatim.
//! 3. **Normalize to NFC** — each property value and each parameter
//!    value, individually. Not names (rule 9).
//! 4. **Sort** — properties by name, each property's parameters by name
//!    (rule 2); top-level components by `UID`, or `TZID` for a
//!    VTIMEZONE, byte-wise on UTF-8, stable, key-less last (rule 6).
//!    Sub-components keep input order.
//! 5. **Assemble** each content line with escaping and parameter
//!    quoting (rule 4).
//! 6. **Fold** each assembled line at 75 octets (rule 3).
//! 7. **Terminate** every physical line with CRLF (rule 1).
//!
//! Steps 5–7 belong to the RFC 5545 encoder, which owns folding, CRLF
//! and TEXT escaping so there is exactly one implementation of each.
//!
//! NFC therefore sits **after** datetime resolution and **before**
//! sorting and folding. Before folding matters: normalization changes a
//! string's UTF-8 length — `e` + U+0301 is three octets, `é` is two —
//! so folding a pre-normalization string puts the break at the wrong
//! octet.
//!
//! Nothing here mutates its input.

use crate::codec::rfc5545;
use crate::hashing::X_VSTAR_HASH_PROPERTY;
use crate::model::{Calendar, Card, Component, Param, Property};
use crate::time::{format_time, is_datetime_property, parse_time_with_tzid};
use crate::CompType;
use unicode_normalization::{is_nfc_quick, IsNormalized, UnicodeNormalization};

/// The canonical byte form of a single component, emitting datetimes
/// **verbatim**.
///
/// This form is for components carrying no `TZID`-tagged datetimes. The
/// VTIMEZONE registry lives on the [`Calendar`], not on the
/// [`Component`], so this entry point cannot resolve a `TZID` reference
/// and emits the value with its `TZID` parameter retained — output is
/// non-canonical for such a component. Use [`component_in_context`] to
/// thread the parent calendar.
///
/// The asymmetry is real, not an overload: a component without a parent
/// calendar genuinely has no registry to consult.
pub fn component(c: &Component) -> Vec<u8> {
    component_in_context(c, &Calendar::default())
}

/// The canonical byte form of a single component, resolving
/// `TZID`-tagged datetimes against `cal`'s VTIMEZONE registry.
///
/// For each property on the rule-5 allow-list carrying a `TZID`: on
/// successful resolution the value is re-emitted as UTC form #2 and the
/// `TZID` parameter is dropped. On failure — no matching VTIMEZONE, or
/// one outside the v0.1 subset — the value AND the `TZID` pass through
/// verbatim. Canonical bytes are not deterministic across calendars
/// carrying different VTIMEZONE definitions in that branch; a producer
/// is expected to ship coverage inside the subset.
///
/// A value already in UTC form #2, and one with no `TZID` at all, pass
/// through unchanged — there is nothing to resolve.
///
/// `STANDARD` and `DAYLIGHT` children inside a VTIMEZONE carry a
/// wall-clock `DTSTART` that defines the transition rule itself. Those
/// are deliberately not `TZID`-tagged and pass through by design.
pub fn component_in_context(c: &Component, cal: &Calendar) -> Vec<u8> {
    let mut out = Vec::new();
    // `encode_component` writes into a `Vec`, whose `Write` impl is
    // infallible, so the error branch is unreachable.
    let _ = rfc5545::encode_component(&mut out, &prepare_component(c, cal));
    out
}

/// The canonical byte form of a full VCALENDAR.
///
/// Top-level components are sorted per rule 6 and each is prepared
/// against the calendar's own VTIMEZONE registry, so a `TZID`-bearing
/// datetime in any child resolves against a VTIMEZONE in the same
/// document.
///
/// The `PRODID` value is NFC-normalized here; the encoder owns its TEXT
/// escaping, so no pre-escaping happens at this layer.
pub fn calendar(c: &Calendar) -> Vec<u8> {
    let wire = Calendar {
        prod_id: nfc(&c.prod_id),
        components: sorted_components(&c.components)
            .into_iter()
            .map(|sub| prepare_component(sub, c))
            .collect(),
    };
    let mut out = Vec::new();
    let _ = rfc5545::encode(&mut out, &wire);
    out
}

/// The canonical byte form of a single VCARD:
///
/// ```text
/// BEGIN:VCARD
/// VERSION:4.0
/// <properties sorted by name; UID is one of them>
/// END:VCARD
/// ```
///
/// `VERSION` is promoted ahead of alphabetical order, because RFC 6350
/// §3.3 requires it immediately after `BEGIN:VCARD`. [`Card::uid`] and
/// [`Card::kind`] are emitted as properties, or absorbed when
/// [`Card::props`] already carries one of the same name.
///
/// A vCard has no datetime or `TZID` concerns, so there is no context
/// form.
pub fn card(c: &Card) -> Vec<u8> {
    let mut props = Vec::with_capacity(c.props.len() + 3);
    props.push(Property::new("VERSION", "4.0"));
    if !c.uid.is_empty() && !has_prop(&c.props, "UID") {
        props.push(Property::new("UID", &c.uid));
    }
    if let Some(kind) = c.kind {
        if !has_prop(&c.props, "KIND") {
            props.push(Property::new("KIND", kind.as_str()));
        }
    }
    props.extend(c.props.iter().cloned());

    // A wire-string component type, so the encoder emits BEGIN:VCARD
    // and END:VCARD. The vCard shares the iCalendar content-line
    // grammar, so it shares the encoder rather than duplicating the
    // fold and escape logic.
    let synth = Component {
        r#type: CompType::from_wire("VCARD"),
        props,
        sub: Vec::new(),
    };
    let mut prepared = prepare_component(&synth, &Calendar::default());

    // VERSION must come immediately after BEGIN:VCARD per RFC 6350
    // §3.3, ahead of alphabetical order. Promote it.
    if let Some(i) = prepared
        .props
        .iter()
        .position(|p| p.name.eq_ignore_ascii_case("VERSION"))
    {
        let version = prepared.props.remove(i);
        prepared.props.insert(0, version);
    }

    let mut out = Vec::new();
    let _ = rfc5545::encode_component(&mut out, &prepared);
    out
}

/// A copy of `c` with every canonicalization transform applied except
/// assembly, folding and CRLF, which the encoder owns.
///
/// Sub-components are prepared recursively and are NOT sorted: they
/// have no natural sort key, so rule 6 preserves their input order.
fn prepare_component(c: &Component, cal: &Calendar) -> Component {
    let mut props: Vec<Property> = c
        .props
        .iter()
        .filter(|p| !p.name.eq_ignore_ascii_case(X_VSTAR_HASH_PROPERTY))
        .map(|p| prepare_property(p, cal))
        .collect();
    sort_by_upper_name(&mut props);

    Component {
        r#type: c.r#type.clone(),
        props,
        sub: c.sub.iter().map(|s| prepare_component(s, cal)).collect(),
    }
}

/// A copy of `p` with the value and parameter transforms applied.
///
/// TEXT escaping is deliberately absent: the RFC 5545 encoder owns the
/// single authoritative escape pass on emit, using its own allow-list
/// of TEXT-typed property names. Escaping here would double it.
fn prepare_property(p: &Property, cal: &Calendar) -> Property {
    let mut value = nfc(&p.value);

    // Rule 11: a DATE value has no time to convert and no zone to
    // resolve, so it is emitted verbatim and the resolution registry is
    // never consulted. VALUE=DATE is RETAINED — unlike a resolved TZID
    // it is load-bearing, since the default value type for these
    // properties is DATE-TIME and an untagged eight-octet value is a
    // malformed DATE-TIME, not a DATE.
    let is_datetime = is_datetime_property(&p.name);
    let date_only = is_datetime && p.has_value_date();

    // Rule 5: a datetime property carrying a TZID resolves to UTC form
    // #2 where the calendar's registry allows, and the TZID is then
    // dropped.
    let mut strip_tzid = date_only;
    if is_datetime && !date_only {
        if let Some(tzid) = p.param("TZID").map(|prm| prm.value.as_str()) {
            if !tzid.is_empty() {
                if let Some(at) = parse_time_with_tzid(&p.value, tzid, cal) {
                    value = format_time(at);
                    strip_tzid = true;
                }
            }
        }
    }

    let is_attach = p.name.eq_ignore_ascii_case("ATTACH");
    let mut params = Vec::with_capacity(p.params.len());
    for prm in &p.params {
        // Rule 10: ATTACH is reference-only on emit. The value itself
        // is untouched — canonical form is best-effort for a malformed
        // URI.
        if is_attach
            && ((prm.name.eq_ignore_ascii_case("VALUE")
                && prm.value.eq_ignore_ascii_case("BINARY"))
                || (prm.name.eq_ignore_ascii_case("ENCODING")
                    && prm.value.eq_ignore_ascii_case("BASE64")))
        {
            continue;
        }
        // A TZID on a DATE is a producer bug (RFC 5545 §3.2.19 scopes
        // TZID to DATE-TIME and TIME) and must not leak into the
        // canonical bytes; a resolved TZID is redundant.
        if strip_tzid && prm.name.eq_ignore_ascii_case("TZID") {
            continue;
        }

        let mut pv = nfc(&prm.value);
        // Rule 11 upper-cases the VALUE argument so `VALUE=date` and
        // `VALUE=DATE` converge. General case-folding of other VALUE
        // tokens is deferred to v0.2, so this is scoped to the DATE
        // branch.
        if date_only && prm.name.eq_ignore_ascii_case("VALUE") {
            pv = pv.to_uppercase();
        }
        params.push(Param {
            name: prm.name.clone(),
            value: pv,
        });
    }
    sort_by_upper_name(&mut params);

    Property {
        name: p.name.clone(),
        params,
        value,
    }
}

/// Sorts `items` in place by uppercased name, stably.
///
/// The comparison is on the uppercased ASCII name, so the byte-order
/// question that governs the component sort does not arise: property
/// and parameter names are ASCII by RFC 5545 §3.1 / RFC 6350 §3.3.
///
/// `sort_by_key` is stable in Rust, so equal names — a repeated
/// `CATEGORIES`, say — keep their relative input order.
fn sort_by_upper_name<T: HasName>(items: &mut [T]) {
    items.sort_by_key(|i| i.name().to_ascii_uppercase());
}

/// The shape `sort_by_upper_name` sorts.
trait HasName {
    fn name(&self) -> &str;
}

impl HasName for Property {
    fn name(&self) -> &str {
        &self.name
    }
}

impl HasName for Param {
    fn name(&self) -> &str {
        &self.name
    }
}

/// A copy of `components`, sorted per rule 6: by `UID`, or by `TZID`
/// for a VTIMEZONE; components with neither key sort last; the sort is
/// stable, so equal keys — a producer bug — keep their relative input
/// order.
///
/// The key comparison is Rust's `str` `Ord`, which **is** UTF-8 byte
/// order, so rule 6 needs no custom comparator here. (The TypeScript
/// port does need one: JavaScript's `<` compares UTF-16 code units, and
/// an astral character sorts below every BMP character from U+E000 up
/// in that order but above them in UTF-8. The `sort_utf8_uids` fixture
/// pins the distinction.)
fn sorted_components(components: &[Component]) -> Vec<&Component> {
    let mut out: Vec<&Component> = components.iter().collect();
    // Keyless components sort last, so the key is lifted into an
    // `Option` whose `None` orders after every `Some`.
    out.sort_by(
        |a, b| match (component_sort_key(a), component_sort_key(b)) {
            (Some(x), Some(y)) => x.cmp(y),
            (Some(_), None) => std::cmp::Ordering::Less,
            (None, Some(_)) => std::cmp::Ordering::Greater,
            (None, None) => std::cmp::Ordering::Equal,
        },
    );
    out
}

/// The rule-6 sort key: `TZID` for a VTIMEZONE, otherwise `UID` then
/// `TZID`.
fn component_sort_key(c: &Component) -> Option<&str> {
    if c.r#type.eq_fold("VTIMEZONE") {
        if let Some(p) = c.get("TZID") {
            return Some(&p.value);
        }
    }
    let uid = c.uid();
    if !uid.is_empty() {
        return Some(uid);
    }
    c.get("TZID").map(|p| p.value.as_str())
}

/// Whether `props` already carries a property named `name`.
fn has_prop(props: &[Property], name: &str) -> bool {
    props.iter().any(|p| p.name.eq_ignore_ascii_case(name))
}

/// The NFC form of `s`, per rule 9.
///
/// The fast path matters: normalization allocates unconditionally, and
/// the overwhelming majority of values are already normalized.
/// `is_nfc_quick` answers `Yes` or `No` without allocating for the
/// common cases and `Maybe` only when a full check is needed, which is
/// what the Go reference's `IsNormalString` pre-check does for the same
/// reason.
fn nfc(s: &str) -> String {
    match is_nfc_quick(s.chars()) {
        IsNormalized::Yes => s.to_owned(),
        _ => s.nfc().collect(),
    }
}

#[cfg(test)]
mod tests {
    use super::{component_sort_key, nfc, sorted_components};
    use crate::model::{Component, Property};
    use crate::CompType;

    #[test]
    fn nfc_composes_and_is_idempotent() {
        let decomposed = "Cafe\u{301}";
        let composed = "Caf\u{e9}";
        assert_eq!(nfc(decomposed), composed);
        assert_eq!(nfc(composed), composed);
        assert_eq!(nfc(&nfc(decomposed)), composed);
        // Pure ASCII takes the fast path unchanged.
        assert_eq!(nfc("plain"), "plain");
    }

    #[test]
    fn str_ord_is_utf8_byte_order() {
        // Rust's `str` Ord compares UTF-8 bytes, which is exactly what
        // rule 6 specifies. A UTF-16 code-unit comparison would order
        // the astral value before the fullwidth one.
        let mut v = ["uid-\u{1D400}", "uid-\u{FF21}", "uid-\u{FF}", "uid-a"];
        v.sort_unstable();
        assert_eq!(v, ["uid-a", "uid-\u{FF}", "uid-\u{FF21}", "uid-\u{1D400}"]);
    }

    #[test]
    fn the_sort_key_prefers_tzid_for_a_vtimezone() {
        let mut tz = Component::new(CompType::from_wire("VTIMEZONE"));
        tz.add(Property::new("TZID", "Zone/One"));
        tz.add(Property::new("UID", "ignored"));
        assert_eq!(component_sort_key(&tz), Some("Zone/One"));

        let mut evt = Component::new(CompType::from_wire("VEVENT"));
        evt.add(Property::new("UID", "u1"));
        assert_eq!(component_sort_key(&evt), Some("u1"));

        assert_eq!(component_sort_key(&Component::default()), None);
    }

    #[test]
    fn keyless_components_sort_last_and_stably() {
        let keyless = |s: &str| {
            let mut c = Component::new(CompType::from_wire("VEVENT"));
            c.add(Property::new("SUMMARY", s));
            c
        };
        let keyed = |uid: &str| {
            let mut c = Component::new(CompType::from_wire("VEVENT"));
            c.add(Property::new("UID", uid));
            c
        };
        let input = vec![
            keyless("first"),
            keyed("zzz"),
            keyless("second"),
            keyed("aaa"),
        ];
        let sorted = sorted_components(&input);
        assert_eq!(component_sort_key(sorted[0]), Some("aaa"));
        assert_eq!(component_sort_key(sorted[1]), Some("zzz"));
        assert_eq!(
            sorted[2].get("SUMMARY").map(|p| p.value.as_str()),
            Some("first")
        );
        assert_eq!(
            sorted[3].get("SUMMARY").map(|p| p.value.as_str()),
            Some("second")
        );
    }
}
