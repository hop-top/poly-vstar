// GENERATED — do not edit.
// Source: spec/registry/ · Generator: tools/registry/gen.py
// Run `make registry-gen` after editing the registry JSON.

/// Required common property UID is missing.
pub const MISSING_UID: &str = "VS001";

/// Required common property DTSTAMP is missing.
pub const MISSING_DTSTAMP: &str = "VS002";

/// Required common property X-VSTAR-HASH is missing.
pub const MISSING_XVSTAR_HASH: &str = "VS003";

/// X-VSTAR-HASH is present but does not match the recomputed hash.
pub const BAD_XVSTAR_HASH: &str = "VS010";

/// Property name is not on the RFC 5545/6350 allow-list and lacks the X- prefix.
pub const UNKNOWN_PROPERTY: &str = "VS020";

/// A supersession VJOURNAL (CATEGORIES contains status-supersession) is missing a required property: RELATED-TO or X-VSTAR-EFFECTIVE-STATUS.
pub const SUPERSESSION_MISSING_PROPS: &str = "VS030";

/// A supersession VJOURNAL's RELATED-TO does not resolve to any component in the same Calendar (orphan supersession).
pub const SUPERSESSION_ORPHAN: &str = "VS031";

/// VTODO requires DUE, OR STATUS=COMPLETED paired with COMPLETED.
pub const VTODO_MISSING_DUE: &str = "VS040";

/// VEVENT requires DTSTART.
pub const VEVENT_MISSING_DTSTART: &str = "VS041";

/// VFREEBUSY requires DTSTART AND DTEND.
pub const VFREEBUSY_MISSING_TIMES: &str = "VS042";

/// VCARD (modeled as Component{Type: "VCARD"}) requires VERSION AND UID.
pub const VCARD_MISSING_REQUIRED: &str = "VS043";

/// STATUS value is outside the vocabulary RFC 5545 §3.8.1.11 scopes to the component's own type.
pub const STATUS_NOT_IN_VOCABULARY: &str = "VS044";

/// RRULE value parses but uses a feature outside the RRULE parsing scope (FREQ=SECONDLY, RSCALE — see spec/03 §RRULE parsing scope).
pub const R_RULE_UNSUPPORTED: &str = "VS050";

/// RRULE value is malformed per RFC 5545 §3.3.10 (missing FREQ, INTERVAL≤0, both UNTIL+COUNT, BYMONTHDAY=0, UNTIL not in form #2, etc.).
pub const R_RULE_MALFORMED: &str = "VS051";

/// A duration-bearing property value is malformed: the DURATION property, the relative (DURATION-valued) form of TRIGGER, or a REPEAT count that is not a non-negative integer (RFC 5545 §3.3.6 / §3.8.6.2) written as a canonical decimal — no sign, no leading zeros, no whitespace (see spec/05 §8); or a TRIGGER whose VALUE parameter contradicts its value, or that carries RELATED on an absolute trigger — see spec/03 §TRIGGER conventions.
pub const MALFORMED_DURATION: &str = "VS052";

/// CLASS value is outside the RFC 5545 §3.8.1.3 vocabulary PUBLIC, PRIVATE, CONFIDENTIAL (compared case-insensitively — see spec/05 §8).
pub const CLASS_NOT_IN_VOCABULARY: &str = "VS053";

/// TRANSP value is outside the RFC 5545 §3.8.2.7 vocabulary OPAQUE, TRANSPARENT (compared case-insensitively — see spec/05 §8).
pub const TRANSP_NOT_IN_VOCABULARY: &str = "VS054";

/// An integer-valued property is not a canonical decimal (no sign, no leading zeros, no whitespace) inside its RFC 5545 domain: PRIORITY 0–9 (§3.8.1.9), PERCENT-COMPLETE 0–100 (§3.8.1.8), SEQUENCE non-negative (§3.8.7.4) — see spec/05 §8.
pub const INTEGER_OUT_OF_DOMAIN: &str = "VS055";

/// Every diagnostic code, sorted by code.
pub const CODES: [&str; 18] = [
    "VS001",
    "VS002",
    "VS003",
    "VS010",
    "VS020",
    "VS030",
    "VS031",
    "VS040",
    "VS041",
    "VS042",
    "VS043",
    "VS044",
    "VS050",
    "VS051",
    "VS052",
    "VS053",
    "VS054",
    "VS055",
];

/// Severity for each code, index-aligned with `CODES`.
pub const CODE_SEVERITIES: [(&str, &str); 18] = [
    ("VS001", "error"),
    ("VS002", "error"),
    ("VS003", "error"),
    ("VS010", "error"),
    ("VS020", "warning"),
    ("VS030", "error"),
    ("VS031", "error"),
    ("VS040", "error"),
    ("VS041", "error"),
    ("VS042", "error"),
    ("VS043", "error"),
    ("VS044", "error"),
    ("VS050", "warning"),
    ("VS051", "error"),
    ("VS052", "error"),
    ("VS053", "error"),
    ("VS054", "error"),
    ("VS055", "error"),
];

/// RFC 5545 / RFC 6350 standard property allow-list.
pub const STANDARD_PROPERTIES: [&str; 77] = [
    "ACTION",
    "ADR",
    "ANNIVERSARY",
    "ATTACH",
    "ATTENDEE",
    "BDAY",
    "CALADRURI",
    "CALSCALE",
    "CALURI",
    "CATEGORIES",
    "CLASS",
    "CLIENTPIDMAP",
    "COMMENT",
    "COMPLETED",
    "CONTACT",
    "CREATED",
    "DESCRIPTION",
    "DTEND",
    "DTSTAMP",
    "DTSTART",
    "DUE",
    "DURATION",
    "EMAIL",
    "EXDATE",
    "EXRULE",
    "FBURL",
    "FN",
    "FREEBUSY",
    "GENDER",
    "GEO",
    "IMPP",
    "KEY",
    "KIND",
    "LANG",
    "LAST-MODIFIED",
    "LOCATION",
    "LOGO",
    "MEMBER",
    "METHOD",
    "N",
    "NICKNAME",
    "NOTE",
    "ORG",
    "ORGANIZER",
    "PERCENT-COMPLETE",
    "PHOTO",
    "PRIORITY",
    "PRODID",
    "RDATE",
    "RECURRENCE-ID",
    "RELATED",
    "RELATED-TO",
    "REPEAT",
    "REQUEST-STATUS",
    "RESOURCES",
    "REV",
    "ROLE",
    "RRULE",
    "SEQUENCE",
    "SOUND",
    "SOURCE",
    "STATUS",
    "SUMMARY",
    "TEL",
    "TITLE",
    "TRANSP",
    "TRIGGER",
    "TZ",
    "TZID",
    "TZNAME",
    "TZOFFSETFROM",
    "TZOFFSETTO",
    "TZURL",
    "UID",
    "URL",
    "VERSION",
    "XML",
];

/// The registry's `STATUS` vocabulary, paired with the component
/// type RFC 5545 §3.8.1.11 scopes it to. A type absent from this
/// table admits no `STATUS` vocabulary.
///
/// This is the cross-language table, not the crate's wire enums.
/// The validator keeps deriving its table from the enums the codec
/// encodes against, and a test asserts the two agree — so the table
/// cannot drift from the codec, nor from the other ports.
pub const STATUS_VOCABULARY: [(&str, &[&str]); 3] = [
    ("VEVENT", &["TENTATIVE", "CONFIRMED", "CANCELLED"]),
    ("VJOURNAL", &["DRAFT", "FINAL", "CANCELLED"]),
    ("VTODO", &["NEEDS-ACTION", "IN-PROCESS", "COMPLETED", "CANCELLED"]),
];

/// The registry's CLASS vocabulary, RFC 5545 §3.8.1.3.
pub const CLASS_VOCABULARY: [&str; 3] = [
    "PUBLIC",
    "PRIVATE",
    "CONFIDENTIAL",
];

/// The registry's TRANSP vocabulary, RFC 5545 §3.8.2.7.
pub const TRANSP_VOCABULARY: [&str; 2] = [
    "OPAQUE",
    "TRANSPARENT",
];

/// The registry's registered RELTYPE vocabulary, RFC 5545 §3.2.15
/// plus RFC 9253 §11.4. Registered, not closed: RELTYPE admits IANA
/// and X- values outside it.
pub const RELTYPE_VOCABULARY: [&str; 12] = [
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
