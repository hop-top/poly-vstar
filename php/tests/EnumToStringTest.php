<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests;

use HopTop\Vstar\CompType;
use HopTop\Vstar\Diff\DiffOp;
use HopTop\Vstar\Duration\Related;
use HopTop\Vstar\EventStatus;
use HopTop\Vstar\Ext\Scope;
use HopTop\Vstar\JournalStatus;
use HopTop\Vstar\Kind;
use HopTop\Vstar\Rrule\Freq;
use HopTop\Vstar\Rrule\RecurrenceRange;
use HopTop\Vstar\Rrule\Weekday;
use HopTop\Vstar\TodoStatus;
use HopTop\Vstar\Transp;
use HopTop\Vstar\Validate\Severity;
use HopTop\Vstar\VClass;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * Every enum in the package spells its display method `toString()`.
 *
 * PHP rejects `__toString()` on an enum at declaration time, so the
 * reference's `String()` lands as a plain method on the enums that have
 * one. The enums the reference gives no `String()` carry the same
 * accessor anyway, returning the wire token, so a caller never has to
 * remember which enums are stringable.
 *
 * The calls go through reflection because the enums share no interface
 * that declares `toString()`; the test is precisely that each declares
 * it on its own.
 */
final class EnumToStringTest extends TestCase
{
    /**
     * Every string-backed enum in the package.
     *
     * @return iterable<string, array{class-string<\BackedEnum>}>
     */
    public static function everyEnum(): iterable
    {
        foreach ([
            CompType::class,
            Kind::class,
            VClass::class,
            Transp::class,
            EventStatus::class,
            TodoStatus::class,
            JournalStatus::class,
            DiffOp::class,
            Related::class,
            Scope::class,
            Freq::class,
            RecurrenceRange::class,
            Weekday::class,
            Severity::class,
        ] as $enum) {
            yield $enum => [$enum];
        }
    }

    /**
     * The enums whose `toString()` is the wire token itself: the
     * reference gives them no `String()`, so there is no display
     * spelling to diverge from `->value`.
     *
     * @return iterable<string, array{class-string<\BackedEnum>}>
     */
    public static function wireValueEnums(): iterable
    {
        foreach ([
            CompType::class,
            Kind::class,
            VClass::class,
            Transp::class,
            EventStatus::class,
            TodoStatus::class,
            JournalStatus::class,
        ] as $enum) {
            yield $enum => [$enum];
        }
    }

    /**
     * @param class-string<\BackedEnum> $enum
     */
    #[DataProvider('everyEnum')]
    public function testEveryEnumDeclaresAPublicToString(string $enum): void
    {
        self::assertTrue(method_exists($enum, 'toString'), "{$enum} lacks toString()");

        $method = new \ReflectionMethod($enum, 'toString');

        self::assertTrue($method->isPublic(), "{$enum}::toString() is not public");
        self::assertSame('string', (string) $method->getReturnType(), "{$enum}::toString() does not return string");

        foreach ($enum::cases() as $case) {
            self::assertIsString($method->invoke($case), "{$enum}::{$case->name}");
        }
    }

    /**
     * @param class-string<\BackedEnum> $enum
     */
    #[DataProvider('wireValueEnums')]
    public function testToStringIsTheWireValue(string $enum): void
    {
        $method = new \ReflectionMethod($enum, 'toString');

        foreach ($enum::cases() as $case) {
            self::assertSame($case->value, $method->invoke($case), "{$enum}::{$case->name}");
        }
    }
}
