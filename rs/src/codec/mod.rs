// SPDX-License-Identifier: MIT

//! The wire codecs.
//!
//! [`rfc5545`] reads and writes iCalendar VCALENDAR documents;
//! [`rfc6350`] reads and writes vCard 4.0. The RFC 5545 §3.1 content-line
//! scanner and line folder are shared between them, because RFC 6350 §3.2
//! defers to §3.1 for folding and the two formats must scan identically.
//!
//! [`stream`] carries the constant-memory counterparts: iterator-style
//! codecs that surface one component or card at a time, for callers
//! whose input does not fit comfortably in memory.

pub mod contentline;
pub mod rfc5545;
pub mod rfc6350;
pub mod stream;
