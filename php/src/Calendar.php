<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * The top-level VCALENDAR container per RFC 5545 §3.4: the PRODID
 * identifying the producing system, plus the contained components.
 *
 * Deliberately small at this layer -- canonicalization, validation and
 * supersession live in their own namespaces.
 */
final class Calendar
{
    /**
     * @param list<Component> $components components in wire order
     */
    public function __construct(
        public string $prodId = '',
        public array $components = [],
    ) {
    }

    /**
     * The first component whose UID property matches, or null.
     *
     * UID comparison is case-**sensitive** per RFC 5545 §3.8.4.7: UIDs
     * are opaque identifiers, not user-facing text, so folding their case
     * would merge two distinct components.
     */
    public function find(string $uid): ?Component
    {
        foreach ($this->components as $c) {
            if ($c->uid() === $uid) {
                return $c;
            }
        }

        return null;
    }

    /**
     * Add a component to the calendar's component list.
     */
    public function append(Component $comp): void
    {
        $this->components[] = $comp;
    }

    /**
     * Every component of the requested type, in wire order.
     *
     * Comparison is case-sensitive: components carry the wire string
     * verbatim and the CompType cases are uppercase per RFC 5545 §3.6.
     *
     * @return list<Component>
     */
    public function filter(CompType|string $t): array
    {
        $wire = $t instanceof CompType ? $t->value : $t;
        $out = [];

        foreach ($this->components as $c) {
            if ($c->type === $wire) {
                $out[] = $c;
            }
        }

        return $out;
    }
}
