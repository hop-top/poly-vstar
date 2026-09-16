// SPDX-License-Identifier: MIT

/**
 * The deterministic canonical byte form of V* objects, per spec rules
 * 1–12.
 *
 * Two V* documents containing the same logical content MUST produce
 * identical canonical bytes. That is the invariant the whole
 * specification exists to hold, and it is what makes an `X-VSTAR-HASH`
 * comparable across implementations.
 *
 * Every function here returns raw bytes — a `Uint8Array`, not a string.
 * The distinction is load-bearing: the canonical form is a byte
 * sequence, its final CRLF is part of the value, and comparing it as
 * re-decoded text is how a port passes its own tests while emitting the
 * wrong bytes.
 *
 * The transforms are applied to one component in this order, which is
 * the order the spec fixes:
 *
 * 1. **Select** — drop `X-VSTAR-HASH` (rule 7); reduce `ATTACH` to URI
 *    form by stripping `VALUE=BINARY` and `ENCODING=BASE64` (rule 10).
 * 2. **Resolve datetimes** on the rule-5 allow-list against the
 *    calendar's VTIMEZONE registry, re-emitting as UTC form #2 and
 *    dropping the TZID where resolution succeeds. A `VALUE=DATE`
 *    property is governed by rule 11 and is never resolved; `RRULE`
 *    (rule 8) and `DURATION` (rule 12) pass through verbatim.
 * 3. **Normalize to NFC** — each property value and each parameter
 *    value, individually. Not names (rule 9).
 * 4. **Sort** — properties by name, each property's parameters by name
 *    (rule 2); top-level components by UID, or TZID for a VTIMEZONE,
 *    byte-wise on UTF-8, stable, key-less last (rule 6). Sub-components
 *    keep input order.
 * 5. **Assemble** each content line with escaping and parameter quoting
 *    (rule 4).
 * 6. **Fold** each assembled line at 75 octets (rule 3).
 * 7. **Terminate** every physical line with CRLF (rule 1).
 *
 * Steps 5–7 belong to the RFC 5545 encoder, which owns folding, CRLF
 * and TEXT escaping so there is exactly one implementation of each.
 *
 * NFC therefore sits **after** datetime resolution and **before**
 * sorting and folding. Before folding matters: normalization changes a
 * string's UTF-8 length — `e` + U+0301 is three octets, `é` is two — so
 * folding a pre-normalization string puts the break at the wrong octet.
 *
 * Nothing here mutates its input.
 */

import { encode as encodeIcs, encodeComponent } from "../codec/rfc5545/index.js";
import { X_VSTAR_HASH_PROPERTY } from "../hashing/names.js";
import { parseTimeWithTzid, formatTime } from "../time.js";
import type { Calendar, Card, Component, Param, Property } from "../types.js";

export { version } from "../version.js";

/** The empty calendar the context-free entry points resolve against. */
const NO_CONTEXT: Calendar = { prodId: "", components: [] };

/**
 * The canonical byte form of a single component, emitting datetimes
 * **verbatim**.
 *
 * This form is for components carrying no TZID-tagged datetimes. The
 * VTIMEZONE registry lives on the Calendar, not on the Component, so
 * this entry point cannot resolve a TZID reference and emits the value
 * with its TZID parameter retained — output is non-canonical for such a
 * component. Use {@link componentInContext} to thread the parent
 * calendar.
 *
 * The asymmetry is real, not an overload: a component without a parent
 * calendar genuinely has no registry to consult.
 */
export function component(c: Component): Uint8Array {
  return encodeComponent(prepareComponent(c, NO_CONTEXT));
}

/**
 * The canonical byte form of a single component, resolving TZID-tagged
 * datetimes against `cal`'s VTIMEZONE registry.
 *
 * For each property on the rule-5 allow-list carrying a TZID: on
 * successful resolution the value is re-emitted as UTC form #2 and the
 * TZID parameter is dropped. On failure — no matching VTIMEZONE, or one
 * outside the spec's VTIMEZONE subset — the value AND the TZID pass through
 * verbatim. Canonical bytes are not deterministic across calendars
 * carrying different VTIMEZONE definitions in that branch; a producer
 * is expected to ship coverage inside the subset.
 *
 * A value already in UTC form #2, and one with no TZID at all, pass
 * through unchanged — there is nothing to resolve.
 *
 * STANDARD and DAYLIGHT children inside a VTIMEZONE carry a wall-clock
 * DTSTART that defines the transition rule itself. Those are
 * deliberately not TZID-tagged and pass through by design.
 */
export function componentInContext(c: Component, cal: Calendar): Uint8Array {
  return encodeComponent(prepareComponent(c, cal));
}

/**
 * The canonical byte form of a full VCALENDAR.
 *
 * Top-level components are sorted per rule 6 and each is prepared
 * against the calendar's own VTIMEZONE registry, so a TZID-bearing
 * datetime in any child resolves against a VTIMEZONE in the same
 * document.
 *
 * The PRODID value is NFC-normalized here; the encoder owns its TEXT
 * escaping, so no pre-escaping happens at this layer.
 */
export function calendar(c: Calendar): Uint8Array {
  return encodeIcs({
    prodId: nfc(c.prodId),
    components: sortedComponents(c.components).map((sub) => prepareComponent(sub, c)),
  });
}

/**
 * The canonical byte form of a single VCARD:
 *
 * ```
 * BEGIN:VCARD
 * VERSION:4.0
 * <properties sorted by name; UID is one of them>
 * END:VCARD
 * ```
 *
 * VERSION is promoted ahead of alphabetical order, because RFC 6350
 * §3.3 requires it immediately after `BEGIN:VCARD`. `card.uid` and
 * `card.kind` are emitted as properties, or absorbed when `card.props`
 * already carries one of the same name.
 *
 * A vCard has no datetime or TZID concerns, so there is no context form.
 */
export function card(c: Card): Uint8Array {
  const props: Property[] = [{ name: "VERSION", params: [], value: "4.0" }];
  if (c.uid !== "" && !hasProp(c.props, "UID")) {
    props.push({ name: "UID", params: [], value: c.uid });
  }
  if (c.kind !== "" && !hasProp(c.props, "KIND")) {
    props.push({ name: "KIND", params: [], value: c.kind });
  }
  props.push(...c.props);

  // A wire-string component type, so the encoder emits BEGIN:VCARD and
  // END:VCARD. The vCard shares the iCalendar content-line grammar, so
  // it shares the encoder rather than duplicating fold and escape logic.
  const prepared = prepareComponent({ type: "VCARD" as Component["type"], props, sub: [] }, NO_CONTEXT);

  const version = prepared.props.find((p) => p.name.toUpperCase() === "VERSION");
  if (version !== undefined) {
    prepared.props = [version, ...prepared.props.filter((p) => p !== version)];
  }
  return encodeComponent(prepared);
}

/**
 * A copy of `c` with every canonicalization transform applied except
 * assembly, folding and CRLF, which the encoder owns.
 *
 * Sub-components are prepared recursively and are NOT sorted: they have
 * no natural sort key, so rule 6 preserves their input order.
 */
function prepareComponent(c: Component, cal: Calendar): Component {
  const props = c.props
    .filter((p) => p.name.toUpperCase() !== X_VSTAR_HASH_PROPERTY)
    .map((p) => prepareProperty(p, cal));

  return {
    type: c.type,
    props: stableSortByUpperName(props),
    sub: c.sub.map((s) => prepareComponent(s, cal)),
  };
}

/**
 * A copy of `p` with the value and parameter transforms applied.
 *
 * TEXT escaping is deliberately absent: the RFC 5545 encoder owns the
 * single authoritative escape pass on emit, using its own allow-list of
 * TEXT-typed property names. Escaping here would double it.
 */
function prepareProperty(p: Property, cal: Calendar): Property {
  let value = nfc(p.value);

  // Rule 11: a DATE value has no time to convert and no zone to
  // resolve, so it is emitted verbatim and the resolution registry is
  // never consulted. VALUE=DATE is RETAINED — unlike a resolved TZID it
  // is load-bearing, since the default value type for these properties
  // is DATE-TIME and an untagged eight-octet value is a malformed
  // DATE-TIME, not a DATE.
  const dateOnly = isDatetimeProperty(p.name) && isValueDate(p.params);

  // Rule 5: a datetime property carrying a TZID resolves to UTC form #2
  // where the calendar's registry allows, and the TZID is then dropped.
  let stripTzid = dateOnly;
  if (!dateOnly && isDatetimeProperty(p.name)) {
    const tzid = paramValue(p.params, "TZID");
    if (tzid !== undefined && tzid !== "") {
      const at = parseTimeWithTzid(p.value, tzid, cal);
      if (at !== undefined) {
        value = formatTime(at);
        stripTzid = true;
      }
    }
  }

  const isAttach = p.name.toUpperCase() === "ATTACH";
  const params: Param[] = [];
  for (const prm of p.params) {
    const name = prm.name.toUpperCase();
    // Rule 10: ATTACH is reference-only on emit. The value itself is
    // untouched — canonical form is best-effort for a malformed URI.
    if (isAttach && name === "VALUE" && prm.value.toUpperCase() === "BINARY") continue;
    if (isAttach && name === "ENCODING" && prm.value.toUpperCase() === "BASE64") continue;
    // A TZID on a DATE is a producer bug (RFC 5545 §3.2.19 scopes TZID
    // to DATE-TIME and TIME) and must not leak into the canonical bytes.
    if (stripTzid && name === "TZID") continue;

    let pv = nfc(prm.value);
    // Rule 11 upper-cases the VALUE argument so `VALUE=date` and
    // `VALUE=DATE` converge. General case-folding of other VALUE tokens
    // is deferred to a later version, so this is scoped to the DATE branch.
    if (dateOnly && name === "VALUE") pv = pv.toUpperCase();
    params.push({ name: prm.name, value: pv });
  }

  return { name: p.name, params: stableSortByUpperName(params), value };
}

/**
 * A stable copy of `items` sorted by uppercased name.
 *
 * The comparison is on the uppercased ASCII name, so the UTF-8 versus
 * UTF-16 distinction that governs the component sort does not arise:
 * property and parameter names are ASCII by RFC 5545 §3.1 / RFC 6350
 * §3.3.
 */
function stableSortByUpperName<T extends { name: string }>(items: readonly T[]): T[] {
  return items
    .map((item, i) => ({ item, i }))
    .sort((a, b) => {
      const x = a.item.name.toUpperCase();
      const y = b.item.name.toUpperCase();
      if (x < y) return -1;
      if (x > y) return 1;
      return a.i - b.i;
    })
    .map((e) => e.item);
}

/**
 * A copy of `components` sorted per rule 6: by UID, or by TZID for a
 * VTIMEZONE; components with neither key sort last; the sort is stable,
 * so equal keys — a producer bug — keep their relative input order.
 */
function sortedComponents(components: readonly Component[]): Component[] {
  return components
    .map((c, i) => ({ c, i, key: componentSortKey(c) }))
    .sort((a, b) => {
      if (a.key !== undefined && b.key !== undefined) {
        const cmp = compareUtf8(a.key, b.key);
        return cmp !== 0 ? cmp : a.i - b.i;
      }
      if (a.key !== undefined) return -1;
      if (b.key !== undefined) return 1;
      return a.i - b.i;
    })
    .map((e) => e.c);
}

/** The rule-6 sort key: TZID for a VTIMEZONE, otherwise UID then TZID. */
function componentSortKey(c: Component): string | undefined {
  if (c.type === "VTIMEZONE") {
    const tzid = propValue(c, "TZID");
    if (tzid !== undefined) return tzid;
  }
  const uid = propValue(c, "UID");
  if (uid !== undefined && uid !== "") return uid;
  return propValue(c, "TZID");
}

/**
 * Byte-wise comparison of two strings as UTF-8, which is what rule 6
 * specifies and what the Go reference's `<` on a string does.
 *
 * JavaScript's `<` compares UTF-16 code units, and the two orders
 * disagree: a surrogate pair (U+10000 and above, `F0`–`F4` in UTF-8)
 * sorts at `0xD800`–`0xDBFF` in UTF-16, i.e. BELOW every BMP character
 * from U+E000 up, whose UTF-8 lead byte is `EE` or `EF`. A port
 * comparing code units puts astral UIDs on the wrong side of those, and
 * the divergence only ever shows up on a real-world document.
 */
function compareUtf8(a: string, b: string): number {
  if (a === b) return 0;
  const encoder = new TextEncoder();
  const x = encoder.encode(a);
  const y = encoder.encode(b);
  const n = Math.min(x.length, y.length);
  for (let i = 0; i < n; i++) {
    const xi = x[i] as number;
    const yi = y[i] as number;
    if (xi !== yi) return xi < yi ? -1 : 1;
  }
  return x.length === y.length ? 0 : x.length < y.length ? -1 : 1;
}

/** The value of the first property named `name`, case-insensitively. */
function propValue(c: Component, name: string): string | undefined {
  return c.props.find((p) => p.name.toUpperCase() === name)?.value;
}

/** The value of the first parameter named `name`, case-insensitively. */
function paramValue(params: readonly Param[], name: string): string | undefined {
  return params.find((prm) => prm.name.toUpperCase() === name)?.value;
}

/** Whether `props` already carries a property named `name`. */
function hasProp(props: readonly Property[], name: string): boolean {
  return props.some((p) => p.name.toUpperCase() === name.toUpperCase());
}

/**
 * The rule-5 allow-list: property names whose values are RFC 5545
 * §3.3.5 DATE-TIME and may carry a TZID the canonical form resolves.
 *
 * DTSTAMP is here even though RFC 5545 §3.8.7.2 requires it to be UTC:
 * resolving defensively catches a non-conforming producer rather than
 * emitting a TZID-tagged DTSTAMP unchanged.
 */
const DATETIME_PROPERTIES: ReadonlySet<string> = new Set([
  "DTSTAMP",
  "DTSTART",
  "DTEND",
  "DUE",
  "COMPLETED",
  "RECURRENCE-ID",
  "CREATED",
  "LAST-MODIFIED",
]);

/** Whether `name` is on the rule-5 allow-list. */
function isDatetimeProperty(name: string): boolean {
  return DATETIME_PROPERTIES.has(name.toUpperCase());
}

/**
 * Whether `params` declares `VALUE=DATE`. Both the parameter name and
 * its registered-token argument compare case-insensitively per RFC 5545
 * §3.2 / §3.2.20.
 */
function isValueDate(params: readonly Param[]): boolean {
  return paramValue(params, "VALUE")?.toUpperCase() === "DATE";
}

/**
 * The NFC form of `s`, per rule 9.
 *
 * The fast path matters: `String#normalize` allocates unconditionally,
 * and the overwhelming majority of values are already normalized. A
 * cheap equality check skips the allocation, which the Go reference
 * does for the same reason.
 */
function nfc(s: string): string {
  const normalized = s.normalize("NFC");
  return normalized === s ? s : normalized;
}
