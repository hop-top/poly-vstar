<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Component;
use HopTop\Vstar\Generated\Codes;

/**
 * The V* semantic invariants the codec layer cannot catch.
 *
 * A document can be syntactically valid RFC 5545 or RFC 6350, so that
 * parsing succeeds, and still violate V* discipline: a missing or
 * corrupted `X-VSTAR-HASH`, a property outside the extension namespace, a
 * type-specific required property absent, a `STATUS` from the wrong
 * vocabulary, a malformed recurrence or duration.
 *
 * The two entry points are {@see self::validate()} (a whole
 * {@see Calendar}) and {@see self::validateComponent()} (one
 * {@see Component}). **Neither throws.** Validation *is* the error
 * channel: a document that fails every check still validates successfully
 * and returns a list of findings. An empty list means clean. A port that
 * threw on a diagnostic would have inverted the API.
 *
 * Every {@see Diagnostic} carries a stable code -- cataloged in
 * `docs/validate-codes.md` and generated from `spec/registry/` -- that
 * consumers may match programmatically. The message is human-readable and
 * may change in any release; do not match on it.
 *
 * Coverage of spec/05, the conformance criteria:
 *
 * - §1 required common properties (UID, DTSTAMP, X-VSTAR-HASH).
 * - §2 X-VSTAR-HASH integrity -- the present-but-wrong case; an absent
 *   hash is reported by §1 instead.
 * - §3 extension namespace compliance -- a non-standard property name
 *   without the `X-` prefix.
 * - §4 supersession discipline -- a supersession VJOURNAL's required
 *   properties, and whether its `RELATED-TO` resolves.
 * - §5 type-specific required properties -- VTODO, VEVENT, VFREEBUSY,
 *   VCARD.
 * - §6 RRULE conformance -- unsupported (warning) and malformed (error).
 * - §7 DURATION well-formedness -- the DURATION property, the relative
 *   form of TRIGGER, and the REPEAT count.
 * - §8 the STATUS value domain. The other domains §8 bounds (CLASS,
 *   TRANSP, PRIORITY, PERCENT-COMPLETE, SEQUENCE, REPEAT) have no code
 *   yet; the `helpers` accessors enforce them at write time.
 *
 * **Path syntax.** A diagnostic's path is a dotted component/property
 * locator:
 *
 * | Path | Meaning |
 * |---|---|
 * | `VCALENDAR` | Calendar-level. |
 * | `VCALENDAR.VTODO[uid=foo]` | Component-level, on the VTODO whose UID is `foo`. |
 * | `VCALENDAR.VTODO[uid=foo].DTSTAMP` | Property-level, on that VTODO's DTSTAMP. |
 * | `VCALENDAR.VTODO[#3]` | A UID-less VTODO, at positional index 3. |
 * | `VTODO[uid=foo].DTSTAMP` | {@see self::validateComponent()} -- no calendar prefix. |
 */
final class Validate
{
    /**
     * Check every component in `$cal` and return the accumulated
     * diagnostics. An empty list means `$cal` is clean.
     *
     * `$cal` is not mutated. Diagnostics follow component order, then rule
     * order within a component; consumers that compare against a fixture
     * should sort, since only the set -- not the emission order -- is
     * contractual.
     *
     * Prefer this over {@see self::validateComponent()} whenever a
     * calendar exists: the paths are more precise, and the cross-component
     * orphan-supersession rule can only run here.
     *
     * @return list<Diagnostic>
     */
    public static function validate(Calendar $cal): array
    {
        $out = [];
        $index = [];

        foreach ($cal->components as $comp) {
            $path = 'VCALENDAR.' . Internal::componentPath($comp, $index);

            foreach (self::run($comp, $path, $cal->components) as $d) {
                $out[] = $d;
            }
        }

        return $out;
    }

    /**
     * Check a single {@see Component} in isolation. Paths start at the
     * component itself, e.g. `VTODO[uid=foo].DTSTAMP`.
     *
     * This is the right entry point when there is no parent calendar -- a
     * freshly minted component, say, checked before it is appended. The
     * cross-component orphan-supersession rule is skipped, not guessed:
     * with no ledger to resolve against, "this target does not exist" is a
     * claim this entry point cannot make.
     *
     * @return list<Diagnostic>
     */
    public static function validateComponent(Component $c): array
    {
        $index = [];

        return self::run($c, Internal::componentPath($c, $index), null);
    }

    /**
     * Every registry diagnostic code, sorted.
     *
     * @return list<string>
     */
    public static function codes(): array
    {
        $codes = array_keys(Codes::SEVERITIES);
        sort($codes);

        return $codes;
    }

    /**
     * The severity the registry assigns `$code`, or null when the string
     * names no known code.
     *
     * Consumers must tolerate unknown codes -- new ones may appear in any
     * release -- so this reports absence rather than throwing.
     */
    public static function severityOf(string $code): ?Severity
    {
        return Severity::tryFrom(Codes::SEVERITIES[$code] ?? '');
    }

    /**
     * How many properties the generated RFC 5545/6350 allow-list carries.
     */
    public static function standardPropertyCount(): int
    {
        return Extensions::standardPropertyCount();
    }

    /**
     * Run every rule against `$c` at `$path`.
     *
     * `$ledger` supplies the cross-component context the
     * orphan-supersession rule needs; null means there is none and that
     * rule is skipped.
     *
     * @param ?list<Component> $ledger
     *
     * @return list<Diagnostic>
     */
    private static function run(Component $c, string $path, ?array $ledger): array
    {
        return array_merge(
            Required::check($c, $path),
            Integrity::check($c, $path),
            Extensions::check($c, $path),
            Types::check($c, $path),
            Status::check($c, $path),
            Supersession::check($c, $ledger, $path),
            Rrule::check($c, $path),
            Duration::check($c, $path),
        );
    }
}
