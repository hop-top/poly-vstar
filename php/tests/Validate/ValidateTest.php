<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Validate;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Codec\Rfc5545\Parser;
use HopTop\Vstar\Component;
use HopTop\Vstar\Generated\Codes;
use HopTop\Vstar\Property;
use HopTop\Vstar\Tests\Behavior;
use HopTop\Vstar\Tests\Corpus;
use HopTop\Vstar\Validate\Diagnostic;
use HopTop\Vstar\Validate\Severity;
use HopTop\Vstar\Validate\Validate;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * The `validate` surface -- V*'s semantic conformance checks -- gated by
 * `spec/behavior/validate/*.diagnostics.json`.
 *
 * Every `.ics` under `spec/behavior/validate/` has a same-stem
 * `.diagnostics.json` sibling naming the exact findings the reference
 * emits. The tree is walked rather than enumerated, so a fixture added to
 * the reference becomes a case here without a test edit.
 *
 * Messages are deliberately absent from the fixtures: a `Diagnostic`'s
 * message is human-readable and not part of the contract (see
 * `docs/validate-codes.md` §Stability). Comparison is on
 * `(code, severity, path)` only, sorted by `(path, code)`.
 */
final class ValidateTest extends TestCase
{
    /**
     * Every `<stem>.ics` in the behavior family.
     *
     * @return iterable<string, array{string}>
     */
    public static function fixtureCases(): iterable
    {
        foreach (Behavior::stems('validate', 'ics') as $stem) {
            yield $stem => [$stem];
        }
    }

    /**
     * Every `.ics` fixture in the two conformance families the gate
     * requires to be free of status diagnostics.
     *
     * @return iterable<string, array{string}>
     */
    public static function conformanceCalendars(): iterable
    {
        foreach (['rfc5545', 'supersession'] as $family) {
            foreach (Corpus::names($family, 'ics') as $name) {
                yield "{$family}/{$name}" => ["{$family}/{$name}"];
            }
        }
    }

    /**
     * The gate directory carries fixtures at all -- a walk over an empty
     * tree would otherwise report a vacuous pass.
     */
    public function testGateDirectoryIsNotEmpty(): void
    {
        self::assertNotSame([], Behavior::stems('validate', 'ics'));
    }

    /**
     * The layer-(d) gate: the diagnostics the port emits for a fixture
     * equal the ones the reference recorded, on code, severity and path.
     */
    #[DataProvider('fixtureCases')]
    public function testBehaviorFixture(string $stem): void
    {
        $got = self::rows(Validate::validate(self::calendar($stem)));

        self::assertSame(self::sortRows(self::expected($stem)), self::sortRows($got));
    }

    /**
     * Every code the registry declares is exercised by at least one
     * fixture. A code no fixture reaches is an untested rule.
     */
    public function testEveryRegistryCodeIsCoveredByAFixture(): void
    {
        $emitted = [];

        foreach (Behavior::stems('validate', 'ics') as $stem) {
            foreach (Validate::validate(self::calendar($stem)) as $d) {
                $emitted[$d->code] = true;
            }
        }

        $missing = array_values(array_filter(
            Validate::codes(),
            static fn (string $code): bool => !isset($emitted[$code]),
        ));

        self::assertSame([], $missing);
    }

    /**
     * Each fixture row's severity is the one the registry assigns its
     * code -- the fixtures and the registry cannot drift apart silently.
     */
    #[DataProvider('fixtureCases')]
    public function testFixtureSeveritiesMatchTheRegistry(string $stem): void
    {
        $rows = self::expected($stem);
        $agree = array_values(array_filter(
            $rows,
            static fn (array $row): bool => (Codes::SEVERITIES[$row['code']] ?? null) === $row['severity'],
        ));

        self::assertSame($rows, $agree);
    }

    /**
     * Each emitted diagnostic's severity is the registry's, not the
     * emitting rule's opinion.
     */
    #[DataProvider('fixtureCases')]
    public function testEmittedSeveritiesMatchTheRegistry(string $stem): void
    {
        $diagnostics = Validate::validate(self::calendar($stem));

        $disagreeing = array_values(array_map(
            static fn (Diagnostic $d): string => $d->code . '=' . $d->severity->toString(),
            array_filter(
                $diagnostics,
                static fn (Diagnostic $d): bool => (Codes::SEVERITIES[$d->code] ?? null) !== $d->severity->toString(),
            ),
        ));

        self::assertSame([], $disagreeing);
    }

    /**
     * `codes()` lists exactly the registry's codes.
     */
    public function testCodesListsEveryRegistryCode(): void
    {
        $want = array_keys(Codes::SEVERITIES);
        sort($want);

        $got = Validate::codes();
        sort($got);

        self::assertSame($want, $got);
    }

    /**
     * `codes()` is sorted, mirroring the reference's own ordering
     * guarantee.
     */
    public function testCodesIsSorted(): void
    {
        $got = Validate::codes();
        $sorted = $got;
        sort($sorted);

        self::assertSame($sorted, $got);
    }

    /**
     * `severityOf()` answers from the registry for every known code.
     */
    public function testSeverityOfAnswersFromTheRegistry(): void
    {
        foreach (Codes::SEVERITIES as $code => $severity) {
            $got = Validate::severityOf($code);

            self::assertNotNull($got, "severityOf({$code}) must not be null");
            self::assertSame($severity, $got->toString());
        }
    }

    /**
     * `severityOf()` distinguishes the two severities rather than
     * answering with a constant -- the registry carries both, so a
     * constant answer would pass a per-code comparison against itself.
     */
    public function testSeverityOfSpansBothSeverities(): void
    {
        $seen = [];

        foreach (array_keys(Codes::SEVERITIES) as $code) {
            $got = Validate::severityOf($code);
            self::assertNotNull($got);
            $seen[$got->toString()] = true;
        }

        self::assertSame(['error', 'warning'], self::sortedKeys($seen));
    }

    /**
     * An unknown code is absence, not a throw: consumers must tolerate
     * codes minted after their release.
     */
    public function testSeverityOfIsNullForAnUnknownCode(): void
    {
        self::assertNull(Validate::severityOf('NOT-A-CODE'));
    }

    /**
     * `standardPropertyCount()` reports the generated allow-list's size.
     */
    public function testStandardPropertyCountMatchesTheGeneratedList(): void
    {
        self::assertSame(count(Codes::STANDARD_PROPERTIES), Validate::standardPropertyCount());
        self::assertGreaterThan(0, Validate::standardPropertyCount());
    }

    /**
     * The `Severity` enum stringifies to the registry's spelling. PHP
     * forbids `__toString()` on an enum, so the API mapping spells this
     * `toString()`.
     */
    public function testSeverityStringifies(): void
    {
        self::assertSame('error', Severity::Error->toString());
        self::assertSame('warning', Severity::Warning->toString());
    }

    /**
     * `validateComponent` paths start at the component, with no calendar
     * prefix.
     */
    public function testValidateComponentPathsOmitTheCalendarPrefix(): void
    {
        $got = self::rows(Validate::validateComponent(self::soleComponent('missing_dtstamp')));

        self::assertSame([[
            'code' => Codes::MISSING_DTSTAMP,
            'severity' => 'error',
            'path' => 'VJOURNAL[uid=journal-no-dtstamp].DTSTAMP',
        ]], $got);
    }

    /**
     * A UID-less component falls back to its positional segment, in both
     * entry points.
     */
    public function testUidLessComponentUsesAPositionalSegment(): void
    {
        $paths = array_column(self::rows(Validate::validateComponent(self::soleComponent('missing_uid'))), 'path');

        self::assertContains('VJOURNAL[#0].UID', $paths);
    }

    /**
     * Two UID-less components of the same type get distinct positional
     * segments -- the counter advances rather than restarting.
     *
     * No behavior fixture carries this shape, so the calendar is built
     * here: without it, a path renderer that froze the index at zero
     * would collapse every unidentified component onto one locator and
     * still pass the whole gate.
     */
    public function testUidLessComponentsOfOneTypeGetDistinctSegments(): void
    {
        $cal = self::calendar('missing_uid');
        $sole = $cal->components[0];
        self::assertInstanceOf(Component::class, $sole);

        $cal->append(new Component($sole->type, $sole->props, $sole->sub));

        $paths = array_column(self::rows(Validate::validate($cal)), 'path');

        self::assertContains('VCALENDAR.VJOURNAL[#0].UID', $paths);
        self::assertContains('VCALENDAR.VJOURNAL[#1].UID', $paths);
    }

    /**
     * The component-local half of the supersession rule still fires
     * without a ledger.
     */
    public function testValidateComponentStillEmitsTheLocalSupersessionDiagnostic(): void
    {
        $codes = array_column(
            self::rows(Validate::validateComponent(self::soleComponent('supersession_missing_props'))),
            'code',
        );

        self::assertContains(Codes::SUPERSESSION_MISSING_PROPS, $codes);
    }

    /**
     * The orphan rule needs a ledger to resolve against, so the
     * single-component entry point stays silent rather than guessing.
     */
    public function testValidateComponentSkipsTheOrphanDiagnostic(): void
    {
        $fromCalendar = array_column(self::rows(Validate::validate(self::calendar('supersession_orphan'))), 'code');
        self::assertContains(Codes::SUPERSESSION_ORPHAN, $fromCalendar);

        $fromComponent = array_column(
            self::rows(Validate::validateComponent(self::soleComponent('supersession_orphan'))),
            'code',
        );
        self::assertNotContains(Codes::SUPERSESSION_ORPHAN, $fromComponent);
    }

    /**
     * The conformance corpus carries only STATUS values inside their own
     * component type's vocabulary. A hit here means the port's status
     * scoping is wrong, not the fixture.
     */
    #[DataProvider('conformanceCalendars')]
    public function testConformanceCorpusIsCleanOfStatusDiagnostics(string $relative): void
    {
        $cal = Parser::parse(Corpus::read($relative));

        $hits = array_values(array_filter(
            Validate::validate($cal),
            static fn (Diagnostic $d): bool => $d->code === Codes::STATUS_NOT_IN_VOCABULARY,
        ));

        self::assertSame([], array_map(static fn (Diagnostic $d): string => $d->path, $hits));
    }

    /**
     * The cross-type shape the status rule exists for: DRAFT is legal
     * iCalendar text, and legal on a VJOURNAL, but not on a VEVENT.
     */
    public function testAJournalOnlyStatusOnAVeventIsFlagged(): void
    {
        $cal = self::calendar('clean_vevent');
        $comp = $cal->components[0];
        self::assertInstanceOf(Component::class, $comp);

        $comp->set(new Property('STATUS', [], 'DRAFT'));

        $rows = self::rows(Validate::validate($cal));

        self::assertContains(Codes::STATUS_NOT_IN_VOCABULARY, array_column($rows, 'code'));
        self::assertContains(
            'VCALENDAR.VEVENT[uid=event-clean].STATUS',
            array_column($rows, 'path'),
        );
    }

    /**
     * Hand-written source must reference the generated constant by name.
     * A literal spelled out by hand is a second source of truth that
     * drifts silently the moment the registry changes; `make
     * registry-check` guards the generated file, and this guards
     * everything else.
     */
    public function testNoHandWrittenDiagnosticCodeLiterals(): void
    {
        $offenders = [];

        foreach (self::phpSources(self::srcRoot()) as $path) {
            if (str_starts_with($path, self::srcRoot() . '/Generated/')) {
                continue;
            }

            $text = file_get_contents($path);
            self::assertIsString($text);

            foreach (explode("\n", $text) as $i => $line) {
                if (preg_match(self::codeLiteralPattern(), $line) === 1) {
                    $offenders[] = $path . ':' . ($i + 1) . ': ' . trim($line);
                }
            }
        }

        self::assertSame([], $offenders);
    }

    /**
     * The guard above actually fires -- the generated module does carry
     * the literals it is written to catch.
     */
    public function testTheCodeLiteralGuardFires(): void
    {
        $generated = file_get_contents(self::srcRoot() . '/Generated/Codes.php');
        self::assertIsString($generated);

        self::assertSame(1, preg_match(self::codeLiteralPattern(), $generated));
    }

    /**
     * The regex matching a diagnostic-code literal, assembled rather than
     * written out so this file does not itself contain one.
     */
    private static function codeLiteralPattern(): string
    {
        return '/' . implode('', ['V', 'S']) . '\d\d\d/';
    }

    /**
     * `php/src`, absolute.
     */
    private static function srcRoot(): string
    {
        return dirname(__DIR__, 2) . '/src';
    }

    /**
     * Every `.php` file under `$dir`, recursively, sorted.
     *
     * @return list<string>
     */
    private static function phpSources(string $dir): array
    {
        $out = [];

        /** @var \SplFileInfo $entry */
        foreach (new \RecursiveIteratorIterator(new \RecursiveDirectoryIterator($dir, \FilesystemIterator::SKIP_DOTS)) as $entry) {
            if ($entry->isFile() && $entry->getExtension() === 'php') {
                $out[] = $entry->getPathname();
            }
        }

        sort($out);

        return $out;
    }

    /**
     * Parse `<stem>.ics` from the behavior family.
     */
    private static function calendar(string $stem): Calendar
    {
        return Parser::parse(Behavior::read("validate/{$stem}.ics"));
    }

    /**
     * The lone component of a single-component fixture.
     */
    private static function soleComponent(string $stem): Component
    {
        $cal = self::calendar($stem);

        self::assertCount(1, $cal->components);
        $comp = $cal->components[0];
        self::assertInstanceOf(Component::class, $comp);

        return $comp;
    }

    /**
     * `<stem>.diagnostics.json`, as comparison rows.
     *
     * @return list<array{code: string, severity: string, path: string}>
     */
    private static function expected(string $stem): array
    {
        /** @var list<array{code: string, severity: string, path: string}> $decoded */
        $decoded = json_decode(
            Behavior::read("validate/{$stem}.diagnostics.json"),
            true,
            512,
            JSON_THROW_ON_ERROR,
        );

        return $decoded;
    }

    /**
     * Diagnostics reduced to the three fields the fixtures compare.
     *
     * @param list<Diagnostic> $diagnostics
     *
     * @return list<array{code: string, severity: string, path: string}>
     */
    private static function rows(array $diagnostics): array
    {
        return array_map(
            static fn (Diagnostic $d): array => [
                'code' => $d->code,
                'severity' => $d->severity->toString(),
                'path' => $d->path,
            ],
            $diagnostics,
        );
    }

    /**
     * Sort rows by `(path, code)`, the fixture-comparison contract.
     *
     * @param list<array{code: string, severity: string, path: string}> $rows
     *
     * @return list<array{code: string, severity: string, path: string}>
     */
    private static function sortRows(array $rows): array
    {
        usort(
            $rows,
            static fn (array $a, array $b): int => [$a['path'], $a['code']] <=> [$b['path'], $b['code']],
        );

        return $rows;
    }

    /**
     * The keys of `$m`, sorted.
     *
     * @param array<string, bool> $m
     *
     * @return list<string>
     */
    private static function sortedKeys(array $m): array
    {
        $keys = array_keys($m);
        sort($keys);

        return $keys;
    }
}
