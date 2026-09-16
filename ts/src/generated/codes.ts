/**
 * GENERATED — do not edit.
 * Source: spec/registry/ · Generator: tools/registry/gen.py
 * Run `make registry-gen` after editing the registry JSON.
 */

export const CODES = {
  /** Required common property UID is missing. */
  CodeMissingUID: "VS001",
  /** Required common property DTSTAMP is missing. */
  CodeMissingDTSTAMP: "VS002",
  /** Required common property X-VSTAR-HASH is missing. */
  CodeMissingXVSTARHash: "VS003",
  /** X-VSTAR-HASH is present but does not match the recomputed hash. */
  CodeBadXVSTARHash: "VS010",
  /** Property name is not on the RFC 5545/6350 allow-list and lacks the X- prefix. */
  CodeUnknownProperty: "VS020",
  /** A supersession VJOURNAL (CATEGORIES contains status-supersession) is missing a required property: RELATED-TO or X-VSTAR-EFFECTIVE-STATUS. */
  CodeSupersessionMissingProps: "VS030",
  /** A supersession VJOURNAL's RELATED-TO does not resolve to any component in the same Calendar (orphan supersession). */
  CodeSupersessionOrphan: "VS031",
  /** VTODO requires DUE, OR STATUS=COMPLETED paired with COMPLETED. */
  CodeVTODOMissingDue: "VS040",
  /** VEVENT requires DTSTART. */
  CodeVEVENTMissingDTSTART: "VS041",
  /** VFREEBUSY requires DTSTART AND DTEND. */
  CodeVFREEBUSYMissingTimes: "VS042",
  /** VCARD (modeled as Component{Type: "VCARD"}) requires VERSION AND UID. */
  CodeVCARDMissingRequired: "VS043",
  /** STATUS value is outside the vocabulary RFC 5545 §3.8.1.11 scopes to the component's own type. */
  CodeStatusNotInVocabulary: "VS044",
  /** RRULE value parses but uses a feature outside the v0.2 rrule scope (FREQ=SECONDLY/MINUTELY, RSCALE — see spec/03 §RRULE parsing scope). */
  CodeRRuleUnsupported: "VS050",
  /** RRULE value is malformed per RFC 5545 §3.3.10 (missing FREQ, INTERVAL≤0, both UNTIL+COUNT, BYMONTHDAY=0, UNTIL not in form #2, etc.). */
  CodeRRuleMalformed: "VS051",
  /** A duration-bearing property value is malformed: the DURATION property, the relative (DURATION-valued) form of TRIGGER, or a REPEAT count that is not a non-negative integer (RFC 5545 §3.3.6 / §3.8.6.2); or a TRIGGER whose VALUE parameter contradicts its value, or that carries RELATED on an absolute trigger — see spec/03 §TRIGGER conventions. */
  CodeMalformedDuration: "VS052",
} as const;

export type Code = (typeof CODES)[keyof typeof CODES];

export type Severity = "error" | "warning";

export const CODE_SEVERITIES: Readonly<Record<Code, Severity>> = {
  "VS001": "error",
  "VS002": "error",
  "VS003": "error",
  "VS010": "error",
  "VS020": "warning",
  "VS030": "error",
  "VS031": "error",
  "VS040": "error",
  "VS041": "error",
  "VS042": "error",
  "VS043": "error",
  "VS044": "error",
  "VS050": "warning",
  "VS051": "error",
  "VS052": "error",
};

export const STANDARD_PROPERTIES: readonly string[] = [
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

/**
 * The registry's STATUS value vocabulary, keyed by the component
 * type RFC 5545 §3.8.1.11 scopes it to. A type absent from this
 * record admits no STATUS vocabulary.
 *
 * This is the cross-language table, not the port's wire constants.
 * The validator keeps deriving its table from the constants the
 * codec encodes against, and a test asserts the two agree — so the
 * table cannot drift from the codec, nor from the other ports.
 */
export const STATUS_VOCABULARY: Readonly<Record<string, readonly string[]>> = {
  "VEVENT": ["TENTATIVE", "CONFIRMED", "CANCELLED"],
  "VJOURNAL": ["DRAFT", "FINAL", "CANCELLED"],
  "VTODO": ["NEEDS-ACTION", "IN-PROCESS", "COMPLETED", "CANCELLED"],
};

/** The registry's CLASS vocabulary, RFC 5545 §3.8.1.3. */
export const CLASS_VOCABULARY: readonly string[] = [
  "PUBLIC",
  "PRIVATE",
  "CONFIDENTIAL",
];

/** The registry's TRANSP vocabulary, RFC 5545 §3.8.2.7. */
export const TRANSP_VOCABULARY: readonly string[] = [
  "OPAQUE",
  "TRANSPARENT",
];

/**
 * The registry's registered RELTYPE vocabulary, RFC 5545 §3.2.15 plus RFC
 * 9253 §11.4. Registered, not closed: RELTYPE admits IANA and X- values
 * outside it.
 */
export const RELTYPE_VOCABULARY: readonly string[] = [
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
