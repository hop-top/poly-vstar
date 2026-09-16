<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * The wire-string KIND value of a {@see Card}, per RFC 6350 §6.1.4.
 *
 * Values are lowercase, following the RFC's IANA registry. The RFC also
 * lists `location`; only the three values V* uses are modeled.
 */
enum Kind: string
{
    case Individual = 'individual';
    case Org = 'org';
    case Group = 'group';

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
