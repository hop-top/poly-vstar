<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Diff;

use HopTop\Vstar\Param;
use HopTop\Vstar\Property;

/**
 * The structural changes between two components, two cards, or one
 * calendar entry pair.
 *
 * `$path` locates the diff site for display, using the same syntax the
 * validate family uses:
 *
 * - `''` for a top-level component or card diff.
 * - `VCALENDAR.VEVENT[uid=…]` for an entry from
 *   {@see Diff::ofCalendar()}.
 * - `<parent>.<TYPE>[uid=…]` or `<parent>.<TYPE>[#<index>]` for a nested
 *   sub-component.
 *
 * `$properties` holds the changes at this level, ordered by property
 * name case-insensitively; `$subDiffs` holds the recursive diffs of
 * sub-components that themselves changed. **Both orders are contract**,
 * not an implementation detail -- a consumer rendering a diff, or a port
 * comparing against the behavior fixtures, depends on them.
 */
final class ComponentDiff implements \Stringable
{
    /**
     * @param list<PropertyDiff>  $properties
     * @param list<ComponentDiff> $subDiffs
     */
    public function __construct(
        public readonly string $path = '',
        public readonly array $properties = [],
        public readonly array $subDiffs = [],
    ) {
    }

    /**
     * Whether this diff records no change at this level nor in any nested
     * sub-component.
     *
     * The reference spells this `Empty()`; every target language spells a
     * predicate with an `is` prefix, so the ports agree on `isEmpty`.
     */
    public function isEmpty(): bool
    {
        if ($this->properties !== []) {
            return false;
        }

        foreach ($this->subDiffs as $sub) {
            if (!$sub->isEmpty()) {
                return false;
            }
        }

        return true;
    }

    /**
     * A unified-diff-ish text block:
     *
     * ```
     * --- <path>
     * + NAME[;PARAM=VAL…]:VALUE
     * - NAME[;PARAM=VAL…]:VALUE
     * ~ NAME: <old> -> <new>
     * ```
     *
     * Sub-component blocks are indented two spaces per level and open
     * with their own header. An empty diff renders as the empty string.
     *
     * The header is a single `--- ` line rather than the unified-diff
     * `---`/`+++` pair, because each block represents the *changes
     * between* two sides, not one side in isolation.
     *
     * This output is informational -- for humans and debug CLIs. It is
     * **not** a wire format and carries no cross-implementation parity
     * guarantee; the fixtures pin the structured shape, not this text.
     *
     * `ComponentDiff` is a class rather than an enum, so the magic method
     * is the correct spelling here; the enums in this library use a plain
     * `toString()` because PHP forbids `__toString()` on an enum.
     */
    public function __toString(): string
    {
        if ($this->isEmpty()) {
            return '';
        }

        $out = '';
        $this->writeTo($out, 0);

        return $out;
    }

    /**
     * Render into `$out` at the given indent depth, two spaces per level.
     */
    private function writeTo(string &$out, int $depth): void
    {
        $indent = str_repeat('  ', $depth);
        $out .= $indent . '--- ' . $this->path . "\n";

        foreach ($this->properties as $pd) {
            $out .= $indent . self::renderPropertyDiff($pd) . "\n";
        }

        foreach ($this->subDiffs as $sub) {
            if (!$sub->isEmpty()) {
                $sub->writeTo($out, $depth + 1);
            }
        }
    }

    /**
     * One property change as a single line, with no terminator.
     */
    private static function renderPropertyDiff(PropertyDiff $pd): string
    {
        return match ($pd->op) {
            DiffOp::Added => '+ ' . self::renderProperty($pd->property),
            DiffOp::Removed => '- ' . self::renderProperty($pd->property),
            // `old` is populated on a change by construction; the
            // nullsafe access keeps a hand-built PropertyDiff from
            // fataling on the rendering path.
            DiffOp::Changed => '~ ' . $pd->property->name . ': '
                . ($pd->old?->value) . ' -> ' . $pd->property->value,
        };
    }

    /**
     * A `NAME[;PARAM=VAL…]:VALUE` wire-style line -- without folding or
     * CRLF, since this is display rather than codec output. Parameters
     * are sorted by name so the rendering is deterministic regardless of
     * the order the producer wrote them in.
     */
    private static function renderProperty(Property $p): string
    {
        $out = $p->name;

        if ($p->params !== []) {
            $params = $p->params;
            usort(
                $params,
                static fn (Param $a, Param $b): int => strcasecmp($a->name, $b->name),
            );

            foreach ($params as $param) {
                $out .= ';' . $param->name . '=' . $param->value;
            }
        }

        return $out . ':' . $p->value;
    }
}
