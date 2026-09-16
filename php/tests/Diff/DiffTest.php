<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Diff;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Card;
use HopTop\Vstar\Codec\Rfc5545\Parser;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Diff\ComponentDiff;
use HopTop\Vstar\Diff\Diff;
use HopTop\Vstar\Diff\DiffOp;
use HopTop\Vstar\Diff\PropertyDiff;
use HopTop\Vstar\Kind;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;
use HopTop\Vstar\Tests\Behavior;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * The `diff/*.diff.json` behavior family, plus the equality trio and the
 * human-readable rendering.
 *
 * Order is the contract here, not an implementation detail: components
 * appear in pairing order and each component's ops are ordered by
 * property name, case-insensitively. The comparison below is
 * order-sensitive throughout -- a port that sorted its output some other
 * way would still produce the same *set* of ops and must not pass.
 */
final class DiffTest extends TestCase
{
    /**
     * @return iterable<string, array{string}>
     */
    public static function diffCases(): iterable
    {
        foreach (Behavior::stems('diff', 'diff.json') as $stem) {
            yield $stem => [$stem];
        }
    }

    #[DataProvider('diffCases')]
    public function testCalendarDiffMatchesFixture(string $stem): void
    {
        /** @var list<array<string, mixed>> $expected */
        $expected = json_decode(
            Behavior::read("diff/{$stem}.diff.json"),
            true,
            512,
            JSON_THROW_ON_ERROR,
        );

        $a = Parser::parse(Behavior::read("diff/{$stem}.a.ics"));
        $b = Parser::parse(Behavior::read("diff/{$stem}.b.ics"));

        self::assertSame($expected, self::renderCalendarDiff(Diff::ofCalendar($a, $b)));
    }

    /**
     * Two equal documents produce no entry at all -- not an entry with no
     * ops.
     */
    public function testIdenticalCalendarsProduceAnEmptyList(): void
    {
        $a = Parser::parse(Behavior::read('diff/identical.a.ics'));
        $b = Parser::parse(Behavior::read('diff/identical.b.ics'));

        self::assertSame([], Diff::ofCalendar($a, $b));
    }

    /**
     * Ops within one component are ordered by property name,
     * case-insensitively -- and the assertion has to see the order, so it
     * compares the list, never a set.
     */
    public function testOpsAreOrderedByPropertyNameCaseInsensitively(): void
    {
        $a = new Component(CompType::Todo, [
            new Property('UID', [], 'u'),
        ]);
        $b = new Component(CompType::Todo, [
            new Property('UID', [], 'u'),
            new Property('zeta', [], '1'),
            new Property('ALPHA', [], '2'),
            new Property('Middle', [], '3'),
        ]);

        $names = array_map(
            static fn (PropertyDiff $pd): string => $pd->property->name,
            Diff::ofComponent($a, $b)->properties,
        );

        self::assertSame(['ALPHA', 'Middle', 'zeta'], $names);
    }

    /**
     * The same ordering rule holds through `ofCalendar`, and it is
     * case-**insensitive**.
     *
     * The committed fixtures cannot discriminate this: every one of them
     * carries all-uppercase property names already in order, so a
     * consumer that re-sorted them -- case-sensitively or otherwise --
     * would still match every file. Only mixed case separates the
     * contract's ordering from a naive byte sort, in which every
     * uppercase name precedes every lowercase one.
     */
    public function testCalendarOpsAreOrderedCaseInsensitively(): void
    {
        $a = new Calendar('p', [
            new Component(CompType::Todo, [new Property('UID', [], 't')]),
        ]);
        $b = new Calendar('p', [
            new Component(CompType::Todo, [
                new Property('UID', [], 't'),
                new Property('zeta', [], '1'),
                new Property('ALPHA', [], '2'),
                new Property('middle', [], '3'),
                new Property('Beta', [], '4'),
            ]),
        ]);

        $diffs = Diff::ofCalendar($a, $b);
        self::assertCount(1, $diffs);

        $names = array_map(
            static fn (PropertyDiff $pd): string => $pd->property->name,
            $diffs[0]->properties,
        );

        // A case-sensitive sort would yield ALPHA, Beta, middle, zeta in
        // a different arrangement: uppercase sorts before lowercase, so
        // "zeta" would trail and "middle" would follow "Beta" only by
        // accident. The contract folds case before comparing.
        self::assertSame(['ALPHA', 'Beta', 'middle', 'zeta'], $names);
    }

    /**
     * Ordering survives an all-lowercase input too, which a sort keyed on
     * the raw name would reorder relative to the folded one.
     */
    public function testCalendarOpsFoldCaseRatherThanByteSort(): void
    {
        $a = new Calendar('p', [
            new Component(CompType::Todo, [new Property('UID', [], 't')]),
        ]);
        $b = new Calendar('p', [
            new Component(CompType::Todo, [
                new Property('UID', [], 't'),
                new Property('aaa', [], '1'),
                new Property('BBB', [], '2'),
            ]),
        ]);

        $names = array_map(
            static fn (PropertyDiff $pd): string => $pd->property->name,
            Diff::ofCalendar($a, $b)[0]->properties,
        );

        // Folded: aaa < BBB. Raw bytes: BBB < aaa.
        self::assertSame(['aaa', 'BBB'], $names);
    }

    /**
     * Components come back in pairing order: UID-bearing pairs sorted by
     * (type, uid) first, then the UID-less buckets by type.
     */
    public function testComponentsComeBackInPairingOrder(): void
    {
        $a = new Calendar('p', []);
        $b = new Calendar('p', [
            self::todo('zulu'),
            self::todo('alpha'),
            new Component(CompType::Event, [new Property('UID', [], 'mike')]),
        ]);

        $paths = array_map(
            static fn (ComponentDiff $d): string => $d->path,
            Diff::ofCalendar($a, $b),
        );

        self::assertSame([
            'VCALENDAR.VEVENT[uid=mike]',
            'VCALENDAR.VTODO[uid=alpha]',
            'VCALENDAR.VTODO[uid=zulu]',
        ], $paths);
    }

    /**
     * X-VSTAR-HASH is excluded from both sides: a diff reports what
     * changed in the content, not the restamped hash that followed.
     */
    public function testHashPropertyIsExcludedFromBothSides(): void
    {
        $a = new Component(CompType::Todo, [
            new Property('UID', [], 'u'),
            new Property('X-VSTAR-HASH', [], 'sha256:aaa'),
        ]);
        $b = new Component(CompType::Todo, [
            new Property('UID', [], 'u'),
            new Property('x-vstar-hash', [], 'sha256:bbb'),
        ]);

        self::assertTrue(Diff::ofComponent($a, $b)->isEmpty());
    }

    /**
     * A parameter-only change is a `change` op whose values are equal and
     * whose parameter lists differ.
     */
    public function testParameterOnlyChangeIsAChangeOp(): void
    {
        $a = new Component(CompType::Todo, [
            new Property('DUE', [new Param('TZID', 'America/Montreal')], '20260101T000000'),
        ]);
        $b = new Component(CompType::Todo, [
            new Property('DUE', [new Param('TZID', 'Europe/Paris')], '20260101T000000'),
        ]);

        $props = Diff::ofComponent($a, $b)->properties;
        self::assertCount(1, $props);
        self::assertSame(DiffOp::Changed, $props[0]->op);
        self::assertSame($props[0]->old?->value, $props[0]->property->value);
    }

    /**
     * Multi-valued properties are paired in order, the i-th against the
     * i-th, with the surplus becoming add/remove.
     */
    public function testMultiValuedPropertiesPairPositionally(): void
    {
        $a = new Component(CompType::Event, [
            new Property('ATTENDEE', [], 'a'),
            new Property('ATTENDEE', [], 'b'),
        ]);
        $b = new Component(CompType::Event, [
            new Property('ATTENDEE', [], 'a'),
            new Property('ATTENDEE', [], 'B2'),
            new Property('ATTENDEE', [], 'c'),
        ]);

        $props = Diff::ofComponent($a, $b)->properties;
        self::assertCount(2, $props);
        self::assertSame(DiffOp::Changed, $props[0]->op);
        self::assertSame('B2', $props[0]->property->value);
        self::assertSame(DiffOp::Added, $props[1]->op);
        self::assertSame('c', $props[1]->property->value);
    }

    /**
     * Sub-components without a UID pair positionally by index within the
     * type bucket.
     */
    public function testSubComponentsWithoutUidPairPositionally(): void
    {
        $a = new Component(CompType::Event, [new Property('UID', [], 'e')], [
            new Component(CompType::Alarm, [new Property('ACTION', [], 'DISPLAY')]),
        ]);
        $b = new Component(CompType::Event, [new Property('UID', [], 'e')], [
            new Component(CompType::Alarm, [new Property('ACTION', [], 'AUDIO')]),
        ]);

        $subs = Diff::ofComponent($a, $b)->subDiffs;
        self::assertCount(1, $subs);
        self::assertSame('VALARM[#0]', $subs[0]->path);
        self::assertSame(DiffOp::Changed, $subs[0]->properties[0]->op);
    }

    public function testOfCardDiffsPropertiesAndHasNoSubDiffs(): void
    {
        $a = new Card('u', Kind::Individual, [new Property('FN', [], 'Jad')]);
        $b = new Card('u', Kind::Individual, [new Property('FN', [], 'Jad B')]);

        $d = Diff::ofCard($a, $b);
        self::assertSame([], $d->subDiffs);
        self::assertCount(1, $d->properties);
        self::assertSame(DiffOp::Changed, $d->properties[0]->op);
    }

    /**
     * The equality trio is canonical-byte equality, so property and
     * parameter order are irrelevant and X-VSTAR-HASH is ignored.
     */
    public function testComponentEqualIgnoresOrderingAndHash(): void
    {
        $a = new Component(CompType::Todo, [
            new Property('UID', [], 'u'),
            new Property('SUMMARY', [], 's'),
            new Property('X-VSTAR-HASH', [], 'sha256:aaa'),
        ]);
        $b = new Component(CompType::Todo, [
            new Property('SUMMARY', [], 's'),
            new Property('UID', [], 'u'),
        ]);

        self::assertTrue(Diff::componentEqual($a, $b));
        self::assertFalse(Diff::componentEqual(
            $a,
            new Component(CompType::Todo, [new Property('UID', [], 'other')]),
        ));
    }

    public function testCardEqualAndCalendarEqual(): void
    {
        $a = new Card('u', Kind::Individual, [new Property('FN', [], 'Jad')]);
        $b = new Card('u', Kind::Individual, [new Property('FN', [], 'Jad')]);
        self::assertTrue(Diff::cardEqual($a, $b));
        self::assertFalse(Diff::cardEqual($a, new Card('v', Kind::Individual, [])));

        $ca = new Calendar('p', [self::todo('t')]);
        $cb = new Calendar('p', [self::todo('t')]);
        self::assertTrue(Diff::calendarEqual($ca, $cb));
        self::assertFalse(Diff::calendarEqual($ca, new Calendar('p', [self::todo('u')])));
    }

    /**
     * `ComponentDiff` is a value class, not an enum, so the magic method
     * is the correct spelling here.
     */
    public function testComponentDiffRendersAsText(): void
    {
        $a = new Component(CompType::Todo, [
            new Property('UID', [], 'u'),
            new Property('SUMMARY', [], 'old'),
            new Property('GONE', [], 'x'),
        ]);
        $b = new Component(CompType::Todo, [
            new Property('UID', [], 'u'),
            new Property('SUMMARY', [], 'new'),
            new Property('ADDED', [new Param('P', 'v')], 'y'),
        ]);

        $text = (string) Diff::ofComponent($a, $b);

        self::assertStringContainsString('--- ', $text);
        self::assertStringContainsString('+ ADDED;P=v:y', $text);
        self::assertStringContainsString('- GONE:x', $text);
        self::assertStringContainsString('~ SUMMARY: old -> new', $text);
    }

    public function testEmptyDiffRendersAsTheEmptyString(): void
    {
        $c = self::todo('u');

        self::assertSame('', (string) Diff::ofComponent($c, $c));
        self::assertTrue(Diff::ofComponent($c, $c)->isEmpty());
    }

    public function testDiffOpToStringUsesTheReferenceSpelling(): void
    {
        self::assertSame('Added', DiffOp::Added->toString());
        self::assertSame('Removed', DiffOp::Removed->toString());
        self::assertSame('Changed', DiffOp::Changed->toString());
    }

    /**
     * Render a calendar diff into the fixture's JSON shape so the two can
     * be compared directly.
     *
     * @param list<ComponentDiff> $diffs
     *
     * @return list<array<string, mixed>>
     */
    private static function renderCalendarDiff(array $diffs): array
    {
        $out = [];

        foreach ($diffs as $d) {
            $entry = [
                'uid' => self::uidFromPath($d->path),
                'path' => $d->path,
                'ops' => self::renderOps($d->properties),
            ];

            $subs = [];

            foreach ($d->subDiffs as $sub) {
                if (!$sub->isEmpty()) {
                    $subs[] = [
                        'uid' => self::uidFromPath($sub->path),
                        'path' => $sub->path,
                        'ops' => self::renderOps($sub->properties),
                    ];
                }
            }

            if ($subs !== []) {
                $entry['subs'] = $subs;
            }

            $out[] = $entry;
        }

        return $out;
    }

    /**
     * @param list<PropertyDiff> $props
     *
     * @return list<array<string, mixed>>
     */
    private static function renderOps(array $props): array
    {
        $out = [];

        foreach ($props as $pd) {
            $op = match ($pd->op) {
                DiffOp::Added => 'add',
                DiffOp::Removed => 'remove',
                DiffOp::Changed => 'change',
            };

            $before = $pd->op === DiffOp::Added ? null : ($pd->op === DiffOp::Removed
                ? $pd->property->value
                : $pd->old?->value);
            $after = $pd->op === DiffOp::Removed ? null : $pd->property->value;

            $entry = [
                'op' => $op,
                'property' => $pd->property->name,
                'before' => $before,
                'after' => $after,
            ];

            $beforeProp = $pd->op === DiffOp::Removed ? $pd->property : $pd->old;
            $afterProp = $pd->op === DiffOp::Removed ? null : $pd->property;

            if ($beforeProp !== null && $beforeProp->params !== []) {
                $entry['before_params'] = self::renderParams($beforeProp->params);
            }

            if ($afterProp !== null && $afterProp->params !== []) {
                $entry['after_params'] = self::renderParams($afterProp->params);
            }

            $out[] = $entry;
        }

        return $out;
    }

    /**
     * @param list<Param> $params
     *
     * @return list<array{name: string, value: string}>
     */
    private static function renderParams(array $params): array
    {
        return array_map(
            static fn (Param $p): array => ['name' => $p->name, 'value' => $p->value],
            $params,
        );
    }

    /**
     * Lift the UID out of a path locator, or the empty string when the
     * component is addressed positionally.
     */
    private static function uidFromPath(string $path): string
    {
        if (preg_match('/\[uid=(.*)\]$/', $path, $m) === 1) {
            return $m[1];
        }

        return '';
    }

    private static function todo(string $uid): Component
    {
        return new Component(CompType::Todo, [
            new Property('UID', [], $uid),
            new Property('DTSTAMP', [], '20260504T120000Z'),
        ]);
    }
}
