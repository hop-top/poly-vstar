<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Duration;

/**
 * An RFC 5545 §3.3.6 DURATION value in the units its producer authored.
 *
 * # Why a class and not a second count
 *
 * This type preserves the units the producer authored -- weeks, days,
 * hours, minutes, seconds -- rather than collapsing them. A plain second
 * count cannot represent "one day" distinctly from "24 hours", but
 * RFC 5545 draws that distinction deliberately: a calendar day is 23, 24
 * or 25 hours across a UTC-offset transition. Re-serializing a second
 * count would silently rewrite `P1D` as `PT24H` -- and canonical form
 * preserves DURATION values verbatim (spec rule 12), so that rewrite
 * changes the hash.
 *
 * Both views are available: {@see self::signed()} reports the nominal
 * length as a `DateInterval` (days as 24h, weeks as 7 days), and
 * {@see self::addTo()} anchors the value against a real instant.
 *
 * The grammar admits either a week form (weeks alone) or a day-and-time
 * form (days plus an optional hour/minute/second part); the two never
 * mix. The sign applies to the WHOLE duration, not to any single field --
 * that is what makes `-PT15M` mean "15 minutes before" on a VALARM
 * TRIGGER.
 *
 * A duration with no non-zero unit is a valid, positive, zero-length
 * value that renders as `PT0S`.
 *
 * Named `VDuration` rather than `Duration` because PHP's near-universal
 * `DateInterval` occupies the same conceptual slot and the V* type is not
 * interchangeable with it; the class of static methods named for the
 * reference package is {@see Duration}.
 */
final class VDuration
{
    private const SECONDS_PER_MINUTE = 60;

    private const SECONDS_PER_HOUR = 3600;

    private const SECONDS_PER_DAY = 86400;

    private const DAYS_PER_WEEK = 7;

    /**
     * @param bool $negative whether the whole duration is subtractive
     * @param int  $weeks    the `nW` count; when non-zero every other unit field is zero
     * @param int  $days     the `nD` count
     * @param int  $hours    the `nH` count from the time part
     * @param int  $minutes  the `nM` count from the time part
     * @param int  $seconds  the `nS` count from the time part
     * @param bool $dayForm  whether the value was authored in the day form
     */
    public function __construct(
        public readonly bool $negative = false,
        public readonly int $weeks = 0,
        public readonly int $days = 0,
        public readonly int $hours = 0,
        public readonly int $minutes = 0,
        public readonly int $seconds = 0,
        public readonly bool $dayForm = false,
    ) {
    }

    /**
     * Render as an RFC 5545 §3.3.6 DURATION value, preserving the units
     * the value carries.
     *
     * {@see Duration::parse()} and this round-trip byte for byte, with one
     * intentional normalization: an explicit `+` is dropped, since a
     * positive duration is the default.
     *
     * The {@see self::$dayForm} flag is what lets `P0D` render as itself
     * rather than as the canonical zero spelling `PT0S`: both are
     * numerically zero and every unit field is zero in both, so without
     * the flag the authored spelling could not be reproduced -- and rule
     * 12 preserves DURATION values verbatim.
     */
    public function __toString(): string
    {
        $out = ($this->negative && !$this->isZeroLength()) ? '-P' : 'P';

        if ($this->weeks !== 0) {
            return $out . $this->weeks . 'W';
        }

        // A wholly zero duration has no unit to render, so it takes the
        // canonical zero spelling rather than a bare `P`, which parse
        // rejects.
        if ($this->isZeroLength() && !$this->dayForm) {
            return $out . 'T0S';
        }

        if ($this->days !== 0 || $this->dayForm) {
            $out .= $this->days . 'D';
        }

        $hasTime = $this->hours !== 0 || $this->minutes !== 0 || $this->seconds !== 0;

        if (!$hasTime) {
            return $out;
        }

        $out .= 'T';

        if ($this->hours !== 0) {
            $out .= $this->hours . 'H';
        }

        if ($this->minutes !== 0) {
            $out .= $this->minutes . 'M';
        }

        if ($this->seconds !== 0) {
            $out .= $this->seconds . 'S';
        }

        return $out;
    }

    /**
     * The nominal length as a `DateInterval`, with `invert` set when the
     * duration is negative. Days count as 24 hours and weeks as 7 days.
     *
     * Exact for time-only values and for any anchor in a fixed-offset
     * zone, UTC included. Use {@see self::addTo()} when a real anchor is
     * available and a day value might cross a transition.
     *
     * The interval is built from the total second count rather than from
     * the authored fields, so it never carries a calendar `d`/`m`/`y`
     * component that `DateTimeImmutable::add()` would interpret by date
     * rather than by elapsed time.
     */
    public function signed(): \DateInterval
    {
        $total = abs($this->totalSeconds());

        $interval = new \DateInterval(sprintf('PT%dS', $total));

        if ($this->isNegative()) {
            $interval->invert = 1;
        }

        return $interval;
    }

    /**
     * The nominal length in seconds, negated when the duration is
     * negative. Days count as 24 hours and weeks as 7 days.
     *
     * The second count is what `spec/behavior/duration/parse.json` pins,
     * and what {@see self::signed()} is built from.
     */
    public function totalSeconds(): int
    {
        $total = $this->weeks * self::DAYS_PER_WEEK * self::SECONDS_PER_DAY
            + $this->days * self::SECONDS_PER_DAY
            + $this->hours * self::SECONDS_PER_HOUR
            + $this->minutes * self::SECONDS_PER_MINUTE
            + $this->seconds;

        return $this->negative ? -$total : $total;
    }

    /**
     * Whether the duration is subtractive. A zero-length duration is never
     * negative, however it was authored -- `-PT0S` is zero.
     */
    public function isNegative(): bool
    {
        return $this->negative && !$this->isZeroLength();
    }

    /**
     * Advance `$t` by this duration, honouring calendar semantics: weeks
     * and days move by calendar date, the hour/minute/second part is added
     * as elapsed time.
     *
     * Every V* instant is UTC, where a calendar day is always 24 hours, so
     * the two paths coincide here. They are kept separate anyway because
     * the distinction is the reason the authored units are preserved at
     * all, and a port that collapses them invites the collapse back into
     * {@see self::__toString()}.
     *
     * `DateTimeImmutable` throughout: the mutable class would modify `$t`
     * in place, which turns a repeat-alarm loop into a bug that appears
     * only after the first iteration.
     */
    public function addTo(\DateTimeInterface $t): \DateTimeImmutable
    {
        $at = \DateTimeImmutable::createFromInterface($t);

        $days = $this->weeks * self::DAYS_PER_WEEK + $this->days;
        $clock = $this->hours * self::SECONDS_PER_HOUR
            + $this->minutes * self::SECONDS_PER_MINUTE
            + $this->seconds;

        if ($days !== 0) {
            $calendar = new \DateInterval(sprintf('P%dD', $days));

            if ($this->negative) {
                $calendar->invert = 1;
            }

            $at = $at->add($calendar);
        }

        if ($clock !== 0) {
            $elapsed = new \DateInterval(sprintf('PT%dS', $clock));

            if ($this->negative) {
                $elapsed->invert = 1;
            }

            $at = $at->add($elapsed);
        }

        return $at;
    }

    /**
     * Whether every unit field is zero.
     */
    private function isZeroLength(): bool
    {
        return $this->weeks === 0
            && $this->days === 0
            && $this->hours === 0
            && $this->minutes === 0
            && $this->seconds === 0;
    }
}
