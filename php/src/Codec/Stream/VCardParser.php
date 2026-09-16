<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Codec\Stream;

use HopTop\Vstar\Card;
use HopTop\Vstar\Codec\Rfc6350\Parser;
use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\UnclosedBlockException;
use HopTop\Vstar\Exception\UnsupportedVersionException;
use HopTop\Vstar\Kind;
use HopTop\Vstar\Property;

/**
 * The streaming VCARD reader: one card per iteration, at constant memory.
 *
 * A vCard stream is a concatenation of self-contained
 * `BEGIN:VCARD…END:VCARD` blocks with no enclosing wrapper, so there is
 * no header to skip and nothing to carry between blocks.
 *
 * # Unescaping is the vCard's own
 *
 * Content lines route through {@see Parser::parseContentLine()}, the
 * batch vCard decoder -- **not** the iCalendar one. The two formats'
 * TEXT rules differ where an unknown escape is concerned: the vCard side
 * keeps the backslash and the iCalendar side drops it. Borrowing the
 * wrong decoder makes the streamed and batch parses of the same file
 * disagree on a value like `FN:Last\, Comma Test`, which is a real
 * divergence rather than a formatting nicety. The reference implementation
 * has that bug; this port does not replicate it.
 *
 * Not safe for concurrent use: construct one per stream.
 *
 * @implements \IteratorAggregate<int, Card>
 */
final class VCardParser implements \IteratorAggregate
{
    private const SUPPORTED_VERSION = '4.0';

    private LineReader $reader;

    /**
     * @param resource $stream an open, readable stream resource
     */
    public function __construct($stream)
    {
        $this->reader = new LineReader($stream);
    }

    /**
     * Yield each card in document order.
     *
     * Exhaustion ends the iteration normally -- a stream that runs out
     * between blocks is simply finished. A stream that runs out *inside*
     * an open block is a defect and throws.
     *
     * @return \Generator<int, Card>
     *
     * @throws MalformedException          content outside a block, a nested
     *                                     BEGIN, a duplicate or missing VERSION
     * @throws UnsupportedVersionException VERSION is not 4.0
     * @throws UnclosedBlockException      end of input inside an open BEGIN:VCARD
     */
    public function getIterator(): \Generator
    {
        while (true) {
            $opening = $this->nextOpening();

            if ($opening === false) {
                return;
            }

            yield $this->readCard();
        }
    }

    /**
     * Advance to the next `BEGIN:VCARD`, or report end of input.
     *
     * Anything other than a BEGIN line between blocks is content outside
     * a block, which is malformed rather than skippable.
     */
    private function nextOpening(): bool
    {
        $line = $this->reader->next();

        if ($line === null) {
            return false;
        }

        if (strcasecmp($line, 'BEGIN:VCARD') !== 0) {
            throw new MalformedException(sprintf(
                'stream/vcard: expected BEGIN:VCARD, got "%s"',
                $line,
            ));
        }

        return true;
    }

    /**
     * Read one block through to its `END:VCARD`.
     *
     * `UID`, `KIND` and `VERSION` are lifted off the property list onto
     * the model's own fields, matching the batch parser -- so a streamed
     * card and a batch-parsed one are the same value, not merely
     * equivalent ones.
     */
    private function readCard(): Card
    {
        $uid = '';
        $kind = null;
        $version = null;
        $props = [];

        while (true) {
            $line = $this->reader->next();

            if ($line === null) {
                throw new UnclosedBlockException(
                    'stream/vcard: BEGIN:VCARD never closed',
                );
            }

            if (strcasecmp($line, 'END:VCARD') === 0) {
                if ($version === null) {
                    throw new MalformedException('stream/vcard: VCARD missing VERSION');
                }

                if ($version !== self::SUPPORTED_VERSION) {
                    throw new UnsupportedVersionException(sprintf(
                        'stream/vcard: VERSION:%s (only %s supported)',
                        $version,
                        self::SUPPORTED_VERSION,
                    ));
                }

                return new Card($uid, $kind, $props);
            }

            if (strcasecmp($line, 'BEGIN:VCARD') === 0) {
                throw new MalformedException('stream/vcard: nested BEGIN:VCARD');
            }

            $prop = Parser::parseContentLine($line);

            switch (strtoupper(self::bareName($prop->name))) {
                case 'VERSION':
                    if ($version !== null) {
                        throw new MalformedException('stream/vcard: duplicate VERSION');
                    }

                    $version = $prop->value;

                    break;

                case 'UID':
                    $uid = $prop->value;

                    break;

                case 'KIND':
                    $kind = Kind::tryFrom(strtolower($prop->value));

                    break;

                default:
                    $props[] = $prop;
            }
        }
    }

    /**
     * Strip the optional `group.` prefix from a vCard property name per
     * RFC 6350 §3.3, so `home.UID` is recognized as UID.
     *
     * The prefix stays on the property itself -- only this lookup ignores
     * it -- because a round-trip must keep the grouping the producer
     * wrote.
     */
    private static function bareName(string $name): string
    {
        $dot = strpos($name, '.');

        return $dot === false ? $name : substr($name, $dot + 1);
    }
}
