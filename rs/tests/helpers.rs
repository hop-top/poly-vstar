// SPDX-License-Identifier: MIT

//! The `helpers` surface: constructors, accessors and mutators.
//!
//! # The emitter gate
//!
//! [`emitter_gate_rebuilds_rfc5545_one_vtodo`] and its vCard sibling
//! rebuild a committed conformance fixture through the helper API alone
//! and assert the canonical bytes and the hash match the corpus. That
//! gate is live but it is **blind to `X-VSTAR-HASH`**: rule 7 strips the
//! property before canonicalization, so no byte comparison can see a
//! constructor that forgot to stamp one. The unit test
//! [`every_constructor_stamps_a_verifying_hash`] is what covers that,
//! and [`emitter_gate_is_not_vacuous`] proves the byte gate itself is
//! live rather than passing on an empty comparison.

mod support;

use hop_top_vstar::canonical;
use hop_top_vstar::duration::{from_signed, Duration, Related};
use hop_top_vstar::hashing;
use hop_top_vstar::helpers::{
    add_category, add_related_to, alarm_fires_at, categories, class_of, class_or_default, complete,
    due, event_status, increment_sequence, journal_status, new_absolute_alarm, new_alarm,
    new_calendar, new_card, new_event, new_free_busy, new_journal, new_relative_alarm, new_todo,
    percent_complete, priority, related_to, remove_percent_complete, remove_priority, sequence,
    set_categories, set_class, set_due, set_event_status, set_journal_status, set_percent_complete,
    set_priority, set_sequence, set_todo_status, set_transp, todo_status, transp,
    transp_or_default, RelatedRef,
};
use hop_top_vstar::{
    parse_time, Calendar, Class, CompType, Component, EventStatus, JournalStatus, Kind, Property,
    RelType, TodoStatus, Transp,
};

fn at(s: &str) -> chrono::DateTime<chrono::Utc> {
    parse_time(s).unwrap_or_else(|| panic!("{s} is not a form #2 timestamp"))
}

/// A VTODO built by hand, hashed, ready for accessor tests.
fn todo(uid: &str) -> Component {
    let mut c = Component::new(CompType::TODO);
    c.set(Property::new("UID", uid));
    c.set(Property::new("DTSTAMP", "20260504T120000Z"));
    c
}

// ---------------------------------------------------------------- //
// Constructors                                                      //
// ---------------------------------------------------------------- //

/// Every component constructor stamps `DTSTAMP` and a **verifying**
/// `X-VSTAR-HASH`.
///
/// This is the test the emitter gate cannot replace: canonicalization
/// strips `X-VSTAR-HASH` (spec/03 rule 7), so a constructor that omits
/// it produces byte-identical canonical output and an identical hash.
/// Only reading the property back catches the omission.
#[test]
fn every_constructor_stamps_a_verifying_hash() {
    let start = at("20260601T090000Z");
    let end = at("20260601T100000Z");

    let built: Vec<(&str, Component)> = vec![
        ("new_todo", new_todo("t-1", end).expect("new_todo")),
        (
            "new_journal",
            new_journal("j-1", start).expect("new_journal"),
        ),
        (
            "new_event",
            new_event("e-1", start, end).expect("new_event"),
        ),
        (
            "new_free_busy",
            new_free_busy("f-1", start, end).expect("new_free_busy"),
        ),
        (
            "new_alarm",
            new_alarm("a-1", "DISPLAY", "-PT15M").expect("new_alarm"),
        ),
        (
            "new_relative_alarm",
            new_relative_alarm(
                "a-2",
                "DISPLAY",
                from_signed(chrono::Duration::minutes(-15)),
                Related::Start,
            )
            .expect("new_relative_alarm"),
        ),
        (
            "new_absolute_alarm",
            new_absolute_alarm("a-3", "AUDIO", start).expect("new_absolute_alarm"),
        ),
    ];

    for (name, c) in built {
        assert!(!c.dtstamp_raw().is_empty(), "{name} left DTSTAMP unset");
        assert!(
            c.dtstamp().is_some(),
            "{name} wrote an unparseable DTSTAMP {:?}",
            c.dtstamp_raw()
        );
        let stored =
            hashing::get_x_vstar(&c).unwrap_or_else(|| panic!("{name} left X-VSTAR-HASH unset"));
        assert!(
            stored.starts_with("sha256:"),
            "{name} stored {stored:?}, want a sha256: prefix"
        );
        let (ok, want, got) = hashing::verify_x_vstar(&c);
        assert!(
            ok,
            "{name}: stored hash does not verify — want {want}, got {got}"
        );
    }
}

/// Every fallible constructor refuses an empty UID with `ErrMissingUID`.
#[test]
fn constructors_refuse_an_empty_uid() {
    let t = at("20260601T090000Z");
    let cases: Vec<(&str, hop_top_vstar::Error)> = vec![
        ("new_todo", new_todo("", t).unwrap_err()),
        ("new_journal", new_journal("", t).unwrap_err()),
        ("new_event", new_event("", t, t).unwrap_err()),
        ("new_free_busy", new_free_busy("", t, t).unwrap_err()),
        ("new_alarm", new_alarm("", "DISPLAY", "-PT15M").unwrap_err()),
        (
            "new_relative_alarm",
            new_relative_alarm("", "DISPLAY", Duration::default(), Related::Start).unwrap_err(),
        ),
        (
            "new_absolute_alarm",
            new_absolute_alarm("", "AUDIO", t).unwrap_err(),
        ),
    ];
    for (name, err) in cases {
        assert_eq!(err.sentinel(), "ErrMissingUID", "{name}");
    }
}

/// Each constructor writes its type's own anchor properties.
#[test]
fn constructors_write_their_anchor_properties() {
    let start = at("20260601T090000Z");
    let end = at("20260601T100000Z");

    let t = new_todo("t-1", end).expect("new_todo");
    assert_eq!(t.r#type, CompType::TODO);
    assert_eq!(
        t.get("DUE").map(|p| p.value.as_str()),
        Some("20260601T100000Z")
    );

    let j = new_journal("j-1", start).expect("new_journal");
    assert_eq!(j.r#type, CompType::JOURNAL);
    assert_eq!(
        j.get("DTSTART").map(|p| p.value.as_str()),
        Some("20260601T090000Z")
    );

    let e = new_event("e-1", start, end).expect("new_event");
    assert_eq!(e.r#type, CompType::EVENT);
    assert_eq!(
        e.get("DTSTART").map(|p| p.value.as_str()),
        Some("20260601T090000Z")
    );
    assert_eq!(
        e.get("DTEND").map(|p| p.value.as_str()),
        Some("20260601T100000Z")
    );

    let f = new_free_busy("f-1", start, end).expect("new_free_busy");
    assert_eq!(f.r#type, CompType::FREE_BUSY);
    assert_eq!(
        f.get("DTSTART").map(|p| p.value.as_str()),
        Some("20260601T090000Z")
    );

    let a = new_alarm("a-1", "DISPLAY", "-PT15M").expect("new_alarm");
    assert_eq!(a.r#type, CompType::ALARM);
    assert_eq!(a.get("ACTION").map(|p| p.value.as_str()), Some("DISPLAY"));
    assert_eq!(a.get("TRIGGER").map(|p| p.value.as_str()), Some("-PT15M"));
}

/// `new_calendar` cannot fail, and an empty PRODID resolves to the
/// package default.
#[test]
fn new_calendar_defaults_its_prodid() {
    assert_eq!(new_calendar("-//Example//EN").prod_id, "-//Example//EN");
    // The default is version-free and language-free: PRODID is part of
    // the hashed canonical form, so it must not change across releases
    // or differ between ports.
    let defaulted = new_calendar("");
    assert_eq!(defaulted.prod_id, "-//hop-top//vstar//EN");
    assert!(defaulted.components.is_empty());
}

/// `new_card` defaults an absent kind to `individual` and writes a
/// `KIND` property — which is why the vCard emitter gate has to clear
/// it: `rfc6350/minimal` carries no KIND.
#[test]
fn new_card_defaults_kind_to_individual_and_writes_the_property() {
    let defaulted = new_card("urn:uuid:1", None);
    assert_eq!(defaulted.kind, Some(Kind::Individual));
    assert_eq!(
        defaulted.get("KIND").map(|p| p.value.as_str()),
        Some("individual"),
        "new_card writes a KIND property"
    );
    assert_eq!(
        defaulted.get("VERSION").map(|p| p.value.as_str()),
        Some("4.0")
    );
    assert_eq!(
        defaulted.get("UID").map(|p| p.value.as_str()),
        Some("urn:uuid:1")
    );

    let explicit = new_card("urn:uuid:2", Some(Kind::Org));
    assert_eq!(explicit.kind, Some(Kind::Org));
    assert_eq!(explicit.get("KIND").map(|p| p.value.as_str()), Some("org"));

    // Cards carry no X-VSTAR-HASH at the constructor layer.
    assert!(defaulted.get("X-VSTAR-HASH").is_none());
}

// ---------------------------------------------------------------- //
// The emitter gate                                                  //
// ---------------------------------------------------------------- //

/// Rebuilds `spec/v1.0/conformance/rfc5545/one_vtodo` through the
/// helper API and asserts the canonical bytes and the hash match the
/// committed fixture exactly.
#[test]
fn emitter_gate_rebuilds_rfc5545_one_vtodo() {
    let mut cal = new_calendar("-//V*//OneVTODO//EN");

    let mut c = new_todo("abc-123", at("20260504T120000Z")).expect("new_todo");
    // The fixture carries no DUE; the constructor's DTSTAMP is "now",
    // so both are replaced with the fixture's own values. Everything
    // else — the type, the UID, the property-set discipline — comes
    // from the helper layer.
    c.remove("DUE");
    c.set(Property::new("DTSTAMP", "20260504T120000Z"));
    c.set(Property::new("SUMMARY", "Buy milk"));
    set_priority(&mut c, 3);
    cal.append(c);

    let want_canonical = support::crlf_to_lf(
        &std::fs::read(
            support::corpus_root()
                .join("rfc5545")
                .join("one_vtodo.canonical"),
        )
        .expect("read one_vtodo.canonical"),
    );
    let got = support::crlf_to_lf(&canonical::calendar(&cal));
    support::assert_bytes_eq(&got, &want_canonical, "one_vtodo canonical");

    let want_hash = std::fs::read_to_string(
        support::corpus_root()
            .join("rfc5545")
            .join("one_vtodo.hash"),
    )
    .expect("read one_vtodo.hash");
    assert_eq!(hashing::calendar(&cal), want_hash.trim(), "one_vtodo hash");
}

/// Rebuilds `spec/v1.0/conformance/rfc6350/minimal` through the helper
/// API. `new_card` writes a `KIND` the fixture does not carry, so the
/// gate clears it — which is exactly the divergence worth pinning.
#[test]
fn emitter_gate_rebuilds_rfc6350_minimal() {
    let mut card = new_card("urn:uuid:11111111-1111-1111-1111-111111111111", None);
    // `rfc6350/minimal.vcf` has no KIND: clear both the field and the
    // property the constructor stamped.
    card.kind = None;
    card.remove("KIND");
    card.remove("VERSION");
    card.remove("UID");
    card.set(Property::new("FN", "Jad Bitar"));

    let want_canonical = support::crlf_to_lf(
        &std::fs::read(
            support::corpus_root()
                .join("rfc6350")
                .join("minimal.canonical"),
        )
        .expect("read minimal.canonical"),
    );
    let got = support::crlf_to_lf(&canonical::card(&card));
    support::assert_bytes_eq(&got, &want_canonical, "rfc6350 minimal canonical");

    let want_hash =
        std::fs::read_to_string(support::corpus_root().join("rfc6350").join("minimal.hash"))
            .expect("read minimal.hash");
    assert_eq!(hashing::card(&card), want_hash.trim(), "minimal hash");
}

/// The control for the emitter gate: a wrong accessor value must move
/// the canonical bytes. If this passes while the gate above also
/// passes, the gate is live rather than comparing nothing.
#[test]
fn emitter_gate_is_not_vacuous() {
    let mut cal = new_calendar("-//V*//OneVTODO//EN");
    let mut c = new_todo("abc-123", at("20260504T120000Z")).expect("new_todo");
    c.remove("DUE");
    c.set(Property::new("DTSTAMP", "20260504T120000Z"));
    c.set(Property::new("SUMMARY", "Buy milk"));
    set_priority(&mut c, 4); // the fixture says 3.
    cal.append(c);

    let want_canonical = support::crlf_to_lf(
        &std::fs::read(
            support::corpus_root()
                .join("rfc5545")
                .join("one_vtodo.canonical"),
        )
        .expect("read one_vtodo.canonical"),
    );
    assert_ne!(
        support::crlf_to_lf(&canonical::calendar(&cal)),
        want_canonical,
        "PRIORITY:4 must not canonicalize to the PRIORITY:3 fixture — \
         the emitter gate would be vacuous"
    );
}

// ---------------------------------------------------------------- //
// Categories                                                        //
// ---------------------------------------------------------------- //

#[test]
fn categories_split_trim_and_drop_empties() {
    let mut c = todo("todo-cat");
    assert!(categories(&c).is_empty(), "absent CATEGORIES is empty");

    c.set(Property::new("CATEGORIES", " work , ,urgent, "));
    assert_eq!(categories(&c), vec!["work", "urgent"]);

    c.set(Property::new("CATEGORIES", ",,"));
    assert!(categories(&c).is_empty(), "only empty tokens is empty");
}

#[test]
fn set_categories_dedupes_preserving_first_seen_order() {
    let mut c = todo("todo-set");
    set_categories(
        &mut c,
        &[
            "work".into(),
            " urgent ".into(),
            "work".into(),
            "".into(),
            "Work".into(),
        ],
    );
    assert_eq!(
        c.get("CATEGORIES").map(|p| p.value.as_str()),
        Some("work,urgent,Work"),
        "case-sensitive dedupe, comma-joined with no space"
    );
    let (ok, ..) = hashing::verify_x_vstar(&c);
    assert!(ok, "set_categories refreshes the hash last");

    // Empty input removes the property.
    set_categories(&mut c, &[]);
    assert!(c.get("CATEGORIES").is_none());
}

#[test]
fn add_category_is_idempotent_and_case_sensitive() {
    let mut c = todo("todo-add");
    add_category(&mut c, "work");
    add_category(&mut c, "work");
    add_category(&mut c, "Work");
    add_category(&mut c, "");
    assert_eq!(
        c.get("CATEGORIES").map(|p| p.value.as_str()),
        Some("work,Work")
    );
}

// ---------------------------------------------------------------- //
// Relations                                                         //
// ---------------------------------------------------------------- //

#[test]
fn related_to_defaults_an_absent_reltype_to_parent() {
    let mut c = todo("todo-rel");
    assert!(related_to(&c).is_empty());

    c.add(Property::new("RELATED-TO", "parent-uid"));
    c.add(Property::new("RELATED-TO", "child-uid").with_param("reltype", "child"));
    c.add(Property::new("RELATED-TO", "ext-uid").with_param("RELTYPE", "X-CUSTOM"));

    assert_eq!(
        related_to(&c),
        vec![
            RelatedRef {
                uid: "parent-uid".into(),
                rel_type: RelType::parent(),
            },
            RelatedRef {
                uid: "child-uid".into(),
                rel_type: RelType::child(),
            },
            RelatedRef {
                uid: "ext-uid".into(),
                rel_type: RelType::new("X-CUSTOM"),
            },
        ],
        "RELTYPE folds case to its canonical spelling; unregistered values pass through"
    );
}

#[test]
fn add_related_to_appends_and_omits_an_empty_reltype() {
    let mut c = todo("todo-addrel");
    add_related_to(&mut c, "a", RelType::child());
    add_related_to(&mut c, "b", RelType::new(""));
    add_related_to(&mut c, "", RelType::child()); // no-op: empty uid.

    let props = c.get_all("RELATED-TO");
    assert_eq!(props.len(), 2, "appends rather than replacing");
    assert_eq!(props[0].value, "a");
    assert_eq!(
        props[0].param("RELTYPE").map(|p| p.value.as_str()),
        Some("CHILD")
    );
    assert_eq!(props[1].value, "b");
    assert!(
        props[1].param("RELTYPE").is_none(),
        "an empty reltype omits the parameter"
    );

    let (ok, ..) = hashing::verify_x_vstar(&c);
    assert!(ok, "add_related_to refreshes the hash last");
}

// ---------------------------------------------------------------- //
// Due                                                               //
// ---------------------------------------------------------------- //

#[test]
fn due_reads_and_set_due_writes_form_two() {
    let cal = Calendar::default();
    let mut c = todo("todo-due");
    assert_eq!(due(&c, &cal), None);

    set_due(&mut c, at("20260101T000000Z"));
    assert_eq!(
        c.get("DUE").map(|p| p.value.as_str()),
        Some("20260101T000000Z")
    );
    assert_eq!(due(&c, &cal), Some(at("20260101T000000Z")));

    let (ok, ..) = hashing::verify_x_vstar(&c);
    assert!(ok, "set_due refreshes the hash last");
}

// ---------------------------------------------------------------- //
// Status                                                            //
// ---------------------------------------------------------------- //

#[test]
fn todo_status_round_trips_and_rejects_foreign_vocabularies() {
    let mut c = todo("todo-status");
    assert_eq!(todo_status(&c), None);

    set_todo_status(&mut c, TodoStatus::InProcess);
    assert_eq!(todo_status(&c), Some(TodoStatus::InProcess));
    assert_eq!(
        c.get("STATUS").map(|p| p.value.as_str()),
        Some("IN-PROCESS")
    );

    // A VEVENT value on a VTODO reads back as absent.
    c.set(Property::new("STATUS", "TENTATIVE"));
    assert_eq!(todo_status(&c), None, "TENTATIVE is not a TodoStatus");
    assert_eq!(
        event_status(&c),
        Some(EventStatus::Tentative),
        "the accessors read the wire, not the component type"
    );
}

#[test]
fn status_setters_gate_on_component_type() {
    let mut t = todo("todo-gate");
    set_event_status(&mut t, EventStatus::Confirmed);
    assert!(
        t.get("STATUS").is_none(),
        "SetEventStatus is a no-op on a VTODO"
    );

    let mut e = Component::new(CompType::EVENT);
    e.set(Property::new("UID", "evt-gate"));
    set_todo_status(&mut e, TodoStatus::Completed);
    assert!(
        e.get("STATUS").is_none(),
        "set_todo_status is a no-op on a VEVENT"
    );
    set_event_status(&mut e, EventStatus::Confirmed);
    assert_eq!(event_status(&e), Some(EventStatus::Confirmed));

    let mut j = Component::new(CompType::JOURNAL);
    j.set(Property::new("UID", "jnl-gate"));
    set_journal_status(&mut j, JournalStatus::Final);
    assert_eq!(journal_status(&j), Some(JournalStatus::Final));
    set_event_status(&mut j, EventStatus::Cancelled);
    assert_eq!(
        journal_status(&j),
        Some(JournalStatus::Final),
        "a foreign setter leaves the component untouched"
    );
}

#[test]
fn complete_writes_the_full_done_marker_set() {
    let mut c = todo("todo-complete");
    complete(&mut c, at("20260601T120000Z"));

    assert_eq!(todo_status(&c), Some(TodoStatus::Completed));
    assert_eq!(
        c.get("COMPLETED").map(|p| p.value.as_str()),
        Some("20260601T120000Z")
    );
    assert_eq!(percent_complete(&c), Some(100));
    let (ok, want, got) = hashing::verify_x_vstar(&c);
    assert!(
        ok,
        "complete refreshes the hash last — want {want}, got {got}"
    );

    // No-op on a non-VTODO.
    let mut e = Component::new(CompType::EVENT);
    e.set(Property::new("UID", "evt-complete"));
    complete(&mut e, at("20260601T120000Z"));
    assert!(e.get("STATUS").is_none());
}

// ---------------------------------------------------------------- //
// Integer-valued properties                                         //
// ---------------------------------------------------------------- //

#[test]
fn sequence_defaults_to_zero_only_via_increment() {
    let mut c = todo("todo-seq");
    assert_eq!(sequence(&c), None, "absence is reported, not defaulted");

    increment_sequence(&mut c);
    assert_eq!(sequence(&c), Some(1), "an absent SEQUENCE is revision 0");
    increment_sequence(&mut c);
    assert_eq!(sequence(&c), Some(2));

    set_sequence(&mut c, 7);
    assert_eq!(sequence(&c), Some(7));
    set_sequence(&mut c, -1);
    assert_eq!(
        sequence(&c),
        Some(7),
        "a negative revision is rejected, not clamped"
    );
}

#[test]
fn a_sloppy_integer_is_unparseable_not_coerced() {
    let mut c = todo("todo-sloppy");
    for raw in ["+3", "03", " 3", "3 ", "three", ""] {
        c.set(Property::new("SEQUENCE", raw));
        assert_eq!(sequence(&c), None, "SEQUENCE:{raw:?} must not coerce to 3");
    }
    c.set(Property::new("SEQUENCE", "3"));
    assert_eq!(sequence(&c), Some(3));
}

#[test]
fn priority_distinguishes_explicit_zero_from_absence() {
    let mut c = todo("todo-prio");
    assert_eq!(priority(&c), None);

    set_priority(&mut c, 0);
    assert_eq!(priority(&c), Some(0), "PRIORITY:0 is the RFC's 'undefined'");
    assert_eq!(c.get("PRIORITY").map(|p| p.value.as_str()), Some("0"));

    set_priority(&mut c, 10);
    assert_eq!(
        priority(&c),
        Some(0),
        "out of range is rejected, not clamped"
    );

    remove_priority(&mut c);
    assert_eq!(priority(&c), None);
    let (ok, ..) = hashing::verify_x_vstar(&c);
    assert!(ok, "remove_priority refreshes the hash last");
}

#[test]
fn percent_complete_is_vtodo_scoped_and_range_checked() {
    let mut c = todo("todo-pct");
    assert_eq!(percent_complete(&c), None);

    set_percent_complete(&mut c, 0);
    assert_eq!(percent_complete(&c), Some(0));
    set_percent_complete(&mut c, 100);
    assert_eq!(percent_complete(&c), Some(100));
    set_percent_complete(&mut c, 120);
    assert_eq!(percent_complete(&c), Some(100), "120 must not clamp to 100");

    remove_percent_complete(&mut c);
    assert_eq!(percent_complete(&c), None);

    let mut e = Component::new(CompType::EVENT);
    e.set(Property::new("UID", "evt-pct"));
    set_percent_complete(&mut e, 50);
    assert!(e.get("PERCENT-COMPLETE").is_none(), "VTODO-scoped");
}

// ---------------------------------------------------------------- //
// Classification and transparency                                   //
// ---------------------------------------------------------------- //

#[test]
fn class_reports_absence_and_or_default_applies_the_rfc_default() {
    let mut c = todo("todo-class");
    assert_eq!(
        class_of(&c),
        None,
        "the plain getter reports absence faithfully"
    );
    assert_eq!(class_or_default(&c), Class::Public);

    set_class(&mut c, Class::Confidential);
    assert_eq!(class_of(&c), Some(Class::Confidential));
    assert_eq!(class_or_default(&c), Class::Confidential);

    c.set(Property::new("CLASS", "NONSENSE"));
    assert_eq!(class_of(&c), None, "an unrecognized value is absence");
    assert_eq!(
        class_or_default(&c),
        Class::Public,
        "and falls back to the default"
    );
}

#[test]
fn transp_is_vevent_scoped() {
    let mut e = Component::new(CompType::EVENT);
    e.set(Property::new("UID", "evt-transp"));
    assert_eq!(transp(&e), None);
    assert_eq!(transp_or_default(&e), Transp::Opaque);

    set_transp(&mut e, Transp::Transparent);
    assert_eq!(transp(&e), Some(Transp::Transparent));

    let mut t = todo("todo-transp");
    set_transp(&mut t, Transp::Transparent);
    assert!(t.get("TRANSP").is_none(), "TRANSP is VEVENT-only");
}

#[test]
fn class_applies_to_event_todo_and_journal_only() {
    for ct in [CompType::EVENT, CompType::TODO, CompType::JOURNAL] {
        let mut c = Component::new(ct.clone());
        c.set(Property::new("UID", "uid"));
        set_class(&mut c, Class::Private);
        assert_eq!(class_of(&c), Some(Class::Private), "{ct:?} admits CLASS");
    }
    let mut a = Component::new(CompType::ALARM);
    a.set(Property::new("UID", "alarm"));
    set_class(&mut a, Class::Private);
    assert!(a.get("CLASS").is_none(), "VALARM does not admit CLASS");
}

// ---------------------------------------------------------------- //
// Alarms                                                            //
// ---------------------------------------------------------------- //

#[test]
fn relative_alarm_leaves_the_related_start_default_implicit() {
    let implicit = new_relative_alarm(
        "a-implicit",
        "DISPLAY",
        from_signed(chrono::Duration::minutes(-15)),
        Related::Start,
    )
    .expect("new_relative_alarm");
    let trigger = implicit.get("TRIGGER").expect("TRIGGER");
    assert_eq!(trigger.value, "-PT15M");
    assert!(
        trigger.param("RELATED").is_none(),
        "RELATED=START is the RFC default and stays implicit"
    );

    let explicit = new_relative_alarm(
        "a-end",
        "DISPLAY",
        from_signed(chrono::Duration::minutes(-15)),
        Related::End,
    )
    .expect("new_relative_alarm");
    assert_eq!(
        explicit
            .get("TRIGGER")
            .and_then(|p| p.param("RELATED"))
            .map(|p| p.value.as_str()),
        Some("END")
    );
}

#[test]
fn absolute_alarm_tags_value_date_time_explicitly() {
    let a =
        new_absolute_alarm("a-abs", "AUDIO", at("20260601T084500Z")).expect("new_absolute_alarm");
    let trigger = a.get("TRIGGER").expect("TRIGGER");
    assert_eq!(trigger.value, "20260601T084500Z");
    assert_eq!(
        trigger.param("VALUE").map(|p| p.value.as_str()),
        Some("DATE-TIME"),
        "the form is stated, never inferred"
    );
}

#[test]
fn alarm_fires_at_resolves_against_its_parent() {
    let cal = Calendar::default();
    let parent =
        new_event("evt-fire", at("20260601T090000Z"), at("20260601T100000Z")).expect("new_event");

    let relative = new_relative_alarm(
        "a-rel",
        "DISPLAY",
        from_signed(chrono::Duration::minutes(-15)),
        Related::Start,
    )
    .expect("new_relative_alarm");
    assert_eq!(
        alarm_fires_at(&relative, &parent, &cal).expect("relative resolves"),
        at("20260601T084500Z")
    );

    let end_relative = new_relative_alarm(
        "a-end",
        "DISPLAY",
        from_signed(chrono::Duration::minutes(30)),
        Related::End,
    )
    .expect("new_relative_alarm");
    assert_eq!(
        alarm_fires_at(&end_relative, &parent, &cal).expect("RELATED=END resolves"),
        at("20260601T103000Z")
    );

    let absolute =
        new_absolute_alarm("a-abs", "AUDIO", at("20260601T120000Z")).expect("new_absolute_alarm");
    assert_eq!(
        alarm_fires_at(&absolute, &parent, &cal).expect("absolute resolves"),
        at("20260601T120000Z")
    );
}

#[test]
fn alarm_fires_at_surfaces_the_trigger_failure_classes() {
    let cal = Calendar::default();
    let parent =
        new_event("evt-err", at("20260601T090000Z"), at("20260601T100000Z")).expect("new_event");

    let mut no_trigger = Component::new(CompType::ALARM);
    no_trigger.set(Property::new("UID", "a-none"));
    no_trigger.set(Property::new("ACTION", "DISPLAY"));
    assert_eq!(
        alarm_fires_at(&no_trigger, &parent, &cal)
            .unwrap_err()
            .sentinel(),
        "ErrNoTrigger"
    );

    let anchorless = Component::new(CompType::EVENT);
    let relative = new_relative_alarm(
        "a-rel",
        "DISPLAY",
        from_signed(chrono::Duration::minutes(-15)),
        Related::Start,
    )
    .expect("new_relative_alarm");
    assert_eq!(
        alarm_fires_at(&relative, &anchorless, &cal)
            .unwrap_err()
            .sentinel(),
        "ErrNoAnchor"
    );
}
