<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Helpers;

use HopTop\Vstar\Canonical\Canonical;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Duration\Related;
use HopTop\Vstar\Duration\VDuration;
use HopTop\Vstar\EventStatus;
use HopTop\Vstar\Exception\MissingUidException;
use HopTop\Vstar\Hashing\Hashing;
use HopTop\Vstar\Helpers\Helpers;
use HopTop\Vstar\Helpers\RelatedRef;
use HopTop\Vstar\JournalStatus;
use HopTop\Vstar\Kind;
use HopTop\Vstar\Property;
use HopTop\Vstar\RelType;
use HopTop\Vstar\Tests\Corpus;
use HopTop\Vstar\TodoStatus;
use HopTop\Vstar\Transp;
use HopTop\Vstar\VClass;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * The constructors and high-level mutators.
 *
 * Two gates run here. The **emitter gate** rebuilds a committed corpus
 * fixture through the helper API and asserts the canonical bytes and the
 * hash both match -- that catches a constructor writing the wrong
 * property, the wrong value, or the wrong order. It cannot catch a
 * missing `X-VSTAR-HASH`, because canonicalization strips that property
 * (spec/03 rule 7) before either side is compared; the **hash-discipline
 * unit tests** below cover that blind spot directly.
 */
final class HelpersTest extends TestCase
{
    private const UTC = 'UTC';

    // ---------------------------------------------------------------
    // Constructors
    // ---------------------------------------------------------------

    public function testNewTodoStampsUidDtstampAndDue(): void
    {
        $due = self::at('2026-03-01T00:00:00');
        $c = Helpers::newTodo('abc-123', $due);

        self::assertSame(CompType::Todo->value, $c->type);
        self::assertSame('abc-123', $c->uid());
        self::assertNotSame('', $c->dtstampRaw());
        self::assertSame('20260301T000000Z', $c->get('DUE')?->value);
    }

    public function testNewJournalStampsDtstart(): void
    {
        $c = Helpers::newJournal('j-1', self::at('2026-03-01T09:00:00'));

        self::assertSame(CompType::Journal->value, $c->type);
        self::assertSame('20260301T090000Z', $c->get('DTSTART')?->value);
    }

    public function testNewEventStampsBothEnds(): void
    {
        $c = Helpers::newEvent(
            'e-1',
            self::at('2026-03-01T09:00:00'),
            self::at('2026-03-01T10:00:00'),
        );

        self::assertSame(CompType::Event->value, $c->type);
        self::assertSame('20260301T090000Z', $c->get('DTSTART')?->value);
        self::assertSame('20260301T100000Z', $c->get('DTEND')?->value);
    }

    public function testNewFreeBusyStampsBothEnds(): void
    {
        $c = Helpers::newFreeBusy(
            'f-1',
            self::at('2026-03-01T09:00:00'),
            self::at('2026-03-01T10:00:00'),
        );

        self::assertSame(CompType::FreeBusy->value, $c->type);
        self::assertSame('20260301T090000Z', $c->get('DTSTART')?->value);
        self::assertSame('20260301T100000Z', $c->get('DTEND')?->value);
    }

    public function testNewAlarmStampsActionAndTrigger(): void
    {
        $c = Helpers::newAlarm('a-1', 'DISPLAY', '-PT15M');

        self::assertSame(CompType::Alarm->value, $c->type);
        self::assertSame('DISPLAY', $c->get('ACTION')?->value);
        self::assertSame('-PT15M', $c->get('TRIGGER')?->value);
    }

    /**
     * @return iterable<string, array{callable(): Component}>
     */
    public static function emptyUidConstructors(): iterable
    {
        $t = new \DateTimeImmutable('2026-03-01T00:00:00', new \DateTimeZone(self::UTC));

        yield 'newTodo' => [static fn (): Component => Helpers::newTodo('', $t)];
        yield 'newJournal' => [static fn (): Component => Helpers::newJournal('', $t)];
        yield 'newEvent' => [static fn (): Component => Helpers::newEvent('', $t, $t)];
        yield 'newFreeBusy' => [static fn (): Component => Helpers::newFreeBusy('', $t, $t)];
        yield 'newAlarm' => [static fn (): Component => Helpers::newAlarm('', 'DISPLAY', '-PT5M')];
        yield 'newAbsoluteAlarm' => [
            static fn (): Component => Helpers::newAbsoluteAlarm('', 'DISPLAY', $t),
        ];
        yield 'newRelativeAlarm' => [
            static fn (): Component => Helpers::newRelativeAlarm(
                '',
                'DISPLAY',
                new VDuration(minutes: 15, negative: true),
                Related::Start,
            ),
        ];
    }

    /**
     * @param callable(): Component $construct
     */
    #[DataProvider('emptyUidConstructors')]
    public function testConstructorsRejectAnEmptyUid(callable $construct): void
    {
        $this->expectException(MissingUidException::class);
        $construct();
    }

    /**
     * The default PRODID is version-free and language-free: it is part of
     * the hashed canonical form, so every port emits the same literal and
     * no release changes it.
     */
    public function testNewCalendarDefaultsTheProdId(): void
    {
        self::assertSame('-//V*//Custom//EN', Helpers::newCalendar('-//V*//Custom//EN')->prodId);
        self::assertSame('-//hop-top//vstar//EN', Helpers::newCalendar('')->prodId);
    }

    /**
     * The documented default: an absent kind becomes `individual` and a
     * KIND property is written from it.
     */
    public function testNewCardDefaultsAnEmptyKindToIndividual(): void
    {
        $card = Helpers::newCard('u-1', null);

        self::assertSame(Kind::Individual, $card->kind);
        self::assertSame('individual', $card->get('KIND')?->value);
        self::assertSame('4.0', $card->get('VERSION')?->value);
        self::assertSame('u-1', $card->get('UID')?->value);
    }

    public function testNewCardKeepsAnExplicitKind(): void
    {
        $card = Helpers::newCard('u-2', Kind::Group);

        self::assertSame(Kind::Group, $card->kind);
        self::assertSame('group', $card->get('KIND')?->value);
    }

    // ---------------------------------------------------------------
    // Hash discipline -- the emitter gate's blind spot
    // ---------------------------------------------------------------

    /**
     * @return iterable<string, array{callable(): Component}>
     */
    public static function hashingConstructors(): iterable
    {
        $t = new \DateTimeImmutable('2026-03-01T00:00:00', new \DateTimeZone(self::UTC));

        yield 'newTodo' => [static fn (): Component => Helpers::newTodo('u', $t)];
        yield 'newJournal' => [static fn (): Component => Helpers::newJournal('u', $t)];
        yield 'newEvent' => [static fn (): Component => Helpers::newEvent('u', $t, $t)];
        yield 'newFreeBusy' => [static fn (): Component => Helpers::newFreeBusy('u', $t, $t)];
        yield 'newAlarm' => [static fn (): Component => Helpers::newAlarm('u', 'DISPLAY', '-PT5M')];
        yield 'newAbsoluteAlarm' => [
            static fn (): Component => Helpers::newAbsoluteAlarm('u', 'DISPLAY', $t),
        ];
        yield 'newRelativeAlarm' => [
            static fn (): Component => Helpers::newRelativeAlarm(
                'u',
                'DISPLAY',
                new VDuration(minutes: 15, negative: true),
                Related::Start,
            ),
        ];
    }

    /**
     * Every component constructor writes X-VSTAR-HASH, last, so the stored
     * value covers every property it set.
     *
     * Canonicalization strips this property, so the emitter gate is blind
     * to its absence -- this assertion is the one that catches a
     * constructor that forgot it.
     *
     * @param callable(): Component $construct
     */
    #[DataProvider('hashingConstructors')]
    public function testConstructorsStampAVerifyingHash(callable $construct): void
    {
        $c = $construct();

        self::assertNotNull(
            Hashing::getXVstar($c),
            'constructor must write X-VSTAR-HASH',
        );
        self::assertTrue(
            Hashing::verifyXVstar($c)['ok'],
            'the stored hash must cover every property the constructor set',
        );
    }

    /**
     * @return iterable<string, array{callable(Component): void}>
     */
    public static function hashingMutators(): iterable
    {
        yield 'setCategories' => [
            static function (Component $c): void {
                Helpers::setCategories($c, ['work']);
            },
        ];
        yield 'addCategory' => [
            static function (Component $c): void {
                Helpers::addCategory($c, 'work');
            },
        ];
        yield 'addRelatedTo' => [
            static function (Component $c): void {
                Helpers::addRelatedTo(
                    $c,
                    'other',
                    new RelType(RelType::CHILD),
                );
            },
        ];
        yield 'setDue' => [
            static function (Component $c): void {
                Helpers::setDue(
                    $c,
                    new \DateTimeImmutable('2026-03-01T00:00:00', new \DateTimeZone(self::UTC)),
                );
            },
        ];
        yield 'setTodoStatus' => [
            static function (Component $c): void {
                Helpers::setTodoStatus(
                    $c,
                    TodoStatus::InProcess,
                );
            },
        ];
        yield 'setSequence' => [
            static function (Component $c): void {
                Helpers::setSequence($c, 3);
            },
        ];
        yield 'incrementSequence' => [
            static function (Component $c): void {
                Helpers::incrementSequence($c);
            },
        ];
        yield 'setPriority' => [
            static function (Component $c): void {
                Helpers::setPriority($c, 3);
            },
        ];
        yield 'removePriority' => [
            static function (Component $c): void {
                Helpers::removePriority($c);
            },
        ];
        yield 'setPercentComplete' => [
            static function (Component $c): void {
                Helpers::setPercentComplete($c, 50);
            },
        ];
        yield 'removePercentComplete' => [
            static function (Component $c): void {
                Helpers::removePercentComplete($c);
            },
        ];
        yield 'setClass' => [
            static function (Component $c): void {
                Helpers::setClass($c, VClass::Private);
            },
        ];
        yield 'complete' => [
            static function (Component $c): void {
                Helpers::complete(
                    $c,
                    new \DateTimeImmutable('2026-03-01T00:00:00', new \DateTimeZone(self::UTC)),
                );
            },
        ];
    }

    /**
     * Every mutator refreshes X-VSTAR-HASH last, so a caller never
     * observes a component whose stored hash lags its content.
     *
     * @param callable(Component): void $mutate
     */
    #[DataProvider('hashingMutators')]
    public function testMutatorsRefreshTheHashLast(callable $mutate): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));
        $mutate($c);

        self::assertTrue(
            Hashing::verifyXVstar($c)['ok'],
            'the stored hash must match the mutated content',
        );
    }

    // ---------------------------------------------------------------
    // Categories
    // ---------------------------------------------------------------

    public function testCategoriesSplitsTrimsAndDropsEmpties(): void
    {
        $c = new Component(CompType::Todo, [
            new Property('CATEGORIES', [], 'work, home ,,urgent'),
        ]);

        self::assertSame(['work', 'home', 'urgent'], Helpers::categories($c));
    }

    public function testCategoriesIsEmptyWhenAbsent(): void
    {
        self::assertSame([], Helpers::categories(new Component(CompType::Todo)));
    }

    public function testSetCategoriesJoinsWithoutSpacesAndDedupes(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));
        Helpers::setCategories($c, ['work', ' home ', 'work', '']);

        self::assertSame('work,home', $c->get('CATEGORIES')?->value);
    }

    public function testSetCategoriesWithNothingRemovesTheProperty(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));
        Helpers::setCategories($c, ['work']);
        Helpers::setCategories($c, []);

        self::assertNull($c->get('CATEGORIES'));
    }

    public function testAddCategoryIsCaseSensitiveAndIdempotent(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));
        Helpers::addCategory($c, 'Work');
        Helpers::addCategory($c, 'Work');
        Helpers::addCategory($c, 'work');

        self::assertSame('Work,work', $c->get('CATEGORIES')?->value);
    }

    // ---------------------------------------------------------------
    // Relations
    // ---------------------------------------------------------------

    public function testRelatedToDefaultsAnAbsentReltypeToParent(): void
    {
        $c = new Component(CompType::Todo, [
            new Property('RELATED-TO', [], 'other'),
        ]);

        $refs = Helpers::relatedTo($c);
        self::assertCount(1, $refs);
        self::assertSame('other', $refs[0]->uid);
        self::assertSame(RelType::PARENT, (string) $refs[0]->relType);
    }

    public function testAddRelatedToWritesTheReltypeParam(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));
        Helpers::addRelatedTo($c, 'kid', new RelType(RelType::CHILD));
        Helpers::addRelatedTo($c, 'ext', new RelType('X-CUSTOM'));

        $refs = Helpers::relatedTo($c);
        self::assertCount(2, $refs);
        self::assertSame('kid', $refs[0]->uid);
        self::assertSame(RelType::CHILD, (string) $refs[0]->relType);
        self::assertSame('X-CUSTOM', (string) $refs[1]->relType);
    }

    public function testRelatedToIsEmptyWhenAbsent(): void
    {
        self::assertSame([], Helpers::relatedTo(new Component(CompType::Todo)));
    }

    public function testRelatedRefIsAValueObject(): void
    {
        $ref = new RelatedRef('u', new RelType(RelType::SIBLING));

        self::assertSame('u', $ref->uid);
        self::assertSame(RelType::SIBLING, (string) $ref->relType);
    }

    // ---------------------------------------------------------------
    // Due, status, completion
    // ---------------------------------------------------------------

    public function testDueAndSetDue(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));
        $cal = Helpers::newCalendar('');

        self::assertSame(
            '20260301T000000Z',
            Helpers::due($c, $cal)?->format('Ymd\THis\Z'),
        );

        Helpers::setDue($c, self::at('2026-04-01T00:00:00'));
        self::assertSame(
            '20260401T000000Z',
            Helpers::due($c, $cal)?->format('Ymd\THis\Z'),
        );
    }

    public function testTodoStatusRoundTripsAndRejectsForeignVocabulary(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));

        self::assertNull(Helpers::todoStatus($c));

        Helpers::setTodoStatus($c, TodoStatus::InProcess);
        self::assertSame(TodoStatus::InProcess, Helpers::todoStatus($c));

        // A VEVENT-only value is not a VTODO status.
        $c->set(new Property('STATUS', [], 'TENTATIVE'));
        self::assertNull(Helpers::todoStatus($c));
    }

    public function testSetTodoStatusIsANoOpOnTheWrongComponentType(): void
    {
        $e = Helpers::newEvent(
            'u',
            self::at('2026-03-01T09:00:00'),
            self::at('2026-03-01T10:00:00'),
        );
        Helpers::setTodoStatus($e, TodoStatus::Completed);

        self::assertNull($e->get('STATUS'));
    }

    public function testEventStatusRoundTrips(): void
    {
        $e = Helpers::newEvent(
            'u',
            self::at('2026-03-01T09:00:00'),
            self::at('2026-03-01T10:00:00'),
        );

        self::assertNull(Helpers::eventStatus($e));
        Helpers::setEventStatus($e, EventStatus::Confirmed);
        self::assertSame(EventStatus::Confirmed, Helpers::eventStatus($e));
    }

    public function testJournalStatusRoundTrips(): void
    {
        $j = Helpers::newJournal('u', self::at('2026-03-01T09:00:00'));

        self::assertNull(Helpers::journalStatus($j));
        Helpers::setJournalStatus($j, JournalStatus::Final);
        self::assertSame(JournalStatus::Final, Helpers::journalStatus($j));
    }

    /**
     * The three vocabularies stay apart: a VTODO status is not a VEVENT
     * status, even where both spell CANCELLED.
     */
    public function testStatusVocabulariesDoNotBleed(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));
        Helpers::setTodoStatus($c, TodoStatus::NeedsAction);

        self::assertNull(Helpers::eventStatus($c));
        self::assertNull(Helpers::journalStatus($c));
    }

    public function testCompleteWritesTheFullDoneMarkerSet(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));
        Helpers::complete($c, self::at('2026-03-02T15:00:00'));

        self::assertSame(TodoStatus::Completed, Helpers::todoStatus($c));
        self::assertSame('20260302T150000Z', $c->get('COMPLETED')?->value);
        self::assertSame(100, Helpers::percentComplete($c));
    }

    public function testCompleteIsANoOpOnTheWrongComponentType(): void
    {
        $e = Helpers::newEvent(
            'u',
            self::at('2026-03-01T09:00:00'),
            self::at('2026-03-01T10:00:00'),
        );
        Helpers::complete($e, self::at('2026-03-02T15:00:00'));

        self::assertNull($e->get('STATUS'));
        self::assertNull($e->get('COMPLETED'));
    }

    // ---------------------------------------------------------------
    // Integer-valued properties
    // ---------------------------------------------------------------

    public function testSequenceRoundTripsAndIncrements(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));

        self::assertNull(Helpers::sequence($c));

        // An absent SEQUENCE is the RFC default of 0, so the first
        // increment yields 1.
        Helpers::incrementSequence($c);
        self::assertSame(1, Helpers::sequence($c));

        Helpers::setSequence($c, 7);
        self::assertSame(7, Helpers::sequence($c));

        Helpers::incrementSequence($c);
        self::assertSame(8, Helpers::sequence($c));
    }

    public function testSetSequenceRejectsANegativeCounter(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));
        Helpers::setSequence($c, 3);
        Helpers::setSequence($c, -1);

        self::assertSame(3, Helpers::sequence($c), 'rejection must leave the good value');
    }

    public function testSequenceRejectsANonCanonicalWireValue(): void
    {
        $c = new Component(CompType::Todo, [new Property('SEQUENCE', [], '03')]);

        self::assertNull(Helpers::sequence($c));
    }

    public function testPriorityRoundTripsAndBoundsTheRange(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));

        self::assertNull(Helpers::priority($c));

        Helpers::setPriority($c, 0);
        self::assertSame(0, Helpers::priority($c), 'PRIORITY:0 is a value, not absence');

        Helpers::setPriority($c, 9);
        self::assertSame(9, Helpers::priority($c));

        // Out of range is rejected, not clamped.
        Helpers::setPriority($c, 10);
        self::assertSame(9, Helpers::priority($c));
        Helpers::setPriority($c, -1);
        self::assertSame(9, Helpers::priority($c));
    }

    public function testRemovePriorityDeletesTheProperty(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));
        Helpers::setPriority($c, 3);
        Helpers::removePriority($c);

        self::assertNull(Helpers::priority($c));
        self::assertNull($c->get('PRIORITY'));
    }

    public function testPercentCompleteRoundTripsAndBoundsTheRange(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));

        self::assertNull(Helpers::percentComplete($c));

        Helpers::setPercentComplete($c, 0);
        self::assertSame(0, Helpers::percentComplete($c));

        Helpers::setPercentComplete($c, 100);
        self::assertSame(100, Helpers::percentComplete($c));

        Helpers::setPercentComplete($c, 101);
        self::assertSame(100, Helpers::percentComplete($c));
    }

    public function testRemovePercentCompleteDeletesTheProperty(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));
        Helpers::setPercentComplete($c, 50);
        Helpers::removePercentComplete($c);

        self::assertNull(Helpers::percentComplete($c));
    }

    // ---------------------------------------------------------------
    // Classification and transparency
    // ---------------------------------------------------------------

    public function testClassReportsAbsenceAndOrDefaultAppliesTheRfcDefault(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));

        self::assertNull(Helpers::classOf($c), 'the plain getter reports absence faithfully');
        self::assertSame(VClass::Public, Helpers::classOrDefault($c));

        Helpers::setClass($c, VClass::Confidential);
        self::assertSame(VClass::Confidential, Helpers::classOf($c));
        self::assertSame(VClass::Confidential, Helpers::classOrDefault($c));
    }

    public function testClassOrDefaultFallsBackOnAnUnrecognizedValue(): void
    {
        $c = new Component(CompType::Todo, [new Property('CLASS', [], 'BOGUS')]);

        self::assertNull(Helpers::classOf($c));
        self::assertSame(VClass::Public, Helpers::classOrDefault($c));
    }

    public function testSetClassIsANoOpOnAComponentThatDoesNotAdmitIt(): void
    {
        $a = Helpers::newAlarm('u', 'DISPLAY', '-PT5M');
        Helpers::setClass($a, VClass::Private);

        self::assertNull($a->get('CLASS'));
    }

    public function testTranspReportsAbsenceAndOrDefaultAppliesOpaque(): void
    {
        $e = Helpers::newEvent(
            'u',
            self::at('2026-03-01T09:00:00'),
            self::at('2026-03-01T10:00:00'),
        );

        self::assertNull(Helpers::transp($e));
        self::assertSame(Transp::Opaque, Helpers::transpOrDefault($e));

        Helpers::setTransp($e, Transp::Transparent);
        self::assertSame(Transp::Transparent, Helpers::transp($e));
    }

    public function testSetTranspIsVeventOnly(): void
    {
        $c = Helpers::newTodo('u', self::at('2026-03-01T00:00:00'));
        Helpers::setTransp($c, Transp::Transparent);

        self::assertNull($c->get('TRANSP'));
    }

    // ---------------------------------------------------------------
    // Alarms
    // ---------------------------------------------------------------

    public function testNewRelativeAlarmEmitsADurationTrigger(): void
    {
        $a = Helpers::newRelativeAlarm(
            'a-1',
            'DISPLAY',
            new VDuration(minutes: 15, negative: true),
            Related::Start,
        );

        $trigger = $a->get('TRIGGER');
        self::assertNotNull($trigger);
        self::assertSame('-PT15M', $trigger->value);
        // RELATED=START is the RFC default and stays implicit.
        self::assertNull($trigger->param('RELATED'));
    }

    public function testNewRelativeAlarmEmitsRelatedEndExplicitly(): void
    {
        $a = Helpers::newRelativeAlarm(
            'a-2',
            'DISPLAY',
            new VDuration(minutes: 10, negative: true),
            Related::End,
        );

        self::assertSame('END', $a->get('TRIGGER')?->param('RELATED'));
    }

    public function testNewAbsoluteAlarmTagsTheValueType(): void
    {
        $a = Helpers::newAbsoluteAlarm('a-3', 'DISPLAY', self::at('2026-06-01T09:00:00'));

        $trigger = $a->get('TRIGGER');
        self::assertNotNull($trigger);
        self::assertSame('20260601T090000Z', $trigger->value);
        self::assertSame('DATE-TIME', $trigger->param('VALUE'));
    }

    public function testAlarmFiresAtResolvesARelativeTrigger(): void
    {
        $parent = Helpers::newEvent(
            'e-1',
            self::at('2026-06-01T09:00:00'),
            self::at('2026-06-01T10:00:00'),
        );
        $alarm = Helpers::newRelativeAlarm(
            'a-1',
            'DISPLAY',
            new VDuration(minutes: 15, negative: true),
            Related::Start,
        );

        self::assertSame(
            '20260601T084500Z',
            Helpers::alarmFiresAt($alarm, $parent, Helpers::newCalendar(''))
                ->format('Ymd\THis\Z'),
        );
    }

    public function testAlarmFiresAtResolvesAnAbsoluteTrigger(): void
    {
        $parent = Helpers::newEvent(
            'e-1',
            self::at('2026-06-01T09:00:00'),
            self::at('2026-06-01T10:00:00'),
        );
        $alarm = Helpers::newAbsoluteAlarm('a-1', 'DISPLAY', self::at('2026-06-01T08:00:00'));

        self::assertSame(
            '20260601T080000Z',
            Helpers::alarmFiresAt($alarm, $parent, Helpers::newCalendar(''))
                ->format('Ymd\THis\Z'),
        );
    }

    // ---------------------------------------------------------------
    // The emitter gate
    // ---------------------------------------------------------------

    /**
     * `rfc5545/one_vtodo`, rebuilt entirely through the helper API.
     *
     * The constructors stamp DTSTAMP with the wall clock, so that one
     * property is restamped to the fixture's literal. Everything else --
     * the property set, its ordering, the X-VSTAR-HASH the constructor
     * wrote and canonicalization then strips -- comes out of the helper
     * API untouched.
     */
    public function testEmitterGateOneVtodo(): void
    {
        $todo = Helpers::newTodo('abc-123', self::at('2026-05-04T12:00:00'));

        // The fixture carries no DUE; the constructor's is removed.
        $todo->remove('DUE');
        $todo->add(new Property('SUMMARY', [], 'Buy milk'));
        Helpers::setPriority($todo, 3);
        $todo->set(new Property('DTSTAMP', [], '20260504T120000Z'));

        $cal = Helpers::newCalendar('-//V*//OneVTODO//EN');
        $cal->append($todo);

        self::assertSame(
            Corpus::read('rfc5545/one_vtodo.canonical'),
            Corpus::crlfToLf(Canonical::calendar($cal)),
        );
        // The hash covers the CRLF bytes, not the LF-transformed ones.
        self::assertSame(
            trim(Corpus::read('rfc5545/one_vtodo.hash')),
            Hashing::calendar($cal),
        );
    }

    /**
     * `rfc6350/minimal`, rebuilt through the helper API.
     *
     * The fixture carries no KIND, and `newCard` defaults an empty kind to
     * `individual` and writes a KIND property from it -- so the rebuild
     * clears both the model field and that derived property. The
     * divergence is the constructor's documented default, not a
     * canonicalization bug.
     */
    public function testEmitterGateMinimalVcard(): void
    {
        $card = Helpers::newCard('urn:uuid:11111111-1111-1111-1111-111111111111', null);

        $card->kind = null;
        $card->remove('KIND');
        $card->remove('VERSION');
        $card->remove('UID');
        $card->add(new Property('FN', [], 'Jad Bitar'));

        self::assertSame(
            Corpus::read('rfc6350/minimal.canonical'),
            Corpus::crlfToLf(Canonical::card($card)),
        );
        self::assertSame(
            trim(Corpus::read('rfc6350/minimal.hash')),
            Hashing::card($card),
        );
    }

    private static function at(string $iso): \DateTimeImmutable
    {
        return new \DateTimeImmutable($iso, new \DateTimeZone(self::UTC));
    }
}
