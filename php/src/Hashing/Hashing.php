<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Hashing;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Canonical\Canonical;
use HopTop\Vstar\Card;
use HopTop\Vstar\Component;
use HopTop\Vstar\Property;

/**
 * SHA-256 content hashes of V* objects, in the `sha256:<hex>` form the
 * specification mandates.
 *
 * Hashes are computed over the canonical byte form, so two
 * implementations that agree on canonical bytes produce identical hashes.
 * That is the whole point: the hash is the cheap, transportable proof
 * that two documents are the same logical content, and it is only as good
 * as the byte agreement underneath it.
 *
 * The `sha256:` prefix is part of the value, not decoration. It exists so
 * a future `sha3-256:` or `blake3:` is expressible without ambiguity;
 * v0.1 emits only `sha256:`.
 *
 * # Hash exclusion
 *
 * Rule 7 excludes `X-VSTAR-HASH` from the bytes its own value is computed
 * over -- otherwise the stored hash would feed back into its own digest.
 * The canonical layer strips it per its own contract, and these methods
 * strip it again. The redundancy is deliberate: it documents the
 * invariant at the API boundary, so a reader of this class does not have
 * to go and confirm the canonical layer's behavior.
 *
 * Every method here is pure except {@see self::setXVstar()}, which is the
 * one writer.
 */
final class Hashing
{
    /**
     * The property name V* uses to carry the content hash.
     *
     * The constant lives here and stays here. It is not hoisted to the
     * root namespace even though that is where {@see Property} lives: the
     * string is meaningful only in company with the hash methods that
     * write and read it, and the canonical layer's rule-7 exclusion of
     * that property is a hashing concern. Callers write
     * `Hashing::X_VSTAR_HASH_PROPERTY` in every language.
     */
    public const X_VSTAR_HASH_PROPERTY = 'X-VSTAR-HASH';

    private const SHA256_PREFIX = 'sha256:';

    /**
     * The `sha256:<hex>` digest of the canonical byte form of `$c`.
     *
     * This delegates to the context-free canonical form, not the
     * context-taking one. A component carrying TZID-tagged datetimes
     * therefore hashes over wire-form bytes: two timezone spellings of the
     * same logical instant hash differently. For calendar-aware hashing
     * that resolves TZIDs against a VTIMEZONE registry, hash the whole
     * calendar with {@see self::calendar()}.
     *
     * The asymmetry mirrors the canonical layer's own, and it is correct:
     * a component without a parent calendar has no registry to consult.
     */
    public static function component(Component $c): string
    {
        return self::digest(Canonical::component(self::stripComponent($c)));
    }

    /**
     * The `sha256:<hex>` digest of the canonical byte form of `$cal`.
     *
     * `X-VSTAR-HASH` is stripped at every depth -- top-level components
     * and their sub-components alike -- before the canonical pass.
     * TZID-tagged datetimes resolve against the calendar's own VTIMEZONE
     * registry, so this is the entry point whose result is stable across
     * producers that spell the same instant differently.
     */
    public static function calendar(Calendar $cal): string
    {
        $stripped = [];

        foreach ($cal->components as $c) {
            $stripped[] = self::stripComponent($c);
        }

        return self::digest(Canonical::calendar(new Calendar($cal->prodId, $stripped)));
    }

    /**
     * The `sha256:<hex>` digest of the canonical byte form of `$c`.
     */
    public static function card(Card $c): string
    {
        return self::digest(
            Canonical::card(new Card($c->uid, $c->kind, self::filterOutHash($c->props))),
        );
    }

    /**
     * Compute {@see self::component()} and write the result to `$c` as the
     * `X-VSTAR-HASH` property, replacing any existing value rather than
     * duplicating it.
     *
     * This mutates `$c` -- it is the one method here that does. The hash
     * is computed over the stripped bytes, so calling it repeatedly on the
     * same logical component is idempotent: the second call computes the
     * same hash and rewrites the same value.
     */
    public static function setXVstar(Component $c): void
    {
        $c->set(new Property(self::X_VSTAR_HASH_PROPERTY, [], self::component($c)));
    }

    /**
     * The stored `X-VSTAR-HASH` value, or null when the property is
     * absent.
     *
     * The stored value's format is not validated here. A caller wanting to
     * confirm both shape and freshness uses {@see self::verifyXVstar()}.
     */
    public static function getXVstar(Component $c): ?string
    {
        $p = $c->get(self::X_VSTAR_HASH_PROPERTY);

        return $p === null ? null : $p->value;
    }

    /**
     * Recompute `$c`'s hash and compare it against the stored
     * `X-VSTAR-HASH`.
     *
     * All three fields are reported regardless of the outcome, so a caller
     * can say *what* differed rather than only *that* something did --
     * which is the difference between a usable corruption report and a
     * shrug. `ok` is true only when a hash is stored AND equals the
     * recomputed one exactly; `got` is the empty string when none is
     * stored.
     *
     * @return array{ok: bool, want: string, got: string}
     */
    public static function verifyXVstar(Component $c): array
    {
        $want = self::component($c);
        $got = self::getXVstar($c);

        if ($got === null) {
            return ['ok' => false, 'want' => $want, 'got' => ''];
        }

        return ['ok' => $got === $want, 'want' => $want, 'got' => $got];
    }

    /**
     * `sha256:` plus the lowercase hex digest of `$bytes`.
     */
    private static function digest(string $bytes): string
    {
        return self::SHA256_PREFIX . hash('sha256', $bytes);
    }

    /**
     * A copy of `$c` with `X-VSTAR-HASH` removed from its properties and
     * from every nested sub-component.
     *
     * Building a fresh component rather than editing in place is what
     * keeps the hash methods pure: a caller never observes the strip.
     */
    private static function stripComponent(Component $c): Component
    {
        $sub = [];

        foreach ($c->sub as $s) {
            $sub[] = self::stripComponent($s);
        }

        return new Component($c->type, self::filterOutHash($c->props), $sub);
    }

    /**
     * A copy of `$props` without any `X-VSTAR-HASH` property.
     *
     * @param list<Property> $props
     *
     * @return list<Property>
     */
    private static function filterOutHash(array $props): array
    {
        $out = [];

        foreach ($props as $p) {
            if (strcasecmp($p->name, self::X_VSTAR_HASH_PROPERTY) === 0) {
                continue;
            }

            $out[] = $p;
        }

        return $out;
    }
}
