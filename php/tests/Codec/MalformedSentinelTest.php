<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Codec;

use HopTop\Vstar\Codec\Rfc5545\Parser as IcsParser;
use HopTop\Vstar\Codec\Rfc6350\Encoder as VcfEncoder;
use HopTop\Vstar\Codec\Rfc6350\Parser as VcfParser;
use HopTop\Vstar\Exception\AlreadyClosedException;
use HopTop\Vstar\Exception\HeaderLockedException;
use HopTop\Vstar\Exception\IterationCapException;
use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\MissingUidException;
use HopTop\Vstar\Exception\NoAnchorException;
use HopTop\Vstar\Exception\NoTriggerException;
use HopTop\Vstar\Exception\TargetCorruptedException;
use HopTop\Vstar\Exception\UnboundedExpansionException;
use HopTop\Vstar\Exception\UnclosedBlockException;
use HopTop\Vstar\Exception\UnsupportedRRuleException;
use HopTop\Vstar\Exception\UnsupportedVersionException;
use HopTop\Vstar\Exception\VstarException;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * The `malformed/` gate: failing is not enough, failing with the sentinel
 * the `.error` sibling names is the gate.
 */
final class MalformedSentinelTest extends TestCase
{
    /**
     * @return iterable<string, array{string, string}>
     */
    public static function malformedFixtures(): iterable
    {
        foreach (['ics', 'vcf'] as $extension) {
            foreach (\HopTop\Vstar\Tests\Corpus::names('malformed', $extension) as $name) {
                yield $name => [$name, $extension];
            }
        }
    }

    #[DataProvider('malformedFixtures')]
    public function testFixtureFailsWithItsNamedSentinel(string $name, string $extension): void
    {
        $stem = substr($name, 0, -(strlen($extension) + 1));
        $expected = \HopTop\Vstar\Tests\Corpus::sentinel($stem);
        $input = \HopTop\Vstar\Tests\Corpus::read("malformed/{$name}");

        if ($extension === 'ics') {
            self::assertSentinel($expected, static function () use ($input): void {
                IcsParser::parse($input);
            });

            return;
        }

        // vCard fixtures may target either the parser or the encoder.
        // ErrMissingUID is encoder-only at v1.0, so a fixture whose parse
        // succeeds must still be refused on re-encode.
        try {
            $cards = VcfParser::parse($input);
        } catch (VstarException $e) {
            self::assertSame($expected, $e->sentinel(), 'parse raised the wrong sentinel');

            return;
        }

        self::assertNotSame([], $cards, 'parse produced no cards: cannot reach the encoder');

        foreach ($cards as $card) {
            try {
                VcfEncoder::encode($card);
            } catch (VstarException $e) {
                self::assertSame($expected, $e->sentinel(), 'encode raised the wrong sentinel');

                return;
            }
        }

        self::fail("expected {$expected} during parse or encode; both succeeded");
    }

    /**
     * Every one of the twelve sentinels exists and reports the Go
     * identifier the corpus asserts on.
     *
     * @return iterable<string, array{class-string<VstarException>, string}>
     */
    public static function sentinelClasses(): iterable
    {
        yield 'malformed' => [MalformedException::class, 'ErrMalformed'];
        yield 'unclosed block' => [UnclosedBlockException::class, 'ErrUnclosedBlock'];
        yield 'unsupported version' => [UnsupportedVersionException::class, 'ErrUnsupportedVersion'];
        yield 'missing uid' => [MissingUidException::class, 'ErrMissingUID'];
        yield 'unsupported rrule' => [UnsupportedRRuleException::class, 'ErrUnsupportedRRule'];
        yield 'iteration cap' => [IterationCapException::class, 'ErrIterationCap'];
        yield 'unbounded expansion' => [UnboundedExpansionException::class, 'ErrUnboundedExpansion'];
        yield 'target corrupted' => [TargetCorruptedException::class, 'ErrTargetCorrupted'];
        yield 'already closed' => [AlreadyClosedException::class, 'ErrAlreadyClosed'];
        yield 'header locked' => [HeaderLockedException::class, 'ErrHeaderLocked'];
        yield 'no trigger' => [NoTriggerException::class, 'ErrNoTrigger'];
        yield 'no anchor' => [NoAnchorException::class, 'ErrNoAnchor'];
    }

    /**
     * @param class-string<VstarException> $class
     */
    #[DataProvider('sentinelClasses')]
    public function testSentinelIdentifierIsTheGoSpelling(string $class, string $identifier): void
    {
        $e = new $class('context');

        self::assertInstanceOf(VstarException::class, $e);
        self::assertSame($identifier, $e->sentinel());
        self::assertStringContainsString('context', $e->getMessage());
    }

    public function testStrayEndVcardIsMalformed(): void
    {
        self::assertSentinel('ErrMalformed', static function (): void {
            VcfParser::parse("END:VCARD\r\n");
        });
    }

    public function testNestedBeginVcardIsMalformed(): void
    {
        self::assertSentinel('ErrMalformed', static function (): void {
            VcfParser::parse("BEGIN:VCARD\r\nBEGIN:VCARD\r\n");
        });
    }

    public function testVcardWithoutVersionIsMalformed(): void
    {
        self::assertSentinel('ErrMalformed', static function (): void {
            VcfParser::parse("BEGIN:VCARD\r\nUID:u\r\nEND:VCARD\r\n");
        });
    }

    public function testUnclosedVcardIsUnclosedBlock(): void
    {
        self::assertSentinel('ErrUnclosedBlock', static function (): void {
            VcfParser::parse(\HopTop\Vstar\Tests\Corpus::read('fuzz-seed/rfc6350/seed_11_only_begin.bytes'));
        });
    }

    public function testVcardVersionThreeIsUnsupported(): void
    {
        self::assertSentinel('ErrUnsupportedVersion', static function (): void {
            VcfParser::parse(\HopTop\Vstar\Tests\Corpus::read('fuzz-seed/rfc6350/seed_12_v3_unsupported.bytes'));
        });
    }

    public function testCalendarVersionOtherThanTwoIsUnsupported(): void
    {
        self::assertSentinel('ErrUnsupportedVersion', static function (): void {
            IcsParser::parse("BEGIN:VCALENDAR\r\nVERSION:1.0\r\nEND:VCALENDAR\r\n");
        });
    }

    public function testEmptyCalendarInputIsMalformed(): void
    {
        self::assertSentinel('ErrMalformed', static function (): void {
            IcsParser::parse('');
        });
    }

    public function testContentLineWithoutAColonIsMalformed(): void
    {
        self::assertSentinel('ErrMalformed', static function (): void {
            IcsParser::parse("BEGIN:VCALENDAR\r\nNOCOLON\r\nEND:VCALENDAR\r\n");
        });
    }

    public function testUnbalancedQuoteInAParameterIsMalformed(): void
    {
        self::assertSentinel('ErrMalformed', static function (): void {
            IcsParser::parse("BEGIN:VCALENDAR\r\nX-A;P=\"open:v\r\nEND:VCALENDAR\r\n");
        });
    }

    public function testParameterWithoutAnEqualsIsMalformed(): void
    {
        self::assertSentinel('ErrMalformed', static function (): void {
            IcsParser::parse("BEGIN:VCALENDAR\r\nX-A;BARE:v\r\nEND:VCALENDAR\r\n");
        });
    }

    public function testMismatchedEndNameIsMalformed(): void
    {
        self::assertSentinel('ErrMalformed', static function (): void {
            IcsParser::parse("BEGIN:VCALENDAR\r\nBEGIN:VTODO\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n");
        });
    }

    private static function assertSentinel(string $expected, callable $fn): void
    {
        try {
            $fn();
        } catch (VstarException $e) {
            self::assertSame($expected, $e->sentinel());

            return;
        }

        self::fail("expected {$expected}; nothing was raised");
    }
}
