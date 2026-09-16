<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Codec\Rfc5545;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Codec\ContentLine;
use HopTop\Vstar\Codec\TextValue;
use HopTop\Vstar\Component;
use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\UnclosedBlockException;
use HopTop\Vstar\Exception\UnsupportedVersionException;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;

/**
 * The RFC 5545 iCalendar reader.
 *
 * Property order, parameter order and component order are preserved
 * verbatim. No canonicalization happens here: sorting and normalizing is
 * the canonical layer's business, and a parser that sorted would make
 * parse → encode lossy in a way round-trip tests cannot detect.
 */
final class Parser
{
    /**
     * The only iCalendar VERSION value V* honors at v0.1.
     */
    public const SUPPORTED_VERSION = '2.0';

    /**
     * Read a single VCALENDAR.
     *
     * @throws MalformedException          structurally invalid input
     * @throws UnclosedBlockException      a BEGIN with no matching END
     * @throws UnsupportedVersionException VERSION present but not 2.0
     */
    public static function parse(string $input): Calendar
    {
        $lines = ContentLine::lines($input);
        $lines->rewind();

        if (!$lines->valid()) {
            throw new MalformedException('rfc5545: empty input');
        }

        $first = $lines->current();

        if (strcasecmp($first, 'BEGIN:VCALENDAR') !== 0) {
            throw new MalformedException(
                sprintf('rfc5545: expected BEGIN:VCALENDAR, got "%s"', $first),
            );
        }

        $lines->next();
        $root = self::parseBlock($lines, 'VCALENDAR');

        $cal = new Calendar();

        foreach ($root->props as $p) {
            switch (strtoupper($p->name)) {
                case 'VERSION':
                    if ($p->value !== self::SUPPORTED_VERSION) {
                        throw new UnsupportedVersionException(sprintf(
                            'rfc5545: VERSION="%s" (only "%s" supported)',
                            $p->value,
                            self::SUPPORTED_VERSION,
                        ));
                    }

                    break;
                case 'PRODID':
                    $cal->prodId = $p->value;

                    break;
            }
        }

        $cal->components = $root->sub;

        // Trailing content after END:VCALENDAR is ignored on purpose:
        // producers concatenate streams and scanners round up trailing
        // whitespace. The stance is "we got a valid calendar; stop
        // reading" rather than pedantry about what follows.
        return $cal;
    }

    /**
     * Parse a single already-unfolded content line into a Property.
     *
     * The grammar (RFC 5545 §3.1):
     *
     *     contentline = name *(";" param) ":" value
     *     param       = param-name "=" param-value *("," param-value)
     *     param-value = paramtext / quoted-string
     *
     * A DQUOTE-wrapped parameter value may contain commas, semicolons and
     * colons; an unquoted one may not.
     *
     * The wire case of names and parameter names is preserved verbatim.
     * Case-insensitive matching is the consumer's job -- see
     * {@see Component::get()}.
     *
     * @throws MalformedException missing colon, empty name, a parameter
     *                            without `=`, or an unbalanced DQUOTE
     */
    public static function parseContentLine(string $line): Property
    {
        $colon = self::findValueColon($line);
        $head = substr($line, 0, $colon);
        $value = substr($line, $colon + 1);

        $segments = self::splitUnquoted($head, ';');

        if ($segments[0] === '') {
            throw new MalformedException(
                sprintf('rfc5545: empty property name in "%s"', $line),
            );
        }

        $params = [];

        foreach (array_slice($segments, 1) as $segment) {
            $eq = strpos($segment, '=');

            if ($eq === false || $eq === 0) {
                throw new MalformedException(sprintf(
                    'rfc5545: malformed parameter "%s" in "%s"',
                    $segment,
                    $line,
                ));
            }

            $pname = substr($segment, 0, $eq);
            $pvalue = substr($segment, $eq + 1);

            if (
                strlen($pvalue) >= 2
                && $pvalue[0] === '"'
                && $pvalue[strlen($pvalue) - 1] === '"'
            ) {
                $pvalue = substr($pvalue, 1, -1);
            }

            $params[] = new Param($pname, $pvalue);
        }

        // Unescape TEXT-typed values here rather than in parseBlock, so
        // every caller gets it by construction. The streaming VCALENDAR
        // parser calls this method directly and used to skip the
        // unescape step, which made identical bytes decode to different
        // values depending on which parser the caller reached for.
        // Non-TEXT values (URI, INTEGER, DATE-TIME, ...) pass through
        // verbatim: unescaping a URI would corrupt the address.
        if (TextValue::isTextProperty($segments[0])) {
            $value = TextValue::unescape($value);
        }

        return new Property($segments[0], $params, $value);
    }

    /**
     * Consume content lines until `END:<typeName>`, returning the
     * assembled component. Properties accumulate on the parent; a nested
     * BEGIN recurses and appends to `$sub`.
     *
     * @param \Generator<int, string> $lines
     *
     * @throws MalformedException     a mismatched END name
     * @throws UnclosedBlockException end of input before END
     */
    private static function parseBlock(\Generator $lines, string $typeName): Component
    {
        $out = new Component(strtoupper($typeName));

        while (true) {
            if (!$lines->valid()) {
                throw new UnclosedBlockException(
                    sprintf('rfc5545: BEGIN:%s never closed', $typeName),
                );
            }

            $line = $lines->current();
            $lines->next();

            $prop = self::parseContentLine($line);

            switch (strtoupper($prop->name)) {
                case 'BEGIN':
                    $out->sub[] = self::parseBlock($lines, $prop->value);

                    break;
                case 'END':
                    if (strcasecmp($prop->value, $typeName) !== 0) {
                        throw new MalformedException(sprintf(
                            'rfc5545: END:%s does not match BEGIN:%s',
                            $prop->value,
                            $typeName,
                        ));
                    }

                    return $out;
                default:
                    // TEXT values arrive already unescaped:
                    // parseContentLine owns that step so batch and
                    // streaming callers decode identical bytes to an
                    // identical model. Unescaping again here would
                    // collapse a legitimately-doubled backslash.
                    $out->props[] = $prop;
            }
        }
    }

    /**
     * The index of the first `:` lying outside any DQUOTE-wrapped span.
     *
     * @throws MalformedException no such colon, or an unbalanced DQUOTE
     */
    private static function findValueColon(string $line): int
    {
        $inQuote = false;

        for ($i = 0, $n = strlen($line); $i < $n; $i++) {
            if ($line[$i] === '"') {
                $inQuote = !$inQuote;

                continue;
            }

            if ($line[$i] === ':' && !$inQuote) {
                return $i;
            }
        }

        if ($inQuote) {
            throw new MalformedException(
                sprintf('rfc5545: unbalanced quote in "%s"', $line),
            );
        }

        throw new MalformedException(
            sprintf('rfc5545: missing colon in content line "%s"', $line),
        );
    }

    /**
     * Split on every `$sep` lying outside any DQUOTE-wrapped span.
     *
     * @return non-empty-list<string>
     *
     * @throws MalformedException an unbalanced DQUOTE
     */
    private static function splitUnquoted(string $s, string $sep): array
    {
        $out = [];
        $inQuote = false;
        $start = 0;

        for ($i = 0, $n = strlen($s); $i < $n; $i++) {
            if ($s[$i] === '"') {
                $inQuote = !$inQuote;

                continue;
            }

            if ($s[$i] === $sep && !$inQuote) {
                $out[] = substr($s, $start, $i - $start);
                $start = $i + 1;
            }
        }

        if ($inQuote) {
            throw new MalformedException(
                sprintf('rfc5545: unbalanced quote in "%s"', $s),
            );
        }

        $out[] = substr($s, $start);

        return $out;
    }
}
