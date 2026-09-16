<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Rrule;

/**
 * One entry in a `BYDAY` list: an optional ordinal plus a weekday.
 *
 * An ordinal of `0` means "every weekday of this kind in the containing
 * FREQ period" -- `BYDAY=MO` under `FREQ=MONTHLY` is every Monday of the
 * month. A non-zero ordinal in -53..-1 or 1..53 picks the nth, counting
 * from the start when positive and from the end when negative, and is
 * meaningful only inside a MONTHLY or YEARLY period.
 *
 * The explicit `0` prefix is invalid per RFC 5545 §3.3.10 and rejected
 * at parse: "every one of these" is spelled by omitting the ordinal, not
 * by writing zero.
 *
 * Readonly, like every value in this namespace: a parsed rule is a
 * record of what the wire said, and mutating one mid-expansion would
 * make the `complete` flag and the iteration bound mean different
 * things.
 */
final class ByDay
{
    public function __construct(
        public readonly int $ordinal,
        public readonly Weekday $weekday,
    ) {
    }

    /**
     * The wire form of this entry: the weekday alone when the ordinal is
     * `0`, else the signed ordinal followed by the weekday.
     */
    public function toString(): string
    {
        return $this->ordinal === 0
            ? $this->weekday->value
            : $this->ordinal . $this->weekday->value;
    }
}
