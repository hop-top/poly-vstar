// SPDX-License-Identifier: MIT

//! The `validate` surface — V\*'s semantic conformance checks — gated by
//! `spec/behavior/validate/*.diagnostics.json`.
//!
//! Every `.ics` under `spec/behavior/validate/` has a same-stem
//! `.diagnostics.json` sibling naming the exact findings the reference
//! emits. The tree is walked rather than enumerated, so a fixture added
//! to the reference becomes a case here without a test edit.
//!
//! Messages are deliberately absent from the fixtures:
//! `Diagnostic::message` is human-readable and not part of the contract
//! (see `docs/validate-codes.md` §Stability). Comparison is on
//! `(code, severity, path)` only, sorted by `(path, code)`.

mod support;

use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::generated::codes::{CODES, CODE_SEVERITIES, STANDARD_PROPERTIES};
use hop_top_vstar::validate::{
    codes, severity_of, standard_property_count, validate, validate_component, Diagnostic, Severity,
};
use hop_top_vstar::{Calendar, Component, Property};
use std::collections::BTreeSet;
use std::fs;
use std::path::{Path, PathBuf};
use support::json::Json;

/// The three fields the fixtures pin. `message` is not one of them.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
struct Row {
    path: String,
    code: String,
    severity: String,
}

/// `spec/behavior/validate/`.
fn validate_dir() -> PathBuf {
    support::behavior_root().join("validate")
}

/// Every behavior-fixture stem, sorted.
fn stems() -> Vec<String> {
    support::behavior_stems("validate", ".ics")
}

/// Parses `<stem>.ics` from `spec/behavior/validate/`.
fn load_calendar(stem: &str) -> Calendar {
    let path = validate_dir().join(format!("{stem}.ics"));
    let raw = fs::read(&path).unwrap_or_else(|e| panic!("read {}: {e}", path.display()));
    rfc5545::parse(&raw[..]).unwrap_or_else(|e| panic!("parse {}: {e}", path.display()))
}

/// Reads `<stem>.diagnostics.json` into the pinned rows, sorted.
fn load_expected(stem: &str) -> Vec<Row> {
    let path = validate_dir().join(format!("{stem}.diagnostics.json"));
    let text = fs::read_to_string(&path).unwrap_or_else(|e| panic!("read {}: {e}", path.display()));
    let doc: Json =
        support::json::parse(&text).unwrap_or_else(|e| panic!("parse {}: {e}", path.display()));
    let mut out: Vec<Row> = doc
        .array()
        .iter()
        .map(|row| Row {
            path: row
                .str_field("path")
                .expect("fixture row carries a path")
                .to_owned(),
            code: row
                .str_field("code")
                .expect("fixture row carries a code")
                .to_owned(),
            severity: row
                .str_field("severity")
                .expect("fixture row carries a severity")
                .to_owned(),
        })
        .collect();
    out.sort();
    out
}

/// A `Diagnostic` reduced to the three fields the fixtures compare.
fn to_row(d: &Diagnostic) -> Row {
    Row {
        path: d.path.clone(),
        code: d.code.to_owned(),
        severity: d.severity.to_string(),
    }
}

/// Produced diagnostics as sorted rows.
fn rows(ds: &[Diagnostic]) -> Vec<Row> {
    let mut out: Vec<Row> = ds.iter().map(to_row).collect();
    out.sort();
    out
}

/// The lone component of a single-component fixture.
fn sole_component(stem: &str) -> Component {
    let cal = load_calendar(stem);
    assert_eq!(
        cal.components.len(),
        1,
        "{stem} must carry exactly one component"
    );
    cal.components[0].clone()
}

#[test]
fn behavior_fixtures_match_the_reference_diagnostics() {
    let stems = stems();
    assert!(!stems.is_empty(), "the gate directory is empty");
    for stem in &stems {
        let got = rows(&validate(&load_calendar(stem)));
        let want = load_expected(stem);
        assert_eq!(got, want, "{stem}: diagnostics diverge from the fixture");
    }
}

#[test]
fn every_registry_code_is_exercised_by_a_fixture() {
    let mut emitted: BTreeSet<&str> = BTreeSet::new();
    for stem in stems() {
        for d in validate(&load_calendar(&stem)) {
            emitted.insert(d.code);
        }
    }
    let missing: Vec<&str> = CODES
        .iter()
        .copied()
        .filter(|c| !emitted.contains(c))
        .collect();
    assert!(
        missing.is_empty(),
        "codes with no fixture coverage: {missing:?}"
    );
}

#[test]
fn every_emitted_severity_matches_the_registry() {
    for stem in stems() {
        for d in validate(&load_calendar(&stem)) {
            let want = severity_of(d.code).unwrap_or_else(|| panic!("{} is not a code", d.code));
            assert_eq!(
                d.severity, want,
                "{stem}: {} severity diverges from the registry",
                d.code
            );
        }
    }
}

#[test]
fn every_fixture_row_severity_matches_the_registry() {
    for stem in stems() {
        for row in load_expected(&stem) {
            let want = CODE_SEVERITIES
                .iter()
                .find(|(c, _)| *c == row.code)
                .unwrap_or_else(|| panic!("{} is not a registry code", row.code))
                .1;
            assert_eq!(row.severity, want, "{stem}: fixture severity diverges");
        }
    }
}

#[test]
fn codes_lists_every_registry_code() {
    let mut got = codes();
    got.sort_unstable();
    let mut want: Vec<&str> = CODES.to_vec();
    want.sort_unstable();
    assert_eq!(got, want);
}

#[test]
fn severity_of_answers_from_the_registry() {
    for (code, severity) in CODE_SEVERITIES {
        let got = severity_of(code).unwrap_or_else(|| panic!("{code} must resolve"));
        assert_eq!(got.to_string(), severity, "{code}");
    }
}

#[test]
fn severity_of_is_none_for_an_unknown_code() {
    assert_eq!(severity_of("NOT-A-CODE"), None);
    assert_eq!(severity_of(""), None);
}

#[test]
fn standard_property_count_matches_the_generated_table() {
    assert_eq!(standard_property_count(), STANDARD_PROPERTIES.len());
    assert!(standard_property_count() > 0);
}

#[test]
fn severity_renders_the_lowercase_wire_name() {
    assert_eq!(Severity::Error.to_string(), "error");
    assert_eq!(Severity::Warning.to_string(), "warning");
}

#[test]
fn validate_component_paths_omit_the_calendar_prefix() {
    let got = rows(&validate_component(&sole_component("missing_dtstamp")));
    assert_eq!(
        got,
        vec![Row {
            path: "VJOURNAL[uid=journal-no-dtstamp].DTSTAMP".to_owned(),
            code: "VS002".to_owned(),
            severity: "error".to_owned(),
        }]
    );
}

#[test]
fn validate_component_still_emits_the_component_local_supersession_diagnostic() {
    let got = validate_component(&sole_component("supersession_missing_props"));
    let want = load_expected("supersession_missing_props")
        .first()
        .expect("the fixture carries a VS030 row")
        .code
        .clone();
    assert!(
        got.iter().any(|d| d.code == want),
        "validate_component must still report {want}"
    );
}

#[test]
fn validate_component_skips_the_orphan_supersession_diagnostic() {
    // The orphan fixture's RELATED-TO resolves to nothing even inside
    // its own calendar, so `validate` flags it. `validate_component`
    // sees one component and cannot resolve anything, so it must stay
    // silent rather than guess.
    let orphan = load_expected("supersession_orphan")
        .first()
        .expect("the fixture carries a VS031 row")
        .code
        .clone();
    let from_calendar = validate(&load_calendar("supersession_orphan"));
    assert!(
        from_calendar.iter().any(|d| d.code == orphan),
        "validate must report {orphan}"
    );
    let from_component = validate_component(&sole_component("supersession_orphan"));
    assert!(
        !from_component.iter().any(|d| d.code == orphan),
        "validate_component must not report {orphan} — it has no ledger"
    );
}

#[test]
fn the_conformance_corpus_is_clean_of_every_value_domain_diagnostic() {
    // Mirrors the Go corpus walk: every VCALENDAR fixture in the
    // conformance corpus carries only STATUS, CLASS and TRANSP values
    // inside their vocabularies and only in-domain integers. A hit
    // here means the port's rule is wrong, not the fixture.
    let value_domain_codes: Vec<String> = [
        "status_not_in_vocabulary",
        "class_not_in_vocabulary",
        "transp_not_in_vocabulary",
        "priority_out_of_range",
    ]
    .into_iter()
    .map(fixture_code)
    .collect();
    let mut checked = 0usize;
    for family in ["rfc5545", "supersession"] {
        let dir = support::corpus_root().join(family);
        for case in support::cases_in(&dir, "ics") {
            let cal = rfc5545::parse(&case.input[..])
                .unwrap_or_else(|e| panic!("parse {}: {e}", case.path.display()));
            let hits: Vec<(String, String)> = validate(&cal)
                .iter()
                .filter(|d| value_domain_codes.iter().any(|c| c == d.code))
                .map(|d| (d.code.to_owned(), d.path.clone()))
                .collect();
            assert!(
                hits.is_empty(),
                "{family}/{}: unexpected value-domain diagnostics {hits:?}",
                case.stem
            );
            checked += 1;
        }
    }
    assert!(checked > 0, "the corpus walk asserted nothing");
}

#[test]
fn a_journal_only_status_on_a_vevent_is_flagged() {
    // The cross-type shape the rule exists for: DRAFT is legal
    // iCalendar text, and legal for a VJOURNAL, but not for a VEVENT.
    let status_code = load_expected("status_not_in_vocabulary")
        .first()
        .expect("the fixture carries a VS044 row")
        .code
        .clone();
    let mut cal = load_calendar("clean_vevent");
    assert!(
        validate(&cal).iter().all(|d| d.code != status_code),
        "the fixture must be clean before the STATUS rewrite"
    );
    cal.components[0].set(Property::new("STATUS", "DRAFT"));
    let hit = validate(&cal)
        .into_iter()
        .find(|d| d.code == status_code)
        .expect("STATUS=DRAFT on a VEVENT must be flagged");
    assert_eq!(hit.path, "VCALENDAR.VEVENT[uid=event-clean].STATUS");
}

#[test]
fn uid_less_components_get_distinct_positional_paths() {
    // Two UID-less components of the same type must not collide: the
    // running index is what makes each diagnostic addressable.
    let mut cal = load_calendar("missing_uid");
    let first = cal.components[0].clone();
    cal.components.push(first);
    let paths: BTreeSet<String> = validate(&cal).iter().map(|d| d.path.clone()).collect();
    assert!(
        paths.iter().any(|p| p.contains("[#0]")),
        "expected a [#0] segment in {paths:?}"
    );
    assert!(
        paths.iter().any(|p| p.contains("[#1]")),
        "expected a [#1] segment in {paths:?}"
    );
}

#[test]
fn a_clean_component_yields_nothing() {
    assert_eq!(validate(&load_calendar("clean_vevent")), Vec::new());
    assert_eq!(
        validate(&load_calendar("clean_vtodo_completed")),
        Vec::new()
    );
    assert_eq!(
        validate_component(&sole_component("clean_vevent")),
        Vec::new()
    );
}

/// The duration code, read off the fixture rather than spelled here.
fn duration_code() -> String {
    load_expected("malformed_duration")
        .first()
        .expect("the fixture carries a duration row")
        .code
        .clone()
}

/// Which codes `validate_component` reports for `c`.
fn component_codes(c: &Component) -> Vec<String> {
    validate_component(c)
        .iter()
        .map(|d| d.code.to_owned())
        .collect()
}

#[test]
fn a_negative_repeat_is_malformed() {
    // The behavior fixtures reach the duration rule only through the
    // DURATION property, so the REPEAT arm is unexercised by the shared
    // corpus. A `REPEAT` that parses as an integer but counts backwards
    // is still not a repeat count (RFC 5545 §3.8.6.2), and without this
    // case dropping the sign check is invisible.
    let mut c = sole_component("clean_vevent");
    c.set(Property::new("REPEAT", "-1"));
    let want = duration_code();
    assert!(
        component_codes(&c).contains(&want),
        "REPEAT=-1 must yield {want}, got {:?}",
        component_codes(&c)
    );

    c.set(Property::new("REPEAT", "0"));
    assert!(
        !component_codes(&c).contains(&want),
        "REPEAT=0 is a legal count and must stay clean"
    );

    c.set(Property::new("REPEAT", "not-a-number"));
    assert!(
        component_codes(&c).contains(&want),
        "a non-numeric REPEAT must yield {want}"
    );
}

#[test]
fn a_malformed_relative_trigger_is_flagged() {
    // The other duration arm the shared fixtures never reach. TRIGGER
    // routes through the trigger parser rather than the bare value
    // parser, so its parameter rules are enforced with the value.
    let mut c = sole_component("clean_vevent");
    c.set(Property::new("TRIGGER", "-PT15Q"));
    let want = duration_code();
    assert!(
        component_codes(&c).contains(&want),
        "a malformed TRIGGER must yield {want}, got {:?}",
        component_codes(&c)
    );

    c.set(Property::new("TRIGGER", "-PT15M"));
    assert!(
        !component_codes(&c).contains(&want),
        "a well-formed relative TRIGGER must stay clean"
    );
}

/// The first code a provoking fixture pins, read off the fixture rather
/// than spelled here.
fn fixture_code(stem: &str) -> String {
    load_expected(stem)
        .first()
        .unwrap_or_else(|| panic!("{stem} must carry a diagnostic row"))
        .code
        .clone()
}

/// The three integer properties the value-domain rule bounds.
const INTEGER_PROPS: [&str; 3] = ["PRIORITY", "PERCENT-COMPLETE", "SEQUENCE"];

#[test]
fn the_vocabulary_rules_are_not_type_gated() {
    // The fixtures put CLASS and TRANSP on a VEVENT. spec/05 §8 binds
    // the value wherever the property appears and component scope is
    // not diagnosed, so a VTODO carrying either gets the same value
    // check — and the same pass for an in-vocabulary value.
    let class_code = fixture_code("class_not_in_vocabulary");
    let transp_code = fixture_code("transp_not_in_vocabulary");
    let mut c = sole_component("clean_vtodo_completed");
    c.set(Property::new("CLASS", "X-SECRET"));
    c.set(Property::new("TRANSP", "BUSY"));
    let got = component_codes(&c);
    assert!(
        got.contains(&class_code),
        "CLASS:X-SECRET on a VTODO must yield {class_code}, got {got:?}"
    );
    assert!(
        got.contains(&transp_code),
        "TRANSP:BUSY on a VTODO must yield {transp_code}, got {got:?}"
    );

    c.set(Property::new("CLASS", "CONFIDENTIAL"));
    c.set(Property::new("TRANSP", "OPAQUE"));
    let got = component_codes(&c);
    assert!(
        !got.contains(&class_code) && !got.contains(&transp_code),
        "in-vocabulary CLASS and TRANSP on a VTODO must stay clean, got {got:?}"
    );
}

#[test]
fn a_whitespace_padded_integer_is_out_of_domain() {
    // No fixture can carry a padded value: the encoder trims it. The
    // canonical-decimal form admits digits only, so a leading or a
    // trailing blank is a defect even though the digits are in range.
    let want = fixture_code("priority_out_of_range");
    for prop in INTEGER_PROPS {
        for value in [" 3", "3 "] {
            let mut c = sole_component("clean_vevent");
            c.set(Property::new(prop, value));
            assert!(
                component_codes(&c).contains(&want),
                "{prop}:{value:?} must yield {want}"
            );
        }
    }
}

#[test]
fn the_canonical_decimal_rule_is_textual() {
    // A sign or a leading zero is out of domain although the number is
    // in range, and a value past every machine integer is in domain:
    // the decision is made on the digit string, never on a parsed
    // value. An integer parse gets all three of these wrong.
    let want = fixture_code("priority_out_of_range");
    for prop in INTEGER_PROPS {
        for value in ["+3", "07"] {
            let mut c = sole_component("clean_vevent");
            c.set(Property::new(prop, value));
            assert!(
                component_codes(&c).contains(&want),
                "{prop}:{value} must yield {want}"
            );
        }
    }

    let mut c = sole_component("clean_vevent");
    c.set(Property::new("SEQUENCE", "18446744073709551616"));
    assert!(
        !component_codes(&c).contains(&want),
        "a SEQUENCE of 2^64 is well-formed; the rule never converts to a machine integer"
    );
}

#[test]
fn the_standard_property_allow_list_is_case_insensitive() {
    // RFC 5545 §3.1 makes property names case-insensitive, and the
    // parser carries the wire spelling through verbatim. A lowercase
    // `summary` is a standard property, not an unknown one — no fixture
    // spells one, so without this case the allow-list could quietly
    // become case-sensitive and flag every lowercase document.
    let unknown = load_expected("unknown_property")
        .first()
        .expect("the fixture carries an unknown-property row")
        .code
        .clone();
    let mut c = sole_component("clean_vevent");
    c.set(Property::new("summary", "lowercased on the wire"));
    assert!(
        !component_codes(&c).contains(&unknown),
        "a lowercase standard property must not be reported as unknown"
    );

    // And the rule still fires for a genuinely unknown name, in either
    // case — so the assertion above is not passing because the whole
    // check went silent.
    c.set(Property::new("notarealproperty", "x"));
    assert!(
        component_codes(&c).contains(&unknown),
        "a lowercase unknown property must still be reported"
    );
}

// The registry is authoritative: `spec/registry/` renders into
// `src/generated/`, and hand-written source must reference the generated
// constant by name. A literal spelled out by hand is a second source of
// truth that drifts silently when the registry changes. `make
// registry-check` guards the generated file; this guards everything else.

/// The needle, assembled so this file does not itself carry the literal.
fn needle() -> String {
    format!("{}{}", "VS", '0')
}

/// Whether `line` carries a `VS0NN` diagnostic-code literal.
fn has_code_literal(line: &str) -> bool {
    let needle = needle();
    let bytes = line.as_bytes();
    line.match_indices(&needle)
        .any(|(i, _)| matches!(bytes.get(i + 3..i + 5), Some(rest) if rest.iter().all(u8::is_ascii_digit)))
}

/// Every `.rs` file under `dir`, recursively, sorted.
fn rust_sources(dir: &Path) -> Vec<PathBuf> {
    let mut out = Vec::new();
    let entries =
        fs::read_dir(dir).unwrap_or_else(|e| panic!("read source dir {}: {e}", dir.display()));
    for entry in entries {
        let path = entry.expect("read dir entry").path();
        if path.is_dir() {
            out.extend(rust_sources(&path));
        } else if path.extension().and_then(|s| s.to_str()) == Some("rs") {
            out.push(path);
        }
    }
    out.sort();
    out
}

/// `rs/src/`.
fn src_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("src")
}

#[test]
fn generated_is_the_sole_home_of_the_code_literals() {
    let generated = src_root().join("generated");
    let mut offenders: Vec<String> = Vec::new();
    for path in rust_sources(&src_root()) {
        if path.starts_with(&generated) {
            continue;
        }
        let text = fs::read_to_string(&path).unwrap_or_else(|e| panic!("read {path:?}: {e}"));
        for (i, line) in text.lines().enumerate() {
            if has_code_literal(line) {
                offenders.push(format!("{}:{}: {}", path.display(), i + 1, line.trim()));
            }
        }
    }
    assert!(
        offenders.is_empty(),
        "hand-written diagnostic-code literals: {offenders:#?}"
    );
}

#[test]
fn the_literal_guard_actually_fires() {
    // A guard that matches nothing would pass vacuously. The generated
    // module does carry the literals, so the matcher must see them.
    let generated = src_root().join("generated").join("codes.rs");
    let text = fs::read_to_string(&generated).expect("read the generated codes module");
    assert!(
        text.lines().any(has_code_literal),
        "the matcher found no code literal in the generated module"
    );
    assert!(!has_code_literal("VS0 is not a code"));
    assert!(!has_code_literal("nothing here"));
}
