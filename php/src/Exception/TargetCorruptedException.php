<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * The target component's X-VSTAR-HASH does not match its recomputed
 * canonical form.
 */
final class TargetCorruptedException extends VstarException
{
    public function sentinel(): string
    {
        return 'ErrTargetCorrupted';
    }
}
