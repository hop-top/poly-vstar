<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Rrule;

use HopTop\Vstar\Codec\Rfc5545\Parser as IcsParser;
use HopTop\Vstar\Component;
use HopTop\Vstar\Exception\UnboundedExpansionException;
use HopTop\Vstar\Exception\UnsupportedRRuleException;
use HopTop\Vstar\Exception\VstarException;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;
use HopTop\Vstar\Rrule\RecurrenceId;
use HopTop\Vstar\Rrule\RecurrenceRange;
use HopTop\Vstar\Rrule\Rrule;
use HopTop\Vstar\Rrule\RuleSet;
use HopTop\Vstar\Tests\Corpus;
use HopTop\Vstar\Time;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * Recurrence sets (RFC 5545 §3.8.5) and RECURRENCE-ID (§3.8.4.4),
 * gated by `rrule/set/`.
 *
 * The evaluation order spec §Recurrence sets fixes is the whole point:
 * DTSTART, then the RRULE expansion, then RDATE merged in, and EXDATE
 * removed **last** -- so an instant named by both an RDATE and an EXDATE
 * stays excluded. `set/exdate_after_rdate` is the fixture that tells the
 * two orders apart.
 */
final class SetTest extends TestCase
{
    /**
     * @return iterable<string, array{string}>
     */
    public static function setFixtures(): iterable
    {
        foreach (Sidecar::icsStems('set') as $stem) {
            yield $stem => [$stem];
        }
    }

    /**
     * @return iterable<string, array{string, string}>
     */
    public static function rejectedSetFixtures(): iterable
    {
        foreach (Sidecar::icsStems('set/rejected') as $stem) {
            $sentinel = Sidecar::sentinel($stem);

            if ($sentinel !== null) {
                yield $stem => [$stem, $sentinel];
            }
        }
    }

    #[DataProvider('setFixtures')]
    public function testSetFixtureExpandsToItsSidecar(string $stem): void
    {
        $spec = Sidecar::outcome($stem, 'occurrences.json');
        self::assertNotNull($spec, "{$stem}: no .occurrences.json sidecar");
        self::assertNotNull($spec['limit']);

        $set = RuleSet::fromComponent(self::firstComponent($stem));
        $result = $set->occurrences($spec['limit']);

        self::assertSame($spec['expected'], array_map(
            static fn (\DateTimeImmutable $t): string => Time::formatTime($t),
            $result['occurrences'],
        ));
        self::assertSame($spec['complete'], $result['complete'], "{$stem}: complete flag");
    }

    #[DataProvider('rejectedSetFixtures')]
    public function testRejectedSetFixtureRaisesItsNamedSentinel(string $stem, string $expected): void
    {
        try {
            RuleSet::fromComponent(self::firstComponent($stem));
        } catch (VstarException $e) {
            self::assertSame($expected, $e->sentinel(), 'fromComponent raised the wrong sentinel');

            return;
        }

        self::fail("{$stem}: fromComponent accepted the component, want {$expected}");
    }

    /**
     * EXDATE removes last. The corpus fixture pins it, and this states
     * the mechanism directly: 2026-04-10 is named by both an RDATE and
     * an EXDATE, and it must not survive.
     */
    public function testExdateRemovesAfterRdateMerges(): void
    {
        $set = RuleSet::fromComponent(self::firstComponent('rrule/set/exdate_after_rdate'));
        $got = array_map(
            static fn (\DateTimeImmutable $t): string => Time::formatTime($t),
            $set->occurrences(10)['occurrences'],
        );

        self::assertNotContains('20260410T120000Z', $got, 'an RDATE resurrected an excluded instant');
        self::assertNotContains('20260402T120000Z', $got, 'an EXDATE did not remove a rule occurrence');
        self::assertSame(['20260401T120000Z', '20260403T120000Z'], $got);
    }

    /**
     * A set with RDATE and no RRULE is legal and finite: DTSTART plus
     * the explicit dates, sorted, and the expansion completes.
     */
    public function testRdateOnlySetIsFiniteAndComplete(): void
    {
        $set = RuleSet::fromComponent(self::firstComponent('rrule/set/rdate_only'));
        $result = $set->occurrences(10);

        self::assertNull($set->rrule, 'an RDATE-only set has no rule');
        self::assertTrue($result['complete']);
        self::assertSame(
            ['20260401T120000Z', '20260403T120000Z', '20260405T120000Z'],
            array_map(static fn (\DateTimeImmutable $t): string => Time::formatTime($t), $result['occurrences']),
        );
    }

    /**
     * The merged stream is sorted and de-duplicated by instant: an RDATE
     * naming an instant the rule already produces is one occurrence, not
     * two.
     */
    public function testMergedStreamIsSortedAndDeduplicated(): void
    {
        $set = new RuleSet(
            self::instant('20260401T120000Z'),
            Rrule::parse('FREQ=DAILY;COUNT=3'),
            [self::instant('20260402T120000Z'), self::instant('20260401T060000Z')],
        );

        self::assertSame([
            '20260401T060000Z',
            '20260401T120000Z',
            '20260402T120000Z',
            '20260403T120000Z',
        ], array_map(
            static fn (\DateTimeImmutable $t): string => Time::formatTime($t),
            $set->occurrences(10)['occurrences'],
        ));
    }

    /**
     * EXDATE removals do not consume limit slots: the limit bounds
     * *returned* occurrences, so excluding the first two still yields a
     * full three.
     */
    public function testExdateRemovalsDoNotConsumeTheLimit(): void
    {
        $set = new RuleSet(
            self::instant('20260401T120000Z'),
            Rrule::parse('FREQ=DAILY'),
            [],
            [self::instant('20260401T120000Z'), self::instant('20260402T120000Z')],
        );

        self::assertSame([
            '20260403T120000Z',
            '20260404T120000Z',
            '20260405T120000Z',
        ], array_map(
            static fn (\DateTimeImmutable $t): string => Time::formatTime($t),
            $set->occurrences(3)['occurrences'],
        ));
    }

    /**
     * A set's window is half-open and applies the same EXDATE-last
     * order.
     */
    public function testSetBetweenIsHalfOpenAndExcludesLast(): void
    {
        $set = new RuleSet(
            self::instant('20260401T120000Z'),
            Rrule::parse('FREQ=DAILY'),
            [self::instant('20260404T120000Z')],
            [self::instant('20260404T120000Z')],
        );

        $got = $set->between(self::instant('20260402T120000Z'), self::instant('20260405T120000Z'));

        self::assertSame(
            ['20260402T120000Z', '20260403T120000Z'],
            array_map(static fn (\DateTimeImmutable $t): string => Time::formatTime($t), $got),
        );
    }

    public function testSetBetweenRejectsADegenerateWindow(): void
    {
        $set = new RuleSet(self::instant('20260401T120000Z'), Rrule::parse('FREQ=DAILY'));

        $this->expectException(UnboundedExpansionException::class);

        $set->between(self::instant('20260403T120000Z'), self::instant('20260403T120000Z'));
    }

    public function testSetBetweenRejectsAMissingEnd(): void
    {
        $set = new RuleSet(self::instant('20260401T120000Z'), Rrule::parse('FREQ=DAILY'));

        $this->expectException(UnboundedExpansionException::class);

        $set->between(self::instant('20260403T120000Z'), null);
    }

    public function testSetOccurrencesRejectsANegativeLimit(): void
    {
        $set = new RuleSet(self::instant('20260401T120000Z'), Rrule::parse('FREQ=DAILY'));

        $this->expectException(UnboundedExpansionException::class);

        $set->occurrences(-1);
    }

    /**
     * A `VALUE=DATE` EXDATE or RDATE is out of the v0.1 scope: a
     * date-only value would need a time-of-day guessed for it, and
     * silently dropping the property would surface an occurrence the
     * producer cancelled.
     *
     * @return iterable<string, array{string}>
     */
    public static function dateOnlyListProperties(): iterable
    {
        yield 'EXDATE' => ['EXDATE'];
        yield 'RDATE' => ['RDATE'];
    }

    #[DataProvider('dateOnlyListProperties')]
    public function testValueDateListPropertyIsUnsupported(string $name): void
    {
        $c = new Component('VEVENT', [
            new Property('DTSTART', [], '20260401T120000Z'),
            new Property($name, [new Param('VALUE', 'DATE')], '20260402'),
        ]);

        $this->expectException(UnsupportedRRuleException::class);

        RuleSet::fromComponent($c);
    }

    #[DataProvider('dateOnlyListProperties')]
    public function testTzidListPropertyIsUnsupported(string $name): void
    {
        $c = new Component('VEVENT', [
            new Property('DTSTART', [], '20260401T120000Z'),
            new Property($name, [new Param('TZID', 'America/New_York')], '20260402T080000'),
        ]);

        $this->expectException(UnsupportedRRuleException::class);

        RuleSet::fromComponent($c);
    }

    /**
     * Both EXDATE and RDATE may repeat and may each carry several
     * comma-separated values; every value accumulates.
     */
    public function testRepeatedListPropertiesAccumulate(): void
    {
        $c = new Component('VEVENT', [
            new Property('DTSTART', [], '20260401T120000Z'),
            new Property('RDATE', [], '20260405T120000Z,20260403T120000Z'),
            new Property('RDATE', [], '20260407T120000Z'),
        ]);

        $set = RuleSet::fromComponent($c);

        self::assertSame(
            ['20260403T120000Z', '20260405T120000Z', '20260407T120000Z'],
            array_map(static fn (\DateTimeImmutable $t): string => Time::formatTime($t), $set->rdate),
        );
    }

    /**
     * `parseDateTimeList` and `formatDateTimeList` are inverses over a
     * canonical (sorted, de-duplicated) list.
     */
    public function testDateTimeListRoundTrips(): void
    {
        $value = '20260401T120000Z,20260403T120000Z,20260405T120000Z';

        self::assertSame($value, Rrule::formatDateTimeList(Rrule::parseDateTimeList($value)));
    }

    /**
     * Both directions canonicalize: a scrambled, duplicated authoring
     * yields the same bytes as the sorted one.
     */
    public function testDateTimeListSortsAndDeduplicates(): void
    {
        $got = Rrule::parseDateTimeList('20260405T120000Z,20260401T120000Z,20260405T120000Z');

        self::assertSame(
            ['20260401T120000Z', '20260405T120000Z'],
            array_map(static fn (\DateTimeImmutable $t): string => Time::formatTime($t), $got),
        );
        self::assertSame(
            '20260401T120000Z,20260405T120000Z',
            Rrule::formatDateTimeList([self::instant('20260405T120000Z'), self::instant('20260401T120000Z')]),
        );
    }

    public function testEmptyDateTimeListFormatsAsTheEmptyString(): void
    {
        self::assertSame('', Rrule::formatDateTimeList([]));
    }

    public function testMalformedDateTimeListIsMalformed(): void
    {
        $this->expectException(\HopTop\Vstar\Exception\MalformedException::class);

        Rrule::parseDateTimeList('20260401T120000');
    }

    /**
     * RECURRENCE-ID parses to an instant plus RANGE. The default is
     * expressed by omitting the parameter, so it has no wire token and
     * re-emitting must not invent one.
     */
    public function testRecurrenceIdDefaultsToThisInstance(): void
    {
        $id = RecurrenceId::parse(new Property('RECURRENCE-ID', [], '20260401T120000Z'));

        self::assertSame(RecurrenceRange::ThisInstance, $id->range);
        self::assertSame('20260401T120000Z', Time::formatTime($id->time));

        $p = $id->toProperty();
        self::assertSame('RECURRENCE-ID', $p->name);
        self::assertSame([], $p->params, 'the default RANGE must not be emitted');
    }

    public function testRecurrenceIdCarriesThisAndFuture(): void
    {
        $id = RecurrenceId::parse(new Property(
            'RECURRENCE-ID',
            [new Param('RANGE', 'THISANDFUTURE')],
            '20260401T120000Z',
        ));

        self::assertSame(RecurrenceRange::ThisAndFuture, $id->range);

        $p = $id->toProperty();
        self::assertCount(1, $p->params);
        self::assertSame('RANGE', $p->params[0]->name);
        self::assertSame('THISANDFUTURE', $p->params[0]->value);
    }

    /**
     * RFC 5545 §3.2.13 defines exactly one RANGE token, so any other is
     * malformed rather than silently ignored.
     */
    public function testUnknownRangeIsMalformed(): void
    {
        $this->expectException(\HopTop\Vstar\Exception\MalformedException::class);

        RecurrenceId::parse(new Property('RECURRENCE-ID', [new Param('RANGE', 'THISANDPRIOR')], '20260401T120000Z'));
    }

    public function testNonRecurrenceIdPropertyIsMalformed(): void
    {
        $this->expectException(\HopTop\Vstar\Exception\MalformedException::class);

        RecurrenceId::parse(new Property('DTSTART', [], '20260401T120000Z'));
    }

    public function testRecurrenceIdWithTzidIsUnsupported(): void
    {
        $this->expectException(UnsupportedRRuleException::class);

        RecurrenceId::parse(new Property(
            'RECURRENCE-ID',
            [new Param('TZID', 'America/New_York')],
            '20260401T080000',
        ));
    }

    public function testRecurrenceIdRangeStringifiesToItsWireSpelling(): void
    {
        self::assertSame('', RecurrenceRange::ThisInstance->toString());
        self::assertSame('THISANDFUTURE', RecurrenceRange::ThisAndFuture->toString());
    }

    /**
     * The first component of an `.ics` fixture, which is what the
     * reference verifier feeds to `SetFromComponent`.
     */
    private static function firstComponent(string $stem): Component
    {
        $cal = IcsParser::parse(Corpus::read($stem . '.ics'));

        self::assertNotSame([], $cal->components, "{$stem}: calendar has no components");

        return $cal->components[0];
    }

    private static function instant(string $s): \DateTimeImmutable
    {
        $t = Time::parseTime($s);
        self::assertNotNull($t, "{$s} is not RFC 5545 form #2");

        return $t;
    }
}
