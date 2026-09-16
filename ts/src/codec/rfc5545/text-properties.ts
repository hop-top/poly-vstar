// SPDX-License-Identifier: MIT

/**
 * The only iCalendar VERSION value V* honors at v0.1.
 */
export const SUPPORTED_VERSION = "2.0";

/**
 * The allow-list of property names whose values are TEXT-typed per
 * RFC 5545 §3.3.11 / §3.7–§3.8 and RFC 6350 §3.4.
 *
 * Only TEXT values are backslash-escaped on emit. URI, INTEGER,
 * DATE-TIME and other value types pass through verbatim — escaping a
 * comma in a URI would corrupt the address.
 *
 * Custom properties (`X-` extensions and unknown names) are NOT treated
 * as TEXT by default; a producer wanting escape semantics for one must
 * land an entry here.
 */
const TEXT_PROPERTIES: ReadonlySet<string> = new Set([
  // RFC 5545 calendar TEXT properties.
  "CATEGORIES",
  "CLASS",
  "COMMENT",
  "CONTACT",
  "DESCRIPTION",
  "LOCATION",
  "PRODID",
  "RELATED-TO",
  "RESOURCES",
  "STATUS",
  "SUMMARY",
  "TRANSP",
  "TZID",
  "TZNAME",
  "UID",
  // RFC 6350 vCard TEXT properties.
  "FN",
  "N",
  "NICKNAME",
  "NOTE",
  "ORG",
  "TITLE",
  "ROLE",
  "KIND",
]);

/** Whether `name` is TEXT-typed per the allow-list, case-insensitively. */
export function isTextProperty(name: string): boolean {
  return TEXT_PROPERTIES.has(name.toUpperCase());
}
