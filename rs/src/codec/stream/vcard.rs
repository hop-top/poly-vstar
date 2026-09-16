// SPDX-License-Identifier: MIT

//! The streaming VCARD reader and writer.

use super::scanner::LineScanner;
use crate::codec::rfc6350;
use crate::enums::Kind;
use crate::error::{Error, Result};
use crate::model::Card;
use std::io::{BufRead, BufReader, Read, Write};

/// The only VERSION the codecs accept, matching the batch codec.
const SUPPORTED_VERSION: &str = "4.0";

/// The streaming VCARD reader: one [`Card`] per iteration.
///
/// A vCard file is a concatenation of independent
/// `BEGIN:VCARD` … `END:VCARD` blocks with no enclosing wrapper, so
/// there is no header to skip and no trailer to expect. The iterator is
/// exhausted when the reader runs out with no block open.
///
/// Not safe for concurrent use; construct one per reader.
pub struct VCardParser<R: BufRead> {
    scanner: LineScanner<R>,
    /// Set once exhaustion or a failure has been reported.
    done: bool,
}

impl<R: Read> VCardParser<BufReader<R>> {
    /// Builds a parser over `r`, buffering internally. Callers should
    /// not pre-buffer; use [`VCardParser::from_buf_read`] when you
    /// already hold a [`BufRead`].
    pub fn new(r: R) -> Self {
        VCardParser::from_buf_read(BufReader::new(r))
    }
}

impl<R: BufRead> VCardParser<R> {
    /// Builds a parser over an already-buffered reader.
    pub fn from_buf_read(r: R) -> Self {
        VCardParser {
            scanner: LineScanner::new(r),
            done: false,
        }
    }

    fn read_next(&mut self) -> Option<Result<Card>> {
        if self.done {
            return None;
        }
        match self.read_card() {
            Ok(Some(card)) => Some(Ok(card)),
            Ok(None) => {
                self.done = true;
                None
            }
            Err(e) => {
                self.done = true;
                Some(Err(e))
            }
        }
    }

    /// Reads one block, or `Ok(None)` at clean end of input.
    fn read_card(&mut self) -> Result<Option<Card>> {
        // Skip blanks and wait for BEGIN:VCARD.
        loop {
            let Some(line) = self.scanner.next_line()? else {
                return Ok(None);
            };
            if line.is_empty() {
                continue;
            }
            if !line.eq_ignore_ascii_case("BEGIN:VCARD") {
                return Err(Error::Malformed(format!(
                    "stream/vcard: expected BEGIN:VCARD, got {line:?}"
                )));
            }
            break;
        }

        let mut card = Card::default();
        let mut version: Option<String> = None;

        loop {
            let Some(line) = self.scanner.next_line()? else {
                return Err(Error::UnclosedBlock(
                    "stream/vcard: BEGIN:VCARD never closed".into(),
                ));
            };
            if line.is_empty() {
                continue;
            }
            if line.eq_ignore_ascii_case("END:VCARD") {
                return match version.as_deref() {
                    None => Err(Error::Malformed(
                        "stream/vcard: VCARD missing VERSION".into(),
                    )),
                    Some(v) if v != SUPPORTED_VERSION => Err(Error::UnsupportedVersion(format!(
                        "stream/vcard: VERSION:{v}"
                    ))),
                    Some(_) => Ok(Some(card)),
                };
            }
            if line.eq_ignore_ascii_case("BEGIN:VCARD") {
                return Err(Error::Malformed("stream/vcard: nested BEGIN:VCARD".into()));
            }

            // RFC 6350's own content-line parser, which applies §3.4
            // TEXT unescaping. The Go reference routes its stream vCard
            // parser through the RFC 5545 parser instead and so never
            // unescapes, making its batch and streamed parses of
            // `rfc6350/escaping.vcf` disagree. That is a known, filed Go
            // bug; this port does not replicate it.
            let prop = rfc6350::parse_content_line(&line)?;

            match bare_name(&prop.name).to_ascii_uppercase().as_str() {
                "VERSION" => {
                    if version.is_some() {
                        return Err(Error::Malformed("stream/vcard: duplicate VERSION".into()));
                    }
                    version = Some(prop.value);
                }
                "UID" => card.uid = prop.value,
                // RFC 6350 §6.1.4 registers lowercase values; the wire
                // form is case-insensitive on read, and an unregistered
                // value reads as absent — matching the batch codec.
                "KIND" => card.kind = Kind::parse(&prop.value),
                _ => card.props.push(prop),
            }
        }
    }
}

/// Strips the optional `group.` prefix from a vCard property name per
/// RFC 6350 §3.3, so `UID` / `VERSION` / `KIND` are recognized whatever
/// group qualifies them on the wire.
fn bare_name(name: &str) -> &str {
    match name.find('.') {
        Some(i) => &name[i + 1..],
        None => name,
    }
}

impl<R: BufRead> Iterator for VCardParser<R> {
    type Item = Result<Card>;

    /// `None` at exhaustion, `Some(Err(_))` on a real failure.
    fn next(&mut self) -> Option<Self::Item> {
        self.read_next()
    }
}

/// The streaming VCARD writer: one self-contained
/// `BEGIN:VCARD` … `END:VCARD` block per [`encode`](Self::encode).
///
/// # There is deliberately no `set_header`
///
/// Only [`VCalendarEncoder`](super::VCalendarEncoder) has one. A VCARD
/// stream has no enclosing wrapper — each `encode` writes a complete,
/// self-contained block — so there is no header to set and no trailer
/// to emit. [`close`](Self::close) exists for symmetry and to flush.
///
/// A port that adds `set_header` here for API symmetry is broken: there
/// is nothing for it to do, and its existence invites callers to emit a
/// wrapper the parsers reject.
pub struct VCardEncoder<W: Write> {
    w: W,
    closed: bool,
}

impl<W: Write> VCardEncoder<W> {
    /// Builds an encoder writing to `w`. Callers should not pre-buffer.
    pub fn new(w: W) -> Self {
        VCardEncoder { w, closed: false }
    }

    /// Writes one card as a complete block in canonical VCARD wire form
    /// — `VERSION:4.0`, `UID`, `KIND`, then [`Card::props`] in input
    /// order, CRLF-folded at 75 octets.
    ///
    /// # Errors
    ///
    /// [`Error::AlreadyClosed`] after [`close`](Self::close), and
    /// [`Error::MissingUid`] for a card with an empty
    /// [`uid`](Card::uid) — the encoder half of the RFC 6350
    /// asymmetry.
    pub fn encode(&mut self, c: &Card) -> Result<()> {
        if self.closed {
            return Err(Error::AlreadyClosed(
                "stream/vcard: encode after close".into(),
            ));
        }
        rfc6350::encode(&mut self.w, c)
    }

    /// Flushes buffered writes. Safe on an encoder that never saw
    /// [`encode`](Self::encode) — there is no wrapper, so nothing is
    /// written.
    ///
    /// # Errors
    ///
    /// [`Error::AlreadyClosed`] when called more than once.
    pub fn close(&mut self) -> Result<()> {
        if self.closed {
            return Err(Error::AlreadyClosed(
                "stream/vcard: close called twice".into(),
            ));
        }
        self.w
            .flush()
            .map_err(|e| Error::Malformed(format!("stream/vcard: flush failed: {e}")))?;
        self.closed = true;
        Ok(())
    }
}
