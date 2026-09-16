<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * The root package's free functions and value-type constants.
 *
 * PHP has no free functions at namespace scope in the idiomatic style, so
 * the reference's package-level functions land here as static methods on
 * a final class named for the package.
 *
 * The datetime helpers delegate to {@see Time}, which is where the wire
 * formats and the VTIMEZONE-only TZID resolution live; they are re-exposed
 * here because the API mapping names them on the root.
 */
final class Vstar
{
    /**
     * The RFC 5545 §3.2.20 parameter name declaring a property's value
     * type explicitly.
     */
    public const VALUE_PARAM = 'VALUE';

    /**
     * The RFC 5545 §3.2.20 VALUE argument selecting the DATE value type
     * (§3.3.4).
     *
     * The parameter is REQUIRED on any date-only DTSTART / DTEND / DUE /
     * COMPLETED: the default value type for those properties is DATE-TIME
     * (RFC 5545 §3.8.2.4, §3.8.2.2, §3.8.2.3, §3.8.2.1), so an untagged
     * eight-octet value is a malformed DATE-TIME, not a DATE.
     */
    public const VALUE_DATE = 'DATE';

    /**
     * Render a date as an RFC 5545 §3.3.4 DATE string.
     *
     * @see VDate::format() for the rules on the zero and out-of-range
     *                      cases
     */
    public static function formatDate(VDate $d): string
    {
        return VDate::format($d);
    }

    /**
     * Parse an RFC 5545 §3.3.4 DATE string, or null.
     *
     * @see VDate::parse() for exactly what is rejected
     */
    public static function parseDate(string $s): ?VDate
    {
        return VDate::parse($s);
    }

    /**
     * Render an instant as an RFC 5545 §3.3.5 form #2 string.
     *
     * @see Time::formatTime() for the UTC conversion and the empty-string
     *                         convention
     */
    public static function formatTime(\DateTimeInterface $t): string
    {
        return Time::formatTime($t);
    }

    /**
     * Parse an RFC 5545 §3.3.5 form #2 string, or null.
     *
     * @see Time::parseTime() for exactly what is rejected
     */
    public static function parseTime(string $s): ?\DateTimeImmutable
    {
        return Time::parseTime($s);
    }

    /**
     * Resolve an RFC 5545 form #1 local value against a VTIMEZONE carried
     * by `$cal` itself, or null when the zone is absent or outside the
     * spec's VTIMEZONE subset.
     *
     * @see Time::parseTimeWithTzid() for the subset and why no IANA
     *                                database is ever consulted
     */
    public static function parseTimeWithTzid(string $s, string $tzid, Calendar $cal): ?\DateTimeImmutable
    {
        return Time::parseTimeWithTzid($s, $tzid, $cal);
    }

    /**
     * The calendar date of an instant, read in UTC.
     *
     * @see Time::dateOf()
     */
    public static function dateOf(\DateTimeInterface $t): VDate
    {
        return Time::dateOf($t);
    }

    /**
     * Whether two properties are semantically equal: names and parameter
     * names compared case-insensitively, values case-sensitively, and
     * parameter order normalized alphabetically before comparing.
     *
     * Named `propertyEqual` rather than `equal`: in the reference the
     * package qualifier carries the meaning, and a bare `equal` says
     * nothing about what it compares.
     *
     * Neither input is mutated.
     */
    public static function propertyEqual(Property $a, Property $b): bool
    {
        if (strcasecmp($a->name, $b->name) !== 0) {
            return false;
        }

        if ($a->value !== $b->value) {
            return false;
        }

        if (count($a->params) !== count($b->params)) {
            return false;
        }

        $ap = self::sortedParams($a->params);
        $bp = self::sortedParams($b->params);

        foreach ($ap as $i => $param) {
            if (strcasecmp($param->name, $bp[$i]->name) !== 0) {
                return false;
            }

            if ($param->value !== $bp[$i]->value) {
                return false;
            }
        }

        return true;
    }

    /**
     * A copy of `$params` sorted by upper-cased name. The sort is stable,
     * so two parameters that differ only in case keep their relative
     * input order and the comparison stays deterministic.
     *
     * @param list<Param> $params
     *
     * @return list<Param>
     */
    private static function sortedParams(array $params): array
    {
        $out = $params;
        usort(
            $out,
            static fn (Param $x, Param $y): int => strcmp(strtoupper($x->name), strtoupper($y->name)),
        );

        return $out;
    }
}
