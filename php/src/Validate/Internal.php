<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Component;

/**
 * The lookup and path helpers every rule module builds on.
 *
 * Internal to the namespace: nothing here is part of the API mapping's
 * `validate` surface, and no consumer should reach for it.
 *
 * @internal
 */
final class Internal
{
    /**
     * The property name V* uses to carry the content hash.
     *
     * Deliberately a local constant rather than a reach into `Hashing`:
     * `validate` inspects raw property names without taking on the
     * hashing module's identity, and the two agreeing is asserted by the
     * fixtures.
     */
    public const X_VSTAR_HASH = 'X-VSTAR-HASH';

    /**
     * Whether `$c` carries a property named `$name`, case-insensitively
     * per RFC 5545 §3.1.
     */
    public static function has(Component $c, string $name): bool
    {
        return $c->get($name) !== null;
    }

    /**
     * Case-insensitive string comparison, per RFC 5545 §3.1.
     */
    public static function equalFold(string $a, string $b): bool
    {
        return strcasecmp($a, $b) === 0;
    }

    /**
     * The path segment identifying `$c` relative to its container:
     * `<TYPE>[uid=<uid>]`, or `<TYPE>[#<n>]` when `$c` has no UID.
     *
     * `$index` carries the running positional counter per component type,
     * by reference, so two UID-less components of the same type get
     * distinct, stable segments.
     *
     * @param array<string, int> $index
     */
    public static function componentPath(Component $c, array &$index): string
    {
        $uid = $c->uid();

        if ($uid !== '') {
            return $c->type . '[uid=' . $uid . ']';
        }

        $n = $index[$c->type] ?? 0;
        $index[$c->type] = $n + 1;

        return $c->type . '[#' . $n . ']';
    }

    /**
     * Which of `$names` `$c` lacks, in the order given.
     *
     * @param list<string> $names
     *
     * @return list<string>
     */
    public static function missing(Component $c, array $names): array
    {
        return array_values(array_filter(
            $names,
            static fn (string $name): bool => !self::has($c, $name),
        ));
    }
}
