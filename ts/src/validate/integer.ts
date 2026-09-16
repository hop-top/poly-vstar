// SPDX-License-Identifier: MIT

// spec/05 §8 — the PRIORITY, PERCENT-COMPLETE and SEQUENCE value
// domains, and the canonical-decimal rule they share with REPEAT.

import { CODES } from "../generated/codes.js";
import type { Component } from "../types.js";
import { diagnostic, equalFold, type Diagnostic } from "./internal.js";

/**
 * One bounded integer property: its name, the RFC 5545 section that
 * bounds it, and its upper bound as a digit string (`undefined` when
 * unbounded). The lower bound is always 0, which the canonical-decimal
 * form already enforces — there is no sign to carry a negative.
 */
interface IntegerDomain {
  readonly name: string;
  readonly section: string;
  readonly max: string | undefined;
}

/** The table the integer-domain rule checks, in catalog order. */
const INTEGER_DOMAINS: readonly IntegerDomain[] = [
  { name: "PRIORITY", section: "§3.8.1.9", max: "9" },
  { name: "PERCENT-COMPLETE", section: "§3.8.1.8", max: "100" },
  { name: "SEQUENCE", section: "§3.8.7.4", max: undefined },
];

/**
 * Whether `s` is a canonical non-negative decimal per spec/05 §8:
 * digits only, no sign, no whitespace, and no leading zero unless `s`
 * is exactly `0`.
 *
 * The check is textual on purpose. It never converts `s` to a machine
 * number — `Number("+3")`, `Number("07")` and `Number(" 3")` all read
 * as in-range integers and would let every non-canonical form through,
 * and a `SEQUENCE` of 2^64 is well-formed even though no JavaScript
 * integer can hold it: the spec bounds `SEQUENCE` below, never above.
 */
export function isCanonicalDecimal(s: string): boolean {
  return /^(0|[1-9][0-9]*)$/.test(s);
}

/**
 * Whether the canonical decimal `v` is numerically greater than the
 * canonical decimal `max`. Both must already satisfy
 * {@link isCanonicalDecimal}: with no leading zeros, a longer string is
 * a larger number and equal lengths compare lexically.
 */
function exceedsDigitString(v: string, max: string): boolean {
  if (v.length !== max.length) return v.length > max.length;
  return v > max;
}

/**
 * One diagnostic per bounded integer property whose value is not a
 * canonical decimal inside its RFC 5545 domain.
 *
 * The value is checked wherever the property appears; the rule does not
 * gate on component type (PERCENT-COMPLETE on a VEVENT is bounded, not
 * flagged for scope). One diagnostic per offending property, at the
 * property's path, like the STATUS and DURATION rules.
 */
export function checkIntegerDomains(c: Component, path: string): Diagnostic[] {
  const out: Diagnostic[] = [];
  for (const p of c.props) {
    const dom = INTEGER_DOMAINS.find((d) => equalFold(p.name, d.name));
    if (dom === undefined) continue;
    if (!isCanonicalDecimal(p.value)) {
      out.push(
        diagnostic(
          CODES.CodeIntegerOutOfDomain,
          `${dom.name} is not a canonical non-negative decimal (RFC 5545 ${dom.section}, spec/05 §8): ${p.value}`,
          `${path}.${dom.name}`,
        ),
      );
    } else if (dom.max !== undefined && exceedsDigitString(p.value, dom.max)) {
      out.push(
        diagnostic(
          CODES.CodeIntegerOutOfDomain,
          `${dom.name} value ${p.value} is outside 0–${dom.max} (RFC 5545 ${dom.section})`,
          `${path}.${dom.name}`,
        ),
      );
    }
  }
  return out;
}
