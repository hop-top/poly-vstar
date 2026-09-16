<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Component;
use HopTop\Vstar\Generated\Codes;

/**
 * spec/05 §1 -- the required common properties.
 *
 * @internal
 */
final class Required
{
    /**
     * One diagnostic per missing required common property.
     *
     * UID, DTSTAMP and X-VSTAR-HASH are checked in that order so the
     * diagnostic stream is deterministic. Each path appends the property
     * name to the component locator.
     *
     * @return list<Diagnostic>
     */
    public static function check(Component $c, string $path): array
    {
        $out = [];

        if (!Internal::has($c, 'UID')) {
            $out[] = Diagnostic::of(
                Codes::MISSING_UID,
                'required common property UID is missing (spec/02)',
                $path . '.UID',
            );
        }

        if (!Internal::has($c, 'DTSTAMP')) {
            $out[] = Diagnostic::of(
                Codes::MISSING_DTSTAMP,
                'required common property DTSTAMP is missing (spec/02)',
                $path . '.DTSTAMP',
            );
        }

        if (!Internal::has($c, Internal::X_VSTAR_HASH)) {
            $out[] = Diagnostic::of(
                Codes::MISSING_XVSTAR_HASH,
                'required common property ' . Internal::X_VSTAR_HASH . ' is missing (spec/02)',
                $path . '.' . Internal::X_VSTAR_HASH,
            );
        }

        return $out;
    }
}
