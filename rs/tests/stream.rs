// SPDX-License-Identifier: MIT

//! The `codec::stream` gate: streaming re-parse of every corpus file
//! must equal the batch parse, and the encoder lifecycle sentinels must
//! be reachable.
//!
//! # A deliberate divergence from the Go reference
//!
//! The Go stream parsers route content lines through
//! `rfc5545.ParseContentLine` and never apply TEXT unescaping, so the
//! reference's batch and streamed parses of `rfc6350/escaping.vcf`
//! disagree (`Last, Comma Test` batch vs `Last\, Comma Test` streamed).
//! That is a known Go bug, already filed, and this port does **not**
//! replicate it: each stream parser routes through its own format's
//! unescaping, so [`streamed_parse_equals_batch_parse_for_vcards`]
//! holds on every corpus file including that one.

mod support;

use hop_top_vstar::codec::stream::{VCalendarEncoder, VCalendarParser, VCardEncoder, VCardParser};
use hop_top_vstar::codec::{rfc5545, rfc6350};
use hop_top_vstar::{Calendar, Card, CompType, Component, Property};
use std::io::Cursor;

/// Collects every component a [`VCalendarParser`] yields, failing the
/// test on the first error.
fn stream_components(input: &[u8]) -> (Calendar, Vec<Component>) {
    let mut parser = VCalendarParser::new(Cursor::new(input.to_vec()));
    let header = parser.header().clone();
    let out = parser
        .collect::<Result<Vec<_>, _>>()
        .expect("stream parse yields components");
    (header, out)
}

/// Collects every card a [`VCardParser`] yields.
fn stream_cards(input: &[u8]) -> Vec<Card> {
    VCardParser::new(Cursor::new(input.to_vec()))
        .collect::<Result<Vec<_>, _>>()
        .expect("stream parse yields cards")
}

// ---------------------------------------------------------------- //
// The corpus gate                                                   //
// ---------------------------------------------------------------- //

/// Every `rfc5545/*.ics` in the conformance corpus: the streamed
/// component sequence equals the batch parse's, and the streamed header
/// recovers the same PRODID.
#[test]
fn streamed_parse_equals_batch_parse_for_calendars() {
    let dir = support::corpus_root().join("rfc5545");
    let cases = support::cases_in(&dir, "ics");

    for case in &cases {
        let batch = rfc5545::parse(case.input.as_slice())
            .unwrap_or_else(|e| panic!("{}: batch parse: {e}", case.stem));
        let (header, streamed) = stream_components(&case.input);

        assert_eq!(
            streamed, batch.components,
            "{}: streamed components diverge from the batch parse",
            case.stem
        );
        assert_eq!(
            header.prod_id, batch.prod_id,
            "{}: streamed header PRODID",
            case.stem
        );
        assert!(
            header.components.is_empty(),
            "{}: the header carries properties only, never components",
            case.stem
        );
    }
}

/// Every `rfc6350/*.vcf`. This is where the Go reference's missing
/// unescaping shows: `escaping.vcf` batch-parses to `Last, Comma Test`
/// and the reference's stream parser yields `Last\, Comma Test`. This
/// port routes the stream through RFC 6350 unescaping, so the two agree.
#[test]
fn streamed_parse_equals_batch_parse_for_vcards() {
    let dir = support::corpus_root().join("rfc6350");
    let cases = support::cases_in(&dir, "vcf");

    for case in &cases {
        let batch = rfc6350::parse(case.input.as_slice())
            .unwrap_or_else(|e| panic!("{}: batch parse: {e}", case.stem));
        let streamed = stream_cards(&case.input);
        assert_eq!(
            streamed, batch,
            "{}: streamed cards diverge from the batch parse",
            case.stem
        );
    }
}

/// The named case, pinned on its own so a regression names itself
/// rather than hiding in a corpus sweep.
#[test]
fn the_vcard_stream_unescapes_text() {
    let path = support::corpus_root().join("rfc6350").join("escaping.vcf");
    let input = std::fs::read(&path).expect("read escaping.vcf");

    let batch = rfc6350::parse(input.as_slice()).expect("batch parse");
    let streamed = stream_cards(&input);
    assert_eq!(streamed, batch);

    let has_raw_escape = streamed
        .iter()
        .flat_map(|c| c.props.iter())
        .any(|p| p.value.contains("\\,") || p.value.contains("\\;"));
    assert!(
        !has_raw_escape,
        "the stream parser left a raw TEXT escape in a value — \
         it must unescape per RFC 6350 §3.4, not replicate the Go bug"
    );
}

/// The VCALENDAR stream unescapes too. A TEXT value carrying an escaped
/// comma must reach the caller decoded.
#[test]
fn the_vcalendar_stream_unescapes_text() {
    let src = "BEGIN:VCALENDAR\r\n\
               VERSION:2.0\r\n\
               PRODID:-//Example//EN\r\n\
               BEGIN:VTODO\r\n\
               UID:todo-esc\r\n\
               SUMMARY:Last\\, First\\; and more\r\n\
               END:VTODO\r\n\
               END:VCALENDAR\r\n";

    let batch = rfc5545::parse(src.as_bytes()).expect("batch parse");
    let (_, streamed) = stream_components(src.as_bytes());
    assert_eq!(streamed, batch.components);
    assert_eq!(
        streamed[0].get("SUMMARY").map(|p| p.value.as_str()),
        Some("Last, First; and more")
    );
}

/// A fold may split a multi-byte UTF-8 sequence (spec rule 3), so
/// unfolding happens on BYTES before decoding. The corpus carries
/// `fold_split_utf8.ics` precisely for this.
#[test]
fn unfolding_happens_on_bytes_before_decoding() {
    let path = support::corpus_root()
        .join("rfc5545")
        .join("fold_split_utf8.ics");
    let input = std::fs::read(&path).expect("read fold_split_utf8.ics");

    let batch = rfc5545::parse(input.as_slice()).expect("batch parse");
    let (_, streamed) = stream_components(&input);
    assert_eq!(
        streamed, batch.components,
        "a fold splitting a UTF-8 sequence must rejoin before decoding"
    );
}

// ---------------------------------------------------------------- //
// Incrementality                                                    //
// ---------------------------------------------------------------- //

/// The parser is genuinely incremental: the first iteration consumes
/// only as far as the first component's `END`, leaving the rest of the
/// reader untouched.
///
/// A reader that counts the bytes it hands out is the only honest way
/// to assert this — a parser that slurped its whole input and replayed
/// it from memory would satisfy every other test in this file.
///
/// The input is deliberately far larger than any plausible internal
/// buffer (roughly 2 MB across 200 components), so a parser that reads
/// ahead by a fixed buffer still shows a byte count well under the
/// total, while one that slurps shows the whole thing. Measuring
/// against a small input cannot tell the two apart: an 8 KB
/// `BufReader` refill consumes a 2 KB document in a single `read`.
#[test]
fn the_vcalendar_parser_is_genuinely_incremental() {
    use std::cell::Cell;
    use std::io::Read;
    use std::rc::Rc;

    /// A reader recording how many bytes have been pulled out of it.
    struct Counting {
        inner: Cursor<Vec<u8>>,
        read: Rc<Cell<usize>>,
    }

    impl Read for Counting {
        fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
            let n = self.inner.read(buf)?;
            self.read.set(self.read.get() + n);
            Ok(n)
        }
    }

    const COMPONENTS: usize = 200;
    let mut src = String::from("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Inc//EN\r\n");
    for i in 0..COMPONENTS {
        src.push_str(&format!(
            "BEGIN:VTODO\r\nUID:todo-{i}\r\nDTSTAMP:20260504T120000Z\r\n\
             SUMMARY:{}\r\nEND:VTODO\r\n",
            "x".repeat(10_000)
        ));
    }
    src.push_str("END:VCALENDAR\r\n");
    let total = src.len();

    let read = Rc::new(Cell::new(0usize));
    let mut parser = VCalendarParser::new(Counting {
        inner: Cursor::new(src.into_bytes()),
        read: Rc::clone(&read),
    });

    let first = parser.next().expect("a first component").expect("parses");
    assert_eq!(first.uid(), "todo-0");
    let after_first = read.get();
    assert!(
        after_first < total / 10,
        "after one of {COMPONENTS} components the parser had consumed {after_first} \
         of {total} bytes — it is buffering the whole input rather than streaming"
    );

    let rest: Vec<Component> = parser.collect::<Result<_, _>>().expect("the rest parses");
    assert_eq!(rest.len(), COMPONENTS - 1);
    assert!(
        read.get() > after_first,
        "draining the rest must pull more bytes ({after_first} then {})",
        read.get()
    );
}

// ---------------------------------------------------------------- //
// Parser exhaustion                                                 //
// ---------------------------------------------------------------- //

/// Exhaustion is `None`, never an error, and it stays `None` on a
/// repeat call.
#[test]
fn exhaustion_is_none_and_is_idempotent() {
    let src = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//E//EN\r\n\
               BEGIN:VTODO\r\nUID:t\r\nEND:VTODO\r\nEND:VCALENDAR\r\n";
    let mut p = VCalendarParser::new(Cursor::new(src.as_bytes().to_vec()));
    assert!(p.next().is_some());
    assert!(p.next().is_none(), "END:VCALENDAR exhausts the parser");
    assert!(p.next().is_none(), "and it stays exhausted");

    let vcf = "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u\r\nFN:N\r\nEND:VCARD\r\n";
    let mut cp = VCardParser::new(Cursor::new(vcf.as_bytes().to_vec()));
    assert!(cp.next().is_some());
    assert!(cp.next().is_none());
    assert!(cp.next().is_none());
}

/// A structurally broken stream surfaces `Some(Err(_))` with the right
/// sentinel — a real failure is never confused with exhaustion.
#[test]
fn a_real_failure_is_some_err_not_none() {
    let unclosed = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//E//EN\r\n\
                    BEGIN:VTODO\r\nUID:t\r\n";
    let mut p = VCalendarParser::new(Cursor::new(unclosed.as_bytes().to_vec()));
    let err = p
        .next()
        .expect("a failure is Some, not None")
        .expect_err("and it is an Err");
    assert_eq!(err.sentinel(), "ErrUnclosedBlock");

    let bad_version = "BEGIN:VCALENDAR\r\nVERSION:1.0\r\nPRODID:-//E//EN\r\n\
                       BEGIN:VTODO\r\nUID:t\r\nEND:VTODO\r\nEND:VCALENDAR\r\n";
    let mut p = VCalendarParser::new(Cursor::new(bad_version.as_bytes().to_vec()));
    assert_eq!(
        p.next().expect("Some").expect_err("Err").sentinel(),
        "ErrUnsupportedVersion"
    );

    let not_a_calendar = "BEGIN:VEVENT\r\nUID:t\r\nEND:VEVENT\r\n";
    let mut p = VCalendarParser::new(Cursor::new(not_a_calendar.as_bytes().to_vec()));
    assert_eq!(
        p.next().expect("Some").expect_err("Err").sentinel(),
        "ErrMalformed"
    );

    let card_bad_version = "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:u\r\nEND:VCARD\r\n";
    let mut cp = VCardParser::new(Cursor::new(card_bad_version.as_bytes().to_vec()));
    assert_eq!(
        cp.next().expect("Some").expect_err("Err").sentinel(),
        "ErrUnsupportedVersion"
    );
}

// ---------------------------------------------------------------- //
// Encoder lifecycle                                                 //
// ---------------------------------------------------------------- //

/// A round trip through the streaming encoder reproduces the batch
/// encoder's bytes.
#[test]
fn streamed_encode_matches_the_batch_encoder() {
    let dir = support::corpus_root().join("rfc5545");
    for case in support::cases_in(&dir, "ics") {
        let cal =
            rfc5545::parse(case.input.as_slice()).unwrap_or_else(|e| panic!("{}: {e}", case.stem));

        let mut batch = Vec::new();
        rfc5545::encode(&mut batch, &cal).expect("batch encode");

        let mut streamed = Vec::new();
        let mut enc = VCalendarEncoder::new(&mut streamed);
        enc.set_header(&cal).expect("set_header before any encode");
        for c in &cal.components {
            enc.encode(c).expect("encode");
        }
        enc.close().expect("close");

        support::assert_bytes_eq(&streamed, &batch, &case.stem);
    }
}

/// The vCard encoder writes one self-contained block per call, and the
/// concatenation matches the batch encoder.
#[test]
fn streamed_card_encode_matches_the_batch_encoder() {
    let dir = support::corpus_root().join("rfc6350");
    for case in support::cases_in(&dir, "vcf") {
        let cards =
            rfc6350::parse(case.input.as_slice()).unwrap_or_else(|e| panic!("{}: {e}", case.stem));

        let mut batch = Vec::new();
        for c in &cards {
            rfc6350::encode(&mut batch, c).expect("batch encode");
        }

        let mut streamed = Vec::new();
        let mut enc = VCardEncoder::new(&mut streamed);
        for c in &cards {
            enc.encode(c).expect("encode");
        }
        enc.close().expect("close");

        support::assert_bytes_eq(&streamed, &batch, &case.stem);
    }
}

/// `Close` twice, and `Encode` after `Close`, both yield
/// `ErrAlreadyClosed` — never a silently ignored no-op.
#[test]
fn a_closed_encoder_refuses_further_work() {
    let mut out = Vec::new();
    let mut enc = VCalendarEncoder::new(&mut out);
    enc.close().expect("first close");
    assert_eq!(
        enc.close().unwrap_err().sentinel(),
        "ErrAlreadyClosed",
        "a double close must be reachable, not silently ignored"
    );

    let c = Component::new(CompType::TODO);
    assert_eq!(enc.encode(&c).unwrap_err().sentinel(), "ErrAlreadyClosed");

    let mut card_out = Vec::new();
    let mut cenc = VCardEncoder::new(&mut card_out);
    cenc.close().expect("first close");
    assert_eq!(cenc.close().unwrap_err().sentinel(), "ErrAlreadyClosed");
    let card = Card {
        uid: "u".into(),
        kind: None,
        props: Vec::new(),
    };
    assert_eq!(
        cenc.encode(&card).unwrap_err().sentinel(),
        "ErrAlreadyClosed"
    );
}

/// `set_header` after the first `encode` yields `ErrHeaderLocked` — the
/// header is on the wire and cannot be retroactively changed.
#[test]
fn set_header_after_the_first_encode_is_locked() {
    let mut out = Vec::new();
    let mut enc = VCalendarEncoder::new(&mut out);

    let cal = Calendar {
        prod_id: "-//First//EN".into(),
        components: Vec::new(),
    };
    enc.set_header(&cal).expect("before the first encode");

    let mut c = Component::new(CompType::TODO);
    c.set(Property::new("UID", "t"));
    enc.encode(&c).expect("encode locks the header");

    let later = Calendar {
        prod_id: "-//Second//EN".into(),
        components: Vec::new(),
    };
    assert_eq!(
        enc.set_header(&later).unwrap_err().sentinel(),
        "ErrHeaderLocked"
    );

    enc.close().expect("close");
    let text = String::from_utf8(out).expect("utf-8");
    assert!(
        text.contains("PRODID:-//First//EN"),
        "the header on the wire is the one that was set before the lock"
    );
    assert!(!text.contains("Second"));
}

/// `VCardEncoder` has **no** `set_header`, matching the reference.
///
/// A VCARD stream has no enclosing wrapper — each `encode` writes a
/// complete, self-contained block — so there is nothing for such a
/// method to do, and its existence would invite callers to emit a
/// wrapper the parsers reject.
///
/// Absence cannot be asserted at runtime: an unused method changes no
/// observable behaviour, so every other test in this file would pass
/// with one bolted on. This asserts it through method resolution
/// instead.
///
/// The extension trait below supplies a `set_header` with **exactly the
/// signature a mistaken port would give the real one** —
/// `(&mut self, &Calendar) -> Result<(), Error>` — and returns a
/// distinctive sentinel error. Rust prefers an inherent method over a
/// trait one, so the call resolves to the trait's, and the sentinel
/// comes back, only while `VCardEncoder` has no inherent `set_header`.
/// Add one and resolution flips to it, its `Ok(())` reaches the
/// assertion, and this fails.
///
/// Matching the signature is the load-bearing part: a mismatched
/// fallback (a zero-argument `-> bool`, say) is never shadowed, and the
/// test then passes no matter what the encoder grows.
#[test]
fn the_vcard_encoder_has_no_set_header() {
    use hop_top_vstar::Error;

    /// The marker the fallback returns. Nothing else in the crate
    /// produces this exact context string.
    const FALLBACK: &str = "no inherent set_header";

    trait NoSetHeader {
        fn set_header(&mut self, _h: &Calendar) -> Result<(), Error> {
            Err(Error::HeaderLocked(FALLBACK.into()))
        }
    }
    impl<W: std::io::Write> NoSetHeader for VCardEncoder<W> {}

    let mut sink = Vec::new();
    let mut enc = VCardEncoder::new(&mut sink);
    let resolved = enc.set_header(&Calendar::default());
    assert_eq!(
        resolved.unwrap_err().context(),
        FALLBACK,
        "the call resolved to an inherent VCardEncoder::set_header — \
         the encoder must not have one"
    );

    // The control. `VCalendarEncoder` *does* have an inherent
    // `set_header` of that signature, so bringing the same trait into
    // scope for it must NOT change what the call does: it still
    // succeeds. Without this half, the assertion above could be passing
    // because the shadowing mechanism is inert rather than because the
    // method is absent.
    //
    // The `expect(dead_code)` is itself part of the proof. This trait's
    // method is unreachable precisely *because* the inherent method
    // shadows it — that is the mechanism under test. Were
    // `VCalendarEncoder::set_header` ever removed, the trait method
    // would become live, the `expect` would go unfulfilled, and
    // `-D warnings` would fail the build.
    #[expect(
        dead_code,
        reason = "shadowed by VCalendarEncoder's inherent set_header — that is the assertion"
    )]
    trait AlsoNoSetHeader {
        fn set_header(&mut self, _h: &Calendar) -> Result<(), Error> {
            Err(Error::HeaderLocked(FALLBACK.into()))
        }
    }
    impl<W: std::io::Write> AlsoNoSetHeader for VCalendarEncoder<W> {}

    let mut cal_sink = Vec::new();
    let mut cal_enc = VCalendarEncoder::new(&mut cal_sink);
    assert!(
        cal_enc.set_header(&Calendar::default()).is_ok(),
        "VCalendarEncoder's inherent set_header must shadow the trait \
         fallback — if it did not, the check above would prove nothing"
    );
}

/// Closing an encoder that never saw `encode` still emits a legal empty
/// calendar — the header is written first.
#[test]
fn closing_an_unused_encoder_emits_an_empty_calendar() {
    let mut out = Vec::new();
    let mut enc = VCalendarEncoder::new(&mut out);
    enc.close().expect("close");
    let text = String::from_utf8(out).expect("utf-8");
    assert!(text.starts_with("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:"));
    assert!(text.ends_with("END:VCALENDAR\r\n"));

    // The vCard encoder has no wrapper, so closing unused writes nothing.
    let mut card_out = Vec::new();
    VCardEncoder::new(&mut card_out).close().expect("close");
    assert!(card_out.is_empty());
}

/// Encoder output round-trips back through the stream parser.
#[test]
fn stream_encode_then_stream_parse_round_trips() {
    let mut a = Component::new(CompType::TODO);
    a.set(Property::new("UID", "todo-rt-1"));
    a.set(Property::new("DTSTAMP", "20260504T120000Z"));
    a.set(Property::new("SUMMARY", "Last, First"));

    let mut b = Component::new(CompType::EVENT);
    b.set(Property::new("UID", "evt-rt-2"));
    b.set(Property::new("DTSTAMP", "20260504T120000Z"));

    let mut out = Vec::new();
    let mut enc = VCalendarEncoder::new(&mut out);
    enc.set_header(&Calendar {
        prod_id: "-//RT//EN".into(),
        components: Vec::new(),
    })
    .expect("set_header");
    enc.encode(&a).expect("encode a");
    enc.encode(&b).expect("encode b");
    enc.close().expect("close");

    let (header, back) = stream_components(&out);
    assert_eq!(header.prod_id, "-//RT//EN");
    assert_eq!(back, vec![a, b]);
}
