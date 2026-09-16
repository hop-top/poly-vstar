<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Codec;

/**
 * The RFC 5545 §3.1 content-line unfolding scanner, shared by the
 * iCalendar and vCard codecs.
 *
 * RFC 6350 §3.2 defers to RFC 5545 §3.1 for line folding, so the two
 * formats need byte-identical scanning behavior; one implementation is
 * how that stays true.
 *
 * The scanner is **liberal on input**: CRLF and bare LF are both accepted
 * as physical-line terminators, a trailing partial line with no
 * terminator is surfaced, and blank physical lines are skipped. The
 * encoders are strict in return -- they always emit CRLF.
 *
 * @internal to the codec namespace; callers outside it consume content
 *           lines through the public `Rfc5545\Scanner` re-export
 */
final class ContentLine
{
    /**
     * Unfold `$input` into its logical content lines, in input order.
     *
     * A fold is a line terminator followed by a SP or HTAB at the start
     * of the next physical line; RFC 5545 §3.1 makes the whitespace part
     * of the fold sequence, so it does **not** appear in the logical
     * line.
     *
     * A WSP-prefixed line with no pending logical line to extend -- after
     * a blank line broke the fold sequence, say -- starts a fresh logical
     * line with its leading WSP stripped. Without that leniency, an input
     * like `"DESCRIPTION:start\r\n\r\n more\r\n"` would either yield a
     * stray leading space or drop the content silently.
     *
     * @return \Generator<int, string>
     */
    public static function lines(string $input): \Generator
    {
        $pending = null;

        foreach (self::physicalLines($input) as $raw) {
            if ($raw !== '' && ($raw[0] === ' ' || $raw[0] === "\t")) {
                // Continuation of the pending line, or -- with nothing
                // pending -- the start of a fresh one.
                $pending = ($pending ?? '') . substr($raw, 1);

                continue;
            }

            // A new logical line starts here; flush any pending one first.
            if ($pending !== null && $pending !== '') {
                yield $pending;
            }

            $pending = $raw === '' ? null : $raw;
        }

        if ($pending !== null && $pending !== '') {
            yield $pending;
        }
    }

    /**
     * Split `$input` into physical lines, accepting CRLF or bare LF and
     * surfacing a terminator-less trailing line.
     *
     * @return \Generator<int, string>
     */
    private static function physicalLines(string $input): \Generator
    {
        if ($input === '') {
            return;
        }

        $offset = 0;
        $length = strlen($input);

        while ($offset < $length) {
            $lf = strpos($input, "\n", $offset);

            if ($lf === false) {
                // Trailing partial line: no terminator before end of
                // input. Surface it rather than dropping it.
                yield rtrim(substr($input, $offset), "\r");

                return;
            }

            $line = substr($input, $offset, $lf - $offset);
            $offset = $lf + 1;

            // Strip the optional CR of a CRLF. Only one: a lone CR inside
            // a line is content, not a terminator.
            if ($line !== '' && $line[strlen($line) - 1] === "\r") {
                $line = substr($line, 0, -1);
            }

            yield $line;
        }
    }

    /**
     * Fold one assembled logical line to physical lines of at most 75
     * octets each, terminate every one with CRLF, and append the result
     * to `$out`.
     *
     * The width is measured in **octets** -- `strlen`, PHP's byte count,
     * never `mb_strlen`, which counts characters and would let a line
     * carrying non-ASCII text run past the limit on the wire. RFC 5545
     * §3.1 says octets, and the canonical bytes (and therefore the hash)
     * depend on the fold points landing exactly where the reference puts
     * them.
     *
     * Continuation lines carry a leading SP, which counts toward the
     * limit -- so the first physical line takes 75 payload octets and
     * every continuation takes 74.
     *
     * Folding runs **after** property assembly, never before: the input
     * here is the complete logical line, name and parameters and value
     * with escaping already applied. Folding a value before appending its
     * parameters puts the fold points elsewhere, and since the result
     * unfolds to the same logical line, no round-trip test can see it --
     * only a byte comparison can.
     */
    public static function fold(string $line, string &$out): void
    {
        $limit = self::FOLD_OCTETS;

        if (strlen($line) <= $limit) {
            $out .= $line . self::CRLF;

            return;
        }

        $out .= substr($line, 0, $limit) . self::CRLF;
        $rest = substr($line, $limit);

        // The leading SP costs one octet, leaving 74 for payload.
        $continuation = $limit - 1;

        while ($rest !== '') {
            $chunk = substr($rest, 0, $continuation);
            $out .= ' ' . $chunk . self::CRLF;
            $rest = substr($rest, strlen($chunk));
        }
    }

    /**
     * The RFC 5545 §3.1 fold limit: a physical line, excluding its CRLF
     * terminator, must not exceed 75 octets.
     */
    public const FOLD_OCTETS = 75;

    /**
     * The wire-format physical-line terminator. RFC 5545 §3.1 requires
     * CRLF on output even though the parser accepts bare LF on input.
     */
    public const CRLF = "\r\n";
}
