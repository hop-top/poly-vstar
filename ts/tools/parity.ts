// SPDX-License-Identifier: MIT

/**
 * The TypeScript parity emitter for the cross-language parity harness.
 *
 * It takes the `spec/` directory as its single argument, runs the V*
 * public API over every fixture in `spec/v0.1/conformance/` and
 * `spec/behavior/`, and prints ONE JSON document to stdout.
 * `tools/parity/parity.py` diffs this document against the Go
 * reference's, key by key; any difference fails the run.
 *
 * The normative description of the document — every key, every value
 * shape, every ordering rule — is `tools/parity/README.md`. This
 * command implements that document, not the Go emitter's source.
 *
 * Three rules shape every value:
 *
 *   - Nothing human-readable is emitted. Diagnostic messages and
 *     exception text reword between versions without the behavior
 *     changing. Codes, paths, severities, op kinds and failure-class
 *     tokens are the stable surface.
 *   - A failure serializes as `{"error": "<SentinelName>"}` using the
 *     Go sentinel's identifier as the cross-language token. This port
 *     carries those identifiers natively as `VstarError.code`, so
 *     classification is a lookup rather than a translation.
 *   - Output is deterministic. `JSON.stringify` preserves insertion
 *     order rather than sorting, so every object this file builds is
 *     assembled through {@link sortedObject}; every array is built in
 *     an order the contract pins.
 *
 * Usage:
 *
 *     node dist/tools/parity.js ../spec
 *
 * Exits 0 having printed the document; exits 1 with a diagnostic on
 * stderr when a fixture cannot be read or a contract it depends on is
 * broken.
 */

import { createHash } from "node:crypto";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { basename, join, relative, resolve, sep } from "node:path";

import { VstarError } from "../src/errors.js";
import { formatTime, parseTime, parseTimeWithTzid } from "../src/time.js";
import { uid as componentUid } from "../src/types.js";
import type { Calendar, Card, Component, Param } from "../src/types.js";
import * as canonical from "../src/canonical/index.js";
import * as diff from "../src/diff/index.js";
import * as duration from "../src/duration/index.js";
import * as ext from "../src/ext/index.js";
import * as hashing from "../src/hashing/index.js";
import * as helpers from "../src/helpers/index.js";
import * as rfc5545 from "../src/codec/rfc5545/index.js";
import * as rfc6350 from "../src/codec/rfc6350/index.js";
import * as rrule from "../src/rrule/index.js";
import * as supersession from "../src/supersession/index.js";
import * as validate from "../src/validate/index.js";

/** Fixture extensions, named once so the families read alike. */
const EXT_ICS = ".ics";
const EXT_VCF = ".vcf";
const EXT_RRULE = ".rrule";

/**
 * The escape hatch for a failure matching no class the family
 * recognizes. It surfaces under a name no port implements — a loud
 * mismatch — rather than silently under a wrong token.
 */
const CLASS_UNCLASSIFIED = "Unclassified";

/** The general class every codec layer may wrap; always tried last. */
const CLASS_MALFORMED = "ErrMalformed";

/**
 * Classify a thrown value into its cross-language token, first-match
 * over `table` ordered specific to general.
 *
 * The Go reference matches with `errors.Is`, which walks a wrap chain;
 * this port raises a single {@link VstarError} whose `code` already
 * names the class, so membership in the table is the whole test. A
 * failure that is not a `VstarError`, or whose code the family does
 * not list, reports `undefined` and the caller turns that into a
 * diagnostic rather than an emitted token no port would recognize.
 */
function classify(err: unknown, table: readonly string[]): string | undefined {
  if (!(err instanceof VstarError)) return undefined;
  return table.includes(err.code) ? err.code : undefined;
}

/**
 * Classify a failure, falling back to {@link CLASS_UNCLASSIFIED}.
 *
 * Used by the families whose contract names that token — duration
 * parsing and alarm resolution — where an unrecognized class travels
 * into the document instead of aborting the emitter.
 */
function classifyOrUnclassified(err: unknown, table: readonly string[]): string {
  return classify(err, table) ?? CLASS_UNCLASSIFIED;
}

/** Abort with a diagnostic on stderr, mirroring the reference's exit 1. */
function fail(message: string): never {
  process.stderr.write(`parity: ${message}\n`);
  process.exit(1);
}

/**
 * Rebuild `obj` with its keys in sorted order.
 *
 * Go's `encoding/json` sorts map keys; `JSON.stringify` emits an
 * object's own insertion order. Every emitted object therefore passes
 * through here, so the two documents compare byte-for-byte rather than
 * merely key-for-key.
 *
 * Only the top level of `obj` is reordered. Entry values are built by
 * their family and are either arrays, whose order is itself the
 * contract, or objects this function already produced.
 */
function sortedObject<T>(obj: Record<string, T>): Record<string, T> {
  const out: Record<string, T> = {};
  for (const key of Object.keys(obj).sort(compareStrings)) {
    out[key] = obj[key] as T;
  }
  return out;
}

/**
 * Compare by UTF-16 code unit, the ordering Go's `sort.Strings`
 * produces over the same ASCII keys. `localeCompare` is deliberately
 * avoided: its order depends on the host's locale.
 */
function compareStrings(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

/** Whether `path` exists, used to test for a sidecar. */
function exists(path: string): boolean {
  try {
    statSync(path);
    return true;
  } catch {
    return false;
  }
}

/**
 * Call `fn` for every non-directory entry of `dir` carrying `ext`, in
 * sorted order, with the full path and the extension-stripped stem.
 *
 * A missing directory is a silent no-op, so the emitter tolerates a
 * corpus that has yet to grow a family.
 */
function eachFixture(dir: string, ext_: string, fn: (path: string, stem: string) => void): void {
  let names: string[];
  try {
    names = readdirSync(dir);
  } catch {
    return;
  }
  const matching = names.filter((n) => n.endsWith(ext_)).sort(compareStrings);
  for (const name of matching) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) continue;
    fn(full, full.slice(0, full.length - ext_.length));
  }
}

/**
 * Every file under `root`, recursively, in a deterministic order.
 *
 * The rrule corpus is the one nested tree; ordering by the rendered
 * key rather than by traversal keeps the walk independent of the
 * platform's directory order.
 */
function walkFiles(root: string): string[] {
  const out: string[] = [];
  const visit = (dir: string): void => {
    let names: string[];
    try {
      names = readdirSync(dir);
    } catch {
      return;
    }
    for (const name of names.sort(compareStrings)) {
      const full = join(dir, name);
      if (statSync(full).isDirectory()) visit(full);
      else out.push(full);
    }
  };
  visit(root);
  return out;
}

/**
 * Render `stem` as a slash-separated path relative to `root`: the
 * document's key space is filesystem layout with the extension
 * dropped, identical on every platform a port runs on.
 */
function fixtureKey(root: string, stem: string): string {
  return relative(root, stem).split(sep).join("/");
}

/**
 * Key `value` by `stem` relative to `root`, refusing a collision.
 *
 * Two fixtures colliding on one key would silently drop a case from
 * the document and shrink the harness' coverage without failing it.
 */
function record<T>(out: Record<string, T>, root: string, stem: string, value: T): void {
  const key = fixtureKey(root, stem);
  if (key in out) fail(`duplicate fixture key ${JSON.stringify(key)}`);
  out[key] = value;
}

/** Read a fixture as raw bytes. */
function readBytes(path: string): Uint8Array {
  try {
    return new Uint8Array(readFileSync(path));
  } catch (err) {
    return fail(`read ${path}: ${String(err)}`);
  }
}

/** Read a fixture as UTF-8 text. */
function readText(path: string): string {
  return new TextDecoder().decode(readBytes(path));
}

/** Read and parse a JSON sidecar. */
function readJson<T>(path: string): T {
  try {
    return JSON.parse(readText(path)) as T;
  } catch (err) {
    return fail(`parse ${path}: ${String(err)}`);
  }
}

/** Parse an `.ics` fixture into a Calendar, failing the emitter on error. */
function readCalendar(path: string): Calendar {
  try {
    return rfc5545.parse(readBytes(path));
  } catch (err) {
    return fail(`parse ${path}: ${String(err)}`);
  }
}

// --- conformance ----------------------------------------------------

/**
 * One parseable fixture's wire contract: the content hash this port
 * computes, and a digest of the canonical bytes it hashed.
 */
interface ConformanceEntry {
  readonly hash: string;
  readonly canonical_sha256: string;
}

/** The universal failure shape. */
interface ErrorEntry {
  readonly error: string;
}

/**
 * The vocabulary a `malformed/*` fixture may produce, most specific
 * first. The emitter reports which one the fixture actually produced;
 * this list only bounds what behavior is recognized.
 */
const MALFORMED_CLASSES = [
  "ErrUnsupportedVersion",
  "ErrUnclosedBlock",
  "ErrMissingUID",
  CLASS_MALFORMED,
] as const;

/** The conformance subdirectories whose `.ics` files parse into a Calendar. */
const CALENDAR_DIRS = ["rfc5545", "supersession"] as const;

/**
 * Hash canonical bytes after folding CRLF to LF.
 *
 * The canonical form per spec/03 is CRLF, but the corpus stores it LF
 * and every port reads the same LF file. Digesting the LF form keeps
 * line-ending handling from producing false mismatches between ports.
 */
function canonicalDigest(bytes: Uint8Array): string {
  return createHash("sha256").update(crlfToLf(bytes)).digest("hex");
}

/** Replace every CRLF pair in `bytes` with a single LF. */
function crlfToLf(bytes: Uint8Array): Uint8Array {
  const out = new Uint8Array(bytes.length);
  let n = 0;
  for (let i = 0; i < bytes.length; i++) {
    if (bytes[i] === 0x0d && bytes[i + 1] === 0x0a) continue;
    out[n++] = bytes[i] as number;
  }
  return out.subarray(0, n);
}

/**
 * Compare a computed hash against the committed `<stem>.hash` sibling.
 *
 * This self-check is what keeps a port from emitting a hash that is
 * merely self-consistent: the corpus carries an independent
 * expectation for this one family, and disagreeing with it is a broken
 * port rather than a parity mismatch to report downstream.
 */
function checkHashSibling(path: string, ext_: string, got: string): void {
  const sibling = `${path.slice(0, path.length - ext_.length)}.hash`;
  const want = readText(sibling).replace(/[\r\n]+$/, "");
  if (got !== want) fail(`${sibling}: hash ${got} does not match committed ${want}`);
}

/** Parse an `.ics` fixture, canonicalize, hash, and self-check. */
function calendarEntry(path: string): ConformanceEntry {
  let cal: Calendar;
  try {
    cal = rfc5545.parse(readBytes(path));
  } catch (err) {
    return fail(`parse ${path}: ${String(err)}`);
  }
  const entry: ConformanceEntry = {
    hash: hashing.calendar(cal),
    canonical_sha256: canonicalDigest(canonical.calendar(cal)),
  };
  checkHashSibling(path, EXT_ICS, entry.hash);
  return entry;
}

/**
 * {@link calendarEntry} for a `.vcf` fixture. The corpus holds exactly
 * one card per file; more would mean the fixture changed shape under
 * the emitter and the contract no longer says which card the hash
 * belongs to.
 */
function cardEntry(path: string): ConformanceEntry {
  let cards: Card[];
  try {
    cards = rfc6350.parse(readBytes(path));
  } catch (err) {
    return fail(`parse ${path}: ${String(err)}`);
  }
  if (cards.length !== 1) {
    fail(`${path}: expected exactly one card, got ${cards.length}`);
  }
  const card = cards[0] as Card;
  const entry: ConformanceEntry = {
    hash: hashing.card(card),
    canonical_sha256: canonicalDigest(canonical.card(card)),
  };
  checkHashSibling(path, EXT_VCF, entry.hash);
  return entry;
}

/**
 * Run a malformed fixture through the codec and report the token it
 * produced.
 *
 * Most fixtures fail at parse time. `ErrMissingUID` is encoder-only in
 * v0.1 — the rfc6350 parser accepts a UID-less VCARD and the encoder
 * refuses it — so a fixture that parses is re-encoded and the encode
 * failure classified instead. A fixture where both stages succeed is a
 * fault.
 */
function malformedToken(path: string, ext_: string): string {
  const input = readBytes(path);
  let parseErr: unknown;
  let encodeErr: unknown;
  let parsed = false;

  if (ext_ === EXT_ICS) {
    try {
      rfc5545.parse(input);
      parsed = true;
    } catch (err) {
      parseErr = err;
    }
  } else {
    let cards: Card[] | undefined;
    try {
      cards = rfc6350.parse(input);
      parsed = true;
    } catch (err) {
      parseErr = err;
    }
    if (cards !== undefined) {
      if (cards.length === 0) {
        fail(`${path}: parse returned no cards and no error`);
      }
      try {
        for (const card of cards) rfc6350.encode(card);
      } catch (err) {
        encodeErr = err;
        parsed = false;
      }
    }
  }

  const fromParse = classify(parseErr, MALFORMED_CLASSES);
  if (fromParse !== undefined) return fromParse;
  const fromEncode = classify(encodeErr, MALFORMED_CLASSES);
  if (fromEncode !== undefined) return fromEncode;
  if (parsed) fail(`${path}: expected a failure, parse and encode both succeeded`);
  return fail(`${path}: failure matches no known sentinel: ${String(parseErr ?? encodeErr)}`);
}

/**
 * Walk the conformance corpus, keying every fixture by its path
 * relative to the corpus root without extension.
 *
 * `time/` fixtures are VTIMEZONE registries the `time` family
 * consumes and carry no `.canonical`/`.hash` siblings, so they are not
 * part of this family; `fuzz-seed/` is fuzz-target input.
 */
function emitConformance(corpus: string): Record<string, ConformanceEntry | ErrorEntry> {
  const out: Record<string, ConformanceEntry | ErrorEntry> = {};

  for (const dir of CALENDAR_DIRS) {
    eachFixture(join(corpus, dir), EXT_ICS, (path, stem) => {
      record(out, corpus, stem, calendarEntry(path));
    });
  }
  eachFixture(join(corpus, "rfc6350"), EXT_VCF, (path, stem) => {
    record(out, corpus, stem, cardEntry(path));
  });
  const malformed = join(corpus, "malformed");
  for (const ext_ of [EXT_ICS, EXT_VCF]) {
    eachFixture(malformed, ext_, (path, stem) => {
      record(out, corpus, stem, { error: malformedToken(path, ext_) });
    });
  }
  return sortedObject(out);
}

// --- rrule ----------------------------------------------------------

/**
 * The failure vocabulary the rrule family reports, most specific
 * first. Every token is a class named by spec/05 §Failure classes.
 */
const RRULE_CLASSES = [
  "ErrUnsupportedRRule",
  "ErrIterationCap",
  "ErrUnboundedExpansion",
  CLASS_MALFORMED,
] as const;

/**
 * The union of every rrule sidecar's input fields. The emitter reads
 * inputs from the sidecar and reports outputs from its own
 * implementation — it never copies a sidecar's expected values into
 * the document, which would make the harness compare fixtures to
 * themselves.
 */
interface SidecarSpec {
  readonly dtstart?: string;
  readonly after?: string;
  readonly start?: string;
  readonly end?: string;
  readonly limit?: number;
  /**
   * Read only to learn how many `nextOccurrence` steps the
   * `.next.json` sidecar walks; its values are never emitted.
   */
  readonly expected?: readonly string[];
}

/** One rrule entry's flat key set. */
type RRuleEntry = Record<string, unknown>;

/** Read a sidecar's input fields, reporting `undefined` when absent. */
function readSidecar(path: string): SidecarSpec | undefined {
  if (!exists(path)) return undefined;
  return readJson<SidecarSpec>(path);
}

/** Read an RFC 5545 form #2 sidecar input value. */
function parseStamp(path: string, field: string, value: string | undefined): number {
  const t = value === undefined ? undefined : parseTime(value);
  if (t === undefined) fail(`${path}: bad ${field} ${JSON.stringify(value ?? "")}`);
  return t;
}

/**
 * Classify a failure into its rrule class token, refusing to emit
 * anything for a success or an unrecognized class. An unrecognized
 * class must fail the emitter rather than travel into the document as
 * a token no port implements.
 */
function failureToken(path: string, err: unknown, succeeded: boolean): string {
  if (succeeded) fail(`${path}: expected a failure, the call succeeded`);
  const token = classify(err, RRULE_CLASSES);
  if (token === undefined) fail(`${path}: failure matches no known class: ${String(err)}`);
  return token;
}

/**
 * Render occurrences as RFC 5545 form #2, the one timestamp shape
 * every family and every port uses.
 */
function formatStamps(ts: readonly number[]): string[] {
  return ts.map((t) => formatTime(t));
}

/**
 * Record a bounded-expansion outcome on `entry`: the occurrence list
 * under `key` plus a `<key>_complete` flag on success, or
 * `<key>_error` naming the failure class.
 *
 * Every key is flat on the entry rather than nested under one
 * sub-object, so which contracts a fixture pins is readable off the
 * entry's key set.
 */
function addBounded(
  entry: RRuleEntry,
  path: string,
  key: string,
  run: () => { times: readonly number[]; complete: boolean },
): void {
  let result: { times: readonly number[]; complete: boolean };
  try {
    result = run();
  } catch (err) {
    entry[`${key}_error`] = failureToken(path, err, false);
    return;
  }
  entry[key] = formatStamps(result.times);
  entry[`${key}_complete`] = result.complete;
}

/** Report `Rule.toString()` when a `<stem>.formatted` sidecar exists. */
function addFormatted(stem: string, rule: rrule.Rule, entry: RRuleEntry): void {
  if (!exists(`${stem}.formatted`)) return;
  entry["formatted"] = rule.toString();
}

/**
 * Walk `nextOccurrence` the way the sidecar's expected list is shaped:
 * one step per entry, each feeding its result back as the next
 * `after`. The emitted list is what this port yielded.
 *
 * One extra step runs past the end, and its outcome is the terminal
 * contract: `next_error` names the class the series failed with, or
 * `next_complete` states whether it terminated. Emitting that step
 * unconditionally means a port cannot pass by stopping early — a
 * series that should terminate and one that should raise
 * `ErrIterationCap` differ in the document.
 */
function addNext(stem: string, rule: rrule.Rule, entry: RRuleEntry): void {
  const path = `${stem}.next.json`;
  const spec = readSidecar(path);
  if (spec === undefined) return;
  const dt = parseStamp(path, "dtstart", spec.dtstart);
  let after = parseStamp(path, "after", spec.after);

  const stamps: string[] = [];
  const steps = spec.expected?.length ?? 0;
  for (let i = 0; i < steps; i++) {
    let got: number | undefined;
    try {
      got = rrule.nextOccurrence(rule, dt, after);
    } catch (err) {
      fail(`${path}: nextOccurrence step ${i} failed: ${String(err)}`);
    }
    if (got === undefined) fail(`${path}: nextOccurrence step ${i} terminated early`);
    stamps.push(formatTime(got));
    after = got;
  }
  entry["next"] = stamps;

  try {
    const got = rrule.nextOccurrence(rule, dt, after);
    entry["next_complete"] = got === undefined;
  } catch (err) {
    entry["next_error"] = failureToken(path, err, false);
  }
}

/** Report bounded expansion over the `.expand.json` sidecar's limit. */
function addExpand(stem: string, rule: rrule.Rule, entry: RRuleEntry): void {
  const path = `${stem}.expand.json`;
  const spec = readSidecar(path);
  if (spec === undefined) return;
  const dt = parseStamp(path, "dtstart", spec.dtstart);
  addBounded(entry, path, "expand", () => rrule.occurrences(rule, dt, spec.limit ?? 0));
}

/**
 * Report `between` over the sidecar's half-open window. `between` has
 * no completeness notion — the window bounds the answer — so the
 * success shape is the list alone.
 */
function addBetween(stem: string, rule: rrule.Rule, entry: RRuleEntry): void {
  const path = `${stem}.between.json`;
  const spec = readSidecar(path);
  if (spec === undefined) return;
  const dt = parseStamp(path, "dtstart", spec.dtstart);
  const start = parseStamp(path, "start", spec.start);
  const end = parseStamp(path, "end", spec.end);
  try {
    entry["between"] = formatStamps(rrule.between(rule, dt, start, end));
  } catch (err) {
    entry["between_error"] = failureToken(path, err, false);
  }
}

/**
 * Evaluate one `<stem>.rrule` fixture against every sidecar it has.
 *
 * A `.expect.json` sidecar means the rule must be rejected: the
 * emitter reports the class `validateRRule` produced and stops, since
 * no other contract applies to a rule that does not parse.
 */
function ruleEntry(path: string): RRuleEntry {
  const value = readText(path).replace(/[\r\n]+$/, "");
  const stem = path.slice(0, path.length - EXT_RRULE.length);

  if (exists(`${stem}.expect.json`)) {
    let succeeded = false;
    let err: unknown;
    try {
      rrule.validateRRule(value);
      succeeded = true;
    } catch (e) {
      err = e;
    }
    return { error: failureToken(path, err, succeeded) };
  }

  let rule: rrule.Rule;
  try {
    rule = rrule.parseRRule(value);
  } catch (err) {
    return fail(`${path}: parseRRule failed: ${String(err)}`);
  }
  const entry: RRuleEntry = { parsed: true };
  addFormatted(stem, rule, entry);
  addNext(stem, rule, entry);
  addExpand(stem, rule, entry);
  addBetween(stem, rule, entry);
  return sortedObject(entry);
}

/**
 * Evaluate one `<stem>.ics` recurrence-set fixture: the calendar's
 * first component goes through `ruleSetFromComponent`, then either
 * `.expect.json` pins a rejection or `.occurrences.json` pins the
 * bounded expansion.
 */
function setEntry(path: string): RRuleEntry {
  const stem = path.slice(0, path.length - EXT_ICS.length);
  const input = readBytes(path);

  let set: rrule.RuleSet | undefined;
  let setErr: unknown;
  try {
    const cal = rfc5545.parse(input);
    const first = cal.components[0];
    if (first === undefined) {
      throw new VstarError(CLASS_MALFORMED, "calendar has no components");
    }
    set = rrule.ruleSetFromComponent(first);
  } catch (err) {
    setErr = err;
  }

  if (exists(`${stem}.expect.json`)) {
    return { error: failureToken(path, setErr, set !== undefined) };
  }
  if (set === undefined) fail(`${path}: ${String(setErr)}`);

  const entry: RRuleEntry = { parsed: true };
  const occPath = `${stem}.occurrences.json`;
  const spec = readSidecar(occPath);
  if (spec === undefined) return entry;
  addBounded(entry, occPath, "occurrences", () => set.occurrences(spec.limit ?? 0));
  return sortedObject(entry);
}

/**
 * Walk the rrule corpus and emit one object per fixture stem, keyed by
 * the stem's path relative to the conformance root.
 */
function emitRRule(root: string): Record<string, RRuleEntry> {
  const corpus = join(root, "..");
  const out: Record<string, RRuleEntry> = {};
  for (const path of walkFiles(root)) {
    if (path.endsWith(EXT_RRULE)) {
      record(out, corpus, path.slice(0, path.length - EXT_RRULE.length), ruleEntry(path));
    } else if (path.endsWith(EXT_ICS)) {
      record(out, corpus, path.slice(0, path.length - EXT_ICS.length), setEntry(path));
    }
  }
  return sortedObject(out);
}

// --- validate -------------------------------------------------------

/**
 * One diagnostic's stable surface. The message is deliberately absent:
 * it is prose that rewords between versions without the behavior
 * changing.
 */
interface DiagnosticEntry {
  readonly code: string;
  readonly severity: string;
  readonly path: string;
}

/**
 * Run `validate` over every `<name>.ics` and emit the diagnostics it
 * raises, sorted by (path, code).
 *
 * The sort is the contract, not any port's emission order: checks run
 * in an order that is an implementation detail. Sorting both sides
 * makes the comparison about which diagnostics were raised.
 */
function emitValidate(dir: string): Record<string, DiagnosticEntry[]> {
  const root = join(dir, "..");
  const out: Record<string, DiagnosticEntry[]> = {};
  eachFixture(dir, EXT_ICS, (path, stem) => {
    const entries: DiagnosticEntry[] = validate.validate(readCalendar(path)).map((d) => ({
      code: d.code,
      severity: d.severity,
      path: d.path,
    }));
    entries.sort((a, b) => compareStrings(a.path, b.path) || compareStrings(a.code, b.code));
    record(out, root, stem, entries);
  });
  return sortedObject(out);
}

// --- diff -----------------------------------------------------------

/** One property parameter, in the order the property carries it. */
interface ParamEntry {
  readonly name: string;
  readonly value: string;
}

/**
 * One property-level change. `before` is null on an add, `after` null
 * on a remove. Params travel apart from the value because a
 * parameter-only change is a `change` op whose values are equal and
 * whose parameter lists differ.
 */
interface OpEntry {
  readonly op: string;
  readonly property: string;
  readonly before: string | null;
  readonly after: string | null;
  readonly before_params: ParamEntry[];
  readonly after_params: ParamEntry[];
}

/**
 * The change set for one paired component. `uid` is lifted out of
 * `path` so the common case needs no path parsing, and is empty for a
 * component addressed positionally.
 */
interface ComponentDiffEntry {
  readonly uid: string;
  readonly path: string;
  readonly ops: OpEntry[];
  readonly subs: ComponentDiffEntry[];
}

/** Project a property's parameters onto the emitted shape. */
function paramEntries(params: readonly Param[]): ParamEntry[] {
  return params.map((p) => ({ name: p.name, value: p.value }));
}

/**
 * Lift the uid out of a rendered path such as
 * `VCALENDAR.VTODO[uid=todo-1]`. Empty for a positional path
 * (`VCALENDAR.VALARM[#0]`), which has no UID to key on.
 */
function uidFromPath(path: string): string {
  const marker = "[uid=";
  const i = path.lastIndexOf(marker);
  if (i < 0 || !path.endsWith("]")) return "";
  return path.slice(i + marker.length, path.length - 1);
}

/** Report the name the op is about, from whichever side carries it. */
function propertyName(pd: diff.PropertyDiff): string {
  return pd.property.name !== "" ? pd.property.name : (pd.old?.name ?? "");
}

function opEntries(pds: readonly diff.PropertyDiff[]): OpEntry[] {
  return pds.map((pd) => {
    const empty: ParamEntry[] = [];
    switch (pd.op) {
      case diff.OP_ADDED:
        return {
          op: diff.OP_ADDED,
          property: propertyName(pd),
          before: null,
          after: pd.property.value,
          before_params: empty,
          after_params: paramEntries(pd.property.params),
        };
      case diff.OP_REMOVED:
        return {
          op: diff.OP_REMOVED,
          property: propertyName(pd),
          before: pd.property.value,
          after: null,
          before_params: paramEntries(pd.property.params),
          after_params: empty,
        };
      case diff.OP_CHANGED:
        return {
          op: diff.OP_CHANGED,
          property: propertyName(pd),
          before: pd.old?.value ?? "",
          after: pd.property.value,
          before_params: paramEntries(pd.old?.params ?? []),
          after_params: paramEntries(pd.property.params),
        };
    }
  });
}

/**
 * Drop sub-diffs that record no change. `ofCalendar` filters these at
 * the top level but carries them nested; the document reports only
 * real changes.
 */
function componentDiffEntries(ds: readonly diff.ComponentDiff[]): ComponentDiffEntry[] {
  return ds.map((d) => ({
    uid: uidFromPath(d.path),
    path: d.path,
    ops: opEntries(d.properties),
    subs: componentDiffEntries(d.subDiffs.filter((s) => !diff.isEmpty(s))),
  }));
}

/**
 * Run `ofCalendar` over every `<name>.a.ics` / `<name>.b.ics` pair.
 *
 * No sorting happens here. `ofCalendar` emits components in pairing
 * order and properties sorted by name case-insensitively, and that
 * ordering is the contract a port reproduces — sorting it again would
 * hide an ordering divergence rather than catch it.
 */
function emitDiff(dir: string): Record<string, ComponentDiffEntry[]> {
  const root = join(dir, "..");
  const out: Record<string, ComponentDiffEntry[]> = {};
  eachFixture(dir, `.a${EXT_ICS}`, (path, stem) => {
    const a = readCalendar(path);
    const b = readCalendar(`${stem}.b${EXT_ICS}`);
    record(out, root, stem, componentDiffEntries(diff.ofCalendar(a, b)));
  });
  return sortedObject(out);
}

// --- supersession ---------------------------------------------------

/**
 * Project the effective status each supersession ledger imposes, keyed
 * by component UID.
 *
 * The inputs are the conformance corpus' supersession fixtures; the
 * behavior tree holds only the `<name>.effective.json` sidecars, so
 * the sidecar names which `.ics` to load. A UID absent from the map is
 * not superseded — supersession is a projection query, not a
 * validator.
 */
function emitSupersession(conformance: string, dir: string): Record<string, Record<string, string>> {
  const root = join(dir, "..");
  const inputs = join(conformance, "supersession");
  const out: Record<string, Record<string, string>> = {};
  eachFixture(dir, ".effective.json", (_path, stem) => {
    const name = basename(stem);
    const cal = readCalendar(join(inputs, `${name}${EXT_ICS}`));
    const effective: Record<string, string> = {};
    for (const c of cal.components) {
      const status = supersession.superseded(c, cal.components);
      if (status === undefined) continue;
      const u = componentUid(c);
      if (u === "") fail(`${name}: superseded component has no UID`);
      effective[u] = status;
    }
    record(out, root, stem, sortedObject(effective));
  });
  return sortedObject(out);
}

// --- duration -------------------------------------------------------

/**
 * The failure vocabulary alarm resolution reports, most specific
 * first. An implementation that also wraps `ErrMalformed` around one
 * of these must still report the specific one, which is what
 * first-match ordering guarantees.
 */
const TRIGGER_CLASSES = ["ErrNoTrigger", "ErrNoAnchor", CLASS_MALFORMED] as const;

/**
 * The vocabulary `duration.parse` failures report. Every failure wraps
 * `ErrMalformed` today; classifying rather than assuming means a class
 * the port grows surfaces as `Unclassified` instead of traveling as a
 * wrong token.
 */
const DURATION_CLASSES = [CLASS_MALFORMED] as const;

/**
 * One parsed duration value. `seconds` is the whole duration as a
 * signed second count, the one representation every target language
 * has. `negative` is kept separate because a zero-length duration
 * written with a leading sign cannot be distinguished by `seconds`
 * alone.
 */
interface DurationParseEntry {
  readonly value: string;
  readonly seconds: number | null;
  readonly negative: boolean | null;
  readonly error: string;
}

/** When one VALARM fires, or the class its resolution failed with. */
interface TriggerEntry {
  readonly alarm_uid: string;
  readonly fires_at: string;
  readonly error: string;
}

/**
 * Report what `duration.parse` makes of one value: the signed second
 * count and sign flag, or the failure class.
 *
 * `VDuration.signed()` is milliseconds, where the Go reference's
 * `Signed()` is nanoseconds; both divide down to the same second
 * count, which is the emitted unit.
 */
function parseDuration(value: string): DurationParseEntry {
  try {
    const d = duration.parse(value);
    return {
      value,
      seconds: Math.trunc(d.signed() / 1000),
      negative: d.isNegative(),
      error: "",
    };
  } catch (err) {
    return {
      value,
      seconds: null,
      negative: null,
      error: classifyOrUnclassified(err, DURATION_CLASSES),
    };
  }
}

/**
 * Resolve every VALARM in the calendar, in document order: parent
 * components in calendar order, VALARMs in the order they appear
 * inside their parent.
 */
function alarmEntries(cal: Calendar): TriggerEntry[] {
  const out: TriggerEntry[] = [];
  for (const parent of cal.components) {
    for (const alarm of parent.sub) {
      if (alarm.type !== "VALARM") continue;
      out.push(resolveAlarm(alarm, parent, cal));
    }
  }
  return out;
}

function resolveAlarm(alarm: Component, parent: Component, cal: Calendar): TriggerEntry {
  const alarmUid = componentUid(alarm);
  try {
    return { alarm_uid: alarmUid, fires_at: formatTime(helpers.alarmFiresAt(alarm, parent, cal)), error: "" };
  } catch (err) {
    return {
      alarm_uid: alarmUid,
      fires_at: "",
      error: classifyOrUnclassified(err, TRIGGER_CLASSES),
    };
  }
}

/**
 * Cover both duration families: the flat parse table at
 * `duration/parse.json`, and one alarm-resolution list per
 * `<name>.ics`.
 *
 * The parse table's inputs come from the committed fixture's `value`
 * column rather than a list hard-coded here, so a value added to the
 * corpus reaches every port without an emitter edit.
 */
function emitDuration(dir: string): Record<string, DurationParseEntry[] | TriggerEntry[]> {
  const root = join(dir, "..");
  const out: Record<string, DurationParseEntry[] | TriggerEntry[]> = {};

  const rows = readJson<{ value: string }[]>(join(dir, "parse.json"));
  out["duration/parse"] = rows.map((r) => parseDuration(r.value));

  eachFixture(dir, EXT_ICS, (path, stem) => {
    record(out, root, stem, alarmEntries(readCalendar(path)));
  });
  return sortedObject(out);
}

// --- ext ------------------------------------------------------------

/**
 * One extension name's spec/04 classification. `system` is the owning
 * system's slug, and null for every scope but `"system"` — only a
 * system-scoped name has an owner.
 */
interface ScopeEntry {
  readonly name: string;
  readonly scope: string;
  readonly system: string | null;
}

/**
 * Classify every name in the committed `ext/scopes.json`, in the order
 * the file lists them. The file is a flat table, not a set: order is
 * part of what a port reproduces.
 */
function emitExt(dir: string): Record<string, ScopeEntry[]> {
  const rows = readJson<{ name: string }[]>(join(dir, "scopes.json"));
  return {
    "ext/scopes": rows.map((r) => ({
      name: r.name,
      scope: ext.scopeOf(r.name),
      system: ext.systemName(r.name) ?? null,
    })),
  };
}

// --- time -----------------------------------------------------------

/**
 * One local timestamp resolved against a named zone. `utc` is null for
 * every rejection: the API reports an absent value, not an error, so
 * there is no failure class to name here.
 */
interface TzidEntry {
  readonly calendar: string;
  readonly tzid: string;
  readonly value: string;
  readonly utc: string | null;
}

/**
 * Resolve every (calendar, tzid, value) triple in the committed
 * `time/tzid.json`, in file order.
 *
 * `calendar` names a conformance fixture supplying the VTIMEZONE
 * registry. Each registry is parsed once and reused, so a fixture's
 * cost does not grow with the number of rows citing it.
 */
function emitTime(conformance: string, dir: string): Record<string, TzidEntry[]> {
  const rows = readJson<{ calendar: string; tzid: string; value: string }[]>(join(dir, "tzid.json"));

  const registries = new Map<string, Calendar>();
  for (const r of rows) {
    if (registries.has(r.calendar)) continue;
    registries.set(r.calendar, readCalendar(join(conformance, "time", `${r.calendar}${EXT_ICS}`)));
  }

  return {
    "time/tzid": rows.map((r) => {
      const cal = registries.get(r.calendar) as Calendar;
      const got = parseTimeWithTzid(r.value, r.tzid, cal);
      return {
        calendar: r.calendar,
        tzid: r.tzid,
        value: r.value,
        utc: got === undefined ? null : formatTime(got),
      };
    }),
  };
}

// --- main -----------------------------------------------------------

function main(): void {
  const args = process.argv.slice(2);
  if (args.length !== 1) fail("usage: parity <spec-dir>");
  const spec = resolve(args[0] as string);

  const conformance = join(spec, "v0.1", "conformance");
  const behavior = join(spec, "behavior");
  for (const dir of [conformance, behavior]) {
    if (!exists(dir) || !statSync(dir).isDirectory()) fail(`not a directory: ${dir}`);
  }

  // The eight top-level keys, in the order the contract lists them.
  // Their order in the emitted text does not matter to the harness,
  // which compares parsed documents, but keeping it matches the
  // reference's own struct order and makes the two files diffable by
  // eye.
  const document = {
    conformance: emitConformance(conformance),
    rrule: emitRRule(join(conformance, "rrule")),
    validate: emitValidate(join(behavior, "validate")),
    diff: emitDiff(join(behavior, "diff")),
    supersession: emitSupersession(conformance, join(behavior, "supersession")),
    duration: emitDuration(join(behavior, "duration")),
    ext: emitExt(join(behavior, "ext")),
    time: emitTime(conformance, join(behavior, "time")),
  };

  process.stdout.write(`${JSON.stringify(document, null, 2)}\n`);
}

main();
