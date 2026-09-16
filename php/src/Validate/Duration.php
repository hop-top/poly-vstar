<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Component;
use HopTop\Vstar\Duration\Duration as DurationParser;
use HopTop\Vstar\Duration\Trigger;
use HopTop\Vstar\Generated\Codes;
use HopTop\Vstar\Property;

/**
 * spec/03 rule 12, spec/05 criterion 7 -- DURATION well-formedness.
 *
 * @internal
 */
final class Duration
{
    private const DURATION = 'DURATION';

    private const TRIGGER = 'TRIGGER';

    private const REPEAT = 'REPEAT';

    /**
     * One diagnostic per duration-bearing property whose value is
     * malformed.
     *
     * TRIGGER goes through {@see Trigger::parse()} rather than a bare
     * duration parse so the two value forms and the RELATED / VALUE
     * parameter rules are enforced together -- an absolute trigger is not
     * a duration, and rejecting it as one would be wrong. DURATION and
     * REPEAT are checked directly: each has exactly one legal shape.
     *
     * @return list<Diagnostic>
     */
    public static function check(Component $c, string $path): array
    {
        $out = [];

        foreach ($c->props as $p) {
            $d = self::checkProperty($p, $path);

            if ($d !== null) {
                $out[] = $d;
            }
        }

        return $out;
    }

    /**
     * The diagnostic `$p` earns, or null when it is clean or carries no
     * duration at all.
     */
    private static function checkProperty(Property $p, string $path): ?Diagnostic
    {
        if (Internal::equalFold($p->name, self::TRIGGER)) {
            $err = self::failure(static fn (): mixed => Trigger::parse($p));

            return $err === null ? null : Diagnostic::of(
                Codes::MALFORMED_DURATION,
                'TRIGGER is malformed (RFC 5545 §3.8.6.3): ' . $err->getMessage(),
                $path . '.' . self::TRIGGER,
            );
        }

        if (Internal::equalFold($p->name, self::DURATION)) {
            $err = self::failure(static fn (): mixed => DurationParser::parse($p->value));

            return $err === null ? null : Diagnostic::of(
                Codes::MALFORMED_DURATION,
                'DURATION is malformed (RFC 5545 §3.3.6): ' . $err->getMessage(),
                $path . '.' . self::DURATION,
            );
        }

        if (Internal::equalFold($p->name, self::REPEAT) && !self::isNonNegativeInteger($p->value)) {
            return Diagnostic::of(
                Codes::MALFORMED_DURATION,
                'REPEAT is not a non-negative integer (RFC 5545 §3.8.6.2): ' . $p->value,
                $path . '.' . self::REPEAT,
            );
        }

        return null;
    }

    /**
     * Whether `$value` is a non-negative decimal integer.
     *
     * Deliberately a pattern rather than a cast, which happily reads
     * `"3abc"` as `3` and would let a malformed REPEAT through.
     */
    private static function isNonNegativeInteger(string $value): bool
    {
        return preg_match('/^\d+$/', $value) === 1;
    }

    /**
     * The failure `$fn` throws, or null when it returns.
     *
     * @param callable(): mixed $fn
     */
    private static function failure(callable $fn): ?\Throwable
    {
        try {
            $fn();

            return null;
        } catch (\Throwable $e) {
            return $e;
        }
    }
}
