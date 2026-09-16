<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Rrule;

use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\UnsupportedRRuleException;
use HopTop\Vstar\Time;

/**
 * RFC 5545 §3.3.10 RRULE value parsing, for the scope
 * `spec/v1.0/03-canonicalization.md` §RRULE parsing scope fixes.
 *
 * Two failure classes, and the `rrule/rejected/` fixtures distinguish
 * them:
 *
 * - `ErrMalformed` -- syntactically wrong: an unknown rule-part, a
 *   missing FREQ, a zero ordinal, INTERVAL below 1, UNTIL and COUNT
 *   together, UNTIL outside form #2, a value out of its RFC range.
 * - `ErrUnsupportedRRule` -- syntactically fine but deferred by this
 *   scope: FREQ=SECONDLY, RSCALE.
 *
 * Getting the two the wrong way round passes "it failed" and fails the
 * corpus, which asserts *which* failure.
 *
 * Internal to the namespace: {@see Rrule::parse()} is the public entry
 * point the API mapping names.
 */
final class Parser
{
    /** BYSETPOS indexes a period's expanded set, which a year bounds. */
    private const MAX_SET_POS = 366;

    /**
     * Decompose an RRULE property value -- no `RRULE:` prefix -- into a
     * {@see Rule}, applying the RFC defaults and enforcing every
     * cross-field invariant.
     *
     * Keys and values are matched case-sensitively in their RFC wire
     * spelling, matching the strict posture {@see Time::parseTime()}
     * takes. The order of rule-parts is irrelevant.
     */
    public static function parse(string $s): Rule
    {
        if ($s === '') {
            self::malformed('empty input');
        }

        $d = self::newDraft();
        $seen = [];

        foreach (explode(';', $s) as $part) {
            $eq = strpos($part, '=');

            // An empty key (`=DAILY`) is as malformed as a missing
            // separator.
            if ($eq === false || $eq === 0) {
                self::malformed('malformed rule-part "' . $part . '"');
            }

            $key = substr($part, 0, $eq);
            $value = substr($part, $eq + 1);

            if (isset($seen[$key])) {
                self::malformed('duplicate rule-part "' . $key . '"');
            }

            $seen[$key] = true;
            self::applyRulePart($d, $key, $value);
        }

        self::validateAfterParse($d);

        return self::toRule($d);
    }

    /**
     * A fresh draft carrying the RFC defaults: `INTERVAL=1`, `WKST=MO`.
     *
     * @return array<string, mixed>
     */
    private static function newDraft(): array
    {
        return [
            'freq' => Freq::Invalid,
            'interval' => 1,
            'until' => null,
            'count' => 0,
            'byDay' => [],
            'byMonth' => [],
            'byMonthDay' => [],
            'byHour' => [],
            'byMinute' => [],
            'bySecond' => [],
            'byYearDay' => [],
            'byWeekNo' => [],
            'bySetPos' => [],
            'weekStart' => Weekday::Mo,
        ];
    }

    /**
     * Freeze a validated draft into the immutable rule.
     *
     * @param array<string, mixed> $d
     */
    private static function toRule(array $d): Rule
    {
        /** @var Freq $freq */
        $freq = $d['freq'];
        /** @var int $interval */
        $interval = $d['interval'];
        /** @var \DateTimeImmutable|null $until */
        $until = $d['until'];
        /** @var int $count */
        $count = $d['count'];
        /** @var list<ByDay> $byDay */
        $byDay = $d['byDay'];
        /** @var list<int> $byMonth */
        $byMonth = $d['byMonth'];
        /** @var list<int> $byMonthDay */
        $byMonthDay = $d['byMonthDay'];
        /** @var list<int> $byHour */
        $byHour = $d['byHour'];
        /** @var list<int> $byMinute */
        $byMinute = $d['byMinute'];
        /** @var list<int> $bySecond */
        $bySecond = $d['bySecond'];
        /** @var list<int> $byYearDay */
        $byYearDay = $d['byYearDay'];
        /** @var list<int> $byWeekNo */
        $byWeekNo = $d['byWeekNo'];
        /** @var list<int> $bySetPos */
        $bySetPos = $d['bySetPos'];
        /** @var Weekday $weekStart */
        $weekStart = $d['weekStart'];

        return new Rule(
            $freq,
            $interval,
            $until,
            $count,
            $byDay,
            $byMonth,
            $byMonthDay,
            $byHour,
            $byMinute,
            $bySecond,
            $byYearDay,
            $byWeekNo,
            $bySetPos,
            $weekStart,
        );
    }

    /**
     * Dispatch one `KEY=VALUE` pair to its field handler.
     *
     * @param array<string, mixed> $d
     */
    private static function applyRulePart(array &$d, string $key, string $value): void
    {
        switch ($key) {
            case 'FREQ':
                $d['freq'] = self::parseFreq($value);

                return;

            case 'INTERVAL':
                $d['interval'] = self::parseBoundedInt('INTERVAL', $value, 1, PHP_INT_MAX);

                return;

            case 'UNTIL':
                $t = Time::parseTime($value);

                if ($t === null) {
                    self::malformed(
                        'UNTIL must be RFC 5545 form #2 (UTC, Z-suffixed), got "' . $value . '"',
                    );
                }

                $d['until'] = $t;

                return;

            case 'COUNT':
                $d['count'] = self::parseBoundedInt('COUNT', $value, 1, PHP_INT_MAX);

                return;

            case 'BYDAY':
                $d['byDay'] = self::parseByDayList($value);

                return;

            case 'BYMONTH':
                $d['byMonth'] = self::parseIntList('BYMONTH', $value, 1, 12);

                return;

            case 'BYMONTHDAY':
                $d['byMonthDay'] = self::parseSignedIntList('BYMONTHDAY', $value, 1, 31);

                return;

            case 'BYHOUR':
                $d['byHour'] = self::parseIntList('BYHOUR', $value, 0, 23);

                return;

            case 'BYMINUTE':
                $d['byMinute'] = self::parseIntList('BYMINUTE', $value, 0, 59);

                return;

            case 'BYSECOND':
                // 60 is retained for leap seconds per RFC 5545 §3.3.10.
                $d['bySecond'] = self::parseIntList('BYSECOND', $value, 0, 60);

                return;

            case 'BYYEARDAY':
                $d['byYearDay'] = self::parseSignedIntList('BYYEARDAY', $value, 1, 366);

                return;

            case 'BYWEEKNO':
                $d['byWeekNo'] = self::parseSignedIntList('BYWEEKNO', $value, 1, 53);

                return;

            case 'BYSETPOS':
                $d['bySetPos'] = self::parseSignedIntList('BYSETPOS', $value, 1, self::MAX_SET_POS);

                return;

            case 'WKST':
                $d['weekStart'] = self::parseWeekday($value);

                return;

            case 'RSCALE':
                // RFC 7529, non-Gregorian calendars -- deferred
                // indefinitely; not on the V* roadmap.
                self::unsupported('rule-part ' . $key . ': outside the RRULE parsing scope');

                // no break -- unsupported() never returns.
            default:
                self::malformed('unknown rule-part "' . $key . '"');
        }
    }

    /**
     * The `FREQ` token, rejecting the one deferred frequency by name.
     *
     * The distinction matters: an unrecognized token is malformed, while
     * `SECONDLY` is a valid RFC value this scope defers, and the corpus
     * asserts which answer each gets.
     */
    private static function parseFreq(string $v): Freq
    {
        switch ($v) {
            case 'MINUTELY':
            case 'HOURLY':
            case 'DAILY':
            case 'WEEKLY':
            case 'MONTHLY':
            case 'YEARLY':
                // The case list above has already proved the token is one
                // of the six, so the total `from` is honest here where a
                // `tryFrom` would imply a null this branch cannot reach.
                return Freq::from($v);

            case 'SECONDLY':
                // Syntactically valid, deliberately deferred: extreme
                // expansion, with no realistic agentic use case.
                self::unsupported('FREQ=' . $v . ': outside the RRULE parsing scope');

                // no break -- unsupported() never returns.
            default:
                self::malformed('invalid FREQ value "' . $v . '"');
        }
    }

    /**
     * A base-10 integer, strictly spelled.
     *
     * A cast is deliberately not used on its own: `(int)` accepts
     * `"0x10"`, `"1e3"`, `" 5 "` and `""`, each of which would turn a
     * malformed rule-part into a plausible value.
     */
    private static function strictInt(string $name, string $raw): int
    {
        if (preg_match('/\A[+-]?\d+\z/', $raw) !== 1) {
            self::malformed($name . ' non-integer "' . $raw . '"');
        }

        return (int) $raw;
    }

    /**
     * A single integer constrained to `[$lo, $hi]`.
     */
    private static function parseBoundedInt(string $name, string $v, int $lo, int $hi): int
    {
        $n = self::strictInt($name, $v);

        if ($n < $lo || $n > $hi) {
            self::malformed($name . ' must be >= ' . $lo . ', got ' . $n);
        }

        return $n;
    }

    /**
     * A comma-separated list of integers, each in `[$lo, $hi]`.
     *
     * The authored order is kept: RFC 5545 gives BY-* lists no ordering
     * semantics, so sorting one here would silently rewrite a producer's
     * value on every round trip.
     *
     * @return list<int>
     */
    private static function parseIntList(string $name, string $v, int $lo, int $hi): array
    {
        if ($v === '') {
            self::malformed($name . ' empty');
        }

        $out = [];

        foreach (explode(',', $v) as $raw) {
            $n = self::strictInt($name, $raw);

            if ($n < $lo || $n > $hi) {
                self::malformed($name . ' ' . $n . ' out of range ' . $lo . '..' . $hi);
            }

            $out[] = $n;
        }

        return $out;
    }

    /**
     * A comma-separated list of signed integers where each `n` satisfies
     * `$lo <= |n| <= $hi` and `n !== 0`, in authored order.
     *
     * The two-sided range is RFC 5545 §3.3.10's "from the start
     * (positive) or from the end (negative)" pattern, shared by
     * BYMONTHDAY, BYYEARDAY, BYWEEKNO and BYSETPOS. Zero is rejected for
     * all four: it would name neither end.
     *
     * @return list<int>
     */
    private static function parseSignedIntList(string $name, string $v, int $lo, int $hi): array
    {
        if ($v === '') {
            self::malformed($name . ' empty');
        }

        $out = [];

        foreach (explode(',', $v) as $raw) {
            $n = self::strictInt($name, $raw);

            if ($n === 0) {
                self::malformed($name . ' 0 invalid (RFC 5545 §3.3.10)');
            }

            $abs = abs($n);

            if ($abs < $lo || $abs > $hi) {
                self::malformed(
                    $name . ' ' . $n . ' out of range -' . $hi . '..-' . $lo . ' or ' . $lo . '..' . $hi,
                );
            }

            $out[] = $n;
        }

        return $out;
    }

    /**
     * A `BYDAY` list: `[<ordinal>]<weekday>` entries, authored order
     * kept for the same reason the integer lists keep theirs.
     *
     * @return list<ByDay>
     */
    private static function parseByDayList(string $v): array
    {
        if ($v === '') {
            self::malformed('BYDAY empty');
        }

        $out = [];

        foreach (explode(',', $v) as $entry) {
            $out[] = self::parseByDayEntry($entry);
        }

        return $out;
    }

    /**
     * One `BYDAY` entry. The weekday is the final two characters.
     */
    private static function parseByDayEntry(string $s): ByDay
    {
        if (strlen($s) < 2) {
            self::malformed('BYDAY entry "' . $s . '" too short');
        }

        $weekday = self::parseWeekday(substr($s, -2));
        $prefix = substr($s, 0, -2);

        if ($prefix === '') {
            return new ByDay(0, $weekday);
        }

        $n = self::strictInt('BYDAY ordinal', $prefix);

        // The explicit "0" prefix is invalid per RFC 5545 §3.3.10 -- it
        // is spelled by omitting the ordinal, not by writing zero.
        if ($n === 0) {
            self::malformed('BYDAY ordinal 0 invalid (RFC 5545 §3.3.10)');
        }

        if ($n < -53 || $n > 53) {
            self::malformed('BYDAY ordinal ' . $n . ' out of range -53..53');
        }

        return new ByDay($n, $weekday);
    }

    /**
     * A two-letter weekday symbol.
     */
    private static function parseWeekday(string $s): Weekday
    {
        $w = Weekday::tryFrom($s);

        if ($w === null) {
            self::malformed('invalid weekday "' . $s . '"');
        }

        return $w;
    }

    /**
     * The cross-field invariants, run once every rule-part is consumed.
     *
     * Rule-part order is irrelevant per RFC, so none of these can be
     * checked while parsing: `COUNT` may precede `UNTIL` or follow it,
     * and `BYYEARDAY` may appear before the `FREQ` it constrains.
     *
     * @param array<string, mixed> $d
     */
    private static function validateAfterParse(array $d): void
    {
        if ($d['freq'] === Freq::Invalid) {
            self::malformed('FREQ is required');
        }

        if ($d['until'] !== null && is_int($d['count']) && $d['count'] > 0) {
            self::malformed('UNTIL and COUNT are mutually exclusive');
        }

        if (is_array($d['byYearDay']) && $d['byYearDay'] !== [] && $d['freq'] !== Freq::Yearly) {
            self::malformed('BYYEARDAY requires FREQ=YEARLY (RFC 5545 §3.3.10)');
        }

        if (is_array($d['byWeekNo']) && $d['byWeekNo'] !== [] && $d['freq'] !== Freq::Yearly) {
            self::malformed('BYWEEKNO requires FREQ=YEARLY (RFC 5545 §3.3.10)');
        }

        if (is_array($d['bySetPos']) && $d['bySetPos'] !== [] && !self::hasOtherBy($d)) {
            self::malformed('BYSETPOS requires at least one other BY-* rule-part (RFC 5545 §3.3.10)');
        }
    }

    /**
     * Whether any BY-* clause other than BYSETPOS is present -- the
     * precondition RFC 5545 §3.3.10 puts on BYSETPOS ("MUST only be used
     * in conjunction with another BYxxx rule part").
     *
     * @param array<string, mixed> $d
     */
    private static function hasOtherBy(array $d): bool
    {
        foreach (['byDay', 'byMonth', 'byMonthDay', 'byHour', 'byMinute', 'bySecond', 'byYearDay', 'byWeekNo'] as $k) {
            if (is_array($d[$k]) && $d[$k] !== []) {
                return true;
            }
        }

        return false;
    }

    /**
     * Raise `ErrMalformed` with the given explanation.
     *
     * @return never
     */
    private static function malformed(string $message): void
    {
        throw new MalformedException('rrule: ' . $message);
    }

    /**
     * Raise `ErrUnsupportedRRule` with the given explanation.
     *
     * @return never
     */
    private static function unsupported(string $message): void
    {
        throw new UnsupportedRRuleException('rrule: ' . $message);
    }
}
