// SPDX-License-Identifier: MIT

// spec/05 §8 — the STATUS value domain.

import { CODES } from "../generated/codes.js";
import {
  COMP_EVENT,
  COMP_JOURNAL,
  COMP_TODO,
  EVENT_CANCELLED,
  EVENT_CONFIRMED,
  EVENT_TENTATIVE,
  JOURNAL_CANCELLED,
  JOURNAL_DRAFT,
  JOURNAL_FINAL,
  TODO_CANCELLED,
  TODO_COMPLETED,
  TODO_IN_PROCESS,
  TODO_NEEDS_ACTION,
  get,
  type Component,
} from "../types.js";
import { diagnostic, equalFold, wireType, type Diagnostic } from "./internal.js";

/**
 * The STATUS values RFC 5545 §3.8.1.11 scopes to each component type.
 *
 * The vocabularies are per-type, not global: `CANCELLED` is the only
 * value all three share, and `DRAFT` — legal iCalendar text, and legal
 * on a VJOURNAL — is a conformance violation on a VEVENT. A type
 * absent from this table admits no STATUS vocabulary at all
 * (VFREEBUSY, VTIMEZONE, VALARM, VCALENDAR) and is skipped.
 *
 * Built from the port's own wire constants rather than read out of the
 * generated `STATUS_VOCABULARY`, deliberately. These are the values the
 * codec encodes against, so a table built from them cannot disagree
 * with what this library actually writes — a guarantee a lookup into a
 * generated table would give up. The registry's cross-language copy is
 * reconciled against this one in `test/registry-vocabulary.test.ts`,
 * which is what keeps the five ports agreeing without any of them
 * losing the codec linkage.
 */
const STATUS_VOCABULARIES: ReadonlyMap<string, readonly string[]> = new Map([
  [COMP_EVENT as string, [EVENT_TENTATIVE, EVENT_CONFIRMED, EVENT_CANCELLED]],
  [COMP_TODO as string, [TODO_NEEDS_ACTION, TODO_IN_PROCESS, TODO_COMPLETED, TODO_CANCELLED]],
  [COMP_JOURNAL as string, [JOURNAL_DRAFT, JOURNAL_FINAL, JOURNAL_CANCELLED]],
]);

/**
 * Flag a STATUS whose value is outside its own component type's
 * vocabulary.
 *
 * Comparison is case-insensitive per RFC 5545 §3.1. An absent STATUS
 * is clean — the property is optional on every type that admits it.
 */
export function checkStatusVocabulary(c: Component, path: string): Diagnostic[] {
  const allowed = STATUS_VOCABULARIES.get(wireType(c));
  if (allowed === undefined) return [];
  const p = get(c, "STATUS");
  if (p === undefined) return [];
  if (allowed.some((want) => equalFold(p.value, want))) return [];
  return [
    diagnostic(
      CODES.CodeStatusNotInVocabulary,
      `STATUS value ${p.value} is not valid for ${wireType(c)}; allowed: ${allowed.join(", ")} (RFC 5545 §3.8.1.11)`,
      `${path}.STATUS`,
    ),
  ];
}
