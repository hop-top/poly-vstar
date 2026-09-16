// SPDX-License-Identifier: MIT

import type { Calendar, Component, Property } from "../../types.js";
import { concatBytes, encodeParamValue, escapeText, foldLine } from "../contentline.js";
import { SUPPORTED_VERSION, isTextProperty } from "./text-properties.js";

/**
 * Encode `cal` in RFC 5545 wire format and return the bytes.
 *
 * Output is always CRLF-terminated; physical lines are folded at 75
 * octets per §3.1 using SP as the continuation lead octet. Property and
 * component order are preserved from `cal` verbatim.
 *
 * The outer VCALENDAR wrapper is always emitted with `VERSION:2.0` and
 * a PRODID derived from `cal.prodId`; any VERSION/PRODID inside
 * `cal.components` is ignored at this layer. PRODID goes through
 * {@link encodeContentLine} so RFC 5545 §3.3.11 TEXT escaping applies —
 * PRODID is a TEXT-typed property.
 */
export function encode(cal: Calendar): Uint8Array {
  const chunks: Uint8Array[] = [
    foldLine("BEGIN:VCALENDAR"),
    foldLine(`VERSION:${SUPPORTED_VERSION}`),
    foldLine(encodeContentLine({ name: "PRODID", params: [], value: cal.prodId })),
  ];
  for (const c of cal.components) chunks.push(...componentChunks(c));
  chunks.push(foldLine("END:VCALENDAR"));
  return concatBytes(chunks);
}

/**
 * Encode a single component — its BEGIN/END wrapper, properties and
 * recursive sub-components — with no VCALENDAR wrapper and no
 * VERSION/PRODID injected.
 *
 * This is the building block canonicalization shares with the calendar
 * encoder, so fold and CRLF logic stay in one place.
 */
export function encodeComponent(c: Component): Uint8Array {
  return concatBytes(componentChunks(c));
}

/** The folded byte chunks of one component, depth-first. */
function componentChunks(c: Component): Uint8Array[] {
  const tname = c.type.toUpperCase();
  const chunks: Uint8Array[] = [foldLine(`BEGIN:${tname}`)];
  for (const p of c.props) chunks.push(foldLine(encodeContentLine(p)));
  for (const s of c.sub) chunks.push(...componentChunks(s));
  chunks.push(foldLine(`END:${tname}`));
  return chunks;
}

/**
 * Render a property as a single **unfolded** wire content line:
 * `NAME[;PARAM=val…]:value`.
 *
 * Folding happens afterwards, on the assembled line — see
 * {@link foldLine} for why the order matters.
 *
 * Parameter values containing `,` `;` `:` or `"` are DQUOTE-wrapped per
 * §3.2. TEXT-typed values (per the allow-list) are escaped per
 * §3.3.11; every other value type emits verbatim.
 */
export function encodeContentLine(p: Property): string {
  let out = p.name.toUpperCase();
  for (const par of p.params) {
    out += `;${par.name.toUpperCase()}=${encodeParamValue(par.value)}`;
  }
  out += ":";
  out += isTextProperty(p.name) ? escapeText(p.value) : p.value;
  return out;
}
