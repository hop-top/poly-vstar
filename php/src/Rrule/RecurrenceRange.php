<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Rrule;

/**
 * The RECURRENCE-ID `RANGE` parameter (RFC 5545 §3.2.13).
 *
 * The reference calls this `Range`; the API mapping renames it for
 * TypeScript, Python and PHP, where a bare `Range` reads as a numeric
 * interval.
 *
 * `ThisInstance` is the default and its backing value is the empty
 * string on purpose: the RFC expresses "this instance only" by omitting
 * the parameter entirely, so the default has no wire token of its own
 * and emitting one would change the bytes.
 */
enum RecurrenceRange: string
{
    /** The default: the override applies to this instance alone. */
    case ThisInstance = '';

    /** `RANGE=THISANDFUTURE`: this instance and every later one. */
    case ThisAndFuture = 'THISANDFUTURE';

    /**
     * The RFC wire spelling -- the empty string for the default, which
     * is how the absent parameter is expressed.
     *
     * A plain method rather than `__toString()`; see
     * {@see Freq::toString()}.
     */
    public function toString(): string
    {
        return $this->value;
    }
}
