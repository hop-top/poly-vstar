<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * The wire-string STATUS value of a VTODO, per RFC 5545 §3.8.1.11.
 *
 * TodoStatus, {@see EventStatus} and {@see JournalStatus} are deliberately
 * distinct types even though the cancellation value is spelled the same
 * in all three: the RFC scopes each vocabulary to one component type, so
 * separate types make a cross-type assignment a type error rather than a
 * wire-level conformance bug found in production.
 */
enum TodoStatus: string
{
    case NeedsAction = 'NEEDS-ACTION';
    case InProcess = 'IN-PROCESS';
    case Completed = 'COMPLETED';
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
