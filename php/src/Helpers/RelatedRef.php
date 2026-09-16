<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Helpers;

use HopTop\Vstar\RelType;

/**
 * One parsed `RELATED-TO` property: the referenced UID, and the
 * relationship it declares.
 *
 * `$relType` carries a registered value from the RFC 5545 §3.2.15 /
 * RFC 9253 vocabulary folded to its canonical spelling, or an
 * unregistered one -- an `X-` extension, say -- verbatim. The vocabulary
 * is open, so this layer accepts any non-empty value; checking it against
 * the spec's registry is the validate layer's job.
 */
final class RelatedRef
{
    public function __construct(
        public readonly string $uid,
        public readonly RelType $relType,
    ) {
    }
}
