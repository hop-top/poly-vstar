// SPDX-License-Identifier: MIT

/**
 * The vCard 4.0 (RFC 6350) wire format: line unfolding (§3.2 → RFC 5545
 * §3.1), content-line parsing including group prefixes (§3.3), TEXT
 * escaping (§3.4), `BEGIN:VCARD…END:VCARD` framing, and the symmetric
 * encoder with 75-octet folding and CRLF terminators.
 *
 * `VERSION:4.0` is the only version accepted at v0.1.
 */

import type { Card } from "../../types.js";

export { encode, formatProperty } from "./encoder.js";
export { parse, parseContentLine } from "./parser.js";
export { SUPPORTED_VERSION } from "./version.js";

export { version } from "../../version.js";

import { encode } from "./encoder.js";
import { parse } from "./parser.js";

/**
 * Reads zero or more vCards from a document.
 *
 * vCards have a different top-level shape from calendars, so this
 * codec defines its own interfaces rather than reusing the root ones:
 * `parse` returns a **list**.
 */
export interface Parser {
  parse(input: string | Uint8Array): Card[];
}

/** Writes a single vCard. */
export interface Encoder {
  encode(card: Card): Uint8Array;
}

/** {@link Parser} and {@link Encoder} composed. */
export interface Codec extends Parser, Encoder {}

/** A fresh, stateless {@link Codec}. */
export function newCodec(): Codec {
  return { parse, encode };
}

/** A fresh {@link Parser}, for callers needing only the read side. */
export function newParser(): Parser {
  return { parse };
}

/** A fresh {@link Encoder}, for callers needing only the write side. */
export function newEncoder(): Encoder {
  return { encode };
}

/** A shared, stateless {@link Codec}. */
export const DEFAULT: Codec = newCodec();
