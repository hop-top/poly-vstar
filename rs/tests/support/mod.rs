// SPDX-License-Identifier: MIT

//! Shared fixture loader for the conformance corpus.
//!
//! The corpus is authored at `spec/v0.1/conformance/`, above the crate
//! root. Paths resolve from `CARGO_MANIFEST_DIR` so the loader works
//! from any working directory `cargo test` is invoked in.
//!
//! Nothing here hard-codes a fixture name: every helper walks the tree
//! and yields whatever is on disk, so a fixture added to the corpus is
//! picked up without touching this file.

#![allow(dead_code)]

pub mod json;

use std::fs;
use std::path::{Path, PathBuf};

/// Root of the conformance corpus.
pub fn corpus_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("spec")
        .join("v0.1")
        .join("conformance")
}

/// Root of the language-agnostic behavior fixtures at `spec/behavior/`.
pub fn behavior_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("spec")
        .join("behavior")
}

/// Reads and decodes a standalone behavior JSON, e.g. `time/tzid.json`.
pub fn behavior_json(family: &str, name: &str) -> json::Json {
    let path = behavior_root().join(family).join(name);
    let text = fs::read_to_string(&path).unwrap_or_else(|e| panic!("read {}: {e}", path.display()));
    json::parse(&text).unwrap_or_else(|e| panic!("parse {}: {e}", path.display()))
}

/// Reads the raw bytes of a behavior-family input document.
pub fn behavior_input(family: &str, name: &str) -> Vec<u8> {
    let path = behavior_root().join(family).join(name);
    fs::read(&path).unwrap_or_else(|e| panic!("read {}: {e}", path.display()))
}

/// Every stem in `spec/behavior/<family>/` whose file ends in `suffix`,
/// sorted.
///
/// The gates walk the tree rather than hard-coding names, so a fixture
/// added to the reference is picked up without a test edit — and one
/// that silently disappears shows up as a dropped case count rather
/// than a green run.
pub fn behavior_stems(family: &str, suffix: &str) -> Vec<String> {
    let dir = behavior_root().join(family);
    let entries =
        fs::read_dir(&dir).unwrap_or_else(|e| panic!("read behavior dir {}: {e}", dir.display()));
    let mut out: Vec<String> = entries
        .map(|e| e.expect("read dir entry").file_name())
        .filter_map(|n| n.to_str().map(str::to_owned))
        .filter_map(|n| n.strip_suffix(suffix).map(str::to_owned))
        .collect();
    assert!(
        !out.is_empty(),
        "no *{suffix} fixtures under {} — the loader found nothing to assert",
        dir.display()
    );
    out.sort();
    out
}

/// Renders a byte difference as the first mismatching offset plus a
/// window of context, so a failure names the divergence rather than
/// dumping two multi-hundred-byte blobs.
///
/// The offset is the load-bearing half: a fold-point divergence and an
/// NFC divergence look identical in a diff of decoded text and are told
/// apart instantly by where the first differing byte sits.
pub fn byte_diff_report(got: &[u8], want: &[u8]) -> String {
    let at = got
        .iter()
        .zip(want)
        .position(|(a, b)| a != b)
        .unwrap_or_else(|| got.len().min(want.len()));
    let lo = at.saturating_sub(32);
    let window = |b: &[u8]| -> String {
        let hi = (at + 32).min(b.len());
        if lo >= b.len() {
            return String::from("<past end of input>");
        }
        format!("{:?}", String::from_utf8_lossy(&b[lo..hi]))
    };
    format!(
        "first divergence at byte offset {at} (got {} bytes, want {} bytes)\n\
         got  [{lo}..]: {}\n\
         want [{lo}..]: {}\n\
         got  byte: {:?}\n\
         want byte: {:?}",
        got.len(),
        want.len(),
        window(got),
        window(want),
        got.get(at),
        want.get(at),
    )
}

/// Asserts byte-for-byte equality, reporting the first divergent offset.
pub fn assert_bytes_eq(got: &[u8], want: &[u8], label: &str) {
    assert!(got == want, "{label}: {}", byte_diff_report(got, want));
}

/// Root of the Go-reference parity bytes committed under `tests/parity/`.
pub fn parity_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests")
        .join("parity")
}

/// One corpus case: the stem, the input path, and the input bytes.
pub struct Case {
    /// Basename with the extension stripped.
    pub stem: String,
    /// Absolute path to the input file.
    pub path: PathBuf,
    /// Raw input bytes, exactly as they sit on disk (LF-terminated).
    pub input: Vec<u8>,
}

impl Case {
    /// Reads a sibling file with the given extension, if it exists.
    pub fn sibling(&self, ext: &str) -> Option<Vec<u8>> {
        let sib = self.path.with_extension(ext.trim_start_matches('.'));
        fs::read(sib).ok()
    }
}

/// Collects every file with `ext` directly inside `dir`, sorted by name.
///
/// Panics when the directory is missing or empty — an empty corpus
/// directory silently turns every gate below into a no-op, which is the
/// failure mode this assertion exists to prevent.
pub fn cases_in(dir: &Path, ext: &str) -> Vec<Case> {
    let mut out = Vec::new();
    let entries =
        fs::read_dir(dir).unwrap_or_else(|e| panic!("read corpus dir {}: {e}", dir.display()));
    for entry in entries {
        let path = entry.expect("read dir entry").path();
        if path.extension().and_then(|s| s.to_str()) != Some(ext) {
            continue;
        }
        let stem = path
            .file_stem()
            .and_then(|s| s.to_str())
            .expect("fixture stem is valid UTF-8")
            .to_owned();
        let input = fs::read(&path).unwrap_or_else(|e| panic!("read {}: {e}", path.display()));
        out.push(Case { stem, path, input });
    }
    assert!(
        !out.is_empty(),
        "no *.{ext} fixtures under {} — the loader found nothing to assert",
        dir.display()
    );
    out.sort_by(|a, b| a.stem.cmp(&b.stem));
    out
}

/// Recursively collects every file with `ext` under `dir`, sorted by path.
pub fn cases_under(dir: &Path, ext: &str) -> Vec<Case> {
    let mut out = Vec::new();
    walk(dir, ext, &mut out);
    assert!(
        !out.is_empty(),
        "no *.{ext} fixtures under {} — the loader found nothing to assert",
        dir.display()
    );
    out.sort_by(|a, b| a.path.cmp(&b.path));
    out
}

fn walk(dir: &Path, ext: &str, out: &mut Vec<Case>) {
    let entries = match fs::read_dir(dir) {
        Ok(e) => e,
        Err(e) => panic!("read corpus dir {}: {e}", dir.display()),
    };
    for entry in entries {
        let path = entry.expect("read dir entry").path();
        if path.is_dir() {
            walk(&path, ext, out);
            continue;
        }
        if path.extension().and_then(|s| s.to_str()) != Some(ext) {
            continue;
        }
        let stem = path
            .file_stem()
            .and_then(|s| s.to_str())
            .expect("fixture stem is valid UTF-8")
            .to_owned();
        let input = fs::read(&path).unwrap_or_else(|e| panic!("read {}: {e}", path.display()));
        out.push(Case { stem, path, input });
    }
}

/// Strips every `\r` that precedes an `\n` in `bytes`.
///
/// This is the one licensed transform between encoder output (CRLF) and
/// the LF-terminated corpus files. It is applied to **produced** bytes,
/// never to the file's — converting the file's LF up to CRLF would
/// silently repair an encoder that emitted a bare `\n`.
pub fn crlf_to_lf(bytes: &[u8]) -> Vec<u8> {
    let mut out = Vec::with_capacity(bytes.len());
    let mut i = 0;
    while i < bytes.len() {
        if bytes[i] == b'\r' && bytes.get(i + 1) == Some(&b'\n') {
            i += 1;
            continue;
        }
        out.push(bytes[i]);
        i += 1;
    }
    out
}

/// Reads a `malformed/<stem>.error` sibling and returns its sentinel name.
pub fn sentinel_name(case: &Case) -> String {
    let raw = case
        .sibling("error")
        .unwrap_or_else(|| panic!("{} has no .error sibling", case.stem));
    let text = String::from_utf8(raw).expect(".error file is valid UTF-8");
    text.lines()
        .map(str::trim)
        .find(|l| !l.is_empty())
        .unwrap_or_else(|| panic!("{}.error is empty", case.stem))
        .to_owned()
}
