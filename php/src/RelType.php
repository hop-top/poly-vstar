<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * The wire-string RELTYPE parameter value on a RELATED-TO property
 * (RFC 5545 §3.2.15, extended by RFC 9253 §4 and §5; the registry is
 * RFC 9253 §11.4).
 *
 * The type is deliberately **open**, which is why it is a value class
 * rather than an enum: IANA may register further values and RFC 5545
 * permits `X-`-prefixed extensions, so any string is a valid RelType. The
 * constants below name the registered vocabulary; they do not bound it.
 * A closed enum here would reject a conformant document.
 *
 * RELTYPE values are compared case-insensitively per RFC 5545 §3.2 --
 * use {@see self::parse()} to fold wire input onto a registered constant,
 * or {@see self::equalFold()} to compare without folding first.
 */
final class RelType implements \Stringable
{
    /** RFC 5545 §3.2.15 hierarchical relationship types. */
    public const PARENT = 'PARENT';
    public const CHILD = 'CHILD';
    public const SIBLING = 'SIBLING';

    /**
     * RFC 9253 §4 temporal relationship types. The edge lives on the
     * predecessor and points at the successor: FINISHTOSTART means the
     * referenced component cannot start until the referencing one
     * finishes, and the other three read the same way.
     */
    public const FINISHTOSTART = 'FINISHTOSTART';
    public const FINISHTOFINISH = 'FINISHTOFINISH';
    public const STARTTOFINISH = 'STARTTOFINISH';
    public const STARTTOSTART = 'STARTTOSTART';

    /**
     * RFC 9253 §5 relationship types. DEPENDS-ON: the referencing
     * component depends on the referenced one. FIRST and NEXT order a
     * chain. CONCEPT and REFID reference every component whose CONCEPT or
     * REFID property matches the RELATED-TO value.
     */
    public const DEPENDS_ON = 'DEPENDS-ON';
    public const FIRST = 'FIRST';
    public const NEXT = 'NEXT';
    public const CONCEPT = 'CONCEPT';
    public const REFID = 'REFID';

    /**
     * The registered vocabulary, keyed by canonical wire string, for
     * {@see self::parse()}'s case-folded lookup.
     *
     * @var list<string>
     */
    private const REGISTERED = [
        self::PARENT,
        self::CHILD,
        self::SIBLING,
        self::FINISHTOSTART,
        self::FINISHTOFINISH,
        self::STARTTOFINISH,
        self::STARTTOSTART,
        self::DEPENDS_ON,
        self::FIRST,
        self::NEXT,
        self::CONCEPT,
        self::REFID,
    ];

    public function __construct(
        /** The wire value, verbatim. */
        public readonly string $value,
    ) {
    }

    /**
     * The value RFC 5545 §3.2.15 assigns when RELTYPE is omitted from a
     * RELATED-TO property.
     *
     * Spelled as a method rather than a constant because PHP constants
     * cannot hold an object; `RelType::DEFAULT` names the wire string.
     */
    public const DEFAULT = self::PARENT;

    /**
     * Fold a wire RELTYPE value onto a registered constant,
     * case-insensitively per RFC 5545 §3.2.
     *
     * The boolean is **not** an optional. It reports "this named a
     * registered value", and the RelType is meaningful either way: an
     * empty input yields PARENT with `true`, matching the RFC 5545
     * §3.2.15 rule that an omitted RELTYPE means PARENT; an unregistered
     * value -- an `X-` extension, or one from a later IANA registration
     * -- comes back verbatim with `false`, so a caller that accepts
     * extensions keeps the original spelling.
     *
     * @return array{RelType, bool}
     */
    public static function parse(string $s): array
    {
        if ($s === '') {
            return [new self(self::DEFAULT), true];
        }

        $upper = strtoupper($s);

        foreach (self::REGISTERED as $registered) {
            if ($upper === $registered) {
                return [new self($registered), true];
            }
        }

        return [new self($s), false];
    }

    /**
     * Whether this and `$s` name the same RELTYPE, compared
     * case-insensitively per RFC 5545 §3.2.
     */
    public function equalFold(string $s): bool
    {
        return strcasecmp($this->value, $s) === 0;
    }

    public function __toString(): string
    {
        return $this->value;
    }
}
