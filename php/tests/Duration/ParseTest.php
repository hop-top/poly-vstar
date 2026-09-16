<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Duration;

use HopTop\Vstar\Duration\Duration;
use HopTop\Vstar\Duration\VDuration;
use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Tests\Behavior;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * RFC 5545 §3.3.6 DURATION parsing, formatting and arithmetic.
 *
 * `spec/behavior/duration/parse.json` is the gate. Two of its columns are
 * easy to misread:
 *
 * - `seconds` is the **nominal** length with days as 24h and weeks as 7
 *   days -- what `signed()` reports, not what an anchored `addTo` would.
 * - `negative` is `isNegative()`, NOT the struct's sign flag. `-PT0S`
 *   parses with the flag set and reports `negative: false`, because a
 *   zero-length duration is never negative however it was authored.
 */
final class ParseTest extends TestCase
{
    /**
     * @return iterable<string, array{string, ?int, ?bool, ?string}>
     */
    public static function behaviorCases(): iterable
    {
        foreach (Behavior::cases('duration/parse.json') as $case) {
            /** @var string $value */
            $value = $case['value'];
            $seconds = null;

            if (isset($case['seconds']) && is_int($case['seconds'])) {
                $seconds = $case['seconds'];
            }

            $negative = null;

            if (isset($case['negative']) && is_bool($case['negative'])) {
                $negative = $case['negative'];
            }

            $error = null;

            if (isset($case['error']) && is_string($case['error'])) {
                $error = $case['error'];
            }

            yield ($value === '' ? '(empty)' : $value) => [$value, $seconds, $negative, $error];
        }
    }

    #[DataProvider('behaviorCases')]
    public function testParseMatchesTheBehaviorCorpus(
        string $value,
        ?int $seconds,
        ?bool $negative,
        ?string $error,
    ): void {
        if ($error !== null) {
            try {
                Duration::parse($value);
                self::fail("parsing {$value} should have failed with {$error}");
            } catch (MalformedException $e) {
                self::assertSame($error, $e->sentinel());
            }

            self::assertFalse(Duration::valid($value));

            return;
        }

        $d = Duration::parse($value);

        self::assertSame($seconds, $d->totalSeconds(), 'nominal second count');
        self::assertSame($negative, $d->isNegative(), 'isNegative(), not the sign flag');
        self::assertTrue(Duration::valid($value));
    }

    /**
     * The `negative` column is `isNegative()`. `-PT0S` carries the sign
     * flag and still reports false, because a zero-length duration is not
     * subtractive -- there is nothing to subtract.
     */
    public function testANegativeZeroIsNotNegative(): void
    {
        $d = Duration::parse('-PT0S');

        self::assertTrue($d->negative, 'the authored sign is preserved on the struct');
        self::assertFalse($d->isNegative(), 'but a zero-length duration is never negative');
        self::assertSame(0, $d->totalSeconds());
        self::assertSame('PT0S', (string) $d, 'and the sign is not re-emitted');
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function roundTrips(): iterable
    {
        foreach ([
            'P1W', 'P26W', 'P7D', 'PT1H', 'PT15M', 'PT30S', 'P1DT2H30M45S',
            'PT1H30M', '-PT15M', '-P1DT2H', 'PT0S', 'P0D', 'P1D', 'PT24H',
            'PT1M30S', '-P2W', 'P0DT1H',
        ] as $value) {
            yield $value => [$value];
        }
    }

    /**
     * Parse and format round-trip byte for byte.
     */
    #[DataProvider('roundTrips')]
    public function testParseAndFormatRoundTrip(string $value): void
    {
        self::assertSame($value, (string) Duration::parse($value));
    }

    /**
     * The one intentional normalization: an explicit `+` is dropped, since
     * a positive duration is the default.
     */
    public function testAnExplicitPlusSignIsDroppedOnFormat(): void
    {
        self::assertSame('PT15M', (string) Duration::parse('+PT15M'));
    }

    /**
     * `dayForm` distinguishes `P0D` from `PT0S`. Both are numerically zero
     * and every unit field is zero in both, so without the flag the
     * authored spelling could not be reproduced -- and rule 12 preserves
     * DURATION values verbatim, so the two hash differently.
     */
    public function testDayFormDistinguishesP0dFromPt0s(): void
    {
        $dayForm = Duration::parse('P0D');
        $zero = Duration::parse('PT0S');

        self::assertTrue($dayForm->dayForm);
        self::assertFalse($zero->dayForm);
        self::assertSame($zero->totalSeconds(), $dayForm->totalSeconds());
        self::assertNotSame((string) $zero, (string) $dayForm);
        self::assertSame('P0D', (string) $dayForm);
        self::assertSame('PT0S', (string) $zero);
    }

    /**
     * The flag affects formatting only.
     */
    public function testDayFormDoesNotAffectSignedIsNegativeOrAddTo(): void
    {
        $dayForm = Duration::parse('P0D');
        $zero = Duration::parse('PT0S');
        $anchor = new \DateTimeImmutable('2026-05-04T12:00:00', new \DateTimeZone('UTC'));

        self::assertSame($zero->totalSeconds(), $dayForm->totalSeconds());
        self::assertSame($zero->isNegative(), $dayForm->isNegative());
        self::assertSame(
            $zero->addTo($anchor)->getTimestamp(),
            $dayForm->addTo($anchor)->getTimestamp(),
        );
    }

    /**
     * `P1D` and `PT24H` are numerically equal on a UTC anchor and remain
     * two distinct values -- which is the whole reason the authored units
     * are preserved.
     */
    public function testP1dAndPt24hAreDistinctValuesOfEqualNominalLength(): void
    {
        $day = Duration::parse('P1D');
        $hours = Duration::parse('PT24H');

        self::assertSame($hours->totalSeconds(), $day->totalSeconds());
        self::assertNotSame((string) $hours, (string) $day);
    }

    public function testAddToAdvancesAnInstant(): void
    {
        $anchor = new \DateTimeImmutable('2026-06-01T09:00:00', new \DateTimeZone('UTC'));

        self::assertSame(
            '2026-06-02T11:30:45+00:00',
            Duration::parse('P1DT2H30M45S')->addTo($anchor)->format('c'),
        );
    }

    public function testAddToSubtractsForANegativeDuration(): void
    {
        $anchor = new \DateTimeImmutable('2026-06-01T09:00:00', new \DateTimeZone('UTC'));

        self::assertSame(
            '2026-06-01T08:45:00+00:00',
            Duration::parse('-PT15M')->addTo($anchor)->format('c'),
        );
    }

    /**
     * The sign applies to the WHOLE duration, calendar part included.
     */
    public function testANegativeSignAppliesToBothTheCalendarAndClockParts(): void
    {
        $anchor = new \DateTimeImmutable('2026-06-03T12:00:00', new \DateTimeZone('UTC'));

        self::assertSame(
            '2026-06-02T10:00:00+00:00',
            Duration::parse('-P1DT2H')->addTo($anchor)->format('c'),
        );
    }

    /**
     * `addTo` never mutates its anchor: the immutable type is used
     * throughout, so a repeat-alarm loop advances from the same base each
     * time rather than compounding.
     */
    public function testAddToDoesNotMutateItsAnchor(): void
    {
        $anchor = new \DateTimeImmutable('2026-06-01T09:00:00', new \DateTimeZone('UTC'));
        $before = $anchor->getTimestamp();

        Duration::parse('P1D')->addTo($anchor);

        self::assertSame($before, $anchor->getTimestamp());
    }

    public function testSignedReportsTheNominalLengthWithItsInvertFlag(): void
    {
        $positive = Duration::parse('PT1H30M')->signed();
        self::assertSame(0, $positive->invert);
        self::assertSame(5400, $positive->s);

        $negative = Duration::parse('-PT15M')->signed();
        self::assertSame(1, $negative->invert);
        self::assertSame(900, $negative->s);
    }

    public function testSignedIsNotInvertedForAZeroLengthNegative(): void
    {
        self::assertSame(0, Duration::parse('-PT0S')->signed()->invert);
    }

    /**
     * `fromSigned` never uses the week or day units: an elapsed interval
     * carries no calendar information, so emitting `P1D` from 24 hours
     * would invent a distinction the input never made.
     */
    public function testFromSignedNeverEmitsCalendarUnits(): void
    {
        $day = new \DateInterval('PT86400S');

        self::assertSame('PT24H', (string) Duration::fromSigned($day));
    }

    public function testFromSignedRoundTripsATimeOnlyValue(): void
    {
        self::assertSame('PT1H30M45S', (string) Duration::fromSigned(new \DateInterval('PT5445S')));
    }

    public function testFromSignedHonoursTheInvertFlag(): void
    {
        $i = new \DateInterval('PT900S');
        $i->invert = 1;

        $d = Duration::fromSigned($i);

        self::assertTrue($d->isNegative());
        self::assertSame('-PT15M', (string) $d);
    }

    public function testFromSignedRendersZeroAsThePt0sSpelling(): void
    {
        self::assertSame('PT0S', (string) Duration::fromSigned(new \DateInterval('PT0S')));
    }

    /**
     * A directly constructed zero is a valid, positive, zero-length value
     * that renders as `PT0S` -- never the empty string and never a bare
     * `P`, both of which parse rejects.
     */
    public function testTheDefaultConstructedValueIsTheZeroDuration(): void
    {
        $d = new VDuration();

        self::assertSame('PT0S', (string) $d);
        self::assertSame(0, $d->totalSeconds());
        self::assertFalse($d->isNegative());
        self::assertTrue(Duration::valid((string) $d));
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function additionalRejections(): iterable
    {
        yield 'ISO 8601 years' => ['P1Y'];
        yield 'ISO 8601 months outside a time part' => ['P1M'];
        yield 'fractional seconds' => ['PT1.5S'];
        yield 'per-component sign' => ['PT-15M'];
        yield 'leading whitespace' => [' PT15M'];
        yield 'trailing whitespace' => ['PT15M '];
        yield 'internal whitespace' => ['PT15 M'];
        yield 'bare sign' => ['-'];
        yield 'sign with no P' => ['-T15M'];
        yield 'empty time part' => ['P1DT'];
        yield 'repeated hour unit' => ['PT1H2H'];
        yield 'seconds before minutes' => ['PT30S15M'];
        yield 'weeks after days' => ['P1D1W'];
        yield 'lowercase week designator' => ['P1w'];
        yield 'trailing junk' => ['PT15MX'];
    }

    #[DataProvider('additionalRejections')]
    public function testParseRejects(string $value): void
    {
        self::assertFalse(Duration::valid($value));

        $this->expectException(MalformedException::class);
        Duration::parse($value);
    }

    public function testTheRejectionSentinelIsTheReferenceSpelling(): void
    {
        try {
            Duration::parse('P1W2D');
            self::fail('P1W2D should not parse');
        } catch (MalformedException $e) {
            self::assertSame('ErrMalformed', $e->sentinel());
        }
    }
}
