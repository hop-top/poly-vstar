<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests;

/**
 * Loader for the shared behavior families at `spec/behavior/`.
 *
 * Families are not uniform and this loader does not pretend they are. Two
 * shapes appear: **paired**, an `.ics` input with a same-basename `.json`
 * sibling naming the expectation; and **standalone**, a single `.json`
 * holding a list of self-contained cases. `time/tzid.json` and
 * `duration/parse.json` are standalone; the `duration/*.trigger.json`
 * files are paired.
 *
 * Like {@see Corpus}, this walks the tree rather than naming fixtures, so
 * a case added to the corpus becomes a test case without a code change.
 */
final class Behavior
{
    /**
     * Absolute path to the behavior corpus root.
     */
    public static function root(): string
    {
        return dirname(__DIR__, 2) . '/spec/behavior';
    }

    /**
     * Raw bytes of one behavior file.
     */
    public static function read(string $relative): string
    {
        $raw = file_get_contents(self::root() . '/' . $relative);

        if ($raw === false) {
            throw new \RuntimeException("cannot read behavior/{$relative}");
        }

        return $raw;
    }

    /**
     * One behavior JSON file decoded as a list of case objects.
     *
     * @return list<array<string, mixed>>
     */
    public static function cases(string $relative): array
    {
        /** @var list<array<string, mixed>> $decoded */
        $decoded = json_decode(self::read($relative), true, 512, JSON_THROW_ON_ERROR);

        return $decoded;
    }

    /**
     * Basenames of every `*.<suffix>` file in a behavior family, sorted
     * for a deterministic test order. The suffix is the part after the
     * stem, e.g. `trigger.json`.
     *
     * @return list<string>
     */
    public static function stems(string $family, string $suffix): array
    {
        $paths = glob(self::root() . '/' . $family . '/*.' . $suffix);

        if ($paths === false) {
            throw new \RuntimeException("cannot list {$family}/*.{$suffix}");
        }

        $stems = array_map(
            static fn (string $p): string => basename($p, '.' . $suffix),
            $paths,
        );
        sort($stems);

        return $stems;
    }
}
