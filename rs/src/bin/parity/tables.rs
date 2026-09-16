// SPDX-License-Identifier: MIT

//! The `duration`, `ext` and `time` families.
//!
//! These three carry the flat tables — `duration/parse`, `ext/scopes`
//! and `time/tzid` — whose rows are read out of a committed fixture in
//! file order. The emitter reads only the input columns and computes
//! the rest, so a row added to the corpus reaches every port without an
//! emitter edit.

use std::collections::BTreeMap;
use std::path::Path;

use hop_top_vstar::duration;
use hop_top_vstar::ext::{self, Scope};
use hop_top_vstar::helpers;
use hop_top_vstar::{format_time, parse_time_with_tzid, Calendar, CompType};

use super::corpus::{
    classify_or_unclassified, each_fixture, parent_of, read_calendar, record, row_str, table_rows,
    CLASS_MALFORMED, EXT_ICS,
};
use super::json::Json;

/// The failure vocabulary alarm resolution reports, most specific
/// first. An implementation that also wraps `ErrMalformed` around one
/// of these must still report the specific one, which is what
/// first-match ordering guarantees.
const TRIGGER_CLASSES: &[&str] = &["ErrNoTrigger", "ErrNoAnchor", CLASS_MALFORMED];

/// The vocabulary `duration::parse` failures report. Every failure is
/// `ErrMalformed` today; classifying rather than assuming means a class
/// the port grows surfaces as `Unclassified` instead of traveling as a
/// wrong token.
const DURATION_CLASSES: &[&str] = &[CLASS_MALFORMED];

/// Cover both duration families: the flat parse table at
/// `duration/parse.json`, and one alarm-resolution list per
/// `<name>.ics`.
pub fn emit_duration(dir: &Path) -> Json {
    let root = parent_of(dir);
    let mut out = BTreeMap::new();

    let path = dir.join("parse.json");
    let rows = table_rows(&path)
        .iter()
        .map(|r| parse_duration(&row_str(&path, r, "value")))
        .collect();
    out.insert("duration/parse".to_owned(), Json::Array(rows));

    for f in each_fixture(dir, EXT_ICS) {
        record(
            &mut out,
            &root,
            &f.stem,
            alarm_entries(&read_calendar(&f.path)),
        );
    }
    Json::Object(out)
}

/// Report what `duration::parse` makes of one value: the signed second
/// count and sign flag, or the failure class.
///
/// `seconds` is the whole duration as a signed second count, the one
/// representation every target language has. `negative` is kept
/// separate because a zero-length duration written with a leading sign
/// cannot be distinguished by `seconds` alone.
fn parse_duration(value: &str) -> Json {
    match duration::parse(value) {
        Ok(d) => Json::obj([
            ("value", Json::str(value)),
            ("seconds", Json::Int(d.signed().num_seconds())),
            ("negative", Json::Bool(d.is_negative())),
            ("error", Json::str("")),
        ]),
        Err(e) => Json::obj([
            ("value", Json::str(value)),
            ("seconds", Json::Null),
            ("negative", Json::Null),
            (
                "error",
                Json::str(classify_or_unclassified(&e, DURATION_CLASSES)),
            ),
        ]),
    }
}

/// Resolve every VALARM in the calendar, in document order: parent
/// components in calendar order, VALARMs in the order they appear
/// inside their parent.
fn alarm_entries(cal: &Calendar) -> Json {
    let mut out = Vec::new();
    for parent in &cal.components {
        for alarm in &parent.sub {
            if alarm.r#type != CompType::ALARM {
                continue;
            }
            let uid = alarm.uid();
            out.push(match helpers::alarm_fires_at(alarm, parent, cal) {
                Ok(at) => Json::obj([
                    ("alarm_uid", Json::str(uid)),
                    ("fires_at", Json::str(format_time(at))),
                    ("error", Json::str("")),
                ]),
                Err(e) => Json::obj([
                    ("alarm_uid", Json::str(uid)),
                    ("fires_at", Json::str("")),
                    (
                        "error",
                        Json::str(classify_or_unclassified(&e, TRIGGER_CLASSES)),
                    ),
                ]),
            });
        }
    }
    Json::Array(out)
}

/// Classify every name in the committed `ext/scopes.json`, in the order
/// the file lists them. The file is a flat table, not a set: order is
/// part of what a port reproduces.
pub fn emit_ext(dir: &Path) -> Json {
    let path = dir.join("scopes.json");
    let entries = table_rows(&path)
        .iter()
        .map(|r| {
            let name = row_str(&path, r, "name");
            let system = ext::system_name(&name).map_or(Json::Null, Json::Str);
            Json::obj([
                ("name", Json::str(&name)),
                ("scope", Json::str(scope_token(ext::scope_of(&name)))),
                ("system", system),
            ])
        })
        .collect();
    Json::obj([("ext/scopes", Json::Array(entries))])
}

/// The lowercase scope tokens the document records.
///
/// `Scope`'s `Display` is capitalized for human display; one flat
/// convention spares every port a case mapping of its own.
fn scope_token(scope: Scope) -> &'static str {
    match scope {
        Scope::None => "none",
        Scope::VStar => "vstar",
        Scope::System => "system",
        Scope::Experimental => "experimental",
        Scope::Unknown => "unknown",
    }
}

/// Resolve every (calendar, tzid, value) triple in the committed
/// `time/tzid.json`, in file order.
///
/// `calendar` names a conformance fixture supplying the VTIMEZONE
/// registry. Each registry is parsed once and reused, so a fixture's
/// cost does not grow with the number of rows citing it.
///
/// `utc` is null for every rejection: the API reports an absent value,
/// not an error, so there is no failure class to name here.
pub fn emit_time(conformance: &Path, dir: &Path) -> Json {
    let path = dir.join("tzid.json");
    let rows = table_rows(&path);

    let mut registries: BTreeMap<String, Calendar> = BTreeMap::new();
    for r in &rows {
        let name = row_str(&path, r, "calendar");
        registries.entry(name.clone()).or_insert_with(|| {
            read_calendar(&conformance.join("time").join(format!("{name}.ics")))
        });
    }

    let entries = rows
        .iter()
        .map(|r| {
            let calendar = row_str(&path, r, "calendar");
            let tzid = row_str(&path, r, "tzid");
            let value = row_str(&path, r, "value");
            let cal = &registries[&calendar];
            let utc = parse_time_with_tzid(&value, &tzid, cal)
                .map_or(Json::Null, |t| Json::str(format_time(t)));
            Json::obj([
                ("calendar", Json::str(calendar)),
                ("tzid", Json::str(tzid)),
                ("value", Json::str(value)),
                ("utc", utc),
            ])
        })
        .collect();
    Json::obj([("time/tzid", Json::Array(entries))])
}
