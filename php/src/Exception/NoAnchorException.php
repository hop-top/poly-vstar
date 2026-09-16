<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * A relative trigger resolved against a component lacking its anchor
 * — DTSTART for RELATED=START, or DTEND / DTSTART+DURATION / DUE for
 * RELATED=END.
 */
final class NoAnchorException extends VstarException
{
    public function sentinel(): string
    {
        return 'ErrNoAnchor';
    }
}
