// SPDX-License-Identifier: MIT

/**
 * The iCalendar (VCALENDAR) wire format per RFC 5545: a content-line
 * scanner with §3.1 line unfolding, a content-line parser (§3.2), a
 * recursive BEGIN/END block parser (§3.4–§3.6), and an encoder emitting
 * CRLF-terminated, 75-octet-folded output.
 */

import type { Codec } from "../../codec.js";
import { Scanner, newScanner } from "../contentline.js";

export { encode, encodeComponent, encodeContentLine } from "./encoder.js";
export { parse, parseContentLine } from "./parser.js";
export { Scanner, newScanner };

export { version } from "../../version.js";

import { encode } from "./encoder.js";
import { parse } from "./parser.js";

/**
 * A fresh {@link Codec} backed by the RFC 5545 parser and encoder. The
 * returned value is stateless — share it or reconstruct it freely.
 */
export function newCodec(): Codec {
  return { parse, encode };
}

/**
 * A shared, stateless {@link Codec} for callers that do not need to
 * construct their own. Equivalent to {@link newCodec}.
 */
export const DEFAULT: Codec = newCodec();
