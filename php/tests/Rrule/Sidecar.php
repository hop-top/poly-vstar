<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Rrule;

use HopTop\Vstar\Tests\Corpus;

/**
 * Loader for the `rrule/` corpus and its JSON sidecars.
 *
 * The corpus names a fixture by its stem and hangs optional siblings off
 * it: `.rrule` or `.ics` is the input, and `.expect.json`,
 * `.formatted`, `.next.json`, `.expand.json`, `.between.json` and
 * `.occurrences.json` each pin one call's outcome. A fixture with no
 * sidecar pins exactly one thing -- that the input parses -- and 17 of
 * them do only that.
 *
 * Like {@see Corpus}, every accessor walks the tree rather than naming
 * fixtures, so a fixture added to the corpus becomes a test case without
 * a code change here.
 */
final class Sidecar
{
    /**
     * Corpus-relative stems of every `*.rrule` fixture under
     * `rrule/<subdir>`, sorted for a deterministic test order.
     *
     * A stem is the path with the extension removed, e.g.
     * `rrule/happy/freq_daily` -- which is what every sibling lookup
     * below appends to.
     *
     * @return list<string>
     */
    public static function rruleStems(string $subdir): array
    {
        return self::stems($subdir, 'rrule');
    }

    /**
     * Corpus-relative stems of every `*.ics` fixture under
     * `rrule/<subdir>`.
     *
     * @return list<string>
     */
    public static function icsStems(string $subdir): array
    {
        return self::stems($subdir, 'ics');
    }

    /**
     * The RRULE property value of a `.rrule` fixture, with the file's
     * trailing newline removed -- the corpus stores the value alone, and
     * the newline is the file's, not the value's.
     */
    public static function rruleValue(string $stem): string
    {
        return rtrim(Corpus::read($stem . '.rrule'), "\r\n");
    }

    /**
     * The expected wire form from a `.formatted` sibling, or null when
     * the fixture has none.
     */
    public static function formatted(string $stem): ?string
    {
        $raw = self::readOptional($stem . '.formatted');

        return $raw === null ? null : rtrim($raw, "\r\n");
    }

    /**
     * The sentinel identifier a `.expect.json` sibling names, or null
     * when the fixture has none -- in which case the input must parse.
     */
    public static function sentinel(string $stem): ?string
    {
        $spec = self::json($stem . '.expect.json');

        if ($spec === null) {
            return null;
        }

        $sentinel = $spec['sentinel'] ?? null;

        if (!is_string($sentinel)) {
            throw new \RuntimeException("{$stem}.expect.json has no sentinel");
        }

        return $sentinel;
    }

    /**
     * One outcome sidecar decoded into the shape the verifier uses: the
     * call's inputs plus either an expected occurrence list (and, for a
     * bounded expansion, the `complete` flag) or the failure class the
     * call must raise.
     *
     * Null when the sibling does not exist, which is how an optional
     * sidecar is spelled.
     *
     * @return array{dtstart: ?string, after: ?string, start: ?string, end: ?string, limit: ?int, expected: list<string>, complete: bool, error: ?string}|null
     */
    public static function outcome(string $stem, string $suffix): ?array
    {
        $spec = self::json($stem . '.' . $suffix);

        if ($spec === null) {
            return null;
        }

        /** @var list<string> $expected */
        $expected = [];

        if (isset($spec['expected']) && is_array($spec['expected'])) {
            foreach ($spec['expected'] as $t) {
                if (is_string($t)) {
                    $expected[] = $t;
                }
            }
        }

        return [
            'dtstart' => self::optionalString($spec, 'dtstart'),
            'after' => self::optionalString($spec, 'after'),
            'start' => self::optionalString($spec, 'start'),
            'end' => self::optionalString($spec, 'end'),
            'limit' => isset($spec['limit']) && is_int($spec['limit']) ? $spec['limit'] : null,
            'expected' => $expected,
            'complete' => isset($spec['complete']) && $spec['complete'] === true,
            'error' => self::optionalString($spec, 'error'),
        ];
    }

    /**
     * Stems of every `*.<extension>` fixture under `rrule/<subdir>`.
     *
     * @return list<string>
     */
    private static function stems(string $subdir, string $extension): array
    {
        $dir = 'rrule/' . $subdir;
        $out = [];

        foreach (Corpus::names($dir, $extension) as $name) {
            $out[] = $dir . '/' . basename($name, '.' . $extension);
        }

        sort($out);

        return $out;
    }

    /**
     * A corpus file's bytes, or null when it does not exist.
     */
    private static function readOptional(string $relative): ?string
    {
        $path = Corpus::root() . '/' . $relative;

        if (!is_file($path)) {
            return null;
        }

        return Corpus::read($relative);
    }

    /**
     * A corpus JSON sidecar decoded to an array, or null when absent.
     *
     * @return array<string, mixed>|null
     */
    private static function json(string $relative): ?array
    {
        $raw = self::readOptional($relative);

        if ($raw === null) {
            return null;
        }

        /** @var array<string, mixed> $decoded */
        $decoded = json_decode($raw, true, 512, JSON_THROW_ON_ERROR);

        return $decoded;
    }

    /**
     * A string-valued key of a decoded sidecar, or null when absent.
     *
     * @param array<string, mixed> $spec
     */
    private static function optionalString(array $spec, string $key): ?string
    {
        $v = $spec[$key] ?? null;

        return is_string($v) ? $v : null;
    }
}
