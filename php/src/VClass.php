<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * The wire-string CLASS value per RFC 5545 §3.8.1.3: the access
 * classification of a VEVENT, VTODO or VJOURNAL.
 *
 * Named `VClass` rather than `Class`: `class` is a reserved word in PHP,
 * and a type spelled `Class` would read as the language's own reflection
 * type at every call site.
 *
 * CLASS is optional; RFC 5545 §3.8.1.3 assigns PUBLIC when the property
 * is absent. Applying that default is a helper's job, not the getter's.
 */
enum VClass: string
{
    case Public = 'PUBLIC';
    case Private = 'PRIVATE';
    case Confidential = 'CONFIDENTIAL';

    /**
     * The RFC wire spelling, as `->value`.
     *
     * The reference gives this type no `String()`; the method exists so
     * every enum in the package carries the same accessor. See
     * {@see Rrule\Freq::toString()} for why an enum spells it
     * `toString()`.
     */
    public function toString(): string
    {
        return $this->value;
    }
}
