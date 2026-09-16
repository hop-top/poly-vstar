<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Rrule;

use HopTop\Vstar\Exception\IterationCapException;
use HopTop\Vstar\Exception\UnboundedExpansionException;
use HopTop\Vstar\Exception\UnsupportedRRuleException;
use HopTop\Vstar\Exception\VstarException;
use HopTop\Vstar\Rrule\Freq;
use HopTop\Vstar\Rrule\Rrule;
use HopTop\Vstar\Rrule\Rule;
use HopTop\Vstar\Time;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * Forward evaluation, gated by `rrule/evaluator/*.next.json` and
 * `rrule/expansion/`.
 *
 * The `.next.json` walk has a subtlety worth stating once. After
 * stepping `expected.length` times, the test steps **one past the end**,
 * and what that step must produce is a three-way decision:
 *
 * - the sidecar names an `error`   -> that sentinel;
 * - the rule carries COUNT or UNTIL -> null, ordinary termination;
 * - otherwise                       -> still non-null.
 *
 * The third case is the one that bites: most fixtures here are unbounded
 * (`FREQ=DAILY`, `FREQ=YEARLY;BYMONTH=...`), and asserting "the step past
 * the end is always null" would pass on the two terminating fixtures and
 * silently wrong-assert the rest.
 */
final class EvaluatorTest extends TestCase
{
    /**
     * @return iterable<string, array{string}>
     */
    public static function nextFixtures(): iterable
    {
        foreach (Sidecar::rruleStems('evaluator') as $stem) {
            if (Sidecar::outcome($stem, 'next.json') !== null) {
                yield $stem => [$stem];
            }
        }
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function expandFixtures(): iterable
    {
        foreach (Sidecar::rruleStems('expansion') as $stem) {
            if (Sidecar::outcome($stem, 'expand.json') !== null) {
                yield $stem => [$stem];
            }
        }
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function betweenFixtures(): iterable
    {
        foreach (Sidecar::rruleStems('expansion') as $stem) {
            if (Sidecar::outcome($stem, 'between.json') !== null) {
                yield $stem => [$stem];
            }
        }
    }

    #[DataProvider('nextFixtures')]
    public function testNextOccurrenceWalksItsSidecar(string $stem): void
    {
        $spec = Sidecar::outcome($stem, 'next.json');
        self::assertNotNull($spec);

        $rule = Rrule::parse(Sidecar::rruleValue($stem));
        $dtstart = self::instant($spec['dtstart']);
        $after = self::instant($spec['after']);

        foreach ($spec['expected'] as $i => $want) {
            $got = Rrule::nextOccurrence($rule, $dtstart, $after);

            self::assertNotNull($got, "{$stem}: step {$i} terminated, want {$want}");
            self::assertSame($want, Time::formatTime($got), "{$stem}: step {$i}");
            $after = $got;
        }

        self::assertStepPastTheEnd($stem, $rule, $dtstart, $after, $spec['error']);
    }

    /**
     * The step after the last expected occurrence, checked three ways.
     */
    private static function assertStepPastTheEnd(
        string $stem,
        Rule $rule,
        \DateTimeImmutable $dtstart,
        \DateTimeImmutable $after,
        ?string $wantError,
    ): void {
        if ($wantError !== null) {
            try {
                $got = Rrule::nextOccurrence($rule, $dtstart, $after);
            } catch (VstarException $e) {
                self::assertSame($wantError, $e->sentinel(), "{$stem}: wrong sentinel past the end");

                return;
            }

            self::fail(sprintf(
                '%s: expected %s past the end, got %s',
                $stem,
                $wantError,
                $got === null ? 'termination' : Time::formatTime($got),
            ));
        }

        $got = Rrule::nextOccurrence($rule, $dtstart, $after);

        // A rule with COUNT or UNTIL ends; one with neither does not, and
        // demanding null from an unbounded rule would assert a bug.
        if ($rule->count > 0 || $rule->until !== null) {
            self::assertNull($got, "{$stem}: a bounded rule must terminate past its last occurrence");

            return;
        }

        self::assertNotNull($got, "{$stem}: an unbounded rule must keep yielding past the sidecar");
    }

    #[DataProvider('expandFixtures')]
    public function testOccurrencesMatchesItsSidecar(string $stem): void
    {
        $spec = Sidecar::outcome($stem, 'expand.json');
        self::assertNotNull($spec);
        self::assertNotNull($spec['limit']);

        $rule = Rrule::parse(Sidecar::rruleValue($stem));
        $result = Rrule::occurrences($rule, self::instant($spec['dtstart']), $spec['limit']);

        self::assertSame($spec['expected'], array_map(
            static fn (\DateTimeImmutable $t): string => Time::formatTime($t),
            $result['occurrences'],
        ));
        self::assertSame($spec['complete'], $result['complete'], "{$stem}: complete flag");
    }

    #[DataProvider('betweenFixtures')]
    public function testBetweenMatchesItsSidecar(string $stem): void
    {
        $spec = Sidecar::outcome($stem, 'between.json');
        self::assertNotNull($spec);

        $rule = Rrule::parse(Sidecar::rruleValue($stem));
        $got = Rrule::between(
            $rule,
            self::instant($spec['dtstart']),
            self::instant($spec['start']),
            self::instant($spec['end']),
        );

        self::assertSame($spec['expected'], array_map(
            static fn (\DateTimeImmutable $t): string => Time::formatTime($t),
            $got,
        ));
    }

    /**
     * `all()` is lazy: taking five values from an unbounded rule returns,
     * which it could not do if the generator materialized first.
     */
    public function testAllIsLazyOnAnUnboundedRule(): void
    {
        $rule = Rrule::parse('FREQ=DAILY');
        $dtstart = self::instant('20260401T120000Z');

        $got = [];

        foreach (Rrule::all($rule, $dtstart) as $t) {
            $got[] = Time::formatTime($t);

            if (count($got) === 5) {
                break;
            }
        }

        self::assertSame([
            '20260401T120000Z',
            '20260402T120000Z',
            '20260403T120000Z',
            '20260404T120000Z',
            '20260405T120000Z',
        ], $got);
    }

    /**
     * `all()` on a finite rule ends on its own, and its values are the
     * ones `occurrences()` reports.
     */
    public function testAllTerminatesOnABoundedRule(): void
    {
        $rule = Rrule::parse('FREQ=DAILY;COUNT=3');
        $dtstart = self::instant('20260401T120000Z');

        $got = array_map(
            static fn (\DateTimeImmutable $t): string => Time::formatTime($t),
            iterator_to_array(Rrule::all($rule, $dtstart), false),
        );

        self::assertSame([
            '20260401T120000Z',
            '20260402T120000Z',
            '20260403T120000Z',
        ], $got);
    }

    /**
     * The iteration cap is not termination. An unsatisfiable rule has no
     * occurrence to find and no end to reach, and the two answers are
     * different: null would say "the series ended", which is false.
     */
    public function testUnsatisfiableRuleRaisesTheIterationCap(): void
    {
        $rule = Rrule::parse('FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30');
        $dtstart = self::instant('20260101T090000Z');

        $this->expectException(IterationCapException::class);

        Rrule::nextOccurrence($rule, $dtstart, $dtstart);
    }

    /**
     * The same rule through the bounded entry points: a cap, never an
     * empty-but-complete answer.
     */
    public function testUnsatisfiableRuleCapsThroughOccurrences(): void
    {
        $rule = Rrule::parse('FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30');

        $this->expectException(IterationCapException::class);

        Rrule::occurrences($rule, self::instant('20260101T090000Z'), 5);
    }

    public function testUnsatisfiableRuleCapsThroughBetween(): void
    {
        $rule = Rrule::parse('FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30');

        $this->expectException(IterationCapException::class);

        Rrule::between(
            $rule,
            self::instant('20260101T090000Z'),
            self::instant('20260101T090000Z'),
            self::instant('20270101T090000Z'),
        );
    }

    /**
     * The cap sentinel is `ErrIterationCap`, distinct from every other
     * failure the evaluator can report.
     */
    public function testIterationCapCarriesItsSentinel(): void
    {
        $rule = Rrule::parse('FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30');

        try {
            Rrule::occurrences($rule, self::instant('20260101T090000Z'), 1);
            self::fail('expected ErrIterationCap');
        } catch (VstarException $e) {
            self::assertSame('ErrIterationCap', $e->sentinel());
        }
    }

    /**
     * A window with no end, or one not after its start, is an unbounded
     * expansion request -- not an empty result.
     *
     * @return iterable<string, array{string, string}>
     */
    public static function unboundedWindows(): iterable
    {
        yield 'equal' => ['20260403T120000Z', '20260403T120000Z'];
        yield 'inverted' => ['20260406T120000Z', '20260403T120000Z'];
    }

    #[DataProvider('unboundedWindows')]
    public function testDegenerateWindowIsUnbounded(string $start, string $end): void
    {
        $rule = Rrule::parse('FREQ=DAILY');

        $this->expectException(UnboundedExpansionException::class);

        Rrule::between($rule, self::instant('20260401T120000Z'), self::instant($start), self::instant($end));
    }

    /**
     * A missing end is the same failure. PHP spells "missing" as null,
     * so the nullable parameter is where the spec's "no end" arrives.
     */
    public function testMissingEndIsUnbounded(): void
    {
        $rule = Rrule::parse('FREQ=DAILY');

        $this->expectException(UnboundedExpansionException::class);

        Rrule::between($rule, self::instant('20260401T120000Z'), self::instant('20260403T120000Z'), null);
    }

    /**
     * A negative limit is unbounded too: it names no bound at all.
     */
    public function testNegativeLimitIsUnbounded(): void
    {
        $rule = Rrule::parse('FREQ=DAILY');

        $this->expectException(UnboundedExpansionException::class);

        Rrule::occurrences($rule, self::instant('20260401T120000Z'), -1);
    }

    /**
     * A zero limit yields nothing and claims nothing: `complete` is
     * false, because no occurrences were asked for and none of the
     * series was observed to end.
     */
    public function testZeroLimitIsEmptyAndIncomplete(): void
    {
        $rule = Rrule::parse('FREQ=DAILY');
        $result = Rrule::occurrences($rule, self::instant('20260401T120000Z'), 0);

        self::assertSame([], $result['occurrences']);
        self::assertFalse($result['complete']);
    }

    /**
     * The window is half-open: `start` is included, `end` is not.
     */
    public function testWindowIsHalfOpen(): void
    {
        $rule = Rrule::parse('FREQ=DAILY');
        $got = Rrule::between(
            $rule,
            self::instant('20260401T120000Z'),
            self::instant('20260402T120000Z'),
            self::instant('20260404T120000Z'),
        );

        self::assertSame(
            ['20260402T120000Z', '20260403T120000Z'],
            array_map(static fn (\DateTimeImmutable $t): string => Time::formatTime($t), $got),
        );
    }

    /**
     * A rule the evaluator will not walk is `ErrUnsupportedRRule`, not a
     * silent empty answer. The parser cannot produce such a rule, so
     * this reaches the evaluator only from a hand-built literal -- which
     * is exactly why the check is there.
     */
    public function testUnwalkableRuleIsUnsupported(): void
    {
        $this->expectException(UnsupportedRRuleException::class);

        Rrule::nextOccurrence(
            new Rule(Freq::Invalid),
            self::instant('20260401T120000Z'),
            self::instant('20260401T120000Z'),
        );
    }

    /**
     * BYMONTHDAY=29 has no occurrence in a non-leap February: RFC 5545
     * skips, it does not clamp to the 28th.
     */
    public function testByMonthDay29SkipsNonLeapFebruary(): void
    {
        $rule = Rrule::parse('FREQ=MONTHLY;BYMONTHDAY=29');
        $result = Rrule::occurrences($rule, self::instant('20260129T120000Z'), 4);

        self::assertSame([
            '20260129T120000Z',
            '20260329T120000Z',
            '20260429T120000Z',
            '20260529T120000Z',
        ], array_map(static fn (\DateTimeImmutable $t): string => Time::formatTime($t), $result['occurrences']));
    }

    /**
     * BYMONTHDAY=-1 resolves against the real length of each month, so
     * it lands on the 31st, the 28th and the 30th in turn.
     */
    public function testByMonthDayMinusOneIsTheLastDay(): void
    {
        $rule = Rrule::parse('FREQ=MONTHLY;BYMONTHDAY=-1');
        $result = Rrule::occurrences($rule, self::instant('20260131T120000Z'), 4);

        self::assertSame([
            '20260131T120000Z',
            '20260228T120000Z',
            '20260331T120000Z',
            '20260430T120000Z',
        ], array_map(static fn (\DateTimeImmutable $t): string => Time::formatTime($t), $result['occurrences']));
    }

    /**
     * WKST moves the week boundary, so the same BYDAY list under two
     * week starts groups days into different weeks -- which an
     * INTERVAL=2 weekly rule makes visible.
     *
     * With WKST=MO the week runs Mon..Sun, so 2026-01-04 (a Sunday)
     * belongs to the week starting 2025-12-29; with WKST=SU it starts a
     * week of its own.
     */
    public function testWeekStartMovesTheWeekBoundary(): void
    {
        $dtstart = self::instant('20260101T000000Z');

        $mo = Rrule::occurrences(Rrule::parse('FREQ=WEEKLY;INTERVAL=2;BYDAY=TH,SU;WKST=MO'), $dtstart, 4);
        $su = Rrule::occurrences(Rrule::parse('FREQ=WEEKLY;INTERVAL=2;BYDAY=TH,SU;WKST=SU'), $dtstart, 4);

        $fmt = static fn (array $r): array => array_map(
            static fn (\DateTimeImmutable $t): string => Time::formatTime($t),
            $r,
        );

        self::assertNotSame(
            $fmt($mo['occurrences']),
            $fmt($su['occurrences']),
            'WKST did not change the week grouping',
        );
    }

    /**
     * BYSETPOS filters the period's fully expanded set, so `-1` means
     * "the last of whatever the other clauses produced" -- the last
     * weekday of the month, not the last day.
     */
    public function testBySetPosSelectsFromTheExpandedSet(): void
    {
        $rule = Rrule::parse('FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1');
        $result = Rrule::occurrences($rule, self::instant('20260101T000000Z'), 4);

        self::assertSame([
            '20260130T000000Z',
            '20260227T000000Z',
            '20260331T000000Z',
            '20260430T000000Z',
        ], array_map(static fn (\DateTimeImmutable $t): string => Time::formatTime($t), $result['occurrences']));
    }

    /**
     * An instant from a corpus sidecar. Form #2 is the only form the
     * corpus uses, and a value that will not parse is a broken fixture.
     */
    private static function instant(?string $s): \DateTimeImmutable
    {
        self::assertNotNull($s, 'sidecar is missing a required instant');
        $t = Time::parseTime($s);
        self::assertNotNull($t, "sidecar instant {$s} is not RFC 5545 form #2");

        return $t;
    }
}
