<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * A BEGIN line lacks its matching END before the end of input.
 */
final class UnclosedBlockException extends VstarException
{
    public function sentinel(): string
    {
        return 'ErrUnclosedBlock';
    }
}
