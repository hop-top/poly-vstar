<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * A single iCalendar / vCard content line in struct form: a name, zero or
 * more parameters, and a value.
 *
 * The exact wire format is the codec's business -- this layer is pure
 * data. Values here are **raw**: the parser has already reversed RFC 5545
 * §3.3.11 TEXT escaping, and the encoder re-applies it symmetrically.
 *
 * `$params` preserves wire order. Canonicalization sorts them; the codecs
 * do not, so that parse → encode is byte-stable for an already-canonical
 * document.
 */
final class Property
{
    /**
     * @param list<Param> $params parameters in wire order
     */
    public function __construct(
        public readonly string $name,
        public readonly array $params = [],
        public readonly string $value = '',
    ) {
    }

    /**
     * The value of the named parameter, or null when absent. Parameter
     * names are matched case-insensitively per RFC 5545 §3.2.
     */
    public function param(string $name): ?string
    {
        foreach ($this->params as $param) {
            if (strcasecmp($param->name, $name) === 0) {
                return $param->value;
            }
        }

        return null;
    }
}
