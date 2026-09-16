// SPDX-License-Identifier: MIT

//! Corpus walking, fixture keying, and failure classification — the
//! machinery every family shares.
//!
//! Nothing here hard-codes a fixture name. Keys are discovered by
//! walking the tree, so a fixture added to `spec/` reaches this
//! emitter without an edit, which is what `tools/parity/README.md`
//! requires of every port.

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::process;

use hop_top_vstar::codec::rfc5545;
use hop_top_vstar::{Calendar, Error};

use super::json::{self, Json};

/// Fixture extensions, named once so the families read alike.
pub const EXT_ICS: &str = ".ics";
/// The vCard fixture extension.
pub const EXT_VCF: &str = ".vcf";
/// The bare-rule fixture extension.
pub const EXT_RRULE: &str = ".rrule";

/// The general class every codec layer may wrap; always tried last.
pub const CLASS_MALFORMED: &str = "ErrMalformed";

/// The escape hatch for a failure matching no class the family
/// recognizes. It surfaces under a name no port implements — a loud
/// mismatch — rather than silently under a wrong token.
pub const CLASS_UNCLASSIFIED: &str = "Unclassified";

/// Abort with a diagnostic on stderr, mirroring the reference's exit 1.
///
/// Nothing but the document is written to stdout, so a partially built
/// document never reaches the harness as truncated JSON.
pub fn fail(message: impl AsRef<str>) -> ! {
    eprintln!("parity: {}", message.as_ref());
    process::exit(1);
}

/// Classify a failure into its cross-language token, first-match over
/// `table` ordered specific to general.
///
/// The Go reference matches with `errors.Is`, which walks a wrap chain.
/// This port carries the class identifier natively on every [`Error`]
/// as [`Error::sentinel`], so membership in the table is the whole
/// test. A failure whose sentinel the family does not list reports
/// `None`, and the caller turns that into a diagnostic rather than an
/// emitted token no port would recognize.
pub fn classify(err: &Error, table: &[&str]) -> Option<&'static str> {
    let got = err.sentinel();
    table.iter().find(|t| **t == got).map(|_| got)
}

/// Classify a failure, falling back to [`CLASS_UNCLASSIFIED`].
///
/// Used by the families whose contract names that token — duration
/// parsing and alarm resolution — where an unrecognized class travels
/// into the document instead of aborting the emitter.
pub fn classify_or_unclassified(err: &Error, table: &[&str]) -> &'static str {
    classify(err, table).unwrap_or(CLASS_UNCLASSIFIED)
}

/// Read a fixture as raw bytes, aborting with a diagnostic on failure.
pub fn read_bytes(path: &Path) -> Vec<u8> {
    fs::read(path).unwrap_or_else(|e| fail(format!("read {}: {e}", path.display())))
}

/// Read a fixture as UTF-8 text.
pub fn read_text(path: &Path) -> String {
    String::from_utf8(read_bytes(path))
        .unwrap_or_else(|e| fail(format!("{} is not valid UTF-8: {e}", path.display())))
}

/// Read and parse a JSON sidecar.
pub fn read_json(path: &Path) -> Json {
    json::parse(&read_text(path)).unwrap_or_else(|e| fail(format!("parse {}: {e}", path.display())))
}

/// Read a sidecar's input fields, reporting `None` when it does not
/// exist.
pub fn read_sidecar(path: &Path) -> Option<Json> {
    path.exists().then(|| read_json(path))
}

/// Parse an `.ics` fixture into a [`Calendar`], failing the emitter on
/// error.
pub fn read_calendar(path: &Path) -> Calendar {
    rfc5545::parse(read_bytes(path).as_slice())
        .unwrap_or_else(|e| fail(format!("parse {}: {e}", path.display())))
}

/// One fixture the walk yielded: its full path, and the same path with
/// the extension stripped.
pub struct Fixture {
    /// Absolute path to the fixture file.
    pub path: PathBuf,
    /// `path` with the family's extension removed, which every sidecar
    /// name is built from.
    pub stem: PathBuf,
}

impl Fixture {
    /// The path of a sidecar named by appending `suffix` to the stem.
    pub fn sidecar(&self, suffix: &str) -> PathBuf {
        let mut name = self.stem.as_os_str().to_owned();
        name.push(suffix);
        PathBuf::from(name)
    }
}

/// Every non-directory entry of `dir` ending in `ext`, sorted by name.
///
/// A missing directory is a silent empty result, so the emitter
/// tolerates a corpus that has yet to grow a family — the same
/// tolerance the reference's `eachFixture` has.
pub fn each_fixture(dir: &Path, ext: &str) -> Vec<Fixture> {
    let Ok(entries) = fs::read_dir(dir) else {
        return Vec::new();
    };
    let mut names: Vec<PathBuf> = entries
        .map(|e| e.unwrap_or_else(|err| fail(format!("read {}: {err}", dir.display()))))
        .filter(|e| !e.path().is_dir())
        .map(|e| e.path())
        .filter(|p| path_ends_with(p, ext))
        .collect();
    names.sort();
    names.into_iter().map(|p| fixture(p, ext)).collect()
}

/// Every file under `root`, recursively, sorted by path.
///
/// The rrule corpus is the one nested tree. Sorting the collected
/// paths rather than trusting traversal order keeps the walk
/// independent of the platform's directory order.
pub fn walk_files(root: &Path) -> Vec<PathBuf> {
    let mut out = Vec::new();
    visit(root, &mut out);
    out.sort();
    out
}

fn visit(dir: &Path, out: &mut Vec<PathBuf>) {
    let Ok(entries) = fs::read_dir(dir) else {
        return;
    };
    for entry in entries {
        let path = entry
            .unwrap_or_else(|e| fail(format!("read {}: {e}", dir.display())))
            .path();
        if path.is_dir() {
            visit(&path, out);
        } else {
            out.push(path);
        }
    }
}

/// Pair a path with its extension-stripped stem.
pub fn fixture(path: PathBuf, ext: &str) -> Fixture {
    let text = path_str(&path);
    let stem = PathBuf::from(&text[..text.len() - ext.len()]);
    Fixture { path, stem }
}

/// Whether `path` ends with `suffix`, compared over the whole rendered
/// path so multi-part extensions (`.a.ics`, `.effective.json`) work.
pub fn path_ends_with(path: &Path, suffix: &str) -> bool {
    path_str(path).ends_with(suffix)
}

/// Render a path as UTF-8, aborting rather than lossily converting: a
/// fixture key is a contract value, not a display string.
pub fn path_str(path: &Path) -> String {
    path.to_str()
        .unwrap_or_else(|| fail(format!("path is not valid UTF-8: {}", path.display())))
        .to_owned()
}

/// Render `stem` as a slash-separated path relative to `root`.
///
/// The document's key space is filesystem layout with the extension
/// dropped, identical on every platform a port runs on.
pub fn fixture_key(root: &Path, stem: &Path) -> String {
    let rel = stem.strip_prefix(root).unwrap_or_else(|_| {
        fail(format!(
            "relativize {} against {}",
            stem.display(),
            root.display()
        ))
    });
    rel.components()
        .map(|c| path_str(Path::new(c.as_os_str())))
        .collect::<Vec<_>>()
        .join("/")
}

/// Key `value` by `stem` relative to `root`, refusing a collision.
///
/// Two fixtures colliding on one key would silently drop a case from
/// the document and shrink the harness' coverage without failing it.
pub fn record(out: &mut BTreeMap<String, Json>, root: &Path, stem: &Path, value: Json) {
    let key = fixture_key(root, stem);
    if out.contains_key(&key) {
        fail(format!("duplicate fixture key {key:?}"));
    }
    out.insert(key, value);
}

/// The parent of `dir`, which every behavior family keys against.
pub fn parent_of(dir: &Path) -> PathBuf {
    dir.parent()
        .unwrap_or_else(|| fail(format!("{} has no parent", dir.display())))
        .to_path_buf()
}

/// Read the `column` column out of a committed flat table.
///
/// The flat families (`ext/scopes`, `time/tzid`, `duration/parse`)
/// read their inputs from the corpus rather than from a list hard-coded
/// here, so a row added to the corpus reaches every port without an
/// emitter edit.
pub fn table_rows(path: &Path) -> Vec<Json> {
    let doc = read_json(path);
    doc.as_array()
        .unwrap_or_else(|| fail(format!("{}: expected an array of rows", path.display())))
        .to_vec()
}

/// Read a required string column from one table row.
pub fn row_str(path: &Path, row: &Json, column: &str) -> String {
    row.str_field(column)
        .unwrap_or_else(|| fail(format!("{}: row has no {column:?} string", path.display())))
        .to_owned()
}
