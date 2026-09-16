<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Helpers;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Card;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Duration\Duration;
use HopTop\Vstar\Duration\Related;
use HopTop\Vstar\Duration\Trigger;
use HopTop\Vstar\Duration\VDuration;
use HopTop\Vstar\EventStatus;
use HopTop\Vstar\Exception\MissingUidException;
use HopTop\Vstar\Hashing\Hashing;
use HopTop\Vstar\JournalStatus;
use HopTop\Vstar\Kind;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;
use HopTop\Vstar\RelType;
use HopTop\Vstar\Time;
use HopTop\Vstar\TodoStatus;
use HopTop\Vstar\Transp;
use HopTop\Vstar\VClass;
use HopTop\Vstar\Vstar;

/**
 * Convenience constructors and high-level mutators sitting above the bare
 * property API.
 *
 * # Hash discipline
 *
 * Every method here that writes state refreshes `X-VSTAR-HASH` **last**,
 * so a caller never observes a component whose stored hash lags its
 * content. That ordering is the whole discipline: the hash must cover
 * every property the call set, which it can only do if it is computed
 * after all of them.
 *
 * Note that canonicalization strips `X-VSTAR-HASH` (spec/03 rule 7), so
 * an emitter-parity test comparing canonical bytes is structurally blind
 * to a constructor that forgot to stamp one. The discipline is asserted
 * directly instead.
 *
 * # No-op on mismatch
 *
 * The typed mutators are no-ops rather than errors when the component
 * type does not admit the property, or when the value falls outside the
 * RFC's range. Out-of-range input is **rejected, not clamped**: clamping
 * a priority of 10 to 9 would turn a caller's off-by-one into a
 * legitimate-looking value that round-trips cleanly forever after.
 * Rejection also leaves any existing good value untouched and skips the
 * hash refresh, since nothing changed.
 */
final class Helpers
{
    /**
     * The PRODID emitted when a caller passes an empty string to
     * {@see self::newCalendar()}.
     */
    private const DEFAULT_PROD_ID = '-//hop-top//vstar-php v0.1.0//EN';

    private const PROP_UID = 'UID';
    private const PROP_DTSTAMP = 'DTSTAMP';
    private const PROP_ACTION = 'ACTION';
    private const PROP_CATEGORIES = 'CATEGORIES';
    private const PROP_RELATED_TO = 'RELATED-TO';
    private const PARAM_RELTYPE = 'RELTYPE';
    private const PROP_STATUS = 'STATUS';
    private const PROP_SEQUENCE = 'SEQUENCE';
    private const PROP_PRIORITY = 'PRIORITY';
    private const PROP_PERCENT = 'PERCENT-COMPLETE';
    private const PROP_CLASS = 'CLASS';
    private const PROP_TRANSP = 'TRANSP';

    private const PRIORITY_MIN = 0;
    private const PRIORITY_MAX = 9;
    private const PERCENT_MIN = 0;
    private const PERCENT_MAX = 100;

    // ---------------------------------------------------------------
    // Constructors
    // ---------------------------------------------------------------

    /**
     * A fresh VTODO with UID, `DTSTAMP` of now in UTC, and `DUE`.
     *
     * @throws MissingUidException `$uid` is empty
     */
    public static function newTodo(string $uid, \DateTimeInterface $due): Component
    {
        $c = self::stampUid(CompType::Todo, $uid, 'newTodo');
        Time::setDue($c, $due);
        Hashing::setXVstar($c);

        return $c;
    }

    /**
     * A fresh VJOURNAL with UID, `DTSTAMP` of now in UTC, and `DTSTART`.
     *
     * @throws MissingUidException `$uid` is empty
     */
    public static function newJournal(string $uid, \DateTimeInterface $dtstart): Component
    {
        $c = self::stampUid(CompType::Journal, $uid, 'newJournal');
        Time::setDtstart($c, $dtstart);
        Hashing::setXVstar($c);

        return $c;
    }

    /**
     * A fresh VEVENT with UID, `DTSTAMP` of now in UTC, `DTSTART` and
     * `DTEND`.
     *
     * @throws MissingUidException `$uid` is empty
     */
    public static function newEvent(
        string $uid,
        \DateTimeInterface $dtstart,
        \DateTimeInterface $dtend,
    ): Component {
        $c = self::stampUid(CompType::Event, $uid, 'newEvent');
        Time::setDtstart($c, $dtstart);
        Time::setDtend($c, $dtend);
        Hashing::setXVstar($c);

        return $c;
    }

    /**
     * A fresh VFREEBUSY with UID, `DTSTAMP` of now in UTC, `DTSTART` and
     * `DTEND`.
     *
     * @throws MissingUidException `$uid` is empty
     */
    public static function newFreeBusy(
        string $uid,
        \DateTimeInterface $dtstart,
        \DateTimeInterface $dtend,
    ): Component {
        $c = self::stampUid(CompType::FreeBusy, $uid, 'newFreeBusy');
        Time::setDtstart($c, $dtstart);
        Time::setDtend($c, $dtend);
        Hashing::setXVstar($c);

        return $c;
    }

    /**
     * A fresh VALARM with UID, `DTSTAMP` of now in UTC, `ACTION` and a
     * `TRIGGER` taken verbatim as a wire string.
     *
     * VALARM is the one component type RFC 5545 does not require a UID
     * on, but V* requires one on every persisted component (spec/02).
     *
     * For a trigger that cannot encode a malformed duration, prefer
     * {@see self::newRelativeAlarm()} or {@see self::newAbsoluteAlarm()},
     * which take typed values.
     *
     * @throws MissingUidException `$uid` is empty
     */
    public static function newAlarm(string $uid, string $action, string $trigger): Component
    {
        $c = self::stampUid(CompType::Alarm, $uid, 'newAlarm');
        $c->set(new Property(self::PROP_ACTION, [], $action));
        $c->set(new Property('TRIGGER', [], $trigger));
        Hashing::setXVstar($c);

        return $c;
    }

    /**
     * A fresh VCALENDAR carrying `$prodId`, or the package default when
     * that is empty. Cannot fail.
     */
    public static function newCalendar(string $prodId): Calendar
    {
        return new Calendar($prodId === '' ? self::DEFAULT_PROD_ID : $prodId);
    }

    /**
     * A fresh VCARD with `VERSION:4.0`, `KIND` and `UID` set. Cannot fail.
     *
     * A null kind defaults to {@see Kind::Individual} and a `KIND`
     * property is written from it. That default is documented behavior,
     * and worth knowing when rebuilding a fixture that carries no KIND
     * line -- `rfc6350/minimal.vcf` is one -- because the rebuild has to
     * clear both the model field and the derived property to match.
     *
     * vCards are not subject to the `X-VSTAR-HASH` discipline at the
     * constructor layer in v0.1: a Card has no such property of its own,
     * and {@see Hashing::card()} exists for callers wanting a digest.
     */
    public static function newCard(string $uid, ?Kind $kind): Card
    {
        $resolved = $kind ?? Kind::Individual;

        $card = new Card($uid, $resolved);
        $card->set(new Property('VERSION', [], '4.0'));
        $card->set(new Property('KIND', [], $resolved->value));
        $card->set(new Property(self::PROP_UID, [], $uid));

        return $card;
    }

    // ---------------------------------------------------------------
    // Alarms
    // ---------------------------------------------------------------

    /**
     * A fresh VALARM whose `TRIGGER` is an offset from one end of its
     * parent -- the overwhelmingly common alarm shape.
     *
     * A negative offset fires *before* the anchor, so a fifteen-minute
     * warning is `new VDuration(minutes: 15, negative: true)` against
     * {@see Related::Start}. `RELATED=START` is the RFC default and stays
     * implicit on the wire; {@see Related::End} emits `RELATED=END`.
     *
     * @throws MissingUidException `$uid` is empty
     */
    public static function newRelativeAlarm(
        string $uid,
        string $action,
        VDuration $offset,
        Related $related,
    ): Component {
        self::requireUid($uid, 'newRelativeAlarm');

        return self::alarmWithTrigger(
            $uid,
            $action,
            (new Trigger(relative: true, duration: $offset, related: $related))->toProperty(),
        );
    }

    /**
     * A fresh VALARM whose `TRIGGER` is a fixed instant rather than an
     * offset. The emitted property carries `VALUE=DATE-TIME` explicitly,
     * so a consumer never infers the form, and the instant is written in
     * UTC form #2.
     *
     * @throws MissingUidException `$uid` is empty
     */
    public static function newAbsoluteAlarm(
        string $uid,
        string $action,
        \DateTimeInterface $at,
    ): Component {
        self::requireUid($uid, 'newAbsoluteAlarm');

        $absolute = $at instanceof \DateTimeImmutable
            ? $at
            : \DateTimeImmutable::createFromInterface($at);

        return self::alarmWithTrigger(
            $uid,
            $action,
            (new Trigger(absolute: $absolute))->toProperty(),
        );
    }

    /**
     * The instant `$alarm` fires, given the component it hangs off and
     * that component's calendar.
     *
     * A relative trigger offsets from the anchor its `RELATED` parameter
     * selects -- `DTSTART` for START, and for END the parent's `DTEND`,
     * `DTSTART`+`DURATION`, or a VTODO's `DUE`. An absolute trigger
     * returns its instant directly.
     *
     * `$cal` supplies the VTIMEZONE registry for a TZID-bearing anchor;
     * pass a bare calendar when the parent's times are plain UTC.
     *
     * @throws \HopTop\Vstar\Exception\NoTriggerException the VALARM has no TRIGGER
     * @throws \HopTop\Vstar\Exception\NoAnchorException  the parent lacks the anchor
     * @throws \HopTop\Vstar\Exception\MalformedException the TRIGGER value does not parse
     */
    public static function alarmFiresAt(
        Component $alarm,
        Component $parent,
        Calendar $cal,
    ): \DateTimeImmutable {
        return Duration::alarmTrigger($alarm)->resolve($parent, $cal);
    }

    // ---------------------------------------------------------------
    // Categories
    // ---------------------------------------------------------------

    /**
     * The `CATEGORIES` values of `$c`, split on commas.
     *
     * Whitespace adjacent to a comma is trimmed and empty tokens -- from
     * a leading comma, or an `a,,b` run -- are dropped. An empty list
     * when the property is absent or holds nothing but separators.
     *
     * Readers preserve what the producer wrote; dedupe and comparison
     * semantics live in the writers.
     *
     * @return list<string>
     */
    public static function categories(Component $c): array
    {
        $p = $c->get(self::PROP_CATEGORIES);

        if ($p === null) {
            return [];
        }

        $out = [];

        foreach (explode(',', $p->value) as $token) {
            $token = trim($token);

            if ($token !== '') {
                $out[] = $token;
            }
        }

        return $out;
    }

    /**
     * Replace `CATEGORIES` with `$values`, comma-joined with no space
     * after the comma -- RFC 5545 §3.3.11 admits both forms and the
     * no-space variant is the canonical one.
     *
     * Values are trimmed, empties dropped, and duplicates removed keeping
     * first-seen order. Comparison is **case-sensitive**: the RFC makes
     * categories user-facing labels, not registry tokens, so `Work` and
     * `work` are distinct. Passing nothing removes the property.
     *
     * @param list<string> $values
     */
    public static function setCategories(Component $c, array $values): void
    {
        $deduped = self::dedupePreserve($values);

        if ($deduped === []) {
            $c->remove(self::PROP_CATEGORIES);
        } else {
            $c->set(new Property(self::PROP_CATEGORIES, [], implode(',', $deduped)));
        }

        Hashing::setXVstar($c);
    }

    /**
     * Append one category unless it is already present, compared
     * case-sensitively. An empty value changes nothing -- and so does not
     * churn the hash.
     */
    public static function addCategory(Component $c, string $value): void
    {
        if ($value === '') {
            return;
        }

        $current = self::categories($c);

        if (in_array($value, $current, true)) {
            return;
        }

        $current[] = $value;
        $c->set(new Property(self::PROP_CATEGORIES, [], implode(',', $current)));
        Hashing::setXVstar($c);
    }

    // ---------------------------------------------------------------
    // Relations
    // ---------------------------------------------------------------

    /**
     * Every `RELATED-TO` property on `$c`, parsed, in property order.
     *
     * `RELTYPE` is read case-insensitively per RFC 5545 §3.2 and folded to
     * its canonical spelling; an absent one defaults to `PARENT` per
     * §3.2.15. An unregistered value passes through verbatim, because the
     * vocabulary is open.
     *
     * @return list<RelatedRef>
     */
    public static function relatedTo(Component $c): array
    {
        $out = [];

        foreach ($c->getAll(self::PROP_RELATED_TO) as $p) {
            $relType = new RelType(RelType::DEFAULT);
            $raw = $p->param(self::PARAM_RELTYPE);

            if ($raw !== null && $raw !== '') {
                [$relType] = RelType::parse($raw);
            }

            $out[] = new RelatedRef($p->value, $relType);
        }

        return $out;
    }

    /**
     * Append a `RELATED-TO` naming `$uid`, carrying `RELTYPE=$relType`.
     *
     * An empty relType omits the parameter, leaving consumers to apply
     * the RFC default of `PARENT`. Any value is accepted, `X-` extensions
     * included. An empty `$uid` changes nothing.
     */
    public static function addRelatedTo(Component $c, string $uid, RelType $relType): void
    {
        if ($uid === '') {
            return;
        }

        $params = $relType->value === ''
            ? []
            : [new Param(self::PARAM_RELTYPE, $relType->value)];

        $c->add(new Property(self::PROP_RELATED_TO, $params, $uid));
        Hashing::setXVstar($c);
    }

    // ---------------------------------------------------------------
    // Due
    // ---------------------------------------------------------------

    /**
     * The `DUE` value of `$c` as an instant, resolved against `$cal`'s
     * VTIMEZONE registry. Null when absent or unresolvable.
     */
    public static function due(Component $c, Calendar $cal): ?\DateTimeImmutable
    {
        return $c->due($cal);
    }

    /**
     * Write `DUE` in UTC form #2, refreshing the hash last.
     */
    public static function setDue(Component $c, \DateTimeInterface $t): void
    {
        Time::setDue($c, $t);
        Hashing::setXVstar($c);
    }

    // ---------------------------------------------------------------
    // Status -- three distinct vocabularies
    // ---------------------------------------------------------------

    /**
     * The `STATUS` of `$c` read as a VTODO status, or null when absent or
     * carrying a value outside the VTODO vocabulary.
     *
     * The three status types are deliberately distinct: the RFC scopes
     * each vocabulary to one component type, so a VEVENT's `TENTATIVE`
     * reports null here rather than crossing over.
     *
     * The reference spells this pair `Status` / `SetStatus`, VTODO being
     * its historical default. Every port spells them `todoStatus` /
     * `setTodoStatus`, so all three pairs read symmetrically and no
     * caller reaches for a bare `status` expecting it to work on a
     * VEVENT.
     *
     * This does not gate on the component type -- a reader surfaces what
     * the wire holds. The writers enforce applicability.
     */
    public static function todoStatus(Component $c): ?TodoStatus
    {
        $p = $c->get(self::PROP_STATUS);

        return $p === null ? null : TodoStatus::tryFrom($p->value);
    }

    /**
     * Write a VTODO `STATUS`, refreshing the hash last. A no-op when `$c`
     * is not a VTODO.
     */
    public static function setTodoStatus(Component $c, TodoStatus $s): void
    {
        if ($c->type !== CompType::Todo->value) {
            return;
        }

        $c->set(new Property(self::PROP_STATUS, [], $s->value));
        Hashing::setXVstar($c);
    }

    /**
     * The `STATUS` of `$c` read as a VEVENT status, or null when absent
     * or outside the VEVENT vocabulary.
     */
    public static function eventStatus(Component $c): ?EventStatus
    {
        $p = $c->get(self::PROP_STATUS);

        return $p === null ? null : EventStatus::tryFrom($p->value);
    }

    /**
     * Write a VEVENT `STATUS`, refreshing the hash last. A no-op when
     * `$c` is not a VEVENT -- which is what keeps the three vocabularies
     * from bleeding into each other.
     */
    public static function setEventStatus(Component $c, EventStatus $s): void
    {
        if ($c->type !== CompType::Event->value) {
            return;
        }

        $c->set(new Property(self::PROP_STATUS, [], $s->value));
        Hashing::setXVstar($c);
    }

    /**
     * The `STATUS` of `$c` read as a VJOURNAL status, or null when absent
     * or outside the VJOURNAL vocabulary.
     */
    public static function journalStatus(Component $c): ?JournalStatus
    {
        $p = $c->get(self::PROP_STATUS);

        return $p === null ? null : JournalStatus::tryFrom($p->value);
    }

    /**
     * Write a VJOURNAL `STATUS`, refreshing the hash last. A no-op when
     * `$c` is not a VJOURNAL.
     */
    public static function setJournalStatus(Component $c, JournalStatus $s): void
    {
        if ($c->type !== CompType::Journal->value) {
            return;
        }

        $c->set(new Property(self::PROP_STATUS, [], $s->value));
        Hashing::setXVstar($c);
    }

    /**
     * Finalize a VTODO atomically: `STATUS:COMPLETED`, `COMPLETED` at
     * `$t`, `PERCENT-COMPLETE:100`, hash refreshed last.
     *
     * A no-op when `$c` is not a VTODO. One call yields the full set of
     * "done" markers with a matching stored hash, which is the point --
     * doing it property by property leaves windows in which the component
     * is half-finished.
     */
    public static function complete(Component $c, \DateTimeInterface $t): void
    {
        if ($c->type !== CompType::Todo->value) {
            return;
        }

        $c->set(new Property(self::PROP_STATUS, [], TodoStatus::Completed->value));
        Time::setCompleted($c, $t);
        $c->set(new Property(self::PROP_PERCENT, [], (string) self::PERCENT_MAX));
        Hashing::setXVstar($c);
    }

    // ---------------------------------------------------------------
    // Integer-valued properties
    // ---------------------------------------------------------------

    /**
     * The `SEQUENCE` revision counter, or null when absent or holding a
     * value that is not a canonical non-negative integer.
     *
     * A caller wanting the RFC default reads null as 0: RFC 5545 §3.8.7.4
     * makes a component with no SEQUENCE revision 0.
     */
    public static function sequence(Component $c): ?int
    {
        return self::readUint($c, self::PROP_SEQUENCE);
    }

    /**
     * Write `SEQUENCE`, refreshing the hash last. A no-op when `$c` does
     * not carry SEQUENCE (VEVENT, VTODO, VJOURNAL per §3.8.7.4) or `$n`
     * is negative.
     */
    public static function setSequence(Component $c, int $n): void
    {
        if (!self::carriesSequence($c) || $n < 0) {
            return;
        }

        $c->set(new Property(self::PROP_SEQUENCE, [], (string) $n));
        Hashing::setXVstar($c);
    }

    /**
     * Bump `SEQUENCE` by one, refreshing the hash last.
     *
     * "Bump the revision" is the actual use case, and doing it as a
     * read-then-write at the call site leaves a window for an interleaved
     * mutation. An absent or unparseable counter is the RFC default of 0,
     * so the first increment yields 1. A no-op when `$c` does not carry
     * SEQUENCE.
     */
    public static function incrementSequence(Component $c): void
    {
        if (!self::carriesSequence($c)) {
            return;
        }

        $c->set(new Property(
            self::PROP_SEQUENCE,
            [],
            (string) ((self::sequence($c) ?? 0) + 1),
        ));
        Hashing::setXVstar($c);
    }

    /**
     * The `PRIORITY` of `$c`, or null when absent or outside the
     * RFC 5545 §3.8.1.9 range 0-9.
     *
     * An explicit `PRIORITY:0` returns 0 -- the RFC's "undefined
     * priority" -- while an absent property returns null. The two are
     * different states, and {@see self::removePriority()} is how you move
     * from the former to the latter.
     */
    public static function priority(Component $c): ?int
    {
        return self::readBounded($c, self::PROP_PRIORITY, self::PRIORITY_MIN, self::PRIORITY_MAX);
    }

    /**
     * Write `PRIORITY`, refreshing the hash last. A no-op when `$c` does
     * not carry PRIORITY (VEVENT and VTODO per §3.8.1.9) or `$n` is
     * outside 0-9.
     *
     * Passing 0 is not rejection -- it writes the RFC's explicit
     * "undefined" marker.
     */
    public static function setPriority(Component $c, int $n): void
    {
        if (!self::carriesPriority($c) || $n < self::PRIORITY_MIN || $n > self::PRIORITY_MAX) {
            return;
        }

        $c->set(new Property(self::PROP_PRIORITY, [], (string) $n));
        Hashing::setXVstar($c);
    }

    /**
     * Delete `PRIORITY`, refreshing the hash last.
     *
     * The counterpart to `setPriority($c, 0)`: removal means "no priority
     * stated", whereas 0 means "priority explicitly undefined". Unlike
     * the setter this does not gate on type -- removing a property that
     * should not be there is always safe.
     */
    public static function removePriority(Component $c): void
    {
        $c->remove(self::PROP_PRIORITY);
        Hashing::setXVstar($c);
    }

    /**
     * The `PERCENT-COMPLETE` of `$c`, or null when absent or outside the
     * RFC 5545 §3.8.1.8 range 0-100. As with priority, an explicit 0 is
     * distinct from absence.
     */
    public static function percentComplete(Component $c): ?int
    {
        return self::readBounded($c, self::PROP_PERCENT, self::PERCENT_MIN, self::PERCENT_MAX);
    }

    /**
     * Write `PERCENT-COMPLETE`, refreshing the hash last. A no-op when
     * `$c` is not a VTODO or `$n` is outside 0-100.
     *
     * Setting 100 does not by itself mark a task done -- use
     * {@see self::complete()}, which also writes STATUS and COMPLETED.
     */
    public static function setPercentComplete(Component $c, int $n): void
    {
        if ($c->type !== CompType::Todo->value || $n < self::PERCENT_MIN || $n > self::PERCENT_MAX) {
            return;
        }

        $c->set(new Property(self::PROP_PERCENT, [], (string) $n));
        Hashing::setXVstar($c);
    }

    /**
     * Delete `PERCENT-COMPLETE`, refreshing the hash last. Does not gate
     * on type; removal is always safe.
     */
    public static function removePercentComplete(Component $c): void
    {
        $c->remove(self::PROP_PERCENT);
        Hashing::setXVstar($c);
    }

    // ---------------------------------------------------------------
    // Classification and transparency
    // ---------------------------------------------------------------

    /**
     * The `CLASS` of `$c`, or null when absent or unrecognized.
     *
     * An absent CLASS reports null rather than `PUBLIC`, even though
     * RFC 5545 §3.8.1.3 assigns that by default. The getter reports what
     * is on the wire; the default is a separate, opt-in question answered
     * by {@see self::classOrDefault()}. Two reasons: "present and
     * recognized" is what nullability means everywhere else in this
     * class, and a caller that must distinguish "explicitly PUBLIC" from
     * "unset" -- for round-trip fidelity, diffing, or supersession --
     * cannot recover the distinction once a getter has folded it away.
     *
     * The name is `classOf`, not `class`: `class` is reserved in PHP.
     */
    public static function classOf(Component $c): ?VClass
    {
        $p = $c->get(self::PROP_CLASS);

        return $p === null ? null : VClass::tryFrom($p->value);
    }

    /**
     * The effective `CLASS` of `$c`, applying the RFC 5545 §3.8.1.3
     * default of `PUBLIC` when the property is absent or unrecognized.
     *
     * An unrecognized value falls back rather than surfacing: flagging it
     * is the validate layer's job, not this accessor's.
     */
    public static function classOrDefault(Component $c): VClass
    {
        return self::classOf($c) ?? VClass::Public;
    }

    /**
     * Write `CLASS`, refreshing the hash last. A no-op when `$c` is not a
     * type that admits it -- VEVENT, VTODO or VJOURNAL per §3.8.1.3.
     */
    public static function setClass(Component $c, VClass $v): void
    {
        if (!in_array($c->type, [
            CompType::Event->value,
            CompType::Todo->value,
            CompType::Journal->value,
        ], true)) {
            return;
        }

        $c->set(new Property(self::PROP_CLASS, [], $v->value));
        Hashing::setXVstar($c);
    }

    /**
     * The `TRANSP` of `$c`, or null when absent or unrecognized. See
     * {@see self::classOf()} for why an absent property is not reported
     * as the RFC default.
     */
    public static function transp(Component $c): ?Transp
    {
        $p = $c->get(self::PROP_TRANSP);

        return $p === null ? null : Transp::tryFrom($p->value);
    }

    /**
     * The effective `TRANSP` of `$c`, applying the RFC 5545 §3.8.2.7
     * default of `OPAQUE` -- the event consumes free/busy time.
     */
    public static function transpOrDefault(Component $c): Transp
    {
        return self::transp($c) ?? Transp::Opaque;
    }

    /**
     * Write `TRANSP`, refreshing the hash last. A no-op when `$c` is not
     * a VEVENT: §3.8.2.7 scopes the property to events.
     */
    public static function setTransp(Component $c, Transp $v): void
    {
        if ($c->type !== CompType::Event->value) {
            return;
        }

        $c->set(new Property(self::PROP_TRANSP, [], $v->value));
        Hashing::setXVstar($c);
    }

    // ---------------------------------------------------------------
    // Internals
    // ---------------------------------------------------------------

    /**
     * A fresh component of `$type` seeded with UID and `DTSTAMP` of now.
     *
     * The hash is deliberately *not* stamped here: callers add their own
     * properties afterwards and refresh once, last, so the stored value
     * covers everything.
     *
     * @throws MissingUidException `$uid` is empty
     */
    private static function stampUid(CompType $type, string $uid, string $caller): Component
    {
        self::requireUid($uid, $caller);

        $c = new Component($type);
        $c->set(new Property(self::PROP_UID, [], $uid));
        $c->set(new Property(
            self::PROP_DTSTAMP,
            [],
            Vstar::formatTime(new \DateTimeImmutable('now', new \DateTimeZone('UTC'))),
        ));

        return $c;
    }

    /**
     * @throws MissingUidException `$uid` is empty
     */
    private static function requireUid(string $uid, string $caller): void
    {
        if ($uid === '') {
            throw new MissingUidException("helpers: {$caller} requires a non-empty UID");
        }
    }

    /**
     * Assemble a VALARM around an already-rendered TRIGGER, applying the
     * shared seeding and the hash-last discipline.
     */
    private static function alarmWithTrigger(
        string $uid,
        string $action,
        Property $trigger,
    ): Component {
        $c = self::stampUid(CompType::Alarm, $uid, 'newAlarm');
        $c->set(new Property(self::PROP_ACTION, [], $action));
        $c->set($trigger);
        Hashing::setXVstar($c);

        return $c;
    }

    /**
     * `$values` with empties dropped and duplicates removed, first-seen
     * order preserved. Each value is trimmed before both compare and
     * emit.
     *
     * @param list<string> $values
     *
     * @return list<string>
     */
    private static function dedupePreserve(array $values): array
    {
        $seen = [];
        $out = [];

        foreach ($values as $v) {
            $v = trim($v);

            if ($v === '' || isset($seen[$v])) {
                continue;
            }

            $seen[$v] = true;
            $out[] = $v;
        }

        return $out;
    }

    /**
     * The named property read as a canonical non-negative integer, or
     * null.
     *
     * "Canonical" rules out `+3`, `03` and ` 3`: RFC 5545 integer values
     * admit no padding or decoration, so a sloppy wire value is
     * unparseable rather than coerced.
     */
    private static function readUint(Component $c, string $name): ?int
    {
        $p = $c->get($name);

        if ($p === null) {
            return null;
        }

        if (preg_match('/^(0|[1-9][0-9]*)$/', $p->value) !== 1) {
            return null;
        }

        return (int) $p->value;
    }

    /**
     * {@see self::readUint()} additionally bounded to `$min`..`$max`.
     */
    private static function readBounded(Component $c, string $name, int $min, int $max): ?int
    {
        $n = self::readUint($c, $name);

        if ($n === null || $n < $min || $n > $max) {
            return null;
        }

        return $n;
    }

    private static function carriesSequence(Component $c): bool
    {
        return in_array($c->type, [
            CompType::Event->value,
            CompType::Todo->value,
            CompType::Journal->value,
        ], true);
    }

    private static function carriesPriority(Component $c): bool
    {
        return in_array($c->type, [
            CompType::Event->value,
            CompType::Todo->value,
        ], true);
    }
}
