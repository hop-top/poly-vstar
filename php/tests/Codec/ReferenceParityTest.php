<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Codec;

use HopTop\Vstar\Codec\Rfc5545\Encoder as IcsEncoder;
use HopTop\Vstar\Codec\Rfc5545\Parser as IcsParser;
use HopTop\Vstar\Codec\Rfc6350\Encoder as VcfEncoder;
use HopTop\Vstar\Codec\Rfc6350\Parser as VcfParser;
use HopTop\Vstar\Tests\Corpus;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * Byte parity with the reference implementation.
 *
 * A round-trip proves a codec agrees with itself. It does not prove it
 * agrees with the reference: a parser that sorts properties, or an
 * encoder that folds one octet late, round-trips perfectly and emits
 * different bytes -- which is a different canonical form and a different
 * hash. These assertions pin the reference's exact output for every
 * fixture, so such a divergence fails here rather than three layers
 * later as an unexplained hash mismatch.
 *
 * `tests/Fixtures/reference-encoding.json` holds base64 of the bytes the
 * Go implementation under `go/` produces for each fixture.
 */
final class ReferenceParityTest extends TestCase
{
    /**
     * @return array{rfc5545: array<string, string>, rfc6350: array<string, string>}
     */
    private static function reference(): array
    {
        $raw = file_get_contents(__DIR__ . '/../Fixtures/reference-encoding.json');

        if ($raw === false) {
            throw new \RuntimeException('cannot read reference-encoding.json');
        }

        /** @var array{rfc5545: array<string, string>, rfc6350: array<string, string>} $decoded */
        $decoded = json_decode($raw, true, 512, JSON_THROW_ON_ERROR);

        return $decoded;
    }

    /**
     * The reference bytes for one fixture, or a failure naming it.
     *
     * The providers below enumerate the *corpus*, not this table, so a
     * fixture added to the corpus becomes a test case immediately. What
     * it must not do is become a silently passing one: an absent entry
     * fails here, naming the fixture and how to record it, rather than
     * skipping the only assertion that pins agreement with the
     * reference.
     */
    private static function referenceBytes(string $family, string $name): string
    {
        /** @var array<string, string> $table */
        $table = self::reference()[$family] ?? [];

        if (!isset($table[$name])) {
            self::fail(
                "{$family}/{$name} has no entry in tests/Fixtures/reference-encoding.json. "
                . 'Run the Go encoder under go/ over the fixture and record base64 of its '
                . 'exact CRLF output; never record this port\'s own bytes.',
            );
        }

        $bytes = base64_decode($table[$name], true);
        self::assertIsString($bytes, "{$family}/{$name} reference entry is not valid base64");

        return $bytes;
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function icsFixtures(): iterable
    {
        foreach (Corpus::names('rfc5545', 'ics') as $name) {
            yield $name => [$name];
        }
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function vcfFixtures(): iterable
    {
        foreach (Corpus::names('rfc6350', 'vcf') as $name) {
            yield $name => [$name];
        }
    }

    #[DataProvider('icsFixtures')]
    public function testCalendarEncodingMatchesTheReferenceByteForByte(string $name): void
    {
        $expected = self::referenceBytes('rfc5545', $name);

        $actual = IcsEncoder::encode(IcsParser::parse(Corpus::read("rfc5545/{$name}")));

        self::assertSame(
            bin2hex($expected),
            bin2hex($actual),
            "rfc5545/{$name} diverged from the reference encoding",
        );
    }

    #[DataProvider('vcfFixtures')]
    public function testCardEncodingMatchesTheReferenceByteForByte(string $name): void
    {
        $expected = self::referenceBytes('rfc6350', $name);

        $actual = '';

        foreach (VcfParser::parse(Corpus::read("rfc6350/{$name}")) as $card) {
            $actual .= VcfEncoder::encode($card);
        }

        self::assertSame(
            bin2hex($expected),
            bin2hex($actual),
            "rfc6350/{$name} diverged from the reference encoding",
        );
    }

    public function testTheReferenceTableHasNoEntriesTheCorpusDroppedFixturesFor(): void
    {
        // The providers guarantee the other direction: every corpus
        // fixture is asserted, and one without an entry fails in
        // `referenceBytes`. This catches the reverse -- a renamed or
        // deleted fixture leaving a dead entry behind, which no other
        // assertion would ever read.
        $ics = array_keys(self::reference()['rfc5545']);
        $vcf = array_keys(self::reference()['rfc6350']);
        sort($ics);
        sort($vcf);

        self::assertSame(Corpus::names('rfc5545', 'ics'), $ics);
        self::assertSame(Corpus::names('rfc6350', 'vcf'), $vcf);
    }

    /**
     * Folding is measured in octets, and the reference folds a logical
     * line longer than 75 bytes at exactly 75, then at 74 per
     * continuation (the leading SP costs one octet).
     */
    public function testFoldWidthIsSeventyFiveOctets(): void
    {
        $long = str_repeat('a', 200);
        $ics = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:{$long}\r\nEND:VCALENDAR\r\n";

        $encoded = IcsEncoder::encode(IcsParser::parse($ics));
        $lines = explode("\r\n", rtrim($encoded, "\r\n"));

        $prodid = array_values(array_filter(
            $lines,
            static fn (string $l): bool => str_starts_with($l, 'PRODID:'),
        ));
        self::assertCount(1, $prodid);
        self::assertSame(75, strlen($prodid[0]), 'first physical line is not exactly 75 octets');

        $index = array_search($prodid[0], $lines, true);
        self::assertIsInt($index);
        self::assertSame(75, strlen($lines[$index + 1]), 'continuation line is not exactly 75 octets');
        self::assertStringStartsWith(' ', $lines[$index + 1]);
    }

    /**
     * The fold width is 75 **octets**, not 75 characters.
     *
     * This is the trap the porting guide names for PHP: `strlen` is a
     * byte count and is right, `mb_strlen` counts characters and is
     * wrong. A codec that measures characters folds a non-ASCII line
     * late, every physical line runs past the RFC 5545 §3.1 limit, and
     * nothing in a round-trip notices -- the folded output unfolds back
     * to the same logical line either way. Only these bytes catch it.
     *
     * The expected bytes are the reference's own output, which splits the
     * two-octet `é` across the fold: the first physical line ends with
     * `\xc3` and the continuation opens with `\xa9`. That is deliberate
     * on the reference's part (RFC 5545 counts octets and a decoder
     * reassembles before interpreting runes), so a port that refused to
     * split a multi-byte sequence would produce different canonical bytes
     * and a different hash.
     */
    public function testFoldWidthIsMeasuredInOctetsNotCharacters(): void
    {
        $ics = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\nBEGIN:VTODO\r\nUID:u\r\nSUMMARY:"
            . str_repeat("\u{e9}", 60)
            . "\r\nEND:VTODO\r\nEND:VCALENDAR\r\n";

        $expected = base64_decode(
            'QkVHSU46VkNBTEVOREFSDQpWRVJTSU9OOjIuMA0KUFJPRElEOnANCkJFR0lOOlZUT0RPDQpVSUQ6'
            . 'dQ0KU1VNTUFSWTrDqcOpw6nDqcOpw6nDqcOpw6nDqcOpw6nDqcOpw6nDqcOpw6nDqcOpw6nDqcOp'
            . 'w6nDqcOpw6nDqcOpw6nDqcOpw6nDDQogqcOpw6nDqcOpw6nDqcOpw6nDqcOpw6nDqcOpw6nDqcOp'
            . 'w6nDqcOpw6nDqcOpw6nDqcOpw6kNCkVORDpWVE9ETw0KRU5EOlZDQUxFTkRBUg0K',
            true,
        );
        self::assertIsString($expected);

        $actual = IcsEncoder::encode(IcsParser::parse($ics));

        self::assertSame(
            bin2hex($expected),
            bin2hex($actual),
            'the fold is measured in characters, not octets',
        );

        // Stated a second way, so a failure says which half broke: no
        // physical line may exceed 75 bytes.
        foreach (explode("\r\n", rtrim($actual, "\r\n")) as $line) {
            self::assertLessThanOrEqual(
                75,
                strlen($line),
                'a physical line ran past 75 octets',
            );
        }
    }

    /**
     * A logical line of exactly 75 octets is not folded; 76 is.
     */
    public function testFoldBoundaryIsExclusiveAtSeventyFive(): void
    {
        foreach ([75 => 1, 76 => 2] as $width => $expectedPhysicalLines) {
            $value = str_repeat('a', $width - strlen('PRODID:'));
            $ics = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:{$value}\r\nEND:VCALENDAR\r\n";

            $lines = explode("\r\n", rtrim(IcsEncoder::encode(IcsParser::parse($ics)), "\r\n"));
            $prodidLines = array_values(array_filter(
                $lines,
                static fn (string $l): bool => str_starts_with($l, 'PRODID:') || str_starts_with($l, ' a'),
            ));

            self::assertCount(
                $expectedPhysicalLines,
                $prodidLines,
                "a {$width}-octet logical line folded into the wrong number of physical lines",
            );
        }
    }
}
