<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Component;
use HopTop\Vstar\Ext\Ext;
use HopTop\Vstar\Generated\Codes;

/**
 * spec/05 §3 and spec/04 -- extension namespace compliance.
 *
 * @internal
 */
final class Extensions
{
    /**
     * One warning per property whose name is neither on the RFC
     * allow-list nor prefixed `X-`.
     *
     * The `X-` classification is delegated to {@see Ext::isExtension()} so
     * there is one answer to "what counts as an extension"; the allow-list
     * stays here because it answers the orthogonal question "is this a
     * known RFC property?", which `ext` has no business knowing.
     *
     * @return list<Diagnostic>
     */
    public static function check(Component $c, string $path): array
    {
        $standard = self::standard();
        $out = [];

        foreach ($c->props as $p) {
            if (isset($standard[strtoupper($p->name)])) {
                continue;
            }

            if (Ext::isExtension($p->name)) {
                continue;
            }

            $out[] = Diagnostic::of(
                Codes::UNKNOWN_PROPERTY,
                'property ' . $p->name . ' is not a known RFC 5545/6350 property and does not use'
                    . ' the X- extension prefix (spec/05 §3, spec/04)',
                $path . '.' . $p->name,
            );
        }

        return $out;
    }

    /**
     * How many properties the generated RFC allow-list carries.
     */
    public static function standardPropertyCount(): int
    {
        return count(Codes::STANDARD_PROPERTIES);
    }

    /**
     * The RFC 5545 §3.7-§3.8 / RFC 6350 §6 allow-list as an uppercased
     * lookup set. The list itself is generated from
     * `spec/registry/standard-properties.json`; this is only the index.
     *
     * @return array<string, true>
     */
    private static function standard(): array
    {
        /** @var array<string, true>|null $index */
        static $index = null;

        if ($index === null) {
            $index = [];

            foreach (Codes::STANDARD_PROPERTIES as $name) {
                $index[strtoupper($name)] = true;
            }
        }

        return $index;
    }
}
