<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests;

/**
 * Loader for the shared conformance corpus at `spec/v1.0/conformance/`.
 *
 * The corpus is authored once and consumed identically by every port, so
 * the loader walks the tree rather than naming individual fixtures: a
 * fixture added to the corpus is picked up here without a code change.
 *
 * Every read returns a binary string. Fixture files are LF-terminated on
 * disk while the encoders emit CRLF, and the one licensed transform runs
 * on *produced* bytes -- never on the file's -- so a stray bare LF in an
 * encoder cannot be silently repaired by the comparison. See
 * `docs/dev/porting-guide.md`.
 */
final class Corpus
{
    /**
     * Absolute path to the conformance corpus root.
     */
    public static function root(): string
    {
        return dirname(__DIR__, 2) . '/spec/v1.0/conformance';
    }

    /**
     * Basenames of every file in a corpus subdirectory with the given
     * extension, sorted for a deterministic test order.
     *
     * @return list<string>
     */
    public static function names(string $subdir, string $extension): array
    {
        $paths = glob(self::root() . '/' . $subdir . '/*.' . $extension);

        if ($paths === false) {
            throw new \RuntimeException("cannot list {$subdir}/*.{$extension}");
        }

        $names = array_map(static fn (string $p): string => basename($p), $paths);
        sort($names);

        return $names;
    }

    /**
     * Raw bytes of one corpus file.
     */
    public static function read(string $relative): string
    {
        $raw = file_get_contents(self::root() . '/' . $relative);

        if ($raw === false) {
            throw new \RuntimeException("cannot read {$relative}");
        }

        return $raw;
    }

    /**
     * Every `*.bytes` fuzz seed, keyed by its corpus-relative path.
     *
     * @return list<string>
     */
    public static function fuzzSeeds(): array
    {
        $paths = glob(self::root() . '/fuzz-seed/*/*.bytes');

        if ($paths === false) {
            throw new \RuntimeException('cannot list fuzz-seed/*/*.bytes');
        }

        $root = self::root() . '/';
        $out = array_map(
            static fn (string $p): string => substr($p, strlen($root)),
            $paths,
        );
        sort($out);

        return $out;
    }

    /**
     * The sentinel identifier named by a `malformed/<stem>.error` sibling:
     * its first non-empty line, e.g. `ErrMalformed`.
     */
    public static function sentinel(string $stem): string
    {
        $raw = self::read("malformed/{$stem}.error");

        foreach (explode("\n", $raw) as $line) {
            $line = trim($line);

            if ($line !== '') {
                return $line;
            }
        }

        throw new \RuntimeException("malformed/{$stem}.error is empty");
    }

    /**
     * Strip the CR of every CRLF in *produced* bytes, the single transform
     * licensed when comparing encoder output against an LF-on-disk fixture.
     */
    public static function crlfToLf(string $produced): string
    {
        return str_replace("\r\n", "\n", $produced);
    }
}
