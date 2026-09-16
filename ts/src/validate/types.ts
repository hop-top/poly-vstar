// SPDX-License-Identifier: MIT

// spec/05 §5 — type-specific required properties.

import { CODES } from "../generated/codes.js";
import {
  COMP_EVENT,
  COMP_FREE_BUSY,
  COMP_TODO,
  TODO_COMPLETED,
  get,
  type Component,
} from "../types.js";
import { diagnostic, equalFold, has, wireType, type Diagnostic } from "./internal.js";

/**
 * The `VCARD` wire type, as it appears on a `Component`.
 *
 * `CompType` names no constant for it deliberately: a top-level vCard
 * is a `Card`. A `VCARD` block nested inside a `VCALENDAR` still parses
 * into a `Component` carrying the wire string, and the rule below
 * exists so a caller hand-building one still gets the check.
 */
const VCARD = "VCARD";

/**
 * Dispatch the per-type rule for `c`. Component types with no extra
 * MUST in spec/05 §5 — VJOURNAL, VTIMEZONE, VALARM, VCALENDAR —
 * yield nothing.
 */
export function checkTypeSpecific(c: Component, path: string): Diagnostic[] {
  switch (wireType(c)) {
    case COMP_TODO:
      return checkVTODO(c, path);
    case COMP_EVENT:
      return checkVEVENT(c, path);
    case COMP_FREE_BUSY:
      return checkVFREEBUSY(c, path);
    case VCARD:
      return checkVCARDComponent(c, path);
    default:
      return [];
  }
}

/**
 * A VTODO must be reachable as "scheduled": either DUE is present, or
 * STATUS=COMPLETED is paired with a COMPLETED timestamp.
 *
 * RFC 5545 §3.6.2 lets a finished VTODO drop DUE so long as COMPLETED
 * records when it finished; that route is honoured rather than
 * demanding a due date the task no longer has.
 */
function checkVTODO(c: Component, path: string): Diagnostic[] {
  if (has(c, "DUE")) return [];
  const status = get(c, "STATUS");
  if (status !== undefined && equalFold(status.value, TODO_COMPLETED) && has(c, "COMPLETED")) {
    return [];
  }
  return [
    diagnostic(
      CODES.CodeVTODOMissingDue,
      "VTODO requires DUE, or STATUS=COMPLETED paired with COMPLETED (spec/05 §5; RFC 5545 §3.6.2)",
      path,
    ),
  ];
}

/**
 * A VEVENT must carry DTSTART. RFC 5545 §3.6.1 allows its absence
 * outside a PUBLISH METHOD context; V\* is strict, because an agentic
 * playthrough always anchors to a start time.
 */
function checkVEVENT(c: Component, path: string): Diagnostic[] {
  if (has(c, "DTSTART")) return [];
  return [
    diagnostic(
      CODES.CodeVEVENTMissingDTSTART,
      "VEVENT requires DTSTART (spec/05 §5; RFC 5545 §3.6.1)",
      `${path}.DTSTART`,
    ),
  ];
}

/** A VFREEBUSY must carry both DTSTART and DTEND; the message names which is missing. */
function checkVFREEBUSY(c: Component, path: string): Diagnostic[] {
  const missing = requiredMissing(c, ["DTSTART", "DTEND"]);
  if (missing.length === 0) return [];
  return [
    diagnostic(
      CODES.CodeVFREEBUSYMissingTimes,
      `VFREEBUSY requires DTSTART and DTEND; missing: ${missing.join(", ")} (spec/05 §5; RFC 5545 §3.6.4)`,
      path,
    ),
  ];
}

/** A VCARD-as-Component must carry both VERSION and UID. */
function checkVCARDComponent(c: Component, path: string): Diagnostic[] {
  const missing = requiredMissing(c, ["VERSION", "UID"]);
  if (missing.length === 0) return [];
  return [
    diagnostic(
      CODES.CodeVCARDMissingRequired,
      `VCARD requires VERSION and UID; missing: ${missing.join(", ")} (spec/05 §5; RFC 6350 §6.7.6, §6.7.9)`,
      path,
    ),
  ];
}

/** Which of `names` `c` lacks, in the order given. */
function requiredMissing(c: Component, names: readonly string[]): string[] {
  return names.filter((name) => !has(c, name));
}
