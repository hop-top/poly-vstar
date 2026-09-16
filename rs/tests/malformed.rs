// SPDX-License-Identifier: MIT

//! Layer (a) gate: every `malformed/*` fixture produces its named sentinel.
//!
//! Failing is not enough — failing with the *wrong* sentinel is not a
//! pass. The `.error` sibling names the Go spelling (`ErrMalformed` and
//! friends) and [`Error::sentinel`] must return exactly that string.
//!
//! One fixture is deliberately not a parse failure. `missing_uid.vcf`
//! parses cleanly: the RFC 6350 parser accepts a UID-less VCARD by
//! design, and `ErrMissingUID` is raised by the **encoder**. A
//! port that implements only the parse side passes the fixture while
//! being wrong, so this file exercises both halves.

mod support;

use hop_top_vstar::codec::{rfc5545, rfc6350};
use hop_top_vstar::Error;
use support::{cases_in, corpus_root, sentinel_name};

#[test]
fn malformed_ics_fixtures_produce_their_sentinel() {
    let dir = corpus_root().join("malformed");
    let cases = cases_in(&dir, "ics");
    assert_eq!(cases.len(), 2, "malformed .ics corpus size changed");

    for case in &cases {
        let want = sentinel_name(case);
        match rfc5545::parse(case.input.as_slice()) {
            Err(e) => assert_eq!(
                e.sentinel(),
                want,
                "{}: parse failed with {} but the fixture names {want}",
                case.stem,
                e.sentinel()
            ),
            Ok(_) => panic!("{}: parse succeeded; expected {want}", case.stem),
        }
    }
}

#[test]
fn malformed_vcf_fixtures_produce_their_sentinel() {
    let dir = corpus_root().join("malformed");
    let cases = cases_in(&dir, "vcf");
    assert_eq!(cases.len(), 2, "malformed .vcf corpus size changed");

    for case in &cases {
        let want = sentinel_name(case);

        let cards = match rfc6350::parse(case.input.as_slice()) {
            Err(e) => {
                assert_eq!(
                    e.sentinel(),
                    want,
                    "{}: parse failed with {} but the fixture names {want}",
                    case.stem,
                    e.sentinel()
                );
                continue;
            }
            Ok(cards) => cards,
        };

        // Parse succeeded, so the fixture targets an encoder-time
        // sentinel. Re-encode every card and require one of them to
        // refuse with exactly the named sentinel.
        assert!(
            !cards.is_empty(),
            "{}: parse returned no cards and no error — the encoder-time \
             sentinel {want} cannot be exercised",
            case.stem
        );
        let mut saw = false;
        for card in &cards {
            let mut sink = Vec::new();
            match rfc6350::encode(&mut sink, card) {
                Ok(()) => continue,
                Err(e) => {
                    assert_eq!(
                        e.sentinel(),
                        want,
                        "{}: encode failed with {} but the fixture names {want}",
                        case.stem,
                        e.sentinel()
                    );
                    saw = true;
                    break;
                }
            }
        }
        assert!(
            saw,
            "{}: expected {want} during parse or encode; both succeeded",
            case.stem
        );
    }
}

/// The asymmetry, stated directly rather than inferred from a fixture:
/// the parser accepts a UID-less VCARD, the encoder refuses one.
#[test]
fn missing_uid_is_encoder_only() {
    let src = "BEGIN:VCARD\nVERSION:4.0\nFN:No UID Here\nEND:VCARD\n";

    let cards =
        rfc6350::parse(src.as_bytes()).expect("the rfc6350 parser accepts a UID-less VCARD");
    assert_eq!(cards.len(), 1);
    assert_eq!(cards[0].uid, "", "a UID-less card carries an empty uid");

    let mut sink = Vec::new();
    let err = rfc6350::encode(&mut sink, &cards[0])
        .expect_err("the rfc6350 encoder refuses a UID-less card");
    assert_eq!(err.sentinel(), "ErrMissingUID");
    assert!(matches!(err, Error::MissingUid(_)));

    // And the guard is real, not incidental: the same card with a UID
    // encodes without complaint.
    let mut ok_card = cards[0].clone();
    ok_card.uid = "u".into();
    let mut sink = Vec::new();
    rfc6350::encode(&mut sink, &ok_card).expect("a card with a UID encodes");
    assert!(sink.starts_with(b"BEGIN:VCARD\r\n"));
    assert!(
        String::from_utf8_lossy(&sink).contains("UID:u\r\n"),
        "the encoder emits the UID it was given"
    );
}

/// All twelve sentinels exist, are distinguishable, and spell their Go
/// identifier verbatim. Only the first four are reachable at layer (a);
/// the rest are constructed directly so the identifier space is complete
/// from the first layer onward.
#[test]
fn all_twelve_sentinels_carry_their_go_identifier() {
    let all = [
        (Error::Malformed(String::new()), "ErrMalformed"),
        (Error::UnclosedBlock(String::new()), "ErrUnclosedBlock"),
        (
            Error::UnsupportedVersion(String::new()),
            "ErrUnsupportedVersion",
        ),
        (Error::MissingUid(String::new()), "ErrMissingUID"),
        (
            Error::UnsupportedRRule(String::new()),
            "ErrUnsupportedRRule",
        ),
        (Error::IterationCap(String::new()), "ErrIterationCap"),
        (
            Error::UnboundedExpansion(String::new()),
            "ErrUnboundedExpansion",
        ),
        (Error::TargetCorrupted(String::new()), "ErrTargetCorrupted"),
        (Error::AlreadyClosed(String::new()), "ErrAlreadyClosed"),
        (Error::HeaderLocked(String::new()), "ErrHeaderLocked"),
        (Error::NoTrigger(String::new()), "ErrNoTrigger"),
        (Error::NoAnchor(String::new()), "ErrNoAnchor"),
    ];
    assert_eq!(all.len(), 12);

    let mut seen = std::collections::BTreeSet::new();
    for (err, want) in &all {
        assert_eq!(err.sentinel(), *want);
        assert!(seen.insert(err.sentinel()), "{want} is not distinguishable");
        // Display carries context; the sentinel is the stable identity.
        let _: &dyn std::error::Error = err;
        assert!(!err.to_string().is_empty());
    }
    assert_eq!(seen.len(), 12);
}

/// Positional context travels with the sentinel rather than replacing it,
/// mirroring Go's `fmt.Errorf("line %d: %w", n, ErrMalformed)`.
#[test]
fn errors_carry_context_without_losing_the_sentinel() {
    let err = rfc5545::parse(&b"BEGIN:VCALENDAR\nno-colon-here\nEND:VCALENDAR\n"[..])
        .expect_err("a content line without a colon is malformed");
    assert_eq!(err.sentinel(), "ErrMalformed");
    assert!(
        err.to_string().contains("no-colon-here"),
        "the message should name the offending line, got {err}"
    );
}

#[test]
fn unsupported_version_is_distinct_from_malformed() {
    let ics = "BEGIN:VCALENDAR\nVERSION:1.0\nPRODID:p\nEND:VCALENDAR\n";
    let err = rfc5545::parse(ics.as_bytes()).expect_err("VERSION:1.0 is unsupported");
    assert_eq!(err.sentinel(), "ErrUnsupportedVersion");

    let vcf = "BEGIN:VCARD\nVERSION:3.0\nUID:u\nEND:VCARD\n";
    let err = rfc6350::parse(vcf.as_bytes()).expect_err("VERSION:3.0 is unsupported");
    assert_eq!(err.sentinel(), "ErrUnsupportedVersion");
}

#[test]
fn unclosed_block_is_distinct_from_malformed() {
    let ics = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\nBEGIN:VTODO\nUID:u\n";
    let err = rfc5545::parse(ics.as_bytes()).expect_err("VTODO is never closed");
    assert_eq!(err.sentinel(), "ErrUnclosedBlock");

    let vcf = "BEGIN:VCARD\nVERSION:4.0\nUID:u\n";
    let err = rfc6350::parse(vcf.as_bytes()).expect_err("VCARD is never closed");
    assert_eq!(err.sentinel(), "ErrUnclosedBlock");
}
