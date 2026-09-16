<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * A single property parameter, e.g. `CN=Jad` on an ATTENDEE.
 *
 * Parameter names are compared case-insensitively per RFC 5545 §3.2 /
 * RFC 6350 §5; values are compared case-sensitively at this layer, and
 * the codec normalizes where the wire requires it.
 *
 * Readonly: a parsed parameter is a record of what the wire said, and
 * mutating one in place would let a component's canonical bytes depend on
 * its edit history. Build a new one instead.
 */
final class Param
{
    public function __construct(
        public readonly string $name,
        public readonly string $value,
    ) {
    }
}
