<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Ext;

use HopTop\Vstar\Component;
use HopTop\Vstar\Property;

/**
 * Predicates and accessors for the V* `X-*` extension namespace defined
 * in spec/04 (Extension Discipline).
 *
 * V* extensions live in three tiers:
 *
 * - `X-VSTAR-*`    cross-system extensions on a stabilization track.
 * - `X-<SYSTEM>-*` one specific consuming system, e.g. `X-AGR-INTENT`.
 * - `X-EXP-*`      experimental / unstable; no guarantees.
 *
 * The promotion path runs `X-EXP-FOO` → `X-<SYSTEM>-FOO` → `X-VSTAR-FOO`:
 * an experimental property graduates when one system commits to it, and
 * promotes once two independent systems implement compatible semantics.
 * Removing an extension is a breaking change for consumers, so the
 * pattern is promote-then-replace, never rename.
 *
 * PHP has no free functions at namespace scope in the idiomatic style, so
 * the reference's package-level functions become static methods on a
 * final class named for the package.
 */
final class Ext
{
    /**
     * The two reserved slugs. `VSTAR` belongs to {@see Scope::VStar} and
     * `EXP` to {@see Scope::Experimental}, so neither can name an owning
     * system.
     *
     * The reservation covers the **whole** slug segment, not a prefix of
     * it: `X-VSTARLIKE-FOO` is an ordinary system extension owned by
     * `VSTARLIKE`, and the behavior fixture pins that case.
     */
    private const RESERVED_SLUGS = ['VSTAR' => true, 'EXP' => true];

    /**
     * Whether `$name` carries the `X-` prefix per RFC 5545 §3.8.8 /
     * spec/04, case-insensitively -- both `X-FOO` and `x-foo` qualify.
     *
     * The hyphen is required. `X` is a regular IANA-style identifier
     * while `X-` is an (ill-formed) extension, so a bare `X`, a plain
     * name like `DTSTART`, and the empty string are all false. Pair this
     * with {@see self::scopeOf()} to classify a name.
     */
    public static function isExtension(string $name): bool
    {
        if (strlen($name) < 2) {
            return false;
        }

        return ($name[0] === 'X' || $name[0] === 'x') && $name[1] === '-';
    }

    /**
     * Classify `$name` into one of the five {@see Scope} members,
     * case-insensitively.
     *
     * The decision tree:
     *
     * - no `X-` prefix                        → `None`
     * - `X-VSTAR-<NAME>`, `NAME` non-empty    → `VStar`
     * - `X-EXP-<NAME>`, `NAME` non-empty      → `Experimental`
     * - `X-<SYSTEM>-<NAME>`, both non-empty
     *   and `SYSTEM` neither reserved slug    → `System`
     * - anything else carrying the prefix     → `Unknown`
     *
     * To recover the owning system's slug from a `System`-scoped name,
     * use {@see self::systemName()}.
     */
    public static function scopeOf(string $name): Scope
    {
        if (!self::isExtension($name)) {
            return Scope::None;
        }

        $rest = substr($name, 2);

        if ($rest === '') {
            return Scope::Unknown;
        }

        $cut = self::cutSlug($rest);

        // No hyphen at all ("X-FOO"), or a slug with nothing after it
        // ("X-VSTAR-"): the prefix is there but no tier matches.
        if ($cut === null || $cut[1] === '') {
            return Scope::Unknown;
        }

        return match (strtoupper($cut[0])) {
            'VSTAR' => Scope::VStar,
            'EXP' => Scope::Experimental,
            default => Scope::System,
        };
    }

    /**
     * The owning system's slug for an `X-<SYSTEM>-<NAME>` extension,
     * uppercased so callers compare without re-normalizing, or null for
     * any name that is not `System`-scoped.
     *
     * That exclusion is deliberate and covers non-extensions,
     * `X-VSTAR-*` (the V* spec owns it, not a system), `X-EXP-*` (no
     * owner at all), and malformed names lacking the `<SYSTEM>-<NAME>`
     * structure. The question this answers is "which system owns this
     * property?", and only a system-scoped name has an answer.
     */
    public static function systemName(string $name): ?string
    {
        if (!self::isExtension($name)) {
            return null;
        }

        $cut = self::cutSlug(substr($name, 2));

        if ($cut === null) {
            return null;
        }

        [$slug, $suffix] = $cut;

        if ($slug === '' || $suffix === '') {
            return null;
        }

        $upper = strtoupper($slug);

        if (isset(self::RESERVED_SLUGS[$upper])) {
            return null;
        }

        return $upper;
    }

    /**
     * Every property on `$c` whose name classifies into `$scope`, in the
     * component's own property order -- no sort.
     *
     * Passing {@see Scope::None} selects every non-extension property
     * (`UID`, `DTSTART`, …), which is the useful shape for diffing the V*
     * core surface. `System` pairs with {@see self::systemName()} to
     * group by owner, and `Experimental` is what a consumer warns on per
     * spec/04.
     *
     * This does not recurse into `$c->sub`; a caller wanting the whole
     * tree walks the sub-components itself.
     *
     * @return list<Property>
     */
    public static function extensionsByScope(Component $c, Scope $scope): array
    {
        $out = [];

        foreach ($c->props as $p) {
            if (self::scopeOf($p->name) === $scope) {
                $out[] = $p;
            }
        }

        return $out;
    }

    /**
     * Split `SLUG-REST` at the first hyphen, or null when there is none.
     *
     * @return array{string, string}|null
     */
    private static function cutSlug(string $s): ?array
    {
        $i = strpos($s, '-');

        if ($i === false) {
            return null;
        }

        return [substr($s, 0, $i), substr($s, $i + 1)];
    }
}
