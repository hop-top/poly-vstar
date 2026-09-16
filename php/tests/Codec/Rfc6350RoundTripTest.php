<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Codec;

use HopTop\Vstar\Card;
use HopTop\Vstar\Codec\Rfc6350\Encoder;
use HopTop\Vstar\Codec\Rfc6350\Parser;
use HopTop\Vstar\Exception\MissingUidException;
use HopTop\Vstar\Kind;
use HopTop\Vstar\Tests\Corpus;
use HopTop\Vstar\Vstar;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

#[CoversClass(Parser::class)]
#[CoversClass(Encoder::class)]
final class Rfc6350RoundTripTest extends TestCase
{
    /**
     * @return iterable<string, array{string}>
     */
    public static function vcfFixtures(): iterable
    {
        foreach (Corpus::names('rfc6350', 'vcf') as $name) {
            yield $name => [$name];
        }
    }

    #[DataProvider('vcfFixtures')]
    public function testParseEncodeParseIsSemanticallyStable(string $name): void
    {
        $input = Corpus::read("rfc6350/{$name}");

        $first = Parser::parse($input);
        self::assertNotSame([], $first, 'fixture parsed to zero cards');

        $encoded = '';

        foreach ($first as $card) {
            $encoded .= Encoder::encode($card);
        }

        $second = Parser::parse($encoded);
        self::assertCardListsEqual($first, $second);
    }

    #[DataProvider('vcfFixtures')]
    public function testEncoderEmitsCrlfOnly(string $name): void
    {
        foreach (Parser::parse(Corpus::read("rfc6350/{$name}")) as $card) {
            $encoded = Encoder::encode($card);
            self::assertSame(
                substr_count($encoded, "\n"),
                substr_count($encoded, "\r\n"),
                'encoder emitted a bare LF',
            );
            self::assertStringStartsWith("BEGIN:VCARD\r\n", $encoded);
            self::assertStringEndsWith("END:VCARD\r\n", $encoded);
        }
    }

    #[DataProvider('vcfFixtures')]
    public function testNoPhysicalLineExceedsSeventyFiveOctets(string $name): void
    {
        foreach (Parser::parse(Corpus::read("rfc6350/{$name}")) as $card) {
            foreach (explode("\r\n", rtrim(Encoder::encode($card), "\r\n")) as $line) {
                self::assertLessThanOrEqual(75, strlen($line), "over-long physical line: {$line}");
            }
        }
    }

    public function testParseReturnsAListOfCardsNotASingleCard(): void
    {
        $cards = Parser::parse(Corpus::read('fuzz-seed/rfc6350/seed_08_two_cards.bytes'));

        self::assertCount(2, $cards);
        self::assertNotSame($cards[0]->uid, $cards[1]->uid);
    }

    public function testVersionAndUidAreLiftedOffThePropertyList(): void
    {
        $cards = Parser::parse(Corpus::read('rfc6350/minimal.vcf'));

        self::assertCount(1, $cards);
        self::assertNotSame('', $cards[0]->uid);
        self::assertNull($cards[0]->get('UID'), 'UID must be lifted onto Card::$uid');
        self::assertNull($cards[0]->get('VERSION'), 'VERSION must not reach Card::$props');
    }

    public function testKindIsLiftedAndLowercased(): void
    {
        $cards = Parser::parse(Corpus::read('rfc6350/kind_group.vcf'));

        self::assertSame(Kind::Group, $cards[0]->kind);
        self::assertNull($cards[0]->get('KIND'), 'KIND must be lifted onto Card::$kind');
    }

    public function testGroupPrefixesKeepTheirCaseWhileNamesUppercase(): void
    {
        $cards = Parser::parse(Corpus::read('rfc6350/grouped.vcf'));
        $encoded = Encoder::encode($cards[0]);

        self::assertStringContainsString("home.TEL;TYPE=voice:", $encoded);
        self::assertStringContainsString("work.EMAIL:", $encoded);
    }

    public function testParserPreservesPropertyWireOrder(): void
    {
        $cards = Parser::parse(Corpus::read('rfc6350/kind_group.vcf'));
        $names = array_map(
            static fn (object $p): string => $p->name,
            $cards[0]->props,
        );

        self::assertSame(['FN', 'MEMBER', 'MEMBER', 'MEMBER'], $names);
    }

    public function testParserAcceptsAUidLessVcard(): void
    {
        // MissingUID is encoder-only in v0.1: the parser is permissive.
        $cards = Parser::parse(Corpus::read('malformed/missing_uid.vcf'));

        self::assertCount(1, $cards);
        self::assertSame('', $cards[0]->uid);
    }

    public function testEncoderRefusesAUidLessCard(): void
    {
        $card = new Card();

        $this->expectException(MissingUidException::class);
        Encoder::encode($card);
    }

    public function testEncoderRefusesAUidLessCardParsedFromTheCorpus(): void
    {
        $cards = Parser::parse(Corpus::read('malformed/missing_uid.vcf'));

        $this->expectException(MissingUidException::class);
        Encoder::encode($cards[0]);
    }

    public function testParserAcceptsLfOnlyInput(): void
    {
        $lf = Corpus::read('fuzz-seed/rfc6350/seed_02_lf_only.bytes');
        self::assertStringNotContainsString("\r", $lf, 'seed is not LF-only');

        self::assertCount(1, Parser::parse($lf));
    }

    public function testEmptyInputParsesToNoCards(): void
    {
        self::assertSame([], Parser::parse(''));
    }

    /**
     * @param list<Card> $a
     * @param list<Card> $b
     */
    private static function assertCardListsEqual(array $a, array $b): void
    {
        self::assertCount(count($a), $b, 'card count diverged');

        foreach ($a as $i => $left) {
            $right = $b[$i];
            self::assertSame($left->uid, $right->uid, "card {$i} UID diverged");
            self::assertSame($left->kind, $right->kind, "card {$i} KIND diverged");
            self::assertCount(count($left->props), $right->props, "card {$i} property count diverged");

            foreach ($left->props as $j => $prop) {
                self::assertTrue(
                    Vstar::propertyEqual($prop, $right->props[$j]),
                    "property {$j} of card {$i} diverged: {$prop->name}",
                );
            }
        }
    }
}
