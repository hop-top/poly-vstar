// SPDX-License-Identifier: MIT

// The V* in-memory data model: Property, Param, Component, Calendar,
// Card, plus the wire-string enums every later layer builds on.
//
// Wire values are normative. A port that changes a spelling is broken.

/**
 * A single property parameter (e.g. `CN=Jad` on `ATTENDEE`).
 *
 * Name comparisons are case-insensitive per RFC 5545 §3.2 / RFC 6350
 * §5; value comparisons are case-sensitive at this layer.
 */
export interface Param {
  name: string;
  value: string;
}

/**
 * A single iCalendar/vCard content line in struct form: a name, zero
 * or more parameters, and a value. The wire format is the codec's
 * business — this layer is pure data.
 */
export interface Property {
  name: string;
  params: Param[];
  value: string;
}

/**
 * A single iCalendar component (VEVENT, VTODO, VCALENDAR, …): a typed
 * identifier, a property list, and nested sub-components.
 */
export interface Component {
  type: CompType;
  props: Property[];
  sub: Component[];
}

/**
 * The top-level VCALENDAR container per RFC 5545 §3.4: the PRODID of
 * the producing system plus its contained components.
 */
export interface Calendar {
  prodId: string;
  components: Component[];
}

/**
 * A top-level VCARD per RFC 6350. Structurally analogous to
 * {@link Component} but distinct: vCards do not nest sub-components
 * and carry no {@link CompType}.
 */
export interface Card {
  uid: string;
  kind: Kind;
  props: Property[];
}

/**
 * The wire-string component type of a {@link Component}. `VCARD` is
 * deliberately absent — vCards are {@link Card}s, not Components.
 */
export type CompType =
  | "VCALENDAR"
  | "VTODO"
  | "VJOURNAL"
  | "VEVENT"
  | "VFREEBUSY"
  | "VTIMEZONE"
  | "VALARM";

export const COMP_CALENDAR = "VCALENDAR" as const satisfies CompType;
export const COMP_TODO = "VTODO" as const satisfies CompType;
export const COMP_JOURNAL = "VJOURNAL" as const satisfies CompType;
export const COMP_EVENT = "VEVENT" as const satisfies CompType;
export const COMP_FREE_BUSY = "VFREEBUSY" as const satisfies CompType;
export const COMP_TIMEZONE = "VTIMEZONE" as const satisfies CompType;
export const COMP_ALARM = "VALARM" as const satisfies CompType;

/**
 * The wire-string KIND value for a {@link Card} per RFC 6350 §6.1.4.
 * Values are lowercase per the RFC's IANA registry. The empty string
 * means the property is absent.
 */
export type Kind = "individual" | "org" | "group" | "";

export const KIND_INDIVIDUAL = "individual" as const satisfies Kind;
export const KIND_ORG = "org" as const satisfies Kind;
export const KIND_GROUP = "group" as const satisfies Kind;

/**
 * The wire-string STATUS value for a VTODO per RFC 5545 §3.8.1.11.
 *
 * `TodoStatus`, {@link EventStatus} and {@link JournalStatus} are
 * deliberately distinct types even though the cancellation value is
 * spelled identically in all three: the RFC scopes each vocabulary to
 * one component type, and separate types make a cross-type assignment
 * a type error rather than a wire-level conformance bug.
 */
export type TodoStatus = "NEEDS-ACTION" | "IN-PROCESS" | "COMPLETED" | "CANCELLED";

export const TODO_NEEDS_ACTION = "NEEDS-ACTION" as const satisfies TodoStatus;
export const TODO_IN_PROCESS = "IN-PROCESS" as const satisfies TodoStatus;
export const TODO_COMPLETED = "COMPLETED" as const satisfies TodoStatus;
export const TODO_CANCELLED = "CANCELLED" as const satisfies TodoStatus;

/** The wire-string STATUS value for a VEVENT per RFC 5545 §3.8.1.11. */
export type EventStatus = "TENTATIVE" | "CONFIRMED" | "CANCELLED";

export const EVENT_TENTATIVE = "TENTATIVE" as const satisfies EventStatus;
export const EVENT_CONFIRMED = "CONFIRMED" as const satisfies EventStatus;
export const EVENT_CANCELLED = "CANCELLED" as const satisfies EventStatus;

/** The wire-string STATUS value for a VJOURNAL per RFC 5545 §3.8.1.11. */
export type JournalStatus = "DRAFT" | "FINAL" | "CANCELLED";

export const JOURNAL_DRAFT = "DRAFT" as const satisfies JournalStatus;
export const JOURNAL_FINAL = "FINAL" as const satisfies JournalStatus;
export const JOURNAL_CANCELLED = "CANCELLED" as const satisfies JournalStatus;

/**
 * The wire-string CLASS value per RFC 5545 §3.8.1.3.
 *
 * Named `VClass` rather than `Class` because `class` is a reserved
 * word — see the API mapping.
 */
export type VClass = "PUBLIC" | "PRIVATE" | "CONFIDENTIAL";

export const CLASS_PUBLIC = "PUBLIC" as const satisfies VClass;
export const CLASS_PRIVATE = "PRIVATE" as const satisfies VClass;
export const CLASS_CONFIDENTIAL = "CONFIDENTIAL" as const satisfies VClass;

/** The wire-string TRANSP value per RFC 5545 §3.8.2.7. */
export type Transp = "OPAQUE" | "TRANSPARENT";

export const TRANSP_OPAQUE = "OPAQUE" as const satisfies Transp;
export const TRANSP_TRANSPARENT = "TRANSPARENT" as const satisfies Transp;

/**
 * The wire-string RELTYPE parameter value on a RELATED-TO property
 * (RFC 5545 §3.2.15, extended by RFC 9253 §4 and §5).
 *
 * The type is deliberately **open**: IANA may register further values
 * and RFC 5545 permits `X-`-prefixed extensions, so any string is a
 * valid `RelType`. The constants below name the registered vocabulary;
 * they do not bound it.
 */
export type RelType = string;

// RFC 5545 §3.2.15 hierarchical relationship types.
export const REL_PARENT = "PARENT" as const satisfies RelType;
export const REL_CHILD = "CHILD" as const satisfies RelType;
export const REL_SIBLING = "SIBLING" as const satisfies RelType;

// RFC 9253 §4 temporal relationship types.
export const REL_FINISH_TO_START = "FINISHTOSTART" as const satisfies RelType;
export const REL_FINISH_TO_FINISH = "FINISHTOFINISH" as const satisfies RelType;
export const REL_START_TO_FINISH = "STARTTOFINISH" as const satisfies RelType;
export const REL_START_TO_START = "STARTTOSTART" as const satisfies RelType;

// RFC 9253 §5 relationship types.
export const REL_DEPENDS_ON = "DEPENDS-ON" as const satisfies RelType;
export const REL_FIRST = "FIRST" as const satisfies RelType;
export const REL_NEXT = "NEXT" as const satisfies RelType;
export const REL_CONCEPT = "CONCEPT" as const satisfies RelType;
export const REL_REF_ID = "REFID" as const satisfies RelType;

/** The value RFC 5545 §3.2.15 assigns when RELTYPE is omitted. */
export const DEFAULT_REL_TYPE = REL_PARENT;

/** The registered RELTYPE vocabulary, keyed by uppercase wire string. */
const REL_TYPES: ReadonlyMap<string, RelType> = new Map(
  [
    REL_PARENT,
    REL_CHILD,
    REL_SIBLING,
    REL_FINISH_TO_START,
    REL_FINISH_TO_FINISH,
    REL_START_TO_FINISH,
    REL_START_TO_START,
    REL_DEPENDS_ON,
    REL_FIRST,
    REL_NEXT,
    REL_CONCEPT,
    REL_REF_ID,
  ].map((v) => [v, v]),
);

/**
 * Fold a wire RELTYPE value onto a registered constant,
 * case-insensitively per RFC 5545 §3.2, reporting whether `s` named a
 * registered value.
 *
 * This is the one place in the API where the second return is **not**
 * an optional: the first element is meaningful in both cases. An empty
 * input yields `PARENT` with `true` (RFC 5545 §3.2.15: an omitted
 * RELTYPE means PARENT); an unregistered input is returned verbatim
 * with `false`, so a caller accepting extensions keeps the original
 * spelling.
 */
export function parseRelType(s: string): [RelType, boolean] {
  if (s === "") return [DEFAULT_REL_TYPE, true];
  const found = REL_TYPES.get(s.toUpperCase());
  if (found !== undefined) return [found, true];
  return [s, false];
}

/**
 * Report whether `r` and `s` name the same RELTYPE, comparing
 * case-insensitively per RFC 5545 §3.2.
 */
export function relTypeEqualFold(r: RelType, s: string): boolean {
  return r.toUpperCase() === s.toUpperCase();
}

/**
 * Report whether two properties are semantically equal:
 * case-insensitive name and parameter-name comparisons, case-sensitive
 * value comparisons, and parameter order normalized alphabetically
 * before comparing. Inputs are not mutated.
 */
export function propertyEqual(a: Property, b: Property): boolean {
  if (a.name.toUpperCase() !== b.name.toUpperCase()) return false;
  if (a.value !== b.value) return false;
  if (a.params.length !== b.params.length) return false;
  const ap = sortedParams(a.params);
  const bp = sortedParams(b.params);
  for (let i = 0; i < ap.length; i++) {
    const x = ap[i] as Param;
    const y = bp[i] as Param;
    if (x.name.toUpperCase() !== y.name.toUpperCase()) return false;
    if (x.value !== y.value) return false;
  }
  return true;
}

/** A copy of `params` sorted by uppercased name. The input is not mutated. */
function sortedParams(params: readonly Param[]): Param[] {
  return [...params].sort((x, y) => {
    const a = x.name.toUpperCase();
    const b = y.name.toUpperCase();
    return a < b ? -1 : a > b ? 1 : 0;
  });
}

/**
 * The first property whose name matches `name` (case-insensitive per
 * RFC 5545 §3.1), or `undefined` when none match.
 */
export function get(c: Component, name: string): Property | undefined {
  return c.props.find((p) => p.name.toUpperCase() === name.toUpperCase());
}

/** Every property whose name matches `name` (case-insensitive). */
export function getAll(c: Component, name: string): Property[] {
  return c.props.filter((p) => p.name.toUpperCase() === name.toUpperCase());
}

/**
 * Replace every property matching `p.name` (case-insensitive) with a
 * single copy of `p`, in the first match's position. Appends when no
 * property matches.
 */
export function set(c: Component, p: Property): void {
  c.props = replaceProps(c.props, p);
}

/** Append `p` without touching existing properties of the same name. */
export function add(c: Component, p: Property): void {
  c.props.push(p);
}

/** Delete every property matching `name` (case-insensitive). */
export function remove(c: Component, name: string): void {
  c.props = c.props.filter((p) => p.name.toUpperCase() !== name.toUpperCase());
}

/**
 * The component's UID property value, or `""` when absent.
 * Convenience for the universally-required identifier
 * (RFC 5545 §3.8.4.7 / RFC 6350 §6.7.6).
 */
export function uid(c: Component): string {
  return get(c, "UID")?.value ?? "";
}

/**
 * The component's DTSTAMP property value as the raw RFC 5545 wire
 * string, or `""` when absent.
 */
export function dtstampRaw(c: Component): string {
  return get(c, "DTSTAMP")?.value ?? "";
}

/** The first property of `card` matching `name` (case-insensitive). */
export function cardGet(c: Card, name: string): Property | undefined {
  return c.props.find((p) => p.name.toUpperCase() === name.toUpperCase());
}

/** Every property of `card` matching `name` (case-insensitive). */
export function cardGetAll(c: Card, name: string): Property[] {
  return c.props.filter((p) => p.name.toUpperCase() === name.toUpperCase());
}

/** {@link set}, for a {@link Card}. */
export function cardSet(c: Card, p: Property): void {
  c.props = replaceProps(c.props, p);
}

/** {@link add}, for a {@link Card}. */
export function cardAdd(c: Card, p: Property): void {
  c.props.push(p);
}

/** {@link remove}, for a {@link Card}. */
export function cardRemove(c: Card, name: string): void {
  c.props = c.props.filter((p) => p.name.toUpperCase() !== name.toUpperCase());
}

/** Shared replace-or-append used by {@link set} and {@link cardSet}. */
function replaceProps(props: readonly Property[], p: Property): Property[] {
  const out: Property[] = [];
  let replaced = false;
  for (const existing of props) {
    if (existing.name.toUpperCase() === p.name.toUpperCase()) {
      if (!replaced) {
        out.push(p);
        replaced = true;
      }
      continue;
    }
    out.push(existing);
  }
  if (!replaced) out.push(p);
  return out;
}

/**
 * The first component whose UID matches `uid`, or `undefined`.
 *
 * UID comparison is case-sensitive per RFC 5545 §3.8.4.7 — UIDs are
 * opaque identifiers, not user-facing text.
 */
export function find(cal: Calendar, wanted: string): Component | undefined {
  return cal.components.find((c) => uid(c) === wanted);
}

/** Append `comp` to the calendar's component list. */
export function append(cal: Calendar, comp: Component): void {
  cal.components.push(comp);
}

/**
 * Every component of the requested type. CompType comparison is
 * case-sensitive: components carry the wire string verbatim and the
 * constants are uppercase per RFC 5545 §3.6.
 */
export function filter(cal: Calendar, t: CompType): Component[] {
  return cal.components.filter((c) => c.type === t);
}
