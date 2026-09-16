<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Supersession;

use HopTop\Vstar\Codec\Rfc5545\Parser;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Exception\TargetCorruptedException;
use HopTop\Vstar\Hashing\Hashing;
use HopTop\Vstar\Property;
use HopTop\Vstar\Supersession\Supersession;
use HopTop\Vstar\Tests\Behavior;
use HopTop\Vstar\Tests\Corpus;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * The `supersession/*.effective.json` behavior family, plus the
 * construction half the fixtures cannot state: the integrity check that
 * refuses a mutated target.
 *
 * The fixture inputs are the existing conformance calendars at
 * `spec/v1.0/conformance/supersession/<stem>.ics` -- no new `.ics` files
 * are minted for this family.
 */
final class SupersessionTest extends TestCase
{
    /**
     * Every `<stem>.effective.json` in the behavior family.
     *
     * @return iterable<string, array{string}>
     */
    public static function effectiveCases(): iterable
    {
        foreach (Behavior::stems('supersession', 'effective.json') as $stem) {
            yield $stem => [$stem];
        }
    }

    /**
     * The projection: for each component in the calendar, the status the
     * ledger assigns it -- and nothing at all for one it does not mention.
     */
    #[DataProvider('effectiveCases')]
    public function testEffectiveStatusProjection(string $stem): void
    {
        /** @var array<string, string> $expected */
        $expected = json_decode(
            Behavior::read("supersession/{$stem}.effective.json"),
            true,
            512,
            JSON_THROW_ON_ERROR,
        );

        $cal = Parser::parse(Corpus::read("supersession/{$stem}.ics"));
        $ledger = $cal->components;

        $actual = [];

        foreach ($cal->components as $c) {
            $status = Supersession::superseded($c, $ledger);

            if ($status !== null) {
                $actual[$c->uid()] = $status;
            }
        }

        ksort($actual);
        ksort($expected);

        self::assertSame($expected, $actual);
    }

    /**
     * `corrupt_mutated` is the empty projection: its hash is broken, but
     * nothing in the ledger supersedes it. Supersession is a query, not a
     * validator.
     */
    public function testCorruptTargetProjectsNothing(): void
    {
        $cal = Parser::parse(Corpus::read('supersession/corrupt_mutated.ics'));

        foreach ($cal->components as $c) {
            self::assertNull(Supersession::superseded($c, $cal->components));
        }
    }

    /**
     * The constructed record's shape, per spec/02: a VJOURNAL carrying the
     * derived UID, DTSTAMP, RELATED-TO, CATEGORIES, the effective status,
     * and a freshly computed X-VSTAR-HASH.
     */
    public function testSupersedesConstructsTheJournal(): void
    {
        $target = self::hashedTodo('todo-1');
        $at = new \DateTimeImmutable('2026-05-04T14:30:00', new \DateTimeZone('UTC'));

        $j = Supersession::supersedes($target, 'COMPLETED', $at);

        self::assertSame(CompType::Journal->value, $j->type);
        self::assertSame('journal:status:todo-1:20260504T143000Z', $j->uid());
        self::assertSame('20260504T143000Z', $j->dtstampRaw());
        self::assertSame('todo-1', $j->get('RELATED-TO')?->value);
        self::assertSame(
            Supersession::CATEGORY_STATUS_SUPERSESSION,
            $j->get('CATEGORIES')?->value,
        );
        self::assertSame('COMPLETED', $j->get(Supersession::PROP_EFFECTIVE_STATUS)?->value);

        // The hash is written last, so it covers every property above.
        $verified = Hashing::verifyXVstar($j);
        self::assertTrue($verified['ok'], 'constructed journal must verify its own hash');
    }

    /**
     * The target is not mutated -- the ledger is append-only.
     */
    public function testSupersedesLeavesTheTargetAlone(): void
    {
        $target = self::hashedTodo('todo-1');
        $before = Hashing::component($target);

        Supersession::supersedes(
            $target,
            'COMPLETED',
            new \DateTimeImmutable('2026-05-04T14:30:00', new \DateTimeZone('UTC')),
        );

        self::assertSame($before, Hashing::component($target));
    }

    /**
     * The integrity check: a target whose stored X-VSTAR-HASH no longer
     * matches its canonical form is refused outright. Writing a
     * supersession against a component mutated since it was hashed would
     * attach the new status to content nobody vouched for.
     */
    public function testSupersedesRefusesACorruptTarget(): void
    {
        $cal = Parser::parse(Corpus::read('supersession/corrupt_mutated.ics'));
        $target = $cal->components[0];

        // Guard the guard: the fixture really is corrupt.
        self::assertFalse(Hashing::verifyXVstar($target)['ok']);

        $this->expectException(TargetCorruptedException::class);
        Supersession::supersedes(
            $target,
            'COMPLETED',
            new \DateTimeImmutable('2026-05-04T14:30:00', new \DateTimeZone('UTC')),
        );
    }

    public function testTargetCorruptedCarriesTheGoSentinel(): void
    {
        $cal = Parser::parse(Corpus::read('supersession/corrupt_mutated.ics'));

        try {
            Supersession::supersedes(
                $cal->components[0],
                'COMPLETED',
                new \DateTimeImmutable('2026-05-04T14:30:00', new \DateTimeZone('UTC')),
            );
            self::fail('expected TargetCorruptedException');
        } catch (TargetCorruptedException $e) {
            self::assertSame('ErrTargetCorrupted', $e->sentinel());
        }
    }

    /**
     * A target carrying no X-VSTAR-HASH makes no integrity claim, so
     * there is nothing to verify and the construction proceeds.
     */
    public function testSupersedesAcceptsATargetWithoutAHash(): void
    {
        $target = new Component(CompType::Todo, [
            new Property('UID', [], 'todo-unhashed'),
            new Property('DTSTAMP', [], '20260504T120000Z'),
        ]);

        $j = Supersession::supersedes(
            $target,
            'CANCELLED',
            new \DateTimeImmutable('2026-05-04T14:30:00', new \DateTimeZone('UTC')),
        );

        self::assertSame('journal:status:todo-unhashed:20260504T143000Z', $j->uid());
    }

    /**
     * `superseded` answers "no" honestly rather than failing.
     */
    public function testSupersededReturnsNullWhenNothingMatches(): void
    {
        $c = self::hashedTodo('todo-1');

        self::assertNull(Supersession::superseded($c, []));
        self::assertNull(Supersession::superseded(new Component(CompType::Todo), []));
    }

    /**
     * Latest DTSTAMP wins; on a tie the later ledger position does.
     */
    public function testSupersededPicksTheLatestByDtstamp(): void
    {
        $target = self::hashedTodo('todo-multi');
        $ledger = [
            self::journalFor('todo-multi', 'COMPLETED', '20260504T164500Z'),
            self::journalFor('todo-multi', 'IN-PROCESS', '20260504T143000Z'),
        ];

        self::assertSame('COMPLETED', Supersession::superseded($target, $ledger));
    }

    public function testSupersededBreaksTiesByLedgerPosition(): void
    {
        $target = self::hashedTodo('todo-tie');
        $ledger = [
            self::journalFor('todo-tie', 'IN-PROCESS', '20260504T143000Z'),
            self::journalFor('todo-tie', 'COMPLETED', '20260504T143000Z'),
        ];

        self::assertSame('COMPLETED', Supersession::superseded($target, $ledger));
    }

    /**
     * CATEGORIES is comma-delimited per RFC 5545 §3.8.1.2 and matched
     * token-wise, so a longer label that merely contains the token does
     * not falsely match.
     */
    public function testSupersededMatchesCategoriesTokenWise(): void
    {
        $target = self::hashedTodo('todo-cat');

        $multi = self::journalFor('todo-cat', 'COMPLETED', '20260504T143000Z');
        $multi->set(new Property('CATEGORIES', [], 'work, status-supersession ,urgent'));
        self::assertSame('COMPLETED', Supersession::superseded($target, [$multi]));

        $near = self::journalFor('todo-cat', 'COMPLETED', '20260504T143000Z');
        $near->set(new Property('CATEGORIES', [], 'status-supersession-deferred'));
        self::assertNull(Supersession::superseded($target, [$near]));
    }

    public function testSupersededIgnoresNonJournalLedgerEntries(): void
    {
        $target = self::hashedTodo('todo-x');
        $entry = self::journalFor('todo-x', 'COMPLETED', '20260504T143000Z');
        $notAJournal = new Component(CompType::Todo, $entry->props);

        self::assertNull(Supersession::superseded($target, [$notAJournal]));
    }

    /**
     * An entry with no effective-status property supersedes nothing: there
     * is no status to project.
     */
    public function testSupersededSkipsEntriesWithoutAStatus(): void
    {
        $target = self::hashedTodo('todo-y');
        $entry = self::journalFor('todo-y', 'COMPLETED', '20260504T143000Z');
        $entry->remove(Supersession::PROP_EFFECTIVE_STATUS);

        self::assertNull(Supersession::superseded($target, [$entry]));
    }

    private static function hashedTodo(string $uid): Component
    {
        $c = new Component(CompType::Todo, [
            new Property('UID', [], $uid),
            new Property('DTSTAMP', [], '20260504T120000Z'),
            new Property('SUMMARY', [], 'Buy milk'),
        ]);
        Hashing::setXVstar($c);

        return $c;
    }

    private static function journalFor(string $uid, string $status, string $dtstamp): Component
    {
        return new Component(CompType::Journal, [
            new Property('UID', [], "journal:status:{$uid}:{$dtstamp}"),
            new Property('DTSTAMP', [], $dtstamp),
            new Property('RELATED-TO', [], $uid),
            new Property('CATEGORIES', [], Supersession::CATEGORY_STATUS_SUPERSESSION),
            new Property(Supersession::PROP_EFFECTIVE_STATUS, [], $status),
        ]);
    }
}
