<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Codec\Stream;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Codec\ContentLine;
use HopTop\Vstar\Codec\Rfc5545\Encoder;
use HopTop\Vstar\Codec\TextValue;
use HopTop\Vstar\Component;
use HopTop\Vstar\Exception\AlreadyClosedException;
use HopTop\Vstar\Exception\HeaderLockedException;

/**
 * The streaming VCALENDAR writer: one component at a time, straight to
 * the sink.
 *
 * The first {@see self::encode()} emits the `BEGIN:VCALENDAR` header and
 * **locks** it; {@see self::close()} emits the `END:VCALENDAR` trailer.
 * The lock is why {@see self::setHeader()} has an error channel at all:
 * once the header is on the wire it cannot be retroactively changed, and
 * silently ignoring a late call would leave the caller believing a PRODID
 * was written that never was.
 *
 * This does not close the underlying stream -- the caller owns its
 * lifecycle.
 *
 * Not safe for concurrent use: construct one per sink.
 */
final class VCalendarEncoder
{
    /**
     * The PRODID emitted when {@see self::setHeader()} was not called. It
     * mirrors the batch encoder's default so batch and stream output for
     * the same logical calendar stay byte-comparable. Version-free and
     * language-free: PRODID survives canonicalization and is hashed, so
     * the default must not change across releases or differ between ports.
     */
    private const DEFAULT_PROD_ID = '-//hop-top//vstar//EN';

    private const SUPPORTED_VERSION = '2.0';

    private Calendar $header;

    private bool $headerWritten = false;

    private bool $closed = false;

    /**
     * @param resource $stream an open, writable stream resource
     */
    public function __construct(private $stream)
    {
        $this->header = new Calendar();
    }

    /**
     * Configure the header emitted on the next {@see self::encode()}.
     *
     * Only `prodId` is consulted: VERSION is fixed at 2.0 by the spec's
     * supported-version contract.
     *
     * @throws HeaderLockedException the first encode already wrote the header
     */
    public function setHeader(Calendar $h): void
    {
        if ($this->headerWritten) {
            throw new HeaderLockedException(
                'stream/vcalendar: header set after first encode',
            );
        }

        $this->header = $h;
    }

    /**
     * Write one component, emitting the header first if it has not gone
     * out yet.
     *
     * @throws AlreadyClosedException called after {@see self::close()}
     */
    public function encode(Component $c): void
    {
        $this->requireOpen();
        $this->writeHeaderOnce();
        $this->write(Encoder::encodeComponent($c));
    }

    /**
     * Emit `END:VCALENDAR` and finish.
     *
     * Safe on an encoder that never saw an encode: the header goes out
     * first, so the output is still a legal empty calendar rather than a
     * bare trailer.
     *
     * @throws AlreadyClosedException called more than once
     */
    public function close(): void
    {
        $this->requireOpen();
        $this->writeHeaderOnce();
        $this->write('END:VCALENDAR' . ContentLine::CRLF);
        $this->closed = true;
    }

    /**
     * @throws AlreadyClosedException the encoder is already closed
     */
    private function requireOpen(): void
    {
        if ($this->closed) {
            throw new AlreadyClosedException('stream/vcalendar: already closed');
        }
    }

    /**
     * Emit `BEGIN:VCALENDAR` + VERSION + PRODID exactly once, and lock the
     * header against further change.
     *
     * The fixed ordering is the reason the lock exists: a consumer reading
     * the stream sees VERSION and PRODID before any component, every time.
     */
    private function writeHeaderOnce(): void
    {
        if ($this->headerWritten) {
            return;
        }

        $this->headerWritten = true;

        $prodId = $this->header->prodId === ''
            ? self::DEFAULT_PROD_ID
            : $this->header->prodId;

        $out = 'BEGIN:VCALENDAR' . ContentLine::CRLF
            . 'VERSION:' . self::SUPPORTED_VERSION . ContentLine::CRLF;

        // PRODID is TEXT-typed, so it takes the same escaping any other
        // TEXT property would, and the same folding.
        ContentLine::fold('PRODID:' . TextValue::escape($prodId), $out);

        $this->write($out);
    }

    private function write(string $bytes): void
    {
        if ($bytes !== '') {
            fwrite($this->stream, $bytes);
        }
    }
}
