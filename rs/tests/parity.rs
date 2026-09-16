// SPDX-License-Identifier: MIT

//! Reference parity: this port's encoder bytes equal the Go reference's.
//!
//! Round-trip proves self-consistency, not agreement. A codec that sorts
//! properties on parse round-trips perfectly and disagrees with every
//! other implementation — only a byte comparison against the reference
//! catches it.
//!
//! The `.encoded` files under `tests/parity/` hold the exact output of
//! `rfc5545.Encode` / `rfc6350.Encode` over each corpus fixture, CRLF
//! and all. They are compared as `Vec<u8>`: never as `String`, never
//! trimmed, never re-decoded.

mod support;

use hop_top_vstar::codec::{rfc5545, rfc6350};
use support::{cases_in, corpus_root, parity_root};

/// Renders a byte difference as the first mismatching offset plus a
/// window of context, so a failure names the fold point rather than
/// dumping two 600-byte blobs.
fn diff_report(got: &[u8], want: &[u8]) -> String {
    let at = got
        .iter()
        .zip(want)
        .position(|(a, b)| a != b)
        .unwrap_or(got.len().min(want.len()));
    let lo = at.saturating_sub(24);
    let hi_g = (at + 24).min(got.len());
    let hi_w = (at + 24).min(want.len());
    format!(
        "first difference at offset {at} (got {} bytes, want {} bytes)\n  got:  {:?}\n  want: {:?}",
        got.len(),
        want.len(),
        String::from_utf8_lossy(&got[lo..hi_g]),
        String::from_utf8_lossy(&want[lo..hi_w]),
    )
}

#[test]
fn rfc5545_encoder_matches_the_go_reference_bytes() {
    let fixtures = cases_in(&corpus_root().join("rfc5545"), "ics");
    let dir = parity_root().join("rfc5545");

    for case in &fixtures {
        let want = std::fs::read(dir.join(format!("{}.encoded", case.stem)))
            .unwrap_or_else(|e| panic!("{}: missing parity bytes: {e}", case.stem));

        let cal = rfc5545::parse(case.input.as_slice())
            .unwrap_or_else(|e| panic!("{}: parse failed: {e}", case.stem));
        let mut got = Vec::new();
        rfc5545::encode(&mut got, &cal)
            .unwrap_or_else(|e| panic!("{}: encode failed: {e}", case.stem));

        assert_eq!(
            got,
            want,
            "{}: encoder bytes diverge from the Go reference\n{}",
            case.stem,
            diff_report(&got, &want)
        );
    }
}

#[test]
fn rfc6350_encoder_matches_the_go_reference_bytes() {
    let fixtures = cases_in(&corpus_root().join("rfc6350"), "vcf");
    let dir = parity_root().join("rfc6350");

    for case in &fixtures {
        let want = std::fs::read(dir.join(format!("{}.encoded", case.stem)))
            .unwrap_or_else(|e| panic!("{}: missing parity bytes: {e}", case.stem));

        let cards = rfc6350::parse(case.input.as_slice())
            .unwrap_or_else(|e| panic!("{}: parse failed: {e}", case.stem));
        let mut got = Vec::new();
        for card in &cards {
            rfc6350::encode(&mut got, card)
                .unwrap_or_else(|e| panic!("{}: encode failed: {e}", case.stem));
        }

        assert_eq!(
            got,
            want,
            "{}: encoder bytes diverge from the Go reference\n{}",
            case.stem,
            diff_report(&got, &want)
        );
    }
}

/// `encode_component` emits exactly what `encode` puts inside the
/// VCALENDAR wrapper — no wrapper, no VERSION, no PRODID — which is what
/// makes it the shared building block canonicalization will reuse.
#[test]
fn encode_component_matches_the_wrapped_form() {
    for case in cases_in(&corpus_root().join("rfc5545"), "ics") {
        let cal = rfc5545::parse(case.input.as_slice()).expect("parse");

        let mut whole = Vec::new();
        rfc5545::encode(&mut whole, &cal).expect("encode calendar");

        let mut pieces = Vec::new();
        for comp in &cal.components {
            rfc5545::encode_component(&mut pieces, comp).expect("encode component");
        }

        // Take the header from the encoder rather than rebuilding it by
        // interpolation: PRODID is TEXT-typed, so `encode` escapes it,
        // and a hand-built `PRODID:{prod_id}` diverges the moment the
        // value contains a backslash. Encoding the same calendar with no
        // components yields exactly the wrapper, which is what this test
        // wants to hold constant anyway.
        let bare = hop_top_vstar::Calendar {
            prod_id: cal.prod_id.clone(),
            components: Vec::new(),
        };
        let mut wrapper = Vec::new();
        rfc5545::encode(&mut wrapper, &bare).expect("encode wrapper");
        const TRAILER: &[u8] = b"END:VCALENDAR\r\n";
        let header = &wrapper[..wrapper.len() - TRAILER.len()];

        let mut expected = header.to_vec();
        expected.extend_from_slice(&pieces);
        expected.extend_from_slice(TRAILER);

        assert_eq!(
            whole, expected,
            "{}: component encoding is not compositional",
            case.stem
        );
    }
}

/// The parity bytes themselves must obey the RFC: CRLF terminated, no
/// physical line over 75 octets. A regenerated parity set that violated
/// either would quietly license a broken encoder.
///
/// The expected file count is derived from the corpus rather than
/// written down: the two `*_matches_the_go_reference_bytes` tests above
/// already fail on a fixture whose `.encoded` sibling is missing, so a
/// literal here would only add a second place to edit every time the
/// corpus grows. What it still pins is the reverse — a stray `.encoded`
/// left behind by a renamed fixture, which no other assertion reads.
#[test]
fn parity_bytes_are_well_formed() {
    let expected = cases_in(&corpus_root().join("rfc5545"), "ics").len()
        + cases_in(&corpus_root().join("rfc6350"), "vcf").len();

    let mut count = 0;
    for sub in ["rfc5545", "rfc6350"] {
        let dir = parity_root().join(sub);
        for entry in std::fs::read_dir(&dir).expect("read parity dir") {
            let path = entry.expect("entry").path();
            if path.extension().and_then(|s| s.to_str()) != Some("encoded") {
                continue;
            }
            let bytes = std::fs::read(&path).expect("read parity file");
            assert!(
                bytes.ends_with(b"\r\n"),
                "{}: not CRLF terminated",
                path.display()
            );
            for line in bytes.strip_suffix(b"\r\n").unwrap().split(|b| *b == b'\n') {
                let line = line.strip_suffix(b"\r").unwrap_or(line);
                assert!(
                    line.len() <= 75,
                    "{}: physical line of {} octets",
                    path.display(),
                    line.len()
                );
            }
            count += 1;
        }
    }
    assert_eq!(
        count, expected,
        "parity file count does not match the corpus: a stray .encoded from a \
         renamed or deleted fixture, or a missing one"
    );
}
