// SPDX-License-Identifier: MIT

import { VstarError } from "../../errors.js";
import type { Card, Property } from "../../types.js";
import { concatBytes, encodeParamValue, escapeText, foldLine } from "../contentline.js";
import { SUPPORTED_VERSION } from "./version.js";

/**
 * Encode exactly **one** {@link Card} as a self-contained
 * `BEGIN:VCARD…END:VCARD` block and return the bytes.
 *
 * The asymmetry with {@link parse} is deliberate: parse returns a list,
 * encode takes a single card. Encoding a list means calling this once
 * per card and concatenating — which is what a stream encoder does.
 *
 * ## UID is required here, and only here
 *
 * An empty `card.uid` throws `ErrMissingUID`. The parser accepts a
 * UID-less VCARD, so this is the **only** place in the codec that
 * sentinel can fire, and the `malformed/missing_uid.vcf` fixture is
 * exercised by parsing the document successfully and then asking the
 * encoder to refuse the result. A port implementing only the parse side
 * passes that fixture silently and is wrong.
 *
 * Emission order is fixed so output is byte-stable: UID, then KIND when
 * set, then `card.props` in input order. Property names (the segment
 * after any `group.` prefix) are uppercased per RFC 6350 §3.3; group
 * prefixes keep their original case for round-trip fidelity.
 */
export function encode(card: Card): Uint8Array {
  if (card.uid === "") {
    throw new VstarError("ErrMissingUID", "encode: card has no UID");
  }

  const chunks: Uint8Array[] = [
    foldLine("BEGIN:VCARD"),
    foldLine(`VERSION:${SUPPORTED_VERSION}`),
    foldLine(`UID:${escapeText(card.uid)}`),
  ];
  if (card.kind !== "") {
    chunks.push(foldLine(`KIND:${card.kind.toLowerCase()}`));
  }
  for (const p of card.props) chunks.push(foldLine(formatProperty(p)));
  chunks.push(foldLine("END:VCARD"));
  return concatBytes(chunks);
}

/**
 * Serialize one property to its unfolded wire form. A group prefix
 * survives verbatim; the bare name is uppercased; parameter values are
 * DQUOTE-wrapped when they contain `,` `:` or `;`.
 */
export function formatProperty(p: Property): string {
  const [group, name] = splitGroup(p.name);
  let out = group === "" ? "" : `${group}.`;
  out += name.toUpperCase();
  for (const par of p.params) {
    out += `;${par.name.toUpperCase()}=${encodeParamValue(par.value)}`;
  }
  return `${out}:${escapeText(p.value)}`;
}

/**
 * Separate a `group.NAME` property identifier into its group and bare
 * name. With no `.`, the group is `""` and the name is the whole input.
 */
function splitGroup(s: string): [group: string, name: string] {
  const i = s.indexOf(".");
  return i >= 0 ? [s.slice(0, i), s.slice(i + 1)] : ["", s];
}
