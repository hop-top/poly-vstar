<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Codec;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Codec\Rfc5545\Encoder;
use HopTop\Vstar\Codec\Rfc5545\Parser;
use HopTop\Vstar\Component;
use HopTop\Vstar\Tests\Corpus;
use HopTop\Vstar\Vstar;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

#[CoversClass(Parser::class)]
#[CoversClass(Encoder::class)]
final class Rfc5545RoundTripTest extends TestCase
{
    /**
     * @return iterable<string, array{string}>
     */
    public static function icsFixtures(): iterable
    {
        foreach (Corpus::names('rfc5545', 'ics') as $name) {
            yield $name => [$name];
        }
    }

    #[DataProvider('icsFixtures')]
    public function testParseEncodeParseIsSemanticallyStable(string $name): void
    {
        $input = Corpus::read("rfc5545/{$name}");

        $first = Parser::parse($input);
        $encoded = Encoder::encode($first);
        $second = Parser::parse($encoded);

        self::assertSame($first->prodId, $second->prodId, 'PRODID diverged');
        self::assertCalendarsEqual($first, $second);
    }

    #[DataProvider('icsFixtures')]
    public function testEncoderEmitsCrlfOnly(string $name): void
    {
        $encoded = Encoder::encode(Parser::parse(Corpus::read("rfc5545/{$name}")));

        // Every LF must be preceded by a CR: a bare LF would be repaired
        // invisibly by the CRLF->LF comparison transform.
        self::assertSame(
            substr_count($encoded, "\n"),
            substr_count($encoded, "\r\n"),
            'encoder emitted a bare LF',
        );
        self::assertStringEndsWith("\r\n", $encoded);
    }

    #[DataProvider('icsFixtures')]
    public function testNoPhysicalLineExceedsSeventyFiveOctets(string $name): void
    {
        $encoded = Encoder::encode(Parser::parse(Corpus::read("rfc5545/{$name}")));

        foreach (explode("\r\n", rtrim($encoded, "\r\n")) as $line) {
            self::assertLessThanOrEqual(75, strlen($line), "over-long physical line: {$line}");
        }
    }

    public function testParserPreservesPropertyWireOrder(): void
    {
        $cal = Parser::parse(Corpus::read('rfc5545/vevent_status_class_transp.ics'));

        self::assertCount(1, $cal->components);
        $names = array_map(
            static fn (object $p): string => $p->name,
            $cal->components[0]->props,
        );

        // The fixture's own order, not an alphabetical one. A parser that
        // sorts passes every round-trip test and fails right here.
        self::assertSame(
            ['UID', 'DTSTAMP', 'DTSTART', 'DTEND', 'SUMMARY', 'STATUS', 'CLASS', 'TRANSP'],
            $names,
        );
    }

    public function testParserAcceptsLfOnlyInput(): void
    {
        $lf = Corpus::read('fuzz-seed/rfc5545/seed_02_lf_only.bytes');
        self::assertStringNotContainsString("\r", $lf, 'seed is not LF-only');

        $cal = Parser::parse($lf);
        self::assertNotSame('', $cal->prodId);
    }

    public function testParserAcceptsMixedLineEndings(): void
    {
        // The seed alternates CRLF and bare LF within one document.
        $cal = Parser::parse(Corpus::read('fuzz-seed/rfc5545/seed_12_mixed_eol.bytes'));
        self::assertSame('p', $cal->prodId);
    }

    public function testScannerUnfoldsAcrossPhysicalLines(): void
    {
        $cal = Parser::parse(Corpus::read('fuzz-seed/rfc5545/seed_04_folded_long_summary.bytes'));

        $summary = $cal->components[0]->get('SUMMARY');
        self::assertNotNull($summary);
        self::assertStringContainsString('across multiple lines', $summary->value);
        self::assertStringNotContainsString("\n", $summary->value);
    }

    public function testEncodeComponentOmitsTheCalendarWrapper(): void
    {
        $cal = Parser::parse(Corpus::read('rfc5545/one_vtodo.ics'));
        $bytes = Encoder::encodeComponent($cal->components[0]);

        self::assertStringStartsWith("BEGIN:VTODO\r\n", $bytes);
        self::assertStringEndsWith("END:VTODO\r\n", $bytes);
        self::assertStringNotContainsString('VCALENDAR', $bytes);
    }

    public function testQuotedParameterValuesSurviveTheRoundTrip(): void
    {
        $cal = Parser::parse(Corpus::read('fuzz-seed/rfc5545/seed_11_param_with_quote.bytes'));
        $reparsed = Parser::parse(Encoder::encode($cal));

        self::assertCalendarsEqual($cal, $reparsed);
    }

    public function testValueDateConstantsMatchTheWire(): void
    {
        self::assertSame('VALUE', Vstar::VALUE_PARAM);
        self::assertSame('DATE', Vstar::VALUE_DATE);
    }

    private static function assertCalendarsEqual(Calendar $a, Calendar $b): void
    {
        self::assertSame($a->prodId, $b->prodId);
        self::assertComponentListsEqual($a->components, $b->components);
    }

    /**
     * @param list<Component> $a
     * @param list<Component> $b
     */
    private static function assertComponentListsEqual(array $a, array $b): void
    {
        self::assertCount(count($a), $b, 'component count diverged');

        foreach ($a as $i => $left) {
            $right = $b[$i];
            self::assertSame($left->type, $right->type);
            self::assertCount(count($left->props), $right->props, 'property count diverged');

            foreach ($left->props as $j => $prop) {
                self::assertTrue(
                    Vstar::propertyEqual($prop, $right->props[$j]),
                    "property {$j} of component {$i} diverged: {$prop->name}",
                );
            }

            self::assertComponentListsEqual($left->sub, $right->sub);
        }
    }
}
