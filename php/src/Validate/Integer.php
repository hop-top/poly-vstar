<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Component;
use HopTop\Vstar\Generated\Codes;

/**
 * spec/05 §8 -- the bounded integer value domains, and the canonical
 * decimal form they share with REPEAT.
 *
 * @internal
 */
final class Integer
{
    private const PRIORITY = 'PRIORITY';

    private const PERCENT_COMPLETE = 'PERCENT-COMPLETE';

    private const SEQUENCE = 'SEQUENCE';

    /**
     * Whether `$s` is a canonical non-negative decimal per spec/05 §8:
     * digits only, no sign, no whitespace, and no leading zero unless `$s`
     * is exactly `0`.
     *
     * The check is textual on purpose. It never converts `$s` to a
     * machine integer, so a value past `PHP_INT_MAX` (a SEQUENCE of 2^64)
     * is well-formed -- the spec bounds SEQUENCE below, never above. A
     * cast would also read `+3` as 3 and `07` as 7, and `ctype_digit`
     * accepts `01`; none of those is canonical.
     *
     * The anchors are `\A` and `\z`, not `^` and `$`: PCRE's `$` also
     * matches before a trailing newline, which would let `"3\n"` through.
     */
    public static function isCanonicalDecimal(string $s): bool
    {
        return preg_match('/\A(0|[1-9][0-9]*)\z/', $s) === 1;
    }

    /**
     * One diagnostic per bounded integer property whose value is not a
     * canonical decimal inside its RFC 5545 domain.
     *
     * The value is checked wherever the property appears; the rule does
     * not gate on component type (PERCENT-COMPLETE on a VEVENT is
     * bounded, not flagged for scope). One diagnostic per offending
     * property, at the property's path, like the STATUS and DURATION
     * rules.
     *
     * @return list<Diagnostic>
     */
    public static function check(Component $c, string $path): array
    {
        $out = [];

        foreach ($c->props as $p) {
            foreach (self::domains() as $name => [$section, $max]) {
                if (!Internal::equalFold($p->name, $name)) {
                    continue;
                }

                $d = self::domainDiagnostic($name, $section, $max, $p->value, $path);

                if ($d !== null) {
                    $out[] = $d;
                }

                break;
            }
        }

        return $out;
    }

    /**
     * The diagnostic `$value` earns for property `$name`, or null when it
     * is a canonical decimal inside `0..$max` (`$max` null meaning
     * unbounded).
     */
    private static function domainDiagnostic(
        string $name,
        string $section,
        ?string $max,
        string $value,
        string $path,
    ): ?Diagnostic {
        if (!self::isCanonicalDecimal($value)) {
            return Diagnostic::of(
                Codes::INTEGER_OUT_OF_DOMAIN,
                $name . ' is not a canonical non-negative decimal (RFC 5545 '
                    . $section . ', spec/05 §8): ' . $value,
                $path . '.' . $name,
            );
        }

        if ($max !== null && self::exceedsDigitString($value, $max)) {
            return Diagnostic::of(
                Codes::INTEGER_OUT_OF_DOMAIN,
                $name . ' value ' . $value . ' is outside 0–' . $max
                    . ' (RFC 5545 ' . $section . ')',
                $path . '.' . $name,
            );
        }

        return null;
    }

    /**
     * Whether the canonical decimal `$v` is numerically greater than the
     * canonical decimal `$max`. Both must already satisfy
     * {@see self::isCanonicalDecimal()}: with no leading zeros, a longer
     * string is a larger number and equal lengths compare bytewise.
     */
    private static function exceedsDigitString(string $v, string $max): bool
    {
        if (strlen($v) !== strlen($max)) {
            return strlen($v) > strlen($max);
        }

        return strcmp($v, $max) > 0;
    }

    /**
     * The bounded integer properties, in the order the codes catalog lists
     * them: name => [the RFC 5545 section that bounds it, the upper bound
     * as a digit string or null when unbounded]. The lower bound is always
     * 0, which the canonical-decimal form already enforces (no sign).
     *
     * @return array<string, array{string, ?string}>
     */
    private static function domains(): array
    {
        return [
            self::PRIORITY => ['§3.8.1.9', '9'],
            self::PERCENT_COMPLETE => ['§3.8.1.8', '100'],
            self::SEQUENCE => ['§3.8.7.4', null],
        ];
    }
}
