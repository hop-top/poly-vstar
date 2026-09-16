<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * The wire-string KIND value of a {@see Card}, per RFC 6350 §6.1.4.
 *
 * Values are lowercase, following the RFC's IANA registry. The RFC also
 * lists `location`; only the three values V* v0.1 uses are modeled.
 */
enum Kind: string
{
    case Individual = 'individual';
    case Org = 'org';
    case Group = 'group';
}
