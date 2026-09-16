// SPDX-License-Identifier: MIT

//! Constant-memory iterator-style codecs for the V\* wire formats.
//!
//! Where the batch codecs ([`rfc5545`](crate::codec::rfc5545),
//! [`rfc6350`](crate::codec::rfc6350)) load a whole [`Calendar`] or card
//! list into memory, these surface one [`Component`] or [`Card`] at a
//! time, so a caller processing a large ledger keeps memory flat
//! regardless of input size.
//!
//! Two pairs:
//!
//! - **VCALENDAR** — [`VCalendarParser`] yields one component per
//!   iteration; [`VCalendarEncoder`] writes a `BEGIN:VCALENDAR` header
//!   on first `encode` and an `END:VCALENDAR` trailer on `close`.
//! - **VCARD** — [`VCardParser`] yields one card per iteration;
//!   [`VCardEncoder`] writes one self-contained
//!   `BEGIN:VCARD` … `END:VCARD` block per `encode`.
//!
//! # Exhaustion is never an error
//!
//! Both parsers implement [`Iterator`] over
//! `Result<_, `[`Error`](crate::Error)`>`: `None` at exhaustion,
//! `Some(Err(_))` on a real failure. A caller walking a stream to its
//! end hits `None` exactly once and never has to distinguish it from a
//! parse error.
//!
//! # Encoder lifecycle
//!
//! `close` twice, or `encode` after `close`, yields
//! [`Error::AlreadyClosed`](crate::Error::AlreadyClosed).
//! [`VCalendarEncoder::set_header`] after the first `encode` yields
//! [`Error::HeaderLocked`](crate::Error::HeaderLocked) — the header
//! locks at first `encode` so the
//! `BEGIN:VCALENDAR` / `VERSION` / `PRODID` ordering on the wire is
//! deterministic. Both are reachable by callers; silently ignoring a
//! double `close` would be a divergence.
//!
//! # A deliberate divergence from the Go reference
//!
//! The Go stream parsers route every content line through
//! `rfc5545.ParseContentLine` and never apply TEXT unescaping, so the
//! reference's batch and streamed parses of the same document disagree
//! wherever a TEXT value carries an escape — `rfc6350/escaping.vcf`
//! yields `Last, Comma Test` batched and `Last\, Comma Test` streamed.
//! That is a known bug, already filed against the reference. This port
//! does **not** replicate it: each stream parser routes through its own
//! format's unescaping, so a streamed parse equals a batch parse on
//! every corpus file.
//!
//! All implementations are for sequential, single-threaded use;
//! construct one codec per thread. Backpressure and cancellation are the
//! caller's concern.
//!
//! [`Calendar`]: crate::Calendar
//! [`Component`]: crate::Component
//! [`Card`]: crate::Card

mod scanner;
mod vcalendar;
mod vcard;

pub use vcalendar::{VCalendarEncoder, VCalendarParser};
pub use vcard::{VCardEncoder, VCardParser};
