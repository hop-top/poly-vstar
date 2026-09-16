// SPDX-License-Identifier: MIT

//! The Rust parity emitter for the cross-language parity harness.
//!
//! It takes the `spec/` directory as its single argument, runs the V\*
//! public API over every fixture in `spec/v0.1/conformance/` and
//! `spec/behavior/`, and prints ONE JSON document to stdout.
//! `tools/parity/parity.py` diffs this document against the Go
//! reference's, key by key; any difference fails the run.
//!
//! The normative description of the document — every key, every value
//! shape, every ordering rule — is `tools/parity/README.md`. This
//! command implements that document, not the Go emitter's source.
//!
//! Three rules shape every value:
//!
//! - Nothing human-readable is emitted. Diagnostic messages and error
//!   text reword between versions without the behavior changing.
//!   Codes, paths, severities, op kinds and failure-class tokens are
//!   the stable surface.
//! - A failure serializes as `{"error": "<SentinelName>"}` using the
//!   Go sentinel's identifier as the cross-language token. This port
//!   carries those identifiers natively as [`Error::sentinel`], so
//!   classification is a lookup rather than a translation.
//! - Output is deterministic. Every emitted object is a `BTreeMap`,
//!   which sorts its keys the way Go's `encoding/json` sorts a map's;
//!   every list is built in an order the contract pins.
//!
//! Usage:
//!
//! ```sh
//! cargo run --quiet --bin parity -- ../spec
//! ```
//!
//! Exits 0 having printed the document; exits 1 with a diagnostic on
//! stderr when a fixture cannot be read or a contract it depends on is
//! broken.
//!
//! [`Error::sentinel`]: hop_top_vstar::Error::sentinel

// The emitter is a development tool, not part of the published
// library's API, so the crate-level `missing_docs` contract does not
// reach it. Every item is documented regardless.
#![forbid(unsafe_code)]

mod behavior;
mod conformance;
mod corpus;
mod json;
mod rrule;
mod tables;

use std::io::Write as _;
use std::path::{Path, PathBuf};

use corpus::fail;
use json::Json;

fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let [spec] = args.as_slice() else {
        fail("usage: parity <spec-dir>");
    };
    let spec = absolute(Path::new(spec));

    let conformance = spec.join("v0.1").join("conformance");
    let behavior_dir = spec.join("behavior");
    for dir in [&conformance, &behavior_dir] {
        if !dir.is_dir() {
            fail(format!("not a directory: {}", dir.display()));
        }
    }

    // The eight top-level keys the contract requires. Every key is
    // always present — a port that implements one family still emits
    // the other seven as empty objects, so a missing key is a
    // structural fault rather than a coverage gap.
    let document = Json::obj([
        ("conformance", conformance::emit(&conformance)),
        ("rrule", rrule::emit(&conformance.join("rrule"))),
        (
            "validate",
            behavior::emit_validate(&behavior_dir.join("validate")),
        ),
        ("diff", behavior::emit_diff(&behavior_dir.join("diff"))),
        (
            "supersession",
            behavior::emit_supersession(&conformance, &behavior_dir.join("supersession")),
        ),
        (
            "duration",
            tables::emit_duration(&behavior_dir.join("duration")),
        ),
        ("ext", tables::emit_ext(&behavior_dir.join("ext"))),
        (
            "time",
            tables::emit_time(&conformance, &behavior_dir.join("time")),
        ),
    ]);

    let rendered = json::render(&document);
    let mut stdout = std::io::stdout().lock();
    if let Err(e) = stdout
        .write_all(rendered.as_bytes())
        .and_then(|()| stdout.flush())
    {
        fail(format!("write document: {e}"));
    }
}

/// Resolve `path` against the working directory.
///
/// `std::path::absolute` is 1.79+, comfortably inside the crate's 1.98
/// MSRV, and unlike `canonicalize` it does not resolve symlinks — the
/// harness passes an already-absolute path, and a symlinked corpus
/// should key by the name it was given.
fn absolute(path: &Path) -> PathBuf {
    std::path::absolute(path).unwrap_or_else(|e| fail(format!("resolve {}: {e}", path.display())))
}
