// SPDX-License-Identifier: MIT

//! Layer (a) model surface: the names and behaviors `docs/dev/api-mapping.md`
//! fixes for Rust.
//!
//! Rust keeps the Go spellings where TypeScript, Python and PHP had to
//! rename — `Date` (not `VDate`) and `Class` (not `VClass`) — because
//! module paths disambiguate and the crate root has no collision.

use hop_top_vstar::{
    date_of, format_date, parse_date, parse_rel_type, property_equal, Calendar, Card, Class,
    CompType, Component, EventStatus, JournalStatus, Kind, Param, Property, RelType, TodoStatus,
    Transp, DEFAULT_REL_TYPE, VALUE_DATE, VALUE_PARAM,
};

fn prop(name: &str, value: &str) -> Property {
    Property {
        name: name.into(),
        params: Vec::new(),
        value: value.into(),
    }
}

#[test]
fn model_types_derive_the_required_traits() {
    // Debug, Clone, PartialEq, Eq on every model type.
    fn assert_traits<T: std::fmt::Debug + Clone + PartialEq + Eq>(v: T) {
        let c = v.clone();
        assert_eq!(v, c);
        assert!(!format!("{v:?}").is_empty());
    }
    assert_traits(Param {
        name: "CN".into(),
        value: "Jad".into(),
    });
    assert_traits(prop("UID", "u"));
    assert_traits(Component::default());
    assert_traits(Calendar::default());
    assert_traits(Card::default());
    assert_traits(hop_top_vstar::Date {
        year: 2026,
        month: 5,
        day: 15,
    });
}

#[test]
fn component_accessors_are_case_insensitive_and_order_preserving() {
    let mut c = Component {
        r#type: CompType::TODO.clone(),
        props: vec![prop("UID", "u"), prop("X-A", "1"), prop("x-a", "2")],
        sub: Vec::new(),
    };

    assert_eq!(c.get("uid").map(|p| p.value.as_str()), Some("u"));
    assert_eq!(c.get("UID").map(|p| p.value.as_str()), Some("u"));
    assert_eq!(c.get("nope"), None);

    let all = c.get_all("X-A");
    assert_eq!(all.len(), 2, "get_all is case-insensitive");
    assert_eq!(all[0].value, "1");
    assert_eq!(all[1].value, "2", "get_all preserves wire order");

    assert_eq!(c.uid(), "u");
    assert_eq!(c.dtstamp_raw(), "");

    // set replaces every match with one copy, in the first match's slot.
    c.set(prop("X-A", "replaced"));
    let names: Vec<&str> = c.props.iter().map(|p| p.name.as_str()).collect();
    assert_eq!(names, ["UID", "X-A"]);
    assert_eq!(c.get("X-A").unwrap().value, "replaced");

    // add appends without touching existing properties of the same name.
    c.add(prop("X-A", "second"));
    assert_eq!(c.get_all("X-A").len(), 2);

    // remove deletes every match.
    c.remove("x-a");
    assert_eq!(c.get_all("X-A").len(), 0);
    assert_eq!(c.props.len(), 1);

    // set on an absent name appends.
    c.set(prop("SUMMARY", "s"));
    assert_eq!(c.props.last().unwrap().name, "SUMMARY");
}

#[test]
fn card_accessors_mirror_component() {
    let mut card = Card {
        uid: "u".into(),
        kind: Some(Kind::Individual),
        props: vec![prop("FN", "Jad"), prop("fn", "Duplicate")],
    };
    assert_eq!(card.get("FN").map(|p| p.value.as_str()), Some("Jad"));
    assert_eq!(card.get_all("fn").len(), 2);

    card.set(prop("FN", "One"));
    assert_eq!(card.get_all("FN").len(), 1);
    card.add(prop("FN", "Two"));
    assert_eq!(card.get_all("FN").len(), 2);
    card.remove("FN");
    assert!(card.props.is_empty());
}

/// Go spells an absent `KIND` as the empty string; Rust spells it `None`.
/// The distinction is load-bearing at encode time: a card that carried no
/// `KIND` must not gain one on the way out.
#[test]
fn card_kind_is_absent_rather_than_defaulted() {
    use hop_top_vstar::codec::rfc6350;

    let cards = rfc6350::parse(&b"BEGIN:VCARD\nVERSION:4.0\nUID:u\nFN:No Kind\nEND:VCARD\n"[..])
        .expect("parse");
    assert_eq!(cards[0].kind, None, "no KIND on the wire means None");

    let mut out = Vec::new();
    rfc6350::encode(&mut out, &cards[0]).expect("encode");
    assert!(
        !String::from_utf8_lossy(&out).contains("KIND:"),
        "an absent KIND must not be invented on encode"
    );

    let cards =
        rfc6350::parse(&b"BEGIN:VCARD\nVERSION:4.0\nUID:u\nKIND:ORG\nFN:Org\nEND:VCARD\n"[..])
            .expect("parse");
    assert_eq!(
        cards[0].kind,
        Some(Kind::Org),
        "KIND folds case-insensitively on read"
    );
    let mut out = Vec::new();
    rfc6350::encode(&mut out, &cards[0]).expect("encode");
    assert!(
        String::from_utf8_lossy(&out).contains("KIND:org\r\n"),
        "KIND emits in the RFC 6350 §6.1.4 registry's lowercase form"
    );
}

#[test]
fn calendar_find_append_and_filter() {
    let mut cal = Calendar {
        prod_id: "-//V*//Test//EN".into(),
        components: vec![
            Component {
                r#type: CompType::TODO.clone(),
                props: vec![prop("UID", "a")],
                sub: Vec::new(),
            },
            Component {
                r#type: CompType::EVENT.clone(),
                props: vec![prop("UID", "b")],
                sub: Vec::new(),
            },
        ],
    };

    assert_eq!(cal.find("a").map(|c| &c.r#type), Some(&CompType::TODO));
    assert_eq!(cal.find("b").map(|c| &c.r#type), Some(&CompType::EVENT));
    assert!(cal.find("A").is_none(), "UID comparison is case-sensitive");
    assert!(cal.find("missing").is_none());

    assert_eq!(cal.filter(&CompType::TODO).len(), 1);
    assert_eq!(cal.filter(&CompType::JOURNAL).len(), 0);

    cal.append(Component {
        r#type: CompType::TODO.clone(),
        props: vec![prop("UID", "c")],
        sub: Vec::new(),
    });
    assert_eq!(cal.components.len(), 3);
    assert_eq!(cal.filter(&CompType::TODO).len(), 2);
}

#[test]
fn wire_string_enums_carry_their_rfc_spellings() {
    assert_eq!(CompType::CALENDAR.as_str(), "VCALENDAR");
    assert_eq!(CompType::TODO.as_str(), "VTODO");
    assert_eq!(CompType::JOURNAL.as_str(), "VJOURNAL");
    assert_eq!(CompType::EVENT.as_str(), "VEVENT");
    assert_eq!(CompType::FREE_BUSY.as_str(), "VFREEBUSY");
    assert_eq!(CompType::TIMEZONE.as_str(), "VTIMEZONE");
    assert_eq!(CompType::ALARM.as_str(), "VALARM");

    assert_eq!(Kind::Individual.as_str(), "individual");
    assert_eq!(Kind::Org.as_str(), "org");
    assert_eq!(Kind::Group.as_str(), "group");

    assert_eq!(TodoStatus::NeedsAction.as_str(), "NEEDS-ACTION");
    assert_eq!(TodoStatus::InProcess.as_str(), "IN-PROCESS");
    assert_eq!(TodoStatus::Completed.as_str(), "COMPLETED");
    assert_eq!(TodoStatus::Cancelled.as_str(), "CANCELLED");

    assert_eq!(EventStatus::Tentative.as_str(), "TENTATIVE");
    assert_eq!(EventStatus::Confirmed.as_str(), "CONFIRMED");
    assert_eq!(EventStatus::Cancelled.as_str(), "CANCELLED");

    assert_eq!(JournalStatus::Draft.as_str(), "DRAFT");
    assert_eq!(JournalStatus::Final.as_str(), "FINAL");
    assert_eq!(JournalStatus::Cancelled.as_str(), "CANCELLED");

    assert_eq!(Class::Public.as_str(), "PUBLIC");
    assert_eq!(Class::Private.as_str(), "PRIVATE");
    assert_eq!(Class::Confidential.as_str(), "CONFIDENTIAL");

    assert_eq!(Transp::Opaque.as_str(), "OPAQUE");
    assert_eq!(Transp::Transparent.as_str(), "TRANSPARENT");
}

/// The three status vocabularies are distinct types. This does not
/// compile if they collapse into one, which is the point — the assertion
/// here is only that their shared wire spelling did not merge them.
#[test]
fn status_vocabularies_are_three_distinct_types() {
    let t: TodoStatus = TodoStatus::Cancelled;
    let e: EventStatus = EventStatus::Cancelled;
    let j: JournalStatus = JournalStatus::Cancelled;
    assert_eq!(t.as_str(), e.as_str());
    assert_eq!(e.as_str(), j.as_str());

    // Round-tripping through the wire string stays inside one vocabulary.
    assert_eq!(TodoStatus::parse("IN-PROCESS"), Some(TodoStatus::InProcess));
    assert_eq!(EventStatus::parse("IN-PROCESS"), None);
    assert_eq!(JournalStatus::parse("DRAFT"), Some(JournalStatus::Draft));
    assert_eq!(TodoStatus::parse("DRAFT"), None);
}

#[test]
fn rel_type_is_an_open_enum_over_the_rfc_9253_registry() {
    // The registered vocabulary, all twelve.
    for name in [
        "PARENT",
        "CHILD",
        "SIBLING",
        "FINISHTOSTART",
        "FINISHTOFINISH",
        "STARTTOFINISH",
        "STARTTOSTART",
        "DEPENDS-ON",
        "FIRST",
        "NEXT",
        "CONCEPT",
        "REFID",
    ] {
        let (rt, registered) = parse_rel_type(name);
        assert!(registered, "{name} should be registered");
        assert_eq!(rt.as_str(), name);
    }

    // Case-insensitive folding onto the canonical spelling.
    let (rt, registered) = parse_rel_type("finishtostart");
    assert!(registered);
    assert_eq!(rt.as_str(), "FINISHTOSTART");

    // Empty input means PARENT, and reports registered.
    let (rt, registered) = parse_rel_type("");
    assert!(registered);
    assert_eq!(rt, DEFAULT_REL_TYPE);
    assert_eq!(rt.as_str(), "PARENT");

    // An unregistered value comes back VERBATIM with registered=false —
    // not None, not folded, not uppercased.
    let (rt, registered) = parse_rel_type("X-Custom-Link");
    assert!(!registered);
    assert_eq!(rt.as_str(), "X-Custom-Link");

    // eq_fold compares case-insensitively without allocating.
    assert!(RelType::parent().eq_fold("parent"));
    assert!(RelType::parent().eq_fold("PARENT"));
    assert!(!RelType::parent().eq_fold("CHILD"));
}

#[test]
fn value_param_constants_match_the_rfc() {
    assert_eq!(VALUE_PARAM, "VALUE");
    assert_eq!(VALUE_DATE, "DATE");
}

#[test]
fn date_parses_formats_and_rejects_impossible_values() {
    let d = parse_date("20260515").expect("a well-formed DATE");
    assert_eq!(d.year, 2026);
    assert_eq!(d.month, 5);
    assert_eq!(d.day, 15);
    assert_eq!(format_date(d), "20260515");
    assert_eq!(d.to_string(), "20260515");
    assert!(!d.is_zero());

    // The zero Date is the "no date" sentinel and formats as empty.
    let zero = hop_top_vstar::Date::default();
    assert!(zero.is_zero());
    assert_eq!(format_date(zero), "");
    assert_eq!(zero.to_string(), "");

    // Strict: no DATE-TIME, no ISO-8601 extended, no impossible dates,
    // no whitespace, no wrong width.
    for bad in [
        "",
        "2026051",
        "202605155",
        "2026-05-15",
        "20260515T120000",
        "20260515T120000Z",
        " 20260515",
        "20260515 ",
        "20260230",
        "20261301",
        "20260500",
        "20260532",
        "20250229",
        "2026o515",
    ] {
        assert!(
            parse_date(bad).is_none(),
            "{bad:?} should not parse as a DATE"
        );
    }

    // A real leap day does parse.
    assert!(parse_date("20240229").is_some());

    // Out-of-range fields format as the empty string rather than a torn
    // wire form.
    assert_eq!(
        format_date(hop_top_vstar::Date {
            year: 2026,
            month: 13,
            day: 1
        }),
        ""
    );
    assert_eq!(
        format_date(hop_top_vstar::Date {
            year: 2026,
            month: 1,
            day: 32
        }),
        ""
    );
    assert_eq!(
        format_date(hop_top_vstar::Date {
            year: 10000,
            month: 1,
            day: 1
        }),
        ""
    );

    // Zero-padding is exact.
    assert_eq!(
        format_date(hop_top_vstar::Date {
            year: 7,
            month: 2,
            day: 3
        }),
        "00070203"
    );
}

#[test]
fn date_of_reads_a_datetime_as_a_calendar_date() {
    use chrono::TimeZone;
    let t = chrono::Utc.with_ymd_and_hms(2026, 5, 15, 20, 0, 0).unwrap();
    let d = date_of(t);
    assert_eq!(
        d,
        hop_top_vstar::Date {
            year: 2026,
            month: 5,
            day: 15
        }
    );
    assert_eq!(
        d.to_datetime(),
        chrono::Utc.with_ymd_and_hms(2026, 5, 15, 0, 0, 0).unwrap()
    );
}

#[test]
fn date_accessors_require_value_date() {
    let mut c = Component {
        r#type: CompType::TODO.clone(),
        props: vec![
            prop("UID", "u"),
            Property {
                name: "DTSTART".into(),
                params: vec![Param {
                    name: VALUE_PARAM.into(),
                    value: VALUE_DATE.into(),
                }],
                value: "20260515".into(),
            },
            // No VALUE=DATE: an untagged 8-octet value declares itself
            // DATE-TIME by default and must NOT be promoted to a Date.
            prop("DUE", "20260516"),
            prop("DTEND", "20260516T170000Z"),
        ],
        sub: Vec::new(),
    };

    assert!(c.is_date_only("DTSTART"));
    assert!(
        c.is_date_only("dtstart"),
        "the name match is case-insensitive"
    );
    assert!(!c.is_date_only("DUE"));
    assert!(!c.is_date_only("DTEND"));
    assert!(!c.is_date_only("ABSENT"));

    assert_eq!(
        c.dtstart_date(),
        Some(hop_top_vstar::Date {
            year: 2026,
            month: 5,
            day: 15
        })
    );
    assert_eq!(c.due_date(), None, "an untagged value is not a DATE");
    assert_eq!(c.dtend_date(), None);
    assert_eq!(c.completed_date(), None);

    // A lowercase VALUE=date is still VALUE=DATE: the parameter value is
    // a registered token and case-insensitive.
    c.set(Property {
        name: "DUE".into(),
        params: vec![Param {
            name: "value".into(),
            value: "date".into(),
        }],
        value: "20260516".into(),
    });
    assert!(c.is_date_only("DUE"));
    assert_eq!(
        c.due_date(),
        Some(hop_top_vstar::Date {
            year: 2026,
            month: 5,
            day: 16
        })
    );
}

#[test]
fn date_setters_write_value_date_and_drop_stale_params() {
    let mut c = Component {
        r#type: CompType::TODO.clone(),
        props: vec![
            prop("UID", "u"),
            Property {
                name: "DUE".into(),
                params: vec![Param {
                    name: "TZID".into(),
                    value: "America/Montreal".into(),
                }],
                value: "20260516T170000".into(),
            },
        ],
        sub: Vec::new(),
    };

    let d = hop_top_vstar::Date {
        year: 2026,
        month: 6,
        day: 1,
    };
    c.set_due_date(d);
    let p = c.get("DUE").expect("DUE");
    assert_eq!(p.value, "20260601");
    assert_eq!(
        p.params,
        vec![Param {
            name: VALUE_PARAM.into(),
            value: VALUE_DATE.into()
        }],
        "a stale TZID must not survive a date-only write"
    );

    c.set_dtstart_date(d);
    c.set_dtend_date(d);
    c.set_completed_date(d);
    assert!(c.is_date_only("DTSTART"));
    assert!(c.is_date_only("DTEND"));
    assert!(c.is_date_only("COMPLETED"));

    // The zero Date clears the property.
    c.set_due_date(hop_top_vstar::Date::default());
    assert!(c.get("DUE").is_none());
}

#[test]
fn property_equal_folds_names_and_normalizes_param_order() {
    let a = Property {
        name: "ATTENDEE".into(),
        params: vec![
            Param {
                name: "CN".into(),
                value: "Jad".into(),
            },
            Param {
                name: "ROLE".into(),
                value: "CHAIR".into(),
            },
        ],
        value: "mailto:jad@example.com".into(),
    };
    let b = Property {
        name: "attendee".into(),
        params: vec![
            Param {
                name: "role".into(),
                value: "CHAIR".into(),
            },
            Param {
                name: "cn".into(),
                value: "Jad".into(),
            },
        ],
        value: "mailto:jad@example.com".into(),
    };
    assert!(property_equal(&a, &b), "names fold, param order normalizes");

    // Values are case-SENSITIVE.
    let mut c = b.clone();
    c.value = "MAILTO:jad@example.com".into();
    assert!(!property_equal(&a, &c));

    // A differing parameter value is a difference.
    let mut d = b.clone();
    d.params[0].value = "REQ-PARTICIPANT".into();
    assert!(!property_equal(&a, &d));

    // A differing parameter count is a difference.
    let mut e = b.clone();
    e.params.pop();
    assert!(!property_equal(&a, &e));

    // Inputs are not mutated by the comparison.
    assert_eq!(b.params[0].name, "role");
}
