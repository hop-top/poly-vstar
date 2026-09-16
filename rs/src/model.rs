// SPDX-License-Identifier: MIT

//! The in-memory data model: [`Param`], [`Property`], [`Component`],
//! [`Calendar`] and [`Card`].
//!
//! Every collection is a `Vec`, not a map. Wire order is data: the
//! codecs preserve property, parameter and component order verbatim, and
//! canonicalization is the only layer licensed to reorder anything. A
//! map-backed model loses that ordering at parse time, where no test
//! that re-reads its own output can see the loss.
//!
//! The types are owned rather than borrowed. A parsed document outlives
//! the buffer it came from, which is what callers want and what every
//! sister port does.

use crate::date::{format_date, parse_date, Date};
use crate::enums::{CompType, Kind};

/// The RFC 5545 §3.2.20 parameter name that declares a property's value
/// type explicitly.
pub const VALUE_PARAM: &str = "VALUE";

/// The RFC 5545 §3.2.20 `VALUE` parameter value selecting the DATE value
/// type (§3.3.4).
///
/// The parameter is REQUIRED on any date-only `DTSTART` / `DTEND` /
/// `DUE` / `COMPLETED`: the default value type for those properties is
/// DATE-TIME, so an untagged eight-octet value is a malformed DATE-TIME,
/// not a DATE.
pub const VALUE_DATE: &str = "DATE";

/// A single property parameter, e.g. `CN=Jad` on an `ATTENDEE`.
///
/// Parameter name comparisons are case-insensitive per RFC 5545 §3.2 /
/// RFC 6350 §5; value comparisons are case-sensitive at this layer.
#[derive(Debug, Clone, Default, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct Param {
    /// The parameter name, in its wire case.
    pub name: String,
    /// The parameter value, unquoted.
    pub value: String,
}

impl Param {
    /// Builds a parameter from any pair of string-likes.
    pub fn new(name: impl Into<String>, value: impl Into<String>) -> Self {
        Param {
            name: name.into(),
            value: value.into(),
        }
    }
}

/// A single iCalendar / vCard content line in struct form: a name, zero
/// or more parameters, and a value.
///
/// The exact wire format is the codec's responsibility — this layer is
/// pure data, and holds values **unescaped**.
#[derive(Debug, Clone, Default, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct Property {
    /// The property name, in its wire case. For vCard this includes any
    /// `group.` prefix.
    pub name: String,
    /// Parameters in wire order.
    pub params: Vec<Param>,
    /// The raw, unescaped value.
    pub value: String,
}

impl Property {
    /// Builds a parameterless property.
    pub fn new(name: impl Into<String>, value: impl Into<String>) -> Self {
        Property {
            name: name.into(),
            params: Vec::new(),
            value: value.into(),
        }
    }

    /// Adds a parameter, returning `self` so builders chain.
    #[must_use]
    pub fn with_param(mut self, name: impl Into<String>, value: impl Into<String>) -> Self {
        self.params.push(Param::new(name, value));
        self
    }

    /// The first parameter whose name matches `name`, case-insensitively.
    pub fn param(&self, name: &str) -> Option<&Param> {
        self.params
            .iter()
            .find(|p| p.name.eq_ignore_ascii_case(name))
    }

    /// Reports whether this property carries `VALUE=DATE`.
    ///
    /// Both halves fold: parameter names are case-insensitive per
    /// RFC 5545 §3.2, and the `VALUE` argument is a registered value-type
    /// token (§3.2.20), likewise case-insensitive.
    pub fn has_value_date(&self) -> bool {
        self.param(VALUE_PARAM)
            .is_some_and(|p| p.value.eq_ignore_ascii_case(VALUE_DATE))
    }
}

/// Reports whether two properties are semantically equal: names and
/// parameter names fold, values compare case-sensitively, and parameter
/// order is normalized alphabetically before comparing.
///
/// Renamed from Go's `vstar.Equal` in every port: in Go the package
/// qualifier carries the meaning, and a bare `equal` exported from a
/// crate root does not. Inputs are not mutated.
pub fn property_equal(a: &Property, b: &Property) -> bool {
    if !a.name.eq_ignore_ascii_case(&b.name) {
        return false;
    }
    if a.value != b.value {
        return false;
    }
    if a.params.len() != b.params.len() {
        return false;
    }
    fn sorted(params: &[Param]) -> Vec<&Param> {
        let mut out: Vec<&Param> = params.iter().collect();
        out.sort_by(|x, y| {
            x.name
                .to_ascii_uppercase()
                .cmp(&y.name.to_ascii_uppercase())
        });
        out
    }
    sorted(&a.params)
        .into_iter()
        .zip(sorted(&b.params))
        .all(|(x, y)| x.name.eq_ignore_ascii_case(&y.name) && x.value == y.value)
}

/// Replaces every property matching `name` with a single copy of `prop`,
/// in the first match's slot. Appends when nothing matches.
fn set_in(props: &mut Vec<Property>, prop: Property) {
    let mut out = Vec::with_capacity(props.len());
    let mut replaced = false;
    for existing in props.drain(..) {
        if existing.name.eq_ignore_ascii_case(&prop.name) {
            if !replaced {
                out.push(prop.clone());
                replaced = true;
            }
            continue;
        }
        out.push(existing);
    }
    if !replaced {
        out.push(prop);
    }
    *props = out;
}

/// Deletes every property matching `name`, case-insensitively.
fn remove_in(props: &mut Vec<Property>, name: &str) {
    props.retain(|p| !p.name.eq_ignore_ascii_case(name));
}

/// A single iCalendar component: a typed identifier, a property list,
/// and nested sub-components (VTIMEZONE inside VCALENDAR, VALARM inside
/// VEVENT, and so on).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Component {
    /// The wire component type.
    ///
    /// Spelled `r#type` to match the Go field name across the ports;
    /// `type` is a Rust keyword.
    pub r#type: CompType,
    /// Properties in wire order.
    pub props: Vec<Property>,
    /// Sub-components in wire order.
    pub sub: Vec<Component>,
}

impl Component {
    /// Builds an empty component of the given type.
    pub fn new(r#type: CompType) -> Self {
        Component {
            r#type,
            props: Vec::new(),
            sub: Vec::new(),
        }
    }

    /// The first property whose name matches `name`, case-insensitively
    /// per RFC 5545 §3.1.
    pub fn get(&self, name: &str) -> Option<&Property> {
        self.props
            .iter()
            .find(|p| p.name.eq_ignore_ascii_case(name))
    }

    /// Every property whose name matches `name`, in wire order.
    pub fn get_all(&self, name: &str) -> Vec<&Property> {
        self.props
            .iter()
            .filter(|p| p.name.eq_ignore_ascii_case(name))
            .collect()
    }

    /// Replaces every property matching `p.name` with a single copy of
    /// `p`. Appends when nothing matches.
    pub fn set(&mut self, p: Property) {
        set_in(&mut self.props, p);
    }

    /// Appends `p` without touching existing properties of the same name.
    pub fn add(&mut self, p: Property) {
        self.props.push(p);
    }

    /// Deletes every property matching `name`.
    pub fn remove(&mut self, name: &str) {
        remove_in(&mut self.props, name);
    }

    /// The component's `UID` value, or `""` when absent.
    pub fn uid(&self) -> &str {
        self.get("UID").map_or("", |p| p.value.as_str())
    }

    /// The component's `DTSTAMP` value as the raw RFC 5545 wire string,
    /// or `""` when absent.
    pub fn dtstamp_raw(&self) -> &str {
        self.get("DTSTAMP").map_or("", |p| p.value.as_str())
    }

    /// Reports whether the named property is present and carries
    /// `VALUE=DATE` — the branch point for callers that do not know the
    /// wire form up front. `false` for an absent property.
    pub fn is_date_only(&self, name: &str) -> bool {
        self.get(name).is_some_and(Property::has_value_date)
    }

    /// Parses a date-bearing property's value.
    ///
    /// `None` for a missing property, one that does not declare
    /// `VALUE=DATE`, or a malformed DATE. The `VALUE=DATE` requirement is
    /// deliberate: an untagged `20260515` declares itself DATE-TIME by
    /// default and is torn data, so promoting it to a [`Date`] would be
    /// the silent coercion the parsers exist to prevent.
    fn date_prop(&self, name: &str) -> Option<Date> {
        let p = self.get(name)?;
        if !p.has_value_date() {
            return None;
        }
        parse_date(&p.value)
    }

    /// The `DTSTART` value as a calendar date, when it carries
    /// `VALUE=DATE`. A midnight DATE-TIME does not surface here.
    pub fn dtstart_date(&self) -> Option<Date> {
        self.date_prop("DTSTART")
    }

    /// The `DTEND` value as a calendar date.
    ///
    /// Per RFC 5545 §3.6.1 an all-day `DTEND` is EXCLUSIVE; this reports
    /// the wire value as written and does not adjust it.
    pub fn dtend_date(&self) -> Option<Date> {
        self.date_prop("DTEND")
    }

    /// The VTODO `DUE` value as a calendar date.
    pub fn due_date(&self) -> Option<Date> {
        self.date_prop("DUE")
    }

    /// The VTODO `COMPLETED` value as a calendar date.
    ///
    /// RFC 5545 §3.8.2.1 defines `COMPLETED` as DATE-TIME only, so a
    /// `VALUE=DATE` `COMPLETED` is non-conforming input; the accessor
    /// exists so a reader can recover such a value rather than lose it.
    pub fn completed_date(&self) -> Option<Date> {
        self.date_prop("COMPLETED")
    }

    /// Writes a date-only value, or removes the property when `d` is the
    /// zero [`Date`].
    ///
    /// The written property carries exactly one parameter, `VALUE=DATE`,
    /// and nothing else. Dropping pre-existing parameters is required
    /// rather than tidy: a stale `TZID` would be meaningless on a DATE
    /// (RFC 5545 §3.2.19 scopes `TZID` to DATE-TIME and TIME values), and
    /// a stale parameter set would make the canonical bytes depend on the
    /// property's edit history.
    fn set_or_clear_date(&mut self, name: &str, d: Date) {
        if d.is_zero() {
            self.remove(name);
            return;
        }
        self.set(Property {
            name: name.to_owned(),
            params: vec![Param::new(VALUE_PARAM, VALUE_DATE)],
            value: format_date(d),
        });
    }

    /// Writes an all-day `DTSTART`. The zero [`Date`] removes it.
    pub fn set_dtstart_date(&mut self, d: Date) {
        self.set_or_clear_date("DTSTART", d);
    }

    /// Writes an all-day `DTEND`. Per RFC 5545 §3.6.1 the all-day
    /// `DTEND` is EXCLUSIVE; this writes what it is given.
    pub fn set_dtend_date(&mut self, d: Date) {
        self.set_or_clear_date("DTEND", d);
    }

    /// Writes an all-day `DUE`.
    pub fn set_due_date(&mut self, d: Date) {
        self.set_or_clear_date("DUE", d);
    }

    /// Writes a date-only `COMPLETED`.
    ///
    /// RFC 5545 §3.8.2.1 mandates DATE-TIME for `COMPLETED`, so this
    /// emits non-conforming output; it exists for symmetry.
    pub fn set_completed_date(&mut self, d: Date) {
        self.set_or_clear_date("COMPLETED", d);
    }
}

/// The top-level VCALENDAR container: the `PRODID` identifying the
/// producing system, plus the contained components (RFC 5545 §3.4).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Calendar {
    /// The `PRODID` value.
    pub prod_id: String,
    /// Contained components in wire order.
    pub components: Vec<Component>,
}

impl Calendar {
    /// The first component whose `UID` matches `uid`.
    ///
    /// Comparison is **case-sensitive** per RFC 5545 §3.8.4.7: UIDs are
    /// opaque identifiers, not user-facing text.
    pub fn find(&self, uid: &str) -> Option<&Component> {
        self.components.iter().find(|c| c.uid() == uid)
    }

    /// Adds `comp` to the component list.
    pub fn append(&mut self, comp: Component) {
        self.components.push(comp);
    }

    /// Every component of the requested type, in wire order.
    pub fn filter(&self, t: &CompType) -> Vec<&Component> {
        self.components.iter().filter(|c| &c.r#type == t).collect()
    }
}

/// A top-level VCARD object per RFC 6350: a `UID`, a [`Kind`]
/// discriminator, and the property list.
///
/// Structurally analogous to [`Component`] but distinct: vCards do not
/// nest sub-components and do not carry a [`CompType`].
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Card {
    /// The `UID` value, `""` when the source carried none.
    pub uid: String,
    /// The `KIND` value, or `None` when the card carried no `KIND`
    /// property.
    ///
    /// Go spells the absent state as the empty string; Rust spells it
    /// `None`, so the encoder's "emit `KIND` only when set" rule is a
    /// type-level fact rather than a comparison against a sentinel.
    pub kind: Option<Kind>,
    /// Properties in wire order, excluding `VERSION`, `UID` and `KIND`,
    /// which are lifted into the fields above.
    pub props: Vec<Property>,
}

impl Card {
    /// The first property whose name matches `name`, case-insensitively
    /// per RFC 6350 §3.3.
    pub fn get(&self, name: &str) -> Option<&Property> {
        self.props
            .iter()
            .find(|p| p.name.eq_ignore_ascii_case(name))
    }

    /// Every property whose name matches `name`, in wire order.
    pub fn get_all(&self, name: &str) -> Vec<&Property> {
        self.props
            .iter()
            .filter(|p| p.name.eq_ignore_ascii_case(name))
            .collect()
    }

    /// Replaces every property matching `p.name` with a single copy of
    /// `p`. Appends when nothing matches.
    pub fn set(&mut self, p: Property) {
        set_in(&mut self.props, p);
    }

    /// Appends `p` without touching existing properties of the same name.
    pub fn add(&mut self, p: Property) {
        self.props.push(p);
    }

    /// Deletes every property matching `name`.
    pub fn remove(&mut self, name: &str) {
        remove_in(&mut self.props, name);
    }
}
