<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Codec\Rfc6350;

use HopTop\Vstar\Card;
use HopTop\Vstar\Codec\ContentLine;
use HopTop\Vstar\Codec\TextValue;
use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\UnclosedBlockException;
use HopTop\Vstar\Exception\UnsupportedVersionException;
use HopTop\Vstar\Kind;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;

/**
 * The RFC 6350 vCard 4.0 reader.
 *
 * A vCard stream is a sequence of self-contained `BEGIN:VCARD…END:VCARD`
 * blocks with no enclosing wrapper, so {@see self::parse()} returns a
 * **list** of cards. A file with three cards parses to three Cards; a
 * parser returning a single Card would pass the single-card fixtures and
 * fail the rest.
 *
 * UID handling is asymmetric across the codec layer, and deliberately so:
 * this parser accepts a VCARD with no UID (leaving `Card::$uid` empty),
 * while {@see Encoder::encode()} refuses one. Strict-on-read is the
 * validation layer's job -- an adopter who wants it wraps the parse.
 */
final class Parser
{
    /**
     * The only VERSION value accepted at v0.1. Whether 3.0 should be
     * accepted on read is an open spec question; until it resolves,
     * anything but 4.0 is refused.
     */
    public const SUPPORTED_VERSION = '4.0';

    /**
     * Parse zero or more vCards.
     *
     * Empty input yields an empty list -- "no cards", not an error.
     *
     * @return list<Card>
     *
     * @throws MalformedException          a stray END, a nested BEGIN, a
     *                                     missing or duplicate VERSION, or
     *                                     content outside a VCARD
     * @throws UnclosedBlockException      end of input inside an open VCARD
     * @throws UnsupportedVersionException VERSION present but not 4.0
     */
    public static function parse(string $input): array
    {
        $cards = [];
        $open = false;
        $current = new Card();
        $version = '';
        $lineNumber = 0;

        foreach (ContentLine::lines($input) as $line) {
            $lineNumber++;

            if ($line === '') {
                continue;
            }

            if (strcasecmp($line, 'BEGIN:VCARD') === 0) {
                if ($open) {
                    throw new MalformedException(
                        sprintf('rfc6350: line %d: nested BEGIN:VCARD', $lineNumber),
                    );
                }

                $open = true;
                $current = new Card();
                $version = '';

                continue;
            }

            if (strcasecmp($line, 'END:VCARD') === 0) {
                if (!$open) {
                    throw new MalformedException(
                        sprintf('rfc6350: line %d: stray END:VCARD', $lineNumber),
                    );
                }

                if ($version === '') {
                    throw new MalformedException('rfc6350: VCARD missing VERSION');
                }

                if ($version !== self::SUPPORTED_VERSION) {
                    throw new UnsupportedVersionException(
                        sprintf('rfc6350: VERSION:%s', $version),
                    );
                }

                $cards[] = $current;
                $open = false;
                $current = new Card();
                $version = '';

                continue;
            }

            if (!$open) {
                // Real vCards never carry content outside BEGIN/END, so
                // refusing keeps the parser honest rather than silently
                // discarding data.
                throw new MalformedException(
                    sprintf('rfc6350: line %d: content outside VCARD', $lineNumber),
                );
            }

            $prop = self::parseContentLine($line, $lineNumber);

            // VERSION, UID and KIND are lifted off the property list: they
            // are Card fields, and leaving copies on $props would emit
            // each of them twice.
            if (strcasecmp($prop->name, 'VERSION') === 0) {
                if ($version !== '') {
                    throw new MalformedException(
                        sprintf('rfc6350: line %d: duplicate VERSION', $lineNumber),
                    );
                }

                $version = $prop->value;

                continue;
            }

            if (strcasecmp($prop->name, 'UID') === 0) {
                $current->uid = $prop->value;

                continue;
            }

            if (strcasecmp($prop->name, 'KIND') === 0) {
                // KIND values are lowercase in the RFC 6350 §6.1.4 IANA
                // registry and case-insensitive on read. An unregistered
                // value is kept as a property rather than dropped.
                $kind = Kind::tryFrom(strtolower($prop->value));

                if ($kind !== null) {
                    $current->kind = $kind;

                    continue;
                }
            }

            $current->props[] = $prop;
        }

        if ($open) {
            throw new UnclosedBlockException('rfc6350: BEGIN:VCARD never closed');
        }

        return $cards;
    }

    /**
     * Decompose one already-unfolded vCard content line into a Property,
     * per RFC 6350 §3.3 / §3.4.
     *
     * Format: `[group "."] name *(";" param) ":" value`
     *
     * A group prefix, when present, is preserved verbatim in the property
     * name -- `home.TEL` stays `home.TEL` -- so a round-trip keeps the
     * grouping the producer wrote.
     *
     * @throws MalformedException any structural defect
     */
    public static function parseContentLine(string $line, ?int $lineNumber = null): Property
    {
        $context = $lineNumber === null ? '' : sprintf('line %d: ', $lineNumber);

        if ($line === '') {
            throw new MalformedException("rfc6350: {$context}empty line");
        }

        $colon = self::findFirstUnquoted($line, ':', $context);

        if ($colon < 0) {
            throw new MalformedException("rfc6350: {$context}missing value separator");
        }

        $head = substr($line, 0, $colon);
        $rawValue = substr($line, $colon + 1);

        $nameEnd = self::findFirstUnquoted($head, ';', $context);
        $name = $nameEnd < 0 ? $head : substr($head, 0, $nameEnd);
        $paramText = $nameEnd < 0 ? '' : substr($head, $nameEnd + 1);

        if ($name === '') {
            throw new MalformedException("rfc6350: {$context}empty property name");
        }

        return new Property(
            $name,
            self::parseParams($paramText, $context),
            // The vCard side keeps an unknown escape's backslash, unlike
            // the iCalendar side. The two references differ, and the
            // difference is observable in the parsed value.
            TextValue::unescape($rawValue, true),
        );
    }

    /**
     * Parse the parameter portion of a content-line head: everything
     * between the first `;` after the name and the value separator. Each
     * parameter is `NAME=VALUE`, where VALUE may be DQUOTE-wrapped to
     * embed `,`, `:` or `;`.
     *
     * @return list<Param>
     *
     * @throws MalformedException a parameter without `=`, an empty
     *                            parameter name, or an unterminated quote
     */
    private static function parseParams(string $s, string $context): array
    {
        if ($s === '') {
            return [];
        }

        $params = [];
        $rest = $s;

        while ($rest !== '') {
            $end = self::findFirstUnquoted($rest, ';', $context);

            if ($end < 0) {
                $token = $rest;
                $rest = '';
            } else {
                $token = substr($rest, 0, $end);
                $rest = substr($rest, $end + 1);
            }

            $eq = self::findFirstUnquoted($token, '=', $context);

            if ($eq < 0) {
                throw new MalformedException(
                    sprintf('rfc6350: %sparam "%s" missing "="', $context, $token),
                );
            }

            $name = substr($token, 0, $eq);
            $value = substr($token, $eq + 1);

            if ($name === '') {
                throw new MalformedException("rfc6350: {$context}empty param name");
            }

            if (
                strlen($value) >= 2
                && $value[0] === '"'
                && $value[strlen($value) - 1] === '"'
            ) {
                $value = substr($value, 1, -1);
            }

            $params[] = new Param($name, $value);
        }

        return $params;
    }

    /**
     * The byte index of the first `$target` not inside a DQUOTE-delimited
     * region, or -1 when there is none.
     *
     * @throws MalformedException the string ends inside an open quote
     */
    private static function findFirstUnquoted(string $s, string $target, string $context): int
    {
        $inQuote = false;

        for ($i = 0, $n = strlen($s); $i < $n; $i++) {
            $c = $s[$i];

            if ($c === '"') {
                $inQuote = !$inQuote;

                continue;
            }

            if (!$inQuote && $c === $target) {
                return $i;
            }
        }

        if ($inQuote) {
            throw new MalformedException("rfc6350: {$context}unterminated quoted value");
        }

        return -1;
    }
}
