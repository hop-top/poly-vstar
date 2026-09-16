<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * The wire-string STATUS value of a VJOURNAL, per RFC 5545 §3.8.1.11.
 *
 * See {@see TodoStatus} for why the shared cancellation spelling does not
 * collapse the three status vocabularies into one type.
 */
enum JournalStatus: string
{
    case Draft = 'DRAFT';
    case Final = 'FINAL';
    case Cancelled = 'CANCELLED';

    /**
     * The RFC wire spelling, as `->value`.
     *
     * The reference gives this type no `String()`; the method exists so
     * every enum in the package carries the same accessor. See
     * {@see Rrule\Freq::toString()} for why an enum spells it
     * `toString()`.
     */
    public function toString(): string
    {
        return $this->value;
    }
}
