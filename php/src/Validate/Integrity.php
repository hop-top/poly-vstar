<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Component;
use HopTop\Vstar\Generated\Codes;
use HopTop\Vstar\Hashing\Hashing;

/**
 * spec/05 §2 -- `X-VSTAR-HASH` integrity.
 *
 * @internal
 */
final class Integrity
{
    /**
     * Recompute `$c`'s content hash and flag a stored `X-VSTAR-HASH` that
     * is present but wrong.
     *
     * An **absent** hash is intentionally not flagged here -- that case
     * belongs to {@see Required}. The split keeps the diagnostic surface
     * unambiguous: present-but-wrong is a different bug than absent, and a
     * consumer that sees both codes at once is looking at a genuinely
     * different document than one that sees either alone.
     *
     * @return list<Diagnostic>
     */
    public static function check(Component $c, string $path): array
    {
        if (!Internal::has($c, Internal::X_VSTAR_HASH)) {
            return [];
        }

        $result = Hashing::verifyXVstar($c);

        if ($result['ok']) {
            return [];
        }

        return [Diagnostic::of(
            Codes::BAD_XVSTAR_HASH,
            Internal::X_VSTAR_HASH . ' does not match recomputed canonical hash; want='
                . $result['want'] . ' got=' . $result['got'] . ' (spec/05 §2)',
            $path . '.' . Internal::X_VSTAR_HASH,
        )];
    }
}
