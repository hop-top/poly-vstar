<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Card;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\EventStatus;
use HopTop\Vstar\JournalStatus;
use HopTop\Vstar\Kind;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;
use HopTop\Vstar\RelType;
use HopTop\Vstar\TodoStatus;
use HopTop\Vstar\Transp;
use HopTop\Vstar\VClass;
use HopTop\Vstar\VDate;
use HopTop\Vstar\Vstar;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

#[CoversClass(Calendar::class)]
#[CoversClass(Card::class)]
#[CoversClass(Component::class)]
#[CoversClass(Param::class)]
#[CoversClass(Property::class)]
#[CoversClass(RelType::class)]
#[CoversClass(VDate::class)]
#[CoversClass(Vstar::class)]
final class ModelTest extends TestCase
{
    public function testComponentGetIsCaseInsensitiveAndReturnsTheFirstMatch(): void
    {
        $c = new Component(CompType::Event, [
            new Property('uid', [], 'first'),
            new Property('UID', [], 'second'),
        ]);

        $got = $c->get('UiD');
        self::assertNotNull($got);
        self::assertSame('first', $got->value);
        self::assertNull($c->get('SUMMARY'));
    }

    public function testComponentGetAllReturnsEveryMatchInWireOrder(): void
    {
        $c = new Component(CompType::Event, [
            new Property('CATEGORIES', [], 'a'),
            new Property('SUMMARY', [], 's'),
            new Property('categories', [], 'b'),
        ]);

        $values = array_map(static fn (Property $p): string => $p->value, $c->getAll('CATEGORIES'));
        self::assertSame(['a', 'b'], $values);
        self::assertSame([], $c->getAll('LOCATION'));
    }

    public function testComponentSetReplacesEveryMatchWithOneCopyInPlace(): void
    {
        $c = new Component(CompType::Event, [
            new Property('A', [], '1'),
            new Property('UID', [], 'old'),
            new Property('B', [], '2'),
            new Property('uid', [], 'older'),
        ]);

        $c->set(new Property('UID', [], 'new'));

        $names = array_map(static fn (Property $p): string => $p->name, $c->props);
        self::assertSame(['A', 'UID', 'B'], $names);
        $uid = $c->get('UID');
        self::assertNotNull($uid);
        self::assertSame('new', $uid->value);
    }

    public function testComponentSetAppendsWhenNothingMatches(): void
    {
        $c = new Component(CompType::Event, [new Property('A', [], '1')]);
        $c->set(new Property('B', [], '2'));

        self::assertSame(['A', 'B'], array_map(static fn (Property $p): string => $p->name, $c->props));
    }

    public function testComponentAddDoesNotTouchExistingProperties(): void
    {
        $c = new Component(CompType::Event, [new Property('UID', [], 'a')]);
        $c->add(new Property('UID', [], 'b'));

        self::assertCount(2, $c->props);
    }

    public function testComponentRemoveDropsEveryCaseInsensitiveMatch(): void
    {
        $c = new Component(CompType::Event, [
            new Property('UID', [], 'a'),
            new Property('SUMMARY', [], 's'),
            new Property('uid', [], 'b'),
        ]);

        $c->remove('UID');
        self::assertSame(['SUMMARY'], array_map(static fn (Property $p): string => $p->name, $c->props));
    }

    public function testComponentUidAndDtstampRaw(): void
    {
        $c = new Component(CompType::Todo, [
            new Property('UID', [], 'u-1'),
            new Property('DTSTAMP', [], '20260504T120000Z'),
        ]);

        self::assertSame('u-1', $c->uid());
        self::assertSame('20260504T120000Z', $c->dtstampRaw());
        self::assertSame('', (new Component(CompType::Todo))->uid());
        self::assertSame('', (new Component(CompType::Todo))->dtstampRaw());
    }

    public function testCalendarFindMatchesUidCaseSensitively(): void
    {
        $cal = new Calendar('p', [
            new Component(CompType::Todo, [new Property('UID', [], 'a')]),
            new Component(CompType::Event, [new Property('UID', [], 'B')]),
        ]);

        $found = $cal->find('B');
        self::assertNotNull($found);
        self::assertSame(CompType::Event->value, $found->type);
        self::assertNull($cal->find('b'));
    }

    public function testCalendarAppendAndFilter(): void
    {
        $cal = new Calendar('p');
        $cal->append(new Component(CompType::Todo));
        $cal->append(new Component(CompType::Event));
        $cal->append(new Component(CompType::Todo));

        self::assertCount(2, $cal->filter(CompType::Todo));
        self::assertSame([], $cal->filter(CompType::FreeBusy));
    }

    public function testCardAccessors(): void
    {
        $card = new Card('u', Kind::Org, [new Property('FN', [], 'Jad')]);

        $fn = $card->get('fn');
        self::assertNotNull($fn);
        self::assertSame('Jad', $fn->value);

        $card->add(new Property('FN', [], 'Second'));
        self::assertCount(2, $card->getAll('FN'));

        $card->set(new Property('FN', [], 'Only'));
        self::assertCount(1, $card->getAll('FN'));

        $card->remove('FN');
        self::assertSame([], $card->props);
    }

    public function testPropertyEqualIgnoresNameCaseAndParameterOrder(): void
    {
        $a = new Property('summary', [new Param('B', '2'), new Param('A', '1')], 'v');
        $b = new Property('SUMMARY', [new Param('a', '1'), new Param('b', '2')], 'v');

        self::assertTrue(Vstar::propertyEqual($a, $b));
    }

    public function testPropertyEqualIsCaseSensitiveOnValues(): void
    {
        self::assertFalse(Vstar::propertyEqual(
            new Property('S', [], 'v'),
            new Property('S', [], 'V'),
        ));
    }

    public function testPropertyEqualComparesParameterCount(): void
    {
        self::assertFalse(Vstar::propertyEqual(
            new Property('S', [new Param('A', '1')], 'v'),
            new Property('S', [], 'v'),
        ));
    }

    public function testVDateParseFormatRoundTrip(): void
    {
        $d = Vstar::parseDate('20260515');
        self::assertNotNull($d);
        self::assertSame(2026, $d->year);
        self::assertSame(5, $d->month);
        self::assertSame(15, $d->day);
        self::assertSame('20260515', Vstar::formatDate($d));
        self::assertSame('20260515', (string) $d);
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function rejectedDateStrings(): iterable
    {
        yield 'empty' => [''];
        yield 'datetime' => ['20260515T000000Z'];
        yield 'iso extended' => ['2026-05-15'];
        yield 'feb 30' => ['20260230'];
        yield 'month 13' => ['20261301'];
        yield 'day zero' => ['20260500'];
        yield 'feb 29 non-leap' => ['20260229'];
        yield 'too short' => ['2026051'];
        yield 'too long' => ['202605150'];
        yield 'non digits' => ['2026O515'];
        yield 'leading space' => [' 0260515'];
    }

    #[DataProvider('rejectedDateStrings')]
    public function testVDateParseIsStrict(string $input): void
    {
        self::assertNull(Vstar::parseDate($input));
    }

    public function testVDateAcceptsLeapDayInALeapYear(): void
    {
        self::assertNotNull(Vstar::parseDate('20240229'));
    }

    public function testZeroVDateFormatsAsTheEmptyString(): void
    {
        $zero = VDate::of(0, 0, 0);

        self::assertTrue($zero->isZero());
        self::assertSame('', Vstar::formatDate($zero));
    }

    public function testOutOfRangeVDateFormatsAsTheEmptyString(): void
    {
        self::assertSame('', Vstar::formatDate(VDate::of(2026, 13, 1)));
        self::assertSame('', Vstar::formatDate(VDate::of(2026, 5, 32)));
        self::assertSame('', Vstar::formatDate(VDate::of(10000, 5, 1)));
    }

    public function testIsDateOnlyRequiresTheValueDateParameter(): void
    {
        $c = new Component(CompType::Todo, [
            new Property('DUE', [new Param('VALUE', 'DATE')], '20260515'),
            new Property('DTSTART', [], '20260515'),
        ]);

        self::assertTrue($c->isDateOnly('DUE'));
        self::assertFalse($c->isDateOnly('DTSTART'), 'an untagged 8-octet value is a DATE-TIME, not a DATE');
        self::assertFalse($c->isDateOnly('DTEND'), 'absent property');
    }

    public function testIsDateOnlyFoldsParameterCase(): void
    {
        $c = new Component(CompType::Todo, [
            new Property('DUE', [new Param('value', 'date')], '20260515'),
        ]);

        self::assertTrue($c->isDateOnly('DUE'));
    }

    public function testDateAccessorsRequireValueDate(): void
    {
        $c = new Component(CompType::Todo, [
            new Property('DTSTART', [new Param('VALUE', 'DATE')], '20260515'),
            new Property('DUE', [], '20260516T000000Z'),
            new Property('DTEND', [new Param('VALUE', 'DATE')], 'not-a-date'),
        ]);

        $start = $c->dtstartDate();
        self::assertNotNull($start);
        self::assertSame(15, $start->day);
        self::assertNull($c->dueDate(), 'a DATE-TIME must not surface as a date');
        self::assertNull($c->dtendDate(), 'a malformed DATE must not surface');
        self::assertNull($c->completedDate(), 'absent property');
    }

    public function testDateSettersWriteTheValueDateParameterAndDropOthers(): void
    {
        $c = new Component(CompType::Todo, [
            new Property('DTSTART', [new Param('TZID', 'America/Montreal')], '20260515T090000'),
        ]);

        $c->setDtstartDate(VDate::of(2026, 5, 15));

        $p = $c->get('DTSTART');
        self::assertNotNull($p);
        self::assertSame('20260515', $p->value);
        self::assertCount(1, $p->params, 'a stale TZID must not survive onto a DATE');
        self::assertSame('VALUE', $p->params[0]->name);
        self::assertSame('DATE', $p->params[0]->value);
    }

    public function testDateSettersClearThePropertyOnTheZeroDate(): void
    {
        $c = new Component(CompType::Todo, [
            new Property('DUE', [new Param('VALUE', 'DATE')], '20260515'),
        ]);

        $c->setDueDate(VDate::of(0, 0, 0));
        self::assertNull($c->get('DUE'));
    }

    public function testAllFourDateSettersAndGettersPair(): void
    {
        $c = new Component(CompType::Todo);
        $d = VDate::of(2026, 5, 15);

        $c->setDtstartDate($d);
        $c->setDtendDate($d);
        $c->setDueDate($d);
        $c->setCompletedDate($d);

        self::assertEquals($d, $c->dtstartDate());
        self::assertEquals($d, $c->dtendDate());
        self::assertEquals($d, $c->dueDate());
        self::assertEquals($d, $c->completedDate());
    }

    /**
     * @return iterable<string, array{string, string, bool}>
     */
    public static function relTypeInputs(): iterable
    {
        yield 'empty means PARENT' => ['', 'PARENT', true];
        yield 'exact' => ['CHILD', 'CHILD', true];
        yield 'lowercase folds' => ['sibling', 'SIBLING', true];
        yield 'mixed case folds' => ['DePeNdS-On', 'DEPENDS-ON', true];
        yield 'rfc 9253 temporal' => ['finishtostart', 'FINISHTOSTART', true];
        yield 'refid' => ['refid', 'REFID', true];
        yield 'extension kept verbatim' => ['X-CUSTOM', 'X-CUSTOM', false];
        yield 'extension keeps its case' => ['x-custom', 'x-custom', false];
    }

    #[DataProvider('relTypeInputs')]
    public function testParseRelTypeIsAnOpenEnum(string $input, string $wire, bool $registered): void
    {
        [$relType, $ok] = RelType::parse($input);

        self::assertSame($wire, $relType->value);
        self::assertSame($registered, $ok);
    }

    public function testRelTypeRegistryCoversTheTwelveRegisteredValues(): void
    {
        $registered = [
            'PARENT', 'CHILD', 'SIBLING',
            'FINISHTOSTART', 'FINISHTOFINISH', 'STARTTOFINISH', 'STARTTOSTART',
            'DEPENDS-ON', 'FIRST', 'NEXT', 'CONCEPT', 'REFID',
        ];

        foreach ($registered as $wire) {
            [, $ok] = RelType::parse($wire);
            self::assertTrue($ok, "{$wire} must be a registered RELTYPE");
        }
    }

    public function testRelTypeDefaultIsParent(): void
    {
        self::assertSame('PARENT', RelType::DEFAULT);
    }

    public function testRelTypeEqualFoldIsCaseInsensitive(): void
    {
        [$rt] = RelType::parse('CHILD');

        self::assertTrue($rt->equalFold('child'));
        self::assertTrue($rt->equalFold('ChIlD'));
        self::assertFalse($rt->equalFold('PARENT'));
    }

    public function testWireEnumValuesAreNormative(): void
    {
        self::assertSame('VCALENDAR', CompType::Calendar->value);
        self::assertSame('VTODO', CompType::Todo->value);
        self::assertSame('VJOURNAL', CompType::Journal->value);
        self::assertSame('VEVENT', CompType::Event->value);
        self::assertSame('VFREEBUSY', CompType::FreeBusy->value);
        self::assertSame('VTIMEZONE', CompType::Timezone->value);
        self::assertSame('VALARM', CompType::Alarm->value);

        self::assertSame('individual', Kind::Individual->value);
        self::assertSame('org', Kind::Org->value);
        self::assertSame('group', Kind::Group->value);

        self::assertSame('NEEDS-ACTION', TodoStatus::NeedsAction->value);
        self::assertSame('IN-PROCESS', TodoStatus::InProcess->value);
        self::assertSame('COMPLETED', TodoStatus::Completed->value);
        self::assertSame('CANCELLED', TodoStatus::Cancelled->value);

        self::assertSame('TENTATIVE', EventStatus::Tentative->value);
        self::assertSame('CONFIRMED', EventStatus::Confirmed->value);
        self::assertSame('CANCELLED', EventStatus::Cancelled->value);

        self::assertSame('DRAFT', JournalStatus::Draft->value);
        self::assertSame('FINAL', JournalStatus::Final->value);
        self::assertSame('CANCELLED', JournalStatus::Cancelled->value);

        self::assertSame('PUBLIC', VClass::Public->value);
        self::assertSame('PRIVATE', VClass::Private->value);
        self::assertSame('CONFIDENTIAL', VClass::Confidential->value);

        self::assertSame('OPAQUE', Transp::Opaque->value);
        self::assertSame('TRANSPARENT', Transp::Transparent->value);
    }

    public function testTheThreeStatusVocabulariesAreDistinctTypes(): void
    {
        // Same wire string, three types: a cross-type assignment must be a
        // type error, not a wire-level conformance bug found in production.
        self::assertNotSame(TodoStatus::Cancelled, EventStatus::Cancelled);
        self::assertNotSame(EventStatus::Cancelled, JournalStatus::Cancelled);
    }
}
