<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * A VALARM without a TRIGGER; the property is mandatory (RFC 5545
 * §3.6.6), so the alarm cannot be scheduled.
 */
final class NoTriggerException extends VstarException
{
    public function sentinel(): string
    {
        return 'ErrNoTrigger';
    }
}
