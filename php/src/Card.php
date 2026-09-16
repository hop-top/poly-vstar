<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * A top-level VCARD object per RFC 6350: a UID identifier, a KIND
 * discriminator, and the vCard property list.
 *
 * Structurally analogous to {@see Component} but a distinct type, because
 * vCards do not nest sub-components and carry no CompType.
 *
 * UID and KIND are lifted off the property list on parse and written back
 * on encode, so `$props` never contains a UID, KIND or VERSION property.
 *
 * `$uid` may be the empty string: the rfc6350 parser accepts a UID-less
 * VCARD by design. The **encoder** is what refuses one.
 */
final class Card
{
    /**
     * @param list<Property> $props properties in wire order
     */
    public function __construct(
        public string $uid = '',
        public ?Kind $kind = null,
        public array $props = [],
    ) {
    }

    /**
     * The first property whose name matches, case-insensitively per
     * RFC 6350 §3.3. Null when none match.
     */
    public function get(string $name): ?Property
    {
        foreach ($this->props as $p) {
            if (strcasecmp($p->name, $name) === 0) {
                return $p;
            }
        }

        return null;
    }

    /**
     * Every property whose name matches, case-insensitively, in wire
     * order.
     *
     * @return list<Property>
     */
    public function getAll(string $name): array
    {
        $out = [];

        foreach ($this->props as $p) {
            if (strcasecmp($p->name, $name) === 0) {
                $out[] = $p;
            }
        }

        return $out;
    }

    /**
     * Replace every property matching `$p`'s name (case-insensitively)
     * with a single copy of `$p`, in the position the first match held.
     * Appends when nothing matches.
     */
    public function set(Property $p): void
    {
        $out = [];
        $replaced = false;

        foreach ($this->props as $existing) {
            if (strcasecmp($existing->name, $p->name) === 0) {
                if (!$replaced) {
                    $out[] = $p;
                    $replaced = true;
                }

                continue;
            }

            $out[] = $existing;
        }

        if (!$replaced) {
            $out[] = $p;
        }

        $this->props = $out;
    }

    /**
     * Append a property without touching existing properties of the same
     * name.
     */
    public function add(Property $p): void
    {
        $this->props[] = $p;
    }

    /**
     * Delete every property matching the name, case-insensitively.
     */
    public function remove(string $name): void
    {
        $out = [];

        foreach ($this->props as $p) {
            if (strcasecmp($p->name, $name) === 0) {
                continue;
            }

            $out[] = $p;
        }

        $this->props = $out;
    }
}
