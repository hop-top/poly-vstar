// SPDX-License-Identifier: MIT

//! The `rrule/` conformance gate: every fixture under
//! `spec/v0.1/conformance/rrule/`, plus the hand-written assertions the
//! corpus cannot express.
//!
//! The walk is name-free. Each `.rrule` and each `.ics` is discovered on
//! disk and its sidecars decide what is asserted, exactly as
//! `go/cmd/fixtures-verify` does it:
//!
//! - `.expect.json` — the named sentinel, and nothing else is checked.
//! - `.formatted`   — the re-emitted wire form, plus idempotence.
//! - `.next.json`   — `next_occurrence` stepped `expected.len()` times,
//!   then one step past the end.
//! - `.expand.json` / `.between.json` — bounded expansion and window.
//! - `.occurrences.json` — bounded expansion of a recurrence set.
//! - A fixture with no sidecar pins only "this parses".
//!
//! # The step past the end
//!
//! After the last expected occurrence, the sidecar decides the
//! assertion three ways, and only three:
//!
//! 1. the sidecar names an `error` → that sentinel;
//! 2. the rule carries `COUNT` or `UNTIL` → `Ok(None)`, the rule ended;
//! 3. otherwise → still `Some`, because the rule is unbounded.
//!
//! Asserting `None` unconditionally is wrong for case 3 and would pass
//! only because the bounded fixtures outnumber the unbounded ones.

mod support;

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use chrono::{DateTime, Utc};
use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::rrule::{
    all, between, format_date_time_list, next_occurrence, occurrences, parse_date_time_list,
    parse_recurrence_id, parse_rrule, validate_rrule, ByDay, Freq, Range, RecurrenceId, Rule,
    RuleSet, Weekday, MAX_ITERATIONS,
};
use hop_top_vstar::{parse_time, Error, Param, Property};
use support::json::{parse as parse_json, Json};
use support::{cases_under, corpus_root};

/// Root of the `rrule/` fixture tree.
fn rrule_root() -> PathBuf {
    corpus_root().join("rrule")
}

/// Reads a sidecar next to `stem`, or `None` when it does not exist.
fn sidecar(stem: &Path, suffix: &str) -> Option<Json> {
    let path = PathBuf::from(format!("{}{suffix}", stem.display()));
    let text = std::fs::read_to_string(&path).ok()?;
    Some(parse_json(&text).unwrap_or_else(|e| panic!("parse {}: {e}", path.display())))
}

/// Reads a non-JSON sidecar as trimmed text.
fn text_sidecar(stem: &Path, suffix: &str) -> Option<String> {
    let path = PathBuf::from(format!("{}{suffix}", stem.display()));
    let raw = std::fs::read_to_string(path).ok()?;
    Some(raw.trim_end_matches(['\r', '\n']).to_owned())
}

/// The stem of a fixture path: the full path with its extension removed,
/// so `.next.json` and friends resolve as siblings.
fn stem_of(path: &Path) -> PathBuf {
    path.with_extension("")
}

/// A sidecar instant, parsed with the crate's own `parse_time` so the
/// fixture exercises the same code path a document does.
fn instant(path: &Path, field: &str, value: &str) -> DateTime<Utc> {
    parse_time(value).unwrap_or_else(|| panic!("{}: bad {field} {value:?}", path.display()))
}

/// The `expected[]` list of an outcome sidecar, as instants.
fn expected_list(path: &Path, spec: &Json) -> Vec<DateTime<Utc>> {
    let Some(raw) = spec.get("expected") else {
        return Vec::new();
    };
    raw.array()
        .iter()
        .enumerate()
        .map(|(i, v)| match v {
            Json::String(s) => instant(path, &format!("expected[{i}]"), s),
            other => panic!(
                "{}: expected[{i}] is not a string: {other:?}",
                path.display()
            ),
        })
        .collect()
}

/// Asserts a failure carries the sentinel the sidecar names.
fn assert_sentinel(label: &str, want: &str, got: Result<(), Error>) {
    match got {
        Ok(()) => panic!("{label}: expected {want}, the call succeeded"),
        Err(e) => assert_eq!(
            e.sentinel(),
            want,
            "{label}: expected {want}, got {} ({e})",
            e.sentinel()
        ),
    }
}

/// Renders instants for a failure message.
fn render(ts: &[DateTime<Utc>]) -> Vec<String> {
    ts.iter().map(|t| hop_top_vstar::format_time(*t)).collect()
}

// ── The corpus walk ───────────────────────────────────────────────────

#[test]
fn every_rrule_fixture_meets_its_sidecar_contract() {
    let cases = cases_under(&rrule_root(), "rrule");
    let mut seen = 0usize;
    for case in &cases {
        let stem = stem_of(&case.path);
        let value = String::from_utf8(case.input.clone())
            .unwrap_or_else(|e| panic!("{}: not UTF-8: {e}", case.path.display()));
        let value = value.trim_end_matches(['\r', '\n']);
        let label = case.path.display().to_string();

        // A sentinel sidecar short-circuits: the rule must be rejected,
        // and nothing downstream of a rejection is asserted.
        if let Some(expect) = sidecar(&stem, ".expect.json") {
            let want = expect
                .str_field("sentinel")
                .unwrap_or_else(|| panic!("{label}: .expect.json has no sentinel"));
            assert_sentinel(&label, want, validate_rrule(value));
            seen += 1;
            continue;
        }

        let rule =
            parse_rrule(value).unwrap_or_else(|e| panic!("{label}: parse_rrule failed: {e}"));

        check_formatted(&stem, &label, &rule);
        check_next(&stem, &label, &rule);
        check_expand(&stem, &label, &rule);
        check_between(&stem, &label, &rule);
        seen += 1;
    }
    assert_eq!(seen, cases.len(), "every discovered fixture was visited");
}

/// `<stem>.formatted` — the wire form, plus parse∘format idempotence.
fn check_formatted(stem: &Path, label: &str, rule: &Rule) {
    let Some(want) = text_sidecar(stem, ".formatted") else {
        return;
    };
    let got = rule.to_string();
    assert_eq!(got, want, "{label}: wire form diverges");

    // Idempotence: re-parsing the emitted form and re-emitting it is a
    // fixpoint. A formatter that drops a rule-part round-trips its own
    // output only if the loss is total, which this catches.
    let reparsed = parse_rrule(&got)
        .unwrap_or_else(|e| panic!("{label}: the emitted form does not re-parse: {e}"));
    assert_eq!(
        reparsed.to_string(),
        got,
        "{label}: parse∘format is not idempotent"
    );
    assert_eq!(reparsed, *rule, "{label}: parse∘format lost a rule-part");

    // `to_property` carries the same value under the RRULE name.
    let prop = rule.to_property();
    assert_eq!(prop.name, "RRULE", "{label}: to_property name");
    assert_eq!(prop.value, want, "{label}: to_property value");
}

/// `<stem>.next.json` — step `expected.len()` times, then once past the
/// end.
fn check_next(stem: &Path, label: &str, rule: &Rule) {
    let Some(spec) = sidecar(stem, ".next.json") else {
        return;
    };
    let path = PathBuf::from(format!("{}.next.json", stem.display()));
    let dtstart = instant(
        &path,
        "dtstart",
        spec.str_field("dtstart")
            .unwrap_or_else(|| panic!("{label}: .next.json has no dtstart")),
    );
    let mut after = instant(
        &path,
        "after",
        spec.str_field("after")
            .unwrap_or_else(|| panic!("{label}: .next.json has no after")),
    );
    let expected = expected_list(&path, &spec);

    for (i, want) in expected.iter().enumerate() {
        let got = next_occurrence(rule, dtstart, after)
            .unwrap_or_else(|e| panic!("{label}: next_occurrence step {i} failed: {e}"));
        let got = got.unwrap_or_else(|| {
            panic!(
                "{label}: next_occurrence step {i} returned Ok(None), want {}",
                hop_top_vstar::format_time(*want)
            )
        });
        assert_eq!(
            got,
            *want,
            "{label}: next_occurrence step {i}: got {}, want {}",
            hop_top_vstar::format_time(got),
            hop_top_vstar::format_time(*want)
        );
        after = got;
    }

    // The step past the end. Three outcomes, chosen by the sidecar and
    // the rule — never "always None".
    let past = next_occurrence(rule, dtstart, after);
    match spec.str_field("error") {
        Some(want) => assert_sentinel(
            &format!("{label}: the step after {} occurrences", expected.len()),
            want,
            past.map(|_| ()),
        ),
        None if rule.count.is_some() || rule.until.is_some() => {
            let got = past
                .unwrap_or_else(|e| panic!("{label}: the step past a bounded rule failed: {e}"));
            assert!(
                got.is_none(),
                "{label}: a rule with COUNT/UNTIL must report termination, got {:?}",
                got.map(hop_top_vstar::format_time)
            );
        }
        None => {
            let got = past
                .unwrap_or_else(|e| panic!("{label}: the step past an unbounded rule failed: {e}"));
            assert!(
                got.is_some(),
                "{label}: an unbounded rule must keep yielding past the sidecar's list"
            );
        }
    }
}

/// `<stem>.expand.json` — bounded expansion and the complete flag.
fn check_expand(stem: &Path, label: &str, rule: &Rule) {
    let Some(spec) = sidecar(stem, ".expand.json") else {
        return;
    };
    let path = PathBuf::from(format!("{}.expand.json", stem.display()));
    let dtstart = instant(
        &path,
        "dtstart",
        spec.str_field("dtstart")
            .unwrap_or_else(|| panic!("{label}: .expand.json has no dtstart")),
    );
    let limit = usize::try_from(
        spec.i64_field("limit")
            .unwrap_or_else(|| panic!("{label}: .expand.json has no limit")),
    )
    .expect("limit is non-negative");

    let got = occurrences(rule, dtstart, limit);
    if let Some(want) = spec.str_field("error") {
        assert_sentinel(label, want, got.map(|_| ()));
        return;
    }
    let (times, complete) = got.unwrap_or_else(|e| panic!("{label}: occurrences failed: {e}"));
    assert_eq!(
        render(&times),
        render(&expected_list(&path, &spec)),
        "{label}: occurrences"
    );
    assert_eq!(
        complete,
        spec.bool_field("complete")
            .unwrap_or_else(|| panic!("{label}: .expand.json has no complete")),
        "{label}: complete flag"
    );
}

/// `<stem>.between.json` — the half-open window.
fn check_between(stem: &Path, label: &str, rule: &Rule) {
    let Some(spec) = sidecar(stem, ".between.json") else {
        return;
    };
    let path = PathBuf::from(format!("{}.between.json", stem.display()));
    let field = |name: &str| {
        instant(
            &path,
            name,
            spec.str_field(name)
                .unwrap_or_else(|| panic!("{label}: .between.json has no {name}")),
        )
    };
    let (dtstart, start, end) = (field("dtstart"), field("start"), field("end"));

    let got = between(rule, dtstart, start, end);
    if let Some(want) = spec.str_field("error") {
        assert_sentinel(label, want, got.map(|_| ()));
        return;
    }
    let times = got.unwrap_or_else(|e| panic!("{label}: between failed: {e}"));
    assert_eq!(
        render(&times),
        render(&expected_list(&path, &spec)),
        "{label}: between"
    );
}

#[test]
fn every_recurrence_set_fixture_meets_its_sidecar_contract() {
    let cases = cases_under(&rrule_root(), "ics");
    for case in &cases {
        let stem = stem_of(&case.path);
        let label = case.path.display().to_string();
        let built = rfc5545::parse(case.input.as_slice()).and_then(|cal| {
            let first = cal
                .components
                .first()
                .unwrap_or_else(|| panic!("{label}: calendar has no components"))
                .clone();
            RuleSet::from_component(&first)
        });

        if let Some(expect) = sidecar(&stem, ".expect.json") {
            let want = expect
                .str_field("sentinel")
                .unwrap_or_else(|| panic!("{label}: .expect.json has no sentinel"));
            assert_sentinel(&label, want, built.map(|_| ()));
            continue;
        }
        let set = built.unwrap_or_else(|e| panic!("{label}: RuleSet::from_component failed: {e}"));

        let Some(spec) = sidecar(&stem, ".occurrences.json") else {
            continue;
        };
        let path = PathBuf::from(format!("{}.occurrences.json", stem.display()));
        let limit = usize::try_from(
            spec.i64_field("limit")
                .unwrap_or_else(|| panic!("{label}: .occurrences.json has no limit")),
        )
        .expect("limit is non-negative");

        let got = set.occurrences(limit);
        if let Some(want) = spec.str_field("error") {
            assert_sentinel(&label, want, got.map(|_| ()));
            continue;
        }
        let (times, complete) =
            got.unwrap_or_else(|e| panic!("{label}: set occurrences failed: {e}"));
        assert_eq!(
            render(&times),
            render(&expected_list(&path, &spec)),
            "{label}: set occurrences"
        );
        assert_eq!(
            complete,
            spec.bool_field("complete")
                .unwrap_or_else(|| panic!("{label}: .occurrences.json has no complete")),
            "{label}: set complete flag"
        );
    }
}

/// The corpus is a moving target, so the counts are asserted per
/// subdirectory rather than in total: a fixture class that silently
/// empties turns its gate into a no-op, and a count is the cheapest
/// detector.
#[test]
fn the_corpus_carries_the_fixture_classes_the_gates_assume() {
    let mut by_dir: BTreeMap<String, (usize, usize)> = BTreeMap::new();
    for ext in ["rrule", "ics"] {
        for case in cases_under(&rrule_root(), ext) {
            let dir = case
                .path
                .parent()
                .expect("fixture has a parent")
                .strip_prefix(rrule_root())
                .expect("fixture sits under the rrule root")
                .display()
                .to_string();
            let entry = by_dir.entry(dir).or_default();
            entry.0 += 1;
            if sidecar(&stem_of(&case.path), ".expect.json").is_some() {
                entry.1 += 1;
            }
        }
    }

    for dir in [
        "happy",
        "bounds",
        "by-clauses",
        "evaluator",
        "expansion",
        "format",
        "rejected",
        "set",
        "set/rejected",
    ] {
        let (total, _) = by_dir
            .get(dir)
            .copied()
            .unwrap_or_else(|| panic!("no fixtures under rrule/{dir}"));
        assert!(total > 0, "rrule/{dir} is empty");
    }

    // Every fixture in a `rejected/` directory names a sentinel, and no
    // fixture outside one does — the two halves of the corpus must not
    // blur.
    for (dir, (total, rejected)) in &by_dir {
        if dir.ends_with("rejected") {
            assert_eq!(
                total, rejected,
                "rrule/{dir}: every fixture must carry .expect.json"
            );
        } else {
            assert_eq!(
                *rejected, 0,
                "rrule/{dir}: no fixture may carry .expect.json"
            );
        }
    }
}

/// The 18 sidecar-less fixtures pin exactly one thing: the rule parses.
/// Asserting the count keeps that class from quietly emptying.
#[test]
fn the_sidecar_free_fixtures_pin_only_that_they_parse() {
    let mut bare = 0usize;
    for case in cases_under(&rrule_root(), "rrule") {
        let stem = stem_of(&case.path);
        let has_sidecar = [
            ".expect.json",
            ".next.json",
            ".expand.json",
            ".between.json",
        ]
        .iter()
        .any(|s| sidecar(&stem, s).is_some())
            || text_sidecar(&stem, ".formatted").is_some();
        if has_sidecar {
            continue;
        }
        bare += 1;
        let value = String::from_utf8(case.input.clone()).expect("UTF-8");
        parse_rrule(value.trim_end_matches(['\r', '\n']))
            .unwrap_or_else(|e| panic!("{}: must parse: {e}", case.path.display()));
    }
    assert_eq!(
        bare, 18,
        "the corpus carries 18 fixtures whose only assertion is that they parse"
    );
}

// ── Assertions the corpus cannot express ──────────────────────────────

/// The corpus has no `format/` fixture with an unsorted multi-entry
/// BYDAY, so a formatter (or parser) that sorts list values passes every
/// fixture. This is the assertion that kills it.
///
/// Both `format/` fixtures spell `BYDAY=MO,WE` — already in weekday
/// order — so only a deliberately descending list separates "preserved"
/// from "sorted".
#[test]
fn list_values_keep_their_authored_order() {
    let rule = parse_rrule("FREQ=WEEKLY;BYDAY=FR,WE,MO").expect("parses");
    assert_eq!(
        rule.by_day,
        vec![
            ByDay::new(0, Weekday::Fr),
            ByDay::new(0, Weekday::We),
            ByDay::new(0, Weekday::Mo),
        ],
        "parse must not reorder a BYDAY list"
    );
    assert_eq!(
        rule.to_string(),
        "FREQ=WEEKLY;BYDAY=FR,WE,MO",
        "the wire form must not sort a BYDAY list"
    );

    // The same for the integer lists, each of which has its own writer.
    let rule = parse_rrule("FREQ=MONTHLY;BYMONTHDAY=-1,15,3").expect("parses");
    assert_eq!(rule.by_month_day, vec![-1, 15, 3]);
    assert_eq!(rule.to_string(), "FREQ=MONTHLY;BYMONTHDAY=-1,15,3");

    let rule = parse_rrule("FREQ=YEARLY;BYMONTH=12,1,6").expect("parses");
    assert_eq!(rule.by_month, vec![12, 1, 6]);
    assert_eq!(rule.to_string(), "FREQ=YEARLY;BYMONTH=12,1,6");

    // An ordinal BYDAY entry keeps its prefix and its position.
    let rule = parse_rrule("FREQ=MONTHLY;BYDAY=-1FR,2MO").expect("parses");
    assert_eq!(
        rule.by_day,
        vec![ByDay::new(-1, Weekday::Fr), ByDay::new(2, Weekday::Mo)]
    );
    assert_eq!(rule.to_string(), "FREQ=MONTHLY;BYDAY=-1FR,2MO");
}

/// `INTERVAL=1` and `WKST=MO` are the RFC defaults and are elided; every
/// other value is spelled out.
#[test]
fn rfc_defaults_are_elided_and_non_defaults_are_not() {
    assert_eq!(
        parse_rrule("FREQ=DAILY;INTERVAL=1")
            .expect("parses")
            .to_string(),
        "FREQ=DAILY"
    );
    assert_eq!(
        parse_rrule("FREQ=DAILY;WKST=MO")
            .expect("parses")
            .to_string(),
        "FREQ=DAILY"
    );
    assert_eq!(
        parse_rrule("FREQ=DAILY;INTERVAL=2")
            .expect("parses")
            .to_string(),
        "FREQ=DAILY;INTERVAL=2"
    );
    assert_eq!(
        parse_rrule("FREQ=WEEKLY;WKST=SU")
            .expect("parses")
            .to_string(),
        "FREQ=WEEKLY;WKST=SU"
    );
    // A Rule built by hand, not through the parser, elides identically.
    let mut rule = Rule::new(Freq::Daily);
    rule.interval = 1;
    assert_eq!(rule.to_string(), "FREQ=DAILY");
}

/// The fixed rule-part order, exercised on a rule carrying every part at
/// once. The corpus' two `format/` fixtures between them touch four
/// parts; this one touches all fourteen.
#[test]
fn the_wire_form_uses_the_fixed_rule_part_order() {
    let authored = "FREQ=YEARLY;INTERVAL=3;COUNT=5;BYMONTH=3;BYWEEKNO=2;BYYEARDAY=100;\
                    BYMONTHDAY=15;BYDAY=2MO;BYHOUR=9;BYMINUTE=30;BYSECOND=15;BYSETPOS=1;WKST=SU";
    let rule = parse_rrule(authored).expect("parses");
    assert_eq!(
        rule.to_string(),
        authored,
        "order is FREQ..WKST as authored"
    );

    // Scrambling the input does not scramble the output.
    let scrambled = "WKST=SU;BYSETPOS=1;BYSECOND=15;BYMINUTE=30;BYHOUR=9;BYDAY=2MO;\
                     BYMONTHDAY=15;BYYEARDAY=100;BYWEEKNO=2;BYMONTH=3;COUNT=5;INTERVAL=3;FREQ=YEARLY";
    assert_eq!(
        parse_rrule(scrambled).expect("parses").to_string(),
        authored
    );

    // UNTIL sits where COUNT does, before the BY-* clauses.
    let with_until = parse_rrule("FREQ=DAILY;UNTIL=20260401T120000Z;BYHOUR=9").expect("parses");
    assert_eq!(
        with_until.to_string(),
        "FREQ=DAILY;UNTIL=20260401T120000Z;BYHOUR=9"
    );
}

/// The iteration cap is a distinct outcome from termination, on every
/// entry point that has an error channel.
#[test]
fn the_iteration_cap_is_distinct_from_termination() {
    let rule =
        parse_rrule("FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30").expect("an unsatisfiable rule parses");
    let dtstart = parse_time("20260101T090000Z").expect("instant");

    let err = next_occurrence(&rule, dtstart, dtstart).expect_err("cap, not termination");
    assert_eq!(err.sentinel(), "ErrIterationCap");

    let err = occurrences(&rule, dtstart, 5).expect_err("cap, not an empty complete series");
    assert_eq!(err.sentinel(), "ErrIterationCap");

    let end = parse_time("20300101T000000Z").expect("instant");
    let err = between(&rule, dtstart, dtstart, end).expect_err("cap, not an empty window");
    assert_eq!(err.sentinel(), "ErrIterationCap");

    // A terminating rule is the other side of the same coin: it reports
    // Ok, never the cap.
    let bounded = parse_rrule("FREQ=DAILY;COUNT=2").expect("parses");
    assert_eq!(
        next_occurrence(
            &bounded,
            dtstart,
            parse_time("20260102T090000Z").expect("instant")
        )
        .expect("terminates cleanly"),
        None
    );
    assert_eq!(MAX_ITERATIONS, 100_000, "the documented bound");
}

/// An unbounded window is a failure class, not an empty result.
#[test]
fn an_unbounded_window_is_a_failure_class() {
    let rule = parse_rrule("FREQ=DAILY").expect("parses");
    let t = parse_time("20260401T120000Z").expect("instant");

    let err = between(&rule, t, t, t).expect_err("a zero-width window is unbounded");
    assert_eq!(err.sentinel(), "ErrUnboundedExpansion");

    let earlier = parse_time("20260301T120000Z").expect("instant");
    let err = between(&rule, t, t, earlier).expect_err("an inverted window is unbounded");
    assert_eq!(err.sentinel(), "ErrUnboundedExpansion");

    // The same class guards a set.
    let set = RuleSet {
        dtstart: t,
        rrule: Some(rule),
        rdate: Vec::new(),
        exdate: Vec::new(),
    };
    let err = set
        .between(t, t)
        .expect_err("a set's window is bounded too");
    assert_eq!(err.sentinel(), "ErrUnboundedExpansion");
}

/// `all` is lazy: it must be usable on an unbounded rule by stopping.
#[test]
fn all_is_lazy_and_stops_when_the_consumer_does() {
    let rule = parse_rrule("FREQ=DAILY").expect("parses");
    let dtstart = parse_time("20260401T120000Z").expect("instant");
    let first: Vec<_> = all(&rule, dtstart).take(3).collect();
    assert_eq!(
        render(&first),
        vec![
            "20260401T120000Z".to_owned(),
            "20260402T120000Z".to_owned(),
            "20260403T120000Z".to_owned(),
        ]
    );

    // A bounded rule ends on its own.
    let bounded = parse_rrule("FREQ=DAILY;COUNT=2").expect("parses");
    assert_eq!(all(&bounded, dtstart).count(), 2);
}

/// EXDATE is applied last, so an excluded instant stays excluded even
/// when an RDATE names it. Ordering the two the other way makes the
/// RDATE win, which is the bug this pins.
#[test]
fn exdate_is_applied_after_rdate() {
    let dtstart = parse_time("20260401T120000Z").expect("instant");
    let shared = parse_time("20260410T120000Z").expect("instant");
    let set = RuleSet {
        dtstart,
        rrule: None,
        rdate: vec![shared],
        exdate: vec![shared],
    };
    let (times, complete) = set.occurrences(10).expect("expands");
    assert_eq!(
        render(&times),
        vec!["20260401T120000Z".to_owned()],
        "an RDATE cannot resurrect an EXDATE-ed instant"
    );
    assert!(complete);

    // And through the window entry point.
    let end = parse_time("20260501T000000Z").expect("instant");
    assert_eq!(
        render(&set.between(dtstart, end).expect("window")),
        vec!["20260401T120000Z".to_owned()]
    );
}

/// The set pipeline: DTSTART first, RRULE expanded, RDATE merged, the
/// whole sorted and de-duplicated.
#[test]
fn a_set_sorts_and_deduplicates_across_its_sources() {
    let dtstart = parse_time("20260401T120000Z").expect("instant");
    let set = RuleSet {
        dtstart,
        rrule: Some(parse_rrule("FREQ=DAILY;COUNT=3").expect("parses")),
        // Out of order, and one value coincides with a rule occurrence.
        rdate: vec![
            parse_time("20260405T120000Z").expect("instant"),
            parse_time("20260402T120000Z").expect("instant"),
        ],
        exdate: Vec::new(),
    };
    let (times, complete) = set.occurrences(10).expect("expands");
    assert_eq!(
        render(&times),
        vec![
            "20260401T120000Z".to_owned(),
            "20260402T120000Z".to_owned(),
            "20260403T120000Z".to_owned(),
            "20260405T120000Z".to_owned(),
        ],
        "sorted, and 04-02 appears once despite being both RRULE and RDATE"
    );
    assert!(complete, "an RDATE-bounded series ends within the limit");

    // A limit that truncates reports it.
    let (times, complete) = set.occurrences(2).expect("expands");
    assert_eq!(times.len(), 2);
    assert!(!complete, "the limit truncated the series");
}

/// `VALUE=DATE` and `TZID` on EXDATE/RDATE are outside the v0.1 scope.
#[test]
fn zoned_and_date_only_set_values_are_unsupported() {
    let build = |prop: Property| {
        let mut c = hop_top_vstar::Component::new(hop_top_vstar::CompType::event());
        c.set(Property::new("UID", "u"));
        c.set(Property::new("DTSTART", "20260401T120000Z"));
        c.add(prop);
        RuleSet::from_component(&c)
    };

    for name in ["EXDATE", "RDATE"] {
        let tzid = Property::new(name, "20260402T080000").with_param("TZID", "America/New_York");
        assert_sentinel(
            &format!("{name} with TZID"),
            "ErrUnsupportedRRule",
            build(tzid).map(|_| ()),
        );

        let date_only = Property::new(name, "20260402").with_param("VALUE", "DATE");
        assert_sentinel(
            &format!("{name} with VALUE=DATE"),
            "ErrUnsupportedRRule",
            build(date_only).map(|_| ()),
        );
    }
}

/// `RECURRENCE-ID` parses to an instant plus the RANGE parameter.
#[test]
fn recurrence_ids_carry_their_range_parameter() {
    let plain = Property::new("RECURRENCE-ID", "20260401T120000Z");
    let id = parse_recurrence_id(&plain).expect("parses");
    assert_eq!(id.time, parse_time("20260401T120000Z").expect("instant"));
    assert_eq!(id.range, Range::ThisInstance, "the absent default");
    assert_eq!(
        id.to_property().params,
        Vec::<Param>::new(),
        "the default is omitted"
    );

    let ranged = plain.clone().with_param("RANGE", "THISANDFUTURE");
    let id = parse_recurrence_id(&ranged).expect("parses");
    assert_eq!(id.range, Range::ThisAndFuture);
    assert_eq!(
        id.to_property().params,
        vec![Param::new("RANGE", "THISANDFUTURE")]
    );
    assert_eq!(Range::ThisInstance.to_string(), "");
    assert_eq!(Range::ThisAndFuture.to_string(), "THISANDFUTURE");

    // A RANGE the RFC does not define is malformed; a zoned or
    // date-only value is unsupported.
    assert_sentinel(
        "RANGE=BOGUS",
        "ErrMalformed",
        parse_recurrence_id(&plain.clone().with_param("RANGE", "BOGUS")).map(|_| ()),
    );
    assert_sentinel(
        "a TZID RECURRENCE-ID",
        "ErrUnsupportedRRule",
        parse_recurrence_id(
            &Property::new("RECURRENCE-ID", "20260401T120000")
                .with_param("TZID", "America/New_York"),
        )
        .map(|_| ()),
    );
    assert_sentinel(
        "a property that is not RECURRENCE-ID",
        "ErrMalformed",
        parse_recurrence_id(&Property::new("DTSTART", "20260401T120000Z")).map(|_| ()),
    );

    let round_tripped = RecurrenceId {
        time: parse_time("20260401T120000Z").expect("instant"),
        range: Range::ThisAndFuture,
    };
    assert_eq!(
        parse_recurrence_id(&round_tripped.to_property()).expect("re-parses"),
        round_tripped
    );
}

/// The EXDATE/RDATE value list sorts and de-duplicates in both
/// directions, so identical logical content yields identical bytes.
#[test]
fn date_time_lists_round_trip_sorted_and_deduplicated() {
    let parsed =
        parse_date_time_list("20260403T120000Z,20260401T120000Z,20260403T120000Z").expect("parses");
    assert_eq!(
        render(&parsed),
        vec!["20260401T120000Z".to_owned(), "20260403T120000Z".to_owned()]
    );
    assert_eq!(
        format_date_time_list(&parsed),
        "20260401T120000Z,20260403T120000Z"
    );
    assert_eq!(format_date_time_list(&[]), "");

    assert_sentinel(
        "an empty list",
        "ErrMalformed",
        parse_date_time_list("").map(|_| ()),
    );
    assert_sentinel(
        "a local-time value",
        "ErrMalformed",
        parse_date_time_list("20260401T120000").map(|_| ()),
    );
}

/// The deferred features parse syntactically and are reported as
/// unsupported, not malformed — two different answers the corpus
/// distinguishes.
#[test]
fn deferred_features_are_unsupported_not_malformed() {
    for value in ["FREQ=SECONDLY", "FREQ=DAILY;RSCALE=CHINESE"] {
        assert_sentinel(value, "ErrUnsupportedRRule", validate_rrule(value));
        assert_sentinel(value, "ErrUnsupportedRRule", parse_rrule(value).map(|_| ()));
    }
    // MINUTELY is in scope: it parses, and it parses to its own variant.
    assert_eq!(
        parse_rrule("FREQ=MINUTELY").expect("parses").freq,
        Freq::Minutely
    );
    // validate_rrule is parse-and-discard: it agrees with parse_rrule on
    // the malformed side too.
    for value in ["", "INTERVAL=2", "FREQ=DAILY;BOGUS=1"] {
        assert_sentinel(value, "ErrMalformed", validate_rrule(value));
    }
}

/// BYMONTHDAY resolves negatives from the month's end and skips a day
/// the month does not have, rather than clamping it.
#[test]
fn bymonthday_skips_absent_days_and_resolves_negatives() {
    let dtstart = parse_time("20260101T090000Z").expect("instant");

    let rule = parse_rrule("FREQ=MONTHLY;BYMONTHDAY=29").expect("parses");
    let (times, _) = occurrences(&rule, dtstart, 3).expect("expands");
    assert_eq!(
        render(&times),
        vec![
            "20260129T090000Z".to_owned(),
            // February 2026 has 28 days: the occurrence is skipped, not
            // clamped to the 28th nor rolled into March 1.
            "20260329T090000Z".to_owned(),
            "20260429T090000Z".to_owned(),
        ]
    );

    let rule = parse_rrule("FREQ=MONTHLY;BYMONTHDAY=-1").expect("parses");
    let (times, _) = occurrences(&rule, dtstart, 3).expect("expands");
    assert_eq!(
        render(&times),
        vec![
            "20260131T090000Z".to_owned(),
            "20260228T090000Z".to_owned(),
            "20260331T090000Z".to_owned(),
        ],
        "-1 is the last day of each month, whatever its length"
    );
}

/// WKST moves the week boundary, which changes which occurrences a
/// weekly rule produces in a period.
#[test]
fn wkst_moves_the_week_boundary() {
    // 2026-01-04 is a Sunday. Under WKST=MO it closes the week that
    // began on Monday the 29th of December; under WKST=SU it opens a new
    // one.
    let dtstart = parse_time("20260101T090000Z").expect("a Thursday");
    let mo = parse_rrule("FREQ=WEEKLY;INTERVAL=2;BYDAY=SU,TH;WKST=MO").expect("parses");
    let su = parse_rrule("FREQ=WEEKLY;INTERVAL=2;BYDAY=SU,TH;WKST=SU").expect("parses");

    let (mo_times, _) = occurrences(&mo, dtstart, 4).expect("expands");
    let (su_times, _) = occurrences(&su, dtstart, 4).expect("expands");
    assert_ne!(
        render(&mo_times),
        render(&su_times),
        "WKST is load-bearing for a bi-weekly BYDAY rule"
    );
    assert_eq!(su.to_string(), "FREQ=WEEKLY;INTERVAL=2;BYDAY=SU,TH;WKST=SU");
}

/// The weekday numbering is RFC 5545's `SU = 0`, not ISO-8601's
/// `MO = 1`. A port that reuses a platform number without converting has
/// a one-off bug, and this is where it shows.
#[test]
fn weekday_numbering_is_su_zero() {
    assert_eq!(Weekday::Su as u8, 0);
    assert_eq!(Weekday::Sa as u8, 6);
    assert_eq!(Weekday::Su.to_chrono(), chrono::Weekday::Sun);
    assert_eq!(Weekday::Mo.to_chrono(), chrono::Weekday::Mon);
    assert_eq!(Weekday::Sa.to_chrono(), chrono::Weekday::Sat);
    for (w, s) in [
        (Weekday::Su, "SU"),
        (Weekday::Mo, "MO"),
        (Weekday::Tu, "TU"),
        (Weekday::We, "WE"),
        (Weekday::Th, "TH"),
        (Weekday::Fr, "FR"),
        (Weekday::Sa, "SA"),
    ] {
        assert_eq!(w.to_string(), s);
    }
    for (f, s) in [
        (Freq::Minutely, "MINUTELY"),
        (Freq::Hourly, "HOURLY"),
        (Freq::Daily, "DAILY"),
        (Freq::Weekly, "WEEKLY"),
        (Freq::Monthly, "MONTHLY"),
        (Freq::Yearly, "YEARLY"),
    ] {
        assert_eq!(f.to_string(), s);
    }
}

/// `occurrences` with a zero limit makes no claim about the series.
#[test]
fn a_zero_limit_yields_nothing_and_claims_nothing() {
    let rule = parse_rrule("FREQ=DAILY;COUNT=2").expect("parses");
    let dtstart = parse_time("20260401T120000Z").expect("instant");
    let (times, complete) = occurrences(&rule, dtstart, 0).expect("succeeds");
    assert!(times.is_empty());
    assert!(!complete, "zero occurrences is not a completed series");
}
