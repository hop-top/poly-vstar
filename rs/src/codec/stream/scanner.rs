// SPDX-License-Identifier: MIT

//! A constant-memory content-line scanner.
//!
//! [`crate::codec::contentline::Scanner`] reads its whole input up
//! front, which is the right trade for the batch codecs — they
//! materialize a whole document anyway. The streaming codecs cannot
//! afford it: their entire reason to exist is keeping memory flat
//! regardless of input size, and a scanner that slurps first would make
//! every one of them a batch parser wearing an iterator's clothes.
//!
//! This one pulls one physical line at a time out of a [`BufRead`] and
//! holds at most one logical line in assembly.
//!
//! # Unfolding happens on BYTES
//!
//! RFC 5545 §3.1 counts folding in octets (spec rule 3), so a
//! multi-byte UTF-8 sequence straddling a fold boundary is split across
//! two physical lines and **neither half is valid UTF-8 on its own**. A
//! Rust `String` cannot hold a split sequence, so the physical lines are
//! accumulated as `Vec<u8>`, the fold is resolved there, and only the
//! reassembled logical line is decoded. Decoding first would reject a
//! document the reference accepts — the `fold_split_utf8` corpus
//! fixture is the gate.

use crate::error::{Error, Result};
use std::io::BufRead;

/// Reads logical content lines incrementally, resolving folds.
pub(super) struct LineScanner<R: BufRead> {
    reader: R,
    /// The logical line under assembly, as raw bytes.
    pending: Option<Vec<u8>>,
    /// Set once the reader has returned end-of-input.
    exhausted: bool,
}

impl<R: BufRead> LineScanner<R> {
    pub(super) fn new(reader: R) -> Self {
        LineScanner {
            reader,
            pending: None,
            exhausted: false,
        }
    }

    /// Reads one physical line, stripped of its CRLF or LF terminator.
    ///
    /// `Ok(None)` at end of input. Read errors surface as
    /// [`Error::Malformed`]: the V\* error space has no I/O class, and
    /// a truncated read is indistinguishable from truncated input at
    /// this layer.
    fn physical_line(&mut self) -> Result<Option<Vec<u8>>> {
        if self.exhausted {
            return Ok(None);
        }
        let mut buf = Vec::new();
        let n = self
            .reader
            .read_until(b'\n', &mut buf)
            .map_err(|e| Error::Malformed(format!("stream: read failed: {e}")))?;
        if n == 0 {
            self.exhausted = true;
            return Ok(None);
        }
        if buf.last() == Some(&b'\n') {
            buf.pop();
            if buf.last() == Some(&b'\r') {
                buf.pop();
            }
        }
        Ok(Some(buf))
    }

    /// The next logical line, decoded, or `None` at end of input.
    ///
    /// A blank physical line is not a content line; it flushes whatever
    /// is in assembly and is otherwise skipped.
    pub(super) fn next_line(&mut self) -> Result<Option<String>> {
        loop {
            let Some(raw) = self.physical_line()? else {
                return match self.pending.take() {
                    Some(p) if !p.is_empty() => decode(p).map(Some),
                    _ => Ok(None),
                };
            };

            // A WSP-prefixed physical line continues the pending logical
            // line. Per RFC 5545 §3.1 the single SP or HTAB is part of
            // the fold sequence and must NOT appear in the logical line.
            if matches!(raw.first(), Some(b' ' | b'\t')) {
                let rest = &raw[1..];
                match self.pending.as_mut() {
                    Some(p) => p.extend_from_slice(rest),
                    // No pending line: a blank line broke the fold
                    // sequence. Start fresh from the remainder rather
                    // than emitting a stray leading space.
                    None => self.pending = Some(rest.to_vec()),
                }
                continue;
            }

            // A new logical line starts here; flush the pending one.
            let flushed = self.pending.take();
            if !raw.is_empty() {
                self.pending = Some(raw);
            }
            match flushed {
                Some(out) if !out.is_empty() => return decode(out).map(Some),
                _ => continue,
            }
        }
    }
}

/// Decodes a reassembled logical line, reporting invalid UTF-8 as
/// [`Error::Malformed`] — V\* content is UTF-8 per RFC 5545 §3.1 and
/// RFC 6350 §3.1.
fn decode(bytes: Vec<u8>) -> Result<String> {
    String::from_utf8(bytes)
        .map_err(|_| Error::Malformed("stream: content line is not valid UTF-8".into()))
}
