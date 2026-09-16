<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\EventStatus;
use HopTop\Vstar\Generated\Codes;
use HopTop\Vstar\JournalStatus;
use HopTop\Vstar\TodoStatus;

/**
 * spec/05 §8 -- the STATUS value domain.
 *
 * @internal
 */
final class Status
{
    /**
     * Flag a STATUS whose value is outside its own component type's
     * vocabulary.
     *
     * Comparison is case-insensitive per RFC 5545 §3.1. An absent STATUS
     * is clean -- the property is optional on every type that admits it.
     *
     * @return list<Diagnostic>
     */
    public static function check(Component $c, string $path): array
    {
        $allowed = self::vocabularies()[$c->type] ?? null;

        if ($allowed === null) {
            return [];
        }

        $p = $c->get('STATUS');

        if ($p === null) {
            return [];
        }

        foreach ($allowed as $want) {
            if (Internal::equalFold($p->value, $want)) {
                return [];
            }
        }

        return [Diagnostic::of(
            Codes::STATUS_NOT_IN_VOCABULARY,
            'STATUS value ' . $p->value . ' is not valid for ' . $c->type
                . '; allowed: ' . implode(', ', $allowed) . ' (RFC 5545 §3.8.1.11)',
            $path . '.STATUS',
        )];
    }

    /**
     * The STATUS values RFC 5545 §3.8.1.11 scopes to each component type.
     *
     * The vocabularies are per-type, not global: CANCELLED is the only
     * value all three share, and DRAFT -- legal iCalendar text, and legal
     * on a VJOURNAL -- is a conformance violation on a VEVENT. A type
     * absent from this table admits no STATUS vocabulary at all
     * (VFREEBUSY, VTIMEZONE, VALARM, VCALENDAR) and is skipped.
     *
     * The table is built from the port's own backed enums rather than
     * read out of `Codes::STATUS_VOCABULARY`, deliberately. These are the
     * values the codec encodes against, so a table built from them cannot
     * disagree with what this library actually writes -- a guarantee a
     * lookup into the generated table would give up. The registry's
     * cross-language copy is reconciled against this one in
     * `tests/Validate/RegistryVocabularyTest.php`, which is what keeps
     * the five ports agreeing without any of them losing the codec
     * linkage.
     *
     * @return array<string, list<string>>
     */
    private static function vocabularies(): array
    {
        return [
            CompType::Event->value => [
                EventStatus::Tentative->value,
                EventStatus::Confirmed->value,
                EventStatus::Cancelled->value,
            ],
            CompType::Todo->value => [
                TodoStatus::NeedsAction->value,
                TodoStatus::InProcess->value,
                TodoStatus::Completed->value,
                TodoStatus::Cancelled->value,
            ],
            CompType::Journal->value => [
                JournalStatus::Draft->value,
                JournalStatus::Final->value,
                JournalStatus::Cancelled->value,
            ],
        ];
    }
}
