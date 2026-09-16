<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Ext;

/**
 * The classification of an extension name under spec/04 (Extension
 * Discipline).
 *
 * The backing values are the lowercase wire tokens
 * `spec/behavior/ext/scopes.json` uses, so a port needs no case
 * convention of its own and a fixture comparison is a direct `->value`
 * check. The reference's capitalized display spelling is a separate
 * concern, reached through {@see self::toString()}.
 *
 * `None` is the zero-ish member -- the name is not an `X-*` extension at
 * all -- and `Unknown` is the catch-all for a name that carries the `X-`
 * prefix but matches no sanctioned tier.
 */
enum Scope: string
{
    /** Not an `X-*` extension at all. */
    case None = 'none';

    /**
     * `X-VSTAR-*` -- a cross-system V* extension on the stabilization
     * track. `X-VSTAR-HASH` (spec/02) is the only mandatory member in
     * v0.1.
     */
    case VStar = 'vstar';

    /**
     * `X-<SYSTEM>-*` -- an extension owned by one consuming system, e.g.
     * `X-AGR-INTENT`. The slug is any token other than `VSTAR` and `EXP`,
     * both of which are reserved for the tiers above and below.
     */
    case System = 'system';

    /**
     * `X-EXP-*` -- an unstable experimental extension. Senders MUST NOT
     * depend on receivers honoring these (spec/04).
     */
    case Experimental = 'experimental';

    /**
     * Has the `X-` prefix but matches no sanctioned tier -- `X-`,
     * `X-FOO`, `X-VSTAR-`. Treat as opaque; receivers must still ignore
     * it per RFC 5545 compatibility rules.
     */
    case Unknown = 'unknown';

    /**
     * The reference's human-readable spelling -- `None`, `VStar`,
     * `System`, `Experimental`, `Unknown`.
     *
     * This is the Go reference's `Scope.String()`, kept for error
     * messages and debug logging. It is display text, not the contract:
     * the fixtures and every programmatic comparison use the lowercase
     * token this enum is backed by.
     *
     * The method is `toString()`, not `__toString()`: PHP rejects the
     * magic method on an enum at declaration time ("Enum X cannot include
     * magic method __toString"). Non-enum value classes in this library
     * keep `__toString()`.
     */
    public function toString(): string
    {
        return match ($this) {
            self::None => 'None',
            self::VStar => 'VStar',
            self::System => 'System',
            self::Experimental => 'Experimental',
            self::Unknown => 'Unknown',
        };
    }
}
