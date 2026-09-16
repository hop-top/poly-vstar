<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Canonical;

use HopTop\Vstar\Canonical\Canonical;
use HopTop\Vstar\Codec\Rfc5545\Parser as IcsParser;
use HopTop\Vstar\Codec\Rfc6350\Parser as VcfParser;
use HopTop\Vstar\Hashing\Hashing;
use HopTop\Vstar\Tests\Corpus;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * The layer-(b) gate: for every conformance fixture carrying a
 * `.canonical` sibling, this port's canonical bytes MUST equal the file's,
 * byte for byte; for every `.hash` sibling, the hash string MUST equal the
 * file's contents.
 *
 * The comparison compares BYTES. PHP strings are byte arrays, so no
 * decoding happens anywhere in this file -- no `trim()` on produced bytes,
 * no `mb_convert_encoding`, no diff helper that ignores whitespace. Each
 * of those turns a failing port into a passing one while the bytes stay
 * wrong.
 *
 * The single licensed transform is stripping `\r` before `\n` in OUR
 * produced bytes: the corpus is LF on disk, the canonical form is CRLF
 * (spec rule 1). The transform runs on our output, never on the file's,
 * because converting the file's LF to CRLF would silently repair a bare
 * `\n` our encoder should never have emitted.
 *
 * The hash is taken over the CRLF bytes, not the LF-transformed ones. It
 * is the backstop: SHA-256 over trimmed bytes is a different hash, so a
 * passing `.canonical` comparison next to a failing `.hash` means the
 * comparison is lying, not the hasher.
 */
final class CorpusTest extends TestCase
{
    /**
     * @return iterable<string, array{string, string}>
     */
    public static function calendarFixtures(): iterable
    {
        foreach (['rfc5545', 'supersession'] as $family) {
            foreach (Corpus::names($family, 'ics') as $name) {
                yield "{$family}/{$name}" => [$family, $name];
            }
        }
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function cardFixtures(): iterable
    {
        foreach (Corpus::names('rfc6350', 'vcf') as $name) {
            yield $name => [$name];
        }
    }

    #[DataProvider('calendarFixtures')]
    public function testCalendarCanonicalizesToTheCorpusBytes(string $family, string $name): void
    {
        $stem = basename($name, '.ics');
        $want = Corpus::read("{$family}/{$stem}.canonical");

        $got = Canonical::calendar(IcsParser::parse(Corpus::read("{$family}/{$name}")));

        self::assertBytesIdentical($want, Corpus::crlfToLf($got), "{$family}/{$stem}.canonical");
    }

    #[DataProvider('calendarFixtures')]
    public function testCalendarHashesToTheCorpusHash(string $family, string $name): void
    {
        $stem = basename($name, '.ics');
        $want = trim(Corpus::read("{$family}/{$stem}.hash"));

        $got = Hashing::calendar(IcsParser::parse(Corpus::read("{$family}/{$name}")));

        self::assertSame($want, $got, "{$family}/{$stem}.hash");
    }

    #[DataProvider('cardFixtures')]
    public function testCardCanonicalizesToTheCorpusBytes(string $name): void
    {
        $stem = basename($name, '.vcf');
        $cards = VcfParser::parse(Corpus::read("rfc6350/{$name}"));
        self::assertCount(1, $cards, "rfc6350/{$name} should hold exactly one card");

        $want = Corpus::read("rfc6350/{$stem}.canonical");
        $got = Canonical::card($cards[0]);

        self::assertBytesIdentical($want, Corpus::crlfToLf($got), "rfc6350/{$stem}.canonical");
    }

    #[DataProvider('cardFixtures')]
    public function testCardHashesToTheCorpusHash(string $name): void
    {
        $stem = basename($name, '.vcf');
        $cards = VcfParser::parse(Corpus::read("rfc6350/{$name}"));

        self::assertSame(
            trim(Corpus::read("rfc6350/{$stem}.hash")),
            Hashing::card($cards[0]),
            "rfc6350/{$stem}.hash",
        );
    }

    /**
     * Every canonicalizing fixture family carries both siblings.
     *
     * Without this the providers above would silently shrink if a sibling
     * went missing: the fixture would still be enumerated, and reading an
     * absent `.canonical` would fail with a file error rather than naming
     * the gap.
     */
    public function testEveryCanonicalizingFixtureHasBothSiblings(): void
    {
        foreach (['rfc5545' => 'ics', 'supersession' => 'ics', 'rfc6350' => 'vcf'] as $family => $ext) {
            foreach (Corpus::names($family, $ext) as $name) {
                $stem = basename($name, '.' . $ext);

                self::assertFileExists(
                    Corpus::root() . "/{$family}/{$stem}.canonical",
                    "{$family}/{$stem} has no .canonical sibling",
                );
                self::assertFileExists(
                    Corpus::root() . "/{$family}/{$stem}.hash",
                    "{$family}/{$stem} has no .hash sibling",
                );
            }
        }
    }

    /**
     * The canonical form terminates every physical line with CRLF (rule 1)
     * and never emits a bare LF.
     */
    #[DataProvider('calendarFixtures')]
    public function testCanonicalFormEmitsNoBareLf(string $family, string $name): void
    {
        $got = Canonical::calendar(IcsParser::parse(Corpus::read("{$family}/{$name}")));

        for ($i = 0, $n = strlen($got); $i < $n; ++$i) {
            if ($got[$i] !== "\n") {
                continue;
            }

            self::assertTrue(
                $i > 0 && $got[$i - 1] === "\r",
                "{$family}/{$name}: bare LF at byte offset {$i}",
            );
        }
    }

    /**
     * Hashing is deterministic across repeated runs of the same input.
     *
     * A hash that varies run to run would make every stored `X-VSTAR-HASH`
     * worthless, and the usual causes -- an unordered map iteration, a
     * timestamp, a random seed in a sort -- all produce an intermittent
     * failure rather than a reproducible one, so the loop count matters.
     */
    public function testHashingIsDeterministicOverAHundredRuns(): void
    {
        $input = Corpus::read('rfc5545/world.ics');
        $first = Hashing::calendar(IcsParser::parse($input));

        for ($i = 0; $i < 100; ++$i) {
            self::assertSame($first, Hashing::calendar(IcsParser::parse($input)));
        }
    }

    /**
     * Assert two binary strings are byte-identical, naming the first
     * divergent offset and the bytes on each side when they are not.
     *
     * PHPUnit's own string diff renders both sides as text, which is
     * unreadable for canonical bytes that may split a UTF-8 sequence at a
     * fold. The offset and the two hex bytes are what actually locates the
     * bug.
     */
    private static function assertBytesIdentical(string $want, string $got, string $label): void
    {
        if ($want === $got) {
            self::assertSame($want, $got, $label);

            return;
        }

        $n = min(strlen($want), strlen($got));

        for ($i = 0; $i < $n; ++$i) {
            if ($want[$i] === $got[$i]) {
                continue;
            }

            self::fail(sprintf(
                '%s: first divergence at byte offset %d — want 0x%02X (%s), got 0x%02X (%s)%s',
                $label,
                $i,
                ord($want[$i]),
                self::printable($want[$i]),
                ord($got[$i]),
                self::printable($got[$i]),
                self::context($want, $got, $i),
            ));
        }

        self::fail(sprintf(
            '%s: identical for the first %d bytes, then lengths differ — want %d bytes, got %d%s',
            $label,
            $n,
            strlen($want),
            strlen($got),
            self::context($want, $got, $n),
        ));
    }

    /**
     * The 32 bytes on each side of `$offset`, rendered readably.
     */
    private static function context(string $want, string $got, int $offset): string
    {
        $from = max(0, $offset - 32);

        return sprintf(
            "\n  want …%s…\n  got  …%s…",
            self::readable(substr($want, $from, 64)),
            self::readable(substr($got, $from, 64)),
        );
    }

    /**
     * A byte rendered as itself when printable ASCII, else as a hex
     * escape.
     */
    private static function printable(string $byte): string
    {
        $c = ord($byte);

        return ($c >= 0x20 && $c < 0x7F) ? "'{$byte}'" : sprintf('\\x%02X', $c);
    }

    /**
     * A byte run rendered with CR and LF spelled out, so a line-ending
     * divergence is visible rather than reflowing the message.
     */
    private static function readable(string $run): string
    {
        $out = '';

        for ($i = 0, $n = strlen($run); $i < $n; ++$i) {
            $c = ord($run[$i]);
            $out .= match (true) {
                $c === 0x0D => '\\r',
                $c === 0x0A => '\\n',
                $c >= 0x20 && $c < 0x7F => $run[$i],
                default => sprintf('\\x%02X', $c),
            };
        }

        return $out;
    }
}
