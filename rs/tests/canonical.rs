// SPDX-License-Identifier: MIT

//! The layer-(b) gate: for every conformance fixture carrying a
//! `.canonical` sibling, this port's canonical bytes MUST equal the
//! file's, byte for byte; for every `.hash` sibling, the hash string
//! MUST equal the file's contents.
//!
//! The comparison compares BYTES (`Vec<u8>`). The single licensed
//! transform is stripping `\r` before `\n` in OUR produced bytes — the
//! corpus is LF on disk, the canonical form is CRLF (spec rule 1). The
//! transform runs on our output, never on the file's, because
//! converting the file's LF up to CRLF would silently repair a bare
//! `\n` our encoder should never have emitted.
//!
//! The hash is taken over the CRLF bytes, not the LF-transformed ones.
//!
//! Nothing here hard-codes a fixture name: the loader walks the tree,
//! so a fixture added to the corpus is asserted without a test edit.

mod support;

use hop_top_vstar::codec::{rfc5545, rfc6350};
use hop_top_vstar::{canonical, hashing};
use support::{assert_bytes_eq, cases_in, corpus_root, crlf_to_lf, Case};

/// The corpus families whose inputs are VCALENDAR documents.
const CALENDAR_FAMILIES: [&str; 2] = ["rfc5545", "supersession"];

/// The `.hash` sibling's contents with surrounding whitespace trimmed.
fn want_hash(case: &Case) -> String {
    let raw = case
        .sibling("hash")
        .unwrap_or_else(|| panic!("{} has no .hash sibling", case.stem));
    String::from_utf8(raw)
        .expect(".hash file is valid UTF-8")
        .trim()
        .to_owned()
}

/// The `.canonical` sibling's raw bytes, exactly as they sit on disk.
fn want_canonical(case: &Case) -> Vec<u8> {
    case.sibling("canonical")
        .unwrap_or_else(|| panic!("{} has no .canonical sibling", case.stem))
}

#[test]
fn every_calendar_fixture_canonicalizes_to_the_corpus_bytes() {
    let mut checked = 0;
    for family in CALENDAR_FAMILIES {
        for case in cases_in(&corpus_root().join(family), "ics") {
            let cal = rfc5545::parse(case.input.as_slice())
                .unwrap_or_else(|e| panic!("{family}/{}: parse: {e}", case.stem));
            let got = canonical::calendar(&cal);
            assert_bytes_eq(
                &crlf_to_lf(&got),
                &want_canonical(&case),
                &format!("{family}/{}.canonical", case.stem),
            );
            checked += 1;
        }
    }
    assert!(
        checked >= 24,
        "expected the full calendar corpus, saw {checked}"
    );
}

#[test]
fn every_calendar_fixture_hashes_to_the_corpus_hash() {
    for family in CALENDAR_FAMILIES {
        for case in cases_in(&corpus_root().join(family), "ics") {
            let cal = rfc5545::parse(case.input.as_slice())
                .unwrap_or_else(|e| panic!("{family}/{}: parse: {e}", case.stem));
            assert_eq!(
                hashing::calendar(&cal),
                want_hash(&case),
                "{family}/{}.hash",
                case.stem
            );
        }
    }
}

#[test]
fn every_card_fixture_canonicalizes_to_the_corpus_bytes() {
    for case in cases_in(&corpus_root().join("rfc6350"), "vcf") {
        let cards = rfc6350::parse(case.input.as_slice())
            .unwrap_or_else(|e| panic!("rfc6350/{}: parse: {e}", case.stem));
        assert_eq!(
            cards.len(),
            1,
            "rfc6350/{}: one card per fixture",
            case.stem
        );
        let got = canonical::card(&cards[0]);
        assert_bytes_eq(
            &crlf_to_lf(&got),
            &want_canonical(&case),
            &format!("rfc6350/{}.canonical", case.stem),
        );
    }
}

#[test]
fn every_card_fixture_hashes_to_the_corpus_hash() {
    for case in cases_in(&corpus_root().join("rfc6350"), "vcf") {
        let cards = rfc6350::parse(case.input.as_slice())
            .unwrap_or_else(|e| panic!("rfc6350/{}: parse: {e}", case.stem));
        assert_eq!(
            hashing::card(&cards[0]),
            want_hash(&case),
            "rfc6350/{}.hash",
            case.stem
        );
    }
}

#[test]
fn every_fixture_carries_both_siblings() {
    for family in CALENDAR_FAMILIES {
        for case in cases_in(&corpus_root().join(family), "ics") {
            assert!(
                case.sibling("canonical").is_some(),
                "{family}/{}.canonical is missing",
                case.stem
            );
            assert!(
                case.sibling("hash").is_some(),
                "{family}/{}.hash is missing",
                case.stem
            );
        }
    }
}

#[test]
fn canonical_form_emits_no_bare_lf() {
    for case in cases_in(&corpus_root().join("rfc5545"), "ics") {
        let cal = rfc5545::parse(case.input.as_slice()).expect("fixture parses");
        let got = canonical::calendar(&cal);
        for (i, b) in got.iter().enumerate() {
            if *b == b'\n' {
                assert_eq!(
                    got.get(i.wrapping_sub(1)),
                    Some(&b'\r'),
                    "{}: bare LF at offset {i}",
                    case.stem
                );
            }
        }
    }
}

#[test]
fn hashing_is_deterministic_over_one_hundred_runs() {
    let case = cases_in(&corpus_root().join("rfc5545"), "ics")
        .into_iter()
        .find(|c| c.stem == "world")
        .expect("the world fixture is in the corpus");
    let cal = rfc5545::parse(case.input.as_slice()).expect("fixture parses");
    let first = hashing::calendar(&cal);
    for i in 0..100 {
        let again = rfc5545::parse(case.input.as_slice()).expect("fixture parses");
        assert_eq!(hashing::calendar(&again), first, "run {i} diverged");
        assert_eq!(
            canonical::calendar(&again),
            canonical::calendar(&cal),
            "run {i} produced different canonical bytes"
        );
    }
}

#[test]
fn canonical_form_is_a_fixpoint_under_repeated_canonicalization() {
    // NFC is idempotent and sorting is stable, so re-parsing the
    // canonical bytes and canonicalizing again must reproduce them.
    for case in cases_in(&corpus_root().join("rfc5545"), "ics") {
        let cal = rfc5545::parse(case.input.as_slice()).expect("fixture parses");
        let once = canonical::calendar(&cal);
        let reparsed = rfc5545::parse(once.as_slice())
            .unwrap_or_else(|e| panic!("{}: canonical bytes re-parse: {e}", case.stem));
        assert_bytes_eq(
            &canonical::calendar(&reparsed),
            &once,
            &format!("{} is not a canonicalization fixpoint", case.stem),
        );
    }
}

#[test]
fn canonical_form_is_independent_of_input_component_order() {
    // Rule 6 sorts top-level components, so reversing the parsed
    // component list must not change a single byte.
    for case in cases_in(&corpus_root().join("rfc5545"), "ics") {
        let cal = rfc5545::parse(case.input.as_slice()).expect("fixture parses");
        if cal.components.len() < 2 {
            continue;
        }
        let mut reversed = cal.clone();
        reversed.components.reverse();
        assert_bytes_eq(
            &canonical::calendar(&reversed),
            &canonical::calendar(&cal),
            &format!("{} depends on input component order", case.stem),
        );
        assert_eq!(
            hashing::calendar(&reversed),
            hashing::calendar(&cal),
            "{}: hash depends on input component order",
            case.stem
        );
    }
}
