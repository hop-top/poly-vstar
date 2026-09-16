<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Rrule;

use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\UnboundedExpansionException;
use HopTop\Vstar\Time;

/**
 * RFC 5545 §3.3.10 recurrence: parsing, validation, the fixed wire form,
 * forward evaluation, and bounded and lazy expansion.
 *
 * The accepted scope is fixed by
 * `spec/v0.1/03-canonicalization.md` §RRULE parsing scope: a `FREQ` of
 * `MINUTELY`, `HOURLY`, `DAILY`, `WEEKLY`, `MONTHLY` or `YEARLY`, with
 * `INTERVAL`, `UNTIL` (UTC form #2 only), `COUNT`, every `BY-*` clause,
 * and `WKST`. `FREQ=SECONDLY` and `RSCALE` are recognized by name and
 * reported `ErrUnsupportedRRule`.
 *
 * Parser scope equals evaluator scope: everything the parser accepts,
 * the evaluator evaluates.
 *
 * PHP has no free functions at namespace scope in the idiomatic style,
 * so the reference's package-level functions land here as static
 * methods -- the same convention {@see Time} follows for the root
 * package.
 *
 * All arithmetic is UTC `DateTimeImmutable` arithmetic. There is no IANA
 * timezone database here and there must never be one: zone resolution
 * against a document's own VTIMEZONE happens a layer below, so every
 * instant reaching this namespace is already absolute.
 */
final class Rrule
{
    /**
     * The evaluator's iteration bound. See
     * {@see Evaluator::MAX_ITERATIONS} for what the number means and why
     * it is not configurable.
     */
    public const MAX_ITERATIONS = Evaluator::MAX_ITERATIONS;

    /**
     * Parse an RRULE property value -- no `RRULE:` prefix -- into a
     * {@see Rule}.
     *
     * Throws `MalformedException` (`ErrMalformed`) for a syntactic
     * failure and `UnsupportedRRuleException` (`ErrUnsupportedRRule`)
     * for a feature this scope defers. The two are different answers and
     * the `rrule/rejected/` corpus asserts which.
     */
    public static function parse(string $s): Rule
    {
        return Parser::parse($s);
    }

    /**
     * Check that `$s` would parse cleanly, discarding the result.
     *
     * Returns nothing and throws exactly what {@see self::parse()}
     * would. Deliberately not a boolean: the *identity* of the failure
     * is the payload, and the `rrule/rejected/` fixtures assert it.
     */
    public static function validate(string $s): void
    {
        Parser::parse($s);
    }

    /**
     * The next occurrence strictly after `$after`, or null when the rule
     * has terminated.
     *
     * The two channels this call carries are the reason it exists in
     * this shape:
     *
     * - **null is exhaustion.** UNTIL passed, COUNT spent, no further
     *   period. This is not an error; it is the ordinary end of a finite
     *   series, and a caller walking a rule to its end hits it once.
     * - **A throw is a rule the evaluator will not or cannot evaluate.**
     *   `ErrUnsupportedRRule` for a rule outside the parsing scope, or
     *   `ErrIterationCap` when the bound is reached without finding an
     *   occurrence -- which is how an unsatisfiable rule such as
     *   `FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30` surfaces, since the parser
     *   stays permissive about such combinations.
     *
     * Collapsing the two -- treating exhaustion as an error, or
     * swallowing the cap as an empty result -- fails
     * `rrule/evaluator/unsatisfiable_feb_30`, which asserts precisely
     * that an unsatisfiable rule produces the cap and not an empty
     * answer.
     *
     * The first occurrence of a rule is `$dtstart` itself whenever it
     * satisfies the BY-* filters, per RFC 5545 -- pass
     * `$after = $dtstart` to step past it.
     */
    public static function nextOccurrence(
        Rule $rule,
        \DateTimeImmutable $dtstart,
        \DateTimeImmutable $after,
    ): ?\DateTimeImmutable {
        Evaluator::checkExpandable($rule);

        $found = null;
        $capped = Evaluator::walk($rule, $dtstart, static function (\DateTimeImmutable $occ) use ($after, &$found): bool {
            if ($occ > $after) {
                $found = $occ;

                return false;
            }

            return true;
        });

        if ($found !== null) {
            return $found;
        }

        if ($capped) {
            throw Evaluator::iterationCap('nextOccurrence', $rule);
        }

        return null;
    }

    /**
     * Every occurrence of `$rule` from `$dtstart`, lazily.
     *
     * A rule with neither UNTIL nor COUNT is infinite, and iterating
     * this without breaking will not return -- that is the documented
     * behaviour of a lazy sequence and the reason it exists. It is the
     * third of the spec's three bounding strategies (count, window,
     * laziness), and the only one that works on an unbounded rule
     * without the caller choosing a bound up front. Use
     * {@see self::occurrences()} or {@see self::between()} when a
     * bounded result is what you want.
     *
     * The sequence also ends at the iteration bound. A generator has
     * nowhere to put an error, so `all` cannot tell that apart from
     * termination; the bounded entry points report it as
     * `ErrIterationCap`.
     *
     * @return \Generator<int, \DateTimeImmutable, mixed, void>
     */
    public static function all(Rule $rule, \DateTimeImmutable $dtstart): \Generator
    {
        // The walk is push-shaped and a generator is pull-shaped, so the
        // period loop is re-stated here rather than re-entering the walk
        // per value.
        try {
            Evaluator::checkExpandable($rule);
        } catch (\HopTop\Vstar\Exception\UnsupportedRRuleException) {
            return;
        }

        $emitted = 0;
        $empty = 0;
        $current = $dtstart;

        while ($empty < Evaluator::MAX_ITERATIONS) {
            $produced = false;

            foreach (Evaluator::periodOccurrences($rule, $current, $dtstart) as $occ) {
                if ($occ < $dtstart) {
                    continue;
                }

                if ($rule->until !== null && $occ > $rule->until) {
                    return;
                }

                $produced = true;
                ++$emitted;

                yield $occ;

                if ($rule->count > 0 && $emitted >= $rule->count) {
                    return;
                }
            }

            $empty = $produced ? 0 : $empty + 1;
            $next = Evaluator::advance($rule, $current);

            if ($next === null) {
                return;
            }

            $current = $next;
        }
    }

    /**
     * Up to `$limit` occurrences, with the flag that says which way the
     * expansion stopped.
     *
     * `complete` is true when the rule itself terminated within the
     * limit -- the returned list is the entire series -- and false when
     * the limit truncated it. Reporting that distinction is required by
     * spec §Expansion: a caller that stops after N steps otherwise never
     * learns whether N was the whole series or merely the first N. The
     * result is a keyed record rather than a positional pair so the flag
     * cannot be silently ignored.
     *
     * A limit of `0` returns no occurrences and `complete: false` -- no
     * occurrences, and no claim that the series ended. A negative limit
     * is `ErrUnboundedExpansion`.
     *
     * @return array{occurrences: list<\DateTimeImmutable>, complete: bool}
     */
    public static function occurrences(Rule $rule, \DateTimeImmutable $dtstart, int $limit): array
    {
        if ($limit < 0) {
            throw new UnboundedExpansionException(
                'rrule: occurrences: limit must be >= 0, got ' . $limit,
            );
        }

        Evaluator::checkExpandable($rule);

        if ($limit === 0) {
            return ['occurrences' => [], 'complete' => false];
        }

        $out = [];
        $truncated = false;

        $capped = Evaluator::walk($rule, $dtstart, static function (\DateTimeImmutable $occ) use ($limit, &$out, &$truncated): bool {
            if (count($out) === $limit) {
                // The walk produced one more than asked for, so the
                // series definitively continues past the limit.
                $truncated = true;

                return false;
            }

            $out[] = $occ;

            return true;
        });

        if ($capped) {
            throw Evaluator::iterationCap('occurrences', $rule);
        }

        return ['occurrences' => $out, 'complete' => !$truncated];
    }

    /**
     * Every occurrence in the half-open window `[$start, $end)` --
     * `$start` inclusive, `$end` exclusive.
     *
     * The window bounds the result, so this terminates even for a rule
     * with neither UNTIL nor COUNT. It does not bound the *search*: a
     * rule that never yields never reaches `$end`, so the iteration
     * bound stops it and the failure is `ErrIterationCap`.
     *
     * A missing `$end`, or an `$end` not strictly after `$start`, is
     * `ErrUnboundedExpansion` rather than a silent empty result -- an
     * unbounded window is an infinite expansion request.
     *
     * @return list<\DateTimeImmutable>
     */
    public static function between(
        Rule $rule,
        \DateTimeImmutable $dtstart,
        \DateTimeImmutable $start,
        ?\DateTimeImmutable $end,
    ): array {
        if ($end === null) {
            throw new UnboundedExpansionException(
                'rrule: between: end must be present (an open-ended window is an infinite expansion; use all)',
            );
        }

        if ($end <= $start) {
            throw new UnboundedExpansionException(sprintf(
                'rrule: between: end %s must be after start %s',
                Time::formatTime($end),
                Time::formatTime($start),
            ));
        }

        Evaluator::checkExpandable($rule);

        $out = [];
        $capped = Evaluator::walk($rule, $dtstart, static function (\DateTimeImmutable $occ) use ($start, $end, &$out): bool {
            if ($occ >= $end) {
                return false;
            }

            if ($occ >= $start) {
                $out[] = $occ;
            }

            return true;
        });

        if ($capped) {
            throw Evaluator::iterationCap('between', $rule);
        }

        return $out;
    }

    /**
     * Parse a comma-separated list of RFC 5545 form #2 (UTC) datetimes
     * -- the value form of a DATE-TIME-valued EXDATE or RDATE.
     *
     * The result is sorted and de-duplicated, so callers get a canonical
     * set whatever order the producer wrote.
     *
     * @return list<\DateTimeImmutable>
     */
    public static function parseDateTimeList(string $s): array
    {
        if ($s === '') {
            throw new MalformedException('rrule: empty date-time list');
        }

        $out = [];

        foreach (explode(',', $s) as $raw) {
            $t = Time::parseTime($raw);

            if ($t === null) {
                throw new MalformedException(
                    'rrule: "' . $raw . '" is not an RFC 5545 form #2 date-time',
                );
            }

            $out[] = $t;
        }

        return self::sortDedupe($out);
    }

    /**
     * Render instants as an EXDATE/RDATE property value: comma-separated
     * UTC form #2, sorted and de-duplicated so identical logical content
     * yields identical bytes. An empty input renders as the empty
     * string.
     *
     * @param list<\DateTimeImmutable> $times
     */
    public static function formatDateTimeList(array $times): string
    {
        return implode(',', array_map(
            static fn (\DateTimeImmutable $t): string => Time::formatTime($t),
            self::sortDedupe($times),
        ));
    }

    /**
     * Sort ascending and drop duplicate instants, returning a new list.
     *
     * Equality is by instant, not by object identity: two
     * `DateTimeImmutable` values naming the same second are one
     * occurrence.
     *
     * @param list<\DateTimeImmutable> $times
     *
     * @return list<\DateTimeImmutable>
     */
    public static function sortDedupe(array $times): array
    {
        usort($times, static fn (\DateTimeImmutable $a, \DateTimeImmutable $b): int => $a <=> $b);

        $out = [];

        foreach ($times as $t) {
            if ($out !== [] && $out[count($out) - 1] == $t) {
                continue;
            }

            $out[] = $t;
        }

        return $out;
    }
}
