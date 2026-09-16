<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Codec\Stream;

/**
 * An incremental RFC 5545 §3.1 content-line reader over a stream
 * resource.
 *
 * This is the piece that makes the streaming codecs actually stream. The
 * batch scanner takes the whole document as a string and unfolds it in
 * one pass; this one pulls physical lines with `fgets` and holds at most
 * one logical line in memory, so a caller walking a million-component
 * ledger keeps memory flat regardless of input size.
 *
 * # Unfolding happens on bytes
 *
 * A fold is a line terminator followed by SP or HTAB, and the whitespace
 * is part of the fold sequence rather than of the content. Critically,
 * the rejoin operates on **bytes**: RFC 5545 measures the 75-octet limit
 * in octets, so a producer may legitimately fold in the middle of a
 * multi-byte UTF-8 sequence. Decoding each physical line before rejoining
 * would turn that sequence into two replacement characters; rejoining
 * first and decoding after restores the original character. The
 * `rfc5545/fold_split_utf8` fixture exists for exactly this.
 *
 * Blank lines are skipped, CRLF and bare LF are both accepted, and a
 * trailing line with no terminator is surfaced rather than dropped --
 * matching the batch scanner's liberal reading.
 *
 * @internal to the stream namespace
 */
final class LineReader
{
    /**
     * The next logical line held back because it terminated the previous
     * one's fold sequence.
     */
    private ?string $pending = null;

    /** True once the underlying stream is exhausted. */
    private bool $eof = false;

    /**
     * @param resource $stream an open, readable stream resource
     */
    public function __construct(private $stream)
    {
    }

    /**
     * The next logical content line, or null at end of input.
     *
     * Exhaustion is not an error: it is the ordinary end of the stream,
     * and a caller walking to the end hits it exactly once.
     */
    public function next(): ?string
    {
        while (true) {
            $raw = $this->nextPhysical();

            if ($raw === null) {
                // End of stream: flush whatever logical line was still
                // being assembled.
                $line = $this->pending;
                $this->pending = null;

                return $line === '' ? null : $line;
            }

            // A WSP-prefixed line continues the pending one. With nothing
            // pending -- after a blank line broke the sequence -- it
            // starts a fresh line with its leading WSP stripped, which is
            // what keeps such content from being dropped silently.
            if ($raw !== '' && ($raw[0] === ' ' || $raw[0] === "\t")) {
                $this->pending = ($this->pending ?? '') . substr($raw, 1);

                continue;
            }

            $flushed = $this->pending;
            $this->pending = $raw === '' ? null : $raw;

            if ($flushed !== null && $flushed !== '') {
                return $flushed;
            }
        }
    }

    /**
     * One physical line with its terminator stripped, or null at end of
     * input.
     *
     * `fgets` reads up to and including the newline, so the read stays
     * bounded by one line rather than by the document.
     */
    private function nextPhysical(): ?string
    {
        if ($this->eof) {
            return null;
        }

        $raw = fgets($this->stream);

        if ($raw === false) {
            $this->eof = true;

            return null;
        }

        // Strip the terminator: the LF, and the CR of a CRLF. Only one
        // CR -- a lone CR inside a line is content, not a terminator.
        if (str_ends_with($raw, "\n")) {
            $raw = substr($raw, 0, -1);

            if (str_ends_with($raw, "\r")) {
                $raw = substr($raw, 0, -1);
            }
        }

        return $raw;
    }
}
