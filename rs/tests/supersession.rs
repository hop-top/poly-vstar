// SPDX-License-Identifier: MIT

//! The `behavior/supersession` gate: the status each ledger projects
//! onto each component.
//!
//! The inputs are the *conformance* fixtures at
//! `spec/v1.0/conformance/supersession/<name>.ics` — the behavior family
//! mints no `.ics` of its own, so a port that already loads the
//! conformance corpus gets this table keyed by the same file names.
//!
//! A UID absent from the map is **not superseded**. That is why
//! `corrupt_mutated.effective.json` is `{}`: its hash is broken but no
//! supersession entry targets it, and supersession is a projection
//! query, not a validator.

mod support;

use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::hashing;
use hop_top_vstar::supersession::{
    superseded, supersedes, CATEGORY_STATUS_SUPERSESSION, PROP_EFFECTIVE_STATUS,
};
use hop_top_vstar::{parse_time, CompType, Component, Error, Property};
use std::collections::BTreeMap;
use support::json::Json;

/// Parses a form #2 timestamp or panics — test-local convenience.
fn at(s: &str) -> chrono::DateTime<chrono::Utc> {
    parse_time(s).unwrap_or_else(|| panic!("{s} is not a form #2 timestamp"))
}

/// The gate proper: walk every `*.effective.json`, load its sibling
/// conformance `.ics`, and project the ledger over every component.
#[test]
fn effective_status_fixtures_match_the_reference() {
    let stems = support::behavior_stems("supersession", ".effective.json");
    assert!(
        stems.len() >= 5,
        "behavior/supersession shrank to {} cases",
        stems.len()
    );

    for stem in &stems {
        let path = support::corpus_root()
            .join("supersession")
            .join(format!("{stem}.ics"));
        let bytes = std::fs::read(&path).unwrap_or_else(|e| panic!("read {}: {e}", path.display()));
        let cal = rfc5545::parse(bytes.as_slice()).unwrap_or_else(|e| panic!("{stem}.ics: {e}"));

        let mut got = BTreeMap::new();
        for c in &cal.components {
            if let Some(status) = superseded(c, &cal.components) {
                let uid = c.uid();
                assert!(
                    !uid.is_empty(),
                    "{stem}: a superseded component carries no UID"
                );
                got.insert(uid.to_owned(), status);
            }
        }

        let want_doc = support::behavior_json("supersession", &format!("{stem}.effective.json"));
        let want: BTreeMap<String, String> = match &want_doc {
            Json::Object(m) => m
                .iter()
                .map(|(k, v)| match v {
                    Json::String(s) => (k.clone(), s.clone()),
                    other => panic!("{stem}: status {other:?} is not a string"),
                })
                .collect(),
            other => panic!("{stem}.effective.json is {other:?}, want an object"),
        };

        assert_eq!(got, want, "{stem}: projected effective statuses");
    }
}

/// `corrupt_mutated` projects nothing: no supersession entry targets it.
/// Its hash violation belongs to the validate family, not this one.
#[test]
fn a_corrupt_target_is_not_a_supersession_failure() {
    let path = support::corpus_root()
        .join("supersession")
        .join("corrupt_mutated.ics");
    let bytes = std::fs::read(&path).expect("read corrupt_mutated.ics");
    let cal = rfc5545::parse(bytes.as_slice()).expect("corrupt_mutated.ics parses");

    for c in &cal.components {
        assert_eq!(
            superseded(c, &cal.components),
            None,
            "nothing in corrupt_mutated supersedes {}",
            c.uid()
        );
    }
}

/// A VTODO whose stored hash verifies can be superseded, and the record
/// the call returns carries the six properties the spec enumerates.
#[test]
fn supersedes_builds_the_journal_record() {
    let mut target = Component::new(CompType::TODO);
    target.set(Property::new("UID", "todo-1"));
    target.set(Property::new("DTSTAMP", "20260504T120000Z"));
    target.set(Property::new("SUMMARY", "Buy milk"));
    hashing::set_x_vstar(&mut target);

    let journal = supersedes(&target, "COMPLETED", at("20260504T143000Z"))
        .expect("a verifying target is supersedable");

    assert_eq!(journal.r#type, CompType::JOURNAL);
    assert_eq!(journal.uid(), "journal:status:todo-1:20260504T143000Z");
    assert_eq!(journal.dtstamp_raw(), "20260504T143000Z");
    assert_eq!(
        journal.get("RELATED-TO").map(|p| p.value.as_str()),
        Some("todo-1")
    );
    assert_eq!(
        journal.get("CATEGORIES").map(|p| p.value.as_str()),
        Some(CATEGORY_STATUS_SUPERSESSION)
    );
    assert_eq!(
        journal.get(PROP_EFFECTIVE_STATUS).map(|p| p.value.as_str()),
        Some("COMPLETED")
    );

    // The stored hash covers everything set above: it is refreshed last.
    let (ok, want, got) = hashing::verify_x_vstar(&journal);
    assert!(
        ok,
        "the record's own hash must verify: want {want}, got {got}"
    );

    // And the record, placed in a ledger, projects onto the target.
    assert_eq!(
        superseded(&target, &[journal]),
        Some("COMPLETED".to_owned())
    );
}

/// The integrity check is the point of `supersedes` returning a
/// `Result`: a target mutated after it was hashed is refused.
#[test]
fn supersedes_refuses_a_corrupted_target() {
    let mut target = Component::new(CompType::TODO);
    target.set(Property::new("UID", "todo-corrupt"));
    target.set(Property::new("DTSTAMP", "20260504T120000Z"));
    target.set(Property::new("SUMMARY", "Buy milk"));
    hashing::set_x_vstar(&mut target);

    // Mutate AFTER hashing — the append-only discipline's forbidden move.
    target.set(Property::new("SUMMARY", "Buy oat milk"));

    let err = supersedes(&target, "COMPLETED", at("20260504T143000Z"))
        .expect_err("a mutated target must be refused");
    assert_eq!(err.sentinel(), "ErrTargetCorrupted");
    assert!(matches!(err, Error::TargetCorrupted(_)));
}

/// A target carrying no `X-VSTAR-HASH` makes no integrity claim, so
/// there is nothing to verify and the call proceeds.
#[test]
fn supersedes_accepts_an_unhashed_target() {
    let mut target = Component::new(CompType::TODO);
    target.set(Property::new("UID", "todo-unhashed"));
    target.set(Property::new("DTSTAMP", "20260504T120000Z"));
    assert!(hashing::get_x_vstar(&target).is_none());

    let journal = supersedes(&target, "CANCELLED", at("20260504T143000Z"))
        .expect("an unhashed target carries no integrity claim");
    assert_eq!(
        journal.uid(),
        "journal:status:todo-unhashed:20260504T143000Z"
    );
}

/// `supersedes` does not mutate its target — the append-only contract
/// depends on it.
#[test]
fn supersedes_does_not_mutate_the_target() {
    let mut target = Component::new(CompType::TODO);
    target.set(Property::new("UID", "todo-immutable"));
    target.set(Property::new("DTSTAMP", "20260504T120000Z"));
    hashing::set_x_vstar(&mut target);

    let before = target.clone();
    let _ = supersedes(&target, "COMPLETED", at("20260504T143000Z")).expect("supersedes");
    assert_eq!(target, before);
}

/// Two entries targeting the same component: the later `DTSTAMP` wins.
#[test]
fn the_latest_dtstamp_wins() {
    let mut target = Component::new(CompType::TODO);
    target.set(Property::new("UID", "todo-multi"));

    let ledger = vec![
        journal_entry("todo-multi", "IN-PROCESS", "20260504T100000Z"),
        journal_entry("todo-multi", "COMPLETED", "20260504T143000Z"),
        journal_entry("todo-multi", "NEEDS-ACTION", "20260504T090000Z"),
    ];

    assert_eq!(
        superseded(&target, &ledger),
        Some("COMPLETED".to_owned()),
        "the latest DTSTAMP wins regardless of ledger position"
    );
}

/// An unparseable `DTSTAMP` sorts to the zero time — demoted, never
/// fatal. `Superseded` is a query, not a validator.
#[test]
fn an_unparseable_dtstamp_is_demoted_not_fatal() {
    let mut target = Component::new(CompType::TODO);
    target.set(Property::new("UID", "todo-noise"));

    let ledger = vec![
        journal_entry("todo-noise", "COMPLETED", "20260504T100000Z"),
        journal_entry("todo-noise", "GARBAGE", "not-a-timestamp"),
    ];
    assert_eq!(superseded(&target, &ledger), Some("COMPLETED".to_owned()));
}

/// Every reason `superseded` reports "no": empty ledger, no UID on the
/// target, nothing pointing at it, a non-VJOURNAL entry, a missing
/// category, and a matching entry with no effective-status property.
#[test]
fn superseded_reports_absence_faithfully() {
    let mut target = Component::new(CompType::TODO);
    target.set(Property::new("UID", "todo-absent"));

    assert_eq!(superseded(&target, &[]), None, "empty ledger");

    let uidless = Component::new(CompType::TODO);
    let full = vec![journal_entry("", "COMPLETED", "20260504T100000Z")];
    assert_eq!(superseded(&uidless, &full), None, "target has no UID");

    let other = vec![journal_entry("todo-other", "COMPLETED", "20260504T100000Z")];
    assert_eq!(superseded(&target, &other), None, "points elsewhere");

    // A VTODO carrying the same properties is not a supersession entry.
    let mut wrong_type = journal_entry("todo-absent", "COMPLETED", "20260504T100000Z");
    wrong_type.r#type = CompType::TODO;
    assert_eq!(superseded(&target, &[wrong_type]), None, "not a VJOURNAL");

    let mut no_category = journal_entry("todo-absent", "COMPLETED", "20260504T100000Z");
    no_category.remove("CATEGORIES");
    assert_eq!(superseded(&target, &[no_category]), None, "no category");

    let mut no_status = journal_entry("todo-absent", "COMPLETED", "20260504T100000Z");
    no_status.remove(PROP_EFFECTIVE_STATUS);
    assert_eq!(
        superseded(&target, &[no_status]),
        None,
        "no status property"
    );
}

/// `CATEGORIES` is comma-delimited per RFC 5545 §3.8.1.2: each token is
/// trimmed and compared case-insensitively, so a longer label that
/// merely *contains* the token does not falsely match.
#[test]
fn categories_match_per_token_not_by_substring() {
    let mut target = Component::new(CompType::TODO);
    target.set(Property::new("UID", "todo-cat"));

    let mut among_others = journal_entry("todo-cat", "COMPLETED", "20260504T100000Z");
    among_others.set(Property::new(
        "CATEGORIES",
        "work, STATUS-SUPERSESSION ,urgent",
    ));
    assert_eq!(
        superseded(&target, &[among_others]),
        Some("COMPLETED".to_owned()),
        "a trimmed, case-folded token among others matches"
    );

    let mut longer = journal_entry("todo-cat", "COMPLETED", "20260504T100000Z");
    longer.set(Property::new("CATEGORIES", "status-supersession-deferred"));
    assert_eq!(
        superseded(&target, &[longer]),
        None,
        "a label that merely contains the token must not match"
    );
}

/// Builds a supersession VJOURNAL by hand, so the ledger tests do not
/// depend on `supersedes` being correct.
fn journal_entry(target_uid: &str, status: &str, dtstamp: &str) -> Component {
    let mut c = Component::new(CompType::JOURNAL);
    c.set(Property::new(
        "UID",
        format!("journal:status:{target_uid}:{dtstamp}"),
    ));
    c.set(Property::new("DTSTAMP", dtstamp));
    c.set(Property::new("RELATED-TO", target_uid));
    c.set(Property::new("CATEGORIES", CATEGORY_STATUS_SUPERSESSION));
    c.set(Property::new(PROP_EFFECTIVE_STATUS, status));
    c
}

/// The two constants keep their wire spellings.
#[test]
fn constants_carry_the_wire_spellings() {
    assert_eq!(CATEGORY_STATUS_SUPERSESSION, "status-supersession");
    assert_eq!(PROP_EFFECTIVE_STATUS, "X-VSTAR-EFFECTIVE-STATUS");
}
