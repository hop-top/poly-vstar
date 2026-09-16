<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Codec;

use HopTop\Vstar\Codec\Rfc5545\Encoder as IcsEncoder;
use HopTop\Vstar\Codec\Rfc5545\Parser as IcsParser;
use HopTop\Vstar\Codec\Rfc6350\Encoder as VcfEncoder;
use HopTop\Vstar\Codec\Rfc6350\Parser as VcfParser;
use HopTop\Vstar\Exception\VstarException;
use HopTop\Vstar\Tests\Corpus;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * The fuzz seeds are adversarial byte sequences. The contract is narrow
 * and absolute: a codec either succeeds or raises a `VstarException`.
 *
 * A `TypeError`, a `ValueError`, a `DivisionByZeroError`, an
 * `ErrorException` escalated from a PHP warning or notice, or any other
 * `Throwable` is a defect -- the failure carries no sentinel, so a caller
 * cannot dispatch on it and the corpus cannot assert it.
 */
final class FuzzSeedTest extends TestCase
{
    protected function setUp(): void
    {
        // Promote every warning and notice to an exception so a silent
        // "Undefined array key" cannot pass for a clean parse.
        set_error_handler(
            static function (int $severity, string $message, string $file, int $line): bool {
                throw new \ErrorException($message, 0, $severity, $file, $line);
            },
        );
    }

    protected function tearDown(): void
    {
        restore_error_handler();
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function seeds(): iterable
    {
        foreach (Corpus::fuzzSeeds() as $relative) {
            yield $relative => [$relative];
        }
    }

    #[DataProvider('seeds')]
    public function testSeedNeverEscapesTheVstarExceptionHierarchy(string $relative): void
    {
        $input = Corpus::read($relative);
        $isCalendar = str_contains($relative, '/rfc5545/');
        $outcome = 'parsed';

        try {
            if ($isCalendar) {
                $cal = IcsParser::parse($input);
                // Re-encoding is part of the surface under test: an encoder
                // fed a parser's own output must not throw off-contract
                // either.
                IcsParser::parse(IcsEncoder::encode($cal));
            } else {
                foreach (VcfParser::parse($input) as $card) {
                    if ($card->uid !== '') {
                        VcfParser::parse(VcfEncoder::encode($card));
                    }
                }
            }
        } catch (VstarException $e) {
            $outcome = $e->sentinel();
            self::assertNotSame('', $outcome);
        } catch (\Throwable $e) {
            self::fail(sprintf(
                '%s escaped as %s: %s',
                $relative,
                $e::class,
                $e->getMessage(),
            ));
        }

        self::assertContains(
            $outcome,
            ['parsed', 'ErrMalformed', 'ErrUnclosedBlock', 'ErrUnsupportedVersion', 'ErrMissingUID'],
            "{$relative} produced an outcome no codec-layer seed should reach",
        );
    }

    /**
     * Truncations of every seed at every byte boundary: the cheapest way
     * to reach the partial-input paths a corpus of whole documents cannot.
     */
    #[DataProvider('seeds')]
    public function testTruncationsOfTheSeedStayOnContract(string $relative): void
    {
        $input = Corpus::read($relative);
        $isCalendar = str_contains($relative, '/rfc5545/');
        $checked = 0;

        for ($i = 0, $n = strlen($input); $i <= $n; $i++) {
            $slice = substr($input, 0, $i);
            $checked++;

            try {
                if ($isCalendar) {
                    IcsParser::parse($slice);
                } else {
                    VcfParser::parse($slice);
                }
            } catch (VstarException) {
                continue;
            } catch (\Throwable $e) {
                self::fail(sprintf(
                    '%s truncated to %d bytes escaped as %s: %s',
                    $relative,
                    $i,
                    $e::class,
                    $e->getMessage(),
                ));
            }
        }

        self::assertSame(strlen($input) + 1, $checked, 'not every truncation was exercised');
    }
}
