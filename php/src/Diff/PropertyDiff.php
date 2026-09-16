<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Diff;

use HopTop\Vstar\Property;

/**
 * A single property-level change between two V* values.
 *
 * Which side `$property` holds depends on the operation, and the rule is
 * uniform: `$property` is always the surviving side and `$old` is only
 * populated when there are two sides to compare.
 *
 * - {@see DiffOp::Added} — `$property` is the new property, `$old` null.
 * - {@see DiffOp::Removed} — `$property` is the original property that
 *   went away, `$old` null. (The removed property is the *subject* of the
 *   op, so it occupies the main slot rather than the historical one.)
 * - {@see DiffOp::Changed} — `$property` is the new value and `$old` the
 *   original.
 */
final class PropertyDiff
{
    public function __construct(
        public readonly DiffOp $op,
        public readonly Property $property,
        public readonly ?Property $old = null,
    ) {
    }
}
