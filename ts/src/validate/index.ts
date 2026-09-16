// SPDX-License-Identifier: MIT

/**
 * `@hop-top/vstar/validate` — the V\* semantic invariants the codec
 * layer cannot catch.
 *
 * A document can be syntactically valid RFC 5545 or RFC 6350, so that
 * parsing succeeds, and still violate V\* discipline: a missing or
 * corrupted `X-VSTAR-HASH`, a property outside the extension
 * namespace, a type-specific required property absent, a `STATUS` from
 * the wrong vocabulary, a malformed recurrence or duration.
 *
 * The two entry points are {@link validate} (a whole `Calendar`) and
 * {@link validateComponent} (one `Component`). **Neither throws.**
 * Validation *is* the error channel: a document that fails every check
 * still validates successfully and returns a list of findings. An
 * empty list means clean. A port that throws on a diagnostic has
 * inverted the API.
 *
 * Every {@link Diagnostic} carries a stable `code` — cataloged in
 * [docs/validate-codes.md](../../../docs/validate-codes.md) and
 * generated from `spec/registry/` — that consumers may match
 * programmatically. `message` is human-readable and may change in any
 * release; do not match on it.
 *
 * Coverage of spec/05, the conformance criteria:
 *
 * - §1 required common properties (UID, DTSTAMP, X-VSTAR-HASH).
 * - §2 X-VSTAR-HASH integrity — the present-but-wrong case; an absent
 *   hash is reported by §1 instead.
 * - §3 extension namespace compliance — a non-standard property name
 *   without the `X-` prefix.
 * - §4 supersession discipline — a supersession VJOURNAL's required
 *   properties, and whether its `RELATED-TO` resolves.
 * - §5 type-specific required properties — VTODO, VEVENT, VFREEBUSY,
 *   VCARD.
 * - §6 RRULE conformance — unsupported (warning) and malformed
 *   (error).
 * - §7 DURATION well-formedness — the DURATION property, the relative
 *   form of TRIGGER, and the REPEAT count.
 * - §8 the STATUS value domain. The other domains §8 bounds (CLASS,
 *   TRANSP, PRIORITY, PERCENT-COMPLETE, SEQUENCE, REPEAT) have no code
 *   yet; the `helpers` accessors enforce them at write time.
 *
 * **Path syntax.** `Diagnostic.path` is a dotted component/property
 * locator:
 *
 * | Path | Meaning |
 * |---|---|
 * | `VCALENDAR` | Calendar-level. |
 * | `VCALENDAR.VTODO[uid=foo]` | Component-level, on the VTODO whose UID is `foo`. |
 * | `VCALENDAR.VTODO[uid=foo].DTSTAMP` | Property-level, on that VTODO's DTSTAMP. |
 * | `VCALENDAR.VTODO[#3]` | A UID-less VTODO, at positional index 3. |
 * | `VTODO[uid=foo].DTSTAMP` | {@link validateComponent} — no calendar prefix. |
 */

import { CODES, CODE_SEVERITIES, type Code, type Severity } from "../generated/codes.js";
import type { Calendar, Component } from "../types.js";
import { checkDuration } from "./duration.js";
import { checkExtensionNamespace, standardPropertyCount } from "./extensions.js";
import { checkHashIntegrity } from "./integrity.js";
import { componentPath, type Diagnostic, type PathIndex } from "./internal.js";
import { checkRequiredCommon } from "./required.js";
import { checkRRule } from "./rrule.js";
import { checkStatusVocabulary } from "./status.js";
import { checkSupersessionDiscipline } from "./supersession.js";
import { checkTypeSpecific } from "./types.js";

export { version } from "../version.js";
export { CODES, CODE_SEVERITIES };
export type { Code, Diagnostic, Severity };
export { standardPropertyCount };

/**
 * Check every component in `cal` and return the accumulated
 * diagnostics. An empty array means `cal` is clean.
 *
 * `cal` is not mutated. Diagnostics follow component order, then rule
 * order within a component; consumers that compare against a fixture
 * should sort, since only the set — not the emission order — is
 * contractual.
 *
 * Prefer this over {@link validateComponent} whenever a calendar
 * exists: the paths are more precise, and the cross-component rules
 * (the orphan-supersession check) can only run here.
 */
export function validate(cal: Calendar): Diagnostic[] {
  const out: Diagnostic[] = [];
  const index: PathIndex = new Map();
  for (const comp of cal.components) {
    const path = `VCALENDAR.${componentPath(comp, index)}`;
    out.push(...runChecks(comp, path, cal.components));
  }
  return out;
}

/**
 * Check a single `Component` in isolation. Paths start at the
 * component itself, e.g. `VTODO[uid=foo].DTSTAMP`.
 *
 * This is the right entry point when there is no parent calendar — a
 * freshly minted component, say, checked before it is appended. The
 * cross-component orphan-supersession rule is skipped, not guessed:
 * with no ledger to resolve against, "this target does not exist" is a
 * claim this entry point cannot make.
 */
export function validateComponent(c: Component): Diagnostic[] {
  return runChecks(c, componentPath(c, new Map()), undefined);
}

/** Every registry diagnostic code, in registry order. */
export function codes(): Code[] {
  return Object.values(CODES);
}

/**
 * The severity the registry assigns `code`, or `undefined` when the
 * string names no known code.
 *
 * Consumers must tolerate unknown codes — new ones may appear in any
 * release — so this reports absence rather than throwing.
 */
export function severityOf(code: string): Severity | undefined {
  return CODE_SEVERITIES[code as Code];
}

/**
 * Run every rule against `c` at `path`.
 *
 * `ledger` supplies the cross-component context the orphan-supersession
 * rule needs; `undefined` means there is none and that rule is skipped.
 */
function runChecks(c: Component, path: string, ledger: readonly Component[] | undefined): Diagnostic[] {
  return [
    ...checkRequiredCommon(c, path),
    ...checkHashIntegrity(c, path),
    ...checkExtensionNamespace(c, path),
    ...checkTypeSpecific(c, path),
    ...checkStatusVocabulary(c, path),
    ...checkSupersessionDiscipline(c, ledger, path),
    ...checkRRule(c, path),
    ...checkDuration(c, path),
  ];
}
