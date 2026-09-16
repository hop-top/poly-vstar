<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * Syntactically valid but outside the RRULE parsing scope:
 * FREQ=SECONDLY, RSCALE, or a non-UTC EXDATE / RDATE / RECURRENCE-ID
 * value.
 */
final class UnsupportedRRuleException extends VstarException
{
    public function sentinel(): string
    {
        return 'ErrUnsupportedRRule';
    }
}
