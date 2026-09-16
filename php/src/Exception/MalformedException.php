<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * Structurally invalid input: a bad escape, bad parameter syntax, an
 * unparseable value, or an RRULE on the parsing scope's hard-error
 * list.
 */
final class MalformedException extends VstarException
{
    public function sentinel(): string
    {
        return 'ErrMalformed';
    }
}
