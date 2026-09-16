<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * An RFC 5545 §3.3.4 DATE value: a calendar date with no time and no
 * time zone.
 *
 * Named `VDate` rather than `Date` because PHP ships a `Date` class of
 * its own; the reference calls it `Date`.
 *
 * # Why a distinct type
 *
 * DATE and DATE-TIME are semantically different, not two spellings of one
 * thing. `DUE;VALUE=DATE:20260515` means "due on the 15th, as reckoned by
 * whoever reads it"; `DUE:20260515T000000Z` means "due at one specific
 * instant, the stroke of midnight UTC". A task due on the 15th is not
 * late at 00:00:01Z; a task due at midnight UTC is.
 *
 * A `DateTimeImmutable` cannot carry that distinction: it always has
 * clock fields and a zone, so a date-only value stored in one is
 * indistinguishable from a midnight instant, and the caller is left
 * consulting an out-of-band flag to know which meaning applies. Forgetting
 * to is silent data corruption rather than a type error. VDate has no
 * clock and no zone at all, so the distinction cannot be lost by accident.
 *
 * The zero VDate is the "no date" sentinel: it formats as the empty string
 * and the date-typed setters read it as "clear the property".
 *
 * `$month` is 1-based (January is 1), not a zero-based month index.
 */
final class VDate
{
    /** The exact wire width of an RFC 5545 §3.3.4 DATE. */
    private const OCTETS = 8;

    public function __construct(
        public readonly int $year,
        public readonly int $month,
        public readonly int $day,
    ) {
    }

    /**
     * A VDate from its three fields. `VDate::of(0, 0, 0)` is the zero
     * date.
     */
    public static function of(int $year, int $month, int $day): self
    {
        return new self($year, $month, $day);
    }

    /**
     * Whether this is the zero VDate -- the "no date" sentinel.
     */
    public function isZero(): bool
    {
        return $this->year === 0 && $this->month === 0 && $this->day === 0;
    }

    /**
     * This date as midnight UTC, for callers handing the value to
     * time-based arithmetic.
     *
     * The conversion is lossy by design and one-way: the result no longer
     * records that its source was date-only. Do not round-trip a VDate
     * through this to store it -- use the date-typed accessors, which
     * preserve the DATE value type on the wire.
     */
    public function toDateTime(): \DateTimeImmutable
    {
        if ($this->isZero()) {
            return new \DateTimeImmutable('@0', new \DateTimeZone('UTC'));
        }

        return new \DateTimeImmutable(
            sprintf('%04d-%02d-%02dT00:00:00', $this->year, $this->month, $this->day),
            new \DateTimeZone('UTC'),
        );
    }

    /**
     * Whether two dates name the same day.
     */
    public function equals(self $other): bool
    {
        return $this->year === $other->year
            && $this->month === $other->month
            && $this->day === $other->day;
    }

    /**
     * The RFC 5545 §3.3.4 wire form, or the empty string for the zero
     * date. Equivalent to {@see Vstar::formatDate()}.
     */
    public function __toString(): string
    {
        return Vstar::formatDate($this);
    }

    /**
     * Render a date as an RFC 5545 §3.3.4 DATE string -- `YYYYMMDD`,
     * zero-padded to eight octets.
     *
     * The zero date renders as the empty string, which the date-typed
     * writers use to mean "clear the property".
     *
     * Out-of-range fields (month 13, day 32, a year outside 0000-9999)
     * render as the empty string rather than an impossible wire form: the
     * DATE production is a fixed-width four-digit year, and emitting torn
     * data would defeat the strictness the reader enforces. Note the
     * deliberate absence of any normalization -- Feb 30 does not become
     * Mar 2 here, it becomes nothing, so a producer bug surfaces instead
     * of being laundered.
     */
    public static function format(self $d): string
    {
        if ($d->isZero()) {
            return '';
        }

        if (
            $d->year < 0 || $d->year > 9999
            || $d->month < 1 || $d->month > 12
            || $d->day < 1 || $d->day > 31
        ) {
            return '';
        }

        return sprintf('%04d%02d%02d', $d->year, $d->month, $d->day);
    }

    /**
     * Parse an RFC 5545 §3.3.4 DATE string (`YYYYMMDD`). Returns null for
     * any other shape -- strict by design, matching the reference.
     *
     * Rejected, specifically: DATE-TIME forms; ISO 8601 extended layouts
     * (`2026-05-15`); impossible calendar dates (Feb 30, month 13, day 0,
     * Feb 29 in a non-leap year) with no silent roll-over; empty strings;
     * leading or trailing whitespace; and anything not exactly eight
     * octets long.
     */
    public static function parse(string $s): ?self
    {
        if (strlen($s) !== self::OCTETS) {
            return null;
        }

        if (preg_match('/\A\d{8}\z/', $s) !== 1) {
            return null;
        }

        $year = (int) substr($s, 0, 4);
        $month = (int) substr($s, 4, 2);
        $day = (int) substr($s, 6, 2);

        // checkdate rejects Feb 30 and Feb 29 in a non-leap year rather
        // than rolling them forward, which is the behavior we want: a
        // torn date is not a date one month later.
        if (!checkdate($month, $day, $year)) {
            return null;
        }

        return new self($year, $month, $day);
    }
}
