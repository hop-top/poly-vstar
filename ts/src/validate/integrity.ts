// SPDX-License-Identifier: MIT

// spec/05 §2 — X-VSTAR-HASH integrity.

import { CODES } from "../generated/codes.js";
import { verifyXVstar } from "../hashing/index.js";
import type { Component } from "../types.js";
import { diagnostic, has, X_VSTAR_HASH, type Diagnostic } from "./internal.js";

/**
 * Recompute `c`'s content hash and flag a stored X-VSTAR-HASH that is
 * present but wrong.
 *
 * An **absent** hash is intentionally not flagged here — that case
 * belongs to the required-common-property rule. The split keeps the
 * diagnostic surface unambiguous: present-but-wrong is a different bug
 * than absent, and a consumer that sees both codes at once is looking
 * at a genuinely different document than one that sees either alone.
 */
export function checkHashIntegrity(c: Component, path: string): Diagnostic[] {
  if (!has(c, X_VSTAR_HASH)) return [];
  const { ok, want, got } = verifyXVstar(c);
  if (ok) return [];
  return [
    diagnostic(
      CODES.CodeBadXVSTARHash,
      `${X_VSTAR_HASH} does not match recomputed canonical hash; want=${want} got=${got} (spec/05 §2)`,
      `${path}.${X_VSTAR_HASH}`,
    ),
  ];
}
