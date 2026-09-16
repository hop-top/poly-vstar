// SPDX-License-Identifier: MIT

//! The twelve V\* failure classes.
//!
//! One crate-wide enum, one variant per Go sentinel. A caller composing
//! parse → canonical → validate matches one enum, and the conformance
//! corpus asserts one identifier space: `malformed/*.error` files and
//! `rrule/**/*.expect.json` sidecars name a class by its Go spelling, so
//! the string `ErrMalformed` is data rather than a Go implementation
//! detail.
//!
//! Variants carry their positional context as payload, mirroring Go's
//! `fmt.Errorf("line %d: %w", n, ErrMalformed)`: the context travels
//! with the sentinel instead of replacing it.

use std::fmt;

/// A V\* failure.
///
/// Recover the stable identifier with [`Error::sentinel`]; match the
/// variant directly when the payload matters.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum Error {
    /// Structurally invalid input: a bad escape, bad parameter syntax,
    /// an unparseable value, or an RRULE on the parsing scope's
    /// hard-error list.
    Malformed(String),
    /// A `BEGIN` line lacks its matching `END` before end of input.
    UnclosedBlock(String),
    /// A `VERSION` property is present but is neither vCard 4.0 nor
    /// iCalendar 2.0.
    UnsupportedVersion(String),
    /// A component that requires `UID` has none.
    ///
    /// Encoder-only: the RFC 6350 parser accepts a UID-less
    /// VCARD by design and the encoder is what refuses it.
    MissingUid(String),
    /// Syntactically valid but outside the RRULE parsing scope.
    UnsupportedRRule(String),
    /// The evaluator reached its iteration bound without finding an
    /// occurrence; the rule did not terminate.
    IterationCap(String),
    /// A request to expand into a list with no bound.
    UnboundedExpansion(String),
    /// The target component's `X-VSTAR-HASH` does not match its
    /// recomputed canonical form.
    TargetCorrupted(String),
    /// `Close` called twice, or `Encode` called after `Close`.
    AlreadyClosed(String),
    /// `SetHeader` called after the first `Encode` locked the header.
    HeaderLocked(String),
    /// A `VALARM` without a `TRIGGER`.
    NoTrigger(String),
    /// A relative trigger resolved against a component lacking its
    /// anchor.
    NoAnchor(String),
}

impl Error {
    /// The stable Go sentinel identifier for this failure class.
    ///
    /// This is what the conformance corpus compares against, so the
    /// spelling is normative — `ErrMissingUID` keeps Go's acronym run.
    pub fn sentinel(&self) -> &'static str {
        match self {
            Error::Malformed(_) => "ErrMalformed",
            Error::UnclosedBlock(_) => "ErrUnclosedBlock",
            Error::UnsupportedVersion(_) => "ErrUnsupportedVersion",
            Error::MissingUid(_) => "ErrMissingUID",
            Error::UnsupportedRRule(_) => "ErrUnsupportedRRule",
            Error::IterationCap(_) => "ErrIterationCap",
            Error::UnboundedExpansion(_) => "ErrUnboundedExpansion",
            Error::TargetCorrupted(_) => "ErrTargetCorrupted",
            Error::AlreadyClosed(_) => "ErrAlreadyClosed",
            Error::HeaderLocked(_) => "ErrHeaderLocked",
            Error::NoTrigger(_) => "ErrNoTrigger",
            Error::NoAnchor(_) => "ErrNoAnchor",
        }
    }

    /// The positional context carried alongside the sentinel.
    pub fn context(&self) -> &str {
        match self {
            Error::Malformed(c)
            | Error::UnclosedBlock(c)
            | Error::UnsupportedVersion(c)
            | Error::MissingUid(c)
            | Error::UnsupportedRRule(c)
            | Error::IterationCap(c)
            | Error::UnboundedExpansion(c)
            | Error::TargetCorrupted(c)
            | Error::AlreadyClosed(c)
            | Error::HeaderLocked(c)
            | Error::NoTrigger(c)
            | Error::NoAnchor(c) => c,
        }
    }

    /// The human-readable class name used as the `Display` prefix.
    fn label(&self) -> &'static str {
        match self {
            Error::Malformed(_) => "malformed",
            Error::UnclosedBlock(_) => "unclosed block",
            Error::UnsupportedVersion(_) => "unsupported version",
            Error::MissingUid(_) => "missing UID",
            Error::UnsupportedRRule(_) => "unsupported RRULE",
            Error::IterationCap(_) => "iteration cap reached",
            Error::UnboundedExpansion(_) => "unbounded expansion",
            Error::TargetCorrupted(_) => "target corrupted",
            Error::AlreadyClosed(_) => "already closed",
            Error::HeaderLocked(_) => "header locked",
            Error::NoTrigger(_) => "no trigger",
            Error::NoAnchor(_) => "no anchor",
        }
    }
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let ctx = self.context();
        if ctx.is_empty() {
            f.write_str(self.label())
        } else {
            write!(f, "{}: {ctx}", self.label())
        }
    }
}

impl std::error::Error for Error {}

/// Shorthand for the crate's pervasive `Result` shape.
pub type Result<T> = std::result::Result<T, Error>;
