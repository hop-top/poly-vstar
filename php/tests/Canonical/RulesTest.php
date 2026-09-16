<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Tests\Canonical;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Canonical\Canonical;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Hashing\Hashing;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;
use PHPUnit\Framework\TestCase;

/**
 * Spec rules 1-12, exercised directly on the model rather than through the
 * corpus.
 *
 * Byte identity against the corpus is necessary but **not sufficient**:
 * disabling NFC entirely once passed every byte-identity fixture the
 * corpus then had, because none of them carried a decomposed value. Four
 * fixtures now close that particular hole, but the general lesson holds --
 * the corpus pins the rules it happens to reach, and a rule no fixture
 * reaches can regress silently.
 *
 * These tests cover the rest, so a regression surfaces here, at the rule it
 * broke, rather than as an unexplained corpus byte offset several layers
 * away.
 */
final class RulesTest extends TestCase
{
    private const DTSTAMP_VALUE = '20260504T120000Z';

    public function testRuleOneTerminatesEveryPhysicalLineWithCrlf(): void
    {
        $got = Canonical::component(
            self::todo([self::dtstamp(), new Property('UID', [], 'u')]),
        );

        self::assertSame(
            "BEGIN:VTODO\r\nDTSTAMP:20260504T120000Z\r\nUID:u\r\nEND:VTODO\r\n",
            $got,
        );
    }

    public function testRuleTwoSortsPropertiesAlphabeticallyByName(): void
    {
        $got = Canonical::component(self::todo([
            new Property('SUMMARY', [], 's'),
            new Property('UID', [], 'u'),
            self::dtstamp(),
        ]));

        self::assertSame(
            ['DTSTAMP:20260504T120000Z', 'SUMMARY:s', 'UID:u'],
            array_slice(explode("\r\n", $got), 1, 3),
        );
    }

    public function testRuleTwoSortsEachPropertysParametersAlphabetically(): void
    {
        $got = Canonical::component(self::todo([
            new Property('ATTENDEE', [
                new Param('ROLE', 'CHAIR'),
                new Param('CN', 'Ada'),
                new Param('PARTSTAT', 'ACCEPTED'),
            ], 'mailto:a@example.com'),
        ]));

        self::assertStringContainsString(
            'ATTENDEE;CN=Ada;PARTSTAT=ACCEPTED;ROLE=CHAIR:mailto:a@example.com',
            $got,
        );
    }

    public function testRuleTwoSortIsStableForEquallyNamedProperties(): void
    {
        $got = Canonical::component(self::todo([
            new Property('MEMBER', [], 'b'),
            new Property('MEMBER', [], 'a'),
            new Property('MEMBER', [], 'c'),
        ]));

        self::assertSame(
            ['MEMBER:b', 'MEMBER:a', 'MEMBER:c'],
            array_slice(explode("\r\n", $got), 1, 3),
        );
    }

    public function testRuleThreeFoldsAnAsciiLineAtExactlySeventyFiveOctets(): void
    {
        $lines = self::physicalLines(
            Canonical::component(self::todo([new Property('SUMMARY', [], str_repeat('x', 200))])),
        );

        foreach ($lines as $line) {
            self::assertLessThanOrEqual(75, strlen($line));
        }

        self::assertSame(75, strlen($lines[1]));
    }

    /**
     * Rule 3 counts OCTETS. `strlen` is PHP's byte count and is correct;
     * `mb_strlen` counts characters and would let a line carrying
     * non-ASCII text run past the limit on the wire.
     *
     * Forty `é` plus `SUMMARY:` is 88 octets but only 48 characters, so a
     * character-counting implementation would not fold this line at all.
     */
    public function testRuleThreeMeasuresUtf8BytesNotCharacters(): void
    {
        $lines = self::physicalLines(
            Canonical::component(self::todo([new Property('SUMMARY', [], str_repeat("\u{e9}", 40))])),
        );

        foreach ($lines as $line) {
            self::assertLessThanOrEqual(75, strlen($line));
        }

        self::assertGreaterThan(3, count($lines));
        // Continuation lines lead with the SP RFC 5545 §3.1 requires.
        self::assertSame(' ', $lines[2][0]);
    }

    /**
     * Rule 3 is octet-exact and **may split a multi-byte UTF-8 sequence**.
     * An individual physical line is therefore not necessarily valid UTF-8
     * on its own; unfolding rejoins the sequence and the logical line
     * decodes cleanly.
     *
     * An implementation that retreated the cut to a character boundary
     * would move the fold and change both the canonical bytes and the
     * hash, so this asserts the split happens: the first physical line is
     * exactly 75 octets and its last byte is a UTF-8 lead byte with its
     * continuation bytes stranded on the next line.
     */
    public function testRuleThreeSplitsAMultiByteSequenceRatherThanRetreating(): void
    {
        // `SUMMARY:` is 8 octets; 66 ASCII `a` take the line to 74, so the
        // next character -- a two-octet `é` -- straddles the boundary.
        $value = str_repeat('a', 66) . str_repeat("\u{e9}", 4);
        $lines = self::physicalLines(
            Canonical::component(self::todo([new Property('SUMMARY', [], $value)])),
        );

        $first = $lines[1];
        self::assertSame(75, strlen($first));
        // 0xC3 is the lead byte of `é` in UTF-8; its 0xA9 continuation is
        // on the following physical line.
        self::assertSame(0xC3, ord($first[74]), 'the fold must cut mid-sequence, not retreat');
        self::assertSame(0xA9, ord($lines[2][1]), 'the continuation byte leads the next line');

        // The split line is not valid UTF-8 on its own -- which is the
        // point -- while the whole canonical form unfolds cleanly.
        self::assertFalse(mb_check_encoding($first, 'UTF-8'));
    }

    public function testRuleThreeFoldsAfterParametersAreAppended(): void
    {
        // The parameters push the fold point into the value. Folding the
        // value alone would put the break 30 octets later and still unfold
        // to the same logical line, so only the byte positions catch it.
        $got = Canonical::component(self::todo([
            new Property(
                'ATTENDEE',
                [new Param('CN', 'A Person With A Fairly Long Common Name')],
                'mailto:someone-with-a-long-address@example.com',
            ),
        ]));

        $first = self::physicalLines($got)[1];

        self::assertSame(75, strlen($first));
        self::assertStringStartsWith('ATTENDEE;CN=', $first);
    }

    public function testRuleSixSortsTopLevelComponentsByUid(): void
    {
        $got = Canonical::calendar(self::cal([
            self::todo([self::dtstamp(), new Property('UID', [], 'charlie')]),
            self::todo([self::dtstamp(), new Property('UID', [], 'alpha')]),
            self::todo([self::dtstamp(), new Property('UID', [], 'bravo')]),
        ]));

        preg_match_all('/UID:(\w+)/', $got, $m);
        self::assertSame(['UID:alpha', 'UID:bravo', 'UID:charlie'], $m[0]);
    }

    public function testRuleSixSortsAVtimezoneByItsTzid(): void
    {
        $tz = static fn (string $id): Component => new Component(
            CompType::Timezone,
            [new Property('TZID', [], $id)],
        );

        $got = Canonical::calendar(self::cal([$tz('Zulu'), $tz('Alpha'), $tz('Mike')]));

        preg_match_all('/TZID:(\w+)/', $got, $m);
        self::assertSame(['TZID:Alpha', 'TZID:Mike', 'TZID:Zulu'], $m[0]);
    }

    /**
     * Rule 6 specifies a byte-wise (UTF-8 lexicographic) comparison.
     *
     * PHP strings are byte arrays and `strcmp` compares them byte by byte,
     * so byte order **is** UTF-8 order here and no custom comparator is
     * needed. This confirms that rather than assuming it.
     *
     * U+FF21 FULLWIDTH LATIN CAPITAL A encodes as `EF BC A1`; U+1D400
     * MATHEMATICAL BOLD CAPITAL A encodes as `F0 9D 90 80`. Byte-wise the
     * fullwidth A sorts FIRST (0xEF < 0xF0). A UTF-16 language gets the
     * opposite answer, because the astral character is the surrogate pair
     * D835 DC00 and sorts below 0xFF21 -- which is the trap this asserts
     * PHP does not fall into.
     */
    public function testRuleSixComparesUidsByteWiseOnUtf8(): void
    {
        $got = Canonical::calendar(self::cal([
            self::todo([self::dtstamp(), new Property('UID', [], "\u{1D400}")]),
            self::todo([self::dtstamp(), new Property('UID', [], "\u{FF21}")]),
        ]));

        self::assertLessThan(
            strpos($got, "UID:\u{1D400}"),
            strpos($got, "UID:\u{FF21}"),
            'the fullwidth A must sort before the astral A, as its UTF-8 bytes do',
        );
    }

    public function testRuleSixSortsKeylessComponentsLastInStableInputOrder(): void
    {
        $keyless = static fn (string $s): Component => new Component(
            CompType::Todo,
            [new Property('DTSTAMP', [], self::DTSTAMP_VALUE), new Property('SUMMARY', [], $s)],
        );

        $got = Canonical::calendar(self::cal([
            $keyless('second'),
            self::todo([self::dtstamp(), new Property('UID', [], 'u')]),
            $keyless('third'),
        ]));

        preg_match_all('/UID:u|SUMMARY:(\w+)/', $got, $m);
        self::assertSame(['UID:u', 'SUMMARY:second', 'SUMMARY:third'], $m[0]);
    }

    public function testRuleSixPreservesSubComponentInputOrder(): void
    {
        $alarm = static fn (string $uid): Component => new Component(
            CompType::Alarm,
            [new Property('UID', [], $uid), new Property('ACTION', [], 'DISPLAY')],
        );

        $got = Canonical::component(new Component(
            CompType::Todo,
            [self::dtstamp(), new Property('UID', [], 'u')],
            [$alarm('zulu'), $alarm('alpha')],
        ));

        self::assertLessThan(strpos($got, 'UID:alpha'), strpos($got, 'UID:zulu'));
    }

    public function testRuleSevenStripsTheHashPropertyFromTheCanonicalBytes(): void
    {
        $got = Canonical::component(self::todo([
            self::dtstamp(),
            new Property('UID', [], 'u'),
            new Property(Hashing::X_VSTAR_HASH_PROPERTY, [], 'sha256:deadbeef'),
        ]));

        self::assertStringNotContainsString(Hashing::X_VSTAR_HASH_PROPERTY, $got);
    }

    public function testRuleSevenHashesIdenticallyWithAndWithoutAStoredHash(): void
    {
        $bare = self::cal([self::todo([self::dtstamp(), new Property('UID', [], 'u')])]);
        $stamped = self::cal([self::todo([
            self::dtstamp(),
            new Property('UID', [], 'u'),
            new Property(Hashing::X_VSTAR_HASH_PROPERTY, [], 'sha256:00'),
        ])]);

        self::assertSame(Hashing::calendar($bare), Hashing::calendar($stamped));
    }

    public function testRuleSevenStripsTheHashPropertyAtNestedDepth(): void
    {
        $alarm = self::alarm(...);

        $bare = self::cal([new Component(
            CompType::Todo,
            [self::dtstamp(), new Property('UID', [], 'u')],
            [$alarm([new Property('ACTION', [], 'DISPLAY')])],
        )]);

        $stamped = self::cal([new Component(
            CompType::Todo,
            [self::dtstamp(), new Property('UID', [], 'u')],
            [$alarm([
                new Property('ACTION', [], 'DISPLAY'),
                new Property(Hashing::X_VSTAR_HASH_PROPERTY, [], 'sha256:ff'),
            ])],
        )]);

        self::assertSame(Hashing::calendar($bare), Hashing::calendar($stamped));
    }

    public function testRuleEightPreservesRruleValuesVerbatim(): void
    {
        $a = Canonical::component(self::todo([new Property('RRULE', [], 'FREQ=DAILY;INTERVAL=1')]));
        $b = Canonical::component(self::todo([new Property('RRULE', [], 'FREQ=DAILY')]));

        self::assertStringContainsString('RRULE:FREQ=DAILY;INTERVAL=1', $a);
        self::assertStringContainsString('RRULE:FREQ=DAILY', $b);
        self::assertNotSame($a, $b, 'an elided default must not be normalized away');
    }

    public function testRuleEightDoesNotReorderRuleParts(): void
    {
        $got = Canonical::component(self::todo([new Property('RRULE', [], 'BYDAY=MO,WE;FREQ=WEEKLY')]));

        self::assertStringContainsString('RRULE:BYDAY=MO,WE;FREQ=WEEKLY', $got);
    }

    public function testRuleNineNormalizesPropertyValues(): void
    {
        $got = Canonical::component(self::todo([
            new Property('SUMMARY', [], 'caf' . self::DECOMPOSED),
        ]));

        self::assertStringContainsString('SUMMARY:caf' . self::COMPOSED, $got);
        self::assertStringNotContainsString(self::DECOMPOSED, $got);
    }

    public function testRuleNineNormalizesParameterValues(): void
    {
        $got = Canonical::component(self::todo([
            new Property('ATTENDEE', [new Param('CN', 'Ren' . self::DECOMPOSED)], 'mailto:a@b.c'),
        ]));

        self::assertStringContainsString('CN=Ren' . self::COMPOSED, $got);
    }

    public function testRuleNineDoesNotNormalizePropertyNames(): void
    {
        // A name is ASCII by construction; the assertion is that the name
        // path is the uppercase path and nothing else touches it.
        $got = Canonical::component(self::todo([new Property('x-lower', [], 'v')]));

        self::assertStringContainsString('X-LOWER:v', $got);
    }

    public function testRuleNineMakesTwoSpellingsOfTheSameTextHashIdentically(): void
    {
        $with = static fn (string $summary): Calendar => self::cal([self::todo([
            self::dtstamp(),
            new Property('UID', [], 'u'),
            new Property('SUMMARY', [], $summary),
        ])]);

        self::assertSame(
            Hashing::calendar($with('caf' . self::DECOMPOSED)),
            Hashing::calendar($with('caf' . self::COMPOSED)),
        );
    }

    /**
     * NFC runs BEFORE folding. Normalization changes a string's UTF-8
     * length -- forty decomposed `é` are 120 octets, composed they are 80
     * -- so folding a pre-normalization string puts the break at a
     * different octet and produces different canonical bytes.
     */
    public function testRuleNineNormalizesBeforeFolding(): void
    {
        $a = Canonical::component(self::todo([
            new Property('SUMMARY', [], str_repeat(self::DECOMPOSED, 40)),
        ]));
        $b = Canonical::component(self::todo([
            new Property('SUMMARY', [], str_repeat(self::COMPOSED, 40)),
        ]));

        self::assertSame($b, $a);
    }

    public function testRuleNineIsIdempotentSoCanonicalFormIsAFixpoint(): void
    {
        $once = Canonical::component(self::todo([
            self::dtstamp(),
            new Property('UID', [], 'u'),
            new Property('SUMMARY', [], 'caf' . self::DECOMPOSED),
        ]));

        $twice = Canonical::component(self::todo([
            self::dtstamp(),
            new Property('UID', [], 'u'),
            new Property('SUMMARY', [], 'caf' . self::COMPOSED),
        ]));

        self::assertSame($twice, $once);
    }

    public function testRuleTenStripsBinaryValueAndBase64Encoding(): void
    {
        $got = Canonical::component(self::todo([
            new Property('ATTACH', [
                new Param('VALUE', 'BINARY'),
                new Param('ENCODING', 'BASE64'),
                new Param('FMTTYPE', 'text/plain'),
            ], 'aGVsbG8='),
        ]));

        self::assertStringContainsString('ATTACH;FMTTYPE=text/plain:aGVsbG8=', $got);
        self::assertStringNotContainsString('BINARY', $got);
        self::assertStringNotContainsString('BASE64', $got);
    }

    public function testRuleTenMatchesTheParameterNamesAndValuesCaseInsensitively(): void
    {
        $got = Canonical::component(self::todo([
            new Property('ATTACH', [
                new Param('value', 'binary'),
                new Param('encoding', 'base64'),
            ], 'x'),
        ]));

        self::assertStringContainsString('ATTACH:x', $got);
    }

    public function testRuleTenLeavesANonAttachBinaryValueAlone(): void
    {
        $got = Canonical::component(self::todo([
            new Property('X-BLOB', [new Param('VALUE', 'BINARY')], 'x'),
        ]));

        self::assertStringContainsString('X-BLOB;VALUE=BINARY:x', $got);
    }

    public function testRuleElevenEmitsTheDateVerbatimAndRetainsValueDate(): void
    {
        $got = Canonical::component(self::todo([
            new Property('DTSTART', [new Param('VALUE', 'DATE')], '20260515'),
        ]));

        self::assertStringContainsString('DTSTART;VALUE=DATE:20260515', $got);
    }

    public function testRuleElevenUpperCasesTheValueArgumentSoCaseVariantsConverge(): void
    {
        $got = Canonical::component(self::todo([
            new Property('DUE', [new Param('VALUE', 'date')], '20260516'),
        ]));

        self::assertStringContainsString('DUE;VALUE=DATE:20260516', $got);
    }

    public function testRuleElevenStripsAStrayTzidFromADate(): void
    {
        $got = Canonical::component(self::todo([
            new Property('DUE', [
                new Param('TZID', 'America/Montreal'),
                new Param('VALUE', 'DATE'),
            ], '20260516'),
        ]));

        self::assertStringContainsString('DUE;VALUE=DATE:20260516', $got);
        self::assertStringNotContainsString('TZID', $got);
    }

    /**
     * A DATE is never promoted to a DATE-TIME, even when the calendar
     * carries a VTIMEZONE that would resolve the TZID. Rule 11 says the
     * resolution registry is not consulted at all.
     */
    public function testRuleElevenNeverPromotesADateEvenWithAResolvableVtimezone(): void
    {
        $zone = new Component(CompType::Timezone, [new Property('TZID', [], 'Fixed/Plus05')], [
            new Component('STANDARD', [
                new Property('DTSTART', [], '19700101T000000'),
                new Property('TZOFFSETFROM', [], '+0500'),
                new Property('TZOFFSETTO', [], '+0500'),
            ]),
        ]);

        $c = self::todo([
            self::dtstamp(),
            new Property('UID', [], 'u'),
            new Property('DTSTART', [
                new Param('TZID', 'Fixed/Plus05'),
                new Param('VALUE', 'DATE'),
            ], '20260515'),
        ]);

        $got = Canonical::componentInContext($c, self::cal([$zone, $c]));

        self::assertStringContainsString('DTSTART;VALUE=DATE:20260515', $got);
        self::assertStringNotContainsString('T000000Z', $got);
        self::assertStringNotContainsString('20260514', $got);
    }

    public function testRuleTwelveDoesNotNormalizeDurationUnits(): void
    {
        self::assertStringContainsString(
            'DURATION:P1D',
            Canonical::component(self::todo([new Property('DURATION', [], 'P1D')])),
        );
        self::assertStringContainsString(
            'DURATION:PT24H',
            Canonical::component(self::todo([new Property('DURATION', [], 'PT24H')])),
        );
    }

    public function testRuleTwelvePreservesARelativeTriggersSignAndSpelling(): void
    {
        $got = Canonical::component(self::todo([new Property('TRIGGER', [], '-PT15M')]));

        self::assertStringContainsString('TRIGGER:-PT15M', $got);
    }

    public function testRuleTwelveDistinguishesP0dFromPt0s(): void
    {
        $a = Canonical::component(self::todo([new Property('DURATION', [], 'P0D')]));
        $b = Canonical::component(self::todo([new Property('DURATION', [], 'PT0S')]));

        self::assertNotSame($b, $a);
    }

    public function testHashesSemanticallyIdenticalCalendarsInDifferentOrdersIdentically(): void
    {
        $a = self::cal([
            self::todo([new Property('UID', [], 'b'), self::dtstamp(), new Property('SUMMARY', [], 'two')]),
            self::todo([new Property('SUMMARY', [], 'one'), new Property('UID', [], 'a'), self::dtstamp()]),
        ]);

        $b = self::cal([
            self::todo([self::dtstamp(), new Property('UID', [], 'a'), new Property('SUMMARY', [], 'one')]),
            self::todo([new Property('SUMMARY', [], 'two'), self::dtstamp(), new Property('UID', [], 'b')]),
        ]);

        self::assertSame(Hashing::calendar($b), Hashing::calendar($a));
    }

    public function testHashesIdenticallyWhenOnlyParameterOrderDiffers(): void
    {
        self::assertSame(
            Hashing::calendar(self::attendeeWith([new Param('CN', 'Ada'), new Param('ROLE', 'CHAIR')])),
            Hashing::calendar(self::attendeeWith([new Param('ROLE', 'CHAIR'), new Param('CN', 'Ada')])),
        );
    }

    public function testCanonicalizationDoesNotMutateItsInput(): void
    {
        $c = self::todo([
            new Property('SUMMARY', [], 's'),
            new Property('UID', [], 'u'),
            new Property(Hashing::X_VSTAR_HASH_PROPERTY, [], 'sha256:00'),
        ]);

        $before = self::snapshot($c);
        Canonical::component($c);

        self::assertSame($before, self::snapshot($c));
    }

    public function testCalendarCanonicalizationDoesNotMutateItsInput(): void
    {
        $cal = self::cal([
            self::todo([new Property('SUMMARY', [], 's'), new Property('UID', [], 'z')]),
            self::todo([new Property('UID', [], 'a')]),
        ]);

        $before = array_map(self::snapshot(...), $cal->components);
        Canonical::calendar($cal);

        self::assertSame($before, array_map(self::snapshot(...), $cal->components));
    }

    /** `e` + COMBINING ACUTE ACCENT: three UTF-8 octets, NFD. */
    private const DECOMPOSED = "e\u{0301}";

    /** LATIN SMALL LETTER E WITH ACUTE: two UTF-8 octets, NFC. */
    private const COMPOSED = "\u{e9}";

    private static function dtstamp(): Property
    {
        return new Property('DTSTAMP', [], self::DTSTAMP_VALUE);
    }

    /**
     * @param list<Property> $props
     */
    private static function todo(array $props): Component
    {
        return new Component(CompType::Todo, $props);
    }

    /**
     * @param list<Component> $components
     */
    private static function cal(array $components): Calendar
    {
        return new Calendar('-//V*//Test//EN', $components);
    }

    /**
     * @param list<Property> $props
     */
    private static function alarm(array $props): Component
    {
        return new Component(CompType::Alarm, $props);
    }

    /**
     * A one-VTODO calendar whose single ATTENDEE carries `$params`, so two
     * parameter orderings can be hashed against each other.
     *
     * @param list<Param> $params
     */
    private static function attendeeWith(array $params): Calendar
    {
        return self::cal([self::todo([
            new Property('DTSTAMP', [], self::DTSTAMP_VALUE),
            new Property('UID', [], 'u'),
            new Property('ATTENDEE', $params, 'mailto:a@b.c'),
        ])]);
    }

    /**
     * A structural snapshot of a component, for mutation assertions.
     */
    private static function snapshot(Component $c): string
    {
        $parts = [$c->type];

        foreach ($c->props as $p) {
            $params = [];

            foreach ($p->params as $prm) {
                $params[] = $prm->name . '=' . $prm->value;
            }

            $parts[] = $p->name . '[' . implode(',', $params) . ']:' . $p->value;
        }

        foreach ($c->sub as $s) {
            $parts[] = self::snapshot($s);
        }

        return implode('|', $parts);
    }

    /**
     * Split canonical bytes into physical lines, measured as BYTES.
     *
     * Decoding first and splitting the text would be wrong here: RFC 5545
     * folding breaks on an octet boundary, which can land mid-UTF-8
     * sequence, and a decoder replaces each orphaned fragment with U+FFFD
     * -- three octets where there was one. That inflation is exactly what
     * a length assertion would then misread as an over-long line.
     *
     * @return list<string>
     */
    private static function physicalLines(string $bytes): array
    {
        $out = [];
        $start = 0;

        for ($i = 0, $n = strlen($bytes); $i + 1 < $n; ++$i) {
            if ($bytes[$i] !== "\r" || $bytes[$i + 1] !== "\n") {
                continue;
            }

            $out[] = substr($bytes, $start, $i - $start);
            ++$i;
            $start = $i + 1;
        }

        return $out;
    }
}
