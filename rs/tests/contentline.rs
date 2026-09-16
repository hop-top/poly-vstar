// SPDX-License-Identifier: MIT

//! Content-line scanning, parsing and assembly.
//!
//! RFC 5545 §3.1 unfolding, §3.2 parameter syntax including quoted
//! values, and §3.3.11 TEXT escaping. RFC 6350 §3.2 defers to §3.1 for
//! folding, so the scanner is shared.

use hop_top_vstar::codec::rfc5545::{self, Scanner};

fn lines(src: &str) -> Vec<String> {
    let mut s = Scanner::new(src.as_bytes());
    let mut out = Vec::new();
    while let Some(line) = s.next_line().expect("scan") {
        out.push(line);
    }
    out
}

#[test]
fn scanner_unfolds_space_and_tab_continuations() {
    assert_eq!(
        lines("SUMMARY:one\r\n two\r\n"),
        vec!["SUMMARY:onetwo".to_string()]
    );
    assert_eq!(
        lines("SUMMARY:one\r\n\ttwo\r\n"),
        vec!["SUMMARY:onetwo".to_string()],
        "HTAB is a fold lead octet too"
    );
    // The fold's lead WSP is consumed, not kept.
    assert_eq!(
        lines("DESCRIPTION:a\r\n  b\r\n"),
        vec!["DESCRIPTION:a b".to_string()],
        "only the first WSP belongs to the fold"
    );
}

#[test]
fn scanner_accepts_lf_crlf_and_a_mixture() {
    assert_eq!(lines("A:1\nB:2\n"), vec!["A:1", "B:2"]);
    assert_eq!(lines("A:1\r\nB:2\r\n"), vec!["A:1", "B:2"]);
    assert_eq!(lines("A:1\nB:2\r\nC:3\n"), vec!["A:1", "B:2", "C:3"]);
}

#[test]
fn scanner_surfaces_a_terminator_less_final_line() {
    assert_eq!(lines("A:1\nB:2"), vec!["A:1", "B:2"]);
    assert_eq!(lines("A:1"), vec!["A:1"]);
    assert_eq!(lines(""), Vec::<String>::new());
}

#[test]
fn scanner_skips_blank_lines_and_recovers_from_a_broken_fold() {
    assert_eq!(lines("A:1\n\n\nB:2\n"), vec!["A:1", "B:2"]);
    // A WSP-prefixed line with nothing pending starts a fresh logical
    // line with its lead WSP stripped — no stray leading space.
    assert_eq!(lines("A:1\n\n more\n"), vec!["A:1", "more"]);
}

#[test]
fn parse_content_line_splits_name_params_and_value() {
    let p = rfc5545::parse_content_line("SUMMARY:Hello").expect("parse");
    assert_eq!(p.name, "SUMMARY");
    assert!(p.params.is_empty());
    assert_eq!(p.value, "Hello");

    let p = rfc5545::parse_content_line("DTSTART;VALUE=DATE:20260515").expect("parse");
    assert_eq!(p.name, "DTSTART");
    assert_eq!(p.params.len(), 1);
    assert_eq!(p.params[0].name, "VALUE");
    assert_eq!(p.params[0].value, "DATE");
    assert_eq!(p.value, "20260515");

    // The wire case of names is preserved verbatim; matching folds later.
    let p = rfc5545::parse_content_line("x-lower;Mixed=v:val").expect("parse");
    assert_eq!(p.name, "x-lower");
    assert_eq!(p.params[0].name, "Mixed");

    // A value may itself contain colons.
    let p = rfc5545::parse_content_line("UID:urn:uuid:1234").expect("parse");
    assert_eq!(p.value, "urn:uuid:1234");

    // An empty value is legal.
    let p = rfc5545::parse_content_line("X-EMPTY:").expect("parse");
    assert_eq!(p.value, "");
}

#[test]
fn parse_content_line_honors_quoted_parameter_values() {
    // A quoted value may hold ',' ';' and ':' — none of them terminate.
    let p = rfc5545::parse_content_line("ATTENDEE;CN=\"Doe, Jane; Dr:\":mailto:j@example.com")
        .expect("parse");
    assert_eq!(p.name, "ATTENDEE");
    assert_eq!(p.params.len(), 1);
    assert_eq!(p.params[0].name, "CN");
    assert_eq!(
        p.params[0].value, "Doe, Jane; Dr:",
        "the surrounding DQUOTEs are stripped, the content is kept"
    );
    assert_eq!(p.value, "mailto:j@example.com");

    // Multiple parameters, one quoted.
    let p = rfc5545::parse_content_line("X;A=plain;B=\"q;q\":v").expect("parse");
    assert_eq!(p.params.len(), 2);
    assert_eq!(p.params[0].value, "plain");
    assert_eq!(p.params[1].value, "q;q");
}

#[test]
fn parse_content_line_rejects_structural_defects_as_malformed() {
    for bad in [
        "no-colon-at-all",
        ":empty-name",
        ";leading-semicolon:v",
        "NAME;noequals:v",
        "NAME;=novalue:v",
        "NAME;A=\"unbalanced:v",
    ] {
        let e = rfc5545::parse_content_line(bad).expect_err("should be rejected");
        assert_eq!(e.sentinel(), "ErrMalformed", "{bad:?}");
    }
}

#[test]
fn text_values_unescape_on_parse_and_re_escape_on_encode() {
    let src = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\nBEGIN:VTODO\nUID:u\nSUMMARY:a\\, b\\; c\\nd\\\\e\nEND:VTODO\nEND:VCALENDAR\n";
    let cal = rfc5545::parse(src.as_bytes()).expect("parse");
    let summary = &cal.components[0].get("SUMMARY").expect("SUMMARY").value;
    assert_eq!(
        summary, "a, b; c\nd\\e",
        "the in-memory model holds raw, unescaped values"
    );

    let mut out = Vec::new();
    rfc5545::encode(&mut out, &cal).expect("encode");
    let text = String::from_utf8(out).expect("utf8");
    assert!(
        text.contains("SUMMARY:a\\, b\\; c\\nd\\\\e\r\n"),
        "the encoder re-applies escaping symmetrically, got {text:?}"
    );
}

/// Escaping is applied only to TEXT-typed properties. Escaping a comma
/// in a URI would corrupt the address.
#[test]
fn non_text_properties_pass_through_verbatim() {
    let src = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\nBEGIN:VTODO\nUID:u\nATTACH:https://x.test/a,b;c\nEND:VTODO\nEND:VCALENDAR\n";
    let cal = rfc5545::parse(src.as_bytes()).expect("parse");
    assert_eq!(
        cal.components[0].get("ATTACH").unwrap().value,
        "https://x.test/a,b;c",
        "a non-TEXT value is not unescaped on parse"
    );

    let mut out = Vec::new();
    rfc5545::encode(&mut out, &cal).expect("encode");
    let text = String::from_utf8(out).expect("utf8");
    assert!(
        text.contains("ATTACH:https://x.test/a,b;c\r\n"),
        "a non-TEXT value is not escaped on encode, got {text:?}"
    );
}

/// Parameter values containing a separator are DQUOTE-wrapped on encode
/// and survive the round-trip.
#[test]
fn parameter_values_are_quoted_when_they_need_to_be() {
    let src = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\nBEGIN:VTODO\nUID:u\nATTENDEE;CN=\"Doe, Jane\":mailto:j@x.test\nEND:VTODO\nEND:VCALENDAR\n";
    let cal = rfc5545::parse(src.as_bytes()).expect("parse");
    assert_eq!(
        cal.components[0].get("ATTENDEE").unwrap().params[0].value,
        "Doe, Jane"
    );

    let mut out = Vec::new();
    rfc5545::encode(&mut out, &cal).expect("encode");
    let text = String::from_utf8(out).expect("utf8");
    assert!(text.contains("ATTENDEE;CN=\"Doe, Jane\":"), "got {text:?}");

    let back = rfc5545::parse(text.as_bytes()).expect("re-parse");
    assert_eq!(cal, back);
}

/// vCard group prefixes keep their case; the bare name uppercases.
#[test]
fn vcard_group_prefixes_survive_the_round_trip() {
    use hop_top_vstar::codec::rfc6350;
    let src = "BEGIN:VCARD\nVERSION:4.0\nUID:u\nhome.tel;TYPE=voice:tel:+15555550100\nEND:VCARD\n";
    let cards = rfc6350::parse(src.as_bytes()).expect("parse");
    assert_eq!(
        cards[0].props[0].name, "home.tel",
        "the wire name is preserved verbatim"
    );

    let mut out = Vec::new();
    rfc6350::encode(&mut out, &cards[0]).expect("encode");
    let text = String::from_utf8(out).expect("utf8");
    assert!(
        text.contains("home.TEL;TYPE=voice:"),
        "the group keeps its case, the bare name uppercases; got {text:?}"
    );
}

/// The encoder uppercases property and parameter names per RFC 5545 §3.1.
#[test]
fn encoder_uppercases_names_but_not_values() {
    let src = "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:p\nBEGIN:vtodo\nuid:MixedCaseValue\nx-p;lower=KeepMe:v\nEND:vtodo\nEND:VCALENDAR\n";
    let cal = rfc5545::parse(src.as_bytes()).expect("parse");
    let mut out = Vec::new();
    rfc5545::encode(&mut out, &cal).expect("encode");
    let text = String::from_utf8(out).expect("utf8");
    assert!(text.contains("BEGIN:VTODO\r\n"), "got {text:?}");
    assert!(text.contains("UID:MixedCaseValue\r\n"), "got {text:?}");
    assert!(text.contains("X-P;LOWER=KeepMe:v\r\n"), "got {text:?}");
}
