// SPDX-License-Identifier: MIT

// spec/05 §4 — supersession discipline.

import { CODES } from "../generated/codes.js";
import { CATEGORY_STATUS_SUPERSESSION, PROP_EFFECTIVE_STATUS } from "../supersession/index.js";
import { COMP_JOURNAL, get, getAll, uid as componentUid, type Component } from "../types.js";
import { diagnostic, equalFold, wireType, type Diagnostic } from "./internal.js";

/** The RFC 5545 §3.8.4.5 property carrying the supersession target. */
const RELATED_TO = "RELATED-TO";

/**
 * Check a supersession VJOURNAL against spec/05 §4.
 *
 * Two rules live here. The missing-properties rule is component-local:
 * a supersession journal that names no target, or records no effective
 * status, is incomplete on its own terms. The orphan rule is not — it
 * asks whether the named target exists, which only a ledger can
 * answer.
 *
 * `ledger` is `undefined` for the single-component entry point, and
 * the orphan rule is then skipped entirely rather than guessed at.
 * Absence of evidence is not evidence of an orphan: a component
 * validated in isolation has no calendar, so "the target is missing"
 * is a claim it is not in a position to make.
 */
export function checkSupersessionDiscipline(
  c: Component,
  ledger: readonly Component[] | undefined,
  path: string,
): Diagnostic[] {
  if (wireType(c) !== COMP_JOURNAL) return [];
  if (!categoriesContainSupersession(c)) return [];

  const out: Diagnostic[] = [];

  const related = get(c, RELATED_TO);
  const target = related?.value.trim() ?? "";
  if (target === "") {
    out.push(
      diagnostic(
        CODES.CodeSupersessionMissingProps,
        `supersession VJOURNAL missing ${RELATED_TO} (spec/02, spec/05 §4)`,
        `${path}.${RELATED_TO}`,
      ),
    );
  }

  const effective = get(c, PROP_EFFECTIVE_STATUS);
  if ((effective?.value.trim() ?? "") === "") {
    out.push(
      diagnostic(
        CODES.CodeSupersessionMissingProps,
        `supersession VJOURNAL missing ${PROP_EFFECTIVE_STATUS} (spec/02, spec/05 §4)`,
        `${path}.${PROP_EFFECTIVE_STATUS}`,
      ),
    );
  }

  if (related !== undefined && ledger !== undefined && target !== "" && !ledgerContainsUid(ledger, target)) {
    out.push(
      diagnostic(
        CODES.CodeSupersessionOrphan,
        `supersession VJOURNAL ${RELATED_TO}=${target} has no matching component in calendar (spec/02, spec/05 §4)`,
        `${path}.${RELATED_TO}`,
      ),
    );
  }

  return out;
}

/**
 * Whether `c` carries a CATEGORIES token naming the supersession
 * category, comma-separated and compared case-insensitively.
 */
function categoriesContainSupersession(c: Component): boolean {
  return getAll(c, "CATEGORIES").some((p) =>
    p.value.split(",").some((tok) => equalFold(tok.trim(), CATEGORY_STATUS_SUPERSESSION)),
  );
}

/**
 * Whether any component in `ledger` has UID `uid`.
 *
 * Comparison is case-sensitive per RFC 5545 §3.8.4.7: UIDs are opaque
 * identifiers, not user-facing text.
 */
function ledgerContainsUid(ledger: readonly Component[], uid: string): boolean {
  return ledger.some((c) => componentUid(c) === uid);
}
