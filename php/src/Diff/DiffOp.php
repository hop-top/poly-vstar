<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Diff;

/**
 * The kind of property change recorded in a {@see PropertyDiff}.
 *
 * The backing values are the wire tokens `spec/behavior/diff/*.diff.json`
 * uses, so a fixture comparison is a direct `->value` check.
 *
 * The reference numbers this enum from 1, deliberately: zero is not a
 * valid operation, so a default-constructed value cannot masquerade as
 * "added". A string-backed PHP enum has no zero value to guard against,
 * which reaches the same guarantee by construction.
 */
enum DiffOp: string
{
    /** Present in the new side but missing from the old. */
    case Added = 'add';

    /** Present in the old side but missing from the new. */
    case Removed = 'remove';

    /** Present in both, with a differing value or parameter set. */
    case Changed = 'change';

    /**
     * The reference's human-readable spelling -- `Added`, `Removed`,
     * `Changed` -- used by {@see ComponentDiff::__toString()}.
     *
     * A plain method, not `__toString()`: PHP rejects the magic method on
     * an enum at declaration time. {@see ComponentDiff} is a class rather
     * than an enum, so it keeps the magic spelling.
     */
    public function toString(): string
    {
        return match ($this) {
            self::Added => 'Added',
            self::Removed => 'Removed',
            self::Changed => 'Changed',
        };
    }
}
