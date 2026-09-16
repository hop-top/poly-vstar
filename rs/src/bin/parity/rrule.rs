// SPDX-License-Identifier: MIT

//! The `rrule` family: rule parsing, formatting, stepping, bounded
//! expansion and windowed queries.
//!
//! Each entry's keys are flat, one group per sidecar the fixture
//! carries, so which contracts a fixture pins is readable off its key
//! set.

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use chrono::{DateTime, Utc};
use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::rrule::{self, Rule, RuleSet};
use hop_top_vstar::{format_time, parse_time, Error};

use super::corpus::{
    classify, fail, parent_of, path_ends_with, read_bytes, read_sidecar, read_text, record,
    walk_files, CLASS_MALFORMED, EXT_ICS, EXT_RRULE,
};
use super::json::Json;

/// The failure vocabulary the rrule family reports, most specific
/// first. Every token is a class named by spec/05 §Failure classes.
const RRULE_CLASSES: &[&str] = &[
    "ErrUnsupportedRRule",
    "ErrIterationCap",
    "ErrUnboundedExpansion",
    CLASS_MALFORMED,
];

/// Walk the rrule corpus and emit one object per fixture stem, keyed by
/// the stem's path relative to the conformance root.
pub fn emit(root: &Path) -> Json {
    let corpus = parent_of(root);
    let mut out = BTreeMap::new();
    for path in walk_files(root) {
        let (ext, entry) = if path_ends_with(&path, EXT_RRULE) {
            (EXT_RRULE, rule_entry(&path))
        } else if path_ends_with(&path, EXT_ICS) {
            (EXT_ICS, set_entry(&path))
        } else {
            continue;
        };
        let stem = super::corpus::fixture(path, ext).stem;
        record(&mut out, &corpus, &stem, entry);
    }
    Json::Object(out)
}

/// Evaluate one `<stem>.rrule` fixture against every sidecar it has.
///
/// A `.expect.json` sidecar means the rule must be rejected: the
/// emitter reports the class `validate_rrule` produced and stops, since
/// no other contract applies to a rule that does not parse.
fn rule_entry(path: &Path) -> Json {
    let value = read_text(path).trim_end_matches(['\r', '\n']).to_owned();
    let stem = PathBuf::from(&super::corpus::path_str(path)[..path_len_without(path, EXT_RRULE)]);

    if sidecar(&stem, ".expect.json").exists() {
        let token = failure_token(path, rrule::validate_rrule(&value).err().as_ref());
        return Json::obj([("error", Json::str(token))]);
    }

    let rule = rrule::parse_rrule(&value)
        .unwrap_or_else(|e| fail(format!("{}: parse_rrule failed: {e}", path.display())));

    let mut entry = BTreeMap::new();
    entry.insert("parsed".to_owned(), Json::Bool(true));
    add_formatted(&stem, &rule, &mut entry);
    add_next(&stem, &rule, &mut entry);
    add_expand(&stem, &rule, &mut entry);
    add_between(&stem, &rule, &mut entry);
    Json::Object(entry)
}

/// Evaluate one `<stem>.ics` recurrence-set fixture: the calendar's
/// first component goes through [`RuleSet::from_component`], then
/// either `.expect.json` pins a rejection or `.occurrences.json` pins
/// the bounded expansion.
fn set_entry(path: &Path) -> Json {
    let stem = PathBuf::from(&super::corpus::path_str(path)[..path_len_without(path, EXT_ICS)]);
    let set = set_from_calendar(&read_bytes(path));

    if sidecar(&stem, ".expect.json").exists() {
        let token = failure_token(path, set.as_ref().err());
        return Json::obj([("error", Json::str(token))]);
    }
    let set = set.unwrap_or_else(|e| fail(format!("{}: {e}", path.display())));

    let mut entry = BTreeMap::new();
    entry.insert("parsed".to_owned(), Json::Bool(true));
    let occ_path = sidecar(&stem, ".occurrences.json");
    if let Some(spec) = read_sidecar(&occ_path) {
        let limit = limit_of(&spec);
        add_bounded(&mut entry, &occ_path, "occurrences", set.occurrences(limit));
    }
    Json::Object(entry)
}

/// Parse an `.ics` fixture and build a set from its first component,
/// mirroring the reference verifier's entry point.
fn set_from_calendar(input: &[u8]) -> Result<RuleSet, Error> {
    let cal = rfc5545::parse(input)?;
    let first = cal
        .components
        .first()
        .ok_or_else(|| Error::Malformed("calendar has no components".into()))?;
    RuleSet::from_component(first)
}

/// Report `Rule`'s wire form when a `<stem>.formatted` sidecar exists.
fn add_formatted(stem: &Path, rule: &Rule, entry: &mut BTreeMap<String, Json>) {
    if sidecar(stem, ".formatted").exists() {
        entry.insert("formatted".to_owned(), Json::str(rule.to_string()));
    }
}

/// Walk `next_occurrence` the way the sidecar's expected list is
/// shaped: one step per entry, each feeding its result back as the next
/// `after`. The emitted list is what this port yielded.
///
/// One extra step runs past the end, and its outcome is the terminal
/// contract: `next_error` names the class the series failed with, or
/// `next_complete` states whether it terminated. Emitting that step
/// unconditionally means a port cannot pass by stopping early — a
/// series that should terminate and one that should raise
/// `ErrIterationCap` differ in the document.
fn add_next(stem: &Path, rule: &Rule, entry: &mut BTreeMap<String, Json>) {
    let path = sidecar(stem, ".next.json");
    let Some(spec) = read_sidecar(&path) else {
        return;
    };
    let dt = parse_stamp(&path, "dtstart", spec.str_field("dtstart"));
    let mut after = parse_stamp(&path, "after", spec.str_field("after"));

    // `expected` is read only to learn how many steps to take; its
    // values are never emitted. Copying them would make the harness
    // compare fixtures to themselves.
    let steps = spec
        .get("expected")
        .and_then(Json::as_array)
        .map_or(0, <[Json]>::len);

    let mut stamps = Vec::with_capacity(steps);
    for i in 0..steps {
        match rrule::next_occurrence(rule, dt, after) {
            Err(e) => fail(format!(
                "{}: next_occurrence step {i} failed: {e}",
                path.display()
            )),
            Ok(None) => fail(format!(
                "{}: next_occurrence step {i} terminated early",
                path.display()
            )),
            Ok(Some(got)) => {
                stamps.push(Json::str(format_time(got)));
                after = got;
            }
        }
    }
    entry.insert("next".to_owned(), Json::Array(stamps));

    match rrule::next_occurrence(rule, dt, after) {
        Ok(got) => {
            entry.insert("next_complete".to_owned(), Json::Bool(got.is_none()));
        }
        Err(e) => {
            entry.insert(
                "next_error".to_owned(),
                Json::str(failure_token(&path, Some(&e))),
            );
        }
    }
}

/// Report bounded expansion over the `.expand.json` sidecar's limit.
fn add_expand(stem: &Path, rule: &Rule, entry: &mut BTreeMap<String, Json>) {
    let path = sidecar(stem, ".expand.json");
    let Some(spec) = read_sidecar(&path) else {
        return;
    };
    let dt = parse_stamp(&path, "dtstart", spec.str_field("dtstart"));
    let limit = limit_of(&spec);
    add_bounded(entry, &path, "expand", rrule::occurrences(rule, dt, limit));
}

/// Report `between` over the sidecar's half-open window.
///
/// `between` has no completeness notion — the window bounds the answer
/// — so the success shape is the list alone.
fn add_between(stem: &Path, rule: &Rule, entry: &mut BTreeMap<String, Json>) {
    let path = sidecar(stem, ".between.json");
    let Some(spec) = read_sidecar(&path) else {
        return;
    };
    let dt = parse_stamp(&path, "dtstart", spec.str_field("dtstart"));
    let start = parse_stamp(&path, "start", spec.str_field("start"));
    let end = parse_stamp(&path, "end", spec.str_field("end"));
    match rrule::between(rule, dt, start, end) {
        Ok(times) => {
            entry.insert("between".to_owned(), format_stamps(&times));
        }
        Err(e) => {
            entry.insert(
                "between_error".to_owned(),
                Json::str(failure_token(&path, Some(&e))),
            );
        }
    }
}

/// Record a bounded-expansion outcome on `entry`: the occurrence list
/// under `key` plus a `<key>_complete` flag on success, or
/// `<key>_error` naming the failure class.
fn add_bounded(
    entry: &mut BTreeMap<String, Json>,
    path: &Path,
    key: &str,
    result: Result<(Vec<DateTime<Utc>>, bool), Error>,
) {
    match result {
        Ok((times, complete)) => {
            entry.insert(key.to_owned(), format_stamps(&times));
            entry.insert(format!("{key}_complete"), Json::Bool(complete));
        }
        Err(e) => {
            entry.insert(
                format!("{key}_error"),
                Json::str(failure_token(path, Some(&e))),
            );
        }
    }
}

/// Classify a failure into its rrule class token, refusing to emit
/// anything for a success or an unrecognized class.
///
/// An unrecognized class must fail the emitter rather than travel into
/// the document as a token no port implements.
fn failure_token(path: &Path, err: Option<&Error>) -> &'static str {
    let Some(err) = err else {
        fail(format!(
            "{}: expected a failure, the call succeeded",
            path.display()
        ));
    };
    classify(err, RRULE_CLASSES).unwrap_or_else(|| {
        fail(format!(
            "{}: failure matches no known class: {err}",
            path.display()
        ))
    })
}

/// Render occurrences as RFC 5545 form #2, the one timestamp shape
/// every family and every port uses. Always a list, never `null`.
fn format_stamps(times: &[DateTime<Utc>]) -> Json {
    Json::Array(times.iter().map(|t| Json::str(format_time(*t))).collect())
}

/// Read an RFC 5545 form #2 sidecar input value.
fn parse_stamp(path: &Path, field: &str, value: Option<&str>) -> DateTime<Utc> {
    value.and_then(parse_time).unwrap_or_else(|| {
        fail(format!(
            "{}: bad {field} {:?}",
            path.display(),
            value.unwrap_or_default()
        ))
    })
}

/// Read a sidecar's `limit`, defaulting to zero the way Go's
/// zero-valued `int` field does when the key is absent.
fn limit_of(spec: &Json) -> usize {
    usize::try_from(spec.int_field("limit").unwrap_or(0)).unwrap_or(0)
}

/// The path of a sidecar named by appending `suffix` to `stem`.
fn sidecar(stem: &Path, suffix: &str) -> PathBuf {
    let mut name = stem.as_os_str().to_owned();
    name.push(suffix);
    PathBuf::from(name)
}

/// The byte length of `path` with `ext` removed.
fn path_len_without(path: &Path, ext: &str) -> usize {
    super::corpus::path_str(path).len() - ext.len()
}
