<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Codec;

/**
 * The RFC 5545 §3.3.11 / RFC 6350 §3.4 TEXT-value escaping rules, shared
 * by both codecs.
 *
 * Parsing unescapes and encoding re-escapes, symmetrically -- that
 * pairing is what makes the in-memory model hold raw values and
 * parse → encode byte-stable.
 *
 * @internal to the codec namespace
 */
final class TextValue
{
    /**
     * The allow-list of property names whose values are TEXT-typed per
     * RFC 5545 §3.3.11 / §3.7-§3.8 and RFC 6350 §3.4.
     *
     * Only TEXT values are backslash-escaped on emit. URI, INTEGER,
     * DATE-TIME and the other value types pass through verbatim, because
     * escaping a comma inside a URI would corrupt the address.
     *
     * Custom properties -- `X-` extensions and unknown names -- are
     * deliberately **not** TEXT by default. A producer wanting escape
     * semantics for one must land it here.
     *
     * @var array<string, true>
     */
    private const TEXT_PROPERTIES = [
        // RFC 5545 calendar TEXT properties.
        'CATEGORIES' => true,
        'CLASS' => true,
        'COMMENT' => true,
        'CONTACT' => true,
        'DESCRIPTION' => true,
        'LOCATION' => true,
        'PRODID' => true,
        'RELATED-TO' => true,
        'RESOURCES' => true,
        'STATUS' => true,
        'SUMMARY' => true,
        'TRANSP' => true,
        'TZID' => true,
        'TZNAME' => true,
        'UID' => true,
        // RFC 6350 vCard TEXT properties.
        'FN' => true,
        'N' => true,
        'NICKNAME' => true,
        'NOTE' => true,
        'ORG' => true,
        'TITLE' => true,
        'ROLE' => true,
        'KIND' => true,
    ];

    /**
     * Whether the named property is TEXT-typed, matched
     * case-insensitively.
     */
    public static function isTextProperty(string $name): bool
    {
        return isset(self::TEXT_PROPERTIES[strtoupper($name)]);
    }

    /**
     * Apply RFC 5545 §3.3.11 TEXT escaping: `\` becomes `\\`, `,` becomes
     * `\,`, `;` becomes `\;`, and LF becomes `\n`.
     *
     * CR is dropped -- the RFC permits only LF inside TEXT -- so a literal
     * CRLF collapses to a single escaped `\n`.
     *
     * Multi-value TEXT (CATEGORIES, RESOURCES) uses the *unescaped* comma
     * as its element separator. This escapes every comma uniformly, so a
     * producer wanting a comma to survive as a separator must join the
     * elements itself and pass a value that omits the per-element commas.
     *
     * Not idempotent: a string holding a literal backslash gains a second
     * one on a second pass. The encoder calls this exactly once per emit,
     * and the parser's unescape is its exact inverse.
     */
    public static function escape(string $s): string
    {
        if (strpbrk($s, "\\,;\n\r") === false) {
            return $s;
        }

        $out = '';

        for ($i = 0, $n = strlen($s); $i < $n; $i++) {
            $out .= match ($s[$i]) {
                '\\' => '\\\\',
                ',' => '\\,',
                ';' => '\\;',
                "\n" => '\\n',
                "\r" => '',
                default => $s[$i],
            };
        }

        return $out;
    }

    /**
     * Reverse RFC 5545 §3.3.11 TEXT escaping.
     *
     * `\\` is considered before any other pair, so a literal `\\,` is not
     * mis-decoded as an escape. `\n` and `\N` both yield LF.
     *
     * A solitary trailing backslash is preserved verbatim, which keeps
     * the function total and mirrors real-world parser leniency.
     * `$keepUnknownEscapes` selects what an unrecognized two-character
     * escape such as `\x` becomes: the iCalendar side drops the backslash
     * and keeps the character, the vCard side keeps both. The two
     * references differ here, and the difference is observable, so it is a
     * parameter rather than a guess.
     */
    public static function unescape(string $s, bool $keepUnknownEscapes = false): string
    {
        if (!str_contains($s, '\\')) {
            return $s;
        }

        $out = '';
        $n = strlen($s);

        for ($i = 0; $i < $n; $i++) {
            if ($s[$i] !== '\\' || $i + 1 >= $n) {
                $out .= $s[$i];

                continue;
            }

            $next = $s[$i + 1];
            $out .= match ($next) {
                '\\' => '\\',
                ',' => ',',
                ';' => ';',
                'n', 'N' => "\n",
                default => $keepUnknownEscapes ? '\\' . $next : $next,
            };
            $i++;
        }

        return $out;
    }

    /**
     * Wrap a parameter value in DQUOTE when it holds a character that
     * would otherwise be read as a parameter boundary (`,`, `;`, `:`),
     * per RFC 5545 §3.2 / RFC 6350 §3.3.
     *
     * `$stripInnerQuotes` drops any DQUOTE already inside the value: the
     * grammar does not permit one inside a quoted-string, so emitting it
     * would produce a line no parser can read back. The iCalendar encoder
     * strips; the vCard encoder does not, and the difference is
     * observable in the emitted bytes.
     */
    public static function encodeParamValue(string $v, bool $stripInnerQuotes): string
    {
        if ($stripInnerQuotes) {
            $v = str_replace('"', '', $v);
        }

        if (strpbrk($v, ',;:') !== false) {
            return '"' . $v . '"';
        }

        return $v;
    }
}
