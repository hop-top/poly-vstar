// SPDX-License-Identifier: MIT

//! The `behavior/ext` gate: every extension-name classification the
//! reference records in `spec/behavior/ext/scopes.json`.
//!
//! The fixture's `scope` column is the reference's `Scope.String()`
//! lowercased — the generator writes `strings.ToLower(ScopeOf(name).
//! String())` — so this gate lowercases [`Scope`]'s `Display` the same
//! way rather than introducing a second spelling of the same value.

mod support;

use hop_top_vstar::ext::{extensions_by_scope, is_extension, scope_of, system_name, Scope};
use hop_top_vstar::{CompType, Component, Property};
use support::json::Json;

/// Walks every row of `ext/scopes.json` and asserts the classification,
/// the system-slug extraction and the `is_extension` predicate agree
/// with the reference.
#[test]
fn scopes_fixture_classifies_every_name() {
    let doc = support::behavior_json("ext", "scopes.json");
    let rows = doc.array();
    assert!(
        rows.len() >= 17,
        "ext/scopes.json shrank to {} rows — the gate is losing coverage",
        rows.len()
    );

    for row in rows {
        let name = row.str_field("name").expect("row has a name");
        let want_scope = row.str_field("scope").expect("row has a scope");

        let got = scope_of(name);
        assert_eq!(
            got.to_string().to_lowercase(),
            want_scope,
            "scope_of({name:?})"
        );

        // `system` is a string for a system-scoped name and JSON null
        // for every other scope.
        let want_system = match row.get("system") {
            None | Some(Json::Null) => None,
            Some(Json::String(s)) => Some(s.clone()),
            other => panic!("ext/scopes.json {name:?}: unexpected system field {other:?}"),
        };
        assert_eq!(system_name(name), want_system, "system_name({name:?})");

        // Every scope but `none` implies the `X-` prefix; `none` denies
        // it. That is the reference's decision tree read backwards, and
        // it catches a port whose predicate and classifier disagree.
        assert_eq!(
            is_extension(name),
            want_scope != "none",
            "is_extension({name:?})"
        );
    }
}

/// Every one of the five scopes appears in the fixture, so a port
/// cannot pass by getting four right and never exercising the fifth.
#[test]
fn scopes_fixture_covers_every_scope() {
    let doc = support::behavior_json("ext", "scopes.json");
    for want in ["vstar", "system", "experimental", "none", "unknown"] {
        assert!(
            doc.array()
                .iter()
                .any(|r| r.str_field("scope") == Some(want)),
            "ext/scopes.json carries no {want:?} row"
        );
    }
}

/// `Display` renders the reference's capitalized spelling. The fixture
/// column is that value lowercased, so pinning both halves keeps the
/// wire token and the display string from drifting into each other.
#[test]
fn scope_display_matches_the_reference_spelling() {
    assert_eq!(Scope::None.to_string(), "None");
    assert_eq!(Scope::VStar.to_string(), "VStar");
    assert_eq!(Scope::System.to_string(), "System");
    assert_eq!(Scope::Experimental.to_string(), "Experimental");
    assert_eq!(Scope::Unknown.to_string(), "Unknown");
}

/// `ScopeNone` is the Go zero value and must stay the Rust default.
#[test]
fn scope_none_is_the_default() {
    assert_eq!(Scope::default(), Scope::None);
}

/// `extensions_by_scope` filters in property order without sorting and
/// without recursing into sub-components.
#[test]
fn extensions_by_scope_filters_in_property_order() {
    let mut c = Component::new(CompType::TODO);
    c.add(Property::new("UID", "todo-ext"));
    c.add(Property::new("X-VSTAR-HASH", "sha256:deadbeef"));
    c.add(Property::new("X-AGR-INTENT", "ship"));
    c.add(Property::new("X-EXP-DRAFT", "yes"));
    c.add(Property::new("X-ACME-TICKET-ID", "T-9"));
    c.add(Property::new("SUMMARY", "Ship the port"));
    c.add(Property::new("X-FOO", "no tier"));

    let mut child = Component::new(CompType::ALARM);
    child.add(Property::new("X-VSTAR-NESTED", "not counted"));
    c.sub.push(child);

    let names = |scope| -> Vec<&str> {
        extensions_by_scope(&c, scope)
            .into_iter()
            .map(|p| p.name.as_str())
            .collect()
    };

    assert_eq!(names(Scope::VStar), vec!["X-VSTAR-HASH"]);
    // System-scoped entries keep property order — AGR before ACME,
    // which is the order they were added and NOT alphabetical.
    assert_eq!(
        names(Scope::System),
        vec!["X-AGR-INTENT", "X-ACME-TICKET-ID"]
    );
    assert_eq!(names(Scope::Experimental), vec!["X-EXP-DRAFT"]);
    assert_eq!(names(Scope::Unknown), vec!["X-FOO"]);
    assert_eq!(names(Scope::None), vec!["UID", "SUMMARY"]);
}

/// The reservation on `VSTAR` and `EXP` covers the whole slug segment,
/// not a prefix of it.
#[test]
fn slug_reservation_is_whole_segment() {
    assert_eq!(scope_of("X-VSTARLIKE-FOO"), Scope::System);
    assert_eq!(system_name("X-VSTARLIKE-FOO").as_deref(), Some("VSTARLIKE"));
    assert_eq!(scope_of("X-EXPANSE-FOO"), Scope::System);
    assert_eq!(system_name("X-EXPANSE-FOO").as_deref(), Some("EXPANSE"));
}

/// Classification folds case; the extracted slug is normalized upward
/// so callers compare without re-normalizing.
#[test]
fn classification_folds_case_and_uppercases_the_slug() {
    assert_eq!(scope_of("x-vstar-hash"), Scope::VStar);
    assert_eq!(scope_of("x-exp-draft"), Scope::Experimental);
    assert_eq!(system_name("x-agr-intent").as_deref(), Some("AGR"));
}
