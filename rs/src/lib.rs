// SPDX-License-Identifier: MIT

//! V\* calendar and contact interchange — Rust port.
//!
//! V\* is a convention over RFC 5545 (iCalendar) and RFC 6350 (vCard) for
//! agentic systems: a deterministic canonical form, a content hash over
//! it, and an append-only supersession model. Two conformant
//! implementations produce identical bytes for the same document, on any
//! machine, in any year.
//!
//! **Status: layer (e).** The in-memory model, both wire codecs,
//! canonicalization, hashing, TZID resolution, durations, the extension
//! namespace, the helper surfaces, supersession, diff and the streaming
//! codecs are implemented. Recurrence and validation land in their own
//! layers; see `docs/dev/porting-guide.md`.
//!
//! # Example
//!
//! ```
//! use hop_top_vstar::codec::rfc5545;
//!
//! let src = "BEGIN:VCALENDAR\r\n\
//!            VERSION:2.0\r\n\
//!            PRODID:-//Example//EN\r\n\
//!            BEGIN:VTODO\r\n\
//!            UID:task-1\r\n\
//!            SUMMARY:Ship the port\r\n\
//!            END:VTODO\r\n\
//!            END:VCALENDAR\r\n";
//!
//! let cal = rfc5545::parse(src.as_bytes())?;
//! assert_eq!(cal.prod_id, "-//Example//EN");
//! assert_eq!(cal.components[0].uid(), "task-1");
//!
//! let mut out = Vec::new();
//! rfc5545::encode(&mut out, &cal)?;
//! assert_eq!(out, src.as_bytes());
//! # Ok::<(), Box<dyn std::error::Error>>(())
//! ```
//!
//! # Failure classes
//!
//! Every failure carries one of twelve sentinels, recoverable as a stable
//! string with [`Error::sentinel`]. The identifier is the cross-language
//! contract — the conformance corpus asserts it by name.

#![forbid(unsafe_code)]
#![warn(missing_docs)]

pub mod canonical;
pub mod codec;
pub mod diff;
pub mod duration;
pub mod ext;
pub mod generated;
pub mod hashing;
pub mod helpers;
pub mod rrule;
pub mod supersession;
pub mod validate;

mod date;
mod enums;
mod error;
mod model;
mod time;

pub use date::{date_of, format_date, parse_date, Date};
pub use enums::{
    default_rel_type, parse_rel_type, Class, CompType, EventStatus, JournalStatus, Kind, RelType,
    TodoStatus, Transp, DEFAULT_REL_TYPE,
};
pub use error::{Error, Result};
pub use model::{
    property_equal, Calendar, Card, Component, Param, Property, VALUE_DATE, VALUE_PARAM,
};
pub use time::{format_time, parse_time, parse_time_with_tzid};

/// Crate version, taken from `Cargo.toml` at compile time so it cannot
/// drift from the manifest release-please rewrites.
pub const fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}

#[cfg(test)]
mod tests {
    use super::version;

    #[test]
    fn version_matches_the_manifest() {
        assert_eq!(version(), env!("CARGO_PKG_VERSION"));
        assert!(!version().is_empty());
    }
}
