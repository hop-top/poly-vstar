<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Rrule;

use HopTop\Vstar\Property;
use HopTop\Vstar\Time;

/**
 * A parsed RRULE: the structured form of an RFC 5545 §3.3.10 value,
 * plus the wire form `spec/v0.1/03-canonicalization.md` §RRULE wire form
 * fixes for emitters.
 *
 * Absent list rule-parts are empty arrays; an absent `UNTIL` is null and
 * an absent `COUNT` is `0`. `INTERVAL` defaults to 1 and `WKST` to `MO`,
 * both applied at parse.
 *
 * Every field is readonly. A rule is a value: mutating one after an
 * evaluator has read it would make the `complete` flag and the iteration
 * bound mean different things mid-expansion.
 */
final class Rule
{
    /**
     * The rule-part order every emitter uses, from spec §RRULE wire form.
     *
     * RFC 5545 §3.3.10 imposes no order -- a producer may emit rule-parts
     * any way it likes and the value means the same thing. The spec fixes
     * one so identical logical content produces byte-identical output,
     * and it is the order the RFC's own `recur` ABNF lists: FREQ and its
     * modifiers, the termination bound, then the BY-* filters from
     * coarsest to finest, BYSETPOS last because it applies last, and
     * WKST last of all because it modifies the whole rule rather than
     * filtering it.
     */
    private const BY_LIST_ORDER = [
        'BYMONTH',
        'BYWEEKNO',
        'BYYEARDAY',
        'BYMONTHDAY',
        'BYDAY',
        'BYHOUR',
        'BYMINUTE',
        'BYSECOND',
        'BYSETPOS',
    ];

    /**
     * @param list<ByDay> $byDay
     * @param list<int>   $byMonth
     * @param list<int>   $byMonthDay
     * @param list<int>   $byHour
     * @param list<int>   $byMinute
     * @param list<int>   $bySecond
     * @param list<int>   $byYearDay
     * @param list<int>   $byWeekNo
     * @param list<int>   $bySetPos
     */
    public function __construct(
        public readonly Freq $freq = Freq::Invalid,
        public readonly int $interval = 1,
        public readonly ?\DateTimeImmutable $until = null,
        public readonly int $count = 0,
        public readonly array $byDay = [],
        public readonly array $byMonth = [],
        public readonly array $byMonthDay = [],
        public readonly array $byHour = [],
        public readonly array $byMinute = [],
        public readonly array $bySecond = [],
        public readonly array $byYearDay = [],
        public readonly array $byWeekNo = [],
        public readonly array $bySetPos = [],
        public readonly Weekday $weekStart = Weekday::Mo,
    ) {
    }

    /**
     * Render the rule as an RFC 5545 §3.3.10 RRULE property value, with
     * no `RRULE:` prefix.
     *
     * Three properties are contract, not implementation detail:
     *
     * - **Fixed rule-part order**, per spec §RRULE wire form.
     * - **Defaults elided** -- `INTERVAL=1` and `WKST=MO` are omitted, so
     *   two rules differing only in whether the producer spelled out a
     *   default render identically.
     * - **List order preserved** -- `BYDAY=WE,MO` stays `BYDAY=WE,MO`.
     *   RFC 5545 gives BY-* lists no ordering semantics, so sorting them
     *   would rewrite the producer's content while looking tidier.
     *   Callers wanting order-insensitive equality compare parsed rules,
     *   not strings.
     *
     * Parsing the result and re-emitting it is idempotent.
     *
     * A rule with no `FREQ` renders as the empty string rather than a
     * partial value that would fail to re-parse.
     *
     * The API mapping spells this `__toString()`: `Rule` is a plain
     * class, not an enum, so the magic method is available here.
     */
    public function __toString(): string
    {
        if ($this->freq === Freq::Invalid) {
            return '';
        }

        $parts = ['FREQ=' . $this->freq->value];

        if ($this->interval > 1) {
            $parts[] = 'INTERVAL=' . $this->interval;
        }

        // UNTIL and COUNT are mutually exclusive; emit whichever is set.
        if ($this->until !== null) {
            $parts[] = 'UNTIL=' . Time::formatTime($this->until);
        }

        if ($this->count > 0) {
            $parts[] = 'COUNT=' . $this->count;
        }

        foreach (self::BY_LIST_ORDER as $name) {
            $rendered = $name === 'BYDAY'
                ? implode(',', array_map(static fn (ByDay $bd): string => $bd->toString(), $this->byDay))
                : implode(',', $this->listFor($name));

            if ($rendered !== '') {
                $parts[] = $name . '=' . $rendered;
            }
        }

        if ($this->weekStart !== Weekday::Mo) {
            $parts[] = 'WKST=' . $this->weekStart->value;
        }

        return implode(';', $parts);
    }

    /**
     * Render the rule as a complete RRULE {@see Property}, ready to
     * attach to a component.
     *
     * A rule that cannot produce a valid value yields the empty property
     * rather than a broken one.
     */
    public function toProperty(): Property
    {
        $value = (string) $this;

        return $value === '' ? new Property('') : new Property('RRULE', [], $value);
    }

    /**
     * The integer list behind one BY-* rule-part name.
     *
     * BYDAY is rendered by its own entry mapper and never reaches here.
     *
     * @return list<int>
     */
    private function listFor(string $name): array
    {
        return match ($name) {
            'BYMONTH' => $this->byMonth,
            'BYWEEKNO' => $this->byWeekNo,
            'BYYEARDAY' => $this->byYearDay,
            'BYMONTHDAY' => $this->byMonthDay,
            'BYHOUR' => $this->byHour,
            'BYMINUTE' => $this->byMinute,
            'BYSECOND' => $this->bySecond,
            'BYSETPOS' => $this->bySetPos,
            default => [],
        };
    }
}
