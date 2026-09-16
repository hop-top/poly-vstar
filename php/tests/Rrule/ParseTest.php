<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Rrule;

use HopTop\Vstar\Exception\VstarException;
use HopTop\Vstar\Rrule\ByDay;
use HopTop\Vstar\Rrule\Freq;
use HopTop\Vstar\Rrule\Rrule;
use HopTop\Vstar\Rrule\Rule;
use HopTop\Vstar\Rrule\Weekday;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * RFC 5545 §3.3.10 RRULE parsing and the wire form, gated by
 * `spec/v0.1/conformance/rrule/`.
 *
 * Three fixture classes meet here:
 *
 * - `happy/`, `bounds/` and `by-clauses/` carry **no sidecar** and pin
 *   exactly one thing: the value parses. Seventeen of them.
 * - `rejected/` carries a `.expect.json` naming the sentinel, and the
 *   gate is that identifier -- "it failed" is not the assertion.
 * - `format/` carries a `.formatted` sibling holding the wire form the
 *   emitter must produce.
 */
final class ParseTest extends TestCase
{
    /**
     * The three subdirectories whose fixtures pin only "this parses".
     *
     * @return iterable<string, array{string}>
     */
    public static function parsingFixtures(): iterable
    {
        foreach (['happy', 'bounds', 'by-clauses'] as $subdir) {
            foreach (Sidecar::rruleStems($subdir) as $stem) {
                yield $stem => [$stem];
            }
        }
    }

    /**
     * @return iterable<string, array{string, string}>
     */
    public static function rejectedFixtures(): iterable
    {
        foreach (Sidecar::rruleStems('rejected') as $stem) {
            $sentinel = Sidecar::sentinel($stem);

            if ($sentinel !== null) {
                yield $stem => [$stem, $sentinel];
            }
        }
    }

    /**
     * @return iterable<string, array{string, string}>
     */
    public static function formatFixtures(): iterable
    {
        foreach (Sidecar::rruleStems('format') as $stem) {
            $want = Sidecar::formatted($stem);

            if ($want !== null) {
                yield $stem => [$stem, $want];
            }
        }
    }

    #[DataProvider('parsingFixtures')]
    public function testFixtureParses(string $stem): void
    {
        $rule = Rrule::parse(Sidecar::rruleValue($stem));

        // A successful parse never yields the unset marker: a missing
        // FREQ is ErrMalformed, so reaching here with Invalid would mean
        // the required-field check did not run.
        self::assertNotSame(Freq::Invalid, $rule->freq);
    }

    /**
     * A fixture with no sidecar also has to survive the round trip: the
     * corpus pins the parse, and the wire form the spec fixes is
     * idempotent for every rule, not only the two in `format/`.
     */
    #[DataProvider('parsingFixtures')]
    public function testFixtureRoundTrips(string $stem): void
    {
        $rule = Rrule::parse(Sidecar::rruleValue($stem));
        $emitted = (string) $rule;

        self::assertSame($emitted, (string) Rrule::parse($emitted), 'parse o format is not idempotent');
    }

    #[DataProvider('rejectedFixtures')]
    public function testRejectedFixtureRaisesItsNamedSentinel(string $stem, string $expected): void
    {
        $value = Sidecar::rruleValue($stem);

        try {
            Rrule::parse($value);
        } catch (VstarException $e) {
            self::assertSame($expected, $e->sentinel(), 'parse raised the wrong sentinel');

            return;
        }

        self::fail("{$stem}: parse accepted {$value}, want {$expected}");
    }

    /**
     * `validate` is the same gate through the other entry point: it
     * parses and discards, so it raises exactly what `parse` would.
     */
    #[DataProvider('rejectedFixtures')]
    public function testValidateRaisesItsNamedSentinel(string $stem, string $expected): void
    {
        $value = Sidecar::rruleValue($stem);

        try {
            Rrule::validate($value);
        } catch (VstarException $e) {
            self::assertSame($expected, $e->sentinel(), 'validate raised the wrong sentinel');

            return;
        }

        self::fail("{$stem}: validate accepted {$value}, want {$expected}");
    }

    #[DataProvider('formatFixtures')]
    public function testFormatFixtureEmitsItsWireForm(string $stem, string $want): void
    {
        $rule = Rrule::parse(Sidecar::rruleValue($stem));

        self::assertSame($want, (string) $rule);
    }

    /**
     * Re-parsing the emitted form and emitting again must produce the
     * same bytes -- the idempotence spec §RRULE wire form requires.
     */
    #[DataProvider('formatFixtures')]
    public function testFormatFixtureIsIdempotent(string $stem, string $want): void
    {
        self::assertSame($want, (string) Rrule::parse($want));
    }

    /**
     * List values keep the order the producer authored, and BYDAY is
     * where that is load-bearing: RFC 5545 gives BY-* lists no ordering
     * semantics, so sorting one rewrites the producer's content while
     * looking tidier.
     *
     * Neither `format/` fixture carries an unsorted multi-entry BYDAY --
     * `scrambled_parts` scrambles rule-*parts*, not list entries, and
     * `MO,WE` is already in weekday order -- so the corpus alone cannot
     * catch a sort here. This is the assertion that does.
     */
    public function testByDayListKeepsAuthoredOrder(): void
    {
        $value = 'FREQ=WEEKLY;BYDAY=SA,WE,MO,FR';
        $rule = Rrule::parse($value);

        self::assertSame(
            ['SA', 'WE', 'MO', 'FR'],
            array_map(static fn (ByDay $bd): string => $bd->weekday->value, $rule->byDay),
        );
        self::assertSame($value, (string) $rule);
    }

    /**
     * The same guarantee for the integer lists, where a sort would be
     * just as invisible: `BYMONTHDAY=-1,15,1` is not `-1,1,15`.
     */
    public function testIntegerListsKeepAuthoredOrder(): void
    {
        $value = 'FREQ=MONTHLY;BYMONTHDAY=-1,15,1;BYHOUR=17,9';

        self::assertSame($value, (string) Rrule::parse($value));
    }

    /**
     * `INTERVAL=1` and `WKST=MO` are the RFC defaults and are elided, so
     * a producer that spells one out and one that omits it render
     * identically.
     */
    public function testDefaultsAreElided(): void
    {
        $spelled = Rrule::parse('FREQ=DAILY;INTERVAL=1;WKST=MO');

        self::assertSame('FREQ=DAILY', (string) $spelled);
        self::assertStringNotContainsString('INTERVAL=', (string) $spelled);
        self::assertStringNotContainsString('WKST=', (string) $spelled);
    }

    /**
     * A non-default INTERVAL or WKST is emitted -- eliding those would
     * lose the rule's meaning rather than a redundant token.
     */
    public function testNonDefaultsAreEmitted(): void
    {
        self::assertSame('FREQ=DAILY;INTERVAL=2', (string) Rrule::parse('FREQ=DAILY;INTERVAL=2'));
        self::assertSame('FREQ=WEEKLY;WKST=SU', (string) Rrule::parse('FREQ=WEEKLY;WKST=SU'));
    }

    /**
     * Rule-part order on the wire is irrelevant on parse and fixed on
     * emit, so two authorings of the same rule converge.
     */
    public function testRulePartOrderIsFixedOnEmit(): void
    {
        $a = Rrule::parse('BYDAY=MO;FREQ=MONTHLY;BYMONTH=3;COUNT=2');
        $b = Rrule::parse('COUNT=2;BYMONTH=3;BYDAY=MO;FREQ=MONTHLY');

        self::assertSame('FREQ=MONTHLY;COUNT=2;BYMONTH=3;BYDAY=MO', (string) $a);
        self::assertSame((string) $a, (string) $b);
    }

    /**
     * The full rule-part order from spec §RRULE wire form, exercised in
     * one value so a reordering of any pair fails here.
     */
    public function testEveryRulePartEmitsInSpecOrder(): void
    {
        $want = 'FREQ=YEARLY;INTERVAL=2;COUNT=5;BYMONTH=3;BYWEEKNO=10;BYYEARDAY=100;'
            . 'BYMONTHDAY=15;BYDAY=MO;BYHOUR=9;BYMINUTE=30;BYSECOND=0;BYSETPOS=1;WKST=SU';

        // Authored in reverse to prove the emitter imposes the order.
        $scrambled = 'WKST=SU;BYSETPOS=1;BYSECOND=0;BYMINUTE=30;BYHOUR=9;BYDAY=MO;'
            . 'BYMONTHDAY=15;BYYEARDAY=100;BYWEEKNO=10;BYMONTH=3;COUNT=5;INTERVAL=2;FREQ=YEARLY';

        self::assertSame($want, (string) Rrule::parse($scrambled));
    }

    /**
     * UNTIL renders as RFC 5545 form #2, the only form the scope accepts.
     */
    public function testUntilRendersAsFormTwo(): void
    {
        $value = 'FREQ=DAILY;UNTIL=20261231T235959Z';
        $rule = Rrule::parse($value);

        self::assertNotNull($rule->until);
        self::assertSame($value, (string) $rule);
    }

    /**
     * A BYDAY ordinal round-trips with its sign, and an ordinal-less
     * entry renders with no prefix -- the explicit `0` prefix is invalid
     * per RFC, so "every weekday of this kind" is spelled by omission.
     */
    public function testByDayOrdinalsRoundTrip(): void
    {
        $value = 'FREQ=MONTHLY;BYDAY=-1FR,2MO,WE';
        $rule = Rrule::parse($value);

        self::assertSame([-1, 2, 0], array_map(static fn (ByDay $bd): int => $bd->ordinal, $rule->byDay));
        self::assertSame($value, (string) $rule);
    }

    /**
     * `toProperty()` produces a complete RRULE property, name included,
     * carrying the same value `__toString()` renders.
     */
    public function testToPropertyCarriesTheWireForm(): void
    {
        $rule = Rrule::parse('FREQ=DAILY;COUNT=3');
        $p = $rule->toProperty();

        self::assertSame('RRULE', $p->name);
        self::assertSame('FREQ=DAILY;COUNT=3', $p->value);
        self::assertSame([], $p->params);
    }

    /**
     * Weekday numbering is RFC 5545 §3.3.10's `SU = 0`, which is neither
     * ISO-8601's `MO = 1` nor what `DateTime::format('N')` reports. The
     * conversion lives in one named place.
     */
    public function testWeekdayNumberingIsRfcNotIso(): void
    {
        self::assertSame(0, Weekday::Su->number());
        self::assertSame(1, Weekday::Mo->number());
        self::assertSame(6, Weekday::Sa->number());

        self::assertSame(7, Weekday::Su->toWeekday());
        self::assertSame(1, Weekday::Mo->toWeekday());
        self::assertSame(6, Weekday::Sa->toWeekday());
    }

    /**
     * The enums carry the wire spelling through `toString()`. The API
     * mapping's corrected form: PHP rejects `__toString()` on an enum at
     * declaration time, so enums use a plain method.
     */
    public function testEnumsStringifyToTheirWireSpelling(): void
    {
        self::assertSame('DAILY', Freq::Daily->toString());
        self::assertSame('YEARLY', Freq::Yearly->toString());
        self::assertSame('MO', Weekday::Mo->toString());
    }

    /**
     * A rule with no FREQ renders as the empty string rather than a
     * partial value that would fail to re-parse.
     */
    public function testUnsetRuleRendersEmpty(): void
    {
        self::assertSame('', (string) new Rule(Freq::Invalid));
    }

    /**
     * The empty value is malformed, not an empty rule: every RRULE needs
     * a FREQ.
     */
    public function testEmptyValueIsMalformed(): void
    {
        $this->expectException(\HopTop\Vstar\Exception\MalformedException::class);

        Rrule::parse('');
    }

    /**
     * A repeated rule-part is malformed: RFC 5545 §3.3.10 permits each
     * at most once, and silently keeping the last would make the value's
     * meaning depend on parse order.
     */
    public function testDuplicateRulePartIsMalformed(): void
    {
        $this->expectException(\HopTop\Vstar\Exception\MalformedException::class);

        Rrule::parse('FREQ=DAILY;COUNT=2;COUNT=3');
    }

    /**
     * Range violations the corpus does not spell out, each malformed.
     *
     * @return iterable<string, array{string}>
     */
    public static function outOfRangeValues(): iterable
    {
        foreach ([
            'FREQ=YEARLY;BYMONTH=13',
            'FREQ=YEARLY;BYMONTH=0',
            'FREQ=DAILY;BYHOUR=24',
            'FREQ=DAILY;BYMINUTE=60',
            'FREQ=DAILY;BYSECOND=61',
            'FREQ=MONTHLY;BYMONTHDAY=32',
            'FREQ=MONTHLY;BYMONTHDAY=-32',
            'FREQ=YEARLY;BYYEARDAY=367',
            'FREQ=YEARLY;BYYEARDAY=0',
            'FREQ=YEARLY;BYWEEKNO=54',
            'FREQ=MONTHLY;BYDAY=54MO',
            'FREQ=DAILY;INTERVAL=-1',
            'FREQ=DAILY;COUNT=0',
            'FREQ=DAILY;WKST=XX',
            'FREQ=DAILY;BYDAY=',
            'FREQ=DAILY;BYHOUR=',
            'FREQ=DAILY;BYHOUR=0x10',
            'FREQ=DAILY;BYHOUR= 5',
            'FREQ=NEVER',
            '=DAILY',
            'FREQ',
        ] as $value) {
            yield $value => [$value];
        }
    }

    /**
     * `BYSECOND=60` is retained for leap seconds per RFC 5545 §3.3.10 --
     * the one boundary in the group that is legal.
     */
    public function testBySecondSixtyIsAccepted(): void
    {
        self::assertSame('FREQ=DAILY;BYSECOND=60', (string) Rrule::parse('FREQ=DAILY;BYSECOND=60'));
    }

    #[DataProvider('outOfRangeValues')]
    public function testOutOfRangeValueIsMalformed(string $value): void
    {
        $this->expectException(\HopTop\Vstar\Exception\MalformedException::class);

        Rrule::parse($value);
    }
}
