// SPDX-License-Identifier: MIT

//! Rule-level assertions for canonicalization rules 1–12.
//!
//! Byte identity against the corpus is necessary but NOT sufficient:
//! disabling NFC entirely once passed every byte-identity fixture the
//! corpus then carried, because none of them held decomposed text. The
//! corpus has since grown fixtures that close that hole, and these
//! tests close it from the other side — each asserts one rule on input
//! built in this file, so a regression names the rule rather than a
//! fixture stem.

mod support;

use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::{canonical, hashing, Calendar, Card, CompType, Component, Param, Property};

/// A VEVENT carrying `props`, wrapped in a calendar with a fixed PRODID
/// so a divergence is attributable to the properties under test.
fn calendar_with(props: Vec<Property>) -> Calendar {
    Calendar {
        prod_id: "-//V*//test//EN".to_owned(),
        components: vec![Component {
            r#type: CompType::from_wire("VEVENT"),
            props,
            sub: Vec::new(),
        }],
    }
}

/// The canonical bytes of `cal`, decoded lossily for readable asserts.
///
/// Only used where the assertion is about ordering or a parameter, never
/// where it is about a fold point — a lossy decode would paper over a
/// split sequence, which is precisely what the fold tests check for.
fn text(cal: &Calendar) -> String {
    String::from_utf8(canonical::calendar(cal)).expect("no split sequence in this fixture")
}

// ---------------------------------------------------------------------
// Rule 1 — CRLF line endings.
// ---------------------------------------------------------------------

#[test]
fn rule_1_terminates_every_physical_line_with_crlf() {
    let bytes = canonical::calendar(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("SUMMARY", "short"),
    ]));
    assert!(bytes.ends_with(b"\r\n"), "canonical form ends with CRLF");
    for (i, b) in bytes.iter().enumerate() {
        if *b == b'\n' {
            assert_eq!(bytes.get(i.wrapping_sub(1)), Some(&b'\r'), "bare LF at {i}");
        }
    }
}

// ---------------------------------------------------------------------
// Rule 2 — property and parameter order.
// ---------------------------------------------------------------------

#[test]
fn rule_2_sorts_properties_alphabetically_by_name() {
    let out = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("SUMMARY", "s"),
        Property::new("DTSTAMP", "20260101T000000Z"),
        Property::new("LOCATION", "l"),
    ]));
    let names: Vec<&str> = out
        .lines()
        .filter_map(|l| l.split([':', ';']).next())
        .filter(|n| ["UID", "SUMMARY", "DTSTAMP", "LOCATION"].contains(n))
        .collect();
    assert_eq!(names, ["DTSTAMP", "LOCATION", "SUMMARY", "UID"]);
}

#[test]
fn rule_2_sorts_parameters_alphabetically_within_a_property() {
    let out = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property {
            name: "ATTENDEE".to_owned(),
            params: vec![
                Param::new("ROLE", "REQ-PARTICIPANT"),
                Param::new("CN", "Ada"),
                Param::new("PARTSTAT", "ACCEPTED"),
            ],
            value: "mailto:ada@example.com".to_owned(),
        },
    ]));
    assert!(
        out.contains("ATTENDEE;CN=Ada;PARTSTAT=ACCEPTED;ROLE=REQ-PARTICIPANT:"),
        "parameters are not alphabetical:\n{out}"
    );
}

#[test]
fn rule_2_property_sort_is_stable_for_repeated_names() {
    let out = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("CATEGORIES", "first"),
        Property::new("CATEGORIES", "second"),
        Property::new("CATEGORIES", "third"),
    ]));
    let order: Vec<&str> = out
        .lines()
        .filter(|l| l.starts_with("CATEGORIES:"))
        .collect();
    assert_eq!(
        order,
        ["CATEGORIES:first", "CATEGORIES:second", "CATEGORIES:third"],
        "equal names lost their relative input order"
    );
}

// ---------------------------------------------------------------------
// Rule 3 — folding at 75 octets, on bytes.
// ---------------------------------------------------------------------

#[test]
fn rule_3_folds_at_seventy_five_octets() {
    let bytes = canonical::calendar(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("DESCRIPTION", "x".repeat(300)),
    ]));
    // Every physical line, terminator excluded, is at most 75 octets.
    for line in bytes.split(|b| *b == b'\n') {
        let line = line.strip_suffix(b"\r").unwrap_or(line);
        assert!(
            line.len() <= 75,
            "physical line of {} octets exceeds the 75-octet limit: {:?}",
            line.len(),
            String::from_utf8_lossy(line)
        );
    }
    // And at least one line uses the full budget, so a port folding at
    // some smaller width is caught rather than passing vacuously.
    assert!(
        bytes
            .split(|b| *b == b'\n')
            .any(|l| l.strip_suffix(b"\r").unwrap_or(l).len() == 75),
        "no line reached 75 octets — the fold width is wrong"
    );
}

#[test]
fn rule_3_splits_a_multibyte_sequence_across_the_fold() {
    // The boundary is measured in octets, so a two-octet `é` straddling
    // it is cut in half. A port that retreats the cut to a character
    // boundary moves every subsequent fold point and produces different
    // canonical bytes and a different hash.
    let summary = format!("{}{}", "a".repeat(56), "é".repeat(8));
    let bytes = canonical::calendar(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("SUMMARY", summary),
    ]));
    assert!(
        String::from_utf8(bytes.clone()).is_err(),
        "the canonical bytes decode cleanly, so no sequence was split — \
         the fold retreated to a character boundary"
    );
    // Specifically: a lead byte immediately before the CRLF fold.
    let split_at = bytes
        .windows(4)
        .position(|w| w[0] == 0xC3 && w[1] == b'\r' && w[2] == b'\n' && w[3] == b' ');
    assert!(
        split_at.is_some(),
        "expected a UTF-8 lead octet immediately before a fold"
    );
}

#[test]
fn rule_3_folds_after_property_assembly_not_before() {
    // Folding a value before appending parameters puts the fold points
    // elsewhere, and the result unfolds to the same logical line — only
    // a byte comparison catches it. The head here is long enough that a
    // value-only fold and a whole-line fold cannot coincide.
    let bytes = canonical::calendar(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property {
            name: "DESCRIPTION".to_owned(),
            params: vec![Param::new("X-LONG-PARAMETER-NAME", "y".repeat(60))],
            value: "z".repeat(120),
        },
    ]));
    let first = bytes
        .split(|b| *b == b'\n')
        .map(|l| l.strip_suffix(b"\r").unwrap_or(l))
        .find(|l| l.starts_with(b"DESCRIPTION"))
        .expect("the DESCRIPTION line is emitted");
    assert_eq!(
        first.len(),
        75,
        "the first physical line of an assembled long property is not a full 75 octets"
    );
    // The fold lands inside the parameter, which can only happen when
    // the whole assembled line was folded.
    assert!(
        !first.contains(&b'z'),
        "the value reached the first physical line, so the parameters were folded separately"
    );
}

// ---------------------------------------------------------------------
// Rule 6 — component order.
// ---------------------------------------------------------------------

/// A VEVENT with the given UID and nothing else that sorts.
fn event(uid: &str) -> Component {
    Component {
        r#type: CompType::from_wire("VEVENT"),
        props: vec![Property::new("UID", uid)],
        sub: Vec::new(),
    }
}

/// A VEVENT carrying both a UID and a `DTSTART` whose order is the
/// REVERSE of the UID's.
///
/// The `day` argument counts down as the UID counts up, so a port
/// sorting on any other datetime-bearing property — `DTSTART` being the
/// obvious wrong choice — produces exactly the opposite order. Without
/// this the rule-6 tests pass under a `DTSTART` sort, because a
/// component carrying only a UID has nothing else to key on.
fn event_with_counter_dtstart(uid: &str, day: u32) -> Component {
    Component {
        r#type: CompType::from_wire("VEVENT"),
        props: vec![
            Property::new("UID", uid),
            Property::new("DTSTART", format!("202601{day:02}T000000Z")),
            Property::new("DTSTAMP", format!("202602{day:02}T000000Z")),
        ],
        sub: Vec::new(),
    }
}

/// The UID values in the order the canonical form emitted them.
fn emitted_uids(cal: &Calendar) -> Vec<String> {
    text(cal)
        .lines()
        .filter_map(|l| l.strip_prefix("UID:"))
        .map(str::to_owned)
        .collect()
}

#[test]
fn rule_6_sorts_top_level_components_by_uid_in_utf8_byte_order() {
    // The four UIDs disagree between UTF-8 byte order and UTF-16 code
    // unit order: the astral character's UTF-8 lead byte is 0xF0, above
    // the fullwidth 'Ａ' (0xEF...), while in UTF-16 its surrogate
    // (0xD83D) sorts BELOW every BMP character from U+E000 up.
    //
    // Each component's DTSTART and DTSTAMP run counter to its UID, so
    // the expected order is reachable only by keying on the UID.
    let cal = Calendar {
        prod_id: "-//V*//test//EN".to_owned(),
        components: vec![
            event_with_counter_dtstart("uid-\u{1D400}-astral", 1),
            event_with_counter_dtstart("uid-\u{FF21}-fullwidth", 2),
            event_with_counter_dtstart("uid-\u{FF}-latin1", 3),
            event_with_counter_dtstart("uid-a-ascii", 4),
        ],
    };
    assert_eq!(
        emitted_uids(&cal),
        [
            "uid-a-ascii",
            "uid-\u{FF}-latin1",
            "uid-\u{FF21}-fullwidth",
            "uid-\u{1D400}-astral",
        ],
        "components are not in UTF-8 byte order"
    );
}

#[test]
fn rule_6_uses_tzid_as_the_sort_key_for_a_vtimezone() {
    let mut tz = Component::new(CompType::from_wire("VTIMEZONE"));
    tz.add(Property::new("TZID", "aaa/Zone"));
    // Counter-running datetimes again, so keying on anything but the
    // UID and the TZID reverses the expected order.
    let cal = Calendar {
        prod_id: "-//V*//test//EN".to_owned(),
        components: vec![
            event_with_counter_dtstart("zzz-event", 1),
            tz,
            event_with_counter_dtstart("bbb-event", 2),
        ],
    };
    let out = text(&cal);
    let tz_at = out.find("TZID:aaa/Zone").expect("the VTIMEZONE is emitted");
    let bbb_at = out.find("UID:bbb-event").expect("bbb is emitted");
    let zzz_at = out.find("UID:zzz-event").expect("zzz is emitted");
    assert!(tz_at < bbb_at, "TZID did not act as a sort key");
    assert!(bbb_at < zzz_at, "UIDs are out of order");
}

#[test]
fn rule_6_sorts_keyless_components_last_in_stable_input_order() {
    // The keyless components carry a DTSTART that would sort them FIRST
    // under any datetime key, so a port keying on one puts them ahead of
    // the keyed component instead of last.
    let keyless = |summary: &str| Component {
        r#type: CompType::from_wire("VEVENT"),
        props: vec![
            Property::new("SUMMARY", summary),
            Property::new("DTSTART", "20260101T000000Z"),
        ],
        sub: Vec::new(),
    };
    let cal = Calendar {
        prod_id: "-//V*//test//EN".to_owned(),
        components: vec![
            keyless("first"),
            event_with_counter_dtstart("zzz", 9),
            keyless("second"),
        ],
    };
    let out = text(&cal);
    let uid_at = out.find("UID:zzz").expect("the keyed component is emitted");
    let first_at = out.find("SUMMARY:first").expect("first is emitted");
    let second_at = out.find("SUMMARY:second").expect("second is emitted");
    assert!(
        uid_at < first_at,
        "a keyless component sorted before a keyed one"
    );
    assert!(
        first_at < second_at,
        "keyless components lost their input order"
    );
}

#[test]
fn rule_6_preserves_sub_component_input_order() {
    let alarm = |uid: &str| Component {
        r#type: CompType::from_wire("VALARM"),
        props: vec![
            Property::new("UID", uid),
            Property::new("ACTION", "DISPLAY"),
            Property::new("TRIGGER", "-PT15M"),
        ],
        sub: Vec::new(),
    };
    let cal = Calendar {
        prod_id: "-//V*//test//EN".to_owned(),
        components: vec![Component {
            r#type: CompType::from_wire("VEVENT"),
            props: vec![Property::new("UID", "evt")],
            sub: vec![alarm("zzz-alarm"), alarm("aaa-alarm")],
        }],
    };
    let out = text(&cal);
    let zzz_at = out.find("UID:zzz-alarm").expect("zzz alarm is emitted");
    let aaa_at = out.find("UID:aaa-alarm").expect("aaa alarm is emitted");
    assert!(
        zzz_at < aaa_at,
        "sub-components were sorted; rule 6 preserves their input order"
    );
}

// ---------------------------------------------------------------------
// Rule 7 — X-VSTAR-HASH exclusion.
// ---------------------------------------------------------------------

#[test]
fn rule_7_strips_the_hash_property_from_the_canonical_bytes() {
    let out = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new(hashing::X_VSTAR_HASH_PROPERTY, "sha256:deadbeef"),
    ]));
    assert!(
        !out.contains(hashing::X_VSTAR_HASH_PROPERTY),
        "the hash property survived into the canonical bytes:\n{out}"
    );
}

#[test]
fn rule_7_makes_a_stored_hash_irrelevant_to_the_computed_one() {
    let bare = calendar_with(vec![Property::new("UID", "u1")]);
    let stamped = calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new(hashing::X_VSTAR_HASH_PROPERTY, "sha256:0000"),
    ]);
    assert_eq!(
        hashing::calendar(&bare),
        hashing::calendar(&stamped),
        "a stored hash fed back into its own digest"
    );
}

#[test]
fn rule_7_strips_the_hash_property_at_every_depth() {
    let mut inner = Component::new(CompType::from_wire("VALARM"));
    inner.add(Property::new("ACTION", "DISPLAY"));
    inner.add(Property::new(hashing::X_VSTAR_HASH_PROPERTY, "sha256:1111"));

    let mut clean = inner.clone();
    clean.remove(hashing::X_VSTAR_HASH_PROPERTY);

    let with = Calendar {
        prod_id: "-//V*//test//EN".to_owned(),
        components: vec![Component {
            r#type: CompType::from_wire("VEVENT"),
            props: vec![Property::new("UID", "u1")],
            sub: vec![inner],
        }],
    };
    let without = Calendar {
        prod_id: "-//V*//test//EN".to_owned(),
        components: vec![Component {
            r#type: CompType::from_wire("VEVENT"),
            props: vec![Property::new("UID", "u1")],
            sub: vec![clean],
        }],
    };
    assert_eq!(hashing::calendar(&with), hashing::calendar(&without));
}

// ---------------------------------------------------------------------
// Rule 8 — RRULE values verbatim.
// ---------------------------------------------------------------------

#[test]
fn rule_8_preserves_rrule_values_verbatim() {
    let spelled = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("RRULE", "FREQ=DAILY;INTERVAL=1"),
    ]));
    assert!(
        spelled.contains("RRULE:FREQ=DAILY;INTERVAL=1"),
        "the RRULE value was normalized:\n{spelled}"
    );
    let elided = calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("RRULE", "FREQ=DAILY"),
    ]);
    assert_ne!(
        hashing::calendar(&elided),
        hashing::calendar(&calendar_with(vec![
            Property::new("UID", "u1"),
            Property::new("RRULE", "FREQ=DAILY;INTERVAL=1"),
        ])),
        "two wire spellings of one rule hashed the same — rule 8 normalizes nothing"
    );
}

// ---------------------------------------------------------------------
// Rule 9 — NFC normalization.
// ---------------------------------------------------------------------

/// `Café` with a combining acute — the decomposed (NFD) spelling.
const DECOMPOSED: &str = "Cafe\u{301}";
/// The same text precomposed (NFC).
const COMPOSED: &str = "Caf\u{e9}";

#[test]
fn rule_9_normalizes_property_values_to_nfc() {
    let out = canonical::calendar(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("SUMMARY", DECOMPOSED),
    ]));
    let want = canonical::calendar(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("SUMMARY", COMPOSED),
    ]));
    assert_eq!(
        out, want,
        "a decomposed value did not canonicalize to its composed form"
    );
    assert!(
        String::from_utf8_lossy(&out).contains(COMPOSED),
        "the composed form is absent from the canonical bytes"
    );
}

#[test]
fn rule_9_normalizes_parameter_values_to_nfc() {
    let decomposed = calendar_with(vec![
        Property::new("UID", "u1"),
        Property {
            name: "LOCATION".to_owned(),
            params: vec![Param::new("X-CITY", "Zu\u{308}rich")],
            value: "here".to_owned(),
        },
    ]);
    let composed = calendar_with(vec![
        Property::new("UID", "u1"),
        Property {
            name: "LOCATION".to_owned(),
            params: vec![Param::new("X-CITY", "Z\u{fc}rich")],
            value: "here".to_owned(),
        },
    ]);
    assert_eq!(
        canonical::calendar(&decomposed),
        canonical::calendar(&composed),
        "a decomposed parameter value was not normalized"
    );
}

#[test]
fn rule_9_normalizes_the_prodid() {
    let a = Calendar {
        prod_id: DECOMPOSED.to_owned(),
        components: vec![event("u1")],
    };
    let b = Calendar {
        prod_id: COMPOSED.to_owned(),
        components: vec![event("u1")],
    };
    assert_eq!(canonical::calendar(&a), canonical::calendar(&b));
}

#[test]
fn rule_9_normalizes_before_folding_so_the_fold_points_are_right() {
    // NFC shortens `e` + U+0301 (three octets) to `é` (two), so folding
    // a pre-normalization string puts the break at the wrong octet.
    let decomposed = "a".repeat(40) + &"e\u{301}".repeat(20);
    let composed = "a".repeat(40) + &"\u{e9}".repeat(20);
    assert_eq!(
        canonical::calendar(&calendar_with(vec![
            Property::new("UID", "u1"),
            Property::new("SUMMARY", decomposed),
        ])),
        canonical::calendar(&calendar_with(vec![
            Property::new("UID", "u1"),
            Property::new("SUMMARY", composed),
        ])),
        "the fold ran before NFC, so the break landed at the wrong octet"
    );
}

#[test]
fn rule_9_leaves_property_names_unnormalized_and_uppercased() {
    let out = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("summary", "s"),
    ]));
    assert!(
        out.contains("SUMMARY:s"),
        "names are not uppercased:\n{out}"
    );
}

// ---------------------------------------------------------------------
// Rule 10 — ATTACH reference-only.
// ---------------------------------------------------------------------

#[test]
fn rule_10_strips_value_binary_and_encoding_base64_from_attach() {
    let out = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property {
            name: "ATTACH".to_owned(),
            params: vec![
                Param::new("VALUE", "BINARY"),
                Param::new("ENCODING", "BASE64"),
            ],
            value: "aGVsbG8=".to_owned(),
        },
    ]));
    assert!(
        out.contains("ATTACH:aGVsbG8="),
        "ATTACH was not reduced:\n{out}"
    );
    assert!(!out.contains("VALUE=BINARY"), "VALUE=BINARY survived");
    assert!(!out.contains("ENCODING=BASE64"), "ENCODING=BASE64 survived");
}

#[test]
fn rule_10_keeps_other_attach_parameters() {
    let out = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property {
            name: "ATTACH".to_owned(),
            params: vec![
                Param::new("VALUE", "BINARY"),
                Param::new("FMTTYPE", "text/plain"),
            ],
            value: "https://example.com/a.txt".to_owned(),
        },
    ]));
    assert!(
        out.contains("ATTACH;FMTTYPE=text/plain:"),
        "FMTTYPE was dropped:\n{out}"
    );
}

#[test]
fn rule_10_is_scoped_to_attach() {
    // The same parameters on another property are none of rule 10's
    // business and must survive.
    let out = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        Property {
            name: "X-THING".to_owned(),
            params: vec![Param::new("VALUE", "BINARY")],
            value: "v".to_owned(),
        },
    ]));
    assert!(
        out.contains("X-THING;VALUE=BINARY:v"),
        "rule 10 leaked beyond ATTACH:\n{out}"
    );
}

// ---------------------------------------------------------------------
// Rule 11 — DATE values.
// ---------------------------------------------------------------------

/// A DTSTART bearing `VALUE=DATE` plus whatever extra parameters.
fn date_prop(params: Vec<Param>) -> Property {
    Property {
        name: "DTSTART".to_owned(),
        params,
        value: "20260515".to_owned(),
    }
}

#[test]
fn rule_11_emits_a_date_value_verbatim() {
    let out = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        date_prop(vec![Param::new("VALUE", "DATE")]),
    ]));
    assert!(
        out.contains("DTSTART;VALUE=DATE:20260515"),
        "the DATE value was not emitted verbatim:\n{out}"
    );
}

#[test]
fn rule_11_never_promotes_a_date_to_a_date_time() {
    let out = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        date_prop(vec![Param::new("VALUE", "DATE")]),
    ]));
    assert!(
        !out.contains("20260515T"),
        "a DATE was promoted to a DATE-TIME:\n{out}"
    );
    assert!(
        !out.contains("20260515Z") && !out.contains("T000000Z"),
        "a DATE acquired a time or a zone:\n{out}"
    );
}

#[test]
fn rule_11_retains_and_uppercases_the_value_parameter() {
    let lower = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        date_prop(vec![Param::new("VALUE", "date")]),
    ]));
    assert!(
        lower.contains("DTSTART;VALUE=DATE:20260515"),
        "VALUE=date did not converge on VALUE=DATE:\n{lower}"
    );
}

#[test]
fn rule_11_strips_a_tzid_from_a_date() {
    // RFC 5545 §3.2.19 scopes TZID to DATE-TIME and TIME values, so a
    // TZID on a DATE is a producer bug and must not leak into the
    // canonical bytes.
    let out = text(&calendar_with(vec![
        Property::new("UID", "u1"),
        date_prop(vec![
            Param::new("VALUE", "DATE"),
            Param::new("TZID", "America/Montreal"),
        ]),
    ]));
    assert!(!out.contains("TZID"), "a TZID survived on a DATE:\n{out}");
    assert!(out.contains("DTSTART;VALUE=DATE:20260515"));
}

#[test]
fn rule_11_never_consults_the_timezone_registry_for_a_date() {
    // The calendar carries a resolvable VTIMEZONE; a DATE must ignore
    // it entirely.
    let cal = Calendar {
        prod_id: "-//V*//test//EN".to_owned(),
        components: vec![fixed_offset_vtimezone("Test/Zone", "-0500"), {
            let mut c = Component::new(CompType::from_wire("VEVENT"));
            c.add(Property::new("UID", "u1"));
            c.add(date_prop(vec![
                Param::new("VALUE", "DATE"),
                Param::new("TZID", "Test/Zone"),
            ]));
            c
        }],
    };
    let out = text(&cal);
    assert!(
        out.contains("DTSTART;VALUE=DATE:20260515"),
        "a DATE was resolved against the registry:\n{out}"
    );
}

// ---------------------------------------------------------------------
// Rule 12 — DURATION verbatim, including the relative TRIGGER.
// ---------------------------------------------------------------------

#[test]
fn rule_12_preserves_duration_values_verbatim() {
    for spelling in ["P1D", "PT24H", "P1W", "P7D", "P0D", "PT0S", "-PT15M"] {
        let out = text(&calendar_with(vec![
            Property::new("UID", "u1"),
            Property::new("DURATION", spelling),
        ]));
        assert!(
            out.contains(&format!("DURATION:{spelling}")),
            "DURATION {spelling} was rewritten:\n{out}"
        );
    }
}

#[test]
fn rule_12_keeps_equivalent_duration_spellings_distinct() {
    let day = calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("DURATION", "P1D"),
    ]);
    let hours = calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("DURATION", "PT24H"),
    ]);
    assert_ne!(
        hashing::calendar(&day),
        hashing::calendar(&hours),
        "P1D and PT24H hashed the same — a day is not 24 hours"
    );
    let zero_day = calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("DURATION", "P0D"),
    ]);
    let zero_sec = calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("DURATION", "PT0S"),
    ]);
    assert_ne!(
        hashing::calendar(&zero_day),
        hashing::calendar(&zero_sec),
        "P0D and PT0S hashed the same — the day form was lost"
    );
}

#[test]
fn rule_12_preserves_a_relative_trigger_verbatim() {
    let mut alarm = Component::new(CompType::from_wire("VALARM"));
    alarm.add(Property::new("ACTION", "DISPLAY"));
    alarm.add(Property {
        name: "TRIGGER".to_owned(),
        params: vec![Param::new("RELATED", "END")],
        value: "-PT15M".to_owned(),
    });
    let cal = Calendar {
        prod_id: "-//V*//test//EN".to_owned(),
        components: vec![Component {
            r#type: CompType::from_wire("VEVENT"),
            props: vec![Property::new("UID", "u1")],
            sub: vec![alarm],
        }],
    };
    let out = text(&cal);
    assert!(
        out.contains("TRIGGER;RELATED=END:-PT15M"),
        "the relative TRIGGER was rewritten:\n{out}"
    );
}

// ---------------------------------------------------------------------
// Rule 5 / datetime resolution.
// ---------------------------------------------------------------------

/// A VTIMEZONE with one STANDARD child and no DAYLIGHT — the
/// fixed-offset shape the v0.1 subset accepts.
fn fixed_offset_vtimezone(tzid: &str, offset: &str) -> Component {
    let mut std = Component::new(CompType::from_wire("STANDARD"));
    std.add(Property::new("DTSTART", "19700101T000000"));
    std.add(Property::new("TZOFFSETFROM", offset));
    std.add(Property::new("TZOFFSETTO", offset));
    let mut tz = Component::new(CompType::from_wire("VTIMEZONE"));
    tz.add(Property::new("TZID", tzid));
    tz.sub.push(std);
    tz
}

/// A calendar holding `zone` plus one VEVENT whose DTSTART carries the
/// supplied value and parameters.
fn calendar_with_zone(zone: Component, dtstart: Property) -> Calendar {
    let mut evt = Component::new(CompType::from_wire("VEVENT"));
    evt.add(Property::new("UID", "u1"));
    evt.add(dtstart);
    Calendar {
        prod_id: "-//V*//test//EN".to_owned(),
        components: vec![zone, evt],
    }
}

#[test]
fn rule_5_resolves_a_tzid_datetime_to_utc_and_drops_the_tzid() {
    let out = text(&calendar_with_zone(
        fixed_offset_vtimezone("Test/Minus5", "-0500"),
        Property {
            name: "DTSTART".to_owned(),
            params: vec![Param::new("TZID", "Test/Minus5")],
            value: "20260515T120000".to_owned(),
        },
    ));
    assert!(
        out.contains("DTSTART:20260515T170000Z"),
        "the local time did not resolve to UTC:\n{out}"
    );
    assert!(
        !out.contains("TZID=Test/Minus5"),
        "the TZID survived resolution"
    );
}

#[test]
fn rule_5_passes_an_unresolvable_tzid_and_its_value_through_verbatim() {
    let out = text(&calendar_with_zone(
        fixed_offset_vtimezone("Test/Other", "-0500"),
        Property {
            name: "DTSTART".to_owned(),
            params: vec![Param::new("TZID", "Test/Missing")],
            value: "20260515T120000".to_owned(),
        },
    ));
    assert!(
        out.contains("DTSTART;TZID=Test/Missing:20260515T120000"),
        "an unresolvable TZID did not fall back to verbatim emit:\n{out}"
    );
}

#[test]
fn rule_5_leaves_a_utc_value_alone_even_with_a_tzid() {
    let out = text(&calendar_with_zone(
        fixed_offset_vtimezone("Test/Minus5", "-0500"),
        Property {
            name: "DTSTART".to_owned(),
            params: vec![Param::new("TZID", "Test/Minus5")],
            value: "20260515T120000Z".to_owned(),
        },
    ));
    assert!(
        out.contains("DTSTART;TZID=Test/Minus5:20260515T120000Z"),
        "an already-absolute value was re-offset:\n{out}"
    );
}

#[test]
fn rule_5_is_scoped_to_the_allow_list() {
    // X-WHEN is not a rule-5 property, so its TZID is none of the
    // canonical layer's business.
    let out = text(&calendar_with_zone(
        fixed_offset_vtimezone("Test/Minus5", "-0500"),
        Property {
            name: "X-WHEN".to_owned(),
            params: vec![Param::new("TZID", "Test/Minus5")],
            value: "20260515T120000".to_owned(),
        },
    ));
    assert!(
        out.contains("X-WHEN;TZID=Test/Minus5:20260515T120000"),
        "rule 5 reached beyond its allow-list:\n{out}"
    );
}

#[test]
fn component_without_context_emits_a_tzid_datetime_verbatim() {
    // The registry lives on the Calendar, so the context-free entry
    // point genuinely cannot resolve. This is a real distinction, not
    // an overload.
    let mut evt = Component::new(CompType::from_wire("VEVENT"));
    evt.add(Property::new("UID", "u1"));
    evt.add(Property {
        name: "DTSTART".to_owned(),
        params: vec![Param::new("TZID", "Test/Minus5")],
        value: "20260515T120000".to_owned(),
    });
    let bare = String::from_utf8(canonical::component(&evt)).expect("ASCII only");
    assert!(bare.contains("DTSTART;TZID=Test/Minus5:20260515T120000"));

    let cal = Calendar {
        prod_id: "-//V*//test//EN".to_owned(),
        components: vec![fixed_offset_vtimezone("Test/Minus5", "-0500")],
    };
    let resolved =
        String::from_utf8(canonical::component_in_context(&evt, &cal)).expect("ASCII only");
    assert!(
        resolved.contains("DTSTART:20260515T170000Z"),
        "component_in_context did not resolve against the supplied registry:\n{resolved}"
    );
}

// ---------------------------------------------------------------------
// Cards.
// ---------------------------------------------------------------------

#[test]
fn card_emits_version_immediately_after_begin() {
    let c = Card {
        uid: "urn:uuid:1".to_owned(),
        kind: None,
        props: vec![Property::new("FN", "Ada"), Property::new("NOTE", "n")],
    };
    let out = String::from_utf8(canonical::card(&c)).expect("ASCII only");
    let lines: Vec<&str> = out.lines().collect();
    assert_eq!(lines[0], "BEGIN:VCARD");
    assert_eq!(
        lines[1], "VERSION:4.0",
        "RFC 6350 §3.3 requires VERSION immediately after BEGIN:VCARD"
    );
}

#[test]
fn card_sorts_the_remaining_properties_by_name() {
    let c = Card {
        uid: "urn:uuid:1".to_owned(),
        kind: None,
        props: vec![Property::new("NOTE", "n"), Property::new("FN", "Ada")],
    };
    let out = String::from_utf8(canonical::card(&c)).expect("ASCII only");
    let fn_at = out.find("FN:Ada").expect("FN is emitted");
    let note_at = out.find("NOTE:n").expect("NOTE is emitted");
    let uid_at = out.find("UID:urn:uuid:1").expect("UID is emitted");
    assert!(
        fn_at < note_at && note_at < uid_at,
        "card properties are unsorted:\n{out}"
    );
}

#[test]
fn card_absorbs_a_uid_already_present_in_props() {
    let c = Card {
        uid: "urn:uuid:1".to_owned(),
        kind: None,
        props: vec![Property::new("UID", "urn:uuid:1")],
    };
    let out = String::from_utf8(canonical::card(&c)).expect("ASCII only");
    assert_eq!(
        out.matches("UID:").count(),
        1,
        "the UID was emitted twice:\n{out}"
    );
}

#[test]
fn card_normalizes_values_to_nfc() {
    let decomposed = Card {
        uid: "u".to_owned(),
        kind: None,
        props: vec![Property::new("FN", DECOMPOSED)],
    };
    let composed = Card {
        uid: "u".to_owned(),
        kind: None,
        props: vec![Property::new("FN", COMPOSED)],
    };
    assert_eq!(canonical::card(&decomposed), canonical::card(&composed));
}

// ---------------------------------------------------------------------
// Hashing surface.
// ---------------------------------------------------------------------

#[test]
fn hashes_carry_the_sha256_prefix_and_lowercase_hex() {
    let h = hashing::calendar(&calendar_with(vec![Property::new("UID", "u1")]));
    let hex = h
        .strip_prefix("sha256:")
        .expect("the sha256: prefix is part of the value");
    assert_eq!(hex.len(), 64, "a SHA-256 digest is 64 hex characters");
    assert!(
        hex.bytes()
            .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b)),
        "the digest is not lowercase hex: {hex}"
    );
}

#[test]
fn set_x_vstar_writes_a_hash_that_verifies() {
    let mut c = Component::new(CompType::from_wire("VEVENT"));
    c.add(Property::new("UID", "u1"));
    c.add(Property::new("DTSTAMP", "20260101T000000Z"));

    assert_eq!(hashing::get_x_vstar(&c), None);
    let (ok, want, got) = hashing::verify_x_vstar(&c);
    assert!(!ok, "an unstamped component verified");
    assert_eq!(got, "", "no hash is stored yet");
    assert!(want.starts_with("sha256:"));

    hashing::set_x_vstar(&mut c);
    let stored = hashing::get_x_vstar(&c).expect("a hash was written");
    assert_eq!(stored, want);
    let (ok, want2, got2) = hashing::verify_x_vstar(&c);
    assert!(ok, "a freshly stamped component failed verification");
    assert_eq!(want2, got2);
}

#[test]
fn set_x_vstar_is_idempotent_and_replaces_rather_than_duplicates() {
    let mut c = Component::new(CompType::from_wire("VEVENT"));
    c.add(Property::new("UID", "u1"));
    hashing::set_x_vstar(&mut c);
    let first = hashing::get_x_vstar(&c).expect("stamped").to_owned();
    hashing::set_x_vstar(&mut c);
    assert_eq!(hashing::get_x_vstar(&c), Some(first.as_str()));
    assert_eq!(
        c.get_all(hashing::X_VSTAR_HASH_PROPERTY).len(),
        1,
        "the hash property was duplicated"
    );
}

#[test]
fn verify_reports_want_and_got_on_a_mismatch() {
    let mut c = Component::new(CompType::from_wire("VEVENT"));
    c.add(Property::new("UID", "u1"));
    c.add(Property::new(
        hashing::X_VSTAR_HASH_PROPERTY,
        "sha256:wrong",
    ));
    let (ok, want, got) = hashing::verify_x_vstar(&c);
    assert!(!ok);
    assert_eq!(got, "sha256:wrong");
    assert_ne!(
        want, got,
        "want must be the recomputed hash, not the stored one"
    );
}

#[test]
fn component_hash_matches_the_digest_of_the_component_canonical_bytes() {
    // The hash is the digest of the canonical bytes and nothing else;
    // re-deriving it here would just restate the implementation, so the
    // assertion is the weaker but independent one: equal canonical
    // bytes imply equal hashes, and unequal imply unequal.
    let mut a = Component::new(CompType::from_wire("VEVENT"));
    a.add(Property::new("UID", "u1"));
    let mut b = a.clone();
    b.add(Property::new("SUMMARY", "s"));
    assert_ne!(canonical::component(&a), canonical::component(&b));
    assert_ne!(hashing::component(&a), hashing::component(&b));

    let a2 = a.clone();
    assert_eq!(canonical::component(&a), canonical::component(&a2));
    assert_eq!(hashing::component(&a), hashing::component(&a2));
}

#[test]
fn canonicalization_does_not_mutate_its_input() {
    let cal = calendar_with(vec![
        Property::new("UID", "u1"),
        Property::new("SUMMARY", DECOMPOSED),
        Property::new(hashing::X_VSTAR_HASH_PROPERTY, "sha256:0"),
    ]);
    let before = cal.clone();
    let _ = canonical::calendar(&cal);
    let _ = hashing::calendar(&cal);
    assert_eq!(cal, before, "canonicalization mutated its input");
}

#[test]
fn the_corpus_fold_fixture_round_trips_through_the_parser() {
    // A split sequence is only decodable after unfolding; this asserts
    // the layer-(a) unfolder and the layer-(b) folder agree.
    let src = std::fs::read(
        std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("..")
            .join("spec")
            .join("v0.1")
            .join("conformance")
            .join("rfc5545")
            .join("fold_split_utf8.ics"),
    )
    .expect("the fixture is in the corpus");
    let cal = rfc5545::parse(src.as_slice()).expect("the fixture parses");
    let once = canonical::calendar(&cal);
    let again = rfc5545::parse(once.as_slice()).expect("canonical bytes re-parse");
    assert_eq!(
        canonical::calendar(&again),
        once,
        "the split-sequence fixture is not a fixpoint"
    );
}
