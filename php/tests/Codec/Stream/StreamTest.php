<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Codec\Stream;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Card;
use HopTop\Vstar\Codec\Rfc5545\Parser as CalendarParser;
use HopTop\Vstar\Codec\Rfc6350\Parser as CardParser;
use HopTop\Vstar\Codec\Stream\VCalendarEncoder;
use HopTop\Vstar\Codec\Stream\VCalendarParser;
use HopTop\Vstar\Codec\Stream\VCardEncoder;
use HopTop\Vstar\Codec\Stream\VCardParser;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Exception\AlreadyClosedException;
use HopTop\Vstar\Exception\HeaderLockedException;
use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\UnclosedBlockException;
use HopTop\Vstar\Exception\UnsupportedVersionException;
use HopTop\Vstar\Kind;
use HopTop\Vstar\Property;
use HopTop\Vstar\Tests\Corpus;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

/**
 * The constant-memory streaming codecs.
 *
 * The headline gate is agreement with the batch codecs: streaming every
 * corpus file must yield exactly what parsing it whole yields. That is
 * what stops the streaming parser drifting into its own dialect --
 * notably around TEXT unescaping, where each format's rules differ and
 * routing the vCard stream through the iCalendar unescaper would make the
 * two parses of `rfc6350/escaping.vcf` disagree.
 */
final class StreamTest extends TestCase
{
    // ---------------------------------------------------------------
    // Parser parity with the batch codecs
    // ---------------------------------------------------------------

    /**
     * @return iterable<string, array{string}>
     */
    public static function calendarFixtures(): iterable
    {
        foreach (Corpus::names('rfc5545', 'ics') as $name) {
            yield $name => [$name];
        }
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function cardFixtures(): iterable
    {
        foreach (Corpus::names('rfc6350', 'vcf') as $name) {
            yield $name => [$name];
        }
    }

    #[DataProvider('calendarFixtures')]
    public function testStreamedCalendarParseEqualsBatchParse(string $name): void
    {
        $bytes = Corpus::read("rfc5545/{$name}");
        $batch = CalendarParser::parse($bytes);

        $parser = new VCalendarParser(self::streamOf($bytes));
        $streamed = [];

        foreach ($parser as $component) {
            $streamed[] = $component;
        }

        self::assertSame(
            self::describeComponents($batch->components),
            self::describeComponents($streamed),
        );
        self::assertSame($batch->prodId, $parser->header()->prodId);
    }

    #[DataProvider('cardFixtures')]
    public function testStreamedCardParseEqualsBatchParse(string $name): void
    {
        $bytes = Corpus::read("rfc6350/{$name}");
        $batch = CardParser::parse($bytes);

        $streamed = [];

        foreach (new VCardParser(self::streamOf($bytes)) as $card) {
            $streamed[] = $card;
        }

        self::assertSame(self::describeCards($batch), self::describeCards($streamed));
    }

    /**
     * The escaping fixture is the one that catches a stream parser
     * borrowing the other format's unescaper: the iCalendar rules drop an
     * unknown escape's backslash while the vCard rules keep it, and
     * `FN:Last\, Comma Test` is observably different under each.
     */
    public function testStreamedVcardUnescapesWithVcardRules(): void
    {
        $bytes = Corpus::read('rfc6350/escaping.vcf');

        $batch = CardParser::parse($bytes);
        $streamed = [];

        foreach (new VCardParser(self::streamOf($bytes)) as $card) {
            $streamed[] = $card;
        }

        self::assertCount(1, $streamed);
        self::assertSame(
            $batch[0]->get('FN')?->value,
            $streamed[0]->get('FN')?->value,
        );
        self::assertSame('Last, Comma Test', $streamed[0]->get('FN')?->value);
        self::assertSame("line one\nline two", $streamed[0]->get('NOTE')?->value);
    }

    /**
     * Folds are resolved on **bytes**, so a fold that lands in the middle
     * of a multi-byte UTF-8 sequence rejoins into the original character
     * rather than two replacement bytes (spec rule 3).
     */
    public function testUnfoldingHappensOnBytes(): void
    {
        $bytes = Corpus::read('rfc5545/fold_split_utf8.ics');

        $batch = CalendarParser::parse($bytes);
        $streamed = [];

        foreach (new VCalendarParser(self::streamOf($bytes)) as $c) {
            $streamed[] = $c;
        }

        self::assertSame(
            self::describeComponents($batch->components),
            self::describeComponents($streamed),
        );

        foreach ($streamed as $c) {
            foreach ($c->props as $p) {
                self::assertTrue(
                    mb_check_encoding($p->value, 'UTF-8'),
                    "a fold split a UTF-8 sequence in {$p->name}",
                );
            }
        }
    }

    /**
     * Nested sub-components survive the stream: `STANDARD` and `DAYLIGHT`
     * inside a VTIMEZONE are neither of the named component types, and a
     * parser that dropped or flattened them would still round-trip.
     */
    public function testNestedSubComponentsSurvive(): void
    {
        $bytes = Corpus::read('rfc5545/nested_vtimezone.ics');
        $streamed = [];

        foreach (new VCalendarParser(self::streamOf($bytes)) as $c) {
            $streamed[] = $c;
        }

        self::assertSame(
            self::describeComponents(CalendarParser::parse($bytes)->components),
            self::describeComponents($streamed),
        );
    }

    // ---------------------------------------------------------------
    // Parser behavior
    // ---------------------------------------------------------------

    public function testHeaderIsReadableBeforeTheFirstComponent(): void
    {
        $parser = new VCalendarParser(self::streamOf(Corpus::read('rfc5545/one_vtodo.ics')));

        self::assertSame('-//V*//OneVTODO//EN', $parser->header()->prodId);
    }

    public function testExhaustionIsNotAnError(): void
    {
        $parser = new VCalendarParser(self::streamOf(Corpus::read('rfc5545/empty.ics')));

        self::assertSame([], iterator_to_array($parser, false));
    }

    /**
     * The parser is genuinely incremental: it must yield its first
     * component without having read the bytes that follow it.
     */
    public function testParserIsIncremental(): void
    {
        $bytes = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//p//EN\r\n"
            . "BEGIN:VTODO\r\nUID:first\r\nEND:VTODO\r\n"
            . "BEGIN:VTODO\r\nUID:second\r\nEND:VTODO\r\nEND:VCALENDAR\r\n";

        $stream = self::streamOf($bytes);
        $parser = new VCalendarParser($stream);

        $it = $parser->getIterator();
        $first = $it->current();

        self::assertInstanceOf(Component::class, $first);
        self::assertSame('first', $first->uid());
        self::assertLessThan(
            strlen($bytes),
            ftell($stream),
            'the first component must be yielded before the whole input is read',
        );
    }

    public function testVcardParserRejectsAnUnsupportedVersion(): void
    {
        $this->expectException(UnsupportedVersionException::class);
        iterator_to_array(
            new VCardParser(self::streamOf("BEGIN:VCARD\r\nVERSION:3.0\r\nEND:VCARD\r\n")),
            false,
        );
    }

    public function testVcardParserRejectsAnUnclosedBlock(): void
    {
        $this->expectException(UnclosedBlockException::class);
        iterator_to_array(
            new VCardParser(self::streamOf("BEGIN:VCARD\r\nVERSION:4.0\r\n")),
            false,
        );
    }

    public function testCalendarParserRejectsAMissingBegin(): void
    {
        $this->expectException(MalformedException::class);
        iterator_to_array(
            new VCalendarParser(self::streamOf("VERSION:2.0\r\n")),
            false,
        );
    }

    public function testCalendarParserRejectsAnUnclosedCalendar(): void
    {
        $this->expectException(UnclosedBlockException::class);
        iterator_to_array(
            new VCalendarParser(self::streamOf("BEGIN:VCALENDAR\r\nVERSION:2.0\r\n")),
            false,
        );
    }

    // ---------------------------------------------------------------
    // Encoder lifecycle
    // ---------------------------------------------------------------

    public function testCalendarEncoderWritesHeaderOnceAndTrailerOnClose(): void
    {
        $sink = self::sink();
        $enc = new VCalendarEncoder($sink);
        $enc->setHeader(new Calendar('-//V*//Stream//EN'));
        $enc->encode(self::todo('a'));
        $enc->encode(self::todo('b'));
        $enc->close();

        $out = self::drain($sink);

        self::assertSame(1, substr_count($out, "BEGIN:VCALENDAR\r\n"));
        self::assertSame(1, substr_count($out, "END:VCALENDAR\r\n"));
        self::assertStringContainsString("PRODID:-//V*//Stream//EN\r\n", $out);
        self::assertStringContainsString("UID:a\r\n", $out);
        self::assertStringContainsString("UID:b\r\n", $out);
    }

    /**
     * Streamed output re-parses to what was encoded.
     */
    public function testCalendarEncoderOutputReparses(): void
    {
        $sink = self::sink();
        $enc = new VCalendarEncoder($sink);
        $enc->setHeader(new Calendar('-//V*//Stream//EN'));
        $enc->encode(self::todo('a'));
        $enc->close();

        $cal = CalendarParser::parse(self::drain($sink));

        self::assertSame('-//V*//Stream//EN', $cal->prodId);
        self::assertCount(1, $cal->components);
        self::assertSame('a', $cal->components[0]->uid());
    }

    public function testCalendarEncoderEmitsAnEmptyCalendarOnBareClose(): void
    {
        $sink = self::sink();
        (new VCalendarEncoder($sink))->close();

        $out = self::drain($sink);
        self::assertStringContainsString("BEGIN:VCALENDAR\r\n", $out);
        self::assertStringContainsString("END:VCALENDAR\r\n", $out);
    }

    public function testSetHeaderAfterTheFirstEncodeIsLocked(): void
    {
        $enc = new VCalendarEncoder(self::sink());
        $enc->encode(self::todo('a'));

        $this->expectException(HeaderLockedException::class);
        $enc->setHeader(new Calendar('-//too//late//EN'));
    }

    public function testHeaderLockedCarriesTheGoSentinel(): void
    {
        $enc = new VCalendarEncoder(self::sink());
        $enc->encode(self::todo('a'));

        try {
            $enc->setHeader(new Calendar('-//too//late//EN'));
            self::fail('expected HeaderLockedException');
        } catch (HeaderLockedException $e) {
            self::assertSame('ErrHeaderLocked', $e->sentinel());
        }
    }

    public function testDoubleCloseIsAnError(): void
    {
        $enc = new VCalendarEncoder(self::sink());
        $enc->close();

        $this->expectException(AlreadyClosedException::class);
        $enc->close();
    }

    public function testEncodeAfterCloseIsAnError(): void
    {
        $enc = new VCalendarEncoder(self::sink());
        $enc->close();

        $this->expectException(AlreadyClosedException::class);
        $enc->encode(self::todo('a'));
    }

    public function testCardEncoderWritesSelfContainedBlocks(): void
    {
        $sink = self::sink();
        $enc = new VCardEncoder($sink);
        $enc->encode(new Card('u-1', Kind::Individual, [new Property('FN', [], 'One')]));
        $enc->encode(new Card('u-2', Kind::Individual, [new Property('FN', [], 'Two')]));
        $enc->close();

        $out = self::drain($sink);

        self::assertSame(2, substr_count($out, "BEGIN:VCARD\r\n"));
        self::assertSame(2, substr_count($out, "END:VCARD\r\n"));
        // No enclosing wrapper of any kind.
        self::assertStringNotContainsString('VCALENDAR', $out);

        self::assertCount(2, CardParser::parse($out));
    }

    /**
     * `VCardEncoder` has no `setHeader`, and the absence is contract: a
     * VCARD stream has no wrapper, so there is nothing for such a method
     * to do and its existence would invite callers to emit one the
     * parsers reject.
     */
    public function testCardEncoderHasNoSetHeader(): void
    {
        $names = array_map(
            static fn (\ReflectionMethod $m): string => strtolower($m->getName()),
            (new \ReflectionClass(VCardEncoder::class))->getMethods(),
        );

        self::assertNotContains(
            'setheader',
            $names,
            'VCardEncoder must not declare setHeader',
        );

        // The calendar encoder is the one that does have it, so the
        // assertion above is testing an absence rather than a typo.
        $calendarNames = array_map(
            static fn (\ReflectionMethod $m): string => strtolower($m->getName()),
            (new \ReflectionClass(VCalendarEncoder::class))->getMethods(),
        );
        self::assertContains('setheader', $calendarNames);
    }

    public function testCardEncoderDoubleCloseIsAnError(): void
    {
        $enc = new VCardEncoder(self::sink());
        $enc->close();

        $this->expectException(AlreadyClosedException::class);
        $enc->close();
    }

    public function testCardEncodeAfterCloseIsAnError(): void
    {
        $enc = new VCardEncoder(self::sink());
        $enc->close();

        $this->expectException(AlreadyClosedException::class);
        $enc->encode(new Card('u', Kind::Individual, []));
    }

    public function testAlreadyClosedCarriesTheGoSentinel(): void
    {
        $enc = new VCardEncoder(self::sink());
        $enc->close();

        try {
            $enc->close();
            self::fail('expected AlreadyClosedException');
        } catch (AlreadyClosedException $e) {
            self::assertSame('ErrAlreadyClosed', $e->sentinel());
        }
    }

    // ---------------------------------------------------------------
    // Round trip
    // ---------------------------------------------------------------

    #[DataProvider('calendarFixtures')]
    public function testStreamRoundTrip(string $name): void
    {
        $bytes = Corpus::read("rfc5545/{$name}");
        $original = CalendarParser::parse($bytes);

        $sink = self::sink();
        $enc = new VCalendarEncoder($sink);
        $enc->setHeader(new Calendar($original->prodId));

        foreach ($original->components as $c) {
            $enc->encode($c);
        }

        $enc->close();

        $reparsed = CalendarParser::parse(self::drain($sink));

        self::assertSame($original->prodId, $reparsed->prodId);
        self::assertSame(
            self::describeComponents($original->components),
            self::describeComponents($reparsed->components),
        );
    }

    // ---------------------------------------------------------------
    // Helpers
    // ---------------------------------------------------------------

    /**
     * @return resource
     */
    private static function streamOf(string $bytes)
    {
        $h = fopen('php://memory', 'r+b');

        if ($h === false) {
            throw new \RuntimeException('cannot open memory stream');
        }

        fwrite($h, $bytes);
        rewind($h);

        return $h;
    }

    /**
     * @return resource
     */
    private static function sink()
    {
        $h = fopen('php://memory', 'r+b');

        if ($h === false) {
            throw new \RuntimeException('cannot open memory stream');
        }

        return $h;
    }

    /**
     * @param resource $sink
     */
    private static function drain($sink): string
    {
        rewind($sink);
        $out = stream_get_contents($sink);

        return $out === false ? '' : $out;
    }

    /**
     * A structural description of a component list, so comparisons report
     * a readable diff rather than an object dump.
     *
     * @param list<Component> $components
     *
     * @return list<array<string, mixed>>
     */
    private static function describeComponents(array $components): array
    {
        return array_map(
            static fn (Component $c): array => [
                'type' => $c->type,
                'props' => array_map(
                    static fn (Property $p): array => [
                        'name' => $p->name,
                        'value' => $p->value,
                        'params' => array_map(
                            static fn (object $par): string => $par->name . '=' . $par->value,
                            $p->params,
                        ),
                    ],
                    $c->props,
                ),
                'sub' => self::describeComponents($c->sub),
            ],
            $components,
        );
    }

    /**
     * @param list<Card> $cards
     *
     * @return list<array<string, mixed>>
     */
    private static function describeCards(array $cards): array
    {
        return array_map(
            static fn (Card $c): array => [
                'uid' => $c->uid,
                'kind' => $c->kind?->value,
                'props' => array_map(
                    static fn (Property $p): array => [
                        'name' => $p->name,
                        'value' => $p->value,
                        'params' => array_map(
                            static fn (object $par): string => $par->name . '=' . $par->value,
                            $p->params,
                        ),
                    ],
                    $c->props,
                ),
            ],
            $cards,
        );
    }

    private static function todo(string $uid): Component
    {
        return new Component(CompType::Todo, [
            new Property('UID', [], $uid),
            new Property('DTSTAMP', [], '20260504T120000Z'),
        ]);
    }
}
