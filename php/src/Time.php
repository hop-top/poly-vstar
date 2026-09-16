<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * RFC 5545 §3.3.5 DATE-TIME: form #2 (`YYYYMMDDTHHMMSSZ`, UTC) on the
 * wire, and form #1 (`YYYYMMDDTHHMMSS`, local) resolved against a
 * VTIMEZONE carried by the document itself.
 *
 * # No IANA timezone database, ever
 *
 * A local time resolves **only** against the VTIMEZONE definitions inside
 * the calendar being processed. `DateTimeZone('America/Montreal')` and
 * `date_default_timezone_get()` are deliberately unused: a named zone
 * reads the system tzdata, and two machines with different tzdata
 * releases would then produce different canonical bytes for the same
 * document -- the exact failure the specification exists to prevent.
 *
 * The offsets this class reconstructs come from the document's own
 * `TZOFFSETTO` properties and are applied as fixed-offset
 * `DateTimeZone('+HH:MM')` values, which consult nothing.
 *
 * Every instant crossing this boundary is a `DateTimeImmutable` in UTC.
 * The mutable `DateTime` is never used: `$a->add($i)` modifies `$a` in
 * place, which turns an occurrence loop into a bug that only appears
 * after the first iteration.
 *
 * PHP has no free functions at namespace scope in the idiomatic style, so
 * the reference's package-level functions land here as static methods.
 * {@see Vstar} re-exports the four the API mapping names on the root.
 */
final class Time
{
    /** Form #2 is exactly 16 octets: 8 date + `T` + 6 time + `Z`. */
    private const FORM_TWO_OCTETS = 16;

    /** Form #1 is exactly 15 octets: 8 date + `T` + 6 time, no `Z`. */
    private const FORM_ONE_OCTETS = 15;

    private const SECONDS_PER_HOUR = 3600;

    private const SECONDS_PER_MINUTE = 60;

    /**
     * Render `$t` as an RFC 5545 §3.3.5 form #2 string --
     * `YYYYMMDDTHHMMSSZ` in UTC.
     *
     * The epoch instant renders as the empty string, which is how the
     * property writers spell "clear the property" -- the same convention
     * {@see VDate::format()} uses for the zero date.
     *
     * The value is converted to UTC first: an input carrying any other
     * offset would otherwise render its local wall clock with a `Z`
     * suffix, which is a different instant wearing the right shape.
     */
    public static function formatTime(\DateTimeInterface $t): string
    {
        if ($t->getTimestamp() === 0) {
            return '';
        }

        $utc = \DateTimeImmutable::createFromInterface($t)
            ->setTimezone(new \DateTimeZone('UTC'));

        return $utc->format('Ymd\THis\Z');
    }

    /**
     * Parse an RFC 5545 §3.3.5 form #2 string (`YYYYMMDDTHHMMSSZ`) into a
     * UTC `DateTimeImmutable`. Null for any other input shape -- strict by
     * design.
     *
     * Rejected, specifically: form #1 (no zone, unsupported bare in
     * v0.1); RFC 3339 / ISO 8601 extended layouts; date-only values; a
     * lowercase `z`; leading or trailing whitespace; any length other than
     * sixteen octets; and impossible calendar dates -- February 30th does
     * not roll into March.
     */
    public static function parseTime(string $s): ?\DateTimeImmutable
    {
        if (strlen($s) !== self::FORM_TWO_OCTETS) {
            return null;
        }

        if ($s[15] !== 'Z') {
            return null;
        }

        $wall = self::parseWallFields(substr($s, 0, self::FORM_ONE_OCTETS));

        return $wall === null ? null : self::wallToUtc($wall);
    }

    /**
     * Parse an RFC 5545 §3.3.5 form #1 value as a wall-clock time in the
     * zone `$tzid` names, where the zone is reconstructed from a VTIMEZONE
     * component inside `$cal`. The result is in UTC.
     *
     * Null -- not an exception -- when:
     *
     * - `$tzid` is empty;
     * - `$cal` carries no VTIMEZONE whose TZID matches (comparison is
     *   case-**sensitive**: TZIDs are opaque identifiers per RFC 5545
     *   §3.2.19);
     * - the matching VTIMEZONE falls outside the spec's v0.1 subset --
     *   multiple STANDARD or DAYLIGHT children, a missing offset or
     *   DTSTART, or an RRULE the subset does not accept;
     * - `$s` is not form #1 (15 octets, `T` at index 8, no `Z` suffix).
     *
     * Form #2 input is rejected even with a TZID present: the value is
     * already absolute, and resolving it against a zone would apply an
     * offset twice. Rule 5 falls back to verbatim emit in that case.
     *
     * Resolution failure is a normal outcome. The canonical layer passes
     * the value and its TZID parameter through unchanged.
     */
    public static function parseTimeWithTzid(string $s, string $tzid, Calendar $cal): ?\DateTimeImmutable
    {
        if ($tzid === '') {
            return null;
        }

        $wall = self::parseFormOne($s);

        if ($wall === null) {
            return null;
        }

        $tz = self::findVTimezone($tzid, $cal);

        if ($tz === null) {
            return null;
        }

        $rules = self::loadTzRules($tz);

        if ($rules === null) {
            return null;
        }

        $offset = self::selectActiveOffset($wall, $rules);

        // The offset is applied as a fixed `+HH:MM` zone built from the
        // document's own TZOFFSETTO -- never a named IANA zone, which
        // would read the system tzdata.
        return self::wallInFixedOffset($wall, $offset)
            ->setTimezone(new \DateTimeZone('UTC'));
    }

    /**
     * The DTSTART value as an instant, honouring a TZID parameter against
     * `$cal`'s VTIMEZONE registry.
     *
     * The calendar argument is not optional plumbing: the registry lives
     * on the Calendar, not on the Component.
     */
    public static function dtstart(Component $c, Calendar $cal): ?\DateTimeImmutable
    {
        return self::datetimeProp($c, 'DTSTART', $cal);
    }

    /**
     * The DTEND value as an instant.
     */
    public static function dtend(Component $c, Calendar $cal): ?\DateTimeImmutable
    {
        return self::datetimeProp($c, 'DTEND', $cal);
    }

    /**
     * The VTODO DUE value as an instant.
     */
    public static function due(Component $c, Calendar $cal): ?\DateTimeImmutable
    {
        return self::datetimeProp($c, 'DUE', $cal);
    }

    /**
     * The COMPLETED value as an instant.
     */
    public static function completed(Component $c, Calendar $cal): ?\DateTimeImmutable
    {
        return self::datetimeProp($c, 'COMPLETED', $cal);
    }

    /**
     * The DTSTAMP value as an instant.
     *
     * No calendar argument: RFC 5545 §3.8.7.2 requires DTSTAMP to be UTC,
     * so there is never a zone to resolve.
     */
    public static function dtstamp(Component $c): ?\DateTimeImmutable
    {
        return self::parseTime($c->dtstampRaw());
    }

    /**
     * Write DTSTART as a UTC form #2 instant.
     */
    public static function setDtstart(Component $c, \DateTimeInterface $t): void
    {
        self::setOrClearTime($c, 'DTSTART', $t);
    }

    /**
     * Write DTEND as a UTC form #2 instant.
     */
    public static function setDtend(Component $c, \DateTimeInterface $t): void
    {
        self::setOrClearTime($c, 'DTEND', $t);
    }

    /**
     * Write DUE as a UTC form #2 instant.
     */
    public static function setDue(Component $c, \DateTimeInterface $t): void
    {
        self::setOrClearTime($c, 'DUE', $t);
    }

    /**
     * Write COMPLETED as a UTC form #2 instant.
     */
    public static function setCompleted(Component $c, \DateTimeInterface $t): void
    {
        self::setOrClearTime($c, 'COMPLETED', $t);
    }

    /**
     * The calendar date of an instant, read in UTC.
     */
    public static function dateOf(\DateTimeInterface $t): VDate
    {
        $utc = \DateTimeImmutable::createFromInterface($t)
            ->setTimezone(new \DateTimeZone('UTC'));

        return new VDate(
            (int) $utc->format('Y'),
            (int) $utc->format('n'),
            (int) $utc->format('j'),
        );
    }

    /**
     * Resolve a datetime-bearing property to an instant.
     *
     * Null for a missing property, a `VALUE=DATE` property (a calendar
     * date is not an instant -- read it with the date-typed accessors),
     * or a value that is neither form #2 nor a resolvable form #1.
     */
    private static function datetimeProp(Component $c, string $name, Calendar $cal): ?\DateTimeImmutable
    {
        $p = $c->get($name);

        if ($p === null) {
            return null;
        }

        $valueType = $p->param(Vstar::VALUE_PARAM);

        if ($valueType !== null && strcasecmp($valueType, Vstar::VALUE_DATE) === 0) {
            return null;
        }

        $direct = self::parseTime($p->value);

        if ($direct !== null) {
            return $direct;
        }

        $tzid = $p->param('TZID');

        return $tzid === null ? null : self::parseTimeWithTzid($p->value, $tzid, $cal);
    }

    /**
     * Write `$t` as the named property in UTC form #2, or remove the
     * property entirely when `$t` is the epoch instant.
     *
     * The written property carries no parameters. Dropping any it had is
     * required, not merely tidy: a stale TZID on a value now spelled in
     * UTC would contradict the value, and a stale parameter set would make
     * the canonical bytes depend on the property's edit history.
     */
    private static function setOrClearTime(Component $c, string $name, \DateTimeInterface $t): void
    {
        $value = self::formatTime($t);

        if ($value === '') {
            $c->remove($name);

            return;
        }

        $c->set(new Property($name, [], $value));
    }

    /**
     * Decode `$s` as form #1, rejecting a `Z` suffix explicitly so a form
     * #2 value never reaches the zone arithmetic.
     *
     * @return array{year: int, month: int, day: int, hour: int, minute: int, second: int}|null
     */
    private static function parseFormOne(string $s): ?array
    {
        if (strlen($s) !== self::FORM_ONE_OCTETS) {
            return null;
        }

        $last = $s[self::FORM_ONE_OCTETS - 1];

        if ($last === 'Z' || $last === 'z') {
            return null;
        }

        return self::parseWallFields($s);
    }

    /**
     * Decode an RFC 5545 form #1 wire string into its wall-clock fields,
     * validating every field's range and the month's real length. Null on
     * any shape or range violation.
     *
     * @return array{year: int, month: int, day: int, hour: int, minute: int, second: int}|null
     */
    private static function parseWallFields(string $s): ?array
    {
        if (strlen($s) !== self::FORM_ONE_OCTETS) {
            return null;
        }

        if (preg_match('/\A(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})\z/', $s, $m) !== 1) {
            return null;
        }

        $year = (int) $m[1];
        $month = (int) $m[2];
        $day = (int) $m[3];
        $hour = (int) $m[4];
        $minute = (int) $m[5];
        $second = (int) $m[6];

        // checkdate rejects Feb 30 and Feb 29 in a non-leap year rather
        // than rolling forward -- a torn date is not a date in March.
        if (!checkdate($month, $day, $year)) {
            return null;
        }

        if ($hour > 23 || $minute > 59 || $second > 59) {
            return null;
        }

        return [
            'year' => $year,
            'month' => $month,
            'day' => $day,
            'hour' => $hour,
            'minute' => $minute,
            'second' => $second,
        ];
    }

    /**
     * A wall-clock field set read as UTC.
     *
     * @param array{year: int, month: int, day: int, hour: int, minute: int, second: int} $w
     */
    private static function wallToUtc(array $w): \DateTimeImmutable
    {
        return self::wallInFixedOffset($w, 0);
    }

    /**
     * A wall-clock field set read in a fixed offset of `$offsetSeconds`
     * east of UTC.
     *
     * The zone is built from the numeric offset alone -- `+HH:MM`, or
     * `+HH:MM:SS` where the document's TZOFFSETTO carries seconds -- so
     * nothing consults the system timezone database.
     *
     * @param array{year: int, month: int, day: int, hour: int, minute: int, second: int} $w
     */
    private static function wallInFixedOffset(array $w, int $offsetSeconds): \DateTimeImmutable
    {
        $at = new \DateTimeImmutable(
            sprintf(
                '%04d-%02d-%02dT%02d:%02d:%02d',
                $w['year'],
                $w['month'],
                $w['day'],
                $w['hour'],
                $w['minute'],
                $w['second'],
            ),
            self::fixedZone($offsetSeconds),
        );

        return $at;
    }

    /**
     * A fixed-offset `DateTimeZone` for `$offsetSeconds` east of UTC.
     *
     * Never a named zone. `DateTimeZone` accepts `±HH:MM` and, since PHP
     * 8.0, `±HH:MM:SS`; the seconds form is only reached by a VTIMEZONE
     * whose TZOFFSETTO is the `±HHMMSS` variant RFC 5545 §3.3.14 allows.
     */
    private static function fixedZone(int $offsetSeconds): \DateTimeZone
    {
        $sign = $offsetSeconds < 0 ? '-' : '+';
        $abs = abs($offsetSeconds);
        $hours = intdiv($abs, self::SECONDS_PER_HOUR);
        $minutes = intdiv($abs % self::SECONDS_PER_HOUR, self::SECONDS_PER_MINUTE);
        $seconds = $abs % self::SECONDS_PER_MINUTE;

        $spec = $seconds === 0
            ? sprintf('%s%02d:%02d', $sign, $hours, $minutes)
            : sprintf('%s%02d:%02d:%02d', $sign, $hours, $minutes, $seconds);

        return new \DateTimeZone($spec);
    }

    /**
     * The VTIMEZONE in `$cal` whose TZID property equals `$tzid`.
     */
    private static function findVTimezone(string $tzid, Calendar $cal): ?Component
    {
        foreach ($cal->filter(CompType::Timezone) as $c) {
            $p = $c->get('TZID');

            if ($p !== null && $p->value === $tzid) {
                return $c;
            }
        }

        return null;
    }

    /**
     * Sub-components of `$c` whose type matches `$name` exactly (uppercase
     * per RFC 5545 §3.6.5).
     *
     * @return list<Component>
     */
    private static function subsByType(Component $c, string $name): array
    {
        $out = [];

        foreach ($c->sub as $s) {
            if ($s->type === $name) {
                $out[] = $s;
            }
        }

        return $out;
    }

    /**
     * Extract the STANDARD/DAYLIGHT rule pair from a VTIMEZONE, or null
     * for any shape outside the spec's v0.1 subset.
     *
     * Accepted: a single STANDARD (fixed offset); a single STANDARD plus a
     * single DAYLIGHT where both carry an accepted `FREQ=YEARLY` rule; and
     * a DAYLIGHT alone, treated as a fixed offset since there is no
     * transition to compute.
     *
     * @return array{std: array<string, int|bool>, dst: array<string, int|bool>|null}|null
     */
    private static function loadTzRules(Component $tz): ?array
    {
        $standards = self::subsByType($tz, 'STANDARD');
        $daylights = self::subsByType($tz, 'DAYLIGHT');

        if ($standards === [] && $daylights === []) {
            return null;
        }

        // Split-zone histories are outside the subset.
        if (count($standards) > 1 || count($daylights) > 1) {
            return null;
        }

        if ($standards === []) {
            $only = self::parseTzRule($daylights[0]);

            return $only === null ? null : ['std' => $only, 'dst' => null];
        }

        $std = self::parseTzRule($standards[0]);

        if ($std === null) {
            return null;
        }

        if ($daylights === []) {
            return ['std' => $std, 'dst' => null];
        }

        $dst = self::parseTzRule($daylights[0]);

        if ($dst === null) {
            return null;
        }

        // With both children present the subset requires a yearly rule on
        // each: without one there is no transition date to compute, and
        // guessing would produce a plausible instant that is wrong.
        if ($std['yearly'] !== true || $dst['yearly'] !== true) {
            return null;
        }

        return ['std' => $std, 'dst' => $dst];
    }

    /**
     * The offset in seconds east of UTC that applies to `$wall`.
     *
     * With no DAYLIGHT child the STANDARD offset applies unconditionally.
     * Otherwise both transitions are placed in `$wall`'s own year and
     * compared on the same naive timeline -- which orders two wall events
     * within one year correctly, and is all the comparison needs.
     *
     * @param array{year: int, month: int, day: int, hour: int, minute: int, second: int} $wall
     * @param array{std: array<string, int|bool>, dst: array<string, int|bool>|null}      $rs
     */
    private static function selectActiveOffset(array $wall, array $rs): int
    {
        $std = $rs['std'];
        $dst = $rs['dst'];

        /** @var int $stdOffset */
        $stdOffset = $std['offsetTo'];

        if ($dst === null) {
            return $stdOffset;
        }

        /** @var int $dstOffset */
        $dstOffset = $dst['offsetTo'];

        $at = self::wallToUtc($wall)->getTimestamp();
        $dstStart = self::transitionAt($wall['year'], $dst);
        $stdStart = self::transitionAt($wall['year'], $std);

        return ($at >= $dstStart && $at < $stdStart) ? $dstOffset : $stdOffset;
    }

    /**
     * The naive instant at which `$r` becomes active in `$year`, expressed
     * on the UTC timeline. The absolute value is meaningless; only the
     * ordering of two such instants from the same year is used.
     *
     * @param array<string, int|bool> $r
     */
    private static function transitionAt(int $year, array $r): int
    {
        /** @var int $month */
        $month = $r['month'];
        /** @var int $weekday */
        $weekday = $r['weekday'];
        /** @var int $week */
        $week = $r['week'];
        /** @var int $hour */
        $hour = $r['hour'];
        /** @var int $minute */
        $minute = $r['minute'];
        /** @var int $second */
        $second = $r['second'];

        $day = self::nthWeekdayOfMonth($year, $month, $weekday, $week);

        return self::wallToUtc([
            'year' => $year,
            'month' => $month,
            'day' => $day,
            'hour' => $hour,
            'minute' => $minute,
            'second' => $second,
        ])->getTimestamp();
    }

    /**
     * The day-of-month of the `$n`th `$weekday` in `($year, $month)`.
     * Positive `$n` counts from the start (1 = first); negative counts
     * from the end (-1 = last). `$n = 0` is rejected at parse time.
     *
     * `$weekday` is `SU = 0` per RFC 5545 §3.3.10, which is also what
     * `format('w')` reports -- unlike `format('N')`, which is ISO-8601's
     * `MO = 1`.
     */
    private static function nthWeekdayOfMonth(int $year, int $month, int $weekday, int $n): int
    {
        if ($n > 0) {
            $first = self::wallToUtc([
                'year' => $year,
                'month' => $month,
                'day' => 1,
                'hour' => 0,
                'minute' => 0,
                'second' => 0,
            ]);
            $offset = ($weekday - (int) $first->format('w') + 7) % 7;

            return 1 + $offset + ($n - 1) * 7;
        }

        $lastDay = (int) self::wallToUtc([
            'year' => $year,
            'month' => $month,
            'day' => 1,
            'hour' => 0,
            'minute' => 0,
            'second' => 0,
        ])->format('t');

        $last = self::wallToUtc([
            'year' => $year,
            'month' => $month,
            'day' => $lastDay,
            'hour' => 0,
            'minute' => 0,
            'second' => 0,
        ]);
        $offset = ((int) $last->format('w') - $weekday + 7) % 7;

        return $lastDay - $offset + ($n + 1) * 7;
    }

    /**
     * Read one STANDARD/DAYLIGHT child into a rule, or null for any field
     * shape outside the subset.
     *
     * @return array<string, int|bool>|null
     */
    private static function parseTzRule(Component $c): ?array
    {
        $offsetTo = self::readOffset($c, 'TZOFFSETTO');

        if ($offsetTo === null) {
            return null;
        }

        // TZOFFSETFROM is not used in the arithmetic, but the subset
        // requires it present and well-formed -- a child missing it is a
        // producer shape this implementation declines to guess at.
        if (self::readOffset($c, 'TZOFFSETFROM') === null) {
            return null;
        }

        $dtstart = $c->get('DTSTART');

        if ($dtstart === null) {
            return null;
        }

        $wall = self::parseFormOne($dtstart->value);

        if ($wall === null) {
            return null;
        }

        $rrule = $c->get('RRULE');

        if ($rrule === null) {
            return [
                'offsetTo' => $offsetTo,
                'yearly' => false,
                'month' => 0,
                'weekday' => 0,
                'week' => 0,
                'hour' => $wall['hour'],
                'minute' => $wall['minute'],
                'second' => $wall['second'],
            ];
        }

        $yearly = self::parseYearlyRrule($rrule->value);

        if ($yearly === null) {
            return null;
        }

        return [
            'offsetTo' => $offsetTo,
            'yearly' => true,
            'month' => $yearly['month'],
            'weekday' => $yearly['weekday'],
            'week' => $yearly['week'],
            'hour' => $wall['hour'],
            'minute' => $wall['minute'],
            'second' => $wall['second'],
        ];
    }

    /**
     * Read a `±HHMM` or `±HHMMSS` UTC-offset property as seconds east of
     * UTC. Null on any shape mismatch.
     */
    private static function readOffset(Component $c, string $name): ?int
    {
        $p = $c->get($name);

        if ($p === null) {
            return null;
        }

        if (preg_match('/\A([+-])(\d{2})(\d{2})(\d{2})?\z/', $p->value, $m) !== 1) {
            return null;
        }

        $sign = $m[1] === '-' ? -1 : 1;
        $hh = (int) $m[2];
        $mm = (int) $m[3];
        // The seconds group is optional: RFC 5545 §3.3.14 admits both
        // `±HHMM` and `±HHMMSS`, and only the latter fills group 4.
        $ss = isset($m[4]) ? (int) $m[4] : 0;

        return $sign * ($hh * self::SECONDS_PER_HOUR + $mm * self::SECONDS_PER_MINUTE + $ss);
    }

    /**
     * Accept only the VTIMEZONE RRULE subset: `FREQ=YEARLY` with an
     * optional `BYMONTH`, an ordinal `BYDAY`, and a no-op `INTERVAL=1`.
     *
     * Everything else fails closed -- UNTIL, COUNT, BYWEEKNO, BYSETPOS,
     * WKST, an unknown key, a `BYDAY` without an ordinal -- because
     * applying a partial rule would produce a plausible instant that is
     * wrong.
     *
     * This is deliberately narrow and unrelated to the generic RRULE
     * parsing scope, which lands with the recurrence layer.
     *
     * @return array{month: int, weekday: int, week: int}|null
     */
    private static function parseYearlyRrule(string $s): ?array
    {
        $freqSeen = false;
        $month = 0;
        $weekday = 0;
        $week = 0;

        foreach (explode(';', $s) as $part) {
            $eq = strpos($part, '=');

            if ($eq === false) {
                return null;
            }

            $key = strtoupper(substr($part, 0, $eq));
            $val = substr($part, $eq + 1);

            switch ($key) {
                case 'FREQ':
                    if (strcasecmp($val, 'YEARLY') !== 0) {
                        return null;
                    }

                    $freqSeen = true;

                    break;

                case 'BYMONTH':
                    if (preg_match('/\A\d+\z/', $val) !== 1) {
                        return null;
                    }

                    $m = (int) $val;

                    if ($m < 1 || $m > 12) {
                        return null;
                    }

                    $month = $m;

                    break;

                case 'BYDAY':
                    $byday = self::parseByday($val);

                    if ($byday === null) {
                        return null;
                    }

                    $week = $byday['week'];
                    $weekday = $byday['weekday'];

                    break;

                case 'INTERVAL':
                    // A no-op INTERVAL=1 is accepted; anything else is not.
                    if ($val !== '1') {
                        return null;
                    }

                    break;

                default:
                    return null;
            }
        }

        return $freqSeen ? ['month' => $month, 'weekday' => $weekday, 'week' => $week] : null;
    }

    /**
     * Parse one BYDAY entry such as `2SU` or `-1SU`. The ordinal-less
     * forms (`SU`, `0SU`) are rejected: a VTIMEZONE transition needs an
     * explicit nth.
     *
     * @return array{week: int, weekday: int}|null
     */
    private static function parseByday(string $s): ?array
    {
        if (preg_match('/\A([+-]?\d+)(SU|MO|TU|WE|TH|FR|SA)\z/i', $s, $m) !== 1) {
            return null;
        }

        $week = (int) $m[1];

        if ($week === 0) {
            return null;
        }

        return ['week' => $week, 'weekday' => self::WEEKDAYS[strtoupper($m[2])]];
    }

    /**
     * Weekday codes, `SU = 0` per RFC 5545 §3.3.10 -- not ISO-8601's
     * `MO = 1`, and not `DateTime::format('N')`.
     */
    private const WEEKDAYS = [
        'SU' => 0,
        'MO' => 1,
        'TU' => 2,
        'WE' => 3,
        'TH' => 4,
        'FR' => 5,
        'SA' => 6,
    ];
}
