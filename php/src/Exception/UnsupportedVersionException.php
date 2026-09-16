<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * A VERSION property is present but names neither vCard 4.0 nor
 * iCalendar 2.0.
 */
final class UnsupportedVersionException extends VstarException
{
    public function sentinel(): string
    {
        return 'ErrUnsupportedVersion';
    }
}
