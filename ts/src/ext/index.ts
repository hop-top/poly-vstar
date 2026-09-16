// SPDX-License-Identifier: MIT

/**
 * The `ext` subpath export: predicates and accessors for the V* `X-*`
 * extension namespace defined in spec/04 (Extension Discipline).
 *
 * V* extensions live in three tiers:
 *
 * - `X-VSTAR-*`    cross-system extensions on a stabilization track.
 * - `X-<SYSTEM>-*` one specific consuming system (e.g. `X-AGR-INTENT`).
 * - `X-EXP-*`      experimental / unstable; no guarantees.
 *
 * The promotion path runs `X-EXP-FOO` → `X-<SYSTEM>-FOO` → `X-VSTAR-FOO`:
 * an experimental property graduates when one system commits to it, and
 * promotes once two independent systems implement compatible semantics.
 */

import type { Component, Property } from "../types.js";

export { version } from "../version.js";

/**
 * The classification of an extension name under spec/04.
 *
 * `"none"` is the zero-ish value — the name is not an `X-*` extension
 * at all — and `"unknown"` is the catch-all for a name that carries the
 * `X-` prefix but matches no sanctioned tier.
 *
 * The wire tokens are lowercase, matching `spec/behavior/ext/scopes.json`,
 * so a port needs no case convention of its own. {@link scopeToString}
 * renders the reference's capitalized display spelling when a caller
 * wants it.
 */
export type Scope = "none" | "vstar" | "system" | "experimental" | "unknown";

/** `name` is not an `X-*` extension at all. */
export const SCOPE_NONE = "none" as const satisfies Scope;
/** `X-VSTAR-*` — a cross-system V* extension on the stabilization track. */
export const SCOPE_VSTAR = "vstar" as const satisfies Scope;
/** `X-<SYSTEM>-*` — an extension owned by one consuming system. */
export const SCOPE_SYSTEM = "system" as const satisfies Scope;
/** `X-EXP-*` — an unstable experimental extension. */
export const SCOPE_EXPERIMENTAL = "experimental" as const satisfies Scope;
/** Has the `X-` prefix but matches no sanctioned tier. */
export const SCOPE_UNKNOWN = "unknown" as const satisfies Scope;

/**
 * The reference's human-readable spelling of a {@link Scope} —
 * `"VStar"`, `"System"`, `"Experimental"`, `"None"`, `"Unknown"`.
 *
 * This is the `Scope.String()` of the Go reference, kept for error
 * messages and debug logging. It is display text, not the contract:
 * the fixtures and every programmatic comparison use the lowercase
 * {@link Scope} token itself.
 */
export function scopeToString(s: Scope): string {
  switch (s) {
    case SCOPE_NONE:
      return "None";
    case SCOPE_VSTAR:
      return "VStar";
    case SCOPE_SYSTEM:
      return "System";
    case SCOPE_EXPERIMENTAL:
      return "Experimental";
    case SCOPE_UNKNOWN:
      return "Unknown";
  }
}

/**
 * Report whether `name` carries the `X-` prefix per RFC 5545 §3.8.8 /
 * spec/04. Comparison is case-insensitive, so both `X-FOO` and `x-foo`
 * are extensions.
 *
 * The hyphen is required: `"X"` is a regular IANA-style identifier
 * while `"X-"` is an (ill-formed) extension. Pair this with
 * {@link scopeOf} to classify a name.
 */
export function isExtension(name: string): boolean {
  if (name.length < 2) return false;
  return (name[0] === "X" || name[0] === "x") && name[1] === "-";
}

/**
 * Classify `name` into one of the five {@link Scope} values per spec/04,
 * case-insensitively.
 *
 * The decision tree:
 *
 * - no `X-` prefix                    → `"none"`
 * - `X-VSTAR-<NAME>`, `NAME` non-empty → `"vstar"`
 * - `X-EXP-<NAME>`, `NAME` non-empty   → `"experimental"`
 * - `X-<SYSTEM>-<NAME>`, both non-empty and `SYSTEM` neither `VSTAR`
 *   nor `EXP`                          → `"system"`
 * - anything else with the prefix      → `"unknown"`
 *
 * The reservation on `VSTAR` and `EXP` covers the whole slug segment,
 * not a prefix of it: `X-VSTARLIKE-FOO` is an ordinary system extension
 * owned by `VSTARLIKE`.
 */
export function scopeOf(name: string): Scope {
  if (!isExtension(name)) return SCOPE_NONE;
  const rest = name.slice(2);
  if (rest === "") return SCOPE_UNKNOWN;
  const cut = cutSlug(rest);
  if (cut === undefined || cut[1] === "") return SCOPE_UNKNOWN;
  switch (cut[0].toUpperCase()) {
    case "VSTAR":
      return SCOPE_VSTAR;
    case "EXP":
      return SCOPE_EXPERIMENTAL;
    default:
      return SCOPE_SYSTEM;
  }
}

/**
 * The owning system's slug for an `X-<SYSTEM>-<NAME>` extension,
 * uppercased so callers compare without re-normalizing, or `undefined`
 * for any name that is not `"system"`-scoped.
 *
 * That exclusion is deliberate and covers non-extensions, `X-VSTAR-*`
 * (no owning system — the V* spec owns it), `X-EXP-*` (no owner at all)
 * and malformed names lacking the `<SYSTEM>-<NAME>` structure. The
 * question this answers is "which system owns this property?", which
 * only a system-scoped name has an answer to.
 */
export function systemName(name: string): string | undefined {
  if (!isExtension(name)) return undefined;
  const cut = cutSlug(name.slice(2));
  if (cut === undefined) return undefined;
  const [slug, suffix] = cut;
  if (slug === "" || suffix === "") return undefined;
  const upper = slug.toUpperCase();
  if (upper === "VSTAR" || upper === "EXP") return undefined;
  return upper;
}

/**
 * Every property on `c` whose name classifies into `scope`, in the
 * component's own property order — no sort.
 *
 * `scopeOf(name) === "none"` selects every non-extension property
 * (`UID`, `DTSTART`, …), which is the useful shape for diffing the V*
 * core surface. The function does not recurse into `c.sub`; a caller
 * wanting the whole tree walks the sub-components itself.
 */
export function extensionsByScope(c: Component, scope: Scope): Property[] {
  return c.props.filter((p) => scopeOf(p.name) === scope);
}

/**
 * Split `SLUG-REST` at the first hyphen. Returns `undefined` when the
 * input carries no hyphen at all.
 */
function cutSlug(s: string): [slug: string, rest: string] | undefined {
  const i = s.indexOf("-");
  if (i < 0) return undefined;
  return [s.slice(0, i), s.slice(i + 1)];
}
