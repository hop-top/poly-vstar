<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Ext;

use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Ext\Ext;
use HopTop\Vstar\Ext\Scope;
use HopTop\Vstar\Property;
use HopTop\Vstar\Tests\Behavior;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * The `ext/scopes.json` behavior family: how every extension name
 * classifies under spec/04, and which system owns it.
 *
 * The fixture's `scope` tokens are lowercase wire strings, which is what
 * {@see Scope} is backed by. The reference's capitalized display
 * spelling is a separate concern, reachable through
 * {@see Scope::toString()} -- an enum cannot declare `__toString()` in
 * PHP, so the mapping assigns a plain method here.
 */
final class ScopesTest extends TestCase
{
    /**
     * Every row of the flat classification table.
     *
     * @return iterable<string, array{string, string, ?string}>
     */
    public static function scopeCases(): iterable
    {
        foreach (Behavior::cases('ext/scopes.json') as $i => $case) {
            $name = $case['name'];
            $scope = $case['scope'];
            $system = $case['system'];

            self::assertIsString($name);
            self::assertIsString($scope);

            if ($system !== null) {
                self::assertIsString($system);
            }

            // The empty name still needs a distinct data-set key.
            yield sprintf('%d:%s', $i, $name === '' ? '(empty)' : $name) => [
                $name,
                $scope,
                $system,
            ];
        }
    }

    #[DataProvider('scopeCases')]
    public function testScopeOfMatchesFixture(string $name, string $scope, ?string $system): void
    {
        self::assertSame($scope, Ext::scopeOf($name)->value, "scopeOf({$name})");
        self::assertSame($system, Ext::systemName($name), "systemName({$name})");
    }

    #[DataProvider('scopeCases')]
    public function testIsExtensionAgreesWithScope(string $name, string $scope, ?string $system): void
    {
        unset($system);

        // Every scope but "none" implies the X- prefix, and "none"
        // implies its absence. The two predicates cannot disagree.
        self::assertSame($scope !== 'none', Ext::isExtension($name), "isExtension({$name})");
    }

    /**
     * The fixture is the whole vocabulary: every Scope case is exercised
     * by at least one row, so a port cannot pass by never returning one.
     */
    public function testFixtureCoversEveryScope(): void
    {
        $seen = [];

        foreach (Behavior::cases('ext/scopes.json') as $case) {
            $name = $case['name'];
            self::assertIsString($name);
            $seen[Ext::scopeOf($name)->value] = true;
        }

        foreach (Scope::cases() as $scope) {
            self::assertArrayHasKey($scope->value, $seen, "no fixture row yields {$scope->value}");
        }
    }

    /**
     * The display spelling is the reference's `Scope.String()`, and is
     * distinct from the wire token -- notably `vstar` renders `VStar`.
     */
    public function testToStringRendersTheDisplaySpelling(): void
    {
        self::assertSame('None', Scope::None->toString());
        self::assertSame('VStar', Scope::VStar->toString());
        self::assertSame('System', Scope::System->toString());
        self::assertSame('Experimental', Scope::Experimental->toString());
        self::assertSame('Unknown', Scope::Unknown->toString());
    }

    /**
     * Selection preserves the component's own property order and does not
     * recurse into sub-components.
     */
    public function testExtensionsByScopeSelectsInPropertyOrder(): void
    {
        $child = new Component(CompType::Alarm, [
            new Property('X-VSTAR-NESTED', [], 'sub'),
        ]);
        $c = new Component(CompType::Todo, [
            new Property('X-VSTAR-HASH', [], 'sha256:0'),
            new Property('UID', [], 'u'),
            new Property('X-AGR-INTENT', [], 'a'),
            new Property('X-VSTAR-EFFECTIVE-STATUS', [], 'DONE'),
            new Property('X-EXP-DRAFT', [], 'e'),
            new Property('X-FOO', [], 'u'),
        ], [$child]);

        self::assertSame(
            ['X-VSTAR-HASH', 'X-VSTAR-EFFECTIVE-STATUS'],
            array_map(
                static fn (Property $p): string => $p->name,
                Ext::extensionsByScope($c, Scope::VStar),
            ),
        );
        self::assertSame(
            ['X-AGR-INTENT'],
            array_map(
                static fn (Property $p): string => $p->name,
                Ext::extensionsByScope($c, Scope::System),
            ),
        );
        self::assertSame(
            ['X-EXP-DRAFT'],
            array_map(
                static fn (Property $p): string => $p->name,
                Ext::extensionsByScope($c, Scope::Experimental),
            ),
        );
        self::assertSame(
            ['X-FOO'],
            array_map(
                static fn (Property $p): string => $p->name,
                Ext::extensionsByScope($c, Scope::Unknown),
            ),
        );
        self::assertSame(
            ['UID'],
            array_map(
                static fn (Property $p): string => $p->name,
                Ext::extensionsByScope($c, Scope::None),
            ),
        );
    }

    public function testExtensionsByScopeReturnsEmptyListWhenNothingMatches(): void
    {
        $c = new Component(CompType::Todo, [new Property('UID', [], 'u')]);

        self::assertSame([], Ext::extensionsByScope($c, Scope::VStar));
    }
}
