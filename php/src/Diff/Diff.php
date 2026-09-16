<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Diff;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Canonical\Canonical;
use HopTop\Vstar\Card;
use HopTop\Vstar\Component;
use HopTop\Vstar\Hashing\Hashing;
use HopTop\Vstar\Property;
use HopTop\Vstar\Vstar;

/**
 * Semantic equality and structural diff for V* values.
 *
 * "Semantic" means *via canonical form*: two values yielding identical
 * canonical bytes are equal regardless of property order, parameter
 * order, datetime spelling or whitespace. The structural half returns
 * property-level changes suitable for display or programmatic
 * inspection.
 *
 * # Equality rules, delegated to canonicalization
 *
 * - `X-VSTAR-HASH` is excluded from comparison and from diff output on
 *   both sides, matching the canonical layer's spec/03 rule 7. A diff
 *   reports what changed in the content, not the restamped hash that
 *   followed.
 * - Property and parameter ordering are irrelevant.
 * - Datetime forms compare equal when canonicalization resolves them to
 *   the same UTC instant.
 *
 * # Pairing heuristics, and their documented limits
 *
 * - Sub-components pair by (type, UID) when both sides carry a UID.
 *   Those without one -- a VALARM, typically -- pair **positionally** by
 *   index within their type bucket, so reordering UID-less children
 *   surfaces as add + remove rather than a change. That is a known
 *   limitation, not a defect.
 * - Multiple instances of one property name pair in order, the i-th
 *   against the i-th, with the surplus on either side becoming add or
 *   remove. That keeps a multi-valued `ATTENDEE` list intact.
 *
 * # Naming
 *
 * The three boolean functions carry an `Equal` suffix. The reference
 * spells them `diff.Component(a, b)`, where the package qualifier
 * supplies the verb; without it a bare `component(a, b)` returning a
 * boolean is unreadable and collides with `Canonical::component`. The
 * diff entry points keep the reference's `of` prefix for the same
 * reason.
 */
final class Diff
{
    /**
     * The property canonicalization excludes and this layer excludes with
     * it -- see the class doc.
     */
    private const HASH_PROPERTY = Hashing::X_VSTAR_HASH_PROPERTY;

    /**
     * The property-level difference between two components.
     *
     * Both sides are stripped of `X-VSTAR-HASH` first. Properties pair by
     * name case-insensitively and the result is ordered by name; sub
     * components pair by (type, UID), falling back to position.
     */
    public static function ofComponent(Component $a, Component $b): ComponentDiff
    {
        return self::componentDiffAt('', $a, $b);
    }

    /**
     * The property-level difference between two cards.
     *
     * Cards nest nothing, so `$subDiffs` is always empty and `$path` is
     * `''` -- the returned value is usable directly as a rendering root.
     */
    public static function ofCard(Card $a, Card $b): ComponentDiff
    {
        return new ComponentDiff(
            '',
            self::diffProperties(self::filterHash($a->props), self::filterHash($b->props)),
            [],
        );
    }

    /**
     * The per-component difference between two calendars, one entry per
     * component that changed.
     *
     * Components pair by (type, UID) -- the same rule nested
     * sub-components use -- and each pair that differs becomes one entry
     * with path `VCALENDAR.<TYPE>[uid=<uid>]`. A component present on one
     * side only becomes an all-added or all-removed entry. Unchanged
     * components do not appear at all, so two equal calendars yield the
     * empty list rather than a list of empty entries.
     *
     * `PRODID` is deliberately not diffed: a calendar's identity, for V*
     * purposes, is its component set. Use {@see self::calendarEqual()}
     * when PRODID matters.
     *
     * @return list<ComponentDiff>
     */
    public static function ofCalendar(Calendar $a, Calendar $b): array
    {
        $out = [];

        foreach (self::pairSubs($a->components, $b->components) as $pair) {
            $path = self::subPath('VCALENDAR', $pair['label']);
            $entry = self::diffPair($path, $pair['a'], $pair['b']);

            if ($entry !== null) {
                $out[] = $entry;
            }
        }

        return $out;
    }

    /**
     * Whether two components are semantically equal -- their canonical
     * bytes are identical.
     *
     * This routes through full canonical-byte comparison: the design prioritizes
     * correctness over per-property short-circuiting.
     *
     * A component carrying TZID-tagged datetimes has no VTIMEZONE registry
     * of its own, so prefer {@see self::calendarEqual()} for those.
     */
    public static function componentEqual(Component $a, Component $b): bool
    {
        return Canonical::component($a) === Canonical::component($b);
    }

    /**
     * Whether two cards are semantically equal, via canonical bytes.
     */
    public static function cardEqual(Card $a, Card $b): bool
    {
        return Canonical::card($a) === Canonical::card($b);
    }

    /**
     * Whether two calendars are semantically equal, via canonical bytes.
     *
     * This handles top-level component reordering (canonicalization sorts
     * by UID/TZID per rule 6) and resolves TZID-tagged datetimes against
     * the calendar's own VTIMEZONE registry.
     */
    public static function calendarEqual(Calendar $a, Calendar $b): bool
    {
        return Canonical::calendar($a) === Canonical::calendar($b);
    }

    /**
     * The recursive worker. `$path` is the rendered prefix for this level,
     * `''` at the top.
     */
    private static function componentDiffAt(string $path, Component $a, Component $b): ComponentDiff
    {
        return new ComponentDiff(
            $path,
            self::diffProperties(self::filterHash($a->props), self::filterHash($b->props)),
            self::diffSubs($path, $a->sub, $b->sub),
        );
    }

    /**
     * Render one pairing slot, or null when the pair is unchanged.
     *
     * A slot always carries at least one side: {@see self::pairSubs()}
     * emits a slot only for a key or index one of the two inputs
     * supplied, so "both null" is unreachable.
     */
    private static function diffPair(string $path, ?Component $a, ?Component $b): ?ComponentDiff
    {
        if ($a === null) {
            return $b === null ? null : self::wholeComponentDiff($path, $b, DiffOp::Added);
        }

        if ($b === null) {
            return self::wholeComponentDiff($path, $a, DiffOp::Removed);
        }

        $d = self::componentDiffAt($path, $a, $b);

        return $d->isEmpty() ? null : $d;
    }

    /**
     * @param list<Component> $aSub
     * @param list<Component> $bSub
     *
     * @return list<ComponentDiff>
     */
    private static function diffSubs(string $parentPath, array $aSub, array $bSub): array
    {
        $out = [];

        foreach (self::pairSubs($aSub, $bSub) as $pair) {
            $entry = self::diffPair(
                self::subPath($parentPath, $pair['label']),
                $pair['a'],
                $pair['b'],
            );

            if ($entry !== null) {
                $out[] = $entry;
            }
        }

        return $out;
    }

    /**
     * A copy of `$props` with `X-VSTAR-HASH` removed.
     *
     * @param list<Property> $props
     *
     * @return list<Property>
     */
    private static function filterHash(array $props): array
    {
        $out = [];

        foreach ($props as $p) {
            if (strcasecmp($p->name, self::HASH_PROPERTY) === 0) {
                continue;
            }

            $out[] = $p;
        }

        return $out;
    }

    /**
     * Pair properties by name, case-insensitively, and emit the changes
     * ordered by name.
     *
     * Grouping by the upper-cased name is what keeps a multi-valued
     * property intact: all the `ATTENDEE` entries land in one bucket and
     * pair among themselves rather than against some other property.
     *
     * @param list<Property> $a
     * @param list<Property> $b
     *
     * @return list<PropertyDiff>
     */
    private static function diffProperties(array $a, array $b): array
    {
        $ga = self::groupByName($a);
        $gb = self::groupByName($b);

        $keys = array_keys($ga + $gb);
        sort($keys, SORT_STRING);

        $out = [];

        foreach ($keys as $k) {
            foreach (self::diffPropertyGroup($ga[$k] ?? [], $gb[$k] ?? []) as $pd) {
                $out[] = $pd;
            }
        }

        return $out;
    }

    /**
     * @param list<Property> $props
     *
     * @return array<string, list<Property>>
     */
    private static function groupByName(array $props): array
    {
        $out = [];

        foreach ($props as $p) {
            $out[strtoupper($p->name)][] = $p;
        }

        return $out;
    }

    /**
     * Emit the changes for one property name across both sides. Instances
     * pair in order; the surplus on either side becomes add or remove.
     *
     * @param list<Property> $a
     * @param list<Property> $b
     *
     * @return list<PropertyDiff>
     */
    private static function diffPropertyGroup(array $a, array $b): array
    {
        $out = [];
        $n = max(count($a), count($b));

        for ($i = 0; $i < $n; ++$i) {
            if (!isset($a[$i])) {
                $out[] = new PropertyDiff(DiffOp::Added, $b[$i]);

                continue;
            }

            if (!isset($b[$i])) {
                $out[] = new PropertyDiff(DiffOp::Removed, $a[$i]);

                continue;
            }

            if (!Vstar::propertyEqual($a[$i], $b[$i])) {
                $out[] = new PropertyDiff(DiffOp::Changed, $b[$i], $a[$i]);
            }
        }

        return $out;
    }

    /**
     * Pair components by (type, UID), falling back to position within a
     * type bucket for those without a UID.
     *
     * The returned order is the diff's component order, and it is
     * contract:
     *
     * 1. UID-bearing pairs, sorted by type then UID.
     * 2. UID-less buckets in type order, paired by index.
     *
     * @param list<Component> $aSub
     * @param list<Component> $bSub
     *
     * @return list<array{a: ?Component, b: ?Component, label: string}>
     */
    private static function pairSubs(array $aSub, array $bSub): array
    {
        $aByKey = [];
        $bByKey = [];
        $aByType = [];
        $bByType = [];

        self::index($aSub, $aByKey, $aByType);
        self::index($bSub, $bByKey, $bByType);

        $keys = array_keys($aByKey + $bByKey);
        sort($keys, SORT_STRING);

        $out = [];

        foreach ($keys as $key) {
            // The index key is "TYPE\0UID"; NUL cannot occur in either
            // half of a content line, so the split is unambiguous.
            [$type, $uid] = explode("\0", $key, 2);
            $out[] = [
                'a' => $aByKey[$key] ?? null,
                'b' => $bByKey[$key] ?? null,
                'label' => sprintf('%s[uid=%s]', $type, $uid),
            ];
        }

        $types = array_keys($aByType + $bByType);
        sort($types, SORT_STRING);

        foreach ($types as $type) {
            $as = $aByType[$type] ?? [];
            $bs = $bByType[$type] ?? [];

            for ($i = 0, $n = max(count($as), count($bs)); $i < $n; ++$i) {
                $out[] = [
                    'a' => $as[$i] ?? null,
                    'b' => $bs[$i] ?? null,
                    'label' => sprintf('%s[#%d]', $type, $i),
                ];
            }
        }

        return $out;
    }

    /**
     * Split `$subs` into the UID-keyed index and the positional
     * per-type buckets.
     *
     * @param list<Component>              $subs
     * @param array<string, Component>     $byKey
     * @param array<string, list<Component>> $byType
     */
    private static function index(array $subs, array &$byKey, array &$byType): void
    {
        foreach ($subs as $s) {
            $uid = $s->uid();

            if ($uid === '') {
                $byType[$s->type][] = $s;

                continue;
            }

            $byKey[$s->type . "\0" . $uid] = $s;
        }
    }

    /**
     * Render an entire component as added or removed: every property
     * (bar `X-VSTAR-HASH`) becomes one op, ordered by name, and every
     * sub-component recurses the same way.
     */
    private static function wholeComponentDiff(
        string $path,
        Component $c,
        DiffOp $op,
    ): ComponentDiff {
        $props = self::filterHash($c->props);
        usort(
            $props,
            static fn (Property $a, Property $b): int => strcmp(
                strtoupper($a->name),
                strtoupper($b->name),
            ),
        );

        $ops = array_map(
            static fn (Property $p): PropertyDiff => new PropertyDiff($op, $p),
            $props,
        );

        $subs = [];

        foreach ($c->sub as $s) {
            $subs[] = self::wholeComponentDiff(
                self::subPath($path, self::subLabel($s)),
                $s,
                $op,
            );
        }

        return new ComponentDiff($path, $ops, $subs);
    }

    /**
     * The label segment for an unpaired sub-component, mirroring what
     * {@see self::pairSubs()} would have produced for it.
     */
    private static function subLabel(Component $c): string
    {
        $uid = $c->uid();

        return $uid === ''
            ? sprintf('%s[#0]', $c->type)
            : sprintf('%s[uid=%s]', $c->type, $uid);
    }

    private static function subPath(string $parent, string $label): string
    {
        return $parent === '' ? $label : $parent . '.' . $label;
    }
}
