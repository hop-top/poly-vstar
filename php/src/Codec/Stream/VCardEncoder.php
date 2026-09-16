<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Codec\Stream;

use HopTop\Vstar\Card;
use HopTop\Vstar\Codec\Rfc6350\Encoder;
use HopTop\Vstar\Exception\AlreadyClosedException;

/**
 * The streaming VCARD writer: one self-contained
 * `BEGIN:VCARD…END:VCARD` block per {@see self::encode()}.
 *
 * # There is deliberately no `setHeader`
 *
 * A VCARD stream has no enclosing wrapper. Each block is complete on its
 * own, so there is no header to set and no trailer to emit -- and the
 * absence is contract, not an oversight. Adding the method for symmetry
 * with {@see VCalendarEncoder} would give callers something to call that
 * could only either do nothing or emit a wrapper the parsers reject.
 *
 * {@see self::close()} exists for lifecycle symmetry: it marks the
 * encoder finished so a later use is reported rather than silently
 * accepted. This does not close the underlying stream -- the caller owns
 * its lifecycle.
 *
 * Not safe for concurrent use: construct one per sink.
 */
final class VCardEncoder
{
    private bool $closed = false;

    /**
     * @param resource $stream an open, writable stream resource
     */
    public function __construct(private $stream)
    {
    }

    /**
     * Write one complete vCard block in canonical wire form.
     *
     * @throws AlreadyClosedException             called after {@see self::close()}
     * @throws \HopTop\Vstar\Exception\MissingUidException the card has no UID
     */
    public function encode(Card $c): void
    {
        $this->requireOpen();

        $bytes = Encoder::encode($c);

        if ($bytes !== '') {
            fwrite($this->stream, $bytes);
        }
    }

    /**
     * Mark the encoder finished.
     *
     * @throws AlreadyClosedException called more than once
     */
    public function close(): void
    {
        $this->requireOpen();
        $this->closed = true;
    }

    /**
     * @throws AlreadyClosedException the encoder is already closed
     */
    private function requireOpen(): void
    {
        if ($this->closed) {
            throw new AlreadyClosedException('stream/vcard: already closed');
        }
    }
}
