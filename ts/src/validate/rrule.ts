// SPDX-License-Identifier: MIT

// spec/03 §RRULE parsing scope — RRULE conformance.

import { VstarError } from "../errors.js";
import { CODES } from "../generated/codes.js";
import { validateRRule } from "../rrule/index.js";
import type { Component } from "../types.js";
import { diagnostic, equalFold, type Diagnostic } from "./internal.js";

/** RFC 5545 §3.8.5.3. */
const RRULE = "RRULE";

/**
 * One diagnostic per RRULE property whose value fails validation.
 *
 * The split is by failure class, not by guesswork: an unsupported
 * feature is a warning because the property still round-trips through
 * the codec — only its recurrence semantics are out of reach — while a
 * malformed value is an error because no consumer, V\* or otherwise,
 * can evaluate it.
 *
 * A failure that classifies as neither is reported as malformed rather
 * than swallowed: a finding the consumer can see beats silence.
 */
export function checkRRule(c: Component, path: string): Diagnostic[] {
  const out: Diagnostic[] = [];
  for (const p of c.props) {
    if (!equalFold(p.name, RRULE)) continue;
    const err = validationError(p.value);
    if (err === undefined) continue;
    const code =
      err instanceof VstarError && err.code === "ErrUnsupportedRRule"
        ? CODES.CodeRRuleUnsupported
        : CODES.CodeRRuleMalformed;
    const detail =
      code === CODES.CodeRRuleUnsupported
        ? `RRULE uses a feature outside the RRULE parsing scope (spec/03 §RRULE parsing scope): ${String(err)}`
        : `RRULE is malformed (RFC 5545 §3.3.10): ${String(err)}`;
    out.push(diagnostic(code, detail, `${path}.${RRULE}`));
  }
  return out;
}

/** The failure `validateRRule` raises for `value`, or `undefined` when clean. */
function validationError(value: string): unknown {
  try {
    validateRRule(value);
    return undefined;
  } catch (e) {
    return e;
  }
}
