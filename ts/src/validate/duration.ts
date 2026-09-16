// SPDX-License-Identifier: MIT

// spec/03 rule 12, spec/05 criterion 7 — DURATION well-formedness.

import { parse as parseDuration, parseTrigger } from "../duration/index.js";
import { CODES } from "../generated/codes.js";
import type { Component } from "../types.js";
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
    } else if (equalFold(p.name, REPEAT) && !isNonNegativeInteger(p.value)) {
      out.push(
        diagnostic(
          CODES.CodeMalformedDuration,
          `REPEAT is not a non-negative integer (RFC 5545 §3.8.6.2): ${p.value}`,
          `${path}.${REPEAT}`,
        ),
      );
    }
  }
  return out;
}

/**
 * Whether `value` is a non-negative decimal integer.
 *
 * Deliberately a pattern rather than `Number.parseInt`, which happily
 * reads `"3abc"` as `3` and would let a malformed REPEAT through.
 */
function isNonNegativeInteger(value: string): boolean {
  return /^\d+$/.test(value);
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
