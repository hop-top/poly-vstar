// SPDX-License-Identifier: MIT

// spec/05 §3, spec/04 — extension namespace compliance.

import { CODES, STANDARD_PROPERTIES } from "../generated/codes.js";
import { isExtension } from "../ext/index.js";
import type { Component } from "../types.js";
import { diagnostic, type Diagnostic } from "./internal.js";

/**
 * The RFC 5545 §3.7-§3.8 / RFC 6350 §6 allow-list, uppercased for
 * case-insensitive lookup. The list itself is generated from
 * `spec/registry/standard-properties.json` — this is only the index.
 */
const STANDARD = new Set(STANDARD_PROPERTIES.map((name) => name.toUpperCase()));

/**
 * One warning per property whose name is neither on the RFC
 * allow-list nor prefixed `X-`.
 *
 * The `X-` classification is delegated to `ext.isExtension` so there
 * is one answer to "what counts as an extension"; the allow-list stays
 * here because it answers the orthogonal question "is this a known RFC
 * property?", which `ext` has no business knowing.
 */
export function checkExtensionNamespace(c: Component, path: string): Diagnostic[] {
  const out: Diagnostic[] = [];
  for (const p of c.props) {
    if (STANDARD.has(p.name.toUpperCase())) continue;
    if (isExtension(p.name)) continue;
    out.push(
      diagnostic(
        CODES.CodeUnknownProperty,
        `property ${p.name} is not a known RFC 5545/6350 property and does not use the X- extension prefix (spec/05 §3, spec/04)`,
        `${path}.${p.name}`,
      ),
    );
  }
  return out;
}

/** How many properties the generated RFC allow-list carries. */
export function standardPropertyCount(): number {
  return STANDARD_PROPERTIES.length;
}
