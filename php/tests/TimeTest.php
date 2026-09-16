<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Codec\Rfc5545\Parser as IcsParser;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;
use HopTop\Vstar\Time;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * RFC 5545 §3.3.5 wire forms and VTIMEZONE-only TZID resolution.
 *
 * `spec/behavior/time/tzid.json` is the gate, and it is also a trap: it
 * names a real IANA zone (`America/Montreal`), so a port that resolved it
 * from the system tzdata would pass every case here and fail every other
 * zone in the world. The not-ok cases -- `utc: null` -- are what catch
 * that: a zone the document does not define must NOT resolve, however well
 * the operating system knows it.
 */
final class TimeTest extends TestCase
{
    /**
     * Every `spec/behavior/time/tzid.json` case.
     *
     * A `utc` of null is the not-ok answer: resolution must fail, and
     * failing is a normal outcome rather than an exception.
     *
     * @return iterable<string, array{string, string, string, ?string}>
     */
    public static function tzidCases(): iterable
    {
        foreach (Behavior::cases('time/tzid.json') as $i => $case) {
            /** @var string $calendar */
            $calendar = $case['calendar'];
            /** @var string $tzid */
            $tzid = $case['tzid'];
            /** @var string $value */
            $value = $case['value'];
            /** @var ?string $utc */
            $utc = $case['utc'];

            $label = sprintf('#%d %s %s %s', $i, $calendar, $tzid === '' ? '(no tzid)' : $tzid, $value);

            yield $label => [$calendar, $tzid, $value, $utc];
        }
    }

    #[DataProvider('tzidCases')]
    public function testParseTimeWithTzidMatchesTheBehaviorCorpus(
        string $calendar,
        string $tzid,
        string $value,
        ?string $utc,
    ): void {
        $cal = IcsParser::parse(Corpus::read("time/{$calendar}.ics"));

        $got = Time::parseTimeWithTzid($value, $tzid, $cal);

        if ($utc === null) {
            self::assertNull($got, 'resolution must fail for a zone the document does not define');

            return;
        }

        self::assertNotNull($got);
        self::assertSame($utc, Time::formatTime($got));
    }

    /**
     * The every-case check that no IANA database is consulted.
     *
     * `America/Toronto` is a real zone the system certainly knows and the
     * `america_montreal.ics` document certainly does not define. A port
     * reaching for `DateTimeZone('America/Toronto')` resolves it happily;
     * a conformant one returns null.
     */
    public function testAKnownIanaZoneAbsentFromTheDocumentDoesNotResolve(): void
    {
        $cal = IcsParser::parse(Corpus::read('time/america_montreal.ics'));

        self::assertNull(Time::parseTimeWithTzid('20260104T133045', 'America/Toronto', $cal));
    }

    public function testFormatTimeRendersFormTwoInUtc(): void
    {
        $t = new \DateTimeImmutable('2026-05-04T12:00:00', new \DateTimeZone('UTC'));

        self::assertSame('20260504T120000Z', Time::formatTime($t));
    }

    /**
     * An instant carrying a non-UTC offset is converted before rendering.
     * Rendering its local wall clock with a `Z` suffix would be a
     * different instant wearing the right shape.
     */
    public function testFormatTimeConvertsANonUtcInstantBeforeRendering(): void
    {
        $t = new \DateTimeImmutable('2026-05-04T08:00:00', new \DateTimeZone('-04:00'));

        self::assertSame('20260504T120000Z', Time::formatTime($t));
    }

    public function testFormatTimeRendersTheEpochAsTheEmptyString(): void
    {
        self::assertSame('', Time::formatTime(new \DateTimeImmutable('@0')));
    }

    public function testParseTimeAcceptsFormTwo(): void
    {
        $got = Time::parseTime('20260504T120000Z');

        self::assertNotNull($got);
        self::assertSame(1777896000, $got->getTimestamp());
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function rejectedTimeValues(): iterable
    {
        yield 'form #1, no zone' => ['20260504T120000'];
        yield 'RFC 3339 extended' => ['2026-05-04T12:00:00Z'];
        yield 'date only' => ['20260504'];
        yield 'lowercase z' => ['20260504T120000z'];
        yield 'leading space' => [' 20260504T120000Z'];
        yield 'trailing space' => ['20260504T120000Z '];
        yield 'empty' => [''];
        yield 'February 30th' => ['20260230T120000Z'];
        yield 'February 29th in a non-leap year' => ['20260229T120000Z'];
        yield 'month 13' => ['20261304T120000Z'];
        yield 'day 0' => ['20260500T120000Z'];
        yield 'hour 24' => ['20260504T240000Z'];
        yield 'minute 60' => ['20260504T126000Z'];
        yield 'second 60' => ['20260504T120060Z'];
        yield 'non-digits' => ['2026050aT120000Z'];
    }

    #[DataProvider('rejectedTimeValues')]
    public function testParseTimeRejects(string $s): void
    {
        self::assertNull(Time::parseTime($s));
    }

    /**
     * February 29th 2028 is real; the rejection above is a leap-year rule,
     * not a blanket refusal of the 29th.
     */
    public function testParseTimeAcceptsFebruaryTwentyNinthInALeapYear(): void
    {
        self::assertNotNull(Time::parseTime('20280229T120000Z'));
    }

    /**
     * Form #2 input is rejected by the TZID path even with a TZID present:
     * the value is already absolute, and resolving it against a zone would
     * apply the offset twice.
     */
    public function testParseTimeWithTzidRejectsAnAlreadyAbsoluteValue(): void
    {
        $cal = IcsParser::parse(Corpus::read('time/america_montreal.ics'));

        self::assertNull(Time::parseTimeWithTzid('20260315T060000Z', 'America/Montreal', $cal));
    }

    /**
     * A VTIMEZONE outside the v0.1 subset is a resolution failure, not an
     * exception, and rule 5 then falls back to verbatim emit.
     *
     * @return iterable<string, array{list<Component>}>
     */
    public static function rejectedZoneShapes(): iterable
    {
        $child = self::zoneChild(...);
        $std = self::standard(...);

        yield 'two STANDARD children' => [[$std(), $std()]];

        yield 'STANDARD missing TZOFFSETTO' => [[$child('STANDARD', [
            new Property('DTSTART', [], '19700101T020000'),
            new Property('TZOFFSETFROM', [], '-0400'),
        ])]];

        yield 'STANDARD missing TZOFFSETFROM' => [[$child('STANDARD', [
            new Property('DTSTART', [], '19700101T020000'),
            new Property('TZOFFSETTO', [], '-0500'),
        ])]];

        yield 'STANDARD missing DTSTART' => [[$child('STANDARD', [
            new Property('TZOFFSETFROM', [], '-0400'),
            new Property('TZOFFSETTO', [], '-0500'),
        ])]];

        yield 'no children at all' => [[]];

        yield 'RRULE with COUNT' => [[
            $std([new Property('RRULE', [], 'FREQ=YEARLY;BYMONTH=11;BYDAY=1SU;COUNT=3')]),
            $child('DAYLIGHT', [
                new Property('DTSTART', [], '19700308T020000'),
                new Property('TZOFFSETFROM', [], '-0500'),
                new Property('TZOFFSETTO', [], '-0400'),
                new Property('RRULE', [], 'FREQ=YEARLY;BYMONTH=3;BYDAY=2SU'),
            ]),
        ]];

        yield 'RRULE without FREQ=YEARLY' => [[
            $std([new Property('RRULE', [], 'FREQ=MONTHLY;BYMONTH=11;BYDAY=1SU')]),
            $child('DAYLIGHT', [
                new Property('DTSTART', [], '19700308T020000'),
                new Property('TZOFFSETFROM', [], '-0500'),
                new Property('TZOFFSETTO', [], '-0400'),
                new Property('RRULE', [], 'FREQ=YEARLY;BYMONTH=3;BYDAY=2SU'),
            ]),
        ]];

        yield 'BYDAY without an ordinal' => [[
            $std([new Property('RRULE', [], 'FREQ=YEARLY;BYMONTH=11;BYDAY=SU')]),
            $child('DAYLIGHT', [
                new Property('DTSTART', [], '19700308T020000'),
                new Property('TZOFFSETFROM', [], '-0500'),
                new Property('TZOFFSETTO', [], '-0400'),
                new Property('RRULE', [], 'FREQ=YEARLY;BYMONTH=3;BYDAY=2SU'),
            ]),
        ]];

        yield 'INTERVAL other than 1' => [[
            $std([new Property('RRULE', [], 'FREQ=YEARLY;BYMONTH=11;BYDAY=1SU;INTERVAL=2')]),
            $child('DAYLIGHT', [
                new Property('DTSTART', [], '19700308T020000'),
                new Property('TZOFFSETFROM', [], '-0500'),
                new Property('TZOFFSETTO', [], '-0400'),
                new Property('RRULE', [], 'FREQ=YEARLY;BYMONTH=3;BYDAY=2SU'),
            ]),
        ]];

        yield 'DAYLIGHT present but neither child carries an RRULE' => [[
            $std(),
            $child('DAYLIGHT', [
                new Property('DTSTART', [], '19700308T020000'),
                new Property('TZOFFSETFROM', [], '-0500'),
                new Property('TZOFFSETTO', [], '-0400'),
            ]),
        ]];
    }

    /**
     * One VTIMEZONE sub-component, as the wire string types the RFC 5545
     * §3.6.5 grammar names -- neither is a CompType case.
     *
     * @param list<Property> $props
     */
    private static function zoneChild(string $type, array $props): Component
    {
        return new Component($type, $props);
    }

    /**
     * A well-formed STANDARD child, optionally carrying extra properties.
     *
     * @param list<Property> $extra
     */
    private static function standard(array $extra = []): Component
    {
        return self::zoneChild('STANDARD', array_merge([
            new Property('DTSTART', [], '19700101T020000'),
            new Property('TZOFFSETFROM', [], '-0400'),
            new Property('TZOFFSETTO', [], '-0500'),
        ], $extra));
    }

    /**
     * @param list<Component> $children
     */
    #[DataProvider('rejectedZoneShapes')]
    public function testAZoneOutsideTheSubsetDoesNotResolve(array $children): void
    {
        $cal = new Calendar('-//V*//Test//EN', [
            new Component(CompType::Timezone, [new Property('TZID', [], 'X/Zone')], $children),
        ]);

        self::assertNull(Time::parseTimeWithTzid('20260104T133045', 'X/Zone', $cal));
    }

    /**
     * A single STANDARD with no RRULE is the accepted fixed-offset shape.
     */
    public function testASingleFixedOffsetStandardResolves(): void
    {
        $cal = new Calendar('-//V*//Test//EN', [
            new Component(CompType::Timezone, [new Property('TZID', [], 'Fixed/Plus0530')], [
                new Component('STANDARD', [
                    new Property('DTSTART', [], '19700101T000000'),
                    new Property('TZOFFSETFROM', [], '+0530'),
                    new Property('TZOFFSETTO', [], '+0530'),
                ]),
            ]),
        ]);

        $got = Time::parseTimeWithTzid('20260104T133045', 'Fixed/Plus0530', $cal);

        self::assertNotNull($got);
        self::assertSame('20260104T080045Z', Time::formatTime($got));
    }

    /**
     * A DAYLIGHT-only zone is treated as a single fixed-offset rule, since
     * there is no transition to compute.
     */
    public function testADaylightOnlyZoneResolvesAsAFixedOffset(): void
    {
        $cal = new Calendar('-//V*//Test//EN', [
            new Component(CompType::Timezone, [new Property('TZID', [], 'Only/Daylight')], [
                new Component('DAYLIGHT', [
                    new Property('DTSTART', [], '19700101T000000'),
                    new Property('TZOFFSETFROM', [], '-0500'),
                    new Property('TZOFFSETTO', [], '-0400'),
                ]),
            ]),
        ]);

        $got = Time::parseTimeWithTzid('20260104T133045', 'Only/Daylight', $cal);

        self::assertNotNull($got);
        self::assertSame('20260104T173045Z', Time::formatTime($got));
    }

    /**
     * A TZOFFSETTO in the `±HHMMSS` form RFC 5545 §3.3.14 allows resolves
     * with its seconds intact.
     */
    public function testAnOffsetWithSecondsResolves(): void
    {
        $cal = new Calendar('-//V*//Test//EN', [
            new Component(CompType::Timezone, [new Property('TZID', [], 'Odd/Offset')], [
                new Component('STANDARD', [
                    new Property('DTSTART', [], '19700101T000000'),
                    new Property('TZOFFSETFROM', [], '+000030'),
                    new Property('TZOFFSETTO', [], '+000030'),
                ]),
            ]),
        ]);

        $got = Time::parseTimeWithTzid('20260104T000100', 'Odd/Offset', $cal);

        self::assertNotNull($got);
        self::assertSame('20260104T000030Z', Time::formatTime($got));
    }

    /**
     * TZIDs are opaque identifiers per RFC 5545 §3.2.19, so the lookup is
     * case-**sensitive**: folding case would merge two distinct zones.
     */
    public function testTheTzidLookupIsCaseSensitive(): void
    {
        $cal = IcsParser::parse(Corpus::read('time/america_montreal.ics'));

        self::assertNull(Time::parseTimeWithTzid('20260104T133045', 'america/montreal', $cal));
    }

    public function testTheDatetimeAccessorsResolveThroughTheCalendarsRegistry(): void
    {
        $cal = IcsParser::parse(Corpus::read('time/america_montreal.ics'));
        $c = new Component(CompType::Event, [
            new Property('UID', [], 'u'),
            new Property('DTSTART', [new Param('TZID', 'America/Montreal')], '20260104T133045'),
        ]);

        $got = $c->dtstart($cal);

        self::assertNotNull($got);
        self::assertSame('20260104T183045Z', Time::formatTime($got));
    }

    /**
     * A `VALUE=DATE` property is not an instant; the datetime accessors
     * report absence rather than promoting it to midnight.
     */
    public function testTheDatetimeAccessorsIgnoreADateValue(): void
    {
        $c = new Component(CompType::Event, [
            new Property('DTSTART', [new Param('VALUE', 'DATE')], '20260515'),
        ]);

        self::assertNull($c->dtstart(new Calendar()));
    }

    public function testTheTimeSettersWriteUtcFormTwoAndDropParameters(): void
    {
        $c = new Component(CompType::Event, [
            new Property('DTSTART', [new Param('TZID', 'America/Montreal')], '20260104T133045'),
        ]);

        $c->setDtstart(new \DateTimeImmutable('2026-01-04T18:30:45', new \DateTimeZone('UTC')));

        $p = $c->get('DTSTART');
        self::assertNotNull($p);
        self::assertSame('20260104T183045Z', $p->value);
        self::assertSame([], $p->params, 'a stale TZID would contradict a UTC value');
    }
}
