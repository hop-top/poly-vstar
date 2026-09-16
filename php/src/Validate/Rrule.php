<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Component;
use HopTop\Vstar\Exception\UnsupportedRRuleException;
use HopTop\Vstar\Generated\Codes;
use HopTop\Vstar\Rrule\Rrule as RruleParser;

/**
 * spec/03 §RRULE parsing scope -- RRULE conformance.
 *
 * @internal
 */
final class Rrule
{
    /** RFC 5545 §3.8.5.3. */
    private const RRULE = 'RRULE';

    /**
     * One diagnostic per RRULE property whose value fails validation.
     *
     * The split is by failure class, not by guesswork: an unsupported
     * feature is a warning because the property still round-trips through
     * the codec -- only its recurrence semantics are out of reach -- while
     * a malformed value is an error because no consumer, V* or otherwise,
     * can evaluate it.
     *
     * A failure that classifies as neither is reported as malformed rather
     * than swallowed: a finding the consumer can see beats silence.
     *
     * @return list<Diagnostic>
     */
    public static function check(Component $c, string $path): array
    {
        $out = [];

        foreach ($c->props as $p) {
            if (!Internal::equalFold($p->name, self::RRULE)) {
                continue;
            }

            $err = self::validationError($p->value);

            if ($err === null) {
                continue;
            }

            $unsupported = $err instanceof UnsupportedRRuleException;

            $out[] = Diagnostic::of(
                $unsupported ? Codes::R_RULE_UNSUPPORTED : Codes::R_RULE_MALFORMED,
                $unsupported
                    ? 'RRULE uses a feature outside the RRULE parsing scope'
                        . ' (spec/03 §RRULE parsing scope): ' . $err->getMessage()
                    : 'RRULE is malformed (RFC 5545 §3.3.10): ' . $err->getMessage(),
                $path . '.' . self::RRULE,
            );
        }

        return $out;
    }

    /**
     * The failure `Rrule::validate` raises for `$value`, or null when the
     * value is clean.
     */
    private static function validationError(string $value): ?\Throwable
    {
        try {
            RruleParser::validate($value);

            return null;
        } catch (\Throwable $e) {
            return $e;
        }
    }
}
