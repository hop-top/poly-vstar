// SPDX-License-Identifier: MIT

//! RFC 5545 §3.3.5 DATE-TIME: form #2 parsing and formatting, and
//! form #1 resolution against a VTIMEZONE carried by the document
//! itself.
//!
//! The gate is `spec/behavior/time/tzid.json`, whose `"utc": null` rows
//! spell the reference's `(value, ok)` pair reporting not-ok. Those
//! rows are the load-bearing half: they name a real IANA zone, which is
//! the trap — a port resolving it from the system database passes the
//! resolvable rows and fails every zone not in the corpus.
//!
//! There is no IANA timezone database here and there must never be one.

mod support;

use chrono::{TimeZone, Utc};
use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::{format_time, parse_time, parse_time_with_tzid, Calendar};
use std::collections::BTreeMap;
use std::fs;
use support::{behavior_input, behavior_json, corpus_root};

/// Loads the `time/` conformance calendars by stem, so a `tzid.json`
/// row's `"calendar"` field resolves to a parsed document.
fn time_calendars() -> BTreeMap<String, Calendar> {
    let dir = corpus_root().join("time");
    let mut out = BTreeMap::new();
    for entry in fs::read_dir(&dir).expect("the time corpus directory exists") {
        let path = entry.expect("read dir entry").path();
        if path.extension().and_then(|s| s.to_str()) != Some("ics") {
            continue;
        }
        let stem = path
            .file_stem()
            .and_then(|s| s.to_str())
            .expect("stem is UTF-8")
            .to_owned();
        let bytes = fs::read(&path).expect("read fixture");
        let cal = rfc5545::parse(bytes.as_slice())
            .unwrap_or_else(|e| panic!("{}: parse: {e}", path.display()));
        out.insert(stem, cal);
    }
    assert!(!out.is_empty(), "the time corpus is empty");
    out
}

#[test]
fn tzid_resolution_matches_the_behavior_corpus() {
    let cals = time_calendars();
    let rows = behavior_json("time", "tzid.json");
    let cases = rows.array();
    assert!(
        cases.len() >= 17,
        "expected the full tzid table, saw {}",
        cases.len()
    );

    for (i, case) in cases.iter().enumerate() {
        let cal_name = case
            .str_field("calendar")
            .expect("every row names a calendar");
        let cal = cals
            .get(cal_name)
            .unwrap_or_else(|| panic!("row {i}: no time fixture named {cal_name}"));
        // The TZID and the value are present-but-possibly-empty, not
        // nullable: an empty TZID is one of the not-ok rows.
        let tzid = case.str_field("tzid").unwrap_or("");
        let value = case.str_field("value").unwrap_or("");
        let got = parse_time_with_tzid(value, tzid, cal);

        match case.str_field("utc") {
            Some(want) => {
                let at = got.unwrap_or_else(|| {
                    panic!("row {i}: {cal_name} {tzid:?} {value:?} did not resolve; want {want}")
                });
                assert_eq!(
                    format_time(at),
                    want,
                    "row {i}: {cal_name} {tzid:?} {value:?}"
                );
            }
            None => assert!(
                got.is_none(),
                "row {i}: {cal_name} {tzid:?} {value:?} resolved to {:?}, but the reference \
                 reports not-ok — a port reaching for a system timezone database fails here",
                got.map(format_time)
            ),
        }
    }
}

#[test]
fn parse_time_accepts_only_form_two() {
    let at = parse_time("20260515T120000Z").expect("form #2 parses");
    assert_eq!(at, Utc.with_ymd_and_hms(2026, 5, 15, 12, 0, 0).unwrap());

    for bad in [
        "20260515T120000",      // form #1, no zone
        "20260515T120000z",     // lowercase designator
        "2026-05-15T12:00:00Z", // RFC 3339 extended
        "20260515",             // date only
        " 20260515T120000Z",    // leading whitespace
        "20260515T120000Z ",    // trailing whitespace
        "20260230T120000Z",     // February 30th
        "20260515T250000Z",     // hour 25
        "20260515T126000Z",     // minute 60
        "",
    ] {
        assert!(parse_time(bad).is_none(), "{bad:?} should not parse");
    }
}

#[test]
fn format_time_round_trips_form_two() {
    for s in [
        "20260515T120000Z",
        "19700101T000000Z",
        "20261231T235959Z",
        "00010101T000000Z",
    ] {
        let at = parse_time(s).unwrap_or_else(|| panic!("{s} parses"));
        assert_eq!(format_time(at), s, "{s} did not round-trip");
    }
}

#[test]
fn format_time_pads_every_field_to_its_wire_width() {
    let at = Utc.with_ymd_and_hms(2026, 1, 2, 3, 4, 5).unwrap();
    assert_eq!(format_time(at), "20260102T030405Z");
}

#[test]
fn parse_time_with_tzid_rejects_a_form_two_value() {
    // The value is already absolute; resolving it against a zone would
    // apply an offset twice. Rule 5 falls back to verbatim emit.
    let cals = time_calendars();
    let cal = cals.get("america_montreal").expect("the fixture exists");
    assert!(parse_time_with_tzid("20260315T060000Z", "America/Montreal", cal).is_none());
}

#[test]
fn parse_time_with_tzid_rejects_an_empty_tzid() {
    let cals = time_calendars();
    let cal = cals.get("america_montreal").expect("the fixture exists");
    assert!(parse_time_with_tzid("20260104T133045", "", cal).is_none());
}

#[test]
fn parse_time_with_tzid_matches_the_tzid_case_sensitively() {
    // TZIDs are opaque identifiers per RFC 5545 §3.2.19.
    let cals = time_calendars();
    let cal = cals.get("america_montreal").expect("the fixture exists");
    assert!(parse_time_with_tzid("20260104T133045", "america/montreal", cal).is_none());
}

#[test]
fn a_vtimezone_outside_the_subset_fails_resolution() {
    // An RRULE with COUNT is outside the spec's VTIMEZONE subset, so resolution
    // fails and rule 5 falls back to verbatim — a failure, never an
    // error, and never a partially applied rule.
    let src = "BEGIN:VCALENDAR\r\n\
               VERSION:2.0\r\n\
               PRODID:-//V*//test//EN\r\n\
               BEGIN:VTIMEZONE\r\n\
               TZID:Test/Rejected\r\n\
               BEGIN:DAYLIGHT\r\n\
               DTSTART:20070311T020000\r\n\
               TZOFFSETFROM:-0500\r\n\
               TZOFFSETTO:-0400\r\n\
               RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=2SU;COUNT=5\r\n\
               END:DAYLIGHT\r\n\
               BEGIN:STANDARD\r\n\
               DTSTART:20071104T020000\r\n\
               TZOFFSETFROM:-0400\r\n\
               TZOFFSETTO:-0500\r\n\
               RRULE:FREQ=YEARLY;BYMONTH=11;BYDAY=1SU\r\n\
               END:STANDARD\r\n\
               END:VTIMEZONE\r\n\
               END:VCALENDAR\r\n";
    let cal = rfc5545::parse(src.as_bytes()).expect("parses");
    assert!(parse_time_with_tzid("20260704T133045", "Test/Rejected", &cal).is_none());
}

#[test]
fn a_vtimezone_missing_an_offset_fails_resolution() {
    let src = "BEGIN:VCALENDAR\r\n\
               VERSION:2.0\r\n\
               PRODID:-//V*//test//EN\r\n\
               BEGIN:VTIMEZONE\r\n\
               TZID:Test/NoOffsetFrom\r\n\
               BEGIN:STANDARD\r\n\
               DTSTART:19700101T000000\r\n\
               TZOFFSETTO:-0500\r\n\
               END:STANDARD\r\n\
               END:VTIMEZONE\r\n\
               END:VCALENDAR\r\n";
    let cal = rfc5545::parse(src.as_bytes()).expect("parses");
    assert!(parse_time_with_tzid("20260704T133045", "Test/NoOffsetFrom", &cal).is_none());
}

#[test]
fn a_fixed_offset_standard_only_zone_resolves() {
    let src = "BEGIN:VCALENDAR\r\n\
               VERSION:2.0\r\n\
               PRODID:-//V*//test//EN\r\n\
               BEGIN:VTIMEZONE\r\n\
               TZID:Test/Fixed\r\n\
               BEGIN:STANDARD\r\n\
               DTSTART:19700101T000000\r\n\
               TZOFFSETFROM:+0530\r\n\
               TZOFFSETTO:+0530\r\n\
               END:STANDARD\r\n\
               END:VTIMEZONE\r\n\
               END:VCALENDAR\r\n";
    let cal = rfc5545::parse(src.as_bytes()).expect("parses");
    let at = parse_time_with_tzid("20260704T120000", "Test/Fixed", &cal).expect("resolves");
    assert_eq!(format_time(at), "20260704T063000Z");
}

#[test]
fn an_hhmmss_offset_is_accepted() {
    let src = "BEGIN:VCALENDAR\r\n\
               VERSION:2.0\r\n\
               PRODID:-//V*//test//EN\r\n\
               BEGIN:VTIMEZONE\r\n\
               TZID:Test/Seconds\r\n\
               BEGIN:STANDARD\r\n\
               DTSTART:19700101T000000\r\n\
               TZOFFSETFROM:-000044\r\n\
               TZOFFSETTO:-000044\r\n\
               END:STANDARD\r\n\
               END:VTIMEZONE\r\n\
               END:VCALENDAR\r\n";
    let cal = rfc5545::parse(src.as_bytes()).expect("parses");
    let at = parse_time_with_tzid("20260704T120000", "Test/Seconds", &cal).expect("resolves");
    assert_eq!(format_time(at), "20260704T120044Z");
}

#[test]
fn the_time_corpus_carries_the_calendars_the_behavior_table_names() {
    // A silently renamed fixture would otherwise turn the gate above
    // into a panic with a confusing message.
    let cals = time_calendars();
    for case in behavior_json("time", "tzid.json").array() {
        let name = case.str_field("calendar").expect("named");
        assert!(cals.contains_key(name), "no time fixture named {name}");
    }
    // And the behavior directory is where we think it is.
    assert!(!behavior_input("time", "tzid.json").is_empty());
}
