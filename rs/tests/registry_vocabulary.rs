// SPDX-License-Identifier: MIT

//! Reconciles this crate's wire enums with the registry vocabularies.
//!
//! The two are built from different sources on purpose. The validator's
//! table is projected from the wire enums the codec encodes against, so
//! it cannot disagree with what the library writes; the generated
//! tables are rendered from `spec/registry/status-vocabulary.json`, so
//! they cannot disagree with the other four ports. These tests are the
//! join — what lets both guarantees hold at once. Without them,
//! generating the table would buy cross-language agreement by giving up
//! the codec linkage, and deriving it would buy the codec linkage by
//! giving up cross-language agreement.
//!
//! Order matters, not just membership: the VS044 message joins the
//! allowed values, so a reordering is a user-visible change that the
//! behavior fixtures pin.

use hop_top_vstar::generated::codes::{
    CLASS_VOCABULARY, RELTYPE_VOCABULARY, STATUS_VOCABULARY, TRANSP_VOCABULARY,
};
use hop_top_vstar::{Class, CompType, EventStatus, JournalStatus, RelType, TodoStatus, Transp};

/// Projected from the wire enums, exactly as the validator's table is —
/// not read back out of the generated module, which would make the
/// assertion vacuous.
fn from_enums() -> Vec<(&'static str, Vec<&'static str>)> {
    vec![
        (
            "VEVENT",
            EventStatus::ALL.iter().map(EventStatus::as_str).collect(),
        ),
        (
            "VJOURNAL",
            JournalStatus::ALL
                .iter()
                .map(JournalStatus::as_str)
                .collect(),
        ),
        (
            "VTODO",
            TodoStatus::ALL.iter().map(TodoStatus::as_str).collect(),
        ),
    ]
}

#[test]
fn status_vocabulary_matches_wire_enums() {
    let want = from_enums();

    assert_eq!(
        STATUS_VOCABULARY.len(),
        want.len(),
        "registry scopes STATUS to {} component type(s), the wire enums to {}",
        STATUS_VOCABULARY.len(),
        want.len()
    );

    for (comp, want_values) in &want {
        let got = STATUS_VOCABULARY
            .iter()
            .find(|(c, _)| c == comp)
            .unwrap_or_else(|| panic!("registry has no STATUS vocabulary for {comp}"))
            .1;
        assert_eq!(
            got,
            want_values.as_slice(),
            "{comp}: registry and wire enums disagree"
        );
    }
}

#[test]
fn status_vocabulary_component_types_are_the_named_ones() {
    // The keys have to be the wire spellings CompType uses, or the
    // validator's lookup would miss them in every port.
    for (comp, _) in STATUS_VOCABULARY {
        assert!(
            CompType::from_wire(comp).is_named(),
            "{comp} is not a named component type"
        );
    }
}

#[test]
fn class_vocabulary_matches_wire_enums() {
    let want: Vec<&str> = Class::ALL.iter().map(Class::as_str).collect();
    assert_eq!(CLASS_VOCABULARY.as_slice(), want.as_slice());
}

#[test]
fn transp_vocabulary_matches_wire_enums() {
    let want: Vec<&str> = Transp::ALL.iter().map(Transp::as_str).collect();
    assert_eq!(TRANSP_VOCABULARY.as_slice(), want.as_slice());
}

/// RELTYPE is an open vocabulary — RFC 5545 §3.2.15 admits IANA and `X-`
/// values — so the assertion is that the registry lists exactly the
/// *registered* set the crate names, not that no other value may appear
/// on the wire.
#[test]
fn reltype_vocabulary_matches_the_registered_set() {
    let want: Vec<String> = RelType::registered()
        .map(|r| r.as_str().to_owned())
        .collect();
    let got: Vec<String> = RELTYPE_VOCABULARY.iter().map(|s| (*s).to_owned()).collect();
    assert_eq!(got, want);
}

/// Every registered value the registry lists must round-trip through
/// `parse_rel_type` as registered. A value the registry claims but the
/// crate does not recognise would be a silent cross-port divergence.
#[test]
fn every_registry_reltype_is_registered_in_the_crate() {
    for value in RELTYPE_VOCABULARY {
        let (parsed, registered) = hop_top_vstar::parse_rel_type(value);
        assert!(registered, "{value} is in the registry but not registered");
        assert_eq!(parsed.as_str(), value);
    }
}
