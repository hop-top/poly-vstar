<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * The wire-string TRANSP value per RFC 5545 §3.8.2.7: whether a VEVENT
 * consumes free/busy time. TRANSP applies to VEVENT only.
 *
 * RFC 5545 §3.8.2.7 assigns OPAQUE when the property is absent; applying
 * that default is a helper's job, not the getter's.
 */
enum Transp: string
{
    /** The event blocks free/busy time. */
    case Opaque = 'OPAQUE';

    /** The event does not block free/busy time. */
    case Transparent = 'TRANSPARENT';
}
