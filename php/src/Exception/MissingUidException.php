<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Exception;

/**
 * A component that requires UID has none. Encoder-only in v0.1: the
 * rfc6350 parser accepts a UID-less VCARD, the encoder refuses it.
 */
final class MissingUidException extends VstarException
{
    public function sentinel(): string
    {
        return 'ErrMissingUID';
    }
}
