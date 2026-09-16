<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Supersession;

use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Exception\TargetCorruptedException;
use HopTop\Vstar\Hashing\Hashing;
use HopTop\Vstar\Property;
use HopTop\Vstar\Time;

/**
 * V*'s append-only state-change discipline, from spec/02
 * §"Status supersession (append-only ledger)".
 *
 * V* ledgers are append-only: a component MUST NOT be mutated after it is
 * appended. A status change is expressed instead as a fresh VJOURNAL
 * "supersession" entry that references the target via `RELATED-TO` and
 * carries `CATEGORIES:status-supersession`, the new effective status, and
 * its own `X-VSTAR-HASH`.
 *
 * Two primitives, and their asymmetry is contract:
 *
 * - {@see self::supersedes()} **constructs** a record and can fail. It
 *   verifies the target's stored hash first, and refuses a target mutated
 *   since it was hashed -- writing a status against content nobody
 *   vouched for is exactly the corruption the ledger model exists to
 *   prevent.
 * - {@see self::superseded()} **queries** and cannot. "Has anything in
 *   this ledger superseded this component?" has two honest answers, and
 *   "no" is not a failure.
 *
 * V* defines only the encoding. Full ledger projection (state-from-log)
 * is the consumer's job per spec/02.
 */
final class Supersession
{
    /**
     * The `CATEGORIES` wire string marking a VJOURNAL as a supersession
     * entry. A consumer projecting a ledger matches on this exact value,
     * case-insensitively, to identify state-change journals.
     */
    public const CATEGORY_STATUS_SUPERSESSION = 'status-supersession';

    /**
     * The property carrying the new status on a supersession VJOURNAL.
     *
     * The constant lives here and stays here -- it is not hoisted to the
     * root namespace. `X-VSTAR-EFFECTIVE-STATUS` is meaningful only
     * inside the supersession pattern, and a root-level export would
     * invite callers to write it onto components directly, which is
     * precisely the mutation the append-only discipline forbids.
     *
     * The value is opaque to V*: spec/02's example shows VTODO statuses,
     * but an application MAY use any vocabulary its domain demands.
     */
    public const PROP_EFFECTIVE_STATUS = 'X-VSTAR-EFFECTIVE-STATUS';

    /**
     * The literal prefix every supersession UID carries. A consumer MAY
     * filter on it to enumerate supersession journals without parsing
     * CATEGORIES, but matching the category is the spec-blessed path.
     */
    private const UID_PREFIX = 'journal:status:';

    /**
     * Construct a fresh supersession VJOURNAL superseding `$target` with
     * `$status` at `$at`. The target is **not** mutated.
     *
     * Properties on the returned component, in order:
     *
     * - `UID` — `journal:status:<target UID>:<form #2 timestamp>`
     * - `DTSTAMP` — `$at` in UTC form #2
     * - `RELATED-TO` — the target's UID
     * - `CATEGORIES` — `status-supersession`
     * - `X-VSTAR-EFFECTIVE-STATUS` — `$status`, verbatim
     * - `X-VSTAR-HASH` — written last, so it covers everything above
     *
     * The integrity check runs first and only when the target carries an
     * `X-VSTAR-HASH`: a target without one makes no integrity claim, so
     * there is nothing to verify and the caller has implicitly opted out.
     *
     * After this call the caller MUST NOT mutate `$target`. In-place
     * mutation breaks the ledger model and invalidates every downstream
     * hash that referenced the pre-mutation form.
     *
     * @throws TargetCorruptedException the target's stored `X-VSTAR-HASH`
     *                                  does not match its canonical form
     */
    public static function supersedes(
        Component $target,
        string $status,
        \DateTimeInterface $at,
    ): Component {
        if (Hashing::getXVstar($target) !== null && !Hashing::verifyXVstar($target)['ok']) {
            throw new TargetCorruptedException(
                'supersession: target component X-VSTAR-HASH does not match canonical form',
            );
        }

        $stamp = Time::formatTime($at);
        $uid = self::UID_PREFIX . $target->uid() . ':' . $stamp;

        $c = new Component(CompType::Journal);
        $c->set(new Property('UID', [], $uid));
        $c->set(new Property('DTSTAMP', [], $stamp));
        $c->set(new Property('RELATED-TO', [], $target->uid()));
        $c->set(new Property('CATEGORIES', [], self::CATEGORY_STATUS_SUPERSESSION));
        $c->set(new Property(self::PROP_EFFECTIVE_STATUS, [], $status));

        // Hash last: setXVstar strips any existing value before computing,
        // so the stored hash covers every property set above.
        Hashing::setXVstar($c);

        return $c;
    }

    /**
     * The effective status `$ledger` projects onto `$c`, or null when
     * nothing supersedes it.
     *
     * A ledger entry supersedes `$c` when it is a VJOURNAL whose
     * `RELATED-TO` matches `$c`'s UID, whose `CATEGORIES` contains
     * `status-supersession`, and which carries an effective status. The
     * latest such entry by parsed `DTSTAMP` wins; on a tie the later
     * ledger position does, since the scan is stable and in order.
     *
     * Null is returned when the ledger is empty, `$c` has no UID to match
     * against, nothing supersedes it, or the matching entries carry no
     * status.
     *
     * Entries whose `DTSTAMP` cannot be parsed sort to the start and are
     * effectively skipped under "latest wins". This is a query, not a
     * validator: malformed ledger data is demoted, never fatal. The hash
     * violation in `corrupt_mutated` belongs to the validate family, which
     * is why that fixture's projection is empty rather than an error.
     *
     * @param list<Component> $ledger
     */
    public static function superseded(Component $c, array $ledger): ?string
    {
        $targetUid = $c->uid();

        if ($targetUid === '') {
            return null;
        }

        $bestAt = null;
        $bestStatus = null;

        foreach ($ledger as $entry) {
            if ($entry->type !== CompType::Journal->value) {
                continue;
            }

            $rel = $entry->get('RELATED-TO');

            if ($rel === null || $rel->value !== $targetUid) {
                continue;
            }

            if (!self::categoriesContainSupersession($entry)) {
                continue;
            }

            $status = $entry->get(self::PROP_EFFECTIVE_STATUS);

            if ($status === null) {
                continue;
            }

            $at = self::entryTimestamp($entry);

            // ">=" rather than ">" so a tie resolves to the later ledger
            // position: the scan runs in order, so the last writer wins.
            if ($bestAt === null || $at >= $bestAt) {
                $bestAt = $at;
                $bestStatus = $status->value;
            }
        }

        return $bestStatus;
    }

    /**
     * Whether `$c` carries a `CATEGORIES` property listing the
     * supersession token.
     *
     * RFC 5545 §3.8.1.2 makes CATEGORIES values comma-delimited, so each
     * token is trimmed and compared whole rather than substring-matched
     * against the raw value. Without that, `status-supersession-deferred`
     * would falsely match.
     */
    private static function categoriesContainSupersession(Component $c): bool
    {
        foreach ($c->getAll('CATEGORIES') as $p) {
            foreach (explode(',', $p->value) as $token) {
                if (strcasecmp(trim($token), self::CATEGORY_STATUS_SUPERSESSION) === 0) {
                    return true;
                }
            }
        }

        return false;
    }

    /**
     * The entry's DTSTAMP as a sortable epoch second, or `PHP_INT_MIN`
     * when it is absent or unparseable -- which sorts it to the start, so
     * it loses every "latest wins" comparison against a real timestamp.
     */
    private static function entryTimestamp(Component $c): int
    {
        $raw = $c->dtstampRaw();

        if ($raw === '') {
            return PHP_INT_MIN;
        }

        $t = Time::parseTime($raw);

        return $t === null ? PHP_INT_MIN : $t->getTimestamp();
    }
}
