<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Component;
use HopTop\Vstar\Generated\Codes;
use HopTop\Vstar\Transp;
use HopTop\Vstar\VClass;

/**
 * spec/05 §8 -- the CLASS and TRANSP value domains.
 *
 * @internal
 */
final class Classification
{
    private const PROP_CLASS = 'CLASS';

    private const PROP_TRANSP = 'TRANSP';

    /**
     * Flag a CLASS outside the RFC 5545 §3.8.1.3 vocabulary and a TRANSP
     * outside §3.8.2.7's.
     *
     * Comparison is case-insensitive (spec/05 §8, RFC 5545 §3.1). The
     * value is checked on any component carrying the property -- there is
     * no type gating, because V* diagnoses no scope rule for any property.
     * An X- or IANA token on CLASS, which the RFC's ABNF admits, is still
     * flagged: spec/05 §8 binds the value to the three registered names.
     *
     * @return list<Diagnostic>
     */
    public static function check(Component $c, string $path): array
    {
        $out = [];

        $d = self::vocabularyDiagnostic(
            $c,
            $path,
            self::PROP_CLASS,
            self::classVocabulary(),
            Codes::CLASS_NOT_IN_VOCABULARY,
            'RFC 5545 §3.8.1.3',
        );

        if ($d !== null) {
            $out[] = $d;
        }

        $d = self::vocabularyDiagnostic(
            $c,
            $path,
            self::PROP_TRANSP,
            self::transpVocabulary(),
            Codes::TRANSP_NOT_IN_VOCABULARY,
            'RFC 5545 §3.8.2.7',
        );

        if ($d !== null) {
            $out[] = $d;
        }

        return $out;
    }

    /**
     * The STATUS-shaped diagnostic for property `$name` on `$c` when its
     * value is outside `$allowed`, or null when the property is absent or
     * its value is allowed.
     *
     * @param list<string> $allowed
     */
    private static function vocabularyDiagnostic(
        Component $c,
        string $path,
        string $name,
        array $allowed,
        string $code,
        string $ref,
    ): ?Diagnostic {
        $p = $c->get($name);

        if ($p === null) {
            return null;
        }

        foreach ($allowed as $want) {
            if (Internal::equalFold($p->value, $want)) {
                return null;
            }
        }

        return Diagnostic::of(
            $code,
            $name . ' value ' . $p->value . ' is not valid; allowed: '
                . implode(', ', $allowed) . ' (' . $ref . ')',
            $path . '.' . $name,
        );
    }

    /**
     * The CLASS values RFC 5545 §3.8.1.3 admits, projected from the
     * port's own backed enum rather than read out of
     * `Codes::CLASS_VOCABULARY`, for the reason {@see Status} gives: these
     * are the values the codec encodes against, so the table cannot
     * disagree with what this library writes. The registry's copy is
     * reconciled against the enum in
     * `tests/Validate/RegistryVocabularyTest.php`.
     *
     * @return list<string>
     */
    private static function classVocabulary(): array
    {
        return [
            VClass::Public->value,
            VClass::Private->value,
            VClass::Confidential->value,
        ];
    }

    /**
     * The TRANSP values RFC 5545 §3.8.2.7 admits, from the backed enum for
     * the same reason as {@see self::classVocabulary()}.
     *
     * @return list<string>
     */
    private static function transpVocabulary(): array
    {
        return [
            Transp::Opaque->value,
            Transp::Transparent->value,
        ];
    }
}
