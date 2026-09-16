// SPDX-License-Identifier: MIT

/**
 * SHA-256 content hashes of V* objects, in the `sha256:<hex>` form the
 * specification mandates.
 *
 * Hashes are computed over the canonical byte form, so two
 * implementations that agree on canonical bytes produce identical
 * hashes. That is the whole point: the hash is the cheap, transportable
 * proof that two documents are the same logical content, and it is only
 * as good as the byte agreement underneath it.
 *
 * The `sha256:` prefix is part of the value, not decoration. It exists
 * so a future `sha3-256:` or `blake3:` is expressible without ambiguity;
 * v1.0 emits only `sha256:`.
 *
 * ## Hash exclusion
 *
 * Rule 7 excludes `X-VSTAR-HASH` from the bytes its own value is
 * computed over — otherwise the stored hash would feed back into its
 * own digest. The canonical layer strips it per its own contract, and
 * these functions strip it again. The redundancy is deliberate: it
 * documents the invariant at the API boundary, so a reader of this
 * module does not have to go and confirm the canonical layer's
 * behaviour.
 *
 * Every function here is pure except {@link setXVstar}, which is the
 * one writer.
 */

import { sha256 } from "@noble/hashes/sha2";
import { bytesToHex } from "@noble/hashes/utils";

import {
  calendar as canonicalCalendar,
  card as canonicalCard,
  component as canonicalComponent,
} from "../canonical/index.js";
import type { Calendar, Card, Component, Property } from "../types.js";
import { get, set } from "../types.js";
import { X_VSTAR_HASH_PROPERTY } from "./names.js";

export { X_VSTAR_HASH_PROPERTY } from "./names.js";
export { version } from "../version.js";

const SHA256_PREFIX = "sha256:";

/**
 * The `sha256:<hex>` digest of the canonical byte form of `c`.
 *
 * This delegates to the context-free canonical form, not the
 * context-taking one. A component carrying TZID-tagged datetimes
 * therefore hashes over wire-form bytes: two timezone spellings of the
 * same logical instant hash differently. For calendar-aware hashing
 * that resolves TZIDs against a VTIMEZONE registry, hash the whole
 * calendar with {@link calendar}.
 *
 * The asymmetry mirrors the canonical layer's own, and it is correct: a
 * component without a parent calendar has no registry to consult.
 */
export function component(c: Component): string {
  return digest(canonicalComponent(stripComponent(c)));
}

/**
 * The `sha256:<hex>` digest of the canonical byte form of `cal`.
 *
 * `X-VSTAR-HASH` is stripped at every depth — top-level components and
 * their sub-components alike — before the canonical pass. TZID-tagged
 * datetimes resolve against the calendar's own VTIMEZONE registry, so
 * this is the entry point whose result is stable across producers that
 * spell the same instant differently.
 */
export function calendar(cal: Calendar): string {
  return digest(
    canonicalCalendar({
      prodId: cal.prodId,
      components: cal.components.map(stripComponent),
    }),
  );
}

/** The `sha256:<hex>` digest of the canonical byte form of `c`. */
export function card(c: Card): string {
  return digest(canonicalCard({ uid: c.uid, kind: c.kind, props: filterOutHash(c.props) }));
}

/**
 * Compute {@link component} and write the result to `c` as the
 * `X-VSTAR-HASH` property, replacing any existing value rather than
 * duplicating it.
 *
 * This mutates `c` — it is the one function here that does. The hash is
 * computed over the stripped bytes, so calling it repeatedly on the
 * same logical component is idempotent: the second call computes the
 * same hash and rewrites the same value.
 */
export function setXVstar(c: Component): void {
  set(c, { name: X_VSTAR_HASH_PROPERTY, params: [], value: component(c) });
}

/**
 * The stored `X-VSTAR-HASH` value, or `undefined` when the property is
 * absent.
 *
 * The stored value's format is not validated here. A caller wanting to
 * confirm both shape and freshness uses {@link verifyXVstar}.
 */
export function getXVstar(c: Component): string | undefined {
  return get(c, X_VSTAR_HASH_PROPERTY)?.value;
}

/** What {@link verifyXVstar} reports. */
export interface XVstarVerification {
  /** Whether a hash is stored AND equals the recomputed one exactly. */
  readonly ok: boolean;
  /** The recomputed, correct hash. Always populated. */
  readonly want: string;
  /** The stored hash; the empty string when none is stored. */
  readonly got: string;
}

/**
 * Recompute `c`'s hash and compare it against the stored
 * `X-VSTAR-HASH`.
 *
 * All three fields are reported regardless of the outcome, so a caller
 * can say *what* differed rather than only *that* something did — which
 * is the difference between a usable corruption report and a shrug.
 */
export function verifyXVstar(c: Component): XVstarVerification {
  const want = component(c);
  const got = getXVstar(c);
  if (got === undefined) return { ok: false, want, got: "" };
  return { ok: got === want, want, got };
}

/** `sha256:` plus the lowercase hex digest of `b`. */
function digest(b: Uint8Array): string {
  return SHA256_PREFIX + bytesToHex(sha256(b));
}

/**
 * A copy of `c` with `X-VSTAR-HASH` removed from its properties and
 * from every nested sub-component.
 *
 * Building a fresh component rather than editing in place is what keeps
 * the hash functions pure: a caller never observes the strip.
 */
function stripComponent(c: Component): Component {
  return {
    type: c.type,
    props: filterOutHash(c.props),
    sub: c.sub.map(stripComponent),
  };
}

/** A copy of `props` without any `X-VSTAR-HASH` property. */
function filterOutHash(props: readonly Property[]): Property[] {
  return props.filter((p) => p.name.toUpperCase() !== X_VSTAR_HASH_PROPERTY);
}
