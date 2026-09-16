<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Rrule;

/**
 * The RRULE `FREQ` value, restricted to the frequencies
 * `spec/v0.1/03-canonicalization.md` §RRULE parsing scope accepts.
 *
 * `SECONDLY` and `MINUTELY` are deliberately absent. They are
 * syntactically valid RFC 5545 values this scope defers, so they are
 * recognized by name at parse and reported `ErrUnsupportedRRule` --
 * modelling them as cases would let one reach the evaluator.
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
