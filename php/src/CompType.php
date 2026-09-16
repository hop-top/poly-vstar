<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * The wire-string component type of a {@see Component}.
 *
 * Cases mirror the RFC 5545 §3.4 / §3.6 component identifiers exactly.
 * VCARD is deliberately absent: vCards are {@see Card}, not Component.
 *
 * Like the reference's `CompType`, the vocabulary is **open**: the cases
 * name the registered component types, they do not bound them. A
 * document may legitimately carry a block this enum has no case for --
 * `STANDARD` and `DAYLIGHT` inside a VTIMEZONE are the everyday example,
 * and RFC 5545 §3.6 admits `X-`-prefixed components besides. So
 * {@see Component::$type} holds the wire string and this enum is the
 * named view of it, reached with {@see self::tryFrom()} or by comparing
 * against `->value`. A port that stored a `CompType` instance on the
 * component would be unable to represent half the conformance corpus.
 */
enum CompType: string
{
    case Calendar = 'VCALENDAR';
    case Todo = 'VTODO';
    case Journal = 'VJOURNAL';
    case Event = 'VEVENT';
    case FreeBusy = 'VFREEBUSY';
    case Timezone = 'VTIMEZONE';
    case Alarm = 'VALARM';

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
