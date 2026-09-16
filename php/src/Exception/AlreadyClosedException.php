<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * Close called twice, or encode called after close.
 */
final class AlreadyClosedException extends VstarException
{
    public function sentinel(): string
    {
        return 'ErrAlreadyClosed';
    }
}
