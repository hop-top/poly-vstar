<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Rrule;

use HopTop\Vstar\Exception\IterationCapException;
use HopTop\Vstar\Exception\UnsupportedRRuleException;

/**
 * The forward evaluator: one FREQ period expanded into concrete
 * occurrences via the BY-* filters, then stepped.
 *
 * # No IANA timezone database, ever
 *
 * Every calculation here is UTC `DateTimeImmutable` arithmetic. There is
 * no named zone and there must never be one -- no
 * `DateTimeZone('America/Montreal')`, no `date_default_timezone_get()`.
 * A V* implementation resolves local times only against the VTIMEZONE
 * definitions inside the document it is processing, which happens a
 * layer below this one; by the time a rule reaches the evaluator every
 * instant is absolute.
 *
 * Weekday numbering is `SU = 0` per RFC 5545 §3.3.10, which is what
 * `format('w')` reports -- so the numbering matches and no conversion
 * appears at the arithmetic sites. ISO-8601's `MO = 1` is reached only
 * through {@see Weekday::toWeekday()}, at the API boundary.
 *
 * Internal to the namespace: {@see Rrule} carries the public entry
 * points the API mapping names.
 */
final class Evaluator
{
    /**
     * The evaluator's iteration bound: the number of consecutive empty
     * FREQ periods walked before the search is abandoned.
     *
     * 100000 is large enough for every realistic recurrence -- a yearly
     * rule steps once per year, so the bound covers ~100k years of those
     * and ~273 years of daily ones -- and small enough to fail fast on a
     * rule that never yields, such as
     * `FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30`.
     *
     * The bound is a starvation guard, not a total-occurrence limit: the
     * counter resets whenever a period produces an occurrence, so a rule
     * that fires regularly runs as long as the caller wants.
     *
     * Deliberately not configurable: the only inputs that reach it are
     * unsatisfiable rules, for which a larger budget merely costs more
     * before failing identically.
     */
    public const MAX_ITERATIONS = 100000;

    /**
     * Refuse a rule the evaluator cannot walk.
     *
     * {@see Parser::parse()} guarantees both conditions, so this only
     * fires for a rule literal assembled by hand -- which is exactly why
     * it reports `ErrUnsupportedRRule` rather than trusting the value.
     */
    public static function checkExpandable(Rule $rule): void
    {
        if ($rule->freq === Freq::Invalid) {
            throw new UnsupportedRRuleException('rrule: FREQ is required');
        }

        if ($rule->interval < 1) {
            throw new UnsupportedRRuleException(
                'rrule: INTERVAL must be >= 1, got ' . $rule->interval,
            );
        }
    }

    /**
     * The `ErrIterationCap` failure, naming the bound that was hit.
     */
    public static function iterationCap(string $op, Rule $rule): IterationCapException
    {
        return new IterationCapException(sprintf(
            'rrule: %s: no occurrence within %d consecutive %s periods',
            $op,
            self::MAX_ITERATIONS,
            $rule->freq->value,
        ));
    }

    /**
     * Yield every occurrence of `$rule` from `$dtstart`, chronologically,
     * into `$yield`, stopping when it returns false.
     *
     * Returns true when {@see self::MAX_ITERATIONS} consecutive periods
     * produced nothing and the search was abandoned. That is **not**
     * termination -- the rule may well have occurrences beyond the
     * budget -- and the bounded entry points turn it into
     * `ErrIterationCap`.
     *
     * The bound is a starvation guard: the empty-period counter resets
     * whenever a period yields, so a regularly-firing rule runs as long
     * as the caller wants while one whose BY-* clauses can never match
     * still stops.
     *
     * @param callable(\DateTimeImmutable): bool $yield
     */
    public static function walk(Rule $rule, \DateTimeImmutable $dtstart, callable $yield): bool
    {
        try {
            self::checkExpandable($rule);
        } catch (UnsupportedRRuleException) {
            return false;
        }

        $emitted = 0;
        $empty = 0;
        $current = $dtstart;

        while ($empty < self::MAX_ITERATIONS) {
            $produced = false;

            foreach (self::periodOccurrences($rule, $current, $dtstart) as $occ) {
                // A period can reach back before dtstart -- a weekly
                // expansion covers the whole week dtstart falls in -- and
                // the series starts at dtstart, never earlier.
                if ($occ < $dtstart) {
                    continue;
                }

                // UNTIL is inclusive per RFC 5545 §3.3.10.
                if ($rule->until !== null && $occ > $rule->until) {
                    return false;
                }

                $produced = true;
                ++$emitted;

                if (!$yield($occ)) {
                    return false;
                }

                if ($rule->count > 0 && $emitted >= $rule->count) {
                    return false;
                }
            }

            $empty = $produced ? 0 : $empty + 1;
            $next = self::advance($rule, $current);

            if ($next === null) {
                return false;
            }

            $current = $next;
        }

        return true;
    }

    /**
     * One FREQ period, expanded, sorted, and positionally filtered.
     *
     * BYSETPOS is applied here, after every other BY-* clause and after
     * the sort, because RFC 5545 §3.3.10 defines it as a filter over the
     * fully expanded set of the period -- `-1` means "the last of
     * whatever the other clauses produced", so the ordering of these
     * three steps is the semantics, not an implementation choice.
     *
     * @return list<\DateTimeImmutable>
     */
    public static function periodOccurrences(
        Rule $rule,
        \DateTimeImmutable $base,
        \DateTimeImmutable $dtstart,
    ): array {
        $occs = self::expand($rule, $base, $dtstart);
        usort($occs, static fn (\DateTimeImmutable $a, \DateTimeImmutable $b): int => $a <=> $b);

        return $rule->bySetPos === [] ? $occs : self::applyBySetPos($occs, $rule->bySetPos);
    }

    /**
     * Step `$current` forward by one `FREQ x INTERVAL` period, or null
     * when the frequency is not one this evaluator steps.
     *
     * That null is a *termination* signal -- there are no more periods --
     * and is deliberately distinct from exhausting
     * {@see self::MAX_ITERATIONS}, which means periods remained but the
     * budget did not.
     *
     * Namespace-internal, and public only so {@see Rrule::all()} steps
     * identically: a generator is pull-shaped and cannot re-enter the
     * push-shaped walk, so it drives the same stepping directly rather
     * than duplicating the calendar arithmetic.
     */
    public static function advance(Rule $rule, \DateTimeImmutable $current): ?\DateTimeImmutable
    {
        $w = self::toWall($current);

        switch ($rule->freq) {
            case Freq::Hourly:
                return $current->modify('+' . $rule->interval . ' hours');

            case Freq::Daily:
                return $current->modify('+' . $rule->interval . ' days');

            case Freq::Weekly:
                return $current->modify('+' . (7 * $rule->interval) . ' days');

            case Freq::Monthly:
                // Land on day 1 of the target month rather than adding
                // months to the current day: adding a month to January
                // 31st would overflow into March, and the expansion below
                // reconstructs the real day from BYMONTHDAY or from
                // dtstart anyway.
                return self::fromWall(self::addMonths(
                    ['year' => $w['year'], 'month' => $w['month'], 'day' => 1, 'hour' => $w['hour'], 'minute' => $w['minute'], 'second' => $w['second']],
                    $rule->interval,
                ));

            case Freq::Yearly:
                // Same reasoning as MONTHLY, one level up: landing on
                // January 1st stops a February 29th rule sliding into
                // March in a non-leap year, because expandYearly takes
                // the month from dtstart.
                return self::fromWall([
                    'year' => $w['year'] + $rule->interval,
                    'month' => 1,
                    'day' => 1,
                    'hour' => $w['hour'],
                    'minute' => $w['minute'],
                    'second' => $w['second'],
                ]);

            case Freq::Invalid:
                return null;
        }
    }

    /**
     * Expand one FREQ period into its concrete occurrences, unsorted.
     *
     * @return list<\DateTimeImmutable>
     */
    private static function expand(Rule $rule, \DateTimeImmutable $base, \DateTimeImmutable $dtstart): array
    {
        return match ($rule->freq) {
            Freq::Hourly => self::expandHourly($rule, $base, $dtstart),
            Freq::Daily => self::expandDaily($rule, $base, $dtstart),
            Freq::Weekly => self::expandWeekly($rule, $base, $dtstart),
            Freq::Monthly => self::expandMonthly($rule, $base, $dtstart),
            Freq::Yearly => self::expandYearly($rule, $base, $dtstart),
            Freq::Invalid => [],
        };
    }

    /**
     * One occurrence per hour, at BYMINUTE x BYSECOND of the base hour.
     *
     * @return list<\DateTimeImmutable>
     */
    private static function expandHourly(Rule $rule, \DateTimeImmutable $base, \DateTimeImmutable $dtstart): array
    {
        if (!self::matchesByMonth($rule, $base)
            || !self::matchesByMonthDay($rule, $base)
            || !self::matchesByDay($rule, $base)) {
            return [];
        }

        $b = self::toWall($base);
        $s = self::toWall($dtstart);
        $out = [];

        // BYHOUR is a no-op for HOURLY: every hour is already its own
        // period.
        foreach (self::byOrDefault($rule->byMinute, $b['minute']) as $minute) {
            foreach (self::byOrDefault($rule->bySecond, $s['second']) as $second) {
                $out[] = self::fromWall([
                    'year' => $b['year'],
                    'month' => $b['month'],
                    'day' => $b['day'],
                    'hour' => $b['hour'],
                    'minute' => $minute,
                    'second' => $second,
                ]);
            }
        }

        return $out;
    }

    /**
     * One base day, with the time-of-day clauses crossed over it.
     *
     * @return list<\DateTimeImmutable>
     */
    private static function expandDaily(Rule $rule, \DateTimeImmutable $base, \DateTimeImmutable $dtstart): array
    {
        if (!self::matchesByMonth($rule, $base)
            || !self::matchesByMonthDay($rule, $base)
            || !self::matchesByDay($rule, $base)) {
            return [];
        }

        return self::crossTimeOfDay($rule, $base, $dtstart);
    }

    /**
     * With BYDAY, the seven days of the WKST-anchored week, filtered.
     * Without it, only dtstart's own weekday -- which is the base
     * itself, since the weekly step preserves the weekday.
     *
     * @return list<\DateTimeImmutable>
     */
    private static function expandWeekly(Rule $rule, \DateTimeImmutable $base, \DateTimeImmutable $dtstart): array
    {
        if ($rule->byDay === []) {
            if (!self::matchesByMonth($rule, $base) || !self::matchesByMonthDay($rule, $base)) {
                return [];
            }

            return self::crossTimeOfDay($rule, $base, $dtstart);
        }

        $weekStart = self::startOfWeek($base, $rule->weekStart->number());
        $out = [];

        for ($i = 0; $i < 7; ++$i) {
            $day = $weekStart->modify('+' . $i . ' days');

            if (!self::matchesByDay($rule, $day)
                || !self::matchesByMonth($rule, $day)
                || !self::matchesByMonthDay($rule, $day)) {
                continue;
            }

            foreach (self::crossTimeOfDay($rule, $day, $dtstart) as $t) {
                $out[] = $t;
            }
        }

        return $out;
    }

    /**
     * Every matching day of the base month.
     *
     * @return list<\DateTimeImmutable>
     */
    private static function expandMonthly(Rule $rule, \DateTimeImmutable $base, \DateTimeImmutable $dtstart): array
    {
        if (!self::matchesByMonth($rule, $base)) {
            return [];
        }

        $w = self::toWall($base);

        return self::expandMonthDays($rule, $w['year'], $w['month'], $dtstart);
    }

    /**
     * A year, expanded by whichever clause governs it: BYYEARDAY picks
     * days of the year, BYWEEKNO picks whole weeks, and otherwise the
     * BYMONTH months -- or dtstart's own month -- expand as monthly ones.
     *
     * @return list<\DateTimeImmutable>
     */
    private static function expandYearly(Rule $rule, \DateTimeImmutable $base, \DateTimeImmutable $dtstart): array
    {
        $year = self::toWall($base)['year'];

        if ($rule->byYearDay !== []) {
            return self::expandByYearDay($rule, $year, $dtstart);
        }

        if ($rule->byWeekNo !== []) {
            return self::expandByWeekNo($rule, $year, $dtstart);
        }

        // The yearly step resets the base to January to dodge day
        // overflow, so the anchor month has to come from dtstart, not
        // from the base.
        $months = $rule->byMonth !== [] ? $rule->byMonth : [self::toWall($dtstart)['month']];
        $out = [];

        foreach ($months as $month) {
            if ($month < 1 || $month > 12) {
                continue;
            }

            foreach (self::expandMonthDays($rule, $year, $month, $dtstart) as $t) {
                $out[] = $t;
            }
        }

        return $out;
    }

    /**
     * The BYYEARDAY days of a year, with BYMONTH and BYDAY narrowing
     * them. Negative entries count back from year-end, and an entry the
     * year does not have -- day 366 of a non-leap year -- is silently
     * dropped.
     *
     * @return list<\DateTimeImmutable>
     */
    private static function expandByYearDay(Rule $rule, int $year, \DateTimeImmutable $dtstart): array
    {
        $total = self::daysInYear($year);
        $out = [];

        foreach ($rule->byYearDay as $yd) {
            $doy = $yd > 0 ? $yd : $total + $yd + 1;

            if ($doy < 1 || $doy > $total) {
                continue;
            }

            $day = self::fromWall(['year' => $year, 'month' => 1, 'day' => 1, 'hour' => 0, 'minute' => 0, 'second' => 0])
                ->modify('+' . ($doy - 1) . ' days');

            if (!self::matchesByMonth($rule, $day) || !self::matchesByDay($rule, $day)) {
                continue;
            }

            foreach (self::crossTimeOfDay($rule, $day, $dtstart) as $t) {
                $out[] = $t;
            }
        }

        return $out;
    }

    /**
     * All seven days of each BYWEEKNO week, with BYMONTH and BYDAY
     * narrowing them. Negative entries count weeks from year-end, and a
     * week the year does not have -- 53 in a 52-week year -- is dropped.
     *
     * @return list<\DateTimeImmutable>
     */
    private static function expandByWeekNo(Rule $rule, int $year, \DateTimeImmutable $dtstart): array
    {
        $wkst = $rule->weekStart->number();
        $total = self::weeksInYear($year, $wkst);
        $out = [];

        foreach ($rule->byWeekNo as $wn) {
            $n = $wn > 0 ? $wn : $total + $wn + 1;

            if ($n < 1 || $n > $total) {
                continue;
            }

            $weekStart = self::startOfWeekN($year, $n, $wkst);

            for ($i = 0; $i < 7; ++$i) {
                $day = $weekStart->modify('+' . $i . ' days');

                if (!self::matchesByMonth($rule, $day) || !self::matchesByDay($rule, $day)) {
                    continue;
                }

                foreach (self::crossTimeOfDay($rule, $day, $dtstart) as $t) {
                    $out[] = $t;
                }
            }
        }

        return $out;
    }

    /**
     * Every occurrence in the days of one month that pass the filters.
     *
     * @return list<\DateTimeImmutable>
     */
    private static function expandMonthDays(
        Rule $rule,
        int $year,
        int $month,
        \DateTimeImmutable $dtstart,
    ): array {
        $dim = self::daysInMonth($year, $month);
        $out = [];

        foreach (self::monthDayCandidates($rule, $dim, self::toWall($dtstart)['day']) as $d) {
            // An out-of-range candidate is dropped, not clamped:
            // BYMONTHDAY=29 simply has no occurrence in a non-leap
            // February.
            if ($d < 1 || $d > $dim) {
                continue;
            }

            $day = self::fromWall([
                'year' => $year,
                'month' => $month,
                'day' => $d,
                'hour' => 0,
                'minute' => 0,
                'second' => 0,
            ]);

            if (!self::matchesByDayOrdinal($rule, $day)) {
                continue;
            }

            foreach (self::crossTimeOfDay($rule, $day, $dtstart) as $t) {
                $out[] = $t;
            }
        }

        return $out;
    }

    /**
     * The days-of-month to consider in a month of `$dim` days.
     *
     * With BYMONTHDAY, each entry resolved (negative from the end). With
     * BYDAY but no BYMONTHDAY, every day -- the BYDAY filter narrows
     * them. Otherwise the single day dtstart falls on, which is how an
     * unqualified monthly rule fires.
     *
     * A dtstart day past the end of a shorter month yields nothing,
     * which is RFC 5545's explicit skip-don't-clamp behaviour: a January
     * 31st monthly rule has no February occurrence rather than a
     * February 28th one.
     *
     * @return list<int>
     */
    private static function monthDayCandidates(Rule $rule, int $dim, int $dtstartDay): array
    {
        if ($rule->byMonthDay !== []) {
            return array_map(
                static fn (int $md): int => $md > 0 ? $md : $dim + $md + 1,
                $rule->byMonthDay,
            );
        }

        if ($rule->byDay !== []) {
            return range(1, $dim);
        }

        return $dtstartDay > $dim ? [] : [$dtstartDay];
    }

    /**
     * The cartesian product of BYHOUR x BYMINUTE x BYSECOND on the date
     * of `$day`. A time field with no BY-* clause is taken from
     * `$dtstart`, which is the RFC's anchor for everything a rule does
     * not constrain.
     *
     * @return list<\DateTimeImmutable>
     */
    private static function crossTimeOfDay(
        Rule $rule,
        \DateTimeImmutable $day,
        \DateTimeImmutable $dtstart,
    ): array {
        $d = self::toWall($day);
        $s = self::toWall($dtstart);
        $out = [];

        foreach (self::byOrDefault($rule->byHour, $s['hour']) as $hour) {
            foreach (self::byOrDefault($rule->byMinute, $s['minute']) as $minute) {
                foreach (self::byOrDefault($rule->bySecond, $s['second']) as $second) {
                    $out[] = self::fromWall([
                        'year' => $d['year'],
                        'month' => $d['month'],
                        'day' => $d['day'],
                        'hour' => $hour,
                        'minute' => $minute,
                        'second' => $second,
                    ]);
                }
            }
        }

        return $out;
    }

    /**
     * Filter a sorted occurrence list down to the 1-based positions
     * BYSETPOS names.
     *
     * Positive entries index from the start, negative from the end
     * (`-1` is the last). Out-of-range entries are dropped per RFC 5545
     * §3.3.10, and the result is de-duplicated -- two entries may
     * resolve to the same occurrence -- and left in chronological order.
     *
     * @param list<\DateTimeImmutable> $occs
     * @param list<int>                $setPos
     *
     * @return list<\DateTimeImmutable>
     */
    private static function applyBySetPos(array $occs, array $setPos): array
    {
        $n = count($occs);

        if ($n === 0) {
            return [];
        }

        $picked = [];

        foreach ($setPos as $p) {
            $idx = $p > 0 ? $p - 1 : $n + $p;

            if ($idx < 0 || $idx >= $n) {
                continue;
            }

            $picked[$idx] = true;
        }

        $out = [];

        foreach ($occs as $i => $occ) {
            if (isset($picked[$i])) {
                $out[] = $occ;
            }
        }

        return $out;
    }

    // -- BY-* matching ------------------------------------------------

    /**
     * Whether `$t`'s month satisfies BYMONTH (vacuously true when unset).
     */
    private static function matchesByMonth(Rule $rule, \DateTimeImmutable $t): bool
    {
        if ($rule->byMonth === []) {
            return true;
        }

        return in_array(self::toWall($t)['month'], $rule->byMonth, true);
    }

    /**
     * Whether `$t`'s day satisfies BYMONTHDAY, resolving negative
     * entries against the real length of `$t`'s own month -- so `-1` is
     * the last day of whichever month this is, 28 or 31.
     */
    private static function matchesByMonthDay(Rule $rule, \DateTimeImmutable $t): bool
    {
        if ($rule->byMonthDay === []) {
            return true;
        }

        $w = self::toWall($t);
        $dim = self::daysInMonth($w['year'], $w['month']);

        foreach ($rule->byMonthDay as $md) {
            $day = $md > 0 ? $md : $dim + $md + 1;

            if ($day === $w['day']) {
                return true;
            }
        }

        return false;
    }

    /**
     * Whether `$t`'s weekday appears in BYDAY, ignoring ordinals.
     *
     * Ordinals are meaningful only inside a MONTHLY or YEARLY period,
     * where {@see self::matchesByDayOrdinal()} honours them.
     */
    private static function matchesByDay(Rule $rule, \DateTimeImmutable $t): bool
    {
        if ($rule->byDay === []) {
            return true;
        }

        $wd = self::weekdayOf($t);

        foreach ($rule->byDay as $bd) {
            if ($bd->weekday->number() === $wd) {
                return true;
            }
        }

        return false;
    }

    /**
     * The BYDAY check for MONTHLY and YEARLY periods, where `2MO` means
     * "the second Monday of this month" and `-1FR` "the last Friday".
     */
    private static function matchesByDayOrdinal(Rule $rule, \DateTimeImmutable $t): bool
    {
        if ($rule->byDay === []) {
            return true;
        }

        $wd = self::weekdayOf($t);
        $w = self::toWall($t);
        $dim = self::daysInMonth($w['year'], $w['month']);

        foreach ($rule->byDay as $bd) {
            if ($bd->weekday->number() !== $wd) {
                continue;
            }

            if ($bd->ordinal === 0) {
                return true;
            }

            if ($bd->ordinal > 0) {
                // Days 1-7 hold the first of each weekday, 8-14 the
                // second, and so on.
                if ($bd->ordinal === intdiv($w['day'] - 1, 7) + 1) {
                    return true;
                }

                continue;
            }

            $last = self::lastWeekdayOfMonth($w['year'], $w['month'], $dim, $wd);

            if ($bd->ordinal === -(intdiv($last - $w['day'], 7) + 1)) {
                return true;
            }
        }

        return false;
    }

    /**
     * The day-of-month of the last `$weekday` in the given month.
     */
    private static function lastWeekdayOfMonth(int $year, int $month, int $dim, int $weekday): int
    {
        $lastWd = self::weekdayOf(self::fromWall([
            'year' => $year,
            'month' => $month,
            'day' => $dim,
            'hour' => 0,
            'minute' => 0,
            'second' => 0,
        ]));

        return $dim - (($lastWd - $weekday + 7) % 7);
    }

    // -- Calendar helpers ---------------------------------------------

    /**
     * A BY-* list if non-empty, else a single-element list of the
     * default -- the dtstart field the rule does not constrain.
     *
     * @param list<int> $list
     *
     * @return list<int>
     */
    private static function byOrDefault(array $list, int $fallback): array
    {
        return $list === [] ? [$fallback] : $list;
    }

    /**
     * The start of the week containing `$t`, anchored on `$wkst`.
     *
     * WKST-awareness lives here and nowhere else: every week boundary
     * the evaluator draws -- weekly expansion, BYWEEKNO numbering --
     * comes through this function, so a rule's week start is honoured
     * uniformly.
     */
    private static function startOfWeek(\DateTimeImmutable $t, int $wkst): \DateTimeImmutable
    {
        $delta = (self::weekdayOf($t) - $wkst + 7) % 7;

        return self::startOfDay($t)->modify('-' . $delta . ' days');
    }

    /**
     * The WKST-anchored start of week `$n` of `$year`.
     *
     * Week 1 is the week containing January 4th -- the ISO 8601 rule,
     * generalized to an arbitrary week start.
     */
    private static function startOfWeekN(int $year, int $n, int $wkst): \DateTimeImmutable
    {
        $jan4 = self::fromWall([
            'year' => $year,
            'month' => 1,
            'day' => 4,
            'hour' => 0,
            'minute' => 0,
            'second' => 0,
        ]);

        return self::startOfWeek($jan4, $wkst)->modify('+' . (7 * ($n - 1)) . ' days');
    }

    /**
     * The number of WKST-anchored weeks in `$year` under the same "week
     * 1 contains January 4th" rule.
     *
     * Most years have 52; a 53rd exists when the trailing days of
     * December still belong to a week of the current year rather than to
     * week 1 of the next.
     */
    private static function weeksInYear(int $year, int $wkst): int
    {
        $first = self::startOfWeekN($year, 1, $wkst);
        $next = self::startOfWeekN($year + 1, 1, $wkst);

        return intdiv($next->getTimestamp() - $first->getTimestamp(), 7 * 86400);
    }

    /**
     * Midnight UTC on the date `$t` falls on.
     */
    private static function startOfDay(\DateTimeImmutable $t): \DateTimeImmutable
    {
        $w = self::toWall($t);

        return self::fromWall([
            'year' => $w['year'],
            'month' => $w['month'],
            'day' => $w['day'],
            'hour' => 0,
            'minute' => 0,
            'second' => 0,
        ]);
    }

    /**
     * The RFC 5545 weekday number (`SU = 0`) of an instant.
     *
     * `format('w')` is the RFC numbering exactly; `format('N')` is
     * ISO-8601's and would be off by one for Sunday.
     */
    private static function weekdayOf(\DateTimeImmutable $t): int
    {
        return (int) $t->format('w');
    }

    /**
     * Days in a 1-based `$month` of `$year`, honouring the Gregorian
     * leap rule.
     */
    private static function daysInMonth(int $year, int $month): int
    {
        if ($month === 2) {
            $leap = ($year % 4 === 0 && $year % 100 !== 0) || $year % 400 === 0;

            return $leap ? 29 : 28;
        }

        return in_array($month, [4, 6, 9, 11], true) ? 30 : 31;
    }

    /**
     * 365 or 366, for the given Gregorian year.
     */
    private static function daysInYear(int $year): int
    {
        $leap = ($year % 4 === 0 && $year % 100 !== 0) || $year % 400 === 0;

        return $leap ? 366 : 365;
    }

    /**
     * Add `$n` months to a wall-clock field set, normalizing the month
     * into 1..12 and carrying into the year.
     *
     * The day is left alone: every caller passes day 1, precisely so the
     * addition cannot overflow a short month.
     *
     * @param array{year: int, month: int, day: int, hour: int, minute: int, second: int} $w
     *
     * @return array{year: int, month: int, day: int, hour: int, minute: int, second: int}
     */
    private static function addMonths(array $w, int $n): array
    {
        $total = ($w['year'] * 12) + ($w['month'] - 1) + $n;
        $w['year'] = intdiv($total, 12);
        $w['month'] = ($total % 12) + 1;

        return $w;
    }

    /**
     * Decompose an instant into its UTC wall-clock fields.
     *
     * @return array{year: int, month: int, day: int, hour: int, minute: int, second: int}
     */
    private static function toWall(\DateTimeImmutable $t): array
    {
        return [
            'year' => (int) $t->format('Y'),
            'month' => (int) $t->format('n'),
            'day' => (int) $t->format('j'),
            'hour' => (int) $t->format('G'),
            'minute' => (int) $t->format('i'),
            'second' => (int) $t->format('s'),
        ];
    }

    /**
     * Compose UTC wall-clock fields back into an instant.
     *
     * The zone is always UTC, built from the fixed `+00:00` offset --
     * never a named zone, which would read the system tzdata and make
     * two machines with different releases disagree.
     *
     * The fields are set numerically rather than formatted into a string
     * and re-parsed, because **no string form survives year 9999**. A
     * walk that steps a yearly rule to the iteration bound reaches
     * five-digit years, and there both textual routes fail: the
     * relative-string constructor reads `10000-01-01T09:00:00` as a
     * "double time specification", and `createFromFormat`'s `Y` token
     * matches at most four digits, so it rejects the same value. The
     * `setDate`/`setTime` pair takes integers and has no such ceiling.
     *
     * The base is the epoch rather than "now": a constructor reading the
     * current clock would leave microseconds on the result, and two runs
     * of the same expansion would produce instants that compare unequal.
     *
     * @param array{year: int, month: int, day: int, hour: int, minute: int, second: int} $w
     */
    private static function fromWall(array $w): \DateTimeImmutable
    {
        return (new \DateTimeImmutable('@0'))
            ->setTimezone(new \DateTimeZone('UTC'))
            ->setDate($w['year'], $w['month'], $w['day'])
            ->setTime($w['hour'], $w['minute'], $w['second']);
    }
}
