// SPDX-License-Identifier: MIT

//! RFC 5545 §3.3.6 DURATION and the §3.8.6.3 TRIGGER property.
//!
//! The gates are `spec/behavior/duration/parse.json` and the
//! `*.trigger.json` sidecars. One trap in the first: its `negative`
//! column is `is_negative()`, NOT the struct's sign flag — `-PT0S`
//! carries the flag and reports `negative: false`, because a
//! zero-length duration is never subtractive.
//!
//! The `day_form` flag is the second: `P0D` and `PT0S` are numerically
//! identical and every unit field is zero in both, so without the flag
//! `Display` cannot reproduce the authored spelling — and canonical
//! form preserves DURATION verbatim (spec rule 12).

mod support;

use chrono::Duration as ChronoDuration;
use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::duration::{
    alarm_repeat_cycle, alarm_trigger, event_end, from_signed, parse, parse_trigger, valid,
    Duration, Related, Trigger,
};
use hop_top_vstar::{format_time, parse_time, Calendar, Component, Param, Property};
use support::{behavior_input, behavior_json, behavior_stems};

// ---------------------------------------------------------------------
// The parse.json gate.
// ---------------------------------------------------------------------

#[test]
fn duration_parsing_matches_the_behavior_corpus() {
    let rows = behavior_json("duration", "parse.json");
    let cases = rows.array();
    assert!(
        cases.len() >= 23,
        "expected the full parse table, saw {}",
        cases.len()
    );

    for (i, case) in cases.iter().enumerate() {
        let value = case.str_field("value").unwrap_or("");
        match case.str_field("error") {
            Some(sentinel) => {
                let err = parse(value)
                    .err()
                    .unwrap_or_else(|| panic!("row {i}: {value:?} parsed but should fail"));
                assert_eq!(err.sentinel(), sentinel, "row {i}: {value:?}");
                assert!(!valid(value), "row {i}: valid() disagrees with parse()");
            }
            None => {
                let d = parse(value)
                    .unwrap_or_else(|e| panic!("row {i}: {value:?} failed to parse: {e}"));
                let want_seconds = case.i64_field("seconds").expect("an ok row names seconds");
                assert_eq!(
                    d.signed().num_seconds(),
                    want_seconds,
                    "row {i}: {value:?} signed seconds"
                );
                // The column is is_negative(), not the sign flag: a
                // zero-length duration is never negative however it was
                // authored, so `-PT0S` is `negative: false`.
                let want_negative = case
                    .bool_field("negative")
                    .expect("an ok row names negative");
                assert_eq!(
                    d.is_negative(),
                    want_negative,
                    "row {i}: {value:?} is_negative — the column is is_negative(), \
                     not the struct's sign flag"
                );
                assert!(valid(value), "row {i}: valid() disagrees with parse()");
            }
        }
    }
}

// ---------------------------------------------------------------------
// The trigger sidecars.
// ---------------------------------------------------------------------

/// Depth-first search for a sub-component whose UID matches, returning
/// it together with its parent.
fn find_alarm<'a>(cal: &'a Calendar, uid: &str) -> Option<(&'a Component, &'a Component)> {
    fn walk<'a>(c: &'a Component, uid: &str) -> Option<(&'a Component, &'a Component)> {
        for sub in &c.sub {
            if sub.uid() == uid {
                return Some((sub, c));
            }
            if let Some(found) = walk(sub, uid) {
                return Some(found);
            }
        }
        None
    }
    cal.components.iter().find_map(|c| walk(c, uid))
}

#[test]
fn trigger_resolution_matches_the_behavior_corpus() {
    let stems = behavior_stems("duration", ".trigger.json");
    assert!(
        stems.len() >= 7,
        "expected every trigger sidecar, saw {}",
        stems.len()
    );

    for stem in stems {
        let src = behavior_input("duration", &format!("{stem}.ics"));
        let cal =
            rfc5545::parse(src.as_slice()).unwrap_or_else(|e| panic!("{stem}.ics: parse: {e}"));
        let rows = behavior_json("duration", &format!("{stem}.trigger.json"));

        for (i, case) in rows.array().iter().enumerate() {
            let uid = case
                .str_field("alarm_uid")
                .expect("every row names an alarm");
            let (alarm, parent) = find_alarm(&cal, uid)
                .unwrap_or_else(|| panic!("{stem} row {i}: no VALARM with UID {uid}"));

            let got = alarm_trigger(alarm).and_then(|t| t.resolve(parent, &cal));

            match case.str_field("error") {
                Some(sentinel) => {
                    let err = got.err().unwrap_or_else(|| {
                        panic!("{stem} row {i}: {uid} resolved but should fail")
                    });
                    assert_eq!(err.sentinel(), sentinel, "{stem} row {i}: {uid}");
                }
                None => {
                    let want = case
                        .str_field("fires_at")
                        .expect("an ok row names fires_at");
                    let at = got.unwrap_or_else(|e| panic!("{stem} row {i}: {uid}: {e}"));
                    assert_eq!(format_time(at), want, "{stem} row {i}: {uid}");
                }
            }
        }
    }
}

// ---------------------------------------------------------------------
// Parsing and formatting.
// ---------------------------------------------------------------------

#[test]
fn parse_and_display_round_trip_the_authored_units() {
    for s in [
        "P1W",
        "P26W",
        "P7D",
        "P1D",
        "PT1H",
        "PT15M",
        "PT30S",
        "P1DT2H30M45S",
        "PT1H30M",
        "-PT15M",
        "-P1DT2H",
        "PT0S",
        "P0D",
        "PT24H",
    ] {
        let d = parse(s).unwrap_or_else(|e| panic!("{s}: {e}"));
        assert_eq!(d.to_string(), s, "{s} did not round-trip");
    }
}

#[test]
fn an_explicit_plus_sign_is_the_one_normalization() {
    assert_eq!(parse("+PT15M").expect("parses").to_string(), "PT15M");
}

#[test]
fn a_negative_zero_renders_without_its_sign() {
    // `-PT0S` is zero, and `is_negative` reports false, so `Display`
    // drops the sign rather than emitting a signed zero.
    let d = parse("-PT0S").expect("parses");
    assert!(!d.is_negative());
    assert_eq!(d.to_string(), "PT0S");
    assert_eq!(d.signed(), ChronoDuration::zero());
}

#[test]
fn day_form_distinguishes_p0d_from_pt0s() {
    let day = parse("P0D").expect("parses");
    let sec = parse("PT0S").expect("parses");
    assert!(day.day_form, "P0D did not record the day form");
    assert!(!sec.day_form, "PT0S recorded the day form");
    assert_eq!(day.to_string(), "P0D");
    assert_eq!(sec.to_string(), "PT0S");
    // The flag affects formatting only.
    assert_eq!(day.signed(), sec.signed());
    assert_eq!(day.is_negative(), sec.is_negative());
    let anchor = parse_time("20260515T120000Z").expect("parses");
    assert_eq!(day.add_to(anchor), sec.add_to(anchor));
}

#[test]
fn parsing_is_strict() {
    for bad in [
        "",       // empty
        "T15M",   // no P designator
        "P",      // nothing after P
        "P1W2D",  // weeks mixed with days
        "P1WT1H", // weeks mixed with a time part
        "PT1X",   // unknown unit
        "PT1M1H", // out of order
        "P1DT30", // digits with no unit
        "PT1HM",  // unit with no digits
        "pt15m",  // lowercase
        "PT",     // empty time part
        "P1Y",    // ISO 8601 years, not in RFC 5545
        "P1M",    // ISO 8601 months
        "PT1.5H", // fractional
        "PT 15M", // whitespace
        "P-1D",   // per-component sign
        "PT1H1H", // repeated unit
    ] {
        let err = parse(bad).err().unwrap_or_else(|| panic!("{bad:?} parsed"));
        assert_eq!(err.sentinel(), "ErrMalformed", "{bad:?}");
    }
}

#[test]
fn signed_counts_days_as_twenty_four_hours_and_weeks_as_seven_days() {
    assert_eq!(
        parse("P1W").expect("parses").signed().num_seconds(),
        604_800
    );
    assert_eq!(parse("P1D").expect("parses").signed().num_seconds(), 86_400);
    assert_eq!(
        parse("PT24H").expect("parses").signed().num_seconds(),
        86_400
    );
    assert_eq!(
        parse("-PT15M").expect("parses").signed().num_seconds(),
        -900
    );
}

#[test]
fn add_to_moves_forward_and_backward() {
    let anchor = parse_time("20260601T090000Z").expect("parses");
    assert_eq!(
        format_time(parse("-PT15M").expect("parses").add_to(anchor)),
        "20260601T084500Z"
    );
    assert_eq!(
        format_time(parse("P1D").expect("parses").add_to(anchor)),
        "20260602T090000Z"
    );
    assert_eq!(
        format_time(parse("P1W").expect("parses").add_to(anchor)),
        "20260608T090000Z"
    );
    assert_eq!(
        format_time(parse("-P1DT2H").expect("parses").add_to(anchor)),
        "20260531T070000Z"
    );
}

#[test]
fn from_signed_never_invents_calendar_units() {
    // A chrono::Duration carries no calendar information, so emitting
    // `P1D` from 24 hours would invent a distinction the input never
    // made.
    let d = from_signed(ChronoDuration::hours(24));
    assert_eq!(d.days, 0);
    assert_eq!(d.weeks, 0);
    assert_eq!(d.to_string(), "PT24H");

    let neg = from_signed(ChronoDuration::minutes(-15));
    assert!(neg.is_negative());
    assert_eq!(neg.to_string(), "-PT15M");

    assert_eq!(from_signed(ChronoDuration::zero()).to_string(), "PT0S");
}

#[test]
fn from_signed_truncates_sub_second_precision() {
    let d = from_signed(ChronoDuration::milliseconds(1_500));
    assert_eq!(d.to_string(), "PT1S");
    let neg = from_signed(ChronoDuration::milliseconds(-1_500));
    assert_eq!(neg.to_string(), "-PT1S");
}

#[test]
fn the_default_duration_is_a_valid_positive_zero() {
    let d = Duration::default();
    assert_eq!(d.to_string(), "PT0S");
    assert!(!d.is_negative());
    assert_eq!(d.signed(), ChronoDuration::zero());
}

// ---------------------------------------------------------------------
// Triggers.
// ---------------------------------------------------------------------

/// A TRIGGER property with the given parameters and value.
fn trigger_prop(params: Vec<Param>, value: &str) -> Property {
    Property {
        name: "TRIGGER".to_owned(),
        params,
        value: value.to_owned(),
    }
}

#[test]
fn an_explicit_value_parameter_is_authoritative() {
    // VALUE=DURATION with a non-duration is malformed rather than
    // silently re-read as the other form.
    let err = parse_trigger(&trigger_prop(
        vec![Param::new("VALUE", "DURATION")],
        "20260601T090000Z",
    ))
    .expect_err("a contradiction is malformed");
    assert_eq!(err.sentinel(), "ErrMalformed");

    let err = parse_trigger(&trigger_prop(
        vec![Param::new("VALUE", "DATE-TIME")],
        "-PT15M",
    ))
    .expect_err("a contradiction is malformed");
    assert_eq!(err.sentinel(), "ErrMalformed");

    let err = parse_trigger(&trigger_prop(vec![Param::new("VALUE", "TEXT")], "-PT15M"))
        .expect_err("an unsupported VALUE is malformed");
    assert_eq!(err.sentinel(), "ErrMalformed");
}

#[test]
fn an_absent_value_parameter_lets_the_value_decide() {
    let rel = parse_trigger(&trigger_prop(Vec::new(), "-PT15M")).expect("a duration is relative");
    assert!(rel.relative);
    assert_eq!(rel.duration.to_string(), "-PT15M");

    let abs = parse_trigger(&trigger_prop(Vec::new(), "20260601T090000Z"))
        .expect("an instant is absolute");
    assert!(!abs.relative);
    assert_eq!(
        format_time(
            abs.absolute
                .expect("an absolute trigger carries its instant")
        ),
        "20260601T090000Z"
    );

    let err = parse_trigger(&trigger_prop(Vec::new(), "not-a-value")).expect_err("neither form");
    assert_eq!(err.sentinel(), "ErrMalformed");
}

#[test]
fn related_defaults_to_start_and_is_matched_case_insensitively() {
    let implicit = parse_trigger(&trigger_prop(Vec::new(), "-PT15M")).expect("parses");
    assert_eq!(implicit.related, Related::Start);

    let end =
        parse_trigger(&trigger_prop(vec![Param::new("related", "end")], "-PT15M")).expect("parses");
    assert_eq!(end.related, Related::End);
}

#[test]
fn related_is_rejected_on_an_absolute_trigger() {
    // RFC 5545 §3.2.14 scopes RELATED to DURATION-valued triggers.
    let err = parse_trigger(&trigger_prop(
        vec![Param::new("RELATED", "END")],
        "20260601T090000Z",
    ))
    .expect_err("RELATED on an absolute trigger is malformed");
    assert_eq!(err.sentinel(), "ErrMalformed");
}

#[test]
fn an_unknown_related_value_is_malformed() {
    let err = parse_trigger(&trigger_prop(
        vec![Param::new("RELATED", "MIDDLE")],
        "-PT15M",
    ))
    .expect_err("unknown RELATED");
    assert_eq!(err.sentinel(), "ErrMalformed");
}

#[test]
fn related_stringifies_to_its_wire_spelling() {
    assert_eq!(Related::Start.to_string(), "START");
    assert_eq!(Related::End.to_string(), "END");
    assert_eq!(Related::default(), Related::Start);
}

#[test]
fn to_property_omits_the_default_related_and_spells_out_the_absolute_form() {
    let rel = Trigger {
        relative: true,
        duration: parse("-PT15M").expect("parses"),
        related: Related::Start,
        absolute: None,
    };
    let p = rel.to_property();
    assert_eq!(p.name, "TRIGGER");
    assert_eq!(p.value, "-PT15M");
    assert!(p.params.is_empty(), "RELATED=START is the RFC default");

    let end = Trigger {
        related: Related::End,
        ..rel.clone()
    };
    let p = end.to_property();
    assert_eq!(p.params, vec![Param::new("RELATED", "END")]);

    let abs = Trigger {
        relative: false,
        duration: Duration::default(),
        related: Related::Start,
        absolute: parse_time("20260601T090000Z"),
    };
    let p = abs.to_property();
    assert_eq!(p.value, "20260601T090000Z");
    assert_eq!(p.params, vec![Param::new("VALUE", "DATE-TIME")]);
}

#[test]
fn to_property_round_trips_through_parse_trigger() {
    for (params, value) in [
        (Vec::new(), "-PT15M"),
        (vec![Param::new("RELATED", "END")], "-PT10M"),
        (vec![Param::new("VALUE", "DATE-TIME")], "20260601T090000Z"),
    ] {
        let t = parse_trigger(&trigger_prop(params, value)).expect("parses");
        let again = parse_trigger(&t.to_property()).expect("re-parses");
        assert_eq!(again.relative, t.relative, "{value}");
        assert_eq!(again.related, t.related, "{value}");
        assert_eq!(again.absolute, t.absolute, "{value}");
        assert_eq!(
            again.duration.to_string(),
            t.duration.to_string(),
            "{value}"
        );
    }
}

#[test]
fn an_absolute_trigger_resolves_without_consulting_its_parent() {
    let t = parse_trigger(&trigger_prop(Vec::new(), "20260601T090000Z")).expect("parses");
    let empty = Component::default();
    let cal = Calendar::default();
    assert_eq!(
        format_time(
            t.resolve(&empty, &cal)
                .expect("absolute triggers always resolve")
        ),
        "20260601T090000Z"
    );
}

#[test]
fn a_relative_trigger_without_its_anchor_reports_no_anchor() {
    // Reported rather than silently resolving against the zero instant,
    // which would place every such alarm at the epoch.
    let t = parse_trigger(&trigger_prop(Vec::new(), "-PT15M")).expect("parses");
    let err = t
        .resolve(&Component::default(), &Calendar::default())
        .expect_err("no DTSTART, no anchor");
    assert_eq!(err.sentinel(), "ErrNoAnchor");
}

#[test]
fn alarm_trigger_reports_no_trigger_when_the_property_is_absent() {
    // RFC 5545 §3.6.6 makes TRIGGER mandatory on VALARM, so this is a
    // producer bug rather than an absent optional.
    let mut alarm = Component::default();
    alarm.add(Property::new("ACTION", "DISPLAY"));
    let err = alarm_trigger(&alarm).expect_err("no TRIGGER");
    assert_eq!(err.sentinel(), "ErrNoTrigger");
}

// ---------------------------------------------------------------------
// event_end and alarm_repeat_cycle.
// ---------------------------------------------------------------------

/// A VEVENT carrying the given properties.
fn event(props: Vec<Property>) -> Component {
    let mut c = Component::default();
    for p in props {
        c.add(p);
    }
    c
}

#[test]
fn event_end_prefers_dtend_over_duration() {
    let c = event(vec![
        Property::new("DTSTART", "20260601T090000Z"),
        Property::new("DTEND", "20260601T170000Z"),
        Property::new("DURATION", "PT1H"),
    ]);
    let at = event_end(&c, &Calendar::default()).expect("DTEND wins");
    assert_eq!(format_time(at), "20260601T170000Z");
}

#[test]
fn event_end_falls_back_to_dtstart_plus_duration() {
    let c = event(vec![
        Property::new("DTSTART", "20260601T090000Z"),
        Property::new("DURATION", "PT90M"),
    ]);
    let at = event_end(&c, &Calendar::default()).expect("DURATION form");
    assert_eq!(format_time(at), "20260601T103000Z");
}

#[test]
fn event_end_reports_absence_rather_than_guessing() {
    assert!(event_end(&Component::default(), &Calendar::default()).is_none());
    // DURATION with no DTSTART.
    assert!(event_end(
        &event(vec![Property::new("DURATION", "PT1H")]),
        &Calendar::default()
    )
    .is_none());
    // A malformed DURATION.
    assert!(event_end(
        &event(vec![
            Property::new("DTSTART", "20260601T090000Z"),
            Property::new("DURATION", "nonsense"),
        ]),
        &Calendar::default()
    )
    .is_none());
}

#[test]
fn alarm_repeat_cycle_requires_the_pair_or_neither() {
    let (d, n) = alarm_repeat_cycle(&Component::default()).expect("neither is fine");
    assert_eq!(d.to_string(), "PT0S");
    assert_eq!(n, 0);

    let (d, n) = alarm_repeat_cycle(&event(vec![
        Property::new("DURATION", "PT5M"),
        Property::new("REPEAT", "3"),
    ]))
    .expect("the pair parses");
    assert_eq!(d.to_string(), "PT5M");
    assert_eq!(n, 3);

    for props in [
        vec![Property::new("DURATION", "PT5M")],
        vec![Property::new("REPEAT", "3")],
        vec![
            Property::new("DURATION", "nonsense"),
            Property::new("REPEAT", "3"),
        ],
        vec![
            Property::new("DURATION", "PT5M"),
            Property::new("REPEAT", "-1"),
        ],
        vec![
            Property::new("DURATION", "PT5M"),
            Property::new("REPEAT", "many"),
        ],
    ] {
        let err =
            alarm_repeat_cycle(&event(props)).expect_err("a half pair or a bad value is malformed");
        assert_eq!(err.sentinel(), "ErrMalformed");
    }
}
