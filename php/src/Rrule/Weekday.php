<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Rrule;

/**
 * An RFC 5545 §3.3.10 weekday symbol.
 *
 * # The numbering is `SU = 0`
 *
 * RFC 5545 numbers weekdays `SU = 0` through `SA = 6`, and that
 * numbering is normative for every calculation in this namespace. It is
 * **not** ISO-8601's `MO = 1`, and it is **not** what
 * `DateTimeInterface::format('N')` reports -- but it *is* what
 * `format('w')` reports, which is why the arithmetic sites read `w` and
 * carry no conversion.
 *
 * A port that reuses a platform weekday number without converting has a
 * one-off bug the `rrule/by-clauses/byday` fixtures catch.
 * {@see self::toWeekday()} is the explicit conversion to the ISO
 * spelling and is the only place that conversion should live.
 */
enum Weekday: string
{
    case Su = 'SU';

    case Mo = 'MO';

    case Tu = 'TU';

    case We = 'WE';

    case Th = 'TH';

    case Fr = 'FR';

    case Sa = 'SA';

    /**
     * The RFC 5545 §3.3.10 number: `SU = 0` through `SA = 6`.
     *
     * This is the numbering the evaluator uses throughout, and it
     * matches `format('w')` exactly.
     */
    public function number(): int
    {
        return match ($this) {
            self::Su => 0,
            self::Mo => 1,
            self::Tu => 2,
            self::We => 3,
            self::Th => 4,
            self::Fr => 5,
            self::Sa => 6,
        };
    }

    /**
     * The ISO-8601 number: `MO = 1` through `SU = 7`.
     *
     * The reference's `ToTime` -- the explicit counterpart to
     * {@see self::number()}, exposed so a caller needing the ISO
     * spelling converts at one named boundary instead of reusing a
     * platform weekday number and inheriting the off-by-one.
     */
    public function toWeekday(): int
    {
        $n = $this->number();

        return $n === 0 ? 7 : $n;
    }

    /**
     * The weekday with the given RFC number, or null when `$n` is
     * outside 0..6.
     */
    public static function fromNumber(int $n): ?self
    {
        return match ($n) {
            0 => self::Su,
            1 => self::Mo,
            2 => self::Tu,
            3 => self::We,
            4 => self::Th,
            5 => self::Fr,
            6 => self::Sa,
            default => null,
        };
    }

    /**
     * The RFC wire spelling.
     *
     * A plain method rather than `__toString()`: PHP rejects the magic
     * method on an enum at declaration time. See {@see Freq::toString()}.
     */
    public function toString(): string
    {
        return $this->value;
    }
}
