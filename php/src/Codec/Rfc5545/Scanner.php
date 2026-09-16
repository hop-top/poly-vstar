<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Codec\Rfc5545;

use HopTop\Vstar\Codec\ContentLine;

/**
 * The public re-export of the RFC 5545 §3.1 content-line scanner.
 *
 * Unfolding is liberal: CRLF and bare LF are both accepted, blank lines
 * are skipped, and a trailing line with no terminator is surfaced. See
 * {@see ContentLine} for the full rules.
 */
final class Scanner
{
    /**
     * The logical content lines of `$input`, in input order, with folds
     * resolved. Line terminators are not included.
     *
     * @return \Generator<int, string>
     */
    public static function lines(string $input): \Generator
    {
        yield from ContentLine::lines($input);
    }
}
