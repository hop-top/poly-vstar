// SPDX-License-Identifier: MIT

// Loader for the language-agnostic behavior fixtures at
// `spec/behavior/`. Two shapes appear (see spec/behavior/README.md):
// standalone JSON holding a list of self-contained cases, and an `.ics`
// paired with a same-basename `.json` sibling.

import { readFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));

/** Absolute path to `spec/behavior/`. */
export const BEHAVIOR_DIR = join(here, "..", "..", "spec", "behavior");

/**
 * Every stem in `spec/behavior/<family>/` whose file ends in `suffix`,
 * sorted.
 *
 * The gates walk the tree rather than hard-coding names, so a fixture
 * added to the reference is picked up without a test edit — and a
 * fixture that silently disappears shows up as a dropped case count
 * rather than a green run.
 */
export function behaviorStems(family: string, suffix: string): string[] {
  return readdirSync(join(BEHAVIOR_DIR, family))
    .filter((name) => name.endsWith(suffix))
    .map((name) => name.slice(0, -suffix.length))
    .sort();
}

/** Read and decode a standalone behavior JSON, e.g. `time/tzid.json`. */
export function loadBehaviorJson<T>(family: string, name: string): T {
  return JSON.parse(readFileSync(join(BEHAVIOR_DIR, family, name), "utf8")) as T;
}

/** Read the raw bytes of a behavior-family input document. */
export function loadBehaviorInput(family: string, name: string): Uint8Array {
  return new Uint8Array(readFileSync(join(BEHAVIOR_DIR, family, name)));
}

/** One `spec/behavior/time/tzid.json` row. */
export interface TzidCase {
  readonly calendar: string;
  readonly tzid: string;
  readonly value: string;
  /** `null` means the reference's `(value, ok)` pair reported not-ok. */
  readonly utc: string | null;
}

/** One `spec/behavior/duration/parse.json` row. */
export interface DurationParseCase {
  readonly value: string;
  readonly seconds?: number;
  readonly negative?: boolean;
  readonly error?: string;
}

/** One `spec/behavior/duration/<name>.trigger.json` row. */
export interface TriggerCase {
  readonly alarm_uid: string;
  readonly fires_at?: string;
  readonly error?: string;
}

/** One `spec/behavior/ext/scopes.json` row. */
export interface ScopeCase {
  readonly name: string;
  readonly scope: string;
  /** `null` for every scope but `"system"`. */
  readonly system: string | null;
}

/** One property parameter inside a `spec/behavior/diff/` op. */
export interface DiffParamCase {
  readonly name: string;
  readonly value: string;
}

/** One property-level change inside a `spec/behavior/diff/` entry. */
export interface DiffOpCase {
  readonly op: string;
  readonly property: string;
  /** `null` on an add. */
  readonly before: string | null;
  /** `null` on a remove. */
  readonly after: string | null;
  readonly before_params?: DiffParamCase[];
  readonly after_params?: DiffParamCase[];
}

/** One `spec/behavior/diff/<name>.diff.json` entry. */
export interface DiffCase {
  readonly uid: string;
  readonly path: string;
  readonly ops: DiffOpCase[];
  /** Present only when non-empty. */
  readonly subs?: DiffCase[];
}

/**
 * A `spec/behavior/supersession/<name>.effective.json` table: component
 * UID to the status the ledger projects onto it. A UID absent from the
 * map is not superseded.
 */
export type EffectiveStatusCase = Record<string, string>;
