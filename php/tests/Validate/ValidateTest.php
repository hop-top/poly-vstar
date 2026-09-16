<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Validate;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Codec\Rfc5545\Parser;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Generated\Codes;
use HopTop\Vstar\Hashing\Hashing;
use HopTop\Vstar\Property;
use HopTop\Vstar\Tests\Behavior;
use HopTop\Vstar\Tests\Corpus;
use HopTop\Vstar\Transp;
use HopTop\Vstar\Validate\Diagnostic;
use HopTop\Vstar\Validate\Severity;
use HopTop\Vstar\Validate\Validate;
use HopTop\Vstar\VClass;
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
     * The conformance corpus carries only values inside every §8 domain:
     * STATUS in its own component type's vocabulary, CLASS and TRANSP in
     * theirs, and the bounded integers canonical and in range. A hit here
     * means the port's rule is wrong, not the fixture.
     */
    #[DataProvider('conformanceCalendars')]
    public function testConformanceCorpusIsCleanOfValueDomainDiagnostics(string $relative): void
    {
        $cal = Parser::parse(Corpus::read($relative));
        $domainCodes = [
            Codes::STATUS_NOT_IN_VOCABULARY,
            Codes::CLASS_NOT_IN_VOCABULARY,
            Codes::TRANSP_NOT_IN_VOCABULARY,
            Codes::INTEGER_OUT_OF_DOMAIN,
        ];

        $hits = array_values(array_filter(
            Validate::validate($cal),
            static fn (Diagnostic $d): bool => in_array($d->code, $domainCodes, true),
        ));

        self::assertSame([], array_map(static fn (Diagnostic $d): string => $d->code . ' ' . $d->path, $hits));
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
     * The vocabulary rules check the value wherever the property appears:
     * a CLASS or TRANSP on a VTODO gets the same check as on a VEVENT, and
     * a value inside the vocabulary is clean there too, because component
     * scope is not diagnosed. No behavior fixture carries a non-VEVENT
     * shape, so a rule gated on VEVENT would pass the whole gate.
     *
     * @return iterable<string, array{string, string, string, string}>
     */
    public static function vocabularyOnNonEventCases(): iterable
    {
        yield 'CLASS on VTODO' => [CompType::Todo->value, 'CLASS', 'X-SECRET', Codes::CLASS_NOT_IN_VOCABULARY];
        yield 'CLASS on VJOURNAL' => [CompType::Journal->value, 'CLASS', 'X-SECRET', Codes::CLASS_NOT_IN_VOCABULARY];
        yield 'TRANSP on VTODO' => [CompType::Todo->value, 'TRANSP', 'BUSY', Codes::TRANSP_NOT_IN_VOCABULARY];
        yield 'TRANSP on VJOURNAL' => [CompType::Journal->value, 'TRANSP', 'BUSY', Codes::TRANSP_NOT_IN_VOCABULARY];
    }

    #[DataProvider('vocabularyOnNonEventCases')]
    public function testVocabularyRulesAreNotTypeGated(string $type, string $name, string $value, string $code): void
    {
        $rows = self::rows(Validate::validateComponent(self::withValue($type, $name, $value)));
        $hits = array_values(array_filter($rows, static fn (array $row): bool => $row['code'] === $code));

        self::assertSame([[
            'code' => $code,
            'severity' => 'error',
            'path' => $type . '[uid=value-1].' . $name,
        ]], $hits);
    }

    /**
     * The other half of "scope is not diagnosed": a registered value on a
     * component the RFC does not admit it on is clean of everything.
     */
    public function testRegisteredVocabularyValueOnAnotherTypeIsClean(): void
    {
        self::assertSame([], self::rows(Validate::validateComponent(
            self::withValue(CompType::Todo->value, 'TRANSP', Transp::Opaque->value),
        )));
        self::assertSame([], self::rows(Validate::validateComponent(
            self::withValue(CompType::Journal->value, 'CLASS', VClass::Confidential->value),
        )));
    }

    /**
     * spec/05 §8 canonical decimal: a sign, a leading zero or whitespace is
     * out of domain even when the number it spells is in range. No fixture
     * carries the whitespace shapes, and the sign / leading-zero shapes
     * are the ones an integer cast would silently accept.
     *
     * @return iterable<string, array{string, string}>
     */
    public static function nonCanonicalIntegerCases(): iterable
    {
        foreach (['PRIORITY', 'PERCENT-COMPLETE', 'SEQUENCE'] as $prop) {
            foreach (['+3', '07', ' 3', '3 '] as $value) {
                yield $prop . ':' . str_replace(' ', '_', $value) => [$prop, $value];
            }
        }
    }

    #[DataProvider('nonCanonicalIntegerCases')]
    public function testNonCanonicalIntegerIsOutOfDomain(string $prop, string $value): void
    {
        $rows = self::rows(Validate::validateComponent(self::withValue(CompType::Todo->value, $prop, $value)));
        $hits = array_values(array_filter(
            $rows,
            static fn (array $row): bool => $row['code'] === Codes::INTEGER_OUT_OF_DOMAIN,
        ));

        self::assertSame([[
            'code' => Codes::INTEGER_OUT_OF_DOMAIN,
            'severity' => 'error',
            'path' => 'VTODO[uid=value-1].' . $prop,
        ]], $hits);
    }

    /**
     * The domain edges, on a hand-built component so the rule -- not the
     * fixture -- is what decides. SEQUENCE has no upper bound and the
     * check is textual, so 2^64 (past `PHP_INT_MAX`) is well-formed; a
     * port that casts before deciding fails here.
     *
     * @return iterable<string, array{string, string, bool}>
     */
    public static function integerBoundaryCases(): iterable
    {
        yield 'PRIORITY:0' => ['PRIORITY', '0', true];
        yield 'PRIORITY:9' => ['PRIORITY', '9', true];
        yield 'PRIORITY:10' => ['PRIORITY', '10', false];
        yield 'PRIORITY:-1' => ['PRIORITY', '-1', false];
        yield 'PERCENT-COMPLETE:0' => ['PERCENT-COMPLETE', '0', true];
        yield 'PERCENT-COMPLETE:100' => ['PERCENT-COMPLETE', '100', true];
        yield 'PERCENT-COMPLETE:101' => ['PERCENT-COMPLETE', '101', false];
        yield 'PERCENT-COMPLETE:-1' => ['PERCENT-COMPLETE', '-1', false];
        yield 'SEQUENCE:0' => ['SEQUENCE', '0', true];
        yield 'SEQUENCE:-1' => ['SEQUENCE', '-1', false];
        yield 'SEQUENCE:2^64' => ['SEQUENCE', '18446744073709551616', true];
    }

    #[DataProvider('integerBoundaryCases')]
    public function testIntegerDomainBoundaries(string $prop, string $value, bool $clean): void
    {
        $codes = array_column(
            self::rows(Validate::validateComponent(self::withValue(CompType::Todo->value, $prop, $value))),
            'code',
        );

        if ($clean) {
            self::assertNotContains(Codes::INTEGER_OUT_OF_DOMAIN, $codes);
        } else {
            self::assertContains(Codes::INTEGER_OUT_OF_DOMAIN, $codes);
        }
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
     * A minimally valid component of `$type` carrying one extra property,
     * so the only interesting diagnostic is the one that property
     * provokes. The UID is fixed so paths are predictable; the hash is
     * computed last so the §1 and §2 rules stay quiet.
     */
    private static function withValue(string $type, string $name, string $value): Component
    {
        $props = [
            new Property('UID', [], 'value-1'),
            new Property('DTSTAMP', [], '20260504T120000Z'),
            new Property($name, [], $value),
        ];

        if ($type === CompType::Todo->value) {
            $props[] = new Property('DUE', [], '20260601T000000Z');
        } elseif ($type === CompType::Event->value) {
            $props[] = new Property('DTSTART', [], '20260601T000000Z');
        }

        $c = new Component($type, $props);
        Hashing::setXVstar($c);

        return $c;
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
