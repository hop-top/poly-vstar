// SPDX-License-Identifier: MIT

// spec/03 rule 12, spec/05 criterion 7 — DURATION well-formedness.

import { parse as parseDuration, parseTrigger } from "../duration/index.js";
import { CODES } from "../generated/codes.js";
import type { Component } from "../types.js";
import { isCanonicalDecimal } from "./integer.js";
import { diagnostic, equalFold, type Diagnostic } from "./internal.js";

const DURATION = "DURATION";
const TRIGGER = "TRIGGER";
const REPEAT = "REPEAT";

/**
 * One diagnostic per duration-bearing property whose value is
 * malformed.
 *
 * TRIGGER goes through `duration.parseTrigger` rather than a bare
 * duration parse so the two value forms and the RELATED / VALUE
 * parameter rules are enforced together — an absolute trigger is not a
 * duration, and rejecting it as one would be wrong. DURATION and
 * REPEAT are checked directly: each has exactly one legal shape.
 *
 * REPEAT's shape is the textual canonical-decimal rule the integer
 * domains share (spec/05 §8): a sign or a leading zero is malformed
 * even though the number is in range. It stays under the
 * malformed-duration code because a published code never changes
 * meaning.
 */
export function checkDuration(c: Component, path: string): Diagnostic[] {
  const out: Diagnostic[] = [];
  for (const p of c.props) {
    if (equalFold(p.name, TRIGGER)) {
      const err = failure(() => parseTrigger(p));
      if (err !== undefined) {
        out.push(
          diagnostic(
            CODES.CodeMalformedDuration,
            `TRIGGER is malformed (RFC 5545 §3.8.6.3): ${String(err)}`,
            `${path}.${TRIGGER}`,
          ),
        );
      }
    } else if (equalFold(p.name, DURATION)) {
      const err = failure(() => parseDuration(p.value));
      if (err !== undefined) {
        out.push(
          diagnostic(
            CODES.CodeMalformedDuration,
            `DURATION is malformed (RFC 5545 §3.3.6): ${String(err)}`,
            `${path}.${DURATION}`,
          ),
        );
      }
    } else if (equalFold(p.name, REPEAT) && !isCanonicalDecimal(p.value)) {
      out.push(
        diagnostic(
          CODES.CodeMalformedDuration,
          `REPEAT is not a canonical non-negative integer (RFC 5545 §3.8.6.2, spec/05 §8): ${p.value}`,
          `${path}.${REPEAT}`,
        ),
      );
    }
  }
  return out;
}

/** The error `fn` throws, or `undefined` when it returns. */
function failure(fn: () => unknown): unknown {
  try {
    fn();
    return undefined;
  } catch (e) {
    return e;
  }
}
