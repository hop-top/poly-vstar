<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests;

use HopTop\Vstar\Version;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;

#[CoversClass(Version::class)]
final class VersionTest extends TestCase
{
    public function testVersionIsSemverShaped(): void
    {
        self::assertMatchesRegularExpression(
            '/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/',
            Version::get(),
        );
    }

    public function testVersionIsTrimmed(): void
    {
        self::assertSame(trim(Version::get()), Version::get());
    }
}
