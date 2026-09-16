<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Duration;

/**
 * Which end of the parent component a relative TRIGGER is measured from,
 * per the RFC 5545 §3.2.14 RELATED parameter.
 *
 * The wire spelling is the backing value, so a `Related` stringifies to
 * exactly what the property carries.
 */
enum Related: string
{
    /**
     * Anchor the trigger to the parent's start (DTSTART). This is the RFC
     * default when RELATED is absent.
     */
    case Start = 'START';

    /**
     * Anchor the trigger to the parent's end -- DTEND, else
     * DTSTART + DURATION, else a VTODO's DUE.
     */
    case End = 'END';

    /**
     * The RFC wire spelling.
     *
     * The API mapping spells this `__toString()`, which PHP forbids on an
     * enum -- the engine rejects the declaration outright ("Enum ... cannot
     * include magic method __toString"). The same table names `Related` as
     * a PHP 8.1 backed enum, so the two requirements cannot both hold and
     * the enum wins: it is the shape the rest of the API (the `Trigger`
     * property type, the `RELATED` parameter comparison) is built on.
     *
     * `->value` is the idiomatic reach for a backed enum's wire string and
     * remains available; this method exists so the reference's `String()`
     * has a same-shaped counterpart at every call site.
     */
    public function toString(): string
    {
        return $this->value;
    }
}
