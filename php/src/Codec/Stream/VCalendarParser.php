<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Codec\Stream;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Codec\Rfc5545\Parser;
use HopTop\Vstar\Component;
use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\UnclosedBlockException;
use HopTop\Vstar\Exception\UnsupportedVersionException;

/**
 * The streaming VCALENDAR reader: one top-level component per iteration,
 * at constant memory.
 *
 * The header is consumed on first use -- `BEGIN:VCALENDAR` plus the
 * calendar-level properties that precede the first sub-component -- and
 * is available through {@see self::header()} from then on, including
 * before the first component is pulled.
 *
 * Exhaustion is not an error. `END:VCALENDAR` ends the iteration
 * normally; a real defect throws, carrying the reference sentinel.
 *
 * Not safe for concurrent use: construct one per stream.
 *
 * @implements \IteratorAggregate<int, Component>
 */
final class VCalendarParser implements \IteratorAggregate
{
    private const KW_BEGIN = 'BEGIN';
    private const KW_END = 'END';
    private const SUPPORTED_VERSION = '2.0';

    private LineReader $reader;

    private bool $headerRead = false;

    private Calendar $headerCalendar;

    /**
     * A line pulled while detecting the end of the header section, held
     * back for the first component read.
     */
    private ?string $pending = null;

    private bool $done = false;

    /**
     * @param resource $stream an open, readable stream resource
     */
    public function __construct($stream)
    {
        $this->reader = new LineReader($stream);
        $this->headerCalendar = new Calendar();
    }

    /**
     * The calendar-level properties captured from the header.
     *
     * `components` is always empty -- only the VCALENDAR-level metadata
     * populates. Safe to call before the first iteration, in which case
     * the header is read on demand.
     *
     * @throws MalformedException           the input does not open with BEGIN:VCALENDAR
     * @throws UnsupportedVersionException  VERSION is present and is not 2.0
     * @throws UnclosedBlockException       the input ends inside the header
     */
    public function header(): Calendar
    {
        $this->ensureHeader();

        return $this->headerCalendar;
    }

    /**
     * Yield each top-level component in document order.
     *
     * Laziness is the contract: the first component is yielded as soon as
     * its `END:` line is read, without touching the bytes that follow, so
     * a consumer can stop early and a large ledger never lands in memory
     * whole.
     *
     * @return \Generator<int, Component>
     *
     * @throws MalformedException          a structural defect
     * @throws UnclosedBlockException      a BEGIN without its matching END
     * @throws UnsupportedVersionException VERSION is present and is not 2.0
     */
    public function getIterator(): \Generator
    {
        $this->ensureHeader();

        while (!$this->done) {
            $line = $this->nextLine();

            if ($line === null) {
                throw new UnclosedBlockException(
                    'stream/vcalendar: BEGIN:VCALENDAR never closed',
                );
            }

            $prop = Parser::parseContentLine($line);
            $upper = strtoupper($prop->name);

            if ($upper === self::KW_END) {
                if (strcasecmp($prop->value, 'VCALENDAR') !== 0) {
                    throw new MalformedException(sprintf(
                        'stream/vcalendar: END:%s does not match BEGIN:VCALENDAR',
                        $prop->value,
                    ));
                }

                $this->done = true;

                return;
            }

            if ($upper === self::KW_BEGIN) {
                yield $this->readBlock(strtoupper($prop->value));

                continue;
            }

            // RFC 5545 allows METHOD and X-* anywhere inside VCALENDAR,
            // but a calendar-level property appearing *after* components
            // have started means the producer interleaved header metadata
            // with content. Rejected rather than silently reordered: the
            // header is already on the wire and cannot absorb it.
            throw new MalformedException(sprintf(
                'stream/vcalendar: unexpected calendar-level property "%s" after components',
                $prop->name,
            ));
        }
    }

    /**
     * Consume `BEGIN:VCALENDAR` and the calendar-level properties up to
     * the first `BEGIN:` or `END:`, stashing that boundary line.
     *
     * Runs exactly once; the flag is set before the read so a failed
     * attempt cannot loop.
     */
    private function ensureHeader(): void
    {
        if ($this->headerRead) {
            return;
        }

        $this->headerRead = true;

        $first = $this->reader->next();

        if ($first === null) {
            throw new MalformedException('stream/vcalendar: empty input');
        }

        if (strcasecmp($first, 'BEGIN:VCALENDAR') !== 0) {
            throw new MalformedException(sprintf(
                'stream/vcalendar: expected BEGIN:VCALENDAR, got "%s"',
                $first,
            ));
        }

        while (true) {
            $line = $this->reader->next();

            if ($line === null) {
                throw new UnclosedBlockException(
                    'stream/vcalendar: BEGIN:VCALENDAR never closed',
                );
            }

            $prop = Parser::parseContentLine($line);
            $upper = strtoupper($prop->name);

            if ($upper === self::KW_BEGIN || $upper === self::KW_END) {
                $this->pending = $line;

                return;
            }

            if ($upper === 'VERSION') {
                if ($prop->value !== self::SUPPORTED_VERSION) {
                    throw new UnsupportedVersionException(sprintf(
                        'stream/vcalendar: VERSION:%s (only %s supported)',
                        $prop->value,
                        self::SUPPORTED_VERSION,
                    ));
                }

                continue;
            }

            if ($upper === 'PRODID') {
                $this->headerCalendar->prodId = $prop->value;
            }

            // METHOD and X-* are read past: the Calendar model has no
            // field for them, and inventing one here would put
            // this parser ahead of the batch codec's surface.
        }
    }

    /**
     * The next content line, taking the stashed boundary line first.
     */
    private function nextLine(): ?string
    {
        if ($this->pending !== null) {
            $line = $this->pending;
            $this->pending = null;

            return $line;
        }

        return $this->reader->next();
    }

    /**
     * Parse a `BEGIN:$typeName … END:$typeName` subtree, starting from the
     * line after the BEGIN.
     *
     * The type is the wire string verbatim, so `STANDARD` and `DAYLIGHT`
     * inside a VTIMEZONE survive intact -- neither is one of the named
     * component types, and a parser that narrowed the vocabulary here
     * would silently corrupt every timezone it read.
     */
    private function readBlock(string $typeName): Component
    {
        $props = [];
        $subs = [];

        while (true) {
            $line = $this->reader->next();

            if ($line === null) {
                throw new UnclosedBlockException(sprintf(
                    'stream/vcalendar: BEGIN:%s never closed',
                    $typeName,
                ));
            }

            $prop = Parser::parseContentLine($line);
            $upper = strtoupper($prop->name);

            if ($upper === self::KW_BEGIN) {
                $subs[] = $this->readBlock(strtoupper($prop->value));

                continue;
            }

            if ($upper === self::KW_END) {
                if (strcasecmp($prop->value, $typeName) !== 0) {
                    throw new MalformedException(sprintf(
                        'stream/vcalendar: END:%s does not match BEGIN:%s',
                        $prop->value,
                        $typeName,
                    ));
                }

                return new Component($typeName, $props, $subs);
            }

            $props[] = $prop;
        }
    }
}
