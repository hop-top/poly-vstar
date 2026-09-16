// SPDX-License-Identifier: MIT

//! Semantic equality and structural diff for V\* values.
//!
//! "Semantic" means *via canonical form*: two values that yield
//! identical canonical bytes are reported equal regardless of property
//! order, parameter order, datetime form or whitespace. The diff side
//! returns property-level structural changes suitable for human display
//! or programmatic inspection.
//!
//! # Equality rules, delegated to [`canonical`](crate::canonical)
//!
//! - `X-VSTAR-HASH` is excluded from comparison and from diff output on
//!   both sides, mirroring canonicalization's spec/03 rule-7 exclusion.
//!   A diff reports what changed in the content, not the restamped hash
//!   that followed.
//! - Property and parameter ordering is irrelevant.
//! - Datetime forms compare equal when canonicalization resolves them
//!   to the same UTC representation.
//!
//! # Order is the contract
//!
//! [`of_calendar`] emits components in **pairing order**, not sorted by
//! path, and [`ComponentDiff::properties`] is sorted by property name
//! case-insensitively. The `spec/behavior/diff` fixtures compare the
//! sequences positionally, so a port that sorts the component list
//! fails the gate.
//!
//! # Matching heuristics (documented limitations)
//!
//! Sub-components are paired by `(type, uid)` when both carry a UID;
//! those without one — a VALARM, typically — are paired positionally by
//! index within their type bucket. Reordering UID-less sub-components
//! therefore surfaces as add + remove rather than change.

use crate::canonical;
use crate::model::{property_equal, Calendar, Card, Component, Property};
use std::collections::{BTreeMap, BTreeSet};
use std::fmt;

/// The spec/03 sentinel header excluded from diff input on both sides.
const X_VSTAR_HASH: &str = "X-VSTAR-HASH";

/// The kind of property change recorded in a [`PropertyDiff`].
///
/// Go numbers these from 1 (`OpAdded = iota + 1`) so zero is not a valid
/// value. Rust's enum has no default, which keeps the same guarantee:
/// there is no way to construct a meaningless `DiffOp`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum DiffOp {
    /// Present in `b`, missing from `a`.
    Added,
    /// Present in `a`, missing from `b`.
    Removed,
    /// Present in both with a differing value or parameter set.
    Changed,
}

impl fmt::Display for DiffOp {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(match self {
            DiffOp::Added => "Added",
            DiffOp::Removed => "Removed",
            DiffOp::Changed => "Changed",
        })
    }
}

/// A single property-level change between two V\* values.
///
/// For [`DiffOp::Added`], `property` is the new (b-side) value and `old`
/// is the zero [`Property`]. For [`DiffOp::Removed`], `property` is the
/// original (a-side) value and `old` is zero. For [`DiffOp::Changed`],
/// `property` is the new value and `old` the original — which is what
/// makes a parameter-only change expressible: the two values are equal
/// and the parameter lists differ.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PropertyDiff {
    /// Which kind of change this is.
    pub op: DiffOp,
    /// The surviving side's property.
    pub property: Property,
    /// The original property, for [`DiffOp::Changed`] only.
    pub old: Property,
}

/// The structural changes between two components, two cards, or one
/// paired calendar entry.
///
/// `path` identifies the diff site for human display: `""` for a
/// top-level component or card diff, `VCALENDAR.VEVENT[uid=…]` for an
/// [`of_calendar`] entry, and `<parent>.<TYPE>[uid=…]` or
/// `<parent>.<TYPE>[#index]` for a nested sub-diff.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ComponentDiff {
    /// The rendered locator for this level.
    pub path: String,
    /// Property-level changes here, sorted by name case-insensitively.
    pub properties: Vec<PropertyDiff>,
    /// Recursive diffs for sub-components that themselves changed.
    pub sub_diffs: Vec<ComponentDiff>,
}

impl ComponentDiff {
    /// Reports whether the diff records no change at this level nor in
    /// any nested sub-component.
    ///
    /// Renamed from Go's `Empty()`: the Go name is a predicate despite
    /// reading as an adjective, and every target language spells
    /// predicates with an `is` prefix.
    pub fn is_empty(&self) -> bool {
        self.properties.is_empty() && self.sub_diffs.iter().all(ComponentDiff::is_empty)
    }
}

impl fmt::Display for ComponentDiff {
    /// Renders a unified-diff-ish text block:
    ///
    /// ```text
    /// --- <path>
    /// + NAME[;PARAM=VAL...]:VALUE       // added
    /// - NAME[;PARAM=VAL...]:VALUE       // removed
    /// ~ NAME: <old> -> <new>            // changed
    /// ```
    ///
    /// Sub-component diffs indent two spaces per level, each opening
    /// with its own `--- <path>` header. An empty diff renders as `""`.
    ///
    /// **Format stability**: this output is informational, for humans
    /// and debug CLIs. It is not a wire format and is not covered by
    /// V\*'s cross-implementation parity guarantees.
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.is_empty() {
            return Ok(());
        }
        self.write_at(f, 0)
    }
}

impl ComponentDiff {
    fn write_at(&self, f: &mut fmt::Formatter<'_>, depth: usize) -> fmt::Result {
        let indent = "  ".repeat(depth);
        writeln!(f, "{indent}--- {}", self.path)?;
        for pd in &self.properties {
            writeln!(f, "{indent}{}", render_property_diff(pd))?;
        }
        for sd in &self.sub_diffs {
            if sd.is_empty() {
                continue;
            }
            sd.write_at(f, depth + 1)?;
        }
        Ok(())
    }
}

fn render_property_diff(pd: &PropertyDiff) -> String {
    match pd.op {
        DiffOp::Added => format!("+ {}", render_property(&pd.property)),
        DiffOp::Removed => format!("- {}", render_property(&pd.property)),
        DiffOp::Changed => format!(
            "~ {}: {} -> {}",
            pd.property.name, pd.old.value, pd.property.value
        ),
    }
}

/// `NAME[;PARAM=VAL...]:VALUE`, with parameters sorted alphabetically
/// so the rendering is deterministic. No folding, no CRLF — this is
/// display text, not codec output.
fn render_property(p: &Property) -> String {
    let mut out = p.name.clone();
    if !p.params.is_empty() {
        let mut params: Vec<_> = p.params.iter().collect();
        params.sort_by_key(|pr| pr.name.to_ascii_uppercase());
        for pr in params {
            out.push(';');
            out.push_str(&pr.name);
            out.push('=');
            out.push_str(&pr.value);
        }
    }
    out.push(':');
    out.push_str(&p.value);
    out
}

// ---------------------------------------------------------------- //
// Equality                                                          //
// ---------------------------------------------------------------- //

/// Reports whether two components are semantically equal — their
/// canonical bytes match.
///
/// Renamed with an `Equal` suffix in every port. In Go
/// `diff.Component(a, b)` reads correctly because the package qualifier
/// supplies the verb; without it a bare `component(a, b)` returning a
/// boolean is unreadable, and it would collide with
/// [`canonical::component`] in a flat import.
pub fn component_equal(a: &Component, b: &Component) -> bool {
    canonical::component(a) == canonical::component(b)
}

/// Reports whether two cards are semantically equal via
/// [`canonical::card`]. The same `X-VSTAR-HASH` and ordering rules
/// apply.
pub fn card_equal(a: &Card, b: &Card) -> bool {
    canonical::card(a) == canonical::card(b)
}

/// Reports whether two calendars are semantically equal via
/// [`canonical::calendar`].
///
/// This handles top-level component reordering (canonicalization sorts
/// by UID/TZID per spec/03 rule 6) and TZID-tagged datetime resolution
/// against the calendar's VTIMEZONE registry. Unlike [`of_calendar`] it
/// *does* compare `PRODID`, which canonical bytes carry.
pub fn calendar_equal(a: &Calendar, b: &Calendar) -> bool {
    canonical::calendar(a) == canonical::calendar(b)
}

// ---------------------------------------------------------------- //
// Diff                                                              //
// ---------------------------------------------------------------- //

/// The property-level structural difference between two components.
///
/// Both inputs are first stripped of any `X-VSTAR-HASH`. Properties are
/// paired by name case-insensitively and the result sorts by name;
/// sub-components pair by `(type, uid)`, falling back to position
/// within a type bucket for UID-less children.
pub fn of_component(a: &Component, b: &Component) -> ComponentDiff {
    component_diff_at("", a, b)
}

/// The property-level structural difference between two cards.
///
/// Cards have no sub-components, so [`ComponentDiff::sub_diffs`] is
/// always empty and `path` is `""` — callers use the result as a
/// rendering root.
pub fn of_card(a: &Card, b: &Card) -> ComponentDiff {
    ComponentDiff {
        path: String::new(),
        properties: diff_properties(&filter_hash(&a.props), &filter_hash(&b.props)),
        sub_diffs: Vec::new(),
    }
}

/// The per-component structural difference between two calendars.
///
/// Components pair by `(type, uid)` — the same rule sub-components use.
/// Each differing pair becomes one [`ComponentDiff`] with path
/// `VCALENDAR.<TYPE>[uid=<uid>]`; a component present on only one side
/// becomes an all-added or all-removed entry. Unchanged components do
/// not appear at all.
///
/// `PRODID` is deliberately **not** diffed: the calendar's identity —
/// its component set — is what V\* equality cares about here. Use
/// [`calendar_equal`] when `PRODID` matters.
///
/// The returned order is pairing order, which is the documented
/// contract the `spec/behavior/diff` fixtures assert. Do not sort it.
pub fn of_calendar(a: &Calendar, b: &Calendar) -> Vec<ComponentDiff> {
    pair_subs(&a.components, &b.components)
        .into_iter()
        .filter_map(|pr| {
            let path = sub_path("VCALENDAR", &pr.label);
            entry_for(&path, pr.a, pr.b)
        })
        .collect()
}

/// Renders one paired slot as a diff entry, or `None` when nothing
/// changed.
fn entry_for(path: &str, a: Option<&Component>, b: Option<&Component>) -> Option<ComponentDiff> {
    match (a, b) {
        (None, Some(b)) => Some(all_of(path, b, DiffOp::Added)),
        (Some(a), None) => Some(all_of(path, a, DiffOp::Removed)),
        (Some(a), Some(b)) => {
            let d = component_diff_at(path, a, b);
            (!d.is_empty()).then_some(d)
        }
        (None, None) => None,
    }
}

fn component_diff_at(path: &str, a: &Component, b: &Component) -> ComponentDiff {
    ComponentDiff {
        path: path.to_owned(),
        properties: diff_properties(&filter_hash(&a.props), &filter_hash(&b.props)),
        sub_diffs: diff_subs(path, a, b),
    }
}

/// Drops every `X-VSTAR-HASH` from `props`, borrowing the rest.
fn filter_hash(props: &[Property]) -> Vec<&Property> {
    props
        .iter()
        .filter(|p| !p.name.eq_ignore_ascii_case(X_VSTAR_HASH))
        .collect()
}

/// Pairs properties by upper-cased name and emits [`PropertyDiff`]
/// entries sorted by that key.
///
/// Grouping by name keeps multi-valued properties (several `ATTENDEE`,
/// say) intact: the i-th instance on each side pairs with the i-th on
/// the other, and surplus on either side becomes an add or a remove.
fn diff_properties(a: &[&Property], b: &[&Property]) -> Vec<PropertyDiff> {
    let group = |props: &[&Property]| -> BTreeMap<String, Vec<Property>> {
        let mut m: BTreeMap<String, Vec<Property>> = BTreeMap::new();
        for p in props {
            m.entry(p.name.to_ascii_uppercase())
                .or_default()
                .push((*p).clone());
        }
        m
    };
    let ga = group(a);
    let gb = group(b);

    // `BTreeMap` already orders by the upper-cased key, which is the
    // case-insensitive sort the contract calls for.
    let keys: BTreeSet<&String> = ga.keys().chain(gb.keys()).collect();
    let empty: Vec<Property> = Vec::new();
    keys.into_iter()
        .flat_map(|k| diff_property_group(ga.get(k).unwrap_or(&empty), gb.get(k).unwrap_or(&empty)))
        .collect()
}

fn diff_property_group(a: &[Property], b: &[Property]) -> Vec<PropertyDiff> {
    let mut out = Vec::new();
    for i in 0..a.len().max(b.len()) {
        match (a.get(i), b.get(i)) {
            (None, Some(bp)) => out.push(PropertyDiff {
                op: DiffOp::Added,
                property: bp.clone(),
                old: Property::default(),
            }),
            (Some(ap), None) => out.push(PropertyDiff {
                op: DiffOp::Removed,
                property: ap.clone(),
                old: Property::default(),
            }),
            (Some(ap), Some(bp)) if !property_equal(ap, bp) => out.push(PropertyDiff {
                op: DiffOp::Changed,
                property: bp.clone(),
                old: ap.clone(),
            }),
            _ => {}
        }
    }
    out
}

fn diff_subs(parent_path: &str, a: &Component, b: &Component) -> Vec<ComponentDiff> {
    pair_subs(&a.sub, &b.sub)
        .into_iter()
        .filter_map(|pr| entry_for(&sub_path(parent_path, &pr.label), pr.a, pr.b))
        .collect()
}

/// One paired slot: either side may be absent, plus the label segment
/// used to render the path.
struct SubPair<'a> {
    a: Option<&'a Component>,
    b: Option<&'a Component>,
    label: String,
}

/// Pairs sub-components by `(type, uid)`, falling back to position
/// within a type bucket for UID-less children.
///
/// The returned order is: UID-bearing pairs first in `(type, uid)`
/// order, then UID-less buckets in type order paired positionally.
fn pair_subs<'a>(a_sub: &'a [Component], b_sub: &'a [Component]) -> Vec<SubPair<'a>> {
    type Key = (String, String);

    let mut a_by_key: BTreeMap<Key, &Component> = BTreeMap::new();
    let mut b_by_key: BTreeMap<Key, &Component> = BTreeMap::new();
    let mut a_by_type: BTreeMap<String, Vec<&Component>> = BTreeMap::new();
    let mut b_by_type: BTreeMap<String, Vec<&Component>> = BTreeMap::new();

    let collect = |subs: &'a [Component],
                   by_key: &mut BTreeMap<Key, &'a Component>,
                   by_type: &mut BTreeMap<String, Vec<&'a Component>>| {
        for s in subs {
            let typ = s.r#type.as_str().to_owned();
            let uid = s.uid();
            if uid.is_empty() {
                by_type.entry(typ).or_default().push(s);
            } else {
                by_key.insert((typ, uid.to_owned()), s);
            }
        }
    };
    collect(a_sub, &mut a_by_key, &mut a_by_type);
    collect(b_sub, &mut b_by_key, &mut b_by_type);

    let mut out = Vec::new();

    // `BTreeMap` keys are already in (type, uid) order.
    let keys: BTreeSet<&Key> = a_by_key.keys().chain(b_by_key.keys()).collect();
    for k in keys {
        out.push(SubPair {
            a: a_by_key.get(k).copied(),
            b: b_by_key.get(k).copied(),
            label: format!("{}[uid={}]", k.0, k.1),
        });
    }

    let types: BTreeSet<&String> = a_by_type.keys().chain(b_by_type.keys()).collect();
    let empty: Vec<&Component> = Vec::new();
    for t in types {
        let as_ = a_by_type.get(t).unwrap_or(&empty);
        let bs = b_by_type.get(t).unwrap_or(&empty);
        for i in 0..as_.len().max(bs.len()) {
            out.push(SubPair {
                a: as_.get(i).copied(),
                b: bs.get(i).copied(),
                label: format!("{t}[#{i}]"),
            });
        }
    }
    out
}

fn sub_path(parent: &str, label: &str) -> String {
    if parent.is_empty() {
        label.to_owned()
    } else {
        format!("{parent}.{label}")
    }
}

/// Renders a whole component as added or removed: every property
/// (except `X-VSTAR-HASH`) becomes one op, sorted by name, recursing
/// into sub-components the same way.
fn all_of(path: &str, c: &Component, op: DiffOp) -> ComponentDiff {
    let mut properties: Vec<PropertyDiff> = filter_hash(&c.props)
        .into_iter()
        .map(|p| PropertyDiff {
            op,
            property: p.clone(),
            old: Property::default(),
        })
        .collect();
    properties.sort_by_key(|pd| pd.property.name.to_ascii_uppercase());

    ComponentDiff {
        path: path.to_owned(),
        properties,
        sub_diffs: c
            .sub
            .iter()
            .map(|s| all_of(&sub_path(path, &sub_label(s)), s, op))
            .collect(),
    }
}

/// The label segment for an unpaired sub-component, mirroring the
/// format [`pair_subs`] chooses.
fn sub_label(c: &Component) -> String {
    let uid = c.uid();
    if uid.is_empty() {
        format!("{}[#0]", c.r#type.as_str())
    } else {
        format!("{}[uid={uid}]", c.r#type.as_str())
    }
}
