<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Rrule;

/**
 * The RRULE `FREQ` value, restricted to the frequencies
 * `spec/v1.0/03-canonicalization.md` §RRULE parsing scope accepts.
 *
 * `SECONDLY` is deliberately absent. It is a syntactically valid RFC
 * 5545 value this scope defers, so it is recognized by name at parse
 * and reported `ErrUnsupportedRRule` -- modelling it as a case would
 * let it reach the evaluator.
 *
 * The cases sit in RFC order, increasing period length.
 */
enum Freq: string
{
    /**
     * The unset marker, and the only case a successful parse never
     * produces: a missing `FREQ` is `ErrMalformed`. It exists so a
     * hand-built {@see Rule} literal has a defined initial state the
     * evaluator can refuse.
     */
    case Invalid = '';

    case Minutely = 'MINUTELY';

    case Hourly = 'HOURLY';

    case Daily = 'DAILY';

    case Weekly = 'WEEKLY';

    case Monthly = 'MONTHLY';

    case Yearly = 'YEARLY';

    /**
     * The RFC wire spelling.
     *
     * The API mapping's corrected form for enums: PHP rejects
     * `__toString()` on an enum at declaration time, so the reference's
     * `String()` lands as a plain method here while non-enum value
     * classes such as {@see Rule} keep `__toString()`.
     */
    public function toString(): string
    {
        return $this->value;
    }
}
