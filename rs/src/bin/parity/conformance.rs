// SPDX-License-Identifier: MIT

//! The `conformance` family: content hashes, canonical digests, and
//! the malformed corpus' failure tokens.

use std::collections::BTreeMap;
use std::path::Path;

use hop_top_vstar::codec::{rfc5545, rfc6350};
use hop_top_vstar::{canonical, hashing, Card};
use sha2::{Digest, Sha256};

use super::corpus::{
    each_fixture, fail, read_bytes, read_text, record, Fixture, CLASS_MALFORMED, EXT_ICS, EXT_VCF,
};
use super::json::Json;

/// The vocabulary a `malformed/*` fixture may produce, most specific
/// first. The emitter reports which one the fixture actually produced;
/// this list only bounds what behavior is recognized.
const MALFORMED_CLASSES: &[&str] = &[
    "ErrUnsupportedVersion",
    "ErrUnclosedBlock",
    "ErrMissingUID",
    CLASS_MALFORMED,
];

/// The conformance subdirectories whose `.ics` files parse into a
/// Calendar and carry `.canonical`/`.hash` siblings.
const CALENDAR_DIRS: &[&str] = &["rfc5545", "supersession"];

/// Walk the conformance corpus, keying every fixture by its path
/// relative to the corpus root without extension.
///
/// `time/` fixtures are VTIMEZONE registries the `time` family
/// consumes and carry no `.canonical`/`.hash` siblings, so they are
/// not part of this family; `fuzz-seed/` is fuzz-target input.
pub fn emit(corpus: &Path) -> Json {
    let mut out = BTreeMap::new();

    for dir in CALENDAR_DIRS {
        for f in each_fixture(&corpus.join(dir), EXT_ICS) {
            let entry = calendar_entry(&f);
            record(&mut out, corpus, &f.stem, entry);
        }
    }
    for f in each_fixture(&corpus.join("rfc6350"), EXT_VCF) {
        let entry = card_entry(&f);
        record(&mut out, corpus, &f.stem, entry);
    }
    let malformed = corpus.join("malformed");
    for ext in [EXT_ICS, EXT_VCF] {
        for f in each_fixture(&malformed, ext) {
            let token = malformed_token(&f, ext);
            record(
                &mut out,
                corpus,
                &f.stem,
                Json::obj([("error", Json::str(token))]),
            );
        }
    }
    Json::Object(out)
}

/// Parse an `.ics` fixture, canonicalize, hash, and self-check.
fn calendar_entry(f: &Fixture) -> Json {
    let cal = rfc5545::parse(read_bytes(&f.path).as_slice())
        .unwrap_or_else(|e| fail(format!("parse {}: {e}", f.path.display())));
    let hash = hashing::calendar(&cal);
    let entry = Json::obj([
        ("hash", Json::str(&hash)),
        (
            "canonical_sha256",
            Json::str(canonical_digest(&canonical::calendar(&cal))),
        ),
    ]);
    check_hash_sibling(f, &hash);
    entry
}

/// [`calendar_entry`] for a `.vcf` fixture.
///
/// The corpus holds exactly one card per file; more would mean the
/// fixture changed shape under the emitter and the contract no longer
/// says which card the hash belongs to.
fn card_entry(f: &Fixture) -> Json {
    let cards = rfc6350::parse(read_bytes(&f.path).as_slice())
        .unwrap_or_else(|e| fail(format!("parse {}: {e}", f.path.display())));
    let [card] = cards.as_slice() else {
        fail(format!(
            "{}: expected exactly one card, got {}",
            f.path.display(),
            cards.len()
        ));
    };
    let hash = hashing::card(card);
    let entry = Json::obj([
        ("hash", Json::str(&hash)),
        (
            "canonical_sha256",
            Json::str(canonical_digest(&canonical::card(card))),
        ),
    ]);
    check_hash_sibling(f, &hash);
    entry
}

/// Compare a computed hash against the committed `<stem>.hash` sibling.
///
/// This self-check is what keeps a port from emitting a hash that is
/// merely self-consistent: the corpus carries an independent
/// expectation for this one family, and disagreeing with it is a
/// broken port rather than a parity mismatch to report downstream.
fn check_hash_sibling(f: &Fixture, got: &str) {
    let sibling = f.sidecar(".hash");
    let want = read_text(&sibling)
        .trim_end_matches(['\r', '\n'])
        .to_owned();
    if got != want {
        fail(format!(
            "{}: hash {got} does not match committed {want}",
            sibling.display()
        ));
    }
}

/// Hash canonical bytes after folding CRLF to LF.
///
/// The canonical form per spec/03 is CRLF, but the corpus stores it LF
/// and every port reads the same LF file. Digesting the LF form keeps
/// line-ending handling from producing false mismatches between ports.
fn canonical_digest(bytes: &[u8]) -> String {
    let mut hasher = Sha256::new();
    hasher.update(crlf_to_lf(bytes));
    hasher
        .finalize()
        .iter()
        .map(|b| format!("{b:02x}"))
        .collect()
}

/// Replace every CRLF pair in `bytes` with a single LF.
fn crlf_to_lf(bytes: &[u8]) -> Vec<u8> {
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

/// Run a malformed fixture through the codec and report the token it
/// produced.
///
/// Most fixtures fail at parse time. `ErrMissingUID` is encoder-only
/// in v0.1 — the rfc6350 parser accepts a UID-less VCARD and the
/// encoder refuses it — so a fixture that parses is re-encoded and the
/// encode failure classified instead. A fixture where both stages
/// succeed is a fault.
fn malformed_token(f: &Fixture, ext: &str) -> &'static str {
    let input = read_bytes(&f.path);
    let mut parse_err = None;
    let mut encode_err = None;
    let mut parsed = false;

    if ext == EXT_ICS {
        match rfc5545::parse(input.as_slice()) {
            Ok(_) => parsed = true,
            Err(e) => parse_err = Some(e),
        }
    } else {
        match rfc6350::parse(input.as_slice()) {
            Ok(cards) => {
                parsed = true;
                if cards.is_empty() {
                    fail(format!(
                        "{}: parse returned no cards and no error",
                        f.path.display()
                    ));
                }
                if let Some(e) = encode_cards(&cards) {
                    encode_err = Some(e);
                    parsed = false;
                }
            }
            Err(e) => parse_err = Some(e),
        }
    }

    for err in [parse_err.as_ref(), encode_err.as_ref()]
        .into_iter()
        .flatten()
    {
        if let Some(token) = super::corpus::classify(err, MALFORMED_CLASSES) {
            return token;
        }
    }
    if parsed {
        fail(format!(
            "{}: expected a failure, parse and encode both succeeded",
            f.path.display()
        ));
    }
    let shown = parse_err
        .or(encode_err)
        .map_or_else(|| "no error".to_owned(), |e| e.to_string());
    fail(format!(
        "{}: failure matches no known sentinel: {shown}",
        f.path.display()
    ));
}

/// Re-encode every parsed card and report the first failure, which is
/// how an encoder-only sentinel surfaces.
fn encode_cards(cards: &[Card]) -> Option<hop_top_vstar::Error> {
    for c in cards {
        let mut sink = Vec::new();
        if let Err(e) = rfc6350::encode(&mut sink, c) {
            return Some(e);
        }
    }
    None
}
