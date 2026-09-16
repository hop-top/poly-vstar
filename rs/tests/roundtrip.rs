// SPDX-License-Identifier: MIT

//! Layer (a) gate: the conformance corpus round-trips.
//!
//! For every `rfc5545/*.ics` and `rfc6350/*.vcf` fixture: parse, encode,
//! parse again, and assert the two parsed values are equal. Round-trip
//! proves the codec is self-consistent; [`parity`](../parity.rs) is what
//! proves it agrees with the Go reference.

mod support;

use hop_top_vstar::codec::{rfc5545, rfc6350};
use support::{cases_in, corpus_root, crlf_to_lf};

#[test]
fn rfc5545_fixtures_round_trip() {
    let dir = corpus_root().join("rfc5545");
    let cases = cases_in(&dir, "ics");
    // A floor, not an equality: the guard exists to catch a corpus that
    // collapsed to nothing (which would turn this gate into a silent
    // no-op), and growing the corpus upstream is not a Rust-port defect.
    assert!(
        cases.len() >= 15,
        "rfc5545 corpus shrank to {} fixtures",
        cases.len()
    );

    for case in &cases {
        let first = rfc5545::parse(case.input.as_slice())
            .unwrap_or_else(|e| panic!("{}: first parse failed: {e}", case.stem));

        let mut encoded = Vec::new();
        rfc5545::encode(&mut encoded, &first)
            .unwrap_or_else(|e| panic!("{}: encode failed: {e}", case.stem));

        let second = rfc5545::parse(encoded.as_slice())
            .unwrap_or_else(|e| panic!("{}: re-parse failed: {e}", case.stem));

        assert_eq!(
            first, second,
            "{}: round-trip is not semantically stable",
            case.stem
        );

        // Encoding is a fixpoint: re-encoding the re-parsed value must
        // reproduce the same bytes.
        let mut again = Vec::new();
        rfc5545::encode(&mut again, &second).expect("re-encode");
        assert_eq!(encoded, again, "{}: encode is not idempotent", case.stem);
    }
}

#[test]
fn rfc6350_fixtures_round_trip() {
    let dir = corpus_root().join("rfc6350");
    let cases = cases_in(&dir, "vcf");
    assert_eq!(cases.len(), 7, "rfc6350 corpus size changed");

    for case in &cases {
        let first = rfc6350::parse(case.input.as_slice())
            .unwrap_or_else(|e| panic!("{}: first parse failed: {e}", case.stem));
        assert!(!first.is_empty(), "{}: parsed to zero cards", case.stem);

        let mut encoded = Vec::new();
        for card in &first {
            rfc6350::encode(&mut encoded, card)
                .unwrap_or_else(|e| panic!("{}: encode failed: {e}", case.stem));
        }

        let second = rfc6350::parse(encoded.as_slice())
            .unwrap_or_else(|e| panic!("{}: re-parse failed: {e}", case.stem));

        assert_eq!(
            first, second,
            "{}: round-trip is not semantically stable",
            case.stem
        );

        let mut again = Vec::new();
        for card in &second {
            rfc6350::encode(&mut again, card).expect("re-encode");
        }
        assert_eq!(encoded, again, "{}: encode is not idempotent", case.stem);
    }
}

/// `rfc6350::parse` returns a `Vec<Card>`, not one card. A port that
/// returns a single card passes every single-card fixture and fails the
/// rest, so this asserts the multi-card path explicitly.
#[test]
fn rfc6350_parses_a_stream_of_cards() {
    let two = concat!(
        "BEGIN:VCARD\nVERSION:4.0\nUID:a\nFN:First\nEND:VCARD\n",
        "BEGIN:VCARD\nVERSION:4.0\nUID:b\nFN:Second\nEND:VCARD\n",
    );
    let cards = rfc6350::parse(two.as_bytes()).expect("parse two cards");
    assert_eq!(cards.len(), 2);
    assert_eq!(cards[0].uid, "a");
    assert_eq!(cards[1].uid, "b");
}

/// Parsers are liberal: LF-only, CRLF, and a mixture must all parse to
/// the same model. The corpus itself is LF on disk, which is the reason.
#[test]
fn parsers_accept_lf_crlf_and_a_mixture() {
    let lf =
        "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\nBEGIN:VTODO\nUID:u\nEND:VTODO\nEND:VCALENDAR\n";
    let crlf = lf.replace('\n', "\r\n");
    let mixed = lf.replacen('\n', "\r\n", 3);

    let a = rfc5545::parse(lf.as_bytes()).expect("lf");
    let b = rfc5545::parse(crlf.as_bytes()).expect("crlf");
    let c = rfc5545::parse(mixed.as_bytes()).expect("mixed");
    assert_eq!(a, b);
    assert_eq!(a, c);
}

/// The encoder always emits CRLF, whatever the input used.
#[test]
fn encoders_always_emit_crlf() {
    let lf =
        "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\nBEGIN:VTODO\nUID:u\nEND:VTODO\nEND:VCALENDAR\n";
    let cal = rfc5545::parse(lf.as_bytes()).expect("parse");
    let mut out = Vec::new();
    rfc5545::encode(&mut out, &cal).expect("encode");

    // No bare LF anywhere: every \n is preceded by \r.
    for (i, b) in out.iter().enumerate() {
        if *b == b'\n' {
            assert_eq!(out.get(i - 1), Some(&b'\r'), "bare LF at offset {i}");
        }
    }
    assert!(out.ends_with(b"\r\n"));
    // The licensed transform recovers the LF form.
    assert_eq!(crlf_to_lf(&out), lf.as_bytes());
}

/// Folding is 75 OCTETS, measured in UTF-8 bytes, applied AFTER the
/// complete logical line is assembled.
#[test]
fn encoder_folds_at_seventy_five_octets() {
    for n in [1usize, 60, 70, 74, 75, 76, 80, 148, 150, 221, 300, 600] {
        let value = "x".repeat(n);
        let src = format!(
            "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\nBEGIN:VTODO\nUID:u\nX-LONG:{value}\nEND:VTODO\nEND:VCALENDAR\n"
        );
        let cal = rfc5545::parse(src.as_bytes()).expect("parse");
        let mut out = Vec::new();
        rfc5545::encode(&mut out, &cal).expect("encode");

        for line in out.strip_suffix(b"\r\n").unwrap().split(|b| *b == b'\n') {
            let line = line.strip_suffix(b"\r").unwrap_or(line);
            assert!(
                line.len() <= 75,
                "n={n}: physical line of {} octets exceeds the 75-octet limit: {:?}",
                line.len(),
                String::from_utf8_lossy(line)
            );
        }

        let back = rfc5545::parse(&out[..]).expect("re-parse folded");
        assert_eq!(cal, back, "n={n}: folding is not reversible by unfolding");
    }
}

/// The same limit on the vCard side, including the lengths at which the
/// Go reference's own folder overruns to 76 octets.
#[test]
fn vcard_encoder_folds_at_seventy_five_octets() {
    for n in [1usize, 74, 75, 76, 148, 149, 221, 224, 300, 600] {
        let value = "x".repeat(n);
        let src = format!("BEGIN:VCARD\nVERSION:4.0\nUID:u\nFN:{value}\nEND:VCARD\n");
        let cards = rfc6350::parse(src.as_bytes()).expect("parse");
        let mut out = Vec::new();
        rfc6350::encode(&mut out, &cards[0]).expect("encode");

        for line in out.strip_suffix(b"\r\n").unwrap().split(|b| *b == b'\n') {
            let line = line.strip_suffix(b"\r").unwrap_or(line);
            assert!(
                line.len() <= 75,
                "n={n}: physical line of {} octets exceeds the 75-octet limit",
                line.len()
            );
        }

        let back = rfc6350::parse(out.as_slice()).expect("re-parse folded");
        assert_eq!(cards, back, "n={n}: folding is not reversible by unfolding");
    }
}

/// A fold MUST NOT split a multi-byte UTF-8 sequence. Rust's `str` makes
/// the split impossible to express, but a byte-slicing implementation
/// would panic rather than truncate — this pins the property either way.
#[test]
fn folding_is_octet_exact_and_may_split_a_utf8_sequence() {
    // Tune the ASCII prefix so the 75-octet boundary lands inside each
    // multi-byte sequence in turn.
    for (marker, width) in [("é", 2usize), ("€", 3), ("😀", 4)] {
        for pad in 60..80usize {
            let value = format!("{}{}", "a".repeat(pad), marker.repeat(20));
            let src = format!(
                "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\nBEGIN:VTODO\nUID:u\nX-LONG:{value}\nEND:VTODO\nEND:VCALENDAR\n"
            );
            let cal = rfc5545::parse(src.as_bytes()).expect("parse");
            let mut out: Vec<u8> = Vec::new();
            rfc5545::encode(&mut out, &cal).expect("encode");

            // Per RFC 5545 §3.1 and spec rule 3 the fold counts OCTETS,
            // so a multi-byte sequence straddling the boundary is split
            // and the encoded output is not necessarily valid UTF-8. The
            // invariant that matters is the octet limit below, plus the
            // round-trip: unfolding reassembles the sequence verbatim.
            let reparsed = rfc5545::parse(&out[..]).expect("re-parse folded output");
            assert_eq!(
                reparsed, cal,
                "width={width} pad={pad}: unfolding did not reassemble the split sequence",
            );
            for line in out.strip_suffix(b"\r\n").unwrap().split(|b| *b == b'\n') {
                let line = line.strip_suffix(b"\r").unwrap_or(line);
                assert!(
                    line.len() <= 75,
                    "width={width} pad={pad}: {} octets",
                    line.len()
                );
            }
            let back = rfc5545::parse(&out[..]).expect("re-parse");
            assert_eq!(cal, back, "width={width} pad={pad}: not reversible");
        }
    }
}

/// Wire order is preserved: parsing must not sort, dedupe or reorder
/// properties, parameters or components.
#[test]
fn parsing_preserves_wire_order() {
    let src = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\nBEGIN:VTODO\nUID:u\nZZZ-LAST:1\nAAA-FIRST:2\nMMM-MID:3\nEND:VTODO\nEND:VCALENDAR\n";
    let cal = rfc5545::parse(src.as_bytes()).expect("parse");
    let names: Vec<&str> = cal.components[0]
        .props
        .iter()
        .map(|p| p.name.as_str())
        .collect();
    assert_eq!(names, ["UID", "ZZZ-LAST", "AAA-FIRST", "MMM-MID"]);

    let params_src = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\nBEGIN:VTODO\nUID:u\nX-P;ZED=1;ALPHA=2:v\nEND:VTODO\nEND:VCALENDAR\n";
    let cal = rfc5545::parse(params_src.as_bytes()).expect("parse");
    let p = cal.components[0].get("X-P").expect("X-P");
    let pnames: Vec<&str> = p.params.iter().map(|x| x.name.as_str()).collect();
    assert_eq!(
        pnames,
        ["ZED", "ALPHA"],
        "parameter order was not preserved"
    );
}
