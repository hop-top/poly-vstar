<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * setHeader called after the first encode locked the header.
 */
final class HeaderLockedException extends VstarException
{
    public function sentinel(): string
    {
        return 'ErrHeaderLocked';
    }
}
