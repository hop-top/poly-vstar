<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Validate;

use HopTop\Vstar\Generated\Codes;

/**
 * A single validation finding.
 *
 * `$code` is the stable catalog identifier cataloged in
 * `docs/validate-codes.md`. It is stable across minor versions per
 * semver, so consumers may match on it programmatically.
 *
 * `$message` is human-readable detail and is **not** stable across
 * versions. Match `$code` instead -- the behavior fixtures deliberately
 * omit messages for exactly this reason.
 *
 * `$path` is a dotted component/property locator; see {@see Validate} for
 * the syntax.
 */
final class Diagnostic
{
    /**
     * @param Severity $severity the impact level
     * @param string   $code     the stable catalog identifier
     * @param string   $message  human-readable detail; not stable
     * @param string   $path     the dotted locator
     */
    public function __construct(
        public readonly Severity $severity,
        public readonly string $code,
        public readonly string $message,
        public readonly string $path,
    ) {
    }

    /**
     * Build a diagnostic, taking its severity from the generated registry
     * rather than from the call site.
     *
     * Severity is registry data, not a per-rule decision: a rule that
     * spelled its own severity would be a second source of truth, free to
     * drift the moment `spec/registry/diagnostic-codes.json` changes. The
     * only way to change a severity is to change the registry and re-run
     * the generator.
     *
     * A code the registry does not carry is a programming error, not
     * input, so it throws rather than defaulting to a severity nobody
     * declared.
     */
    public static function of(string $code, string $message, string $path): self
    {
        $severity = Severity::tryFrom(Codes::SEVERITIES[$code] ?? '');

        if ($severity === null) {
            throw new \LogicException("validate: no registry severity for diagnostic code {$code}");
        }

        return new self($severity, $code, $message, $path);
    }
}
