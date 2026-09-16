// SPDX-License-Identifier: MIT

/**
 * The `helpers` subpath export: convenience constructors and
 * high-level mutators sitting above the bare property API.
 *
 * ## The hash-last discipline
 *
 * Every helper that mutates state refreshes `X-VSTAR-HASH` **last**, so
 * a caller always receives a component whose stored hash matches its own
 * canonical bytes. That is the invariant the whole layer exists to
 * preserve: a caller stitching the same mutations together out of the
 * bare property API has to remember the refresh, and forgetting it
 * produces a component that fails its own verification.
 *
 * ## Mutators are no-ops on a mismatch, not errors
 *
 * A setter whose property does not apply to the component's type
 * (`TRANSP` on a `VJOURNAL`, say) or whose value falls outside the RFC's
 * vocabulary or range leaves the component untouched, skips the hash
 * refresh, and reports nothing. Out-of-range input is *rejected*, never
 * clamped: silently rewriting a caller's `PERCENT-COMPLETE:120` to `100`
 * would assert the task is finished, and clamping a `PRIORITY:10` to `9`
 * would hide an off-by-one behind data that later round-trips cleanly.
 */

import type { Instant } from "../date.js";
import {
  RELATED_END,
  RELATED_START,
  type Related,
  Trigger,
  VDuration,
  alarmTrigger as durationAlarmTrigger,
  eventEnd as durationEventEnd,
} from "../duration/index.js";
import { VstarError } from "../errors.js";
import { setXVstar } from "../hashing/index.js";
import type {
  Calendar,
  Card,
  Component,
  CompType,
  EventStatus,
  JournalStatus,
  Kind,
  Property,
  RelType,
  TodoStatus,
  Transp,
  VClass,
} from "../types.js";
import {
  DEFAULT_REL_TYPE,
  add,
  cardSet,
  get,
  getAll,
  parseRelType,
  remove,
  set as setProp,
} from "../types.js";
import {
  due as readDue,
  formatTime,
  setCompleted,
  setDtend,
  setDtstart,
  setDue as setDueTime,
} from "../time.js";

export { version } from "../version.js";

/**
 * The `PRODID` {@link newCalendar} emits when the caller passes an
 * empty string. `PRODID` survives canonicalization and is hashed, so
 * the default carries neither a version nor a language: a document
 * built by any release of any port hashes the same. A caller wanting
 * its own identifier supplies one.
 */
const DEFAULT_PROD_ID = "-//hop-top//vstar//EN";

const PROP_UID = "UID";
const PROP_DTSTAMP = "DTSTAMP";
const PROP_ACTION = "ACTION";
const PROP_TRIGGER = "TRIGGER";
const PROP_CATEGORIES = "CATEGORIES";
const PROP_RELATED_TO = "RELATED-TO";
const PARAM_RELTYPE = "RELTYPE";
const PROP_CLASS = "CLASS";
const PROP_TRANSP = "TRANSP";
const PROP_SEQUENCE = "SEQUENCE";
const PROP_PRIORITY = "PRIORITY";
const PROP_PERCENT_COMPLETE = "PERCENT-COMPLETE";
const PROP_STATUS = "STATUS";

/** The RFC 5545 §3.8.1.9 `PRIORITY` bounds. `0` is a legal value meaning "undefined". */
const PRIORITY_MIN = 0;
const PRIORITY_MAX = 9;

/** The RFC 5545 §3.8.1.8 `PERCENT-COMPLETE` bounds. */
const PERCENT_MIN = 0;
const PERCENT_MAX = 100;

// ---------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------

/**
 * A fresh `VTODO` with `UID`, `DTSTAMP` set to now, `DUE` set to `due`,
 * and `X-VSTAR-HASH` computed last.
 *
 * @throws {VstarError} with code `ErrMissingUID` when `uid` is empty.
 */
export function newTodo(uid: string, due: Instant): Component {
  requireUid(uid, "newTodo");
  const c = stampUid({ type: "VTODO", props: [], sub: [] }, uid);
  setDueTime(c, due);
  setXVstar(c);
  return c;
}

/**
 * A fresh `VJOURNAL` with `UID`, `DTSTAMP` set to now, `DTSTART`, and
 * `X-VSTAR-HASH` computed last.
 *
 * @throws {VstarError} with code `ErrMissingUID` when `uid` is empty.
 */
export function newJournal(uid: string, dtstart: Instant): Component {
  requireUid(uid, "newJournal");
  const c = stampUid({ type: "VJOURNAL", props: [], sub: [] }, uid);
  setDtstart(c, dtstart);
  setXVstar(c);
  return c;
}

/**
 * A fresh `VEVENT` with `UID`, `DTSTAMP` set to now, `DTSTART`, `DTEND`,
 * and `X-VSTAR-HASH` computed last.
 *
 * @throws {VstarError} with code `ErrMissingUID` when `uid` is empty.
 */
export function newEvent(uid: string, dtstart: Instant, dtend: Instant): Component {
  requireUid(uid, "newEvent");
  const c = stampUid({ type: "VEVENT", props: [], sub: [] }, uid);
  setDtstart(c, dtstart);
  setDtend(c, dtend);
  setXVstar(c);
  return c;
}

/**
 * A fresh `VFREEBUSY` with `UID`, `DTSTAMP` set to now, `DTSTART`,
 * `DTEND`, and `X-VSTAR-HASH` computed last.
 *
 * @throws {VstarError} with code `ErrMissingUID` when `uid` is empty.
 */
export function newFreeBusy(uid: string, dtstart: Instant, dtend: Instant): Component {
  requireUid(uid, "newFreeBusy");
  const c = stampUid({ type: "VFREEBUSY", props: [], sub: [] }, uid);
  setDtstart(c, dtstart);
  setDtend(c, dtend);
  setXVstar(c);
  return c;
}

/**
 * A fresh `VALARM` with `UID`, `DTSTAMP` set to now, `ACTION`, a raw
 * `TRIGGER` wire string, and `X-VSTAR-HASH` computed last.
 *
 * `VALARM` is the one component type RFC 5545 does not require a `UID`
 * on, but V* requires one on every persisted component (spec/02), so an
 * empty `uid` is refused here too.
 *
 * The trigger is an unvalidated wire string. {@link newRelativeAlarm}
 * and {@link newAbsoluteAlarm} take typed values that cannot encode a
 * malformed duration; prefer those unless the string is already known
 * good.
 *
 * @throws {VstarError} with code `ErrMissingUID` when `uid` is empty.
 */
export function newAlarm(uid: string, action: string, trigger: string): Component {
  requireUid(uid, "newAlarm");
  const c = stampUid({ type: "VALARM", props: [], sub: [] }, uid);
  setProp(c, { name: PROP_ACTION, params: [], value: action });
  setProp(c, { name: PROP_TRIGGER, params: [], value: trigger });
  setXVstar(c);
  return c;
}

/**
 * A fresh `VCALENDAR` carrying `prodId`, or the package default when
 * `prodId` is empty.
 *
 * This cannot fail, and so, unlike the component constructors, it
 * throws nothing. `VERSION` and the component list are the encoder's
 * and the caller's business respectively.
 */
export function newCalendar(prodId: string): Calendar {
  return { prodId: prodId === "" ? DEFAULT_PROD_ID : prodId, components: [] };
}

/**
 * A fresh `VCARD` with `VERSION:4.0`, `KIND` and `UID` set. An empty
 * `kind` defaults to `individual`.
 *
 * vCards are not subject to the `X-VSTAR-HASH` discipline at the
 * constructor layer at v1.0: `hashing.card` exists for a caller wanting
 * a card-level digest, but a `Card` carries no `X-VSTAR-HASH` property
 * of its own and this constructor stamps none. Like
 * {@link newCalendar}, it cannot fail.
 */
export function newCard(uid: string, kind: Kind): Card {
  const resolved: Kind = kind === "" ? "individual" : kind;
  const card: Card = { uid, kind: resolved, props: [] };
  cardSet(card, { name: "VERSION", params: [], value: "4.0" });
  cardSet(card, { name: "KIND", params: [], value: resolved });
  cardSet(card, { name: PROP_UID, params: [], value: uid });
  return card;
}

// ---------------------------------------------------------------------
// Alarms
// ---------------------------------------------------------------------

/**
 * A fresh `VALARM` whose `TRIGGER` is a relative offset from one end of
 * its parent — the overwhelmingly common alarm shape.
 *
 * A negative offset fires *before* the anchor, so a fifteen-minute
 * warning is `new VDuration({ negative: true, minutes: 15 })` against
 * `RELATED_START`. `RELATED=START` is the RFC default and stays
 * implicit on the wire; `RELATED_END` emits the parameter.
 *
 * @throws {VstarError} with code `ErrMissingUID` when `uid` is empty.
 */
export function newRelativeAlarm(
  uid: string,
  action: string,
  offset: VDuration,
  related: Related,
): Component {
  requireUid(uid, "newRelativeAlarm");
  const trigger = new Trigger({ relative: true, duration: offset, related });
  return newAlarmWithTrigger(uid, action, trigger.toProperty());
}

/**
 * A fresh `VALARM` whose `TRIGGER` is a fixed instant rather than an
 * offset. The emitted property carries `VALUE=DATE-TIME` explicitly, so
 * a consumer never infers the form, and the instant is UTC form #2.
 *
 * @throws {VstarError} with code `ErrMissingUID` when `uid` is empty.
 */
export function newAbsoluteAlarm(uid: string, action: string, at: Instant): Component {
  requireUid(uid, "newAbsoluteAlarm");
  return newAlarmWithTrigger(uid, action, new Trigger({ absolute: at }).toProperty());
}

/**
 * The `TRIGGER` of `alarm`, decoded.
 *
 * @throws {VstarError} with code `ErrNoTrigger` when the `VALARM`
 * carries none — the property is mandatory per RFC 5545 §3.6.6, so the
 * alarm cannot be scheduled — or `ErrMalformed` when the value does not
 * parse.
 */
export function alarmTrigger(alarm: Component): Trigger {
  return durationAlarmTrigger(alarm);
}

/**
 * The instant at which `alarm` fires, given the `parent` it hangs off
 * and that parent's calendar.
 *
 * A relative trigger offsets from the anchor its `RELATED` parameter
 * selects — `DTSTART` for `START`; for `END` a `VTODO`'s `DUE`, else
 * the parent's `DTEND` or `DTSTART`+`DURATION`. An absolute trigger
 * returns its instant directly. `cal` supplies the `VTIMEZONE` registry
 * for a `TZID`-bearing anchor.
 *
 * @throws {VstarError} with code `ErrNoTrigger`, `ErrNoAnchor` or
 * `ErrMalformed`.
 */
export function alarmFiresAt(alarm: Component, parent: Component, cal: Calendar): Instant {
  return durationAlarmTrigger(alarm).resolve(parent, cal);
}

/**
 * The repeat cycle of `alarm` as its `DURATION` and `REPEAT` count.
 *
 * @throws {VstarError} with code `ErrMalformed` when either value is
 * present but unparseable.
 */
export { alarmRepeatCycle } from "../duration/index.js";

/**
 * The end instant of `c` — its `DTEND`, or `DTSTART` plus `DURATION`,
 * or a `VTODO`'s `DUE` — or `undefined` when it has no end anchor.
 */
export function eventEnd(c: Component, cal: Calendar): Instant | undefined {
  return durationEventEnd(c, cal);
}

// ---------------------------------------------------------------------
// Categories
// ---------------------------------------------------------------------

/**
 * The `CATEGORIES` values of `c`, split on commas.
 *
 * Whitespace adjacent to a comma is trimmed and empty tokens — from a
 * leading or trailing comma, or an `a,,b` run — are dropped. Returns an
 * empty array when `CATEGORIES` is absent or holds no non-empty token.
 */
export function categories(c: Component): string[] {
  const p = get(c, PROP_CATEGORIES);
  if (p === undefined) return [];
  return p.value
    .split(",")
    .map((t) => t.trim())
    .filter((t) => t !== "");
}

/**
 * Replace `CATEGORIES` with `values`, comma-joined with no space after
 * the comma — RFC 5545 §3.3.11 admits both forms, and the no-space
 * variant is the canonical wire form.
 *
 * Duplicates are dropped keeping first-seen order, and the comparison
 * is **case-sensitive**: `CATEGORIES` are user-facing labels per the
 * RFC, not registry tokens, so `Work` and `work` are distinct. Empty
 * input removes the property. Refreshes `X-VSTAR-HASH` last.
 */
export function setCategories(c: Component, values: readonly string[]): void {
  const deduped = dedupePreserve(values);
  if (deduped.length === 0) {
    remove(c, PROP_CATEGORIES);
    setXVstar(c);
    return;
  }
  setProp(c, { name: PROP_CATEGORIES, params: [], value: deduped.join(",") });
  setXVstar(c);
}

/**
 * Append one category to `CATEGORIES` when it is not already present,
 * comparing case-sensitively. An empty `value` changes nothing — and so
 * triggers no hash refresh either. Refreshes `X-VSTAR-HASH` last
 * otherwise.
 */
export function addCategory(c: Component, value: string): void {
  if (value === "") return;
  const current = categories(c);
  if (current.includes(value)) return;
  setProp(c, { name: PROP_CATEGORIES, params: [], value: [...current, value].join(",") });
  setXVstar(c);
}

// ---------------------------------------------------------------------
// Relations
// ---------------------------------------------------------------------

/**
 * One parsed `RELATED-TO` property: the referenced UID (the property
 * value) and its `RELTYPE`.
 */
export interface RelatedRef {
  readonly uid: string;
  readonly relType: RelType;
}

/**
 * Every `RELATED-TO` property on `c`, parsed, in the component's own
 * property order.
 *
 * `RELTYPE` is read case-insensitively per RFC 5545 §3.2 and folded to
 * its canonical spelling; an absent `RELTYPE` defaults to `PARENT` per
 * RFC 5545 §3.2.15. An unregistered value — an `X-`-prefixed extension,
 * say — passes through verbatim, since validating against the spec
 * vocabulary is the `validate` layer's job and this one accepts any
 * non-empty value.
 */
export function relatedTo(c: Component): RelatedRef[] {
  return getAll(c, PROP_RELATED_TO).map((p) => {
    let relType: RelType = DEFAULT_REL_TYPE;
    for (const par of p.params) {
      if (par.name.toUpperCase() === PARAM_RELTYPE && par.value !== "") {
        [relType] = parseRelType(par.value);
        break;
      }
    }
    return { uid: p.value, relType };
  });
}

/**
 * Append a `RELATED-TO` property naming `uid` with `RELTYPE=relType`.
 *
 * An empty `relType` omits the parameter, so a consumer reading through
 * {@link relatedTo} sees the RFC default of `PARENT`. An empty `uid`
 * changes nothing. Refreshes `X-VSTAR-HASH` last.
 *
 * `relType` is the typed {@link RelType}, deliberately — the type is
 * *open* (any string is valid, since IANA may register more values and
 * RFC 5545 permits `X-` extensions), but the constants name the
 * registered vocabulary so a caller reaches for `REL_CHILD` rather than
 * retyping the wire spelling.
 */
export function addRelatedTo(c: Component, uid: string, relType: RelType): void {
  if (uid === "") return;
  const prop: Property = { name: PROP_RELATED_TO, params: [], value: uid };
  if (relType !== "") {
    prop.params = [{ name: PARAM_RELTYPE, value: relType }];
  }
  add(c, prop);
  setXVstar(c);
}

// ---------------------------------------------------------------------
// Classification and transparency
// ---------------------------------------------------------------------

/**
 * The `CLASS` value of `c` when present and recognized, else
 * `undefined`.
 *
 * An absent `CLASS` reports `undefined` rather than `PUBLIC`, even
 * though RFC 5545 §3.8.1.3 assigns that by default. The getter reports
 * what is on the wire; the default is a separate, opt-in question that
 * {@link classOrDefault} answers. A caller that must distinguish
 * "explicitly PUBLIC" from "unset" — for round-trip fidelity, diffing
 * or supersession projection — cannot recover the distinction once a
 * getter has folded it away.
 *
 * The name is `classOf`, not `class`, because `class` is a reserved
 * word; every port spells it the same way.
 */
export function classOf(c: Component): VClass | undefined {
  const p = get(c, PROP_CLASS);
  if (p === undefined) return undefined;
  return isVClass(p.value) ? p.value : undefined;
}

/**
 * The effective `CLASS` of `c`, applying the RFC 5545 §3.8.1.3 default
 * of `PUBLIC` when the property is absent or holds an unrecognized
 * value.
 *
 * An unrecognized value falls back to the default rather than surfacing:
 * flagging it is the `validate` layer's job, not this accessor's.
 */
export function classOrDefault(c: Component): VClass {
  return classOf(c) ?? "PUBLIC";
}

/**
 * Write `CLASS` on `c` and refresh `X-VSTAR-HASH` last. A no-op when
 * `c.type` does not admit `CLASS` (`VEVENT`, `VTODO` and `VJOURNAL` do,
 * per RFC 5545 §3.8.1.3) or `v` is not one of the three valid values.
 */
export function setClass(c: Component, v: VClass): void {
  if (!classApplies(c.type) || !isVClass(v)) return;
  setProp(c, { name: PROP_CLASS, params: [], value: v });
  setXVstar(c);
}

/**
 * The `TRANSP` value of `c` when present and recognized, else
 * `undefined`. See {@link classOf} for why an absent property is not
 * reported as the RFC default; {@link transpOrDefault} gives the
 * effective value.
 */
export function transp(c: Component): Transp | undefined {
  const p = get(c, PROP_TRANSP);
  if (p === undefined) return undefined;
  return isTransp(p.value) ? p.value : undefined;
}

/**
 * The effective `TRANSP` of `c`, applying the RFC 5545 §3.8.2.7 default
 * of `OPAQUE` — the event consumes free/busy time — when the property
 * is absent or holds an unrecognized value.
 */
export function transpOrDefault(c: Component): Transp {
  return transp(c) ?? "OPAQUE";
}

/**
 * Write `TRANSP` on `c` and refresh `X-VSTAR-HASH` last. A no-op when
 * `c.type` is not `VEVENT` — RFC 5545 §3.8.2.7 scopes the property
 * there — or `v` is not one of the two valid values.
 */
export function setTransp(c: Component, v: Transp): void {
  if (c.type !== "VEVENT" || !isTransp(v)) return;
  setProp(c, { name: PROP_TRANSP, params: [], value: v });
  setXVstar(c);
}

// ---------------------------------------------------------------------
// Integer-valued properties
// ---------------------------------------------------------------------

/**
 * The `SEQUENCE` revision counter of `c`, or `undefined` when the
 * property is absent or holds something other than a canonical
 * non-negative integer.
 *
 * A caller wanting the RFC default reads absence as `0`: RFC 5545
 * §3.8.7.4 makes a component with no `SEQUENCE` revision zero. The
 * getter does not gate on `c.type` — a reader surfaces what the wire
 * holds, and type applicability is the mutators' concern.
 */
export function sequence(c: Component): number | undefined {
  const p = get(c, PROP_SEQUENCE);
  if (p === undefined) return undefined;
  return parseUint(p.value);
}

/**
 * Write `SEQUENCE` on `c` and refresh `X-VSTAR-HASH` last. A no-op when
 * `c.type` does not carry `SEQUENCE` (`VEVENT`, `VTODO`, `VJOURNAL`) or
 * `n` is negative or not an integer.
 */
export function setSequence(c: Component, n: number): void {
  if (!carriesSequence(c.type) || !Number.isInteger(n) || n < 0) return;
  setProp(c, { name: PROP_SEQUENCE, params: [], value: String(n) });
  setXVstar(c);
}

/**
 * Bump `SEQUENCE` by one and refresh `X-VSTAR-HASH` last.
 *
 * "Bump the revision" is the actual use for `SEQUENCE`, and doing it as
 * a read-then-write at the call site leaves a window a concurrent
 * mutator can interleave into; this closes it. An absent or unparseable
 * `SEQUENCE` counts as the RFC default of `0`, so the first increment
 * yields `1`. A no-op when `c.type` does not carry `SEQUENCE`.
 */
export function incrementSequence(c: Component): void {
  if (!carriesSequence(c.type)) return;
  setProp(c, { name: PROP_SEQUENCE, params: [], value: String((sequence(c) ?? 0) + 1) });
  setXVstar(c);
}

/**
 * The `PRIORITY` of `c`, or `undefined` when absent or outside the
 * RFC 5545 §3.8.1.9 range of 0–9.
 *
 * The optional keeps "undefined priority" distinct from "not present":
 * an explicit `PRIORITY:0` — the RFC's own "undefined" marker — returns
 * `0`, while a component with no `PRIORITY` at all returns `undefined`.
 * {@link removePriority} moves from the former to the latter.
 */
export function priority(c: Component): number | undefined {
  return boundedInt(c, PROP_PRIORITY, PRIORITY_MIN, PRIORITY_MAX);
}

/**
 * Write `PRIORITY` on `c` and refresh `X-VSTAR-HASH` last. A no-op when
 * `c.type` does not carry `PRIORITY` (`VEVENT` and `VTODO` do) or `n`
 * falls outside 0–9.
 *
 * Passing `0` is not rejection — it writes `PRIORITY:0`, the RFC's
 * explicit "undefined" marker.
 */
export function setPriority(c: Component, n: number): void {
  if (!carriesPriority(c.type) || !inRange(n, PRIORITY_MIN, PRIORITY_MAX)) return;
  setProp(c, { name: PROP_PRIORITY, params: [], value: String(n) });
  setXVstar(c);
}

/**
 * Delete `PRIORITY` from `c` and refresh `X-VSTAR-HASH` last — the
 * counterpart to `setPriority(c, 0)`: removal means "no priority
 * stated", `0` means "priority explicitly undefined".
 *
 * Unlike {@link setPriority} this does not gate on `c.type`: removing a
 * property that should not be there is always safe.
 */
export function removePriority(c: Component): void {
  remove(c, PROP_PRIORITY);
  setXVstar(c);
}

/**
 * The `PERCENT-COMPLETE` of `c`, or `undefined` when absent or outside
 * the RFC 5545 §3.8.1.8 range of 0–100. As with {@link priority}, an
 * explicit `0` — started, nothing done — is distinct from absence.
 */
export function percentComplete(c: Component): number | undefined {
  return boundedInt(c, PROP_PERCENT_COMPLETE, PERCENT_MIN, PERCENT_MAX);
}

/**
 * Write `PERCENT-COMPLETE` on `c` and refresh `X-VSTAR-HASH` last. A
 * no-op when `c.type` is not `VTODO` — RFC 5545 §3.8.1.8 scopes the
 * property there — or `n` falls outside 0–100.
 *
 * Setting `100` does not by itself mark a `VTODO` done; {@link complete}
 * does, writing `STATUS` and `COMPLETED` alongside.
 */
export function setPercentComplete(c: Component, n: number): void {
  if (c.type !== "VTODO" || !inRange(n, PERCENT_MIN, PERCENT_MAX)) return;
  setProp(c, { name: PROP_PERCENT_COMPLETE, params: [], value: String(n) });
  setXVstar(c);
}

/**
 * Delete `PERCENT-COMPLETE` from `c` and refresh `X-VSTAR-HASH` last.
 * Does not gate on `c.type`; removal is always safe.
 */
export function removePercentComplete(c: Component): void {
  remove(c, PROP_PERCENT_COMPLETE);
  setXVstar(c);
}

// ---------------------------------------------------------------------
// Status
// ---------------------------------------------------------------------

/**
 * The `STATUS` of `c` read against the **VTODO** vocabulary, or
 * `undefined` when absent or holding a value outside it.
 *
 * The name carries the `todo` qualifier — Go spells this pair bare
 * because VTODO is the historical default — so all three status pairs
 * read symmetrically and no caller reaches for a `status` expecting it
 * to work on a `VEVENT`.
 *
 * Like its two siblings, the getter does not gate on `c.type`: it
 * answers "does this component carry a VTODO-shaped STATUS". A caller
 * wanting the type check too tests `c.type` itself.
 */
export function todoStatus(c: Component): TodoStatus | undefined {
  const p = get(c, PROP_STATUS);
  if (p === undefined) return undefined;
  return isTodoStatus(p.value) ? p.value : undefined;
}

/**
 * Write `STATUS` on `c` and refresh `X-VSTAR-HASH` last. A no-op when
 * `c.type` is not `VTODO` or `s` is outside the VTODO vocabulary.
 *
 * The type guard is what keeps the three `STATUS` vocabularies from
 * bleeding into each other.
 */
export function setTodoStatus(c: Component, s: TodoStatus): void {
  if (c.type !== "VTODO" || !isTodoStatus(s)) return;
  setProp(c, { name: PROP_STATUS, params: [], value: s });
  setXVstar(c);
}

/** The `STATUS` of `c` read against the **VEVENT** vocabulary. */
export function eventStatus(c: Component): EventStatus | undefined {
  const p = get(c, PROP_STATUS);
  if (p === undefined) return undefined;
  return isEventStatus(p.value) ? p.value : undefined;
}

/**
 * Write `STATUS` on `c` and refresh `X-VSTAR-HASH` last. A no-op when
 * `c.type` is not `VEVENT` or `s` is outside the VEVENT vocabulary.
 */
export function setEventStatus(c: Component, s: EventStatus): void {
  if (c.type !== "VEVENT" || !isEventStatus(s)) return;
  setProp(c, { name: PROP_STATUS, params: [], value: s });
  setXVstar(c);
}

/** The `STATUS` of `c` read against the **VJOURNAL** vocabulary. */
export function journalStatus(c: Component): JournalStatus | undefined {
  const p = get(c, PROP_STATUS);
  if (p === undefined) return undefined;
  return isJournalStatus(p.value) ? p.value : undefined;
}

/**
 * Write `STATUS` on `c` and refresh `X-VSTAR-HASH` last. A no-op when
 * `c.type` is not `VJOURNAL` or `s` is outside the VJOURNAL vocabulary.
 */
export function setJournalStatus(c: Component, s: JournalStatus): void {
  if (c.type !== "VJOURNAL" || !isJournalStatus(s)) return;
  setProp(c, { name: PROP_STATUS, params: [], value: s });
  setXVstar(c);
}

/**
 * Finalize a `VTODO` atomically: `STATUS:COMPLETED`, `COMPLETED` set to
 * `t`, `PERCENT-COMPLETE:100`, and `X-VSTAR-HASH` refreshed last. A
 * no-op when `c.type` is not `VTODO`.
 *
 * One call yields the full set of "done" markers with a stored hash
 * that matches — which is the point, since stitching the three
 * mutations together by hand invites forgetting one.
 */
export function complete(c: Component, t: Instant): void {
  if (c.type !== "VTODO") return;
  setProp(c, { name: PROP_STATUS, params: [], value: "COMPLETED" });
  setCompleted(c, t);
  setProp(c, { name: PROP_PERCENT_COMPLETE, params: [], value: String(PERCENT_MAX) });
  setXVstar(c);
}

// ---------------------------------------------------------------------
// Due
// ---------------------------------------------------------------------

/**
 * The `DUE` instant of `c`, resolved against `cal`'s `VTIMEZONE`
 * registry, or `undefined` when absent or unparseable.
 */
export function due(c: Component, cal: Calendar): Instant | undefined {
  return readDue(c, cal);
}

/** Write `DUE` in UTC form #2 and refresh `X-VSTAR-HASH` last. */
export function setDue(c: Component, t: Instant): void {
  setDueTime(c, t);
  setXVstar(c);
}

// ---------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------

/** Refuse an empty UID, the one argument every component constructor validates. */
function requireUid(uid: string, fn: string): void {
  if (uid === "") {
    throw new VstarError("ErrMissingUID", `helpers.${fn}: uid is empty`);
  }
}

/** Seed `c` with `UID` and a fresh `DTSTAMP` of now, in UTC. */
function stampUid(c: Component, uid: string): Component {
  setProp(c, { name: PROP_UID, params: [], value: uid });
  setProp(c, { name: PROP_DTSTAMP, params: [], value: formatTime(Date.now()) });
  return c;
}

/**
 * Assemble a `VALARM` around an already-rendered `TRIGGER`, applying
 * the shared UID/DTSTAMP/ACTION seeding and the hash-last discipline.
 */
function newAlarmWithTrigger(uid: string, action: string, trigger: Property): Component {
  const c = stampUid({ type: "VALARM", props: [], sub: [] }, uid);
  setProp(c, { name: PROP_ACTION, params: [], value: action });
  setProp(c, trigger);
  setXVstar(c);
  return c;
}

/**
 * `values` with empty entries dropped and duplicates removed, keeping
 * first-seen order. Entries are trimmed before both the comparison and
 * the emit.
 */
function dedupePreserve(values: readonly string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of values) {
    const v = raw.trim();
    if (v === "" || seen.has(v)) continue;
    seen.add(v);
    out.push(v);
  }
  return out;
}

/**
 * Read `s` as a base-10 non-negative integer with no sign, padding or
 * surrounding whitespace.
 *
 * The RFC 5545 integer value type permits none of that decoration, so a
 * sloppy wire value is treated as unparseable rather than coerced:
 * `"+3"`, `"03"` and `" 3"` all report `undefined` rather than `3`.
 */
function parseUint(s: string): number | undefined {
  if (!/^\d+$/.test(s)) return undefined;
  const n = Number(s);
  if (!Number.isSafeInteger(n)) return undefined;
  // Reject a padded spelling whose canonical rendering differs.
  return String(n) === s ? n : undefined;
}

/** The value of a bounded integer property, or `undefined` when out of range. */
function boundedInt(c: Component, name: string, min: number, max: number): number | undefined {
  const p = get(c, name);
  if (p === undefined) return undefined;
  const n = parseUint(p.value);
  if (n === undefined || !inRange(n, min, max)) return undefined;
  return n;
}

/** Whether `n` is an integer within `[min, max]`. */
function inRange(n: number, min: number, max: number): boolean {
  return Number.isInteger(n) && n >= min && n <= max;
}

/** Whether `t` admits a `SEQUENCE` property (RFC 5545 §3.8.7.4). */
function carriesSequence(t: CompType): boolean {
  return t === "VEVENT" || t === "VTODO" || t === "VJOURNAL";
}

/** Whether `t` admits a `PRIORITY` property (RFC 5545 §3.8.1.9). */
function carriesPriority(t: CompType): boolean {
  return t === "VEVENT" || t === "VTODO";
}

/** Whether `t` admits a `CLASS` property (RFC 5545 §3.8.1.3). */
function classApplies(t: CompType): boolean {
  return t === "VEVENT" || t === "VTODO" || t === "VJOURNAL";
}

function isVClass(v: string): v is VClass {
  return v === "PUBLIC" || v === "PRIVATE" || v === "CONFIDENTIAL";
}

function isTransp(v: string): v is Transp {
  return v === "OPAQUE" || v === "TRANSPARENT";
}

function isTodoStatus(v: string): v is TodoStatus {
  return v === "NEEDS-ACTION" || v === "IN-PROCESS" || v === "COMPLETED" || v === "CANCELLED";
}

function isEventStatus(v: string): v is EventStatus {
  return v === "TENTATIVE" || v === "CONFIRMED" || v === "CANCELLED";
}

function isJournalStatus(v: string): v is JournalStatus {
  return v === "DRAFT" || v === "FINAL" || v === "CANCELLED";
}

// Re-exported so a caller building a relative alarm need not reach into
// `duration/` for the offset type and its anchor constants.
export { RELATED_END, RELATED_START, VDuration };
export type { Related };
