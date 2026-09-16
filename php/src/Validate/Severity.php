<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

/**
 * The impact level of a {@see Diagnostic}.
 *
 * `Error` marks a MUST violation -- the document is not V* conformant.
 * `Warning` marks a SHOULD violation or a stylistic concern, such as an
 * unknown property name that does not use the `X-` extension prefix.
 *
 * The backing values are the registry's own spellings, so a severity read
 * out of `spec/registry/diagnostic-codes.json` maps onto a case with
 * {@see self::tryFrom()} and needs no translation table.
 */
enum Severity: string
{
    /** A MUST violation: the document is not V* conformant. */
    case Error = 'error';

    /** A SHOULD violation or a stylistic concern. */
    case Warning = 'warning';

    /**
     * The lowercase display name, `error` or `warning`.
     *
     * The API mapping spells the reference's `String()` as `toString()`
     * here rather than `__toString()`, which PHP forbids on an enum --
     * the engine rejects the declaration outright. `->value` is the
     * idiomatic reach for a backed enum's string and remains available;
     * this exists so the reference's method has a same-shaped counterpart
     * at every call site.
     *
     * Programmatic checks should still match the case, not the string.
     */
    public function toString(): string
    {
        return $this->value;
    }
}
