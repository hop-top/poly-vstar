// SPDX-License-Identifier: MIT

// The `validate` value types: Severity, Diagnostic, and the shared
// path/lookup helpers every rule module builds on.

import { CODE_SEVERITIES, type Code, type Severity } from "../generated/codes.js";
import { uid as componentUid, type Component } from "../types.js";

export type { Code, Severity };

/**
 * A single validation finding.
 *
 * `code` is the stable catalog identifier cataloged in
 * [docs/validate-codes.md](../../../docs/validate-codes.md); it is
 * stable across minor versions per semver, so consumers may match on
 * it programmatically.
 *
 * `message` is human-readable detail and is **not** stable across
 * versions. Match `code` instead — the behavior fixtures deliberately
 * omit messages for exactly this reason.
 *
 * `path` is a dotted component/property locator (see the module doc).
 */
export interface Diagnostic {
  /** `"error"` for a MUST violation, `"warning"` for a SHOULD. */
  readonly severity: Severity;
  /** The stable catalog identifier. */
  readonly code: Code;
  /** Human-readable detail. Not stable; do not match on it. */
  readonly message: string;
  /** The dotted locator. */
  readonly path: string;
}

/**
 * Build a {@link Diagnostic}, taking its severity from the generated
 * registry rather than from the call site.
 *
 * Severity is registry data, not a per-rule decision: a rule that
 * spelled its own severity would be a second source of truth, free to
 * drift the moment `spec/registry/diagnostic-codes.json` changes. The
 * only way to change a severity here is to change the registry and
 * re-run the generator.
 */
export function diagnostic(code: Code, message: string, path: string): Diagnostic {
  return { severity: CODE_SEVERITIES[code], code, message, path };
}

/**
 * The property name V\* uses to carry the content hash.
 *
 * Deliberately a local constant rather than an import from `hashing`:
 * `validate` inspects raw property names without taking on the hashing
 * module's identity, and the two agreeing is asserted by the fixtures.
 */
export const X_VSTAR_HASH = "X-VSTAR-HASH";

/**
 * The component's wire type as it arrived.
 *
 * `CompType` is open and names no `VCARD` constant — a top-level vCard
 * is a `Card`, not a `Component`. A `VCARD` block nested inside a
 * `VCALENDAR` still reaches the parser, which carries the wire string
 * through verbatim, so the validator switches on the plain string and
 * can still apply the `VCARD` rule to it.
 */
export function wireType(c: Component): string {
  return c.type;
}

/** Running positional index per component type, for UID-less paths. */
export type PathIndex = Map<string, number>;

/**
 * The path segment identifying `c` relative to its container:
 * `<TYPE>[uid=<uid>]`, or `<TYPE>[#<n>]` when `c` has no UID.
 *
 * `index` carries the running positional counter per type so that two
 * UID-less components of the same type get distinct, stable segments.
 */
export function componentPath(c: Component, index: PathIndex): string {
  const uid = componentUid(c);
  if (uid !== "") return `${wireType(c)}[uid=${uid}]`;
  const type = wireType(c);
  const n = index.get(type) ?? 0;
  index.set(type, n + 1);
  return `${type}[#${n}]`;
}

/** Whether `c` carries a property named `name` (case-insensitive). */
export function has(c: Component, name: string): boolean {
  return c.props.some((p) => p.name.toUpperCase() === name.toUpperCase());
}

/** Case-insensitive string comparison, per RFC 5545 §3.1. */
export function equalFold(a: string, b: string): boolean {
  return a.toUpperCase() === b.toUpperCase();
}
