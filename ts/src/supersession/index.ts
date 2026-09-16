// SPDX-License-Identifier: MIT

/**
 * The `supersession` subpath export: V*'s append-only state-change
 * discipline from spec/02 §"Status supersession (append-only ledger)".
 *
 * V* ledgers are append-only — an original component MUST NOT be
 * mutated after it is appended. A status change is instead a fresh
 * `VJOURNAL` referencing the target via `RELATED-TO` and carrying
 * `CATEGORIES:status-supersession`, `X-VSTAR-EFFECTIVE-STATUS:<status>`
 * and its own `X-VSTAR-HASH`.
 *
 * Two primitives express that: {@link supersedes} writes such an entry,
 * and {@link superseded} projects a ledger onto one component. V* itself
 * defines only the encoding; full state-from-log projection is the
 * consumer's job per spec/02.
 */

import { VstarError } from "../errors.js";
import { getXVstar, setXVstar, verifyXVstar } from "../hashing/index.js";
import type { Component } from "../types.js";
import { dtstampRaw, get, getAll, set as setProp, uid as componentUid } from "../types.js";
import type { Instant } from "../date.js";
import { formatTime, parseTime } from "../time.js";

export { version } from "../version.js";

/**
 * The `CATEGORIES` wire string marking a `VJOURNAL` as a supersession
 * entry per spec/02. A consumer projecting a ledger MUST match this
 * exact value, case-insensitively, to identify state-change journals.
 */
export const CATEGORY_STATUS_SUPERSESSION = "status-supersession";

/**
 * The property a supersession `VJOURNAL` carries to state the new
 * effective status of the component its `RELATED-TO` names.
 *
 * The constant lives here, not at the package root, and stays here in
 * every port: it is meaningful only inside the supersession pattern,
 * and a root-level export would invite callers to write it onto
 * components directly — precisely the mutation the append-only
 * discipline forbids.
 *
 * The value itself is opaque to V*. The spec's examples use VTODO
 * statuses, but an application MAY use whatever status vocabulary its
 * domain demands.
 */
export const PROP_EFFECTIVE_STATUS = "X-VSTAR-EFFECTIVE-STATUS";

/**
 * The literal prefix every supersession UID carries. A consumer MAY
 * filter on it to enumerate supersession journals without parsing
 * `CATEGORIES`, but matching {@link CATEGORY_STATUS_SUPERSESSION} is
 * the spec-blessed path.
 */
const UID_PREFIX = "journal:status:";

const PROP_UID = "UID";
const PROP_DTSTAMP = "DTSTAMP";
const PROP_RELATED_TO = "RELATED-TO";
const PROP_CATEGORIES = "CATEGORIES";

/**
 * Construct a fresh supersession `VJOURNAL` superseding `target` with
 * `status`, stamped at `at`. The returned component carries a freshly
 * computed `X-VSTAR-HASH`; `target` is **not** mutated.
 *
 * Properties, in order: `UID` (`journal:status:<target uid>:<stamp>`),
 * `DTSTAMP`, `RELATED-TO`, `CATEGORIES`,
 * {@link PROP_EFFECTIVE_STATUS}, then `X-VSTAR-HASH` last.
 *
 * ## The integrity check is the point
 *
 * When `target` carries an `X-VSTAR-HASH`, this recomputes the target's
 * canonical hash and compares. A mismatch throws `ErrTargetCorrupted`
 * and refuses to construct anything: writing a supersession record
 * against a component mutated since it was hashed would silently attach
 * the new status to different content. A target with no stored hash
 * makes no integrity claim, so there is nothing to verify and the
 * construction proceeds.
 *
 * This is the asymmetric half of the pair — {@link superseded} returns
 * an optional and never throws, because "has anything superseded this?"
 * has two honest answers and "no" is not a failure.
 *
 * @throws {VstarError} with code `ErrTargetCorrupted`.
 */
export function supersedes(target: Component, status: string, at: Instant): Component {
  if (getXVstar(target) !== undefined) {
    const { ok, want, got } = verifyXVstar(target);
    if (!ok) {
      throw new VstarError(
        "ErrTargetCorrupted",
        `supersedes: target ${JSON.stringify(componentUid(target))} hash ${got} does not match canonical form ${want}`,
      );
    }
  }

  const targetUid = componentUid(target);
  const c: Component = { type: "VJOURNAL", props: [], sub: [] };
  setProp(c, { name: PROP_UID, params: [], value: `${UID_PREFIX}${targetUid}:${formatTime(at)}` });
  setProp(c, { name: PROP_DTSTAMP, params: [], value: formatTime(at) });
  setProp(c, { name: PROP_RELATED_TO, params: [], value: targetUid });
  setProp(c, { name: PROP_CATEGORIES, params: [], value: CATEGORY_STATUS_SUPERSESSION });
  setProp(c, { name: PROP_EFFECTIVE_STATUS, params: [], value: status });

  // The hash refresh MUST be the last mutation: it strips any existing
  // X-VSTAR-HASH before computing, so the stored value covers every
  // property written above.
  setXVstar(c);
  return c;
}

/**
 * The effective status `ledger` projects onto `c`, or `undefined` when
 * nothing supersedes it.
 *
 * A ledger entry counts when it is a `VJOURNAL` whose `RELATED-TO`
 * matches `c`'s UID, whose `CATEGORIES` contains
 * {@link CATEGORY_STATUS_SUPERSESSION} case-insensitively, and which
 * carries {@link PROP_EFFECTIVE_STATUS}. Among the matches the latest
 * `DTSTAMP` wins; a tie goes to the entry later in the ledger.
 *
 * This is a projection query, not a validator. An entry whose `DTSTAMP`
 * is missing or unparseable sorts to the epoch and so is effectively
 * skipped under "latest wins" rather than raising — a corrupt hash on
 * the target is likewise not this function's business, which is why the
 * `corrupt_mutated` fixture projects an empty map.
 */
export function superseded(c: Component, ledger: readonly Component[]): string | undefined {
  const targetUid = componentUid(c);
  if (targetUid === "") return undefined;

  let bestT = 0;
  let bestStatus: string | undefined;

  for (const entry of ledger) {
    if (entry.type !== "VJOURNAL") continue;
    const rel = get(entry, PROP_RELATED_TO);
    if (rel === undefined || rel.value !== targetUid) continue;
    if (!categoriesContainSupersession(entry)) continue;
    const statusProp = get(entry, PROP_EFFECTIVE_STATUS);
    if (statusProp === undefined) continue;

    const entryT = parseEntryDtstamp(entry);
    // Stable "latest wins": >= takes the later ledger position when
    // two entries share a timestamp, since the scan runs in order.
    if (bestStatus === undefined || entryT >= bestT) {
      bestT = entryT;
      bestStatus = statusProp.value;
    }
  }

  return bestStatus;
}

/**
 * Report whether `c` carries a `CATEGORIES` property listing
 * {@link CATEGORY_STATUS_SUPERSESSION}.
 *
 * `CATEGORIES` values are comma-delimited per RFC 5545 §3.8.1.2, so
 * each token is trimmed and compared whole rather than substring-
 * matched against the raw value — otherwise
 * `status-supersession-deferred` would falsely match.
 */
function categoriesContainSupersession(c: Component): boolean {
  for (const p of getAll(c, PROP_CATEGORIES)) {
    for (const tok of p.value.split(",")) {
      if (tok.trim().toUpperCase() === CATEGORY_STATUS_SUPERSESSION.toUpperCase()) return true;
    }
  }
  return false;
}

/**
 * The parsed `DTSTAMP` of `c`, or the epoch when it is missing or
 * unparseable. {@link superseded} reads the epoch as "sorts to the
 * start": ledger noise is demoted, never fatal.
 */
function parseEntryDtstamp(c: Component): Instant {
  const raw = dtstampRaw(c);
  if (raw === "") return 0;
  return parseTime(raw) ?? 0;
}
