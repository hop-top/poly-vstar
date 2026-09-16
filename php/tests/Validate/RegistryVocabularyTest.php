<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Validate;

use HopTop\Vstar\CompType;
use HopTop\Vstar\EventStatus;
use HopTop\Vstar\Generated\Codes;
use HopTop\Vstar\JournalStatus;
use HopTop\Vstar\RelType;
use HopTop\Vstar\TodoStatus;
use HopTop\Vstar\Transp;
use HopTop\Vstar\VClass;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * Reconciles this port's backed enums with the registry vocabularies.
 *
 * The two are built from different sources on purpose. The validator's
 * table is projected from the enums the codec encodes against, so it
 * cannot disagree with what the library writes; the generated tables are
 * rendered from `spec/registry/status-vocabulary.json`, so they cannot
 * disagree with the other four ports. These tests are the join -- what
 * lets both guarantees hold at once. Without them, generating the table
 * would buy cross-language agreement by giving up the codec linkage, and
 * deriving it would buy the codec linkage by giving up cross-language
 * agreement.
 *
 * Order matters, not just membership: the VS044 message joins the allowed
 * values, so a reordering is a user-visible change the behavior fixtures
 * pin.
 */
#[CoversClass(Codes::class)]
final class RegistryVocabularyTest extends TestCase
{
    /**
     * Projected from the backed enums, exactly as the validator's table
     * is -- not read back out of Codes, which would make the assertion
     * vacuous.
     *
     * @return array<string, list<string>>
     */
    private static function fromEnums(): array
    {
        return [
            CompType::Event->value => array_column(EventStatus::cases(), 'value'),
            CompType::Journal->value => array_column(JournalStatus::cases(), 'value'),
            CompType::Todo->value => array_column(TodoStatus::cases(), 'value'),
        ];
    }

    public function testStatusVocabularyCoversTheSameComponentTypes(): void
    {
        $got = array_keys(Codes::STATUS_VOCABULARY);
        $want = array_keys(self::fromEnums());
        sort($got);
        sort($want);

        self::assertSame($want, $got);
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function statusComponents(): iterable
    {
        foreach (array_keys(self::fromEnums()) as $comp) {
            yield $comp => [$comp];
        }
    }

    #[DataProvider('statusComponents')]
    public function testStatusVocabularyMatchesWireEnums(string $comp): void
    {
        self::assertSame(self::fromEnums()[$comp], Codes::STATUS_VOCABULARY[$comp]);
    }

    public function testClassVocabularyMatchesWireEnums(): void
    {
        self::assertSame(
            array_column(VClass::cases(), 'value'),
            Codes::CLASS_VOCABULARY,
        );
    }

    public function testTranspVocabularyMatchesWireEnums(): void
    {
        self::assertSame(
            array_column(Transp::cases(), 'value'),
            Codes::TRANSP_VOCABULARY,
        );
    }

    /**
     * RELTYPE is an open vocabulary -- RFC 5545 §3.2.15 admits IANA and
     * `X-` values -- so the assertion is that every value the registry
     * lists folds onto a registered constant, not that no other value may
     * appear on the wire.
     */
    public function testEveryRegistryRelTypeIsRegistered(): void
    {
        foreach (Codes::RELTYPE_VOCABULARY as $value) {
            [$parsed, $registered] = RelType::parse($value);

            self::assertTrue($registered, $value . ' is in the registry but not registered');
            self::assertSame($value, $parsed->value);
        }
    }

    /**
     * The converse of the above: a value this port registers but the
     * registry omits would be a silent cross-port divergence.
     */
    public function testRelTypeVocabularyMatchesTheRegisteredSet(): void
    {
        $registered = [
            RelType::PARENT,
            RelType::CHILD,
            RelType::SIBLING,
            RelType::FINISHTOSTART,
            RelType::FINISHTOFINISH,
            RelType::STARTTOFINISH,
            RelType::STARTTOSTART,
            RelType::DEPENDS_ON,
            RelType::FIRST,
            RelType::NEXT,
            RelType::CONCEPT,
            RelType::REFID,
        ];

        self::assertSame($registered, Codes::RELTYPE_VOCABULARY);
    }
}
