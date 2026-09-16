<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Generated\Codes;
use HopTop\Vstar\TodoStatus;

/**
 * spec/05 §5 -- type-specific required properties.
 *
 * @internal
 */
final class Types
{
    /**
     * The `VCARD` wire type, as it appears on a {@see Component}.
     *
     * {@see CompType} excludes it deliberately: a top-level vCard is a
     * `Card`. A `VCARD` block nested inside a `VCALENDAR` still parses
     * into a `Component` carrying the wire string, and the rule below
     * exists so a caller hand-building one still gets the check.
     */
    private const VCARD = 'VCARD';

    /**
     * Dispatch the per-type rule for `$c`. Component types with no extra
     * MUST in spec/05 §5 -- VJOURNAL, VTIMEZONE, VALARM, VCALENDAR --
     * yield nothing.
     *
     * @return list<Diagnostic>
     */
    public static function check(Component $c, string $path): array
    {
        return match ($c->type) {
            CompType::Todo->value => self::vtodo($c, $path),
            CompType::Event->value => self::vevent($c, $path),
            CompType::FreeBusy->value => self::vfreebusy($c, $path),
            self::VCARD => self::vcard($c, $path),
            default => [],
        };
    }

    /**
     * A VTODO must be reachable as "scheduled": either DUE is present, or
     * STATUS=COMPLETED is paired with a COMPLETED timestamp.
     *
     * RFC 5545 §3.6.2 lets a finished VTODO drop DUE so long as COMPLETED
     * records when it finished; that route is honoured rather than
     * demanding a due date the task no longer has.
     *
     * @return list<Diagnostic>
     */
    private static function vtodo(Component $c, string $path): array
    {
        if (Internal::has($c, 'DUE')) {
            return [];
        }

        $status = $c->get('STATUS');

        if (
            $status !== null
            && Internal::equalFold($status->value, TodoStatus::Completed->value)
            && Internal::has($c, 'COMPLETED')
        ) {
            return [];
        }

        return [Diagnostic::of(
            Codes::VTODO_MISSING_DUE,
            'VTODO requires DUE, or STATUS=COMPLETED paired with COMPLETED (spec/05 §5; RFC 5545 §3.6.2)',
            $path,
        )];
    }

    /**
     * A VEVENT must carry DTSTART. RFC 5545 §3.6.1 allows its absence
     * outside a PUBLISH METHOD context; V* is strict, because an agentic
     * playthrough always anchors to a start time.
     *
     * @return list<Diagnostic>
     */
    private static function vevent(Component $c, string $path): array
    {
        if (Internal::has($c, 'DTSTART')) {
            return [];
        }

        return [Diagnostic::of(
            Codes::VEVENT_MISSING_DTSTART,
            'VEVENT requires DTSTART (spec/05 §5; RFC 5545 §3.6.1)',
            $path . '.DTSTART',
        )];
    }

    /**
     * A VFREEBUSY must carry both DTSTART and DTEND; the message names
     * which is missing.
     *
     * @return list<Diagnostic>
     */
    private static function vfreebusy(Component $c, string $path): array
    {
        $missing = Internal::missing($c, ['DTSTART', 'DTEND']);

        if ($missing === []) {
            return [];
        }

        return [Diagnostic::of(
            Codes::VFREEBUSY_MISSING_TIMES,
            'VFREEBUSY requires DTSTART and DTEND; missing: ' . implode(', ', $missing)
                . ' (spec/05 §5; RFC 5545 §3.6.4)',
            $path,
        )];
    }

    /**
     * A VCARD-as-Component must carry both VERSION and UID.
     *
     * @return list<Diagnostic>
     */
    private static function vcard(Component $c, string $path): array
    {
        $missing = Internal::missing($c, ['VERSION', 'UID']);

        if ($missing === []) {
            return [];
        }

        return [Diagnostic::of(
            Codes::VCARD_MISSING_REQUIRED,
            'VCARD requires VERSION and UID; missing: ' . implode(', ', $missing)
                . ' (spec/05 §5; RFC 6350 §6.7.6, §6.7.9)',
            $path,
        )];
    }
}
