<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Duration;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Codec\Rfc5545\Parser as IcsParser;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Duration\Duration;
use HopTop\Vstar\Duration\Related;
use HopTop\Vstar\Duration\Trigger;
use HopTop\Vstar\Duration\VDuration;
use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\NoAnchorException;
use HopTop\Vstar\Exception\NoTriggerException;
use HopTop\Vstar\Exception\VstarException;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;
use HopTop\Vstar\Tests\Behavior;
use HopTop\Vstar\Time;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * RFC 5545 §3.8.6.3 TRIGGER parsing and anchor resolution.
 *
 * `spec/behavior/duration/*.trigger.json` is the gate: each `.ics` holds
 * one or more VALARMs, and the sidecar names, per alarm UID, either the
 * instant the alarm fires at or the sentinel resolving it must produce.
 */
final class TriggerTest extends TestCase
{
    /**
     * Every alarm case across every `*.trigger.json` sidecar.
     *
     * The stems are walked rather than named, so a case added to the
     * behavior corpus becomes a test without a code change.
     *
     * @return iterable<string, array{string, string, ?string, ?string}>
     */
    public static function alarmCases(): iterable
    {
        foreach (Behavior::stems('duration', 'trigger.json') as $stem) {
            foreach (Behavior::cases("duration/{$stem}.trigger.json") as $case) {
                /** @var string $uid */
                $uid = $case['alarm_uid'];
                $firesAt = null;

                if (isset($case['fires_at']) && is_string($case['fires_at'])) {
                    $firesAt = $case['fires_at'];
                }

                $error = null;

                if (isset($case['error']) && is_string($case['error'])) {
                    $error = $case['error'];
                }

                yield "{$stem}/{$uid}" => [$stem, $uid, $firesAt, $error];
            }
        }
    }

    #[DataProvider('alarmCases')]
    public function testAlarmsResolveAsTheBehaviorCorpusSays(
        string $stem,
        string $uid,
        ?string $firesAt,
        ?string $error,
    ): void {
        $cal = IcsParser::parse(Behavior::read("duration/{$stem}.ics"));
        [$parent, $alarm] = self::findAlarm($cal, $uid);

        if ($error !== null) {
            try {
                Duration::alarmTrigger($alarm)->resolve($parent, $cal);
                self::fail("{$stem}/{$uid} should have failed with {$error}");
            } catch (VstarException $e) {
                self::assertSame($error, $e->sentinel());
            }

            return;
        }

        $at = Duration::alarmTrigger($alarm)->resolve($parent, $cal);

        self::assertSame($firesAt, Time::formatTime($at));
    }

    /**
     * The behavior corpus covers both failure classes, and they are
     * different answers: a VALARM with no TRIGGER at all is `ErrNoTrigger`
     * (the property is mandatory per RFC 5545 §3.6.6), while a TRIGGER
     * whose anchor is absent is `ErrNoAnchor`.
     */
    public function testAMissingTriggerAndAMissingAnchorAreDifferentFailures(): void
    {
        $noTrigger = IcsParser::parse(Behavior::read('duration/missing_trigger.ics'));
        [, $alarm] = self::findAlarm($noTrigger, 'alarm-no-trigger');

        $this->expectException(NoTriggerException::class);
        Duration::alarmTrigger($alarm);
    }

    public function testAMissingAnchorIsNoAnchorNotNoTrigger(): void
    {
        $cal = IcsParser::parse(Behavior::read('duration/missing_anchor.ics'));
        [$parent, $alarm] = self::findAlarm($cal, 'alarm-no-anchor');

        $trigger = Duration::alarmTrigger($alarm);
        self::assertTrue($trigger->relative);

        $this->expectException(NoAnchorException::class);
        $trigger->resolve($parent, $cal);
    }

    /**
     * An explicit VALUE parameter is authoritative. A value contradicting
     * it is `ErrMalformed` rather than being silently re-read as the other
     * form -- which is what `value_contradiction.ics` exercises from both
     * directions.
     */
    public function testAnExplicitValueParameterIsAuthoritative(): void
    {
        $duration = new Property(
            'TRIGGER',
            [new Param('VALUE', 'DURATION')],
            '20260601T090000Z',
        );

        $dateTime = new Property(
            'TRIGGER',
            [new Param('VALUE', 'DATE-TIME')],
            '-PT15M',
        );

        foreach ([$duration, $dateTime] as $p) {
            try {
                Trigger::parse($p);
                self::fail('a contradicted VALUE parameter must not be re-read as the other form');
            } catch (MalformedException $e) {
                self::assertSame('ErrMalformed', $e->sentinel());
            }
        }
    }

    public function testAnUnsupportedValueParameterIsMalformed(): void
    {
        $this->expectException(MalformedException::class);

        Trigger::parse(new Property('TRIGGER', [new Param('VALUE', 'TEXT')], '-PT15M'));
    }

    /**
     * With no VALUE parameter the two value shapes are unambiguous, so the
     * value itself decides -- producers in the wild routinely omit it.
     */
    public function testTheFormIsInferredFromTheValueWhenNoValueParameterIsPresent(): void
    {
        $relative = Trigger::parse(new Property('TRIGGER', [], '-PT15M'));
        self::assertTrue($relative->relative);
        self::assertSame('-PT15M', (string) $relative->duration);

        $absolute = Trigger::parse(new Property('TRIGGER', [], '20260601T090000Z'));
        self::assertFalse($absolute->relative);
        self::assertNotNull($absolute->absolute);
        self::assertSame('20260601T090000Z', Time::formatTime($absolute->absolute));
    }

    public function testAValueThatIsNeitherFormIsMalformed(): void
    {
        $this->expectException(MalformedException::class);

        Trigger::parse(new Property('TRIGGER', [], 'tomorrow'));
    }

    /**
     * RFC 5545 §3.2.14 scopes RELATED to DURATION-valued triggers, so it
     * on an absolute trigger is rejected rather than ignored.
     */
    public function testRelatedOnAnAbsoluteTriggerIsRejected(): void
    {
        $this->expectException(MalformedException::class);

        Trigger::parse(new Property(
            'TRIGGER',
            [new Param('VALUE', 'DATE-TIME'), new Param('RELATED', 'END')],
            '20260601T090000Z',
        ));
    }

    public function testAnUnknownRelatedValueIsMalformed(): void
    {
        $this->expectException(MalformedException::class);

        Trigger::parse(new Property('TRIGGER', [new Param('RELATED', 'MIDDLE')], '-PT15M'));
    }

    /**
     * RELATED defaults to START when absent, per RFC 5545 §3.2.14.
     */
    public function testRelatedDefaultsToStart(): void
    {
        self::assertSame(Related::Start, Trigger::parse(new Property('TRIGGER', [], '-PT15M'))->related);
    }

    /**
     * Parameter names and their values fold case per RFC 5545 §3.2.
     */
    public function testParameterNamesAndValuesFoldCase(): void
    {
        $t = Trigger::parse(new Property(
            'TRIGGER',
            [new Param('related', 'end'), new Param('value', 'duration')],
            '-PT10M',
        ));

        self::assertTrue($t->relative);
        self::assertSame(Related::End, $t->related);
    }

    /**
     * A relative trigger leaves `RELATED=START` implicit on emit -- it is
     * the RFC default -- and an absolute trigger carries `VALUE=DATE-TIME`
     * explicitly so a consumer never has to infer the form.
     */
    public function testToPropertyEmitsTheConventionalWireShape(): void
    {
        $start = new Trigger(relative: true, duration: Duration::parse('-PT15M'));
        self::assertSame([], $start->toProperty()->params);
        self::assertSame('-PT15M', $start->toProperty()->value);

        $end = new Trigger(
            relative: true,
            duration: Duration::parse('-PT10M'),
            related: Related::End,
        );
        self::assertSame('RELATED', $end->toProperty()->params[0]->name);
        self::assertSame('END', $end->toProperty()->params[0]->value);

        $absolute = new Trigger(
            absolute: new \DateTimeImmutable('2026-06-01T09:00:00', new \DateTimeZone('UTC')),
        );
        self::assertSame('VALUE', $absolute->toProperty()->params[0]->name);
        self::assertSame('DATE-TIME', $absolute->toProperty()->params[0]->value);
        self::assertSame('20260601T090000Z', $absolute->toProperty()->value);
    }

    /**
     * `RELATED=END` on a VTODO anchors to DUE; everything else anchors to
     * DTEND, or to DTSTART plus DURATION.
     */
    public function testEventEndPrefersDtendOverDtstartPlusDuration(): void
    {
        $cal = new Calendar();
        $c = new Component(CompType::Event, [
            new Property('DTSTART', [], '20260601T090000Z'),
            new Property('DTEND', [], '20260601T170000Z'),
            new Property('DURATION', [], 'PT1H'),
        ]);

        $end = Duration::eventEnd($c, $cal);

        self::assertNotNull($end);
        self::assertSame('20260601T170000Z', Time::formatTime($end));
    }

    public function testEventEndFallsBackToDtstartPlusDuration(): void
    {
        $cal = new Calendar();
        $c = new Component(CompType::Event, [
            new Property('DTSTART', [], '20260601T090000Z'),
            new Property('DURATION', [], 'PT90M'),
        ]);

        $end = Duration::eventEnd($c, $cal);

        self::assertNotNull($end);
        self::assertSame('20260601T103000Z', Time::formatTime($end));
    }

    /**
     * @return iterable<string, array{list<Property>}>
     */
    public static function unresolvableEnds(): iterable
    {
        yield 'neither form' => [[new Property('DTSTART', [], '20260601T090000Z')]];

        yield 'DURATION without DTSTART' => [[new Property('DURATION', [], 'PT1H')]];

        yield 'malformed DURATION' => [[
            new Property('DTSTART', [], '20260601T090000Z'),
            new Property('DURATION', [], 'P1W2D'),
        ]];
    }

    /**
     * @param list<Property> $props
     */
    #[DataProvider('unresolvableEnds')]
    public function testEventEndReportsAbsenceRatherThanGuessing(array $props): void
    {
        self::assertNull(Duration::eventEnd(new Component(CompType::Event, $props), new Calendar()));
    }

    public function testAlarmRepeatCycleReadsThePair(): void
    {
        $alarm = new Component(CompType::Alarm, [
            new Property('DURATION', [], 'PT5M'),
            new Property('REPEAT', [], '3'),
        ]);

        [$d, $repeat] = Duration::alarmRepeatCycle($alarm);

        self::assertSame('PT5M', (string) $d);
        self::assertSame(3, $repeat);
    }

    public function testAlarmRepeatCycleIsZeroWhenNeitherPropertyIsPresent(): void
    {
        [$d, $repeat] = Duration::alarmRepeatCycle(new Component(CompType::Alarm, []));

        self::assertSame(0, $d->totalSeconds());
        self::assertSame(0, $repeat);
        self::assertInstanceOf(VDuration::class, $d);
    }

    /**
     * @return iterable<string, array{list<Property>}>
     */
    public static function brokenRepeatCycles(): iterable
    {
        yield 'DURATION without REPEAT' => [[new Property('DURATION', [], 'PT5M')]];
        yield 'REPEAT without DURATION' => [[new Property('REPEAT', [], '3')]];
        yield 'invalid DURATION' => [[
            new Property('DURATION', [], 'P1W2D'),
            new Property('REPEAT', [], '3'),
        ]];
        yield 'non-integer REPEAT' => [[
            new Property('DURATION', [], 'PT5M'),
            new Property('REPEAT', [], 'three'),
        ]];
        yield 'negative REPEAT' => [[
            new Property('DURATION', [], 'PT5M'),
            new Property('REPEAT', [], '-1'),
        ]];
    }

    /**
     * @param list<Property> $props
     */
    #[DataProvider('brokenRepeatCycles')]
    public function testAlarmRepeatCycleRejects(array $props): void
    {
        $this->expectException(MalformedException::class);

        Duration::alarmRepeatCycle(new Component(CompType::Alarm, $props));
    }

    /**
     * The VALARM named by `$uid`, with the top-level component enclosing
     * it -- the parent a relative trigger resolves against.
     *
     * @return array{Component, Component}
     */
    private static function findAlarm(Calendar $cal, string $uid): array
    {
        foreach ($cal->components as $parent) {
            foreach ($parent->sub as $alarm) {
                if ($alarm->uid() === $uid) {
                    return [$parent, $alarm];
                }
            }
        }

        self::fail("no VALARM with UID {$uid} in the fixture");
    }
}
