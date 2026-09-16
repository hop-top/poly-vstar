<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Codec;

use HopTop\Vstar\Codec\Rfc5545\Parser;
use HopTop\Vstar\Codec\Rfc5545\Scanner;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

#[CoversClass(Scanner::class)]
final class ContentLineTest extends TestCase
{
    /**
     * @return iterable<string, array{string, list<string>}>
     */
    public static function unfoldCases(): iterable
    {
        yield 'crlf terminators' => ["A:1\r\nB:2\r\n", ['A:1', 'B:2']];
        yield 'lf terminators' => ["A:1\nB:2\n", ['A:1', 'B:2']];
        yield 'mixed terminators' => ["A:1\r\nB:2\n", ['A:1', 'B:2']];
        yield 'no trailing terminator' => ["A:1\r\nB:2", ['A:1', 'B:2']];
        yield 'space fold' => ["A:one\r\n two\r\n", ['A:onetwo']];
        yield 'tab fold' => ["A:one\r\n\ttwo\r\n", ['A:onetwo']];
        yield 'multi fold' => ["A:a\r\n b\r\n c\r\n", ['A:abc']];
        yield 'fold keeps inner space' => ["A:one\r\n  two\r\n", ['A:one two']];
        yield 'blank lines skipped' => ["\r\nA:1\r\n\r\nB:2\r\n", ['A:1', 'B:2']];
        yield 'empty input' => ['', []];
        yield 'only blank lines' => ["\r\n\r\n", []];
        yield 'wsp after blank starts fresh' => ["A:1\r\n\r\n more\r\n", ['A:1', 'more']];
    }

    /**
     * @param list<string> $expected
     */
    #[DataProvider('unfoldCases')]
    public function testScannerUnfoldsLiberally(string $input, array $expected): void
    {
        self::assertSame($expected, iterator_to_array(Scanner::lines($input), false));
    }

    /**
     * @return iterable<string, array{string, Property}>
     */
    public static function contentLineCases(): iterable
    {
        yield 'bare' => ['UID:abc', new Property('UID', [], 'abc')];
        yield 'empty value' => ['UID:', new Property('UID', [], '')];
        yield 'one param' => [
            'DTSTART;VALUE=DATE:20260515',
            new Property('DTSTART', [new Param('VALUE', 'DATE')], '20260515'),
        ];
        yield 'two params' => [
            'X;A=1;B=2:v',
            new Property('X', [new Param('A', '1'), new Param('B', '2')], 'v'),
        ];
        yield 'quoted param value with colon' => [
            'DTSTART;TZID="Etc/GMT+0":20260601T090000',
            new Property('DTSTART', [new Param('TZID', 'Etc/GMT+0')], '20260601T090000'),
        ];
        yield 'quoted param value with semicolon' => [
            'X;P="a;b":v',
            new Property('X', [new Param('P', 'a;b')], 'v'),
        ];
        yield 'quoted param value with comma' => [
            'X;P="a,b":v',
            new Property('X', [new Param('P', 'a,b')], 'v'),
        ];
        yield 'value containing colons' => [
            'UID:urn:uuid:1111',
            new Property('UID', [], 'urn:uuid:1111'),
        ];
        yield 'wire case preserved on names' => [
            'x-Foo;bAr=1:v',
            new Property('x-Foo', [new Param('bAr', '1')], 'v'),
        ];
        yield 'empty param value' => [
            'X;P=:v',
            new Property('X', [new Param('P', '')], 'v'),
        ];
    }

    #[DataProvider('contentLineCases')]
    public function testParseContentLine(string $line, Property $expected): void
    {
        $got = Parser::parseContentLine($line);

        self::assertSame($expected->name, $got->name);
        self::assertSame($expected->value, $got->value);
        self::assertCount(count($expected->params), $got->params);

        foreach ($expected->params as $i => $param) {
            self::assertSame($param->name, $got->params[$i]->name);
            self::assertSame($param->value, $got->params[$i]->value);
        }
    }

    /**
     * The parser unescapes TEXT-typed values so the model holds raw text;
     * the encoder re-escapes symmetrically. Non-TEXT values pass verbatim.
     *
     * @return iterable<string, array{string, string, string}>
     */
    public static function textEscapeCases(): iterable
    {
        yield 'comma' => ['SUMMARY', 'a\\,b', 'a,b'];
        yield 'semicolon' => ['SUMMARY', 'a\\;b', 'a;b'];
        yield 'backslash' => ['SUMMARY', 'a\\\\b', 'a\\b'];
        yield 'lowercase n' => ['SUMMARY', 'a\\nb', "a\nb"];
        yield 'uppercase N' => ['SUMMARY', 'a\\Nb', "a\nb"];
        yield 'escaped backslash before comma' => ['SUMMARY', 'a\\\\\\,b', 'a\\,b'];
        yield 'uri is not unescaped' => ['ATTACH', 'http://x/a\\,b', 'http://x/a\\,b'];
        yield 'plain text untouched' => ['SUMMARY', 'plain', 'plain'];
    }

    #[DataProvider('textEscapeCases')]
    public function testTextValuesAreUnescapedOnParse(string $name, string $wire, string $expected): void
    {
        $ics = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\nBEGIN:VTODO\r\n{$name}:{$wire}\r\nEND:VTODO\r\nEND:VCALENDAR\r\n";

        $prop = Parser::parse($ics)->components[0]->get($name);
        self::assertNotNull($prop);
        self::assertSame($expected, $prop->value);
    }
}
