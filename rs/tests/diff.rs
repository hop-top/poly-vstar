// SPDX-License-Identifier: MIT

//! The `behavior/diff` gate: every `<name>.a.ics` / `<name>.b.ics` pair
//! under `spec/behavior/diff/` and the `<name>.diff.json` the reference
//! records for it.
//!
//! **Order is the contract.** Components come out in pairing order, not
//! sorted by path; ops come out sorted by property name,
//! case-insensitively. The gate compares the two sequences positionally
//! so a port that sorts the component list fails here rather than
//! passing on a set comparison.

mod support;

use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::diff::{
    calendar_equal, card_equal, component_equal, of_calendar, of_card, of_component, ComponentDiff,
    DiffOp, PropertyDiff,
};
use hop_top_vstar::{Calendar, Card, CompType, Component, Param, Property};
use support::json::Json;

/// The lowercase op tokens the fixtures record. The reference's
/// `DiffOp` `Display` is capitalized for humans; these are the wire
/// spellings.
fn op_token(op: DiffOp) -> &'static str {
    match op {
        DiffOp::Added => "add",
        DiffOp::Removed => "remove",
        DiffOp::Changed => "change",
    }
}

/// Lifts the uid out of a rendered path such as
/// `VCALENDAR.VTODO[uid=todo-1]`, mirroring the generator. Returns `""`
/// for a positional path — the component has no UID to key on.
fn uid_from_path(path: &str) -> &str {
    const MARKER: &str = "[uid=";
    match (path.rfind(MARKER), path.ends_with(']')) {
        (Some(i), true) => &path[i + MARKER.len()..path.len() - 1],
        _ => "",
    }
}

/// Reads a `<field>_params` list off a fixture op, defaulting to empty
/// when the key is absent (the generator omits it when there are none).
fn want_params(op: &Json, key: &str) -> Vec<(String, String)> {
    match op.get(key) {
        None | Some(Json::Null) => Vec::new(),
        Some(list) => list
            .array()
            .iter()
            .map(|p| {
                (
                    p.str_field("name").expect("param has a name").to_owned(),
                    p.str_field("value").expect("param has a value").to_owned(),
                )
            })
            .collect(),
    }
}

fn got_params(params: &[Param]) -> Vec<(String, String)> {
    params
        .iter()
        .map(|p| (p.name.clone(), p.value.clone()))
        .collect()
}

/// Reads a nullable string field: absent or `null` is `None`.
fn want_str<'a>(op: &'a Json, key: &str) -> Option<&'a str> {
    match op.get(key) {
        None | Some(Json::Null) => None,
        Some(Json::String(s)) => Some(s.as_str()),
        other => panic!("diff fixture: {key} is {other:?}, want string or null"),
    }
}

/// Asserts one `ops` list against one [`ComponentDiff`]'s properties.
fn assert_ops(got: &[PropertyDiff], want: &[Json], label: &str) {
    assert_eq!(
        got.len(),
        want.len(),
        "{label}: op count — got {:?}, want {:?}",
        got.iter()
            .map(|p| (op_token(p.op), p.property.name.as_str()))
            .collect::<Vec<_>>(),
        want.iter()
            .map(|o| (
                o.str_field("op").unwrap_or("?"),
                o.str_field("property").unwrap_or("?")
            ))
            .collect::<Vec<_>>()
    );

    for (i, (g, w)) in got.iter().zip(want).enumerate() {
        let at = format!("{label}.ops[{i}]");
        assert_eq!(
            op_token(g.op),
            w.str_field("op").expect("op token"),
            "{at}: op kind"
        );

        // The surviving side carries the name in every case; the
        // reference falls back to `old` only when `property` is zero.
        let name = if g.property.name.is_empty() {
            g.old.name.as_str()
        } else {
            g.property.name.as_str()
        };
        assert_eq!(
            name,
            w.str_field("property").expect("property"),
            "{at}: name"
        );

        let (before, before_params, after, after_params) = match g.op {
            DiffOp::Added => (
                None,
                Vec::new(),
                Some(g.property.value.as_str()),
                got_params(&g.property.params),
            ),
            DiffOp::Removed => (
                Some(g.property.value.as_str()),
                got_params(&g.property.params),
                None,
                Vec::new(),
            ),
            DiffOp::Changed => (
                Some(g.old.value.as_str()),
                got_params(&g.old.params),
                Some(g.property.value.as_str()),
                got_params(&g.property.params),
            ),
        };

        assert_eq!(before, want_str(w, "before"), "{at}: before");
        assert_eq!(after, want_str(w, "after"), "{at}: after");
        assert_eq!(
            before_params,
            want_params(w, "before_params"),
            "{at}: before_params"
        );
        assert_eq!(
            after_params,
            want_params(w, "after_params"),
            "{at}: after_params"
        );
    }
}

/// Asserts a `[ComponentDiff]` sequence against a fixture array,
/// positionally — the order IS the contract.
fn assert_component_diffs(got: &[ComponentDiff], want: &[Json], label: &str) {
    assert_eq!(
        got.len(),
        want.len(),
        "{label}: component count — got paths {:?}, want paths {:?}",
        got.iter().map(|d| d.path.as_str()).collect::<Vec<_>>(),
        want.iter()
            .map(|d| d.str_field("path").unwrap_or("?"))
            .collect::<Vec<_>>()
    );

    for (i, (g, w)) in got.iter().zip(want).enumerate() {
        let at = format!("{label}[{i}]");
        assert_eq!(
            g.path,
            w.str_field("path").expect("path"),
            "{at}: path (pairing order is the contract — do NOT sort)"
        );
        assert_eq!(
            uid_from_path(&g.path),
            w.str_field("uid").expect("uid"),
            "{at}: uid"
        );
        assert_ops(&g.properties, w.get("ops").expect("ops").array(), &at);

        // The fixture omits `subs` when empty; the reference carries
        // unchanged sub-diffs internally and the generator drops them.
        let nested: Vec<&ComponentDiff> = g.sub_diffs.iter().filter(|s| !s.is_empty()).collect();
        let want_subs: &[Json] = match w.get("subs") {
            None | Some(Json::Null) => &[],
            Some(list) => list.array(),
        };
        assert_eq!(
            nested.len(),
            want_subs.len(),
            "{at}: sub-diff count — got {:?}",
            nested.iter().map(|d| d.path.as_str()).collect::<Vec<_>>()
        );
        let owned: Vec<ComponentDiff> = nested.into_iter().cloned().collect();
        assert_component_diffs(&owned, want_subs, &format!("{at}.subs"));
    }
}

/// The gate proper: every `*.diff.json` under `spec/behavior/diff/`.
#[test]
fn diff_fixtures_match_the_reference() {
    let stems = support::behavior_stems("diff", ".diff.json");
    assert!(
        stems.len() >= 7,
        "behavior/diff shrank to {} cases",
        stems.len()
    );

    for stem in &stems {
        let a =
            rfc5545::parse(support::behavior_input("diff", &format!("{stem}.a.ics")).as_slice())
                .unwrap_or_else(|e| panic!("{stem}.a.ics: {e}"));
        let b =
            rfc5545::parse(support::behavior_input("diff", &format!("{stem}.b.ics")).as_slice())
                .unwrap_or_else(|e| panic!("{stem}.b.ics: {e}"));
        let want = support::behavior_json("diff", &format!("{stem}.diff.json"));

        assert_component_diffs(&of_calendar(&a, &b), want.array(), stem);
    }
}

/// `identical.diff.json` is `[]` — two equal documents produce no
/// entry, not an entry with no ops. This is the assertion that stops a
/// port from passing by reporting every component as changed.
#[test]
fn identical_calendars_produce_no_entries() {
    let a = rfc5545::parse(support::behavior_input("diff", "identical.a.ics").as_slice())
        .expect("identical.a.ics parses");
    let b = rfc5545::parse(support::behavior_input("diff", "identical.b.ics").as_slice())
        .expect("identical.b.ics parses");
    assert!(of_calendar(&a, &b).is_empty());
    // `calendar_equal` is NOT expected to agree here: the two fixtures
    // carry different PRODIDs by construction, and canonical bytes carry
    // PRODID while `of_calendar` deliberately ignores it. That the two
    // disagree is exactly the distinction both functions exist to draw.
    assert_ne!(a.prod_id, b.prod_id);
    assert!(!calendar_equal(&a, &b));
    assert!(calendar_equal(&a, &a));
}

/// Component order is **pairing order**, not sorted by path.
///
/// Every `spec/behavior/diff` fixture yields exactly one component
/// diff, so none of them can tell the two orders apart — a port that
/// sorted its output would pass the whole fixture gate. This case is
/// built so the two orders genuinely differ: `pair_subs` emits the
/// UID-bearing components first, in `(type, uid)` order, and only then
/// the UID-less ones positionally, so the UID-less VTODO lands *after*
/// a VTODO whose path sorts later than its own.
#[test]
fn component_order_is_pairing_order_not_sorted_by_path() {
    /// A VTODO with an optional UID and one `SUMMARY`, so every entry
    /// below has a real op and none is filtered out as unchanged.
    fn todo(uid: Option<&str>, summary: &str) -> Component {
        let mut c = Component::new(CompType::TODO);
        if let Some(uid) = uid {
            c.add(Property::new("UID", uid));
        }
        c.add(Property::new("SUMMARY", summary));
        c
    }

    let a = Calendar {
        prod_id: "-//Order//EN".into(),
        components: vec![
            todo(None, "positional before"),
            todo(Some("zzz-last"), "keyed before"),
        ],
    };
    let b = Calendar {
        prod_id: "-//Order//EN".into(),
        components: vec![
            todo(None, "positional after"),
            todo(Some("zzz-last"), "keyed after"),
        ],
    };

    let paths: Vec<String> = of_calendar(&a, &b).into_iter().map(|d| d.path).collect();
    assert_eq!(
        paths,
        vec!["VCALENDAR.VTODO[uid=zzz-last]", "VCALENDAR.VTODO[#0]",],
        "components come out in pairing order — UID-bearing first, then \
         positional. Sorting by path would put VTODO[#0] first, and the \
         single-component behavior fixtures cannot catch that."
    );
}

/// `X-VSTAR-HASH` is excluded from both sides: a diff reports what
/// changed in the content, not the restamped hash that followed.
#[test]
fn hash_property_is_excluded_from_both_sides() {
    let mut a = Component::new(CompType::TODO);
    a.add(Property::new("UID", "todo-hash"));
    a.add(Property::new("X-VSTAR-HASH", "sha256:aaaa"));

    let mut b = Component::new(CompType::TODO);
    b.add(Property::new("UID", "todo-hash"));
    b.add(Property::new("X-VSTAR-HASH", "sha256:bbbb"));

    assert!(
        of_component(&a, &b).is_empty(),
        "a differing X-VSTAR-HASH must not surface as a change"
    );
}

/// Ops are sorted by property name case-insensitively, which is a
/// different order from the property order on the wire.
#[test]
fn ops_sort_by_property_name_case_insensitively() {
    let mut a = Component::new(CompType::TODO);
    a.add(Property::new("UID", "todo-sort"));

    let mut b = Component::new(CompType::TODO);
    b.add(Property::new("UID", "todo-sort"));
    b.add(Property::new("zeta", "z"));
    b.add(Property::new("ALPHA", "a"));
    b.add(Property::new("mid", "m"));

    let d = of_component(&a, &b);
    let names: Vec<&str> = d
        .properties
        .iter()
        .map(|p| p.property.name.as_str())
        .collect();
    assert_eq!(names, vec!["ALPHA", "mid", "zeta"]);
}

/// The three equality functions route through canonical bytes, so
/// property order and parameter order are irrelevant.
#[test]
fn equality_ignores_property_and_parameter_order() {
    let mut a = Component::new(CompType::TODO);
    a.add(Property::new("UID", "todo-eq"));
    a.add(Property::new("DTSTAMP", "20260504T120000Z"));
    a.add(
        Property::new("DUE", "20260101T000000")
            .with_param("TZID", "America/Montreal")
            .with_param("VALUE", "DATE-TIME"),
    );

    let mut b = Component::new(CompType::TODO);
    b.add(
        Property::new("DUE", "20260101T000000")
            .with_param("VALUE", "DATE-TIME")
            .with_param("TZID", "America/Montreal"),
    );
    b.add(Property::new("DTSTAMP", "20260504T120000Z"));
    b.add(Property::new("UID", "todo-eq"));

    assert!(component_equal(&a, &b));
    assert!(of_component(&a, &b).is_empty());

    let mut different = b.clone();
    different.set(Property::new("SUMMARY", "changed"));
    assert!(!component_equal(&a, &different));
}

/// `of_card` diffs a card's properties; cards have no sub-components so
/// `sub_diffs` is always empty and `path` is the empty rendering root.
#[test]
fn card_diff_reports_property_changes() {
    let mut a = Card {
        uid: "urn:uuid:1".into(),
        kind: None,
        props: Vec::new(),
    };
    a.add(Property::new("FN", "Jad Bitar"));

    let mut b = a.clone();
    b.set(Property::new("FN", "J. Bitar"));

    assert!(!card_equal(&a, &b));
    let d = of_card(&a, &b);
    assert_eq!(d.path, "");
    assert!(d.sub_diffs.is_empty());
    assert_eq!(d.properties.len(), 1);
    assert_eq!(d.properties[0].op, DiffOp::Changed);
    assert_eq!(d.properties[0].old.value, "Jad Bitar");
    assert_eq!(d.properties[0].property.value, "J. Bitar");
}

/// `Display for ComponentDiff` renders the unified-diff-ish block, and
/// an empty diff renders as the empty string.
#[test]
fn display_renders_a_unified_diff_block() {
    let mut a = Component::new(CompType::TODO);
    a.add(Property::new("UID", "todo-render"));
    a.add(Property::new("DUE", "20260101T000000Z"));

    let mut b = Component::new(CompType::TODO);
    b.add(Property::new("UID", "todo-render"));
    b.add(Property::new("DUE", "20260202T000000Z"));
    b.add(Property::new("SUMMARY", "new"));

    let rendered = of_component(&a, &b).to_string();
    assert_eq!(
        rendered,
        "--- \n~ DUE: 20260101T000000Z -> 20260202T000000Z\n+ SUMMARY:new\n"
    );

    assert_eq!(of_component(&a, &a).to_string(), "");
}

/// `is_empty` recurses: a component whose own properties are unchanged
/// but whose sub-component changed is NOT empty.
#[test]
fn is_empty_recurses_into_sub_diffs() {
    let mut alarm_a = Component::new(CompType::ALARM);
    alarm_a.add(Property::new("UID", "alarm-1"));
    alarm_a.add(Property::new("ACTION", "DISPLAY"));

    let mut a = Component::new(CompType::EVENT);
    a.add(Property::new("UID", "evt-1"));
    a.sub.push(alarm_a.clone());

    let mut b = a.clone();
    b.sub[0].set(Property::new("ACTION", "AUDIO"));

    let d = of_component(&a, &b);
    assert!(d.properties.is_empty(), "top level itself is unchanged");
    assert!(
        !d.is_empty(),
        "a changed sub-component makes the diff non-empty"
    );
    assert_eq!(d.sub_diffs.len(), 1);
    assert_eq!(d.sub_diffs[0].path, "VALARM[uid=alarm-1]");
}

/// `of_calendar` deliberately does NOT diff `PRODID`: the component set
/// is what V* equality cares about.
#[test]
fn of_calendar_ignores_prodid() {
    let mut c = Component::new(CompType::TODO);
    c.add(Property::new("UID", "todo-prodid"));
    c.add(Property::new("DTSTAMP", "20260504T120000Z"));

    let a = Calendar {
        prod_id: "-//A//EN".into(),
        components: vec![c.clone()],
    };
    let b = Calendar {
        prod_id: "-//B//EN".into(),
        components: vec![c],
    };

    assert!(of_calendar(&a, &b).is_empty());
    // `calendar_equal` is the counterpart that DOES notice — it routes
    // through canonical bytes, which carry PRODID.
    assert!(!calendar_equal(&a, &b));
}
