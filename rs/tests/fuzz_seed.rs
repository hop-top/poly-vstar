// SPDX-License-Identifier: MIT

//! Layer (a) gate: no seed input panics.
//!
//! Every `fuzz-seed/**/*.bytes` file must produce an `Ok` or an `Err` —
//! never a panic. That rules out `unwrap`, `expect`, slice indexing and
//! arithmetic overflow anywhere on a parse path, which is the class of
//! bug a fuzz corpus exists to surface.

mod support;

use hop_top_vstar::codec::{rfc5545, rfc6350};
use support::{cases_under, corpus_root};

#[test]
fn rfc5545_fuzz_seeds_never_panic() {
    let dir = corpus_root().join("fuzz-seed").join("rfc5545");
    let cases = cases_under(&dir, "bytes");
    assert_eq!(cases.len(), 12, "rfc5545 fuzz-seed corpus size changed");

    for case in &cases {
        // The result is deliberately unexamined: Ok and Err are both
        // acceptable answers. Reaching this line at all is the assertion.
        match rfc5545::parse(case.input.as_slice()) {
            Ok(cal) => {
                // An Ok must also survive a re-encode without panicking.
                let mut out = Vec::new();
                let _ = rfc5545::encode(&mut out, &cal);
            }
            Err(e) => {
                // The sentinel must still be one of the twelve.
                assert!(
                    e.sentinel().starts_with("Err"),
                    "{}: sentinel {} is not a Go-spelled identifier",
                    case.stem,
                    e.sentinel()
                );
            }
        }
    }
}

#[test]
fn rfc6350_fuzz_seeds_never_panic() {
    let dir = corpus_root().join("fuzz-seed").join("rfc6350");
    let cases = cases_under(&dir, "bytes");
    assert_eq!(cases.len(), 12, "rfc6350 fuzz-seed corpus size changed");

    for case in &cases {
        match rfc6350::parse(case.input.as_slice()) {
            Ok(cards) => {
                for card in &cards {
                    let mut out = Vec::new();
                    let _ = rfc6350::encode(&mut out, card);
                }
            }
            Err(e) => assert!(
                e.sentinel().starts_with("Err"),
                "{}: sentinel {} is not a Go-spelled identifier",
                case.stem,
                e.sentinel()
            ),
        }
    }
}

/// Truncating every seed at every byte offset is the cheap generalization
/// of the seed corpus, and it is where an index-out-of-bounds or a
/// mid-sequence UTF-8 slice actually surfaces.
#[test]
fn truncated_seeds_never_panic() {
    let root = corpus_root().join("fuzz-seed");

    for case in cases_under(&root.join("rfc5545"), "bytes") {
        for n in 0..=case.input.len() {
            let _ = rfc5545::parse(&case.input[..n]);
        }
    }
    for case in cases_under(&root.join("rfc6350"), "bytes") {
        for n in 0..=case.input.len() {
            let _ = rfc6350::parse(&case.input[..n]);
        }
    }
}

/// Hand-built adversarial inputs covering the shapes the corpus does not:
/// unbalanced quotes, empty names, stray separators, deep nesting.
#[test]
fn adversarial_inputs_never_panic() {
    let inputs: &[&str] = &[
        "",
        "\n",
        "\r\n",
        ":",
        ";",
        "BEGIN:",
        "BEGIN:VCALENDAR",
        "BEGIN:VCALENDAR\n",
        "BEGIN:VCALENDAR\n:\nEND:VCALENDAR\n",
        "BEGIN:VCALENDAR\nVERSION:2.0\nX;=:v\nEND:VCALENDAR\n",
        "BEGIN:VCALENDAR\nVERSION:2.0\nX;A=\"unbalanced:v\nEND:VCALENDAR\n",
        "BEGIN:VCALENDAR\nVERSION:2.0\nX;A=\"q;q:q\":v\nEND:VCALENDAR\n",
        "BEGIN:VCALENDAR\nEND:VTODO\n",
        "END:VCALENDAR\n",
        " continuation with no pending line\n",
        "\n\n\n \n",
        "BEGIN:VCALENDAR\nSUMMARY:trailing backslash\\\nEND:VCALENDAR\n",
        "BEGIN:VCARD\nVERSION:4.0\nVERSION:4.0\nUID:u\nEND:VCARD\n",
        "BEGIN:VCARD\nBEGIN:VCARD\nEND:VCARD\n",
        "BEGIN:VCARD\nVERSION:4.0\n.LEADING-DOT:v\nUID:u\nEND:VCARD\n",
        "BEGIN:VCARD\nVERSION:4.0\ngroup.:v\nUID:u\nEND:VCARD\n",
    ];

    for src in inputs {
        let _ = rfc5545::parse(src.as_bytes());
        let _ = rfc6350::parse(src.as_bytes());
    }

    // Deep BEGIN nesting must not blow the stack at a corpus-plausible depth.
    let mut deep = String::from("BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\n");
    for _ in 0..64 {
        deep.push_str("BEGIN:VTODO\n");
    }
    for _ in 0..64 {
        deep.push_str("END:VTODO\n");
    }
    deep.push_str("END:VCALENDAR\n");
    let _ = rfc5545::parse(deep.as_bytes());

    // Non-UTF-8 bytes must be an error, not a panic.
    let _ = rfc5545::parse(&b"BEGIN:VCALENDAR\nSUMMARY:\xff\xfe\nEND:VCALENDAR\n"[..]);
    let _ = rfc6350::parse(&b"BEGIN:VCARD\nVERSION:4.0\nFN:\xff\xfe\nUID:u\nEND:VCARD\n"[..]);
}
