<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Codec\Rfc5545;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Codec\ContentLine;
use HopTop\Vstar\Codec\TextValue;
use HopTop\Vstar\Component;
use HopTop\Vstar\Property;

/**
 * The RFC 5545 iCalendar writer.
 *
 * Output is a binary string -- PHP strings are byte arrays, which is the
 * right type here. Physical lines are always CRLF-terminated and folded
 * at 75 octets per §3.1, with SP as the continuation lead.
 *
 * Property and component order come from the calendar verbatim; nothing
 * is sorted. The outer VCALENDAR wrapper is always emitted with
 * VERSION:2.0 and a PRODID derived from the calendar's own, so any
 * VERSION or PRODID sitting inside a component is ignored at this layer.
 */
final class Encoder
{
    /**
     * Encode a calendar to RFC 5545 wire bytes.
     */
    public static function encode(Calendar $cal): string
    {
        $out = '';

        ContentLine::fold('BEGIN:VCALENDAR', $out);
        ContentLine::fold('VERSION:' . Parser::SUPPORTED_VERSION, $out);

        // PRODID goes through the content-line path so RFC 5545 §3.3.11
        // TEXT escaping applies to its value -- PRODID is TEXT-typed.
        ContentLine::fold(self::contentLine(new Property('PRODID', [], $cal->prodId)), $out);

        foreach ($cal->components as $c) {
            self::appendComponent($c, $out);
        }

        ContentLine::fold('END:VCALENDAR', $out);

        return $out;
    }

    /**
     * Encode a single component -- its BEGIN/END wrapper, properties and
     * recursive sub-components -- exactly as {@see self::encode()} would
     * produce it inside a VCALENDAR wrapper, but with no wrapper and no
     * VERSION or PRODID of its own.
     *
     * This is the building block canonicalization uses, so that the fold
     * and CRLF logic is shared rather than reimplemented.
     */
    public static function encodeComponent(Component $c): string
    {
        $out = '';
        self::appendComponent($c, $out);

        return $out;
    }

    private static function appendComponent(Component $c, string &$out): void
    {
        $name = strtoupper($c->type);

        ContentLine::fold('BEGIN:' . $name, $out);

        foreach ($c->props as $p) {
            ContentLine::fold(self::contentLine($p), $out);
        }

        foreach ($c->sub as $s) {
            self::appendComponent($s, $out);
        }

        ContentLine::fold('END:' . $name, $out);
    }

    /**
     * Render a property as one unfolded wire content line:
     * `NAME[;PARAM=val[;PARAM=val]]:value`.
     *
     * Property and parameter names are uppercased per RFC 5545 §3.1
     * conventions. Parameter values holding `,`, `;` or `:` are
     * DQUOTE-wrapped per §3.2; an inner DQUOTE is stripped, since the
     * grammar does not permit one inside a quoted-string and emitting it
     * would produce a line no parser can read back.
     *
     * TEXT-typed values are escaped per §3.3.11; every other value type
     * -- URI, INTEGER, DATE-TIME -- emits verbatim, because escaping a
     * comma inside a URI would corrupt the address.
     */
    private static function contentLine(Property $p): string
    {
        $line = strtoupper($p->name);

        foreach ($p->params as $param) {
            $line .= ';' . strtoupper($param->name) . '='
                . TextValue::encodeParamValue($param->value, true);
        }

        $line .= ':';
        $line .= TextValue::isTextProperty($p->name)
            ? TextValue::escape($p->value)
            : $p->value;

        return $line;
    }
}
