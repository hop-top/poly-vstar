// SPDX-License-Identifier: MIT

//! The wire-string enums.
//!
//! Each Go type here is a `string` with named constants. The wire value
//! is normative — a port that changes a spelling is broken — so every
//! variant round-trips through [`as_str`](CompType::as_str) and `parse`
//! without translation.
//!
//! [`Class`] keeps its Go spelling: `class` is a reserved word in
//! TypeScript, Python and PHP, which is why those three rename it to
//! `VClass`, but Rust has no collision at the crate root.

use std::borrow::Cow;
use std::fmt;

/// Declares a string-backed wire enum with `as_str`, `parse`, `Display`
/// and `FromStr`, so a variant and its wire spelling cannot drift apart.
macro_rules! wire_enum {
    (
        $(#[$meta:meta])*
        $name:ident { $( $(#[$vmeta:meta])* $variant:ident => $wire:literal ),+ $(,)? }
    ) => {
        $(#[$meta])*
        #[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
        pub enum $name {
            $( $(#[$vmeta])* $variant, )+
        }

        impl $name {
            /// The RFC wire spelling of this value.
            pub const fn as_str(&self) -> &'static str {
                match self { $( $name::$variant => $wire, )+ }
            }

            /// Folds a wire string onto a variant, case-insensitively.
            ///
            /// Returns `None` for a value outside this vocabulary — the
            /// three status vocabularies are separate types precisely so
            /// a cross-type value does not silently resolve.
            pub fn parse(s: &str) -> Option<Self> {
                $( if s.eq_ignore_ascii_case($wire) { return Some($name::$variant); } )+
                None
            }

            /// Every variant, in declaration order.
            pub const ALL: &'static [$name] = &[ $( $name::$variant, )+ ];
        }

        impl fmt::Display for $name {
            fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
                f.write_str(self.as_str())
            }
        }

        impl std::str::FromStr for $name {
            type Err = crate::Error;
            fn from_str(s: &str) -> Result<Self, Self::Err> {
                Self::parse(s).ok_or_else(|| {
                    crate::Error::Malformed(format!(
                        "{s:?} is not a {} value", stringify!($name)
                    ))
                })
            }
        }

        impl AsRef<str> for $name {
            fn as_ref(&self) -> &str { self.as_str() }
        }
    };
}

wire_enum! {
    /// The wire-string `KIND` value for VCARD objects, RFC 6350 §6.1.4.
    /// Values are lowercase per the RFC's IANA registry.
    Kind {
        /// A person.
        Individual => "individual",
        /// An organization.
        Org => "org",
        /// A group of entities.
        Group => "group",
    }
}

wire_enum! {
    /// The wire-string `STATUS` value for VTODO, RFC 5545 §3.8.1.11.
    TodoStatus {
        /// The to-do needs action.
        NeedsAction => "NEEDS-ACTION",
        /// The to-do is in process.
        InProcess => "IN-PROCESS",
        /// The to-do is complete.
        Completed => "COMPLETED",
        /// The to-do was cancelled.
        Cancelled => "CANCELLED",
    }
}

wire_enum! {
    /// The wire-string `STATUS` value for VEVENT, RFC 5545 §3.8.1.11.
    ///
    /// Deliberately a distinct type from [`TodoStatus`] and
    /// [`JournalStatus`] even though the cancellation value is spelled
    /// identically in all three: the RFC scopes each vocabulary to one
    /// component type, and separate types make a cross-type assignment a
    /// compile error rather than a wire-level conformance bug.
    EventStatus {
        /// The event is tentative.
        Tentative => "TENTATIVE",
        /// The event is confirmed.
        Confirmed => "CONFIRMED",
        /// The event was cancelled.
        Cancelled => "CANCELLED",
    }
}

wire_enum! {
    /// The wire-string `STATUS` value for VJOURNAL, RFC 5545 §3.8.1.11.
    /// See [`EventStatus`] for why the shared cancellation spelling does
    /// not collapse the three vocabularies into one.
    JournalStatus {
        /// The journal entry is a draft.
        Draft => "DRAFT",
        /// The journal entry is final.
        Final => "FINAL",
        /// The journal entry was cancelled.
        Cancelled => "CANCELLED",
    }
}

wire_enum! {
    /// The wire-string `CLASS` value, RFC 5545 §3.8.1.3.
    ///
    /// `CLASS` is optional and the RFC assigns `PUBLIC` when absent;
    /// that default belongs to the helpers layer, not to this type.
    Class {
        /// Public access classification.
        Public => "PUBLIC",
        /// Private access classification.
        Private => "PRIVATE",
        /// Confidential access classification.
        Confidential => "CONFIDENTIAL",
    }
}

wire_enum! {
    /// The wire-string `TRANSP` value, RFC 5545 §3.8.2.7. Applies to
    /// VEVENT only; the RFC assigns `OPAQUE` when absent.
    Transp {
        /// The event blocks free/busy time.
        Opaque => "OPAQUE",
        /// The event does not block free/busy time.
        Transparent => "TRANSPARENT",
    }
}

/// The wire-string component type of a [`Component`](crate::Component).
///
/// **Open, not closed.** RFC 5545 §3.6.5 nests `STANDARD` and `DAYLIGHT`
/// sub-components inside a VTIMEZONE, neither of which is one of the
/// seven named identifiers, and the corpus exercises both. A port that
/// models this as a closed seven-variant enum drops those blocks on
/// parse — a loss round-trip testing cannot see, because both halves lose
/// them identically. The Go reference spells the type as a bare `string`
/// for the same reason.
///
/// `VCARD` is intentionally absent from the named set: vCards are
/// represented by [`Card`](crate::Card), not `Component`.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct CompType(Cow<'static, str>);

/// The RFC 5545 §3.4 + §3.6 component identifiers.
const NAMED_COMP_TYPES: [&str; 7] = [
    "VCALENDAR",
    "VTODO",
    "VJOURNAL",
    "VEVENT",
    "VFREEBUSY",
    "VTIMEZONE",
    "VALARM",
];

macro_rules! comp_ctors {
    ( $( $fname:ident => $wire:literal ),+ $(,)? ) => {
        impl CompType {
            $(
                #[doc = concat!("The `", $wire, "` component identifier.")]
                pub const fn $fname() -> Self { CompType(Cow::Borrowed($wire)) }
            )+
        }
    };
}

comp_ctors! {
    calendar => "VCALENDAR",
    todo => "VTODO",
    journal => "VJOURNAL",
    event => "VEVENT",
    free_busy => "VFREEBUSY",
    timezone => "VTIMEZONE",
    alarm => "VALARM",
}

impl CompType {
    /// The `VCALENDAR` container identifier.
    pub const CALENDAR: CompType = CompType::calendar();
    /// The `VTODO` identifier.
    pub const TODO: CompType = CompType::todo();
    /// The `VJOURNAL` identifier.
    pub const JOURNAL: CompType = CompType::journal();
    /// The `VEVENT` identifier.
    pub const EVENT: CompType = CompType::event();
    /// The `VFREEBUSY` identifier.
    pub const FREE_BUSY: CompType = CompType::free_busy();
    /// The `VTIMEZONE` identifier.
    pub const TIMEZONE: CompType = CompType::timezone();
    /// The `VALARM` identifier.
    pub const ALARM: CompType = CompType::alarm();

    /// Wraps a wire string verbatim.
    ///
    /// An unregistered name — `STANDARD`, `DAYLIGHT`, an `X-` extension —
    /// is kept as written, which is what makes the round-trip lossless.
    pub fn new(s: impl Into<String>) -> Self {
        CompType(Cow::Owned(s.into()))
    }

    /// Wraps a `'static` value without allocating.
    pub const fn from_static(s: &'static str) -> Self {
        CompType(Cow::Borrowed(s))
    }

    /// Folds a wire string onto a named identifier, case-insensitively,
    /// keeping an unregistered name verbatim.
    pub fn from_wire(s: &str) -> Self {
        for named in NAMED_COMP_TYPES {
            if s.eq_ignore_ascii_case(named) {
                return CompType(Cow::Borrowed(named));
            }
        }
        CompType(Cow::Owned(s.to_ascii_uppercase()))
    }

    /// The value as it appears on the wire.
    pub fn as_str(&self) -> &str {
        &self.0
    }

    /// Reports whether this and `s` name the same component type,
    /// comparing case-insensitively.
    pub fn eq_fold(&self, s: &str) -> bool {
        self.0.eq_ignore_ascii_case(s)
    }

    /// Reports whether this is one of the seven named identifiers.
    pub fn is_named(&self) -> bool {
        NAMED_COMP_TYPES
            .iter()
            .any(|n| self.0.eq_ignore_ascii_case(n))
    }

    /// The seven named identifiers, in RFC order.
    pub fn named() -> impl Iterator<Item = CompType> {
        NAMED_COMP_TYPES.iter().map(|n| CompType(Cow::Borrowed(n)))
    }
}

impl fmt::Display for CompType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl AsRef<str> for CompType {
    fn as_ref(&self) -> &str {
        &self.0
    }
}

impl Default for CompType {
    fn default() -> Self {
        CompType::CALENDAR
    }
}

/// The wire-string `RELTYPE` parameter value on a `RELATED-TO` property
/// (RFC 5545 §3.2.15, extended by RFC 9253 §4 and §5).
///
/// The type is deliberately **open**: IANA may register further values
/// and RFC 5545 permits `X-` extensions, so any string is a valid
/// `RelType`. The constructors below name the registered vocabulary;
/// they do not bound it. A port that models this as a closed enum
/// rejecting `X-`-prefixed values is broken.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct RelType(Cow<'static, str>);

/// The registered vocabulary: RFC 5545 §3.2.15 plus RFC 9253 §11.4.
const REGISTERED_REL_TYPES: [&str; 12] = [
    "PARENT",
    "CHILD",
    "SIBLING",
    "FINISHTOSTART",
    "FINISHTOFINISH",
    "STARTTOFINISH",
    "STARTTOSTART",
    "DEPENDS-ON",
    "FIRST",
    "NEXT",
    "CONCEPT",
    "REFID",
];

macro_rules! rel_ctors {
    ( $( $fname:ident => $wire:literal ),+ $(,)? ) => {
        impl RelType {
            $(
                #[doc = concat!("The registered `", $wire, "` relationship type.")]
                pub const fn $fname() -> Self { RelType(Cow::Borrowed($wire)) }
            )+
        }
    };
}

rel_ctors! {
    parent => "PARENT",
    child => "CHILD",
    sibling => "SIBLING",
    finish_to_start => "FINISHTOSTART",
    finish_to_finish => "FINISHTOFINISH",
    start_to_finish => "STARTTOFINISH",
    start_to_start => "STARTTOSTART",
    depends_on => "DEPENDS-ON",
    first => "FIRST",
    next => "NEXT",
    concept => "CONCEPT",
    refid => "REFID",
}

impl RelType {
    /// Wraps a value verbatim, without folding it onto the registry.
    ///
    /// Use [`parse_rel_type`] when you want the registered spelling and
    /// the "was this registered" answer.
    pub fn new(s: impl Into<String>) -> Self {
        RelType(Cow::Owned(s.into()))
    }

    /// Wraps a `'static` value without allocating.
    pub const fn from_static(s: &'static str) -> Self {
        RelType(Cow::Borrowed(s))
    }

    /// The value as it will appear on the wire.
    pub fn as_str(&self) -> &str {
        &self.0
    }

    /// Reports whether this value and `s` name the same relationship
    /// type, comparing case-insensitively per RFC 5545 §3.2.
    pub fn eq_fold(&self, s: &str) -> bool {
        self.0.eq_ignore_ascii_case(s)
    }

    /// Reports whether this value is in the registered vocabulary.
    pub fn is_registered(&self) -> bool {
        REGISTERED_REL_TYPES
            .iter()
            .any(|r| self.0.eq_ignore_ascii_case(r))
    }

    /// The registered vocabulary, in RFC order.
    pub fn registered() -> impl Iterator<Item = RelType> {
        REGISTERED_REL_TYPES
            .iter()
            .map(|r| RelType(Cow::Borrowed(r)))
    }
}

impl fmt::Display for RelType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl AsRef<str> for RelType {
    fn as_ref(&self) -> &str {
        &self.0
    }
}

impl Default for RelType {
    fn default() -> Self {
        DEFAULT_REL_TYPE
    }
}

/// Folds a wire `RELTYPE` value onto a registered constant,
/// case-insensitively per RFC 5545 §3.2. The `bool` reports whether `s`
/// named a registered value.
///
/// This is the one place in the API where a second `bool` return is
/// **not** an optional: the first return is meaningful in both cases. An
/// empty input yields `PARENT` with `true`, matching RFC 5545 §3.2.15's
/// rule that an omitted `RELTYPE` means `PARENT`; an unregistered input
/// is returned **verbatim** with `false`, so a caller accepting
/// extensions keeps the original spelling. A bare `Option<RelType>` is
/// not a conformant shape here.
pub fn parse_rel_type(s: &str) -> (RelType, bool) {
    if s.is_empty() {
        return (default_rel_type(), true);
    }
    for registered in REGISTERED_REL_TYPES {
        if s.eq_ignore_ascii_case(registered) {
            return (RelType(Cow::Borrowed(registered)), true);
        }
    }
    (RelType(Cow::Owned(s.to_owned())), false)
}

/// The value RFC 5545 §3.2.15 assigns when `RELTYPE` is omitted.
pub const DEFAULT_REL_TYPE: RelType = RelType::parent();

/// The value RFC 5545 §3.2.15 assigns when `RELTYPE` is omitted.
///
/// Function form of [`DEFAULT_REL_TYPE`], for call sites that read
/// better as a call.
pub const fn default_rel_type() -> RelType {
    DEFAULT_REL_TYPE
}
