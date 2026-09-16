<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Generated\Codes;
use HopTop\Vstar\Supersession\Supersession as SupersessionLedger;

/**
 * spec/05 §4 -- supersession discipline.
 *
 * @internal
 */
final class Supersession
{
    /** The RFC 5545 §3.8.4.5 property carrying the supersession target. */
    private const RELATED_TO = 'RELATED-TO';

    /**
     * Check a supersession VJOURNAL against spec/05 §4.
     *
     * Two rules live here. The missing-properties rule is component-local:
     * a supersession journal that names no target, or records no effective
     * status, is incomplete on its own terms. The orphan rule is not -- it
     * asks whether the named target exists, which only a ledger can
     * answer.
     *
     * `$ledger` is null for the single-component entry point, and the
     * orphan rule is then skipped entirely rather than guessed at. Absence
     * of evidence is not evidence of an orphan: a component validated in
     * isolation has no calendar, so "the target is missing" is a claim it
     * is not in a position to make.
     *
     * @param ?list<Component> $ledger
     *
     * @return list<Diagnostic>
     */
    public static function check(Component $c, ?array $ledger, string $path): array
    {
        if ($c->type !== CompType::Journal->value) {
            return [];
        }

        if (!self::categoriesContainSupersession($c)) {
            return [];
        }

        $out = [];

        $related = $c->get(self::RELATED_TO);
        $target = $related === null ? '' : trim($related->value);

        if ($target === '') {
            $out[] = Diagnostic::of(
                Codes::SUPERSESSION_MISSING_PROPS,
                'supersession VJOURNAL missing ' . self::RELATED_TO . ' (spec/02, spec/05 §4)',
                $path . '.' . self::RELATED_TO,
            );
        }

        $effective = $c->get(SupersessionLedger::PROP_EFFECTIVE_STATUS);

        if ($effective === null || trim($effective->value) === '') {
            $out[] = Diagnostic::of(
                Codes::SUPERSESSION_MISSING_PROPS,
                'supersession VJOURNAL missing ' . SupersessionLedger::PROP_EFFECTIVE_STATUS
                    . ' (spec/02, spec/05 §4)',
                $path . '.' . SupersessionLedger::PROP_EFFECTIVE_STATUS,
            );
        }

        if ($related !== null && $ledger !== null && $target !== '' && !self::ledgerContainsUid($ledger, $target)) {
            $out[] = Diagnostic::of(
                Codes::SUPERSESSION_ORPHAN,
                'supersession VJOURNAL ' . self::RELATED_TO . '=' . $target
                    . ' has no matching component in calendar (spec/02, spec/05 §4)',
                $path . '.' . self::RELATED_TO,
            );
        }

        return $out;
    }

    /**
     * Whether `$c` carries a CATEGORIES token naming the supersession
     * category, comma-separated and compared case-insensitively.
     */
    private static function categoriesContainSupersession(Component $c): bool
    {
        foreach ($c->getAll('CATEGORIES') as $p) {
            foreach (explode(',', $p->value) as $tok) {
                if (Internal::equalFold(trim($tok), SupersessionLedger::CATEGORY_STATUS_SUPERSESSION)) {
                    return true;
                }
            }
        }

        return false;
    }

    /**
     * Whether any component in `$ledger` has UID `$uid`.
     *
     * Comparison is case-sensitive per RFC 5545 §3.8.4.7: UIDs are opaque
     * identifiers, not user-facing text.
     *
     * @param list<Component> $ledger
     */
    private static function ledgerContainsUid(array $ledger, string $uid): bool
    {
        foreach ($ledger as $c) {
            if ($c->uid() === $uid) {
                return true;
            }
        }

        return false;
    }
}
