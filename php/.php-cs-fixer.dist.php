<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

$finder = PhpCsFixer\Finder::create()
    ->in([__DIR__ . '/src', __DIR__ . '/tests'])
    // Rendered by tools/registry/gen.py. Reformatting it here would make
    // `make registry-check` report drift.
    ->exclude('Generated');

return (new PhpCsFixer\Config())
    ->setRiskyAllowed(true)
    ->setRules([
        '@PSR12' => true,
        'declare_strict_types' => true,
        'ordered_imports' => true,
        'no_unused_imports' => true,
    ])
    ->setFinder($finder);
