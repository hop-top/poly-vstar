<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Duration;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Component;
use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\NoTriggerException;
use HopTop\Vstar\Time;

/**
 * The RFC 5545 §3.3.6 DURATION value type's free functions, and the
 * VALARM helpers that read DURATION-shaped properties off a component.
 *
 * PHP has no free functions at namespace scope in the idiomatic style, so
 * the reference package's package-level functions land here as static
 * methods on a final class named for the package. The value type itself
 * is {@see VDuration}.
 */
final class Duration
{
    private const PROP_TRIGGER = 'TRIGGER';

    private const PROP_DURATION = 'DURATION';

    private const PROP_REPEAT = 'REPEAT';

    /**
     * Decode an RFC 5545 §3.3.6 DURATION value -- no property name, no
     * parameters, e.g. `-PT15M`.
     *
     * The grammar accepted is exactly:
     *
     * ```
     * dur-value  = ["+" / "-"] "P" (dur-date / dur-time / dur-week)
     * dur-date   = dur-day [dur-time]
     * dur-time   = "T" (dur-hour / dur-minute / dur-second)
     * dur-week   = 1*DIGIT "W"
     * dur-hour   = 1*DIGIT "H" [dur-minute]
     * dur-minute = 1*DIGIT "M" [dur-second]
     * dur-second = 1*DIGIT "S"
     * dur-day    = 1*DIGIT "D"
     * ```
     *
     * Parsing is strict, matching {@see Time::parseTime()}'s posture.
     * Throws `ErrMalformed` on: empty input, a missing `P`, lowercase
     * designators, weeks mixed with any other unit, time units outside the
     * `T` part, units out of RFC order, repeated units, digits with no
     * unit, an empty `T` part, ISO 8601 years or months (`P1Y`, `P1M` --
     * not in RFC 5545), fractional values, per-component signs, and any
     * whitespace.
     */
    public static function parse(string $s): VDuration
    {
        if ($s === '') {
            throw self::malformed('empty input');
        }

        $rest = $s;
        $negative = false;

        if ($rest[0] === '+') {
            $rest = substr($rest, 1);
        } elseif ($rest[0] === '-') {
            $negative = true;
            $rest = substr($rest, 1);
        }

        if ($rest === '' || $rest[0] !== 'P') {
            throw self::malformed(self::quote($s) . ' is missing its "P" designator');
        }

        $rest = substr($rest, 1);

        if ($rest === '') {
            throw self::malformed(self::quote($s) . ' has no value after "P"');
        }

        // A bare time part: "PT...".
        if ($rest[0] === 'T') {
            $time = self::parseTimePart(substr($rest, 1), $s);

            return new VDuration(
                negative: $negative,
                hours: $time['hours'],
                minutes: $time['minutes'],
                seconds: $time['seconds'],
            );
        }

        $cut = strpos($rest, 'T');
        $hasTime = $cut !== false;
        $datePart = $hasTime ? substr($rest, 0, $cut) : $rest;
        $timePart = $hasTime ? substr($rest, $cut + 1) : '';

        $first = self::nextField($datePart, $s);
        $weeks = 0;
        $days = 0;
        $dayForm = false;

        switch ($first['unit']) {
            case 'W':
                if ($first['rest'] !== '') {
                    throw self::malformed(self::quote($s) . ' mixes weeks with other units');
                }

                if ($hasTime) {
                    throw self::malformed(self::quote($s) . ' mixes weeks with a time part');
                }

                $weeks = $first['n'];

                break;

            case 'D':
                if ($first['rest'] !== '') {
                    throw self::malformed(
                        self::quote($s) . ' has trailing input ' . self::quote($first['rest'])
                        . ' after the day value',
                    );
                }

                $days = $first['n'];
                $dayForm = true;

                break;

            default:
                throw self::malformed(
                    self::quote($s) . ' uses unit ' . self::quote($first['unit'])
                    . ' outside a time part (RFC 5545 has no years or months)',
                );
        }

        $time = $hasTime
            ? self::parseTimePart($timePart, $s)
            : ['hours' => 0, 'minutes' => 0, 'seconds' => 0];

        return new VDuration(
            negative: $negative,
            weeks: $weeks,
            days: $days,
            hours: $time['hours'],
            minutes: $time['minutes'],
            seconds: $time['seconds'],
            dayForm: $dayForm,
        );
    }

    /**
     * Whether `$s` is a well-formed RFC 5545 §3.3.6 DURATION value.
     * Equivalent to discarding {@see self::parse()}'s result, offered so a
     * caller testing a wire string need not catch.
     */
    public static function valid(string $s): bool
    {
        try {
            self::parse($s);

            return true;
        } catch (MalformedException) {
            return false;
        }
    }

    /**
     * Convert a signed `DateInterval` into a {@see VDuration} expressed in
     * hours, minutes and seconds. Sub-second precision is truncated:
     * RFC 5545 durations have second resolution.
     *
     * The result never uses the week or day units -- an elapsed interval
     * carries no calendar information, so emitting `P1D` from 24 hours
     * would invent a distinction the input never made. A caller meaning
     * calendar days constructs the {@see VDuration} directly.
     *
     * The interval's calendar fields are read as their nominal lengths
     * (a year as 365 days, a month as 30) only when the interval was built
     * from a spec string; an interval produced by `DateTimeImmutable::diff`
     * carries its exact day count in `days`, which is preferred when set.
     */
    public static function fromSigned(\DateInterval $td): VDuration
    {
        $seconds = self::intervalSeconds($td);
        $negative = $seconds < 0;
        $rest = abs($seconds);

        $hours = intdiv($rest, 3600);
        $rest -= $hours * 3600;
        $minutes = intdiv($rest, 60);
        $rest -= $minutes * 60;

        return new VDuration(
            negative: $negative,
            hours: $hours,
            minutes: $minutes,
            seconds: $rest,
        );
    }

    /**
     * Read and parse the TRIGGER property of a VALARM.
     *
     * Throws `ErrNoTrigger` when the property is absent: RFC 5545 §3.6.6
     * makes TRIGGER mandatory on VALARM, so this is a producer bug rather
     * than an absent optional.
     */
    public static function alarmTrigger(Component $alarm): Trigger
    {
        $p = $alarm->get(self::PROP_TRIGGER);

        if ($p === null) {
            throw new NoTriggerException('duration: VALARM has no TRIGGER property');
        }

        return Trigger::parse($p);
    }

    /**
     * The end instant of a component that expresses it either as DTEND or
     * as DTSTART plus a DURATION, per RFC 5545 §3.6.1 (which allows
     * exactly one of the two on a VEVENT).
     *
     * DTEND wins when both are present -- it is the explicit statement.
     *
     * Null when neither form is available, when DTSTART is missing for the
     * DURATION form, or when the DURATION value is malformed.
     */
    public static function eventEnd(Component $c, Calendar $cal): ?\DateTimeImmutable
    {
        $end = Time::dtend($c, $cal);

        if ($end !== null) {
            return $end;
        }

        $p = $c->get(self::PROP_DURATION);

        if ($p === null || !self::valid($p->value)) {
            return null;
        }

        $start = Time::dtstart($c, $cal);

        if ($start === null) {
            return null;
        }

        return self::parse($p->value)->addTo($start);
    }

    /**
     * The VALARM DURATION/REPEAT pair from RFC 5545 §3.8.6.2 and §3.8.6.3:
     * the interval between repetitions and how many additional times the
     * alarm repeats after its initial trigger.
     *
     * The two properties travel together -- the RFC requires that if one
     * is present the other must be. Returns a zero-length duration and
     * zero when neither is present; throws `ErrMalformed` when only one
     * is, when the DURATION value is invalid, or when REPEAT is not a
     * non-negative integer.
     *
     * @return array{0: VDuration, 1: int}
     */
    public static function alarmRepeatCycle(Component $alarm): array
    {
        $durProp = $alarm->get(self::PROP_DURATION);
        $repProp = $alarm->get(self::PROP_REPEAT);

        if ($durProp === null && $repProp === null) {
            return [new VDuration(), 0];
        }

        if ($durProp === null) {
            throw self::malformed('VALARM has REPEAT without DURATION (RFC 5545 §3.8.6.2)');
        }

        if ($repProp === null) {
            throw self::malformed('VALARM has DURATION without REPEAT (RFC 5545 §3.8.6.2)');
        }

        $d = self::parse($durProp->value);

        if (preg_match('/\A\d+\z/', $repProp->value) !== 1) {
            throw self::malformed(
                'VALARM REPEAT ' . self::quote($repProp->value) . ' is not a non-negative integer',
            );
        }

        return [$d, (int) $repProp->value];
    }

    /**
     * The elapsed second count of a `DateInterval`, honouring its
     * `invert` flag.
     *
     * `days` is preferred when the interval came from a `diff` (where it
     * holds the exact elapsed day count); otherwise the calendar fields
     * are read at their nominal lengths, which is all an interval built
     * from a spec string can offer.
     */
    private static function intervalSeconds(\DateInterval $i): int
    {
        $days = $i->days !== false ? $i->days : ($i->y * 365 + $i->m * 30 + $i->d);

        $total = $days * 86400 + $i->h * 3600 + $i->i * 60 + $i->s;

        return $i->invert === 1 ? -$total : $total;
    }

    /**
     * Decode the segment after `T` into the hour, minute and second
     * fields. Units appear at most once and in RFC order (H, then M, then
     * S).
     *
     * @return array{hours: int, minutes: int, seconds: int}
     */
    private static function parseTimePart(string $s, string $orig): array
    {
        if ($s === '') {
            throw self::malformed(self::quote($orig) . ' has an empty time part');
        }

        $hours = 0;
        $minutes = 0;
        $seconds = 0;

        // `order` tracks how far through H->M->S we have advanced, so a
        // repeated or out-of-order unit is rejected rather than silently
        // overwriting.
        $order = 0;
        $rest = $s;

        while ($rest !== '') {
            $field = self::nextField($rest, $orig);

            switch ($field['unit']) {
                case 'H':
                    $rank = 1;
                    $hours = $field['n'];

                    break;

                case 'M':
                    $rank = 2;
                    $minutes = $field['n'];

                    break;

                case 'S':
                    $rank = 3;
                    $seconds = $field['n'];

                    break;

                default:
                    throw self::malformed(
                        self::quote($orig) . ' uses unknown time unit ' . self::quote($field['unit']),
                    );
            }

            if ($rank <= $order) {
                throw self::malformed(
                    self::quote($orig) . ' repeats or misorders time unit '
                    . self::quote($field['unit']),
                );
            }

            $order = $rank;
            $rest = $field['rest'];
        }

        return ['hours' => $hours, 'minutes' => $minutes, 'seconds' => $seconds];
    }

    /**
     * Consume one `1*DIGIT UNIT` field off the front of `$s`.
     *
     * @return array{n: int, unit: string, rest: string}
     */
    private static function nextField(string $s, string $orig): array
    {
        $i = 0;
        $len = strlen($s);

        while ($i < $len && $s[$i] >= '0' && $s[$i] <= '9') {
            ++$i;
        }

        if ($i === 0) {
            throw self::malformed(self::quote($orig) . ' has a unit with no digits');
        }

        if ($i === $len) {
            throw self::malformed(self::quote($orig) . ' has digits with no unit');
        }

        return [
            'n' => (int) substr($s, 0, $i),
            'unit' => $s[$i],
            'rest' => substr($s, $i + 1),
        ];
    }

    /**
     * A malformed-duration failure, prefixed so the message names the
     * layer that rejected the value.
     */
    private static function malformed(string $message): MalformedException
    {
        return new MalformedException('duration: ' . $message);
    }

    /**
     * A value rendered for an error message, quoted so an empty or
     * whitespace-only input is visible in the text.
     */
    private static function quote(string $s): string
    {
        return '"' . $s . '"';
    }
}
