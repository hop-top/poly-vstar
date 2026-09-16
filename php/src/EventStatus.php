<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * The wire-string STATUS value of a VEVENT, per RFC 5545 §3.8.1.11.
 *
 * See {@see TodoStatus} for why the shared cancellation spelling does not
 * collapse the three status vocabularies into one type.
 */
enum EventStatus: string
{
    case Tentative = 'TENTATIVE';
    case Confirmed = 'CONFIRMED';
    case Cancelled = 'CANCELLED';
}
