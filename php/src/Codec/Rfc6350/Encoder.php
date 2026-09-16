<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Codec\Rfc6350;

use HopTop\Vstar\Card;
use HopTop\Vstar\Codec\ContentLine;
use HopTop\Vstar\Codec\TextValue;
use HopTop\Vstar\Exception\MissingUidException;
use HopTop\Vstar\Property;

/**
 * The RFC 6350 vCard 4.0 writer.
 *
 * Asymmetric with the parser by design: {@see Parser::parse()} returns a
 * list, this takes exactly **one** Card. Encoding a list means calling
 * this once per card and concatenating, which is what the stream encoder
 * does.
 *
 * Output is a binary string, CRLF-terminated, folded at 75 octets.
 */
final class Encoder
{
    /**
     * Encode one Card as a `BEGIN:VCARD…END:VCARD` block.
     *
     * Property emission order is fixed, so the output is byte-stable:
     * VERSION, then UID, then KIND when set, then the card's own
     * properties in their input order.
     *
     * A bare property name is uppercased per RFC 6350 §3.3; a group
     * prefix keeps its original case, so `home.TEL` round-trips.
     *
     * @throws MissingUidException the card has no UID
     */
    public static function encode(Card $card): string
    {
        // UID is required on emit. This is the *only* place the
        // codec enforces it: the parser accepts a UID-less VCARD, so a
        // port that implements the check on the parse side alone passes
        // the malformed fixture while being wrong on both counts --
        // rejecting input it should accept, and emitting output it should
        // refuse.
        if ($card->uid === '') {
            throw new MissingUidException('rfc6350: encode: card has no UID');
        }

        $out = '';

        ContentLine::fold('BEGIN:VCARD', $out);
        ContentLine::fold('VERSION:' . Parser::SUPPORTED_VERSION, $out);
        ContentLine::fold('UID:' . TextValue::escape($card->uid), $out);

        if ($card->kind !== null) {
            ContentLine::fold('KIND:' . $card->kind->value, $out);
        }

        foreach ($card->props as $p) {
            ContentLine::fold(self::contentLine($p), $out);
        }

        ContentLine::fold('END:VCARD', $out);

        return $out;
    }

    /**
     * Render one property as its unfolded wire form, without the trailing
     * CRLF -- folding adds that.
     *
     * Unlike the iCalendar encoder, this escapes **every** value rather
     * than consulting a TEXT allow-list: RFC 6350 §3.4 makes TEXT the
     * default value type for vCard properties, so the allow-list would
     * have to name nearly all of them. The reference does the same, and
     * the difference is visible in the emitted bytes for a fixture whose
     * N property carries unescaped semicolons.
     */
    private static function contentLine(Property $p): string
    {
        [$group, $name] = self::splitGroup($p->name);

        $line = $group === '' ? '' : $group . '.';
        $line .= strtoupper($name);

        foreach ($p->params as $param) {
            // The vCard encoder does not strip an inner DQUOTE, where the
            // iCalendar one does.
            $line .= ';' . strtoupper($param->name) . '='
                . TextValue::encodeParamValue($param->value, false);
        }

        return $line . ':' . TextValue::escape($p->value);
    }

    /**
     * Separate a `group.NAME` identifier into its group and bare-name
     * pieces. With no `.`, the group is empty and the name is the whole
     * input.
     *
     * @return array{string, string}
     */
    private static function splitGroup(string $s): array
    {
        $dot = strpos($s, '.');

        if ($dot === false) {
            return ['', $s];
        }

        return [substr($s, 0, $dot), substr($s, $dot + 1)];
    }
}
