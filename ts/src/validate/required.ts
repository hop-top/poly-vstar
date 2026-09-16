// SPDX-License-Identifier: MIT

// spec/05 §1 — required common properties.

import { CODES } from "../generated/codes.js";
import type { Component } from "../types.js";
import { diagnostic, has, X_VSTAR_HASH, type Diagnostic } from "./internal.js";

/**
 * One diagnostic per missing required common property.
 *
 * UID, DTSTAMP and X-VSTAR-HASH are checked in that order so the
 * diagnostic stream is deterministic. Each path appends the property
 * name to the component locator.
 */
export function checkRequiredCommon(c: Component, path: string): Diagnostic[] {
  const out: Diagnostic[] = [];
  if (!has(c, "UID")) {
    out.push(diagnostic(CODES.CodeMissingUID, "required common property UID is missing (spec/02)", `${path}.UID`));
  }
  if (!has(c, "DTSTAMP")) {
    out.push(
      diagnostic(
        CODES.CodeMissingDTSTAMP,
        "required common property DTSTAMP is missing (spec/02)",
        `${path}.DTSTAMP`,
      ),
    );
  }
  if (!has(c, X_VSTAR_HASH)) {
    out.push(
      diagnostic(
        CODES.CodeMissingXVSTARHash,
        `required common property ${X_VSTAR_HASH} is missing (spec/02)`,
        `${path}.${X_VSTAR_HASH}`,
      ),
    );
  }
  return out;
}
