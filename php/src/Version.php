<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * Package version.
 *
 * The literal lives in the `VERSION` file at the package root, which is
 * one of the three paths release-please's `php` strategy rewrites
 * natively (alongside `CHANGELOG.md` and `composer.json`) -- so no
 * `extra-files` override is needed to keep this in step with the tag.
 * `composer.json` deliberately carries no `version` key: Packagist
 * derives that from the Git tag.
 */
final class Version
{
    /**
     * The package version as a string.
     */
    public static function get(): string
    {
        $raw = file_get_contents(__DIR__ . '/../VERSION');

        if ($raw === false) {
            throw new \RuntimeException('unable to read the VERSION file');
        }

        return trim($raw);
    }
}
