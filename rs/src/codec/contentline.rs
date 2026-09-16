// SPDX-License-Identifier: MIT

//! The shared RFC 5545 §3.1 content-line scanner and line folder.
//!
//! RFC 6350 §3.2 defers to RFC 5545 §3.1 for folding, so the iCalendar
//! and vCard codecs need byte-identical scanning behavior and share this
//! module rather than each growing its own copy.
//!
//! Parsers are **liberal**: CRLF and bare LF are both accepted as
//! physical-line terminators, blank lines outside a fold sequence are
//! skipped, and a terminator-less final line is surfaced. Encoders are
//! strict: always CRLF, always folded at 75 octets.

use crate::{Error, Result};
use std::io::Read;

/// The RFC 5545 §3.1 fold limit: a physical line, excluding its CRLF
/// terminator, MUST NOT exceed 75 **octets**.
///
/// Octets, not characters, not code points, not grapheme clusters. Rust
/// `str::len()` is already the UTF-8 byte count, which makes this correct
/// by default — `chars().count()` would not be.
pub const MAX_LINE_OCTETS: usize = 75;

/// The wire-format physical-line terminator.
pub const CRLF: &str = "\r\n";

/// Reads a stream of content lines, resolving RFC 5545 §3.1 line folds.
///
/// A logical line may be split across physical lines by inserting CRLF +
/// (SP | HTAB); on read both the terminator and the lead WSP octet are
/// consumed and the continuation is appended.
///
/// Not safe for concurrent use; construct one per input.
pub struct Scanner {
    lines: std::vec::IntoIter<String>,
    pending: Option<String>,
}

/// Removes RFC 5545 §3.1 fold sequences from `raw`, operating on bytes.
///
/// A fold is CRLF (or a bare LF) followed by one SP or HTAB. Both the
/// terminator and the single leading whitespace octet are dropped, which
/// rejoins a multi-byte UTF-8 sequence the encoder split across the
/// boundary. Line terminators that are not folds are preserved so the
/// caller still sees physical line structure.
fn unfold_bytes(raw: &[u8]) -> Vec<u8> {
    let mut out = Vec::with_capacity(raw.len());
    let mut i = 0;
    while i < raw.len() {
        let is_crlf = raw[i] == b'\r' && i + 1 < raw.len() && raw[i + 1] == b'\n';
        let is_lf = raw[i] == b'\n';
        if is_crlf || is_lf {
            let term = if is_crlf { 2 } else { 1 };
            let next = i + term;
            if next < raw.len() && (raw[next] == b' ' || raw[next] == b'\t') {
                // A fold: drop the terminator and the one lead octet.
                i = next + 1;
                continue;
            }
        }
        out.push(raw[i]);
        i += 1;
    }
    out
}

impl Scanner {
    /// Builds a scanner over `r`.
    ///
    /// The whole input is read up front and validated as UTF-8. A
    /// non-UTF-8 byte is [`Error::Malformed`], never a panic: V\* content
    /// is UTF-8 per RFC 6350 §3.1 and RFC 5545 §3.1.
    pub fn new(mut r: impl Read) -> Self {
        let mut raw = Vec::new();
        // A read error and invalid UTF-8 both degrade to "no lines"; the
        // parser above reports the resulting empty input as malformed,
        // which keeps this constructor infallible and panic-free.
        if r.read_to_end(&mut raw).is_err() {
            return Scanner {
                lines: Vec::new().into_iter(),
                pending: None,
            };
        }
        // Unfold on BYTES before decoding. Folding counts octets (RFC
        // 5545 §3.1, spec rule 3), so a multi-byte sequence straddling a
        // fold boundary is split across two physical lines and NEITHER
        // half is valid UTF-8 on its own. Decoding first would reject the
        // whole document; unfolding first reassembles the sequence and
        // the logical line decodes cleanly. Go gets this free because its
        // strings are byte sequences.
        let text = match String::from_utf8(unfold_bytes(&raw)) {
            Ok(t) => t,
            Err(_) => {
                return Scanner {
                    lines: Vec::new().into_iter(),
                    pending: None,
                }
            }
        };
        Scanner::from_text(&text)
    }

    /// Builds a scanner over an already-decoded document.
    pub fn from_text(text: &str) -> Self {
        // Split on LF and drop a trailing CR, so CRLF, bare LF and a
        // mixture all yield the same physical lines.
        let mut physical: Vec<String> = text
            .split('\n')
            .map(|l| l.strip_suffix('\r').unwrap_or(l).to_owned())
            .collect();
        // `split` yields a trailing empty element for a terminated input;
        // it is not a physical line.
        if text.ends_with('\n') {
            physical.pop();
        }
        Scanner {
            lines: physical.into_iter(),
            pending: None,
        }
    }

    /// The next logical content line, or `None` at end of input.
    ///
    /// The returned string does not include a terminator.
    pub fn next_line(&mut self) -> Result<Option<String>> {
        loop {
            let Some(raw) = self.lines.next() else {
                // Input exhausted: flush any line still in assembly.
                return Ok(self.pending.take().filter(|p| !p.is_empty()));
            };

            // A WSP-prefixed line continues the pending logical line.
            // RFC 5545 §3.1: the SP or HTAB is part of the fold sequence
            // and must NOT appear in the logical line.
            if raw.starts_with(' ') || raw.starts_with('\t') {
                let rest = &raw[1..];
                match self.pending.as_mut() {
                    Some(p) => p.push_str(rest),
                    None => {
                        // No pending line — a blank line broke the fold
                        // sequence. Start a fresh logical line from the
                        // remainder rather than emitting a stray leading
                        // space or dropping the content.
                        self.pending = Some(rest.to_owned());
                    }
                }
                continue;
            }

            // A new logical line starts here; flush the pending one.
            let flushed = self.pending.take();
            if !raw.is_empty() {
                self.pending = Some(raw);
            }
            match flushed {
                Some(out) if !out.is_empty() => return Ok(Some(out)),
                _ => continue,
            }
        }
    }

    /// Drains the scanner into every logical line, in input order.
    pub fn collect_lines(mut self) -> Result<Vec<String>> {
        let mut out = Vec::new();
        while let Some(line) = self.next_line()? {
            out.push(line);
        }
        Ok(out)
    }
}

impl Iterator for Scanner {
    type Item = Result<String>;

    fn next(&mut self) -> Option<Self::Item> {
        self.next_line().transpose()
    }
}

/// Appends `line` to `dst`, folded so no physical line exceeds
/// [`MAX_LINE_OCTETS`], each terminated with CRLF and each continuation
/// prefixed with a single SP.
///
/// Two rules the RFC fixes and a port gets wrong in ways a round-trip
/// test cannot see:
///
/// - **Folding is applied AFTER property assembly.** The caller passes a
///   complete logical line — name, every parameter, value, escaping
///   applied — and this folds that. Folding a value before appending
///   parameters puts the fold points elsewhere, and the result unfolds to
///   the same logical line, so only a byte comparison catches it.
/// - **The limit is 75 octets including the continuation's lead SP**, so
///   a continuation carries at most 74 payload octets.
///
/// **A fold splits a multi-byte UTF-8 sequence when the boundary falls
/// inside one**, per RFC 5545 §3.1 and spec rule 3: folding counts octets,
/// not characters, and a decoder reassembles the logical line before any
/// character-level interpretation. A physical line is therefore not
/// independently decodable, and the encoded output as a whole is not
/// guaranteed to be valid UTF-8.
///
/// This is why the buffer is `Vec<u8>` rather than `String`: Rust's
/// `String` cannot hold a split sequence, so a rune-safe cut would be the
/// only option and would put the fold points elsewhere — producing
/// different canonical bytes and a different hash from the reference for
/// the same input. Canonical bytes must agree across every
/// implementation, so the octet rule wins over the convenience of a
/// guaranteed-UTF-8 buffer.
pub fn write_folded(dst: &mut Vec<u8>, line: &str) {
    let mut rest = line.as_bytes();
    let mut limit = MAX_LINE_OCTETS;

    loop {
        if rest.len() <= limit {
            dst.extend_from_slice(rest);
            dst.extend_from_slice(CRLF.as_bytes());
            return;
        }
        dst.extend_from_slice(&rest[..limit]);
        dst.extend_from_slice(CRLF.as_bytes());
        dst.push(b' ');
        rest = &rest[limit..];
        // Every continuation line spends one octet on its lead SP.
        limit = MAX_LINE_OCTETS - 1;
    }
}

/// Splits a head segment on every occurrence of `sep` that lies outside a
/// DQUOTE-delimited span.
///
/// Returns [`Error::Malformed`] when the input ends inside an open quote.
pub fn split_unquoted(s: &str, sep: char) -> Result<Vec<&str>> {
    let mut out = Vec::new();
    let mut in_quote = false;
    let mut start = 0;
    for (i, c) in s.char_indices() {
        if c == '"' {
            in_quote = !in_quote;
            continue;
        }
        if c == sep && !in_quote {
            out.push(&s[start..i]);
            start = i + c.len_utf8();
        }
    }
    if in_quote {
        return Err(Error::Malformed(format!("unbalanced quote in {s:?}")));
    }
    out.push(&s[start..]);
    Ok(out)
}

/// The byte index of the first `target` in `s` outside any DQUOTE span,
/// or `None` when absent.
///
/// Returns [`Error::Malformed`] when the input ends inside an open quote.
pub fn find_unquoted(s: &str, target: char) -> Result<Option<usize>> {
    let mut in_quote = false;
    for (i, c) in s.char_indices() {
        if c == '"' {
            in_quote = !in_quote;
            continue;
        }
        if c == target && !in_quote {
            return Ok(Some(i));
        }
    }
    if in_quote {
        return Err(Error::Malformed(format!(
            "unterminated quoted value in {s:?}"
        )));
    }
    Ok(None)
}

/// Strips one layer of surrounding DQUOTEs, if present.
pub fn unquote(v: &str) -> &str {
    if v.len() >= 2 && v.starts_with('"') && v.ends_with('"') {
        &v[1..v.len() - 1]
    } else {
        v
    }
}

/// Wraps `v` in DQUOTEs when it contains a character that would otherwise
/// be read as a parameter boundary.
///
/// Inner DQUOTEs are dropped: RFC 5545 §3.2 does not permit a DQUOTE
/// inside a quoted-string, so there is no escape to emit.
pub fn encode_param_value(v: &str) -> String {
    let stripped: String = v.chars().filter(|c| *c != '"').collect();
    if stripped.contains([',', ';', ':']) {
        format!("\"{stripped}\"")
    } else {
        stripped
    }
}
