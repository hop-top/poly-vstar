// SPDX-License-Identifier: MIT

//! The `validate`, `diff` and `supersession` families.
//!
//! All three read `spec/behavior/`, keying entries by the input's path
//! relative to that root with the extension dropped.

use std::collections::BTreeMap;
use std::path::Path;

use hop_top_vstar::diff::{self, ComponentDiff, DiffOp, PropertyDiff};
use hop_top_vstar::{supersession, validate, Param};

use super::corpus::{each_fixture, fail, parent_of, read_calendar, record, EXT_ICS};
use super::json::Json;

/// Run `validate` over every `<name>.ics` and emit the diagnostics it
/// raises, sorted by (path, code).
///
/// The sort is the contract, not any port's emission order: checks run
/// in an order that is an implementation detail. Sorting both sides
/// makes the comparison about which diagnostics were raised.
pub fn emit_validate(dir: &Path) -> Json {
    let root = parent_of(dir);
    let mut out = BTreeMap::new();
    for f in each_fixture(dir, EXT_ICS) {
        let mut rows: Vec<(String, &'static str, &'static str)> =
            validate::validate(&read_calendar(&f.path))
                .into_iter()
                .map(|d| (d.path, d.code, d.severity.as_str()))
                .collect();
        rows.sort_by(|a, b| a.0.cmp(&b.0).then_with(|| a.1.cmp(b.1)));
        let entries = rows
            .into_iter()
            .map(|(path, code, severity)| {
                Json::obj([
                    ("code", Json::str(code)),
                    ("severity", Json::str(severity)),
                    ("path", Json::str(path)),
                ])
            })
            .collect();
        record(&mut out, &root, &f.stem, Json::Array(entries));
    }
    Json::Object(out)
}

/// Run `of_calendar` over every `<name>.a.ics` / `<name>.b.ics` pair.
///
/// No sorting happens here. `of_calendar` emits components in pairing
/// order and properties sorted by name case-insensitively, and that
/// ordering is the contract a port reproduces — sorting it again would
/// hide an ordering divergence rather than catch it.
pub fn emit_diff(dir: &Path) -> Json {
    let root = parent_of(dir);
    let mut out = BTreeMap::new();
    for f in each_fixture(dir, ".a.ics") {
        let a = read_calendar(&f.path);
        let b = read_calendar(&f.sidecar(".b.ics"));
        record(
            &mut out,
            &root,
            &f.stem,
            component_diff_entries(&diff::of_calendar(&a, &b)),
        );
    }
    Json::Object(out)
}

/// Project component diffs onto the emitted shape, dropping sub-diffs
/// that record no change.
///
/// `of_calendar` filters those at the top level but carries them
/// nested; the document reports only real changes so a port need not
/// reproduce the reference's internal bookkeeping.
fn component_diff_entries(ds: &[ComponentDiff]) -> Json {
    Json::Array(
        ds.iter()
            .map(|d| {
                let subs: Vec<ComponentDiff> = d
                    .sub_diffs
                    .iter()
                    .filter(|s| !s.is_empty())
                    .cloned()
                    .collect();
                Json::obj([
                    ("uid", Json::str(uid_from_path(&d.path))),
                    ("path", Json::str(&d.path)),
                    ("ops", op_entries(&d.properties)),
                    ("subs", component_diff_entries(&subs)),
                ])
            })
            .collect(),
    )
}

/// Project property diffs onto the emitted shape.
///
/// `before` is null on an add, `after` null on a remove. Params travel
/// apart from the value because a parameter-only change is a `change`
/// op whose values are equal and whose parameter lists differ.
fn op_entries(pds: &[PropertyDiff]) -> Json {
    Json::Array(
        pds.iter()
            .map(|pd| {
                let (before, after, before_params, after_params) = match pd.op {
                    DiffOp::Added => (
                        Json::Null,
                        Json::str(&pd.property.value),
                        empty_params(),
                        param_entries(&pd.property.params),
                    ),
                    DiffOp::Removed => (
                        Json::str(&pd.property.value),
                        Json::Null,
                        param_entries(&pd.property.params),
                        empty_params(),
                    ),
                    DiffOp::Changed => (
                        Json::str(&pd.old.value),
                        Json::str(&pd.property.value),
                        param_entries(&pd.old.params),
                        param_entries(&pd.property.params),
                    ),
                };
                Json::obj([
                    ("op", Json::str(op_token(pd.op))),
                    ("property", Json::str(property_name(pd))),
                    ("before", before),
                    ("after", after),
                    ("before_params", before_params),
                    ("after_params", after_params),
                ])
            })
            .collect(),
    )
}

/// The lowercase op tokens the document records.
///
/// `DiffOp`'s `Display` is capitalized for human display; one flat
/// convention here spares every port a case mapping.
fn op_token(op: DiffOp) -> &'static str {
    match op {
        DiffOp::Added => "add",
        DiffOp::Removed => "remove",
        DiffOp::Changed => "change",
    }
}

/// Report the name the op is about, from whichever side carries it.
///
/// Rust's `PropertyDiff::old` is a plain `Property` rather than an
/// option, matching Go's zero-valued struct field: an add carries an
/// empty `old`, so the fallback reads the same in both.
fn property_name(pd: &PropertyDiff) -> &str {
    if pd.property.name.is_empty() {
        &pd.old.name
    } else {
        &pd.property.name
    }
}

fn param_entries(params: &[Param]) -> Json {
    Json::Array(
        params
            .iter()
            .map(|p| Json::obj([("name", Json::str(&p.name)), ("value", Json::str(&p.value))]))
            .collect(),
    )
}

/// An empty parameter list. Always `[]`, never `null`, so every port's
/// decoder handles one shape.
fn empty_params() -> Json {
    Json::Array(Vec::new())
}

/// Lift the uid out of a rendered path such as
/// `VCALENDAR.VTODO[uid=todo-1]`.
///
/// Empty for a positional path (`VCALENDAR.VALARM[#0]`), which has no
/// UID to key on.
fn uid_from_path(path: &str) -> &str {
    const MARKER: &str = "[uid=";
    let Some(i) = path.rfind(MARKER) else {
        return "";
    };
    if !path.ends_with(']') {
        return "";
    }
    &path[i + MARKER.len()..path.len() - 1]
}

/// Project the effective status each supersession ledger imposes,
/// keyed by component UID.
///
/// The inputs are the conformance corpus' supersession fixtures; the
/// behavior tree holds only the `<name>.effective.json` sidecars, so
/// the sidecar names which `.ics` to load. A UID absent from the map is
/// not superseded — supersession is a projection query, not a
/// validator.
pub fn emit_supersession(conformance: &Path, dir: &Path) -> Json {
    let root = parent_of(dir);
    let inputs = conformance.join("supersession");
    let mut out = BTreeMap::new();
    for f in each_fixture(dir, ".effective.json") {
        let name = f
            .stem
            .file_name()
            .and_then(|n| n.to_str())
            .unwrap_or_else(|| fail(format!("{} has no basename", f.stem.display())));
        let cal = read_calendar(&inputs.join(format!("{name}.ics")));
        let mut effective = BTreeMap::new();
        for c in &cal.components {
            let Some(status) = supersession::superseded(c, &cal.components) else {
                continue;
            };
            let uid = c.uid();
            if uid.is_empty() {
                fail(format!("{name}: superseded component has no UID"));
            }
            effective.insert(uid.to_owned(), Json::str(status));
        }
        record(&mut out, &root, &f.stem, Json::Object(effective));
    }
    Json::Object(out)
}
