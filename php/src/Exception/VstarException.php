<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * Base of the twelve V* failure classes.
 *
 * The identifier returned by {@see self::sentinel()} is the contract, not
 * the class name: the conformance corpus names a failure class by its
 * reference spelling (`ErrMalformed` and friends) in `malformed/*.error`
 * files and `rrule/**\/*.expect.json` sidecars, so a caught failure must
 * be able to hand a test that exact string.
 *
 * Catch by class when you know which failure you are handling, or catch
 * this base and switch on `sentinel()` when you are dispatching.
 */
abstract class VstarException extends \RuntimeException
{
    /**
     * The stable identifier for this failure class.
     */
    abstract public function sentinel(): string;
}
