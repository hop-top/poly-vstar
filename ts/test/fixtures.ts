// SPDX-License-Identifier: MIT

// Shared conformance-corpus loader.
//
// The corpus is authored at `spec/v1.0/conformance/`, two levels above
// `ts/`. Paths are resolved from this file's own URL rather than from
// `process.cwd()` so the loader works whichever directory vitest is
// invoked from.

import { readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));

/** Absolute path to `spec/v1.0/conformance/`. */
export const CONFORMANCE_DIR = join(here, "..", "..", "spec", "v1.0", "conformance");

/** One fixture: the stem plus whichever sibling files exist. */
export interface Fixture {
  /** Basename without extension, e.g. `world`. */
  readonly stem: string;
  /** Absolute path of the input document. */
  readonly path: string;
  /** Raw input bytes, exactly as they sit on disk (LF-terminated). */
  readonly input: Uint8Array;
  /** The `.error` sibling's first non-empty line, when present. */
  readonly sentinel: string | undefined;
}

/**
 * Enumerate every fixture in `spec/v1.0/conformance/<family>/` whose
 * input carries `ext`. Sorted by stem so failures report in a stable
 * order.
 */
export function loadFixtures(family: string, ext: string): Fixture[] {
  const dir = join(CONFORMANCE_DIR, family);
  return readdirSync(dir)
    .filter((name) => name.endsWith(ext))
    .sort()
    .map((name) => {
      const stem = name.slice(0, -ext.length);
      const path = join(dir, name);
      return {
        stem,
        path,
        input: new Uint8Array(readFileSync(path)),
        sentinel: readSentinel(dir, stem),
      };
    });
}

/**
 * Read `<dir>/<stem>.error` and return its first non-empty trimmed
 * line — the sentinel identifier the fixture asserts. Returns
 * `undefined` when the sibling does not exist.
 */
function readSentinel(dir: string, stem: string): string | undefined {
  let raw: string;
  try {
    raw = readFileSync(join(dir, `${stem}.error`), "utf8");
  } catch {
    return undefined;
  }
  for (const line of raw.split("\n")) {
    const trimmed = line.trim();
    if (trimmed !== "") return trimmed;
  }
  return undefined;
}

/** Every `fuzz-seed/<family>/*.bytes` input, sorted by filename. */
export function loadFuzzSeeds(family: string): Fixture[] {
  return loadFixtures(join("fuzz-seed", family), ".bytes");
}

/**
 * Assert two byte sequences are identical, reporting the first
 * divergent offset with a window of context on failure.
 *
 * This compares BYTES. It never trims, never re-decodes, and never
 * normalizes line endings — see the porting guide's
 * "byte comparisons compare BYTES" rule.
 */
export function assertBytesEqual(got: Uint8Array, want: Uint8Array, label = ""): void {
  const n = Math.min(got.length, want.length);
  for (let i = 0; i < n; i++) {
    if (got[i] !== want[i]) {
      throw new Error(
        `${label ? label + ": " : ""}bytes diverge at offset ${i}: ` +
          `got 0x${hex(got[i])} (${show(got, i)}), ` +
          `want 0x${hex(want[i])} (${show(want, i)})`,
      );
    }
  }
  if (got.length !== want.length) {
    throw new Error(
      `${label ? label + ": " : ""}byte lengths differ: got ${got.length}, want ${want.length}; ` +
        `common prefix of ${n} bytes matches`,
    );
  }
}

function hex(b: number | undefined): string {
  return (b ?? 0).toString(16).padStart(2, "0");
}

/** A short printable window around offset `i`, for failure messages. */
function show(bytes: Uint8Array, i: number): string {
  const from = Math.max(0, i - 12);
  const to = Math.min(bytes.length, i + 12);
  const slice = new TextDecoder("utf8", { fatal: false }).decode(bytes.subarray(from, to));
  return JSON.stringify(slice);
}

// ── RRULE corpus ──────────────────────────────────────────────────
//
// `spec/v1.0/conformance/rrule/` follows its own convention: the input
// is `<stem>.rrule` (one RRULE property value) or `<stem>.ics` (a
// VCALENDAR whose first component carries the recurrence set), and each
// JSON sidecar states the result of one evaluator call.
//
// The tree is walked rather than enumerated, so a fixture added to the
// corpus becomes a test without an edit here.

/** The shape shared by every JSON sidecar that states one call's result. */
export interface OutcomeSpec {
  readonly dtstart?: string;
  readonly after?: string;
  readonly start?: string;
  readonly end?: string;
  readonly limit?: number;
  readonly expected?: readonly string[];
  readonly complete?: boolean;
  /** The failure class the call must produce, in place of `expected`. */
  readonly error?: string;
}

/** The `.expect.json` shape: a failure class named by its Go spelling. */
interface SentinelSpec {
  readonly sentinel?: string;
}

/** One `<stem>.rrule` input plus whichever sidecars it carries. */
export interface RruleFixture {
  /** Subdirectory under `rrule/`, e.g. `evaluator`. */
  readonly dir: string;
  /** `<dir>/<stem>`, the label failures report. */
  readonly id: string;
  /** The RRULE property value, with its trailing newline removed. */
  readonly value: string;
  /** `.expect.json`'s `sentinel`, when the fixture names one. */
  readonly sentinel: string | undefined;
  /** `.formatted`'s content, with its trailing newline removed. */
  readonly formatted: string | undefined;
  readonly next: OutcomeSpec | undefined;
  readonly expand: OutcomeSpec | undefined;
  readonly between: OutcomeSpec | undefined;
  /** Whether any sidecar at all exists — a bare fixture pins only "this parses". */
  readonly hasAnySidecar: boolean;
}

/** One `<stem>.ics` recurrence-set input plus its sidecar. */
export interface SetFixture {
  readonly dir: string;
  readonly id: string;
  readonly input: Uint8Array;
  readonly sentinel: string | undefined;
  readonly occurrences: OutcomeSpec | undefined;
}

/** Absolute path to `spec/v1.0/conformance/rrule/`. */
const RRULE_DIR = join(CONFORMANCE_DIR, "rrule");

/** Every file under `dir`, recursively, as absolute paths. */
function walk(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...walk(path));
    else out.push(path);
  }
  return out;
}

/** Read a file, or `undefined` when it does not exist. */
function readOptional(path: string): string | undefined {
  try {
    return readFileSync(path, "utf8");
  } catch {
    return undefined;
  }
}

/** Read and parse an optional JSON sidecar. */
function readJson<T>(path: string): T | undefined {
  const raw = readOptional(path);
  return raw === undefined ? undefined : (JSON.parse(raw) as T);
}

/** Trailing CR/LF removed — the corpus is LF on disk, values are not. */
function trimEol(s: string): string {
  return s.replace(/[\r\n]+$/, "");
}

/** The fixture's subdirectory label relative to `rrule/`, e.g. `set/rejected`. */
function fixtureDir(stem: string): string {
  const rel = stem.slice(RRULE_DIR.length + 1);
  const cut = rel.lastIndexOf("/");
  return cut < 0 ? "" : rel.slice(0, cut);
}

/** Every `<stem>.rrule` fixture in the corpus, sorted by path. */
export function loadRruleFixtures(): RruleFixture[] {
  return walk(RRULE_DIR)
    .filter((p) => p.endsWith(".rrule"))
    .map((path) => {
      const stem = path.slice(0, -".rrule".length);
      const sentinel = readJson<SentinelSpec>(`${stem}.expect.json`)?.sentinel;
      const formatted = readOptional(`${stem}.formatted`);
      const next = readJson<OutcomeSpec>(`${stem}.next.json`);
      const expand = readJson<OutcomeSpec>(`${stem}.expand.json`);
      const betweenSpec = readJson<OutcomeSpec>(`${stem}.between.json`);
      return {
        dir: fixtureDir(stem),
        id: stem.slice(RRULE_DIR.length + 1),
        value: trimEol(readFileSync(path, "utf8")),
        sentinel,
        formatted: formatted === undefined ? undefined : trimEol(formatted),
        next,
        expand,
        between: betweenSpec,
        hasAnySidecar:
          sentinel !== undefined ||
          formatted !== undefined ||
          next !== undefined ||
          expand !== undefined ||
          betweenSpec !== undefined,
      };
    });
}

/** Every `<stem>.ics` recurrence-set fixture in the corpus, sorted by path. */
export function loadSetFixtures(): SetFixture[] {
  return walk(RRULE_DIR)
    .filter((p) => p.endsWith(".ics"))
    .map((path) => {
      const stem = path.slice(0, -".ics".length);
      return {
        dir: fixtureDir(stem),
        id: stem.slice(RRULE_DIR.length + 1),
        input: new Uint8Array(readFileSync(path)),
        sentinel: readJson<SentinelSpec>(`${stem}.expect.json`)?.sentinel,
        occurrences: readJson<OutcomeSpec>(`${stem}.occurrences.json`),
      };
    });
}
