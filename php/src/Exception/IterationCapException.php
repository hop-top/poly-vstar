<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * The evaluator reached its iteration bound without finding an
 * occurrence; the rule did not terminate.
 */
final class IterationCapException extends VstarException
{
    public function sentinel(): string
    {
        return 'ErrIterationCap';
    }
}
