// SPDX-License-Identifier: MIT

//! The streaming VCALENDAR reader and writer.

use super::scanner::LineScanner;
use crate::codec::rfc5545::{self, is_text_property, parse_content_line, unescape_text};
use crate::error::{Error, Result};
use crate::model::{Calendar, Component};
use crate::CompType;
use std::io::{BufRead, BufReader, Read, Write};

/// The only VERSION the v0.1 codecs accept, matching the batch codec.
const SUPPORTED_VERSION: &str = "2.0";

/// The `PRODID` emitted when [`VCalendarEncoder::set_header`] was never
/// called. Mirrors the batch encoder's default so batch and stream
/// output for the same logical calendar are byte-stable against each
/// other.
const DEFAULT_PROD_ID: &str = "-//hop-top//vstar-go v0.1.0//EN";

/// The RFC 5545 `BEGIN:` / `END:` pseudo-property names.
const BEGIN: &str = "BEGIN";
const END: &str = "END";

/// The streaming VCALENDAR reader: one top-level [`Component`] per
/// iteration, in constant memory.
///
/// The header is consumed on first use — the parser reads up to
/// `BEGIN:VCALENDAR`, captures the calendar-level properties
/// (`VERSION`, `PRODID`, …) into its header state for
/// [`header`](Self::header), then yields nested components. Once
/// `END:VCALENDAR` is seen the iterator is exhausted.
///
/// ```
/// use hop_top_vstar::codec::stream::VCalendarParser;
/// use std::io::Cursor;
///
/// let src = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//E//EN\r\n\
///            BEGIN:VTODO\r\nUID:t-1\r\nEND:VTODO\r\nEND:VCALENDAR\r\n";
/// let mut p = VCalendarParser::new(Cursor::new(src));
/// assert_eq!(p.header().prod_id, "-//E//EN");
/// let todo = p.next().unwrap()?;
/// assert_eq!(todo.uid(), "t-1");
/// assert!(p.next().is_none(), "exhaustion is None, never an error");
/// # Ok::<(), hop_top_vstar::Error>(())
/// ```
///
/// Not safe for concurrent use; construct one per reader.
pub struct VCalendarParser<R: BufRead> {
    scanner: LineScanner<R>,
    /// Set once the header has been consumed.
    header_read: bool,
    /// The calendar-level properties seen between `BEGIN:VCALENDAR` and
    /// the first sub-component.
    header: Calendar,
    /// A line peeked while detecting the end of the header section.
    pending: Option<String>,
    /// Set after `END:VCALENDAR`, or after any error — a failed parse
    /// does not resume.
    done: bool,
    /// An error captured by a pre-`next` [`header`](Self::header) call,
    /// surfaced from the next iteration.
    deferred: Option<Error>,
}

impl<R: Read> VCalendarParser<BufReader<R>> {
    /// Builds a parser over `r`, buffering internally.
    ///
    /// Callers should **not** pre-buffer: pass the raw reader and let
    /// the parser own the buffering, exactly as the Go reference does.
    /// Use [`VCalendarParser::from_buf_read`] when you already hold a
    /// [`BufRead`].
    pub fn new(r: R) -> Self {
        VCalendarParser::from_buf_read(BufReader::new(r))
    }
}

impl<R: BufRead> VCalendarParser<R> {
    /// Builds a parser over an already-buffered reader.
    pub fn from_buf_read(r: R) -> Self {
        VCalendarParser {
            scanner: LineScanner::new(r),
            header_read: false,
            header: Calendar::default(),
            pending: None,
            done: false,
            deferred: None,
        }
    }

    /// The calendar-level properties captured from the
    /// `BEGIN:VCALENDAR` header.
    ///
    /// [`Calendar::components`] is always empty: only the
    /// VCALENDAR-level properties populate.
    ///
    /// Safe to call before the first iteration — the header is read on
    /// demand. If that read fails (a malformed stream, an unsupported
    /// `VERSION`) the error is captured and surfaced from the next
    /// iteration, and this still returns the possibly-empty
    /// [`Calendar`] so a caller can introspect what was parsed.
    pub fn header(&mut self) -> &Calendar {
        if !self.header_read {
            if let Err(e) = self.ensure_header() {
                self.deferred = Some(e);
            }
        }
        &self.header
    }

    /// Consumes `BEGIN:VCALENDAR` and the calendar-level properties
    /// that follow, stashing the boundary line for the next pull.
    fn ensure_header(&mut self) -> Result<()> {
        if self.header_read {
            return Ok(());
        }
        // Set before mutating so a retry after a failure does not loop.
        self.header_read = true;

        let Some(first) = self.scanner.next_line()? else {
            return Err(Error::Malformed("stream/vcalendar: empty input".into()));
        };
        if !first.eq_ignore_ascii_case("BEGIN:VCALENDAR") {
            return Err(Error::Malformed(format!(
                "stream/vcalendar: expected BEGIN:VCALENDAR, got {first:?}"
            )));
        }

        loop {
            let Some(line) = self.scanner.next_line()? else {
                return Err(Error::UnclosedBlock(
                    "stream/vcalendar: BEGIN:VCALENDAR never closed".into(),
                ));
            };
            let prop = parse_content_line(&line)?;

            if prop.name.eq_ignore_ascii_case(BEGIN) || prop.name.eq_ignore_ascii_case(END) {
                self.pending = Some(line);
                return Ok(());
            }
            if prop.name.eq_ignore_ascii_case("VERSION") {
                if prop.value != SUPPORTED_VERSION {
                    return Err(Error::UnsupportedVersion(format!(
                        "stream/vcalendar: VERSION={:?} (only {SUPPORTED_VERSION:?} supported)",
                        prop.value
                    )));
                }
            } else if prop.name.eq_ignore_ascii_case("PRODID") {
                self.header.prod_id = unescape_text(&prop.value);
            }
            // METHOD and X-* are not surfaced as dedicated Calendar
            // fields, matching the batch codec's model.
        }
    }

    /// The next content line, taking any peeked one first.
    fn next_line(&mut self) -> Result<Option<String>> {
        match self.pending.take() {
            Some(line) => Ok(Some(line)),
            None => self.scanner.next_line(),
        }
    }

    /// Reads one component, or `None` once the stream is exhausted.
    fn read_next(&mut self) -> Option<Result<Component>> {
        if self.done {
            return None;
        }
        if let Some(e) = self.deferred.take() {
            self.done = true;
            return Some(Err(e));
        }
        if let Err(e) = self.ensure_header() {
            self.done = true;
            return Some(Err(e));
        }

        let line = match self.next_line() {
            Ok(Some(line)) => line,
            // End of input inside an open VCALENDAR: the spec requires
            // `END:VCALENDAR`, so this is an unclosed block, not
            // exhaustion.
            Ok(None) => {
                self.done = true;
                return Some(Err(Error::UnclosedBlock(
                    "stream/vcalendar: BEGIN:VCALENDAR never closed".into(),
                )));
            }
            Err(e) => {
                self.done = true;
                return Some(Err(e));
            }
        };

        let prop = match parse_content_line(&line) {
            Ok(p) => p,
            Err(e) => {
                self.done = true;
                return Some(Err(e));
            }
        };

        if prop.name.eq_ignore_ascii_case(END) {
            self.done = true;
            if !prop.value.eq_ignore_ascii_case("VCALENDAR") {
                return Some(Err(Error::Malformed(format!(
                    "stream/vcalendar: END:{} does not match BEGIN:VCALENDAR",
                    prop.value
                ))));
            }
            return None;
        }
        if prop.name.eq_ignore_ascii_case(BEGIN) {
            let out = self.read_block(&prop.value.to_ascii_uppercase());
            if out.is_err() {
                self.done = true;
            }
            return Some(out);
        }

        // A calendar-level property after the header section. RFC 5545
        // allows METHOD / X-* anywhere before the first sub-component,
        // so seeing one here means header properties are interleaved
        // among components, which the reference rejects.
        self.done = true;
        Some(Err(Error::Malformed(format!(
            "stream/vcalendar: unexpected calendar-level property {:?} after components",
            prop.name
        ))))
    }

    /// Parses a `BEGIN:<type>` … `END:<type>` subtree from the line
    /// after the `BEGIN`, off this parser's own scanner so reads stay
    /// in lockstep with the outer stream.
    fn read_block(&mut self, type_name: &str) -> Result<Component> {
        let mut out = Component::new(CompType::from_wire(type_name));
        loop {
            let Some(line) = self.scanner.next_line()? else {
                return Err(Error::UnclosedBlock(format!(
                    "stream/vcalendar: BEGIN:{type_name} never closed"
                )));
            };
            let mut prop = parse_content_line(&line)?;

            if prop.name.eq_ignore_ascii_case(BEGIN) {
                let child = self.read_block(&prop.value.to_ascii_uppercase())?;
                out.sub.push(child);
                continue;
            }
            if prop.name.eq_ignore_ascii_case(END) {
                if !prop.value.eq_ignore_ascii_case(type_name) {
                    return Err(Error::Malformed(format!(
                        "stream/vcalendar: END:{} does not match BEGIN:{type_name}",
                        prop.value
                    )));
                }
                return Ok(out);
            }

            // Unescape TEXT-typed values per RFC 5545 §3.3.11, exactly
            // as the batch parser does, so the model holds raw values
            // and a streamed parse equals a batch parse.
            //
            // The Go reference does NOT do this in its stream parsers —
            // a known, filed bug that makes its batch and streamed
            // parses of `rfc6350/escaping.vcf` disagree. This port does
            // not replicate it.
            if is_text_property(&prop.name) {
                prop.value = unescape_text(&prop.value);
            }
            out.props.push(prop);
        }
    }
}

impl<R: BufRead> Iterator for VCalendarParser<R> {
    type Item = Result<Component>;

    /// `None` at exhaustion, `Some(Err(_))` on a real failure.
    /// Exhaustion is never an error.
    fn next(&mut self) -> Option<Self::Item> {
        self.read_next()
    }
}

/// The streaming VCALENDAR writer.
///
/// The first [`encode`](Self::encode) emits the `BEGIN:VCALENDAR`
/// header (`VERSION` then `PRODID`, optionally overridden via
/// [`set_header`](Self::set_header)); each later call appends one
/// component; [`close`](Self::close) emits `END:VCALENDAR` and flushes.
///
/// The encoder does **not** close the underlying writer — the caller
/// owns that lifecycle. Not safe for concurrent use.
pub struct VCalendarEncoder<W: Write> {
    w: W,
    header: Calendar,
    /// Set once the header has reached the writer; locks
    /// [`set_header`](Self::set_header).
    header_written: bool,
    /// Set after a successful [`close`](Self::close).
    closed: bool,
}

impl<W: Write> VCalendarEncoder<W> {
    /// Builds an encoder writing to `w`, in the "header pending" state.
    pub fn new(w: W) -> Self {
        VCalendarEncoder {
            w,
            header: Calendar::default(),
            header_written: false,
            closed: false,
        }
    }

    /// Configures the VCALENDAR header emitted on the next
    /// [`encode`](Self::encode).
    ///
    /// Only [`Calendar::prod_id`] is consulted; `VERSION` is fixed at
    /// 2.0 per the v0.1 supported-version contract.
    ///
    /// # Errors
    ///
    /// [`Error::HeaderLocked`] when called after the first `encode`.
    /// The header is already on the wire and cannot be retroactively
    /// changed — the lock is what makes the
    /// `BEGIN:VCALENDAR` / `VERSION` / `PRODID` ordering deterministic.
    pub fn set_header(&mut self, h: &Calendar) -> Result<()> {
        if self.header_written {
            return Err(Error::HeaderLocked(
                "stream/vcalendar: set_header after the first encode".into(),
            ));
        }
        self.header = h.clone();
        Ok(())
    }

    /// Writes one component, emitting the header first on the first
    /// call.
    ///
    /// # Errors
    ///
    /// [`Error::AlreadyClosed`] when called after
    /// [`close`](Self::close), and [`Error::Malformed`] wrapping any
    /// write failure.
    pub fn encode(&mut self, c: &Component) -> Result<()> {
        if self.closed {
            return Err(Error::AlreadyClosed(
                "stream/vcalendar: encode after close".into(),
            ));
        }
        self.write_header_once()?;
        rfc5545::encode_component(&mut self.w, c)
            .map_err(|e| Error::Malformed(format!("stream/vcalendar: write failed: {e}")))
    }

    /// Emits `END:VCALENDAR` and flushes.
    ///
    /// Safe on an encoder that never saw [`encode`](Self::encode): the
    /// header is written first, so the output is still a legal empty
    /// calendar.
    ///
    /// # Errors
    ///
    /// [`Error::AlreadyClosed`] when called more than once — never a
    /// silently ignored no-op.
    pub fn close(&mut self) -> Result<()> {
        if self.closed {
            return Err(Error::AlreadyClosed(
                "stream/vcalendar: close called twice".into(),
            ));
        }
        self.write_header_once()?;
        self.write_all(b"END:VCALENDAR\r\n")?;
        self.w
            .flush()
            .map_err(|e| Error::Malformed(format!("stream/vcalendar: flush failed: {e}")))?;
        self.closed = true;
        Ok(())
    }

    /// Emits `BEGIN:VCALENDAR` + `VERSION` + `PRODID` exactly once.
    fn write_header_once(&mut self) -> Result<()> {
        if self.header_written {
            return Ok(());
        }
        self.header_written = true;

        let prod_id = if self.header.prod_id.is_empty() {
            DEFAULT_PROD_ID
        } else {
            &self.header.prod_id
        }
        .to_owned();

        self.write_all(b"BEGIN:VCALENDAR\r\n")?;
        self.write_all(format!("VERSION:{SUPPORTED_VERSION}\r\n").as_bytes())?;
        // PRODID is TEXT-typed, so the same escape rules apply as to any
        // other TEXT property; route through the codec's escaper rather
        // than hand-rolling a second one.
        let mut line = Vec::new();
        crate::codec::contentline::write_folded(
            &mut line,
            &format!("PRODID:{}", rfc5545::escape_text(&prod_id)),
        );
        self.write_all(&line)
    }

    fn write_all(&mut self, bytes: &[u8]) -> Result<()> {
        self.w
            .write_all(bytes)
            .map_err(|e| Error::Malformed(format!("stream/vcalendar: write failed: {e}")))
    }
}
