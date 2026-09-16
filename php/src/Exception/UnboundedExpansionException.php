<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * A request to expand into a list with no bound: a zero or inverted
 * window, or a negative limit.
 */
final class UnboundedExpansionException extends VstarException
{
    public function sentinel(): string
    {
        return 'ErrUnboundedExpansion';
    }
}
