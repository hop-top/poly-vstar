// SPDX-License-Identifier: MIT

import type { Calendar } from "./types.js";

/**
 * Reads a single {@link Calendar} from a document.
 *
 * Implementations are stateless and safe to share. A failure throws a
 * `VstarError` carrying one of the sentinel identifiers — dispatch on
 * its `code`.
 */
export interface Parser {
  parse(input: string | Uint8Array): Calendar;
}

/**
 * Writes a {@link Calendar} in the codec's wire format.
 *
 * The Go reference writes to an `io.Writer` and returns an error;
 * TypeScript has no idiomatic writer in that position, so the bytes are
 * returned instead. That byte-returning form is the one the conformance
 * gates exercise.
 */
export interface Encoder {
  encode(cal: Calendar): Uint8Array;
}

/** {@link Parser} and {@link Encoder} composed. */
export interface Codec extends Parser, Encoder {}
