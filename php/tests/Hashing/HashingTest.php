<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Hashing;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Codec\Rfc5545\Parser as IcsParser;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Hashing\Hashing;
use HopTop\Vstar\Property;
use HopTop\Vstar\Tests\Corpus;
use PHPUnit\Framework\TestCase;

/**
 * The `sha256:<hex>` hash surface.
 *
 * The digest itself is the corpus's business -- {@see \HopTop\Vstar\Tests\Canonical\CorpusTest}
 * pins every `.hash` sibling. What is asserted here is the API contract
 * around it: the prefix is part of the value, the hash property is
 * excluded from its own digest, and the verification reports all three
 * fields rather than only a boolean.
 */
final class HashingTest extends TestCase
{
    public function testTheHashCarriesTheAlgorithmPrefix(): void
    {
        $got = Hashing::calendar(IcsParser::parse(Corpus::read('rfc5545/world.ics')));

        self::assertMatchesRegularExpression('/\Asha256:[0-9a-f]{64}\z/', $got);
    }

    /**
     * The prefix exists so a future `sha3-256:` or `blake3:` is
     * expressible without ambiguity. It is part of the value, not
     * decoration, so it is never stripped.
     */
    public function testTheDigestIsTheSha256OfTheCanonicalBytes(): void
    {
        $cal = IcsParser::parse(Corpus::read('rfc5545/world.ics'));
        $bytes = \HopTop\Vstar\Canonical\Canonical::calendar($cal);

        self::assertSame('sha256:' . hash('sha256', $bytes), Hashing::calendar($cal));
    }

    public function testSetXVstarWritesTheComponentHash(): void
    {
        $c = self::todo();
        $want = Hashing::component($c);

        Hashing::setXVstar($c);

        self::assertSame($want, Hashing::getXVstar($c));
    }

    /**
     * Rule 7 means the stored hash never feeds back into its own digest,
     * so re-stamping a component computes and writes the same value.
     */
    public function testSetXVstarIsIdempotent(): void
    {
        $c = self::todo();

        Hashing::setXVstar($c);
        $first = Hashing::getXVstar($c);
        Hashing::setXVstar($c);

        self::assertSame($first, Hashing::getXVstar($c));
        self::assertCount(1, $c->getAll(Hashing::X_VSTAR_HASH_PROPERTY), 'replaced, not duplicated');
    }

    public function testGetXVstarReportsAbsenceAsNull(): void
    {
        self::assertNull(Hashing::getXVstar(self::todo()));
    }

    public function testVerifyXVstarReportsAllThreeFields(): void
    {
        $c = self::todo();
        Hashing::setXVstar($c);

        $v = Hashing::verifyXVstar($c);

        self::assertTrue($v['ok']);
        self::assertSame($v['want'], $v['got']);
        self::assertNotSame('', $v['want']);
    }

    /**
     * A caller must be able to say *what* differed, not only *that*
     * something did -- which is the difference between a usable corruption
     * report and a shrug.
     */
    public function testVerifyXVstarReportsWantAndGotOnAMismatch(): void
    {
        $c = self::todo();
        $c->set(new Property(Hashing::X_VSTAR_HASH_PROPERTY, [], 'sha256:deadbeef'));

        $v = Hashing::verifyXVstar($c);

        self::assertFalse($v['ok']);
        self::assertSame('sha256:deadbeef', $v['got']);
        self::assertNotSame($v['got'], $v['want']);
    }

    public function testVerifyXVstarReportsAnEmptyGotWhenNoHashIsStored(): void
    {
        $v = Hashing::verifyXVstar(self::todo());

        self::assertFalse($v['ok']);
        self::assertSame('', $v['got']);
        self::assertNotSame('', $v['want']);
    }

    /**
     * Mutating the component after stamping invalidates the stored hash --
     * which is what makes it a corruption detector at all.
     */
    public function testAMutationAfterStampingFailsVerification(): void
    {
        $c = self::todo();
        Hashing::setXVstar($c);

        $c->set(new Property('SUMMARY', [], 'changed'));

        self::assertFalse(Hashing::verifyXVstar($c)['ok']);
    }

    /**
     * Every hash method except the one writer is pure.
     */
    public function testTheHashMethodsDoNotMutateTheirInput(): void
    {
        $c = self::todo();
        $c->add(new Property(Hashing::X_VSTAR_HASH_PROPERTY, [], 'sha256:00'));
        $before = count($c->props);

        Hashing::component($c);
        Hashing::calendar(new Calendar('-//V*//Test//EN', [$c]));

        self::assertCount($before, $c->props);
        self::assertSame('sha256:00', Hashing::getXVstar($c));
    }

    private static function todo(): Component
    {
        return new Component(CompType::Todo, [
            new Property('DTSTAMP', [], '20260504T120000Z'),
            new Property('UID', [], 'todo-hash-001'),
            new Property('SUMMARY', [], 'File the quarterly report'),
        ]);
    }
}
