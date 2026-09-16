// SPDX-License-Identifier: MIT

/**
 * The `diff` subpath export: semantic equality and structural diff for
 * V* values.
 *
 * "Semantic" means *via canonical form*: two values yielding identical
 * canonical bytes are equal regardless of property order, parameter
 * order, datetime form or whitespace. The `*Equal` family answers that
 * question; the `of*` family reports the property-level changes.
 *
 * ## Ordering is the contract
 *
 * The parity contract deliberately does **not** sort diff output as a
 * whole. Components come back in *pairing order* — UID-bearing pairs
 * sorted by (type, uid), then positionally-paired UID-less buckets by
 * type — and each component's `properties` are sorted by property name
 * case-insensitively. A port that re-sorts the component list to make
 * its output "tidier" fails `spec/behavior/diff/`.
 *
 * ## Pairing heuristics, and their v0.1 limits
 *
 * Sub-components pair by `(type, uid)` when both carry a UID, and
 * positionally by index within a type bucket when they do not (a
 * `VALARM`, typically). Reordering UID-less sub-components therefore
 * surfaces as an add plus a remove rather than a change.
 *
 * `X-VSTAR-HASH` is excluded from both sides, mirroring the
 * canonicalization rule: a diff reports what changed in the content,
 * not the restamped hash that followed.
 */

import { calendar as canonicalCalendar, card as canonicalCard, component as canonicalComponent } from "../canonical/index.js";
import type { Calendar, Card, Component, Param, Property } from "../types.js";
import { propertyEqual as modelPropertyEqual, uid as componentUid } from "../types.js";

export { version } from "../version.js";

/** The wire name excluded from diff input on both sides (spec/03 rule 7). */
const X_VSTAR_HASH = "X-VSTAR-HASH";

/**
 * The kind of property change a {@link PropertyDiff} records.
 *
 * The tokens are the lowercase spellings `spec/behavior/diff/` uses.
 * There is deliberately no zero/default member: in the Go reference
 * `OpAdded` starts at 1 so an un-set `DiffOp` is not a valid one, and a
 * string union reproduces that without a numeric convention.
 */
export type DiffOp = "add" | "remove" | "change";

/** Present in `b` but missing from `a`. */
export const OP_ADDED = "add" as const satisfies DiffOp;
/** Present in `a` but missing from `b`. */
export const OP_REMOVED = "remove" as const satisfies DiffOp;
/** Present in both, with a differing value or differing parameters. */
export const OP_CHANGED = "change" as const satisfies DiffOp;

/**
 * The reference's human-readable spelling of a {@link DiffOp} —
 * `"Added"`, `"Removed"`, `"Changed"`. Display text for
 * {@link componentDiffToString}, not the contract.
 */
export function diffOpToString(op: DiffOp): string {
  switch (op) {
    case OP_ADDED:
      return "Added";
    case OP_REMOVED:
      return "Removed";
    case OP_CHANGED:
      return "Changed";
  }
}

/**
 * One property-level change.
 *
 * For an add, `property` is the new (b-side) property and `old` is
 * undefined. For a remove, `property` is the original (a-side) property
 * and `old` is undefined. For a change, `property` is the new property
 * and `old` the original — which is why a parameter-only change is a
 * change op whose two values are equal and whose parameter lists differ.
 */
export interface PropertyDiff {
  readonly op: DiffOp;
  readonly property: Property;
  readonly old: Property | undefined;
}

/**
 * The structural changes between two components (or two cards, or one
 * paired calendar entry).
 *
 * `path` locates the diff site: `""` for a top-level component or card
 * diff, `VCALENDAR.<TYPE>[uid=…]` for a {@link ofCalendar} entry, and
 * `<parent>.<TYPE>[uid=…]` or `<parent>.<TYPE>[#<n>]` for a nested
 * sub-diff.
 */
export interface ComponentDiff {
  readonly path: string;
  readonly properties: PropertyDiff[];
  readonly subDiffs: ComponentDiff[];
}

/**
 * Report whether `d` records no change at this level nor in any nested
 * sub-component.
 */
export function isEmpty(d: ComponentDiff): boolean {
  if (d.properties.length > 0) return false;
  return d.subDiffs.every(isEmpty);
}

/**
 * Report whether two components are semantically equal — their
 * canonical bytes are identical.
 *
 * Equality therefore ignores property order, parameter order and the
 * `X-VSTAR-HASH` property, which canonicalization strips. For
 * components carrying TZID-tagged datetimes that need a sibling
 * `VTIMEZONE`, compare through {@link calendarEqual} instead so the
 * registry is in scope.
 *
 * The name carries the `Equal` suffix because Go's `diff.Component`
 * reads correctly only with its package qualifier supplying the verb;
 * a bare `component(a, b)` returning a boolean would be unreadable and
 * would collide with `canonical.component`.
 */
export function componentEqual(a: Component, b: Component): boolean {
  return bytesEqual(canonicalComponent(a), canonicalComponent(b));
}

/** Report whether two cards are semantically equal via their canonical bytes. */
export function cardEqual(a: Card, b: Card): boolean {
  return bytesEqual(canonicalCard(a), canonicalCard(b));
}

/**
 * Report whether two calendars are semantically equal via their
 * canonical bytes.
 *
 * This handles top-level component reordering (canonicalization sorts
 * by UID/TZID per spec/03 rule 6) and resolves TZID-tagged datetimes
 * against the calendar's own `VTIMEZONE` registry.
 */
export function calendarEqual(a: Calendar, b: Calendar): boolean {
  return bytesEqual(canonicalCalendar(a), canonicalCalendar(b));
}

/**
 * Report whether two properties are semantically equal:
 * case-insensitive name and parameter names, case-sensitive values,
 * parameter order irrelevant.
 *
 * Re-exported here under the `Equal` suffix that the API mapping's
 * collision rule assigns the whole family; the implementation is the
 * model's own.
 */
export function propertyEqual(a: Property, b: Property): boolean {
  return modelPropertyEqual(a, b);
}

/**
 * The property-level structural difference between two components.
 *
 * Both sides are first stripped of `X-VSTAR-HASH`, so the diff never
 * reports hash drift as a content change. Properties pair by name
 * case-insensitively and the result is sorted by name; sub-components
 * pair by `(type, uid)` when both carry one and positionally otherwise.
 */
export function ofComponent(a: Component, b: Component): ComponentDiff {
  return componentDiffAt("", a, b);
}

/**
 * The property-level structural difference between two cards.
 *
 * Cards carry no sub-components, so `subDiffs` is always empty, and
 * `path` is `""` so the result can serve directly as a rendering root.
 */
export function ofCard(a: Card, b: Card): ComponentDiff {
  return {
    path: "",
    properties: diffProperties(filterHashProps(a.props), filterHashProps(b.props)),
    subDiffs: [],
  };
}

/**
 * The per-component structural difference between two calendars.
 *
 * Components pair by `(type, uid)`, the same rule the sub-component
 * matcher uses; each differing pair becomes one entry with path
 * `VCALENDAR.<TYPE>[uid=…]`, and a component present on only one side
 * becomes an all-added or all-removed entry. Unchanged components do
 * not appear.
 *
 * `PRODID` is deliberately not diffed — the calendar's identity, for
 * this purpose, is its component set. Use {@link calendarEqual} when
 * `PRODID` matters.
 */
export function ofCalendar(a: Calendar, b: Calendar): ComponentDiff[] {
  const out: ComponentDiff[] = [];
  for (const pr of pairSubs(a.components, b.components)) {
    const path = subPath("VCALENDAR", pr.label);
    if (pr.a === undefined && pr.b !== undefined) {
      out.push(allAddedDiff(path, pr.b));
    } else if (pr.a !== undefined && pr.b === undefined) {
      out.push(allRemovedDiff(path, pr.a));
    } else if (pr.a !== undefined && pr.b !== undefined) {
      const cd = componentDiffAt(path, pr.a, pr.b);
      if (!isEmpty(cd)) out.push(cd);
    }
  }
  return out;
}

/**
 * Render a {@link ComponentDiff} as a unified-diff-ish text block:
 *
 * ```text
 * --- <path>
 * + NAME[;PARAM=VAL...]:VALUE
 * - NAME[;PARAM=VAL...]:VALUE
 * ~ NAME: <old value> -> <new value>
 * ```
 *
 * Sub-diffs indent two spaces per level, each opening with its own
 * `--- <path>` header. An empty diff renders as `""`.
 *
 * Both sides collapse into a single `---` header, rather than the
 * unified-diff tradition's `---`/`+++` pair, because each block
 * represents the changes *between* two sides, not one side in
 * isolation.
 *
 * This output is informational — for humans and debug tooling. It is
 * not a wire format and is not covered by the cross-implementation
 * parity guarantees.
 */
export function componentDiffToString(d: ComponentDiff): string {
  if (isEmpty(d)) return "";
  const lines: string[] = [];
  writeTo(d, lines, 0);
  return lines.join("");
}

/** The recursive worker behind {@link ofComponent} and {@link ofCalendar}. */
function componentDiffAt(path: string, a: Component, b: Component): ComponentDiff {
  return {
    path,
    properties: diffProperties(filterHashProps(a.props), filterHashProps(b.props)),
    subDiffs: diffSubs(path, a, b),
  };
}

/** A copy of `props` with any `X-VSTAR-HASH` property dropped. */
function filterHashProps(props: readonly Property[]): Property[] {
  return props.filter((p) => p.name.toUpperCase() !== X_VSTAR_HASH);
}

/**
 * Pair properties by name (case-insensitive) and emit the changes
 * sorted by that uppercased name.
 *
 * Properties group by name first so multi-valued properties (several
 * `ATTENDEE`s, say) stay intact; within a group the i-th instance on
 * each side pairs with the other, and any surplus becomes an add or a
 * remove.
 */
function diffProperties(a: readonly Property[], b: readonly Property[]): PropertyDiff[] {
  const ga = groupByName(a);
  const gb = groupByName(b);
  const keys = [...new Set([...ga.keys(), ...gb.keys()])].sort(compareStrings);

  const out: PropertyDiff[] = [];
  for (const k of keys) {
    out.push(...diffPropertyGroup(ga.get(k) ?? [], gb.get(k) ?? []));
  }
  return out;
}

/** Bucket `props` by uppercased name, preserving input order in each bucket. */
function groupByName(props: readonly Property[]): Map<string, Property[]> {
  const m = new Map<string, Property[]>();
  for (const p of props) {
    const key = p.name.toUpperCase();
    const bucket = m.get(key);
    if (bucket === undefined) m.set(key, [p]);
    else bucket.push(p);
  }
  return m;
}

/** The changes for one property name across both sides. */
function diffPropertyGroup(a: readonly Property[], b: readonly Property[]): PropertyDiff[] {
  const out: PropertyDiff[] = [];
  const n = Math.max(a.length, b.length);
  for (let i = 0; i < n; i++) {
    const av = a[i];
    const bv = b[i];
    if (av === undefined && bv !== undefined) {
      out.push({ op: OP_ADDED, property: bv, old: undefined });
    } else if (av !== undefined && bv === undefined) {
      out.push({ op: OP_REMOVED, property: av, old: undefined });
    } else if (av !== undefined && bv !== undefined && !modelPropertyEqual(av, bv)) {
      out.push({ op: OP_CHANGED, property: bv, old: av });
    }
  }
  return out;
}

/** Pair sub-components, recurse, and keep only the sub-diffs that changed. */
function diffSubs(parentPath: string, a: Component, b: Component): ComponentDiff[] {
  const out: ComponentDiff[] = [];
  for (const pr of pairSubs(a.sub, b.sub)) {
    const path = subPath(parentPath, pr.label);
    if (pr.a === undefined && pr.b !== undefined) {
      out.push(allAddedDiff(path, pr.b));
    } else if (pr.a !== undefined && pr.b === undefined) {
      out.push(allRemovedDiff(path, pr.a));
    } else if (pr.a !== undefined && pr.b !== undefined) {
      const cd = componentDiffAt(path, pr.a, pr.b);
      if (!isEmpty(cd)) out.push(cd);
    }
  }
  return out;
}

/** One paired set of sub-components plus the label segment naming it. */
interface SubPair {
  readonly a: Component | undefined;
  readonly b: Component | undefined;
  /** `VALARM[uid=…]` or `VALARM[#0]`. */
  readonly label: string;
}

/**
 * Match sub-components by `(type, uid)`, falling back to positional
 * pairing within a type bucket for UID-less children.
 *
 * The returned order is the contract: UID-bearing pairs first, sorted
 * by type then UID, then the UID-less buckets in type order with their
 * positional pairs in index order.
 */
function pairSubs(aSub: readonly Component[], bSub: readonly Component[]): SubPair[] {
  const aByKey = new Map<string, Component>();
  const bByKey = new Map<string, Component>();
  const aByType = new Map<string, Component[]>();
  const bByType = new Map<string, Component[]>();

  const collect = (
    subs: readonly Component[],
    byKey: Map<string, Component>,
    byType: Map<string, Component[]>,
  ): void => {
    for (const s of subs) {
      const u = componentUid(s);
      if (u === "") {
        const bucket = byType.get(s.type);
        if (bucket === undefined) byType.set(s.type, [s]);
        else bucket.push(s);
        continue;
      }
      byKey.set(keyOf(s.type, u), s);
    }
  };
  collect(aSub, aByKey, aByType);
  collect(bSub, bByKey, bByType);

  const out: SubPair[] = [];

  const keys = [...new Set([...aByKey.keys(), ...bByKey.keys()])].sort(compareKeys);
  for (const k of keys) {
    const [typ, u] = splitKey(k);
    out.push({ a: aByKey.get(k), b: bByKey.get(k), label: `${typ}[uid=${u}]` });
  }

  const types = [...new Set([...aByType.keys(), ...bByType.keys()])].sort(compareStrings);
  for (const t of types) {
    const as = aByType.get(t) ?? [];
    const bs = bByType.get(t) ?? [];
    const n = Math.max(as.length, bs.length);
    for (let i = 0; i < n; i++) {
      out.push({ a: as[i], b: bs[i], label: `${t}[#${i}]` });
    }
  }
  return out;
}

/**
 * Encode a `(type, uid)` pair into one sortable key.
 *
 * ` ` separates the halves: it cannot occur in either a component
 * type or a UID, so the encoded key sorts exactly as Go's two-field
 * comparison does (type first, then UID) without a tuple comparator.
 */
function keyOf(typ: string, u: string): string {
  return `${typ} ${u}`;
}

/** Undo {@link keyOf}. */
function splitKey(k: string): [typ: string, uid: string] {
  const i = k.indexOf(" ");
  return [k.slice(0, i), k.slice(i + 1)];
}

/** Join a parent path with a sub-component label. */
function subPath(parent: string, label: string): string {
  return parent === "" ? label : `${parent}.${label}`;
}

/** Render an entire component as added, recursing into its sub-components. */
function allAddedDiff(path: string, c: Component): ComponentDiff {
  return wholeComponentDiff(path, c, OP_ADDED);
}

/** The symmetric counterpart of {@link allAddedDiff}. */
function allRemovedDiff(path: string, c: Component): ComponentDiff {
  return wholeComponentDiff(path, c, OP_REMOVED);
}

/** Shared body of {@link allAddedDiff} and {@link allRemovedDiff}. */
function wholeComponentDiff(path: string, c: Component, op: DiffOp): ComponentDiff {
  const properties: PropertyDiff[] = filterHashProps(c.props).map((p) => ({
    op,
    property: p,
    old: undefined,
  }));
  sortPropertyDiffs(properties);
  return {
    path,
    properties,
    subDiffs: c.sub.map((s) => wholeComponentDiff(subPath(path, subLabel(s)), s, op)),
  };
}

/**
 * The label segment for an unpaired sub-component, mirroring the format
 * {@link pairSubs} chooses. A UID-less child is `[#0]` because an
 * unpaired component has no index to inherit.
 */
function subLabel(c: Component): string {
  const u = componentUid(c);
  return u === "" ? `${c.type}[#0]` : `${c.type}[uid=${u}]`;
}

/**
 * Sort in place by property name, case-insensitively.
 * {@link diffProperties} already emits sorted output; the
 * whole-component paths need this.
 */
function sortPropertyDiffs(pds: PropertyDiff[]): void {
  pds.sort((x, y) => compareStrings(x.property.name.toUpperCase(), y.property.name.toUpperCase()));
}

/** Render `d` into `lines` at the given indent depth. */
function writeTo(d: ComponentDiff, lines: string[], depth: number): void {
  const indent = "  ".repeat(depth);
  lines.push(`${indent}--- ${d.path}\n`);
  for (const pd of d.properties) {
    lines.push(`${indent}${renderPropertyDiff(pd)}\n`);
  }
  for (const sd of d.subDiffs) {
    if (isEmpty(sd)) continue;
    writeTo(sd, lines, depth + 1);
  }
}

/** The single-line representation of one {@link PropertyDiff}. */
function renderPropertyDiff(pd: PropertyDiff): string {
  switch (pd.op) {
    case OP_ADDED:
      return `+ ${renderProperty(pd.property)}`;
    case OP_REMOVED:
      return `- ${renderProperty(pd.property)}`;
    case OP_CHANGED:
      return `~ ${pd.property.name}: ${pd.old?.value ?? ""} -> ${pd.property.value}`;
  }
}

/**
 * A `NAME[;PARAM=VAL…]:VALUE` display line — no folding, no CRLF, since
 * this is for humans rather than the wire. Parameters sort by name so
 * the output is deterministic.
 */
function renderProperty(p: Property): string {
  const params: Param[] = [...p.params].sort((x, y) =>
    compareStrings(x.name.toUpperCase(), y.name.toUpperCase()),
  );
  let out = p.name;
  for (const pr of params) out += `;${pr.name}=${pr.value}`;
  return `${out}:${p.value}`;
}

/**
 * Compare two strings by UTF-16 code unit, the ordering Go's
 * `sort.Strings` produces over the same ASCII keys. `localeCompare` is
 * deliberately avoided: its order depends on the host's locale.
 */
function compareStrings(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

/** {@link compareStrings} over {@link keyOf}-encoded keys. */
function compareKeys(a: string, b: string): number {
  return compareStrings(a, b);
}

/** Byte-for-byte comparison of two canonical forms. */
function bytesEqual(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    if (a[i] !== b[i]) return false;
  }
  return true;
}
