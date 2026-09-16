// SPDX-License-Identifier: MIT

//! The V\* semantic invariants the codec layer cannot catch.
//!
//! A document can be syntactically valid RFC 5545 or RFC 6350, so that
//! parsing succeeds, and still violate V\* discipline: a missing or
//! corrupted `X-VSTAR-HASH`, a property outside the extension
//! namespace, a type-specific required property absent, a `STATUS` from
//! the wrong vocabulary, a malformed recurrence or duration.
//!
//! The two entry points are [`validate`] (a whole [`Calendar`]) and
//! [`validate_component`] (one [`Component`]). **Neither returns a
//! `Result`.** Validation *is* the error channel: a document that fails
//! every check still validates successfully and returns a list of
//! findings. An empty list means clean. A port that fails on a
//! diagnostic has inverted the API.
//!
//! Every [`Diagnostic`] carries a stable `code` — cataloged in
//! `docs/validate-codes.md` and generated from `spec/registry/` — that
//! consumers may match programmatically. `message` is human-readable
//! and may change in any release; do not match on it.
//!
//! # Coverage of spec/05, the conformance criteria
//!
//! - §1 required common properties (`UID`, `DTSTAMP`, `X-VSTAR-HASH`).
//! - §2 `X-VSTAR-HASH` integrity — the present-but-wrong case; an
//!   absent hash is reported by §1 instead.
//! - §3 extension namespace compliance — a non-standard property name
//!   without the `X-` prefix.
//! - §4 supersession discipline — a supersession VJOURNAL's required
//!   properties, and whether its `RELATED-TO` resolves.
//! - §5 type-specific required properties — VTODO, VEVENT, VFREEBUSY,
//!   VCARD.
//! - §6 RRULE conformance — unsupported (warning) and malformed
//!   (error).
//! - §7 DURATION well-formedness — the `DURATION` property, the
//!   relative form of `TRIGGER`, and the `REPEAT` count.
//! - §8 the `STATUS` value domain. The other domains §8 bounds
//!   (`CLASS`, `TRANSP`, `PRIORITY`, `PERCENT-COMPLETE`, `SEQUENCE`,
//!   `REPEAT`) have no code yet; the `helpers` accessors enforce them
//!   at write time.
//!
//! # Path syntax
//!
//! [`Diagnostic::path`] is a dotted component/property locator:
//!
//! | Path | Meaning |
//! |---|---|
//! | `VCALENDAR` | Calendar-level. |
//! | `VCALENDAR.VTODO[uid=foo]` | Component-level, on the VTODO whose UID is `foo`. |
//! | `VCALENDAR.VTODO[uid=foo].DTSTAMP` | Property-level, on that VTODO's DTSTAMP. |
//! | `VCALENDAR.VTODO[#3]` | A UID-less VTODO, at positional index 3. |
//! | `VTODO[uid=foo].DTSTAMP` | [`validate_component`] — no calendar prefix. |
//!
//! # Example
//!
//! ```
//! use hop_top_vstar::codec::rfc5545;
//! use hop_top_vstar::validate::validate;
//!
//! let src = "BEGIN:VCALENDAR\r\n\
//!            VERSION:2.0\r\n\
//!            PRODID:-//Example//EN\r\n\
//!            BEGIN:VEVENT\r\n\
//!            UID:evt-1\r\n\
//!            END:VEVENT\r\n\
//!            END:VCALENDAR\r\n";
//! let cal = rfc5545::parse(src.as_bytes())?;
//!
//! // No DTSTAMP, no X-VSTAR-HASH, no DTSTART — three findings.
//! let found = validate(&cal);
//! assert_eq!(found.len(), 3);
//! assert!(found.iter().all(|d| d.path.starts_with("VCALENDAR.VEVENT[uid=evt-1]")));
//! # Ok::<(), Box<dyn std::error::Error>>(())
//! ```

mod duration;
mod extensions;
mod integrity;
mod required;
mod rrule;
mod status;
mod supersession;

use crate::generated::codes::{CODES, CODE_SEVERITIES};
use crate::{Calendar, CompType, Component};
use std::collections::BTreeMap;
use std::fmt;

/// The property name V\* uses to carry the content hash.
///
/// Deliberately a module-local constant rather than an import from
/// `hashing`: `validate` inspects raw property names without taking on
/// the hashing module's identity, and the two agreeing is asserted by
/// the fixtures.
const X_VSTAR_HASH: &str = "X-VSTAR-HASH";

/// The impact level of a [`Diagnostic`].
///
/// `Error` marks a MUST violation: the document is not V\* conformant.
/// `Warning` marks a SHOULD violation or a stylistic concern.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Severity {
    /// A MUST violation — the document is not V\* conformant.
    Error,
    /// A SHOULD violation or stylistic concern.
    Warning,
}

impl Severity {
    /// The lowercase wire name, `"error"` or `"warning"`.
    ///
    /// This is the spelling the registry uses and the behavior fixtures
    /// pin, so it is contract rather than presentation.
    pub const fn as_str(&self) -> &'static str {
        match self {
            Severity::Error => "error",
            Severity::Warning => "warning",
        }
    }

    /// Parses a registry severity name, or `None` when unrecognized.
    fn from_wire(s: &str) -> Option<Severity> {
        match s {
            "error" => Some(Severity::Error),
            "warning" => Some(Severity::Warning),
            _ => None,
        }
    }
}

impl fmt::Display for Severity {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// A single validation finding.
///
/// `code` is the stable catalog identifier cataloged in
/// `docs/validate-codes.md`; it is stable across minor versions per
/// semver, so consumers may match on it programmatically.
///
/// `message` is human-readable detail and is **not** stable across
/// versions. Match `code` instead — the behavior fixtures deliberately
/// omit messages for exactly this reason.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Diagnostic {
    /// The stable catalog identifier, borrowed from the generated
    /// registry table so a diagnostic cannot carry an invented code.
    pub code: &'static str,
    /// `Error` for a MUST violation, `Warning` for a SHOULD.
    pub severity: Severity,
    /// The dotted component/property locator (see the module doc).
    pub path: String,
    /// Human-readable detail. Not stable; do not match on it.
    pub message: String,
}

/// Builds a [`Diagnostic`], taking its severity from the generated
/// registry rather than from the call site.
///
/// Severity is registry data, not a per-rule decision: a rule that
/// spelled its own severity would be a second source of truth, free to
/// drift the moment `spec/registry/diagnostic-codes.json` changes. The
/// only way to change a severity here is to change the registry and
/// re-run the generator.
///
/// # Panics
///
/// Panics when `code` is absent from the generated severity table. That
/// is unreachable for a code taken from the generated module, and the
/// panic is preferable to inventing a severity for an unknown code.
fn diagnostic(code: &'static str, message: String, path: String) -> Diagnostic {
    let severity = severity_of(code)
        .unwrap_or_else(|| panic!("{code} is not in the generated severity table"));
    Diagnostic {
        code,
        severity,
        path,
        message,
    }
}

/// Checks every component in `cal` and returns the accumulated
/// diagnostics. An empty vector means `cal` is clean.
///
/// `cal` is not mutated. Diagnostics follow component order, then rule
/// order within a component; consumers that compare against a fixture
/// should sort, since only the set — not the emission order — is
/// contractual.
///
/// Prefer this over [`validate_component`] whenever a calendar exists:
/// the paths are more precise, and the cross-component rule (the orphan
/// supersession check) can only run here.
pub fn validate(cal: &Calendar) -> Vec<Diagnostic> {
    let mut out = Vec::new();
    let mut index: PathIndex = BTreeMap::new();
    for comp in &cal.components {
        let path = format!("VCALENDAR.{}", component_path(comp, &mut index));
        out.extend(run_checks(comp, &path, Some(&cal.components)));
    }
    out
}

/// Checks a single [`Component`] in isolation. Paths start at the
/// component itself, e.g. `VTODO[uid=foo].DTSTAMP`.
///
/// This is the right entry point when there is no parent calendar — a
/// freshly minted component, say, checked before it is appended. The
/// cross-component orphan-supersession rule is skipped, not guessed:
/// with no ledger to resolve against, "this target does not exist" is a
/// claim this entry point cannot make.
pub fn validate_component(c: &Component) -> Vec<Diagnostic> {
    let mut index: PathIndex = BTreeMap::new();
    let path = component_path(c, &mut index);
    run_checks(c, &path, None)
}

/// Every registry diagnostic code, in registry order.
pub fn codes() -> Vec<&'static str> {
    CODES.to_vec()
}

/// The severity the registry assigns `code`, or `None` when the string
/// names no known code.
///
/// Consumers must tolerate unknown codes — new ones may appear in any
/// release — so this reports absence rather than panicking.
pub fn severity_of(code: &str) -> Option<Severity> {
    CODE_SEVERITIES
        .iter()
        .find(|(c, _)| *c == code)
        .and_then(|(_, s)| Severity::from_wire(s))
}

/// How many properties the generated RFC allow-list carries.
pub fn standard_property_count() -> usize {
    extensions::standard_property_count()
}

/// Runs every rule against `c` at `path`.
///
/// `ledger` supplies the cross-component context the orphan-supersession
/// rule needs; `None` means there is none and that rule is skipped.
fn run_checks(c: &Component, path: &str, ledger: Option<&[Component]>) -> Vec<Diagnostic> {
    let mut out = Vec::new();
    out.extend(required::check(c, path));
    out.extend(integrity::check(c, path));
    out.extend(extensions::check(c, path));
    out.extend(check_type_specific(c, path));
    out.extend(status::check(c, path));
    out.extend(supersession::check(c, ledger, path));
    out.extend(rrule::check(c, path));
    out.extend(duration::check(c, path));
    out
}

/// Running positional index per component type, for UID-less paths.
type PathIndex = BTreeMap<String, usize>;

/// The path segment identifying `c` relative to its container:
/// `<TYPE>[uid=<uid>]`, or `<TYPE>[#<n>]` when `c` has no UID.
///
/// `index` carries the running positional counter per type so that two
/// UID-less components of the same type get distinct, stable segments.
fn component_path(c: &Component, index: &mut PathIndex) -> String {
    let uid = c.uid();
    if !uid.is_empty() {
        return format!("{}[uid={uid}]", c.r#type.as_str());
    }
    let ty = c.r#type.as_str().to_owned();
    let n = index.entry(ty.clone()).or_insert(0);
    let seg = format!("{ty}[#{n}]");
    *n += 1;
    seg
}

/// The `VCARD` wire type, as it appears on a [`Component`].
///
/// [`CompType`] excludes it deliberately: a top-level vCard is a
/// [`Card`](crate::Card). A `VCARD` block nested inside a `VCALENDAR`
/// still parses into a `Component` carrying the wire string, and the
/// rule below exists so a caller hand-building one still gets the check.
const VCARD: &str = "VCARD";

/// spec/05 §5 — the per-type required properties.
///
/// Component types with no extra MUST in spec/05 §5 — VJOURNAL,
/// VTIMEZONE, VALARM, VCALENDAR — yield nothing.
fn check_type_specific(c: &Component, path: &str) -> Vec<Diagnostic> {
    if c.r#type == CompType::TODO {
        return check_vtodo(c, path);
    }
    if c.r#type == CompType::EVENT {
        return check_vevent(c, path);
    }
    if c.r#type == CompType::FREE_BUSY {
        return check_vfreebusy(c, path);
    }
    if c.r#type.as_str() == VCARD {
        return check_vcard_component(c, path);
    }
    Vec::new()
}

/// A VTODO must be reachable as "scheduled": either `DUE` is present,
/// or `STATUS=COMPLETED` is paired with a `COMPLETED` timestamp.
///
/// RFC 5545 §3.6.2 lets a finished VTODO drop `DUE` so long as
/// `COMPLETED` records when it finished; that route is honoured rather
/// than demanding a due date the task no longer has.
fn check_vtodo(c: &Component, path: &str) -> Vec<Diagnostic> {
    if c.get("DUE").is_some() {
        return Vec::new();
    }
    if let Some(status) = c.get("STATUS") {
        if status
            .value
            .eq_ignore_ascii_case(crate::TodoStatus::Completed.as_str())
            && c.get("COMPLETED").is_some()
        {
            return Vec::new();
        }
    }
    vec![diagnostic(
        crate::generated::codes::VTODO_MISSING_DUE,
        "VTODO requires DUE, or STATUS=COMPLETED paired with COMPLETED (spec/05 §5; RFC 5545 §3.6.2)".to_owned(),
        path.to_owned(),
    )]
}

/// A VEVENT must carry `DTSTART`. RFC 5545 §3.6.1 allows its absence
/// outside a PUBLISH METHOD context; V\* is strict, because an agentic
/// playthrough always anchors to a start time.
fn check_vevent(c: &Component, path: &str) -> Vec<Diagnostic> {
    if c.get("DTSTART").is_some() {
        return Vec::new();
    }
    vec![diagnostic(
        crate::generated::codes::VEVENT_MISSING_DTSTART,
        "VEVENT requires DTSTART (spec/05 §5; RFC 5545 §3.6.1)".to_owned(),
        format!("{path}.DTSTART"),
    )]
}

/// A VFREEBUSY must carry both `DTSTART` and `DTEND`; the message names
/// which is missing.
fn check_vfreebusy(c: &Component, path: &str) -> Vec<Diagnostic> {
    let missing = required_missing(c, &["DTSTART", "DTEND"]);
    if missing.is_empty() {
        return Vec::new();
    }
    vec![diagnostic(
        crate::generated::codes::VFREEBUSY_MISSING_TIMES,
        format!(
            "VFREEBUSY requires DTSTART and DTEND; missing: {} (spec/05 §5; RFC 5545 §3.6.4)",
            missing.join(", ")
        ),
        path.to_owned(),
    )]
}

/// A VCARD-as-Component must carry both `VERSION` and `UID`.
fn check_vcard_component(c: &Component, path: &str) -> Vec<Diagnostic> {
    let missing = required_missing(c, &["VERSION", "UID"]);
    if missing.is_empty() {
        return Vec::new();
    }
    vec![diagnostic(
        crate::generated::codes::VCARD_MISSING_REQUIRED,
        format!(
            "VCARD requires VERSION and UID; missing: {} (spec/05 §5; RFC 6350 §6.7.6, §6.7.9)",
            missing.join(", ")
        ),
        path.to_owned(),
    )]
}

/// Which of `names` `c` lacks, in the order given.
fn required_missing(c: &Component, names: &[&'static str]) -> Vec<&'static str> {
    names
        .iter()
        .copied()
        .filter(|name| c.get(name).is_none())
        .collect()
}
