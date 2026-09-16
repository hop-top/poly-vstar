<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

/**
 * The PHP parity emitter for the cross-language parity harness.
 *
 * It takes the `spec/` directory as its single argument, runs the V*
 * public API over every fixture in `spec/v1.0/conformance/` and
 * `spec/behavior/`, and prints ONE JSON document to stdout.
 * `tools/parity/parity.py` diffs this document against the Go
 * reference's, key by key; any difference fails the run.
 *
 * The normative description of the document -- every key, every value
 * shape, every ordering rule -- is `tools/parity/README.md`. This
 * command implements that document, not the Go emitter's source.
 *
 * Three rules shape every value:
 *
 *   - Nothing human-readable is emitted. Diagnostic messages and
 *     exception text reword between versions without the behavior
 *     changing. Codes, paths, severities, op kinds and failure-class
 *     tokens are the stable surface.
 *   - A failure serializes as `{"error": "<SentinelName>"}` using the
 *     Go sentinel's identifier as the cross-language token. This port
 *     carries those identifiers natively as
 *     `VstarException::sentinel()`, so classification is a lookup
 *     rather than a translation.
 *   - Output is deterministic. `json_encode` preserves insertion
 *     order rather than sorting, so every emitted map is assembled
 *     through {@see sortedMap()}; every list is built in an order the
 *     contract pins.
 *
 * Two PHP-specific hazards shape the encoding:
 *
 *   - An empty PHP array encodes as `[]`, a JSON array. Every value
 *     that must be a JSON object -- a caseless family key, a
 *     supersession projection with nothing superseded -- is wrapped in
 *     {@see asObject()} so it renders as `{}`.
 *   - Fixture keys carry `/` separators. `JSON_UNESCAPED_SLASHES`
 *     keeps them from rendering as `\/`, which would differ from the
 *     reference byte for byte.
 *
 * Usage:
 *
 *     php tools/parity.php ../spec
 *
 * Exits 0 having printed the document; exits 1 with a diagnostic on
 * stderr when a fixture cannot be read or a contract it depends on is
 * broken.
 */

namespace HopTop\Vstar\Tools\Parity;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Canonical\Canonical;
use HopTop\Vstar\Card;
use HopTop\Vstar\Codec\Rfc5545\Parser as IcsParser;
use HopTop\Vstar\Codec\Rfc6350\Encoder as VcfEncoder;
use HopTop\Vstar\Codec\Rfc6350\Parser as VcfParser;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Diff\ComponentDiff;
use HopTop\Vstar\Diff\Diff;
use HopTop\Vstar\Diff\DiffOp;
use HopTop\Vstar\Diff\PropertyDiff;
use HopTop\Vstar\Duration\Duration;
use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\VstarException;
use HopTop\Vstar\Ext\Ext;
use HopTop\Vstar\Hashing\Hashing;
use HopTop\Vstar\Helpers\Helpers;
use HopTop\Vstar\Param;
use HopTop\Vstar\Rrule\Rrule;
use HopTop\Vstar\Rrule\Rule;
use HopTop\Vstar\Rrule\RuleSet;
use HopTop\Vstar\Supersession\Supersession;
use HopTop\Vstar\Time;
use HopTop\Vstar\Validate\Validate;

require __DIR__ . '/../vendor/autoload.php';

/** Fixture extensions, named once so the families read alike. */
const EXT_ICS = '.ics';
const EXT_VCF = '.vcf';
const EXT_RRULE = '.rrule';

/**
 * The escape hatch for a failure matching no class the family
 * recognizes. It surfaces under a name no port implements -- a loud
 * mismatch -- rather than silently under a wrong token.
 */
const CLASS_UNCLASSIFIED = 'Unclassified';

/** The general class every codec layer may wrap; always tried last. */
const CLASS_MALFORMED = 'ErrMalformed';

/**
 * The vocabulary a `malformed/*` fixture may produce, most specific
 * first. The emitter reports which one the fixture actually produced;
 * this list only bounds what behavior is recognized.
 *
 * @var list<string>
 */
const MALFORMED_CLASSES = [
    'ErrUnsupportedVersion',
    'ErrUnclosedBlock',
    'ErrMissingUID',
    CLASS_MALFORMED,
];

/**
 * The failure vocabulary the rrule family reports, most specific
 * first. Every token is a class named by spec/05 §Failure classes.
 *
 * @var list<string>
 */
const RRULE_CLASSES = [
    'ErrUnsupportedRRule',
    'ErrIterationCap',
    'ErrUnboundedExpansion',
    CLASS_MALFORMED,
];

/**
 * The failure vocabulary alarm resolution reports, most specific
 * first. An implementation that also wraps `ErrMalformed` around one
 * of these must still report the specific one, which is what
 * first-match ordering guarantees.
 *
 * @var list<string>
 */
const TRIGGER_CLASSES = ['ErrNoTrigger', 'ErrNoAnchor', CLASS_MALFORMED];

/**
 * The vocabulary `Duration::parse` failures report. Every failure is
 * `ErrMalformed` today; classifying rather than assuming means a class
 * the port grows surfaces as `Unclassified` instead of traveling as a
 * wrong token.
 *
 * @var list<string>
 */
const DURATION_CLASSES = [CLASS_MALFORMED];

/** The conformance subdirectories whose `.ics` files parse into a Calendar. */
const CALENDAR_DIRS = ['rfc5545', 'supersession'];

/** Abort with a diagnostic on stderr, mirroring the reference's exit 1. */
function fail(string $message): never
{
    fwrite(STDERR, "parity: {$message}\n");
    exit(1);
}

/**
 * Classify a thrown value into its cross-language token, first-match
 * over `$table` ordered specific to general.
 *
 * The Go reference matches with `errors.Is`, which walks a wrap chain;
 * this port raises a single {@see VstarException} whose `sentinel()`
 * already names the class, so membership in the table is the whole
 * test. A failure that is not a `VstarException`, or whose sentinel the
 * family does not list, reports null and the caller turns that into a
 * diagnostic rather than an emitted token no port would recognize.
 *
 * @param list<string> $table
 */
function classify(?\Throwable $err, array $table): ?string
{
    if (!$err instanceof VstarException) {
        return null;
    }

    $token = $err->sentinel();

    return in_array($token, $table, true) ? $token : null;
}

/**
 * Classify a failure, falling back to {@see CLASS_UNCLASSIFIED}.
 *
 * Used by the families whose contract names that token -- duration
 * parsing and alarm resolution -- where an unrecognized class travels
 * into the document instead of aborting the emitter.
 *
 * @param list<string> $table
 */
function classifyOrUnclassified(\Throwable $err, array $table): string
{
    return classify($err, $table) ?? CLASS_UNCLASSIFIED;
}

/**
 * Rebuild `$map` with its keys in sorted order.
 *
 * Go's `encoding/json` sorts map keys; `json_encode` emits an array's
 * own insertion order. Every emitted map therefore passes through
 * here, so the two documents compare byte-for-byte rather than merely
 * key-for-key.
 *
 * `strcmp` is byte order, which is what Go's `sort.Strings` produces
 * over the same ASCII keys. PHP's default array sort would apply
 * numeric-string coercion, which reorders keys that look like numbers.
 *
 * @template T
 *
 * @param array<string, T> $map
 *
 * @return array<string, T>
 */
function sortedMap(array $map): array
{
    uksort($map, static fn (string $a, string $b): int => strcmp($a, $b));

    return $map;
}

/**
 * Mark a string-keyed map so `json_encode` renders it as a JSON
 * object even when it is empty.
 *
 * PHP cannot tell an empty list from an empty map -- both are `[]` --
 * and the contract requires `{}` for a family key with no cases and
 * for a supersession projection with nothing superseded. Casting to
 * `stdClass` settles it for both the empty and non-empty case, so no
 * call site has to reason about which it holds.
 *
 * @param array<string, mixed> $map
 */
function asObject(array $map): \stdClass
{
    return (object) $map;
}

/** Whether `$path` exists, used to test for a sidecar. */
function exists(string $path): bool
{
    return file_exists($path);
}

/**
 * Call `$fn` for every non-directory entry of `$dir` carrying `$ext`,
 * in sorted order, with the full path and the extension-stripped stem.
 *
 * A missing directory is a silent no-op, so the emitter tolerates a
 * corpus that has yet to grow a family.
 *
 * @param callable(string, string): void $fn
 */
function eachFixture(string $dir, string $ext, callable $fn): void
{
    $names = @scandir($dir);

    if ($names === false) {
        return;
    }

    $matching = [];

    foreach ($names as $name) {
        if (!str_ends_with($name, $ext)) {
            continue;
        }

        $full = $dir . DIRECTORY_SEPARATOR . $name;

        if (is_dir($full)) {
            continue;
        }

        $matching[] = $full;
    }

    usort($matching, static fn (string $a, string $b): int => strcmp($a, $b));

    foreach ($matching as $full) {
        $fn($full, substr($full, 0, strlen($full) - strlen($ext)));
    }
}

/**
 * Every file under `$root`, recursively, in a deterministic order.
 *
 * The rrule corpus is the one nested tree; sorting the rendered paths
 * keeps the walk independent of the platform's directory order.
 *
 * @return list<string>
 */
function walkFiles(string $root): array
{
    $out = [];
    $names = @scandir($root);

    if ($names === false) {
        return $out;
    }

    foreach ($names as $name) {
        if ($name === '.' || $name === '..') {
            continue;
        }

        $full = $root . DIRECTORY_SEPARATOR . $name;

        if (is_dir($full)) {
            foreach (walkFiles($full) as $nested) {
                $out[] = $nested;
            }

            continue;
        }

        $out[] = $full;
    }

    usort($out, static fn (string $a, string $b): int => strcmp($a, $b));

    return $out;
}

/**
 * Render `$stem` as a slash-separated path relative to `$root`: the
 * document's key space is filesystem layout with the extension
 * dropped, identical on every platform a port runs on.
 */
function fixtureKey(string $root, string $stem): string
{
    $root = rtrim($root, DIRECTORY_SEPARATOR);

    if (!str_starts_with($stem, $root . DIRECTORY_SEPARATOR)) {
        fail("cannot relativize {$stem} against {$root}");
    }

    $rel = substr($stem, strlen($root) + 1);

    return str_replace(DIRECTORY_SEPARATOR, '/', $rel);
}

/**
 * Key `$value` by `$stem` relative to `$root`, refusing a collision.
 *
 * Two fixtures colliding on one key would silently drop a case from
 * the document and shrink the harness' coverage without failing it.
 *
 * @param array<string, mixed> $out
 */
function record(array &$out, string $root, string $stem, mixed $value): void
{
    $key = fixtureKey($root, $stem);

    if (array_key_exists($key, $out)) {
        fail('duplicate fixture key ' . json_encode($key, JSON_UNESCAPED_SLASHES));
    }

    $out[$key] = $value;
}

/** Read a fixture as raw bytes. */
function readBytes(string $path): string
{
    $raw = @file_get_contents($path);

    if ($raw === false) {
        fail("read {$path}");
    }

    return $raw;
}

/**
 * Read and decode a JSON sidecar as an associative array.
 *
 * @return array<array-key, mixed>
 */
function readJson(string $path): array
{
    $decoded = json_decode(readBytes($path), true);

    if (!is_array($decoded)) {
        fail("parse {$path}: not a JSON array or object");
    }

    return $decoded;
}

/** Parse an `.ics` fixture into a Calendar, failing the emitter on error. */
function readCalendar(string $path): Calendar
{
    try {
        return IcsParser::parse(readBytes($path));
    } catch (\Throwable $err) {
        fail("parse {$path}: " . $err->getMessage());
    }
}

// --- conformance ----------------------------------------------------

/**
 * Hash canonical bytes after folding CRLF to LF.
 *
 * The canonical form per spec/03 is CRLF, but the corpus stores it LF
 * and every port reads the same LF file. Digesting the LF form keeps
 * line-ending handling from producing false mismatches between ports.
 */
function canonicalDigest(string $bytes): string
{
    return hash('sha256', str_replace("\r\n", "\n", $bytes));
}

/**
 * Compare a computed hash against the committed `<stem>.hash` sibling.
 *
 * This self-check is what keeps a port from emitting a hash that is
 * merely self-consistent: the corpus carries an independent
 * expectation for this one family, and disagreeing with it is a broken
 * port rather than a parity mismatch to report downstream.
 */
function checkHashSibling(string $path, string $ext, string $got): void
{
    $sibling = substr($path, 0, strlen($path) - strlen($ext)) . '.hash';
    $want = rtrim(readBytes($sibling), "\r\n");

    if ($got !== $want) {
        fail("{$sibling}: hash {$got} does not match committed {$want}");
    }
}

/**
 * Parse an `.ics` fixture, canonicalize, hash, and self-check.
 *
 * @return array{hash: string, canonical_sha256: string}
 */
function calendarEntry(string $path): array
{
    try {
        $cal = IcsParser::parse(readBytes($path));
    } catch (\Throwable $err) {
        fail("parse {$path}: " . $err->getMessage());
    }

    $entry = [
        'hash' => Hashing::calendar($cal),
        'canonical_sha256' => canonicalDigest(Canonical::calendar($cal)),
    ];
    checkHashSibling($path, EXT_ICS, $entry['hash']);

    return $entry;
}

/**
 * {@see calendarEntry()} for a `.vcf` fixture. The corpus holds exactly
 * one card per file; more would mean the fixture changed shape under
 * the emitter and the contract no longer says which card the hash
 * belongs to.
 *
 * @return array{hash: string, canonical_sha256: string}
 */
function cardEntry(string $path): array
{
    try {
        $cards = VcfParser::parse(readBytes($path));
    } catch (\Throwable $err) {
        fail("parse {$path}: " . $err->getMessage());
    }

    if (count($cards) !== 1) {
        fail("{$path}: expected exactly one card, got " . count($cards));
    }

    $card = $cards[0];
    $entry = [
        'hash' => Hashing::card($card),
        'canonical_sha256' => canonicalDigest(Canonical::card($card)),
    ];
    checkHashSibling($path, EXT_VCF, $entry['hash']);

    return $entry;
}

/**
 * Run a malformed fixture through the codec and report the token it
 * produced.
 *
 * Most fixtures fail at parse time. `ErrMissingUID` is encoder-only at
 * v1.0 -- the rfc6350 parser accepts a UID-less VCARD and the encoder
 * refuses it -- so a fixture that parses is re-encoded and the encode
 * failure classified instead. A fixture where both stages succeed is a
 * fault.
 */
function malformedToken(string $path, string $ext): string
{
    $input = readBytes($path);
    $parseErr = null;
    $encodeErr = null;
    $parsed = false;

    if ($ext === EXT_ICS) {
        try {
            IcsParser::parse($input);
            $parsed = true;
        } catch (\Throwable $err) {
            $parseErr = $err;
        }
    } else {
        $cards = null;

        try {
            $cards = VcfParser::parse($input);
            $parsed = true;
        } catch (\Throwable $err) {
            $parseErr = $err;
        }

        if ($cards !== null) {
            if (count($cards) === 0) {
                fail("{$path}: parse returned no cards and no error");
            }

            try {
                foreach ($cards as $card) {
                    VcfEncoder::encode($card);
                }
            } catch (\Throwable $err) {
                $encodeErr = $err;
                $parsed = false;
            }
        }
    }

    $token = classify($parseErr, MALFORMED_CLASSES) ?? classify($encodeErr, MALFORMED_CLASSES);

    if ($token !== null) {
        return $token;
    }

    if ($parsed) {
        fail("{$path}: expected a failure, parse and encode both succeeded");
    }

    $err = $parseErr ?? $encodeErr;

    fail("{$path}: failure matches no known sentinel: " . ($err === null ? '<none>' : $err->getMessage()));
}

/**
 * Walk the conformance corpus, keying every fixture by its path
 * relative to the corpus root without extension.
 *
 * `time/` fixtures are VTIMEZONE registries the `time` family
 * consumes and carry no `.canonical`/`.hash` siblings, so they are not
 * part of this family; `fuzz-seed/` is fuzz-target input.
 *
 * @return array<string, mixed>
 */
function emitConformance(string $corpus): array
{
    $out = [];

    foreach (CALENDAR_DIRS as $dir) {
        eachFixture(
            $corpus . DIRECTORY_SEPARATOR . $dir,
            EXT_ICS,
            static function (string $path, string $stem) use (&$out, $corpus): void {
                record($out, $corpus, $stem, calendarEntry($path));
            },
        );
    }

    eachFixture(
        $corpus . DIRECTORY_SEPARATOR . 'rfc6350',
        EXT_VCF,
        static function (string $path, string $stem) use (&$out, $corpus): void {
            record($out, $corpus, $stem, cardEntry($path));
        },
    );

    $malformed = $corpus . DIRECTORY_SEPARATOR . 'malformed';

    foreach ([EXT_ICS, EXT_VCF] as $ext) {
        eachFixture(
            $malformed,
            $ext,
            static function (string $path, string $stem) use (&$out, $corpus, $ext): void {
                record($out, $corpus, $stem, ['error' => malformedToken($path, $ext)]);
            },
        );
    }

    return sortedMap($out);
}

// --- rrule ----------------------------------------------------------

/**
 * Read a sidecar's input fields, reporting null when absent.
 *
 * @return array<array-key, mixed>|null
 */
function readSidecar(string $path): ?array
{
    return exists($path) ? readJson($path) : null;
}

/** Read an RFC 5545 form #2 sidecar input value. */
function parseStamp(string $path, string $field, mixed $value): \DateTimeImmutable
{
    $t = is_string($value) ? Time::parseTime($value) : null;

    if ($t === null) {
        fail("{$path}: bad {$field} " . json_encode(is_string($value) ? $value : '', JSON_UNESCAPED_SLASHES));
    }

    return $t;
}

/**
 * Classify a failure into its rrule class token, refusing to emit
 * anything for a success or an unrecognized class. An unrecognized
 * class must fail the emitter rather than travel into the document as
 * a token no port implements.
 */
function failureToken(string $path, ?\Throwable $err, bool $succeeded): string
{
    if ($succeeded) {
        fail("{$path}: expected a failure, the call succeeded");
    }

    $token = classify($err, RRULE_CLASSES);

    if ($token === null) {
        fail("{$path}: failure matches no known class: " . ($err === null ? '<none>' : $err->getMessage()));
    }

    return $token;
}

/**
 * Render occurrences as RFC 5545 form #2, the one timestamp shape
 * every family and every port uses.
 *
 * @param list<\DateTimeImmutable> $times
 *
 * @return list<string>
 */
function formatStamps(array $times): array
{
    $out = [];

    foreach ($times as $t) {
        $out[] = Time::formatTime($t);
    }

    return $out;
}

/**
 * Record a bounded-expansion outcome on `$entry`: the occurrence list
 * under `$key` plus a `<key>_complete` flag on success, or
 * `<key>_error` naming the failure class.
 *
 * Every key is flat on the entry rather than nested under one
 * sub-object, so which contracts a fixture pins is readable off the
 * entry's key set.
 *
 * @param array<string, mixed>                                       $entry
 * @param callable(): array{occurrences: list<\DateTimeImmutable>, complete: bool} $run
 */
function addBounded(array &$entry, string $path, string $key, callable $run): void
{
    try {
        $result = $run();
    } catch (\Throwable $err) {
        $entry[$key . '_error'] = failureToken($path, $err, false);

        return;
    }

    $entry[$key] = formatStamps($result['occurrences']);
    $entry[$key . '_complete'] = $result['complete'];
}

/**
 * Report the rule's wire form when a `<stem>.formatted` sidecar exists.
 *
 * `Rule` is a class rather than an enum, so it keeps `__toString()`;
 * the enums in this port spell the reference's `String()` as
 * `toString()` because PHP rejects the magic method on an enum.
 *
 * @param array<string, mixed> $entry
 */
function addFormatted(string $stem, Rule $rule, array &$entry): void
{
    if (!exists($stem . '.formatted')) {
        return;
    }

    $entry['formatted'] = (string) $rule;
}

/**
 * Walk `nextOccurrence` the way the sidecar's expected list is shaped:
 * one step per entry, each feeding its result back as the next
 * `after`. The emitted list is what this port yielded.
 *
 * One extra step runs past the end, and its outcome is the terminal
 * contract: `next_error` names the class the series failed with, or
 * `next_complete` states whether it terminated. Emitting that step
 * unconditionally means a port cannot pass by stopping early -- a
 * series that should terminate and one that should raise
 * `ErrIterationCap` differ in the document.
 *
 * @param array<string, mixed> $entry
 */
function addNext(string $stem, Rule $rule, array &$entry): void
{
    $path = $stem . '.next.json';
    $spec = readSidecar($path);

    if ($spec === null) {
        return;
    }

    $dt = parseStamp($path, 'dtstart', $spec['dtstart'] ?? null);
    $after = parseStamp($path, 'after', $spec['after'] ?? null);

    $expected = $spec['expected'] ?? [];
    $steps = is_array($expected) ? count($expected) : 0;

    $stamps = [];

    for ($i = 0; $i < $steps; ++$i) {
        try {
            $got = Rrule::nextOccurrence($rule, $dt, $after);
        } catch (\Throwable $err) {
            fail("{$path}: nextOccurrence step {$i} failed: " . $err->getMessage());
        }

        if ($got === null) {
            fail("{$path}: nextOccurrence step {$i} terminated early");
        }

        $stamps[] = Time::formatTime($got);
        $after = $got;
    }

    $entry['next'] = $stamps;

    try {
        $entry['next_complete'] = Rrule::nextOccurrence($rule, $dt, $after) === null;
    } catch (\Throwable $err) {
        $entry['next_error'] = failureToken($path, $err, false);
    }
}

/**
 * Report bounded expansion over the `.expand.json` sidecar's limit.
 *
 * @param array<string, mixed> $entry
 */
function addExpand(string $stem, Rule $rule, array &$entry): void
{
    $path = $stem . '.expand.json';
    $spec = readSidecar($path);

    if ($spec === null) {
        return;
    }

    $dt = parseStamp($path, 'dtstart', $spec['dtstart'] ?? null);
    $limit = $spec['limit'] ?? 0;
    $limit = is_int($limit) ? $limit : 0;

    addBounded($entry, $path, 'expand', static fn (): array => Rrule::occurrences($rule, $dt, $limit));
}

/**
 * Report `between` over the sidecar's half-open window. `between` has
 * no completeness notion -- the window bounds the answer -- so the
 * success shape is the list alone.
 *
 * @param array<string, mixed> $entry
 */
function addBetween(string $stem, Rule $rule, array &$entry): void
{
    $path = $stem . '.between.json';
    $spec = readSidecar($path);

    if ($spec === null) {
        return;
    }

    $dt = parseStamp($path, 'dtstart', $spec['dtstart'] ?? null);
    $start = parseStamp($path, 'start', $spec['start'] ?? null);
    $end = parseStamp($path, 'end', $spec['end'] ?? null);

    try {
        $entry['between'] = formatStamps(Rrule::between($rule, $dt, $start, $end));
    } catch (\Throwable $err) {
        $entry['between_error'] = failureToken($path, $err, false);
    }
}

/**
 * Evaluate one `<stem>.rrule` fixture against every sidecar it has.
 *
 * A `.expect.json` sidecar means the rule must be rejected: the
 * emitter reports the class `Rrule::validate` produced and stops, since
 * no other contract applies to a rule that does not parse.
 *
 * @return array<string, mixed>
 */
function ruleEntry(string $path): array
{
    $value = rtrim(readBytes($path), "\r\n");
    $stem = substr($path, 0, strlen($path) - strlen(EXT_RRULE));

    if (exists($stem . '.expect.json')) {
        $succeeded = false;
        $err = null;

        try {
            Rrule::validate($value);
            $succeeded = true;
        } catch (\Throwable $e) {
            $err = $e;
        }

        return ['error' => failureToken($path, $err, $succeeded)];
    }

    try {
        $rule = Rrule::parse($value);
    } catch (\Throwable $err) {
        fail("{$path}: parse failed: " . $err->getMessage());
    }

    $entry = ['parsed' => true];
    addFormatted($stem, $rule, $entry);
    addNext($stem, $rule, $entry);
    addExpand($stem, $rule, $entry);
    addBetween($stem, $rule, $entry);

    return sortedMap($entry);
}

/**
 * Evaluate one `<stem>.ics` recurrence-set fixture: the calendar's
 * first component goes through `RuleSet::fromComponent`, then either
 * `.expect.json` pins a rejection or `.occurrences.json` pins the
 * bounded expansion.
 *
 * @return array<string, mixed>
 */
function setEntry(string $path): array
{
    $stem = substr($path, 0, strlen($path) - strlen(EXT_ICS));
    $input = readBytes($path);

    $set = null;
    $setErr = null;

    try {
        $cal = IcsParser::parse($input);

        if (count($cal->components) === 0) {
            throw new MalformedException('calendar has no components');
        }

        $set = RuleSet::fromComponent($cal->components[0]);
    } catch (\Throwable $err) {
        $setErr = $err;
    }

    if (exists($stem . '.expect.json')) {
        return ['error' => failureToken($path, $setErr, $set !== null)];
    }

    if ($set === null) {
        fail("{$path}: " . $setErr->getMessage());
    }

    $entry = ['parsed' => true];
    $occPath = $stem . '.occurrences.json';
    $spec = readSidecar($occPath);

    if ($spec === null) {
        return $entry;
    }

    $limit = $spec['limit'] ?? 0;
    $limit = is_int($limit) ? $limit : 0;

    addBounded($entry, $occPath, 'occurrences', static fn (): array => $set->occurrences($limit));

    return sortedMap($entry);
}

/**
 * Walk the rrule corpus and emit one object per fixture stem, keyed by
 * the stem's path relative to the conformance root.
 *
 * @return array<string, mixed>
 */
function emitRRule(string $root): array
{
    $corpus = dirname($root);
    $out = [];

    foreach (walkFiles($root) as $path) {
        if (str_ends_with($path, EXT_RRULE)) {
            record($out, $corpus, substr($path, 0, strlen($path) - strlen(EXT_RRULE)), ruleEntry($path));
        } elseif (str_ends_with($path, EXT_ICS)) {
            record($out, $corpus, substr($path, 0, strlen($path) - strlen(EXT_ICS)), setEntry($path));
        }
    }

    return sortedMap($out);
}

// --- validate -------------------------------------------------------

/**
 * Run `Validate::validate` over every `<name>.ics` and emit the
 * diagnostics it raises, sorted by (path, code).
 *
 * The sort is the contract, not any port's emission order: checks run
 * in an order that is an implementation detail. Sorting both sides
 * makes the comparison about which diagnostics were raised.
 *
 * @return array<string, mixed>
 */
function emitValidate(string $dir): array
{
    $root = dirname($dir);
    $out = [];

    eachFixture($dir, EXT_ICS, static function (string $path, string $stem) use (&$out, $root): void {
        $entries = [];

        foreach (Validate::validate(readCalendar($path)) as $d) {
            $entries[] = [
                'code' => $d->code,
                'severity' => $d->severity->value,
                'path' => $d->path,
            ];
        }

        usort($entries, static function (array $a, array $b): int {
            return strcmp($a['path'], $b['path']) ?: strcmp($a['code'], $b['code']);
        });

        record($out, $root, $stem, $entries);
    });

    return sortedMap($out);
}

// --- diff -----------------------------------------------------------

/**
 * Project a property's parameters onto the emitted shape.
 *
 * @param list<Param> $params
 *
 * @return list<array{name: string, value: string}>
 */
function paramEntries(array $params): array
{
    $out = [];

    foreach ($params as $p) {
        $out[] = ['name' => $p->name, 'value' => $p->value];
    }

    return $out;
}

/**
 * Lift the uid out of a rendered path such as
 * `VCALENDAR.VTODO[uid=todo-1]`. Empty for a positional path
 * (`VCALENDAR.VALARM[#0]`), which has no UID to key on.
 */
function uidFromPath(string $path): string
{
    $marker = '[uid=';
    $i = strrpos($path, $marker);

    if ($i === false || !str_ends_with($path, ']')) {
        return '';
    }

    return substr($path, $i + strlen($marker), strlen($path) - $i - strlen($marker) - 1);
}

/** Report the name the op is about, from whichever side carries it. */
function propertyName(PropertyDiff $pd): string
{
    if ($pd->property->name !== '') {
        return $pd->property->name;
    }

    return $pd->old->name ?? '';
}

/**
 * @param list<PropertyDiff> $pds
 *
 * @return list<array<string, mixed>>
 */
function opEntries(array $pds): array
{
    $out = [];

    foreach ($pds as $pd) {
        $entry = [
            'op' => $pd->op->value,
            'property' => propertyName($pd),
            'before' => null,
            'after' => null,
            'before_params' => [],
            'after_params' => [],
        ];

        switch ($pd->op) {
            case DiffOp::Added:
                $entry['after'] = $pd->property->value;
                $entry['after_params'] = paramEntries($pd->property->params);

                break;
            case DiffOp::Removed:
                $entry['before'] = $pd->property->value;
                $entry['before_params'] = paramEntries($pd->property->params);

                break;
            case DiffOp::Changed:
                $entry['before'] = $pd->old->value ?? '';
                $entry['after'] = $pd->property->value;
                $entry['before_params'] = paramEntries($pd->old->params ?? []);
                $entry['after_params'] = paramEntries($pd->property->params);

                break;
        }

        $out[] = $entry;
    }

    return $out;
}

/**
 * Drop sub-diffs that record no change. `ofCalendar` filters these at
 * the top level but carries them nested; the document reports only
 * real changes.
 *
 * @param list<ComponentDiff> $ds
 *
 * @return list<array<string, mixed>>
 */
function componentDiffEntries(array $ds): array
{
    $out = [];

    foreach ($ds as $d) {
        $changed = array_values(array_filter(
            $d->subDiffs,
            static fn (ComponentDiff $s): bool => !$s->isEmpty(),
        ));

        $out[] = [
            'uid' => uidFromPath($d->path),
            'path' => $d->path,
            'ops' => opEntries($d->properties),
            'subs' => componentDiffEntries($changed),
        ];
    }

    return $out;
}

/**
 * Run `Diff::ofCalendar` over every `<name>.a.ics` / `<name>.b.ics`
 * pair.
 *
 * No sorting happens here. `ofCalendar` emits components in pairing
 * order and properties sorted by name case-insensitively, and that
 * ordering is the contract a port reproduces -- sorting it again would
 * hide an ordering divergence rather than catch it.
 *
 * @return array<string, mixed>
 */
function emitDiff(string $dir): array
{
    $root = dirname($dir);
    $out = [];

    eachFixture($dir, '.a' . EXT_ICS, static function (string $path, string $stem) use (&$out, $root): void {
        $a = readCalendar($path);
        $b = readCalendar($stem . '.b' . EXT_ICS);
        record($out, $root, $stem, componentDiffEntries(Diff::ofCalendar($a, $b)));
    });

    return sortedMap($out);
}

// --- supersession ---------------------------------------------------

/**
 * Project the effective status each supersession ledger imposes, keyed
 * by component UID.
 *
 * The inputs are the conformance corpus' supersession fixtures; the
 * behavior tree holds only the `<name>.effective.json` sidecars, so
 * the sidecar names which `.ics` to load. A UID absent from the map is
 * not superseded -- supersession is a projection query, not a
 * validator, so a fixture with nothing superseded is `{}`.
 *
 * @return array<string, mixed>
 */
function emitSupersession(string $conformance, string $dir): array
{
    $root = dirname($dir);
    $inputs = $conformance . DIRECTORY_SEPARATOR . 'supersession';
    $out = [];

    eachFixture(
        $dir,
        '.effective.json',
        static function (string $_path, string $stem) use (&$out, $root, $inputs): void {
            $name = basename($stem);
            $cal = readCalendar($inputs . DIRECTORY_SEPARATOR . $name . EXT_ICS);
            $effective = [];

            foreach ($cal->components as $c) {
                $status = Supersession::superseded($c, $cal->components);

                if ($status === null) {
                    continue;
                }

                $uid = $c->uid();

                if ($uid === '') {
                    fail("{$name}: superseded component has no UID");
                }

                $effective[$uid] = $status;
            }

            record($out, $root, $stem, asObject(sortedMap($effective)));
        },
    );

    return sortedMap($out);
}

// --- duration -------------------------------------------------------

/**
 * Report what `Duration::parse` makes of one value: the signed second
 * count and sign flag, or the failure class.
 *
 * `VDuration::totalSeconds()` is the second count directly, where the
 * Go reference divides a nanosecond `Signed()` down; the emitted unit
 * is the same.
 *
 * @return array{value: string, seconds: int|null, negative: bool|null, error: string}
 */
function parseDuration(string $value): array
{
    try {
        $d = Duration::parse($value);

        return [
            'value' => $value,
            'seconds' => $d->totalSeconds(),
            'negative' => $d->isNegative(),
            'error' => '',
        ];
    } catch (\Throwable $err) {
        return [
            'value' => $value,
            'seconds' => null,
            'negative' => null,
            'error' => classifyOrUnclassified($err, DURATION_CLASSES),
        ];
    }
}

/**
 * Resolve every VALARM in the calendar, in document order: parent
 * components in calendar order, VALARMs in the order they appear
 * inside their parent.
 *
 * @return list<array{alarm_uid: string, fires_at: string, error: string}>
 */
function alarmEntries(Calendar $cal): array
{
    $out = [];

    foreach ($cal->components as $parent) {
        foreach ($parent->sub as $alarm) {
            if ($alarm->type !== CompType::Alarm->value) {
                continue;
            }

            $out[] = resolveAlarm($alarm, $parent, $cal);
        }
    }

    return $out;
}

/**
 * @return array{alarm_uid: string, fires_at: string, error: string}
 */
function resolveAlarm(Component $alarm, Component $parent, Calendar $cal): array
{
    $alarmUid = $alarm->uid();

    try {
        return [
            'alarm_uid' => $alarmUid,
            'fires_at' => Time::formatTime(Helpers::alarmFiresAt($alarm, $parent, $cal)),
            'error' => '',
        ];
    } catch (\Throwable $err) {
        return [
            'alarm_uid' => $alarmUid,
            'fires_at' => '',
            'error' => classifyOrUnclassified($err, TRIGGER_CLASSES),
        ];
    }
}

/**
 * Cover both duration families: the flat parse table at
 * `duration/parse.json`, and one alarm-resolution list per
 * `<name>.ics`.
 *
 * The parse table's inputs come from the committed fixture's `value`
 * column rather than a list hard-coded here, so a value added to the
 * corpus reaches every port without an emitter edit.
 *
 * @return array<string, mixed>
 */
function emitDuration(string $dir): array
{
    $root = dirname($dir);
    $out = [];

    $rows = readJson($dir . DIRECTORY_SEPARATOR . 'parse.json');
    $parsed = [];

    foreach ($rows as $row) {
        $value = is_array($row) ? ($row['value'] ?? '') : '';
        $parsed[] = parseDuration(is_string($value) ? $value : '');
    }

    $out['duration/parse'] = $parsed;

    eachFixture($dir, EXT_ICS, static function (string $path, string $stem) use (&$out, $root): void {
        record($out, $root, $stem, alarmEntries(readCalendar($path)));
    });

    return sortedMap($out);
}

// --- ext ------------------------------------------------------------

/**
 * Classify every name in the committed `ext/scopes.json`, in the order
 * the file lists them. The file is a flat table, not a set: order is
 * part of what a port reproduces.
 *
 * The scope token is the enum's lowercase backing value, which is the
 * wire token the contract names; `Scope::toString()` is the
 * capitalized display spelling and is not what travels here.
 *
 * @return array<string, mixed>
 */
function emitExt(string $dir): array
{
    $rows = readJson($dir . DIRECTORY_SEPARATOR . 'scopes.json');
    $entries = [];

    foreach ($rows as $row) {
        $name = is_array($row) ? ($row['name'] ?? '') : '';
        $name = is_string($name) ? $name : '';

        $entries[] = [
            'name' => $name,
            'scope' => Ext::scopeOf($name)->value,
            'system' => Ext::systemName($name),
        ];
    }

    return ['ext/scopes' => $entries];
}

// --- time -----------------------------------------------------------

/**
 * Resolve every (calendar, tzid, value) triple in the committed
 * `time/tzid.json`, in file order.
 *
 * `calendar` names a conformance fixture supplying the VTIMEZONE
 * registry. Each registry is parsed once and reused, so a fixture's
 * cost does not grow with the number of rows citing it.
 *
 * @return array<string, mixed>
 */
function emitTime(string $conformance, string $dir): array
{
    $rows = readJson($dir . DIRECTORY_SEPARATOR . 'tzid.json');

    /** @var array<string, Calendar> $registries */
    $registries = [];
    $entries = [];

    foreach ($rows as $row) {
        if (!is_array($row)) {
            continue;
        }

        $calendar = is_string($row['calendar'] ?? null) ? $row['calendar'] : '';
        $tzid = is_string($row['tzid'] ?? null) ? $row['tzid'] : '';
        $value = is_string($row['value'] ?? null) ? $row['value'] : '';

        if (!isset($registries[$calendar])) {
            $registries[$calendar] = readCalendar(
                $conformance . DIRECTORY_SEPARATOR . 'time' . DIRECTORY_SEPARATOR . $calendar . EXT_ICS,
            );
        }

        $got = Time::parseTimeWithTzid($value, $tzid, $registries[$calendar]);

        $entries[] = [
            'calendar' => $calendar,
            'tzid' => $tzid,
            'value' => $value,
            'utc' => $got === null ? null : Time::formatTime($got),
        ];
    }

    return ['time/tzid' => $entries];
}

// --- main -----------------------------------------------------------

function main(): void
{
    $argv = $_SERVER['argv'] ?? [];
    $args = is_array($argv) ? array_slice($argv, 1) : [];

    if (count($args) !== 1 || !is_string($args[0])) {
        fail('usage: parity <spec-dir>');
    }

    $arg = $args[0];
    $spec = realpath($arg);

    if ($spec === false) {
        fail("not a directory: {$arg}");
    }

    $conformance = $spec . DIRECTORY_SEPARATOR . 'v1.0' . DIRECTORY_SEPARATOR . 'conformance';
    $behavior = $spec . DIRECTORY_SEPARATOR . 'behavior';

    foreach ([$conformance, $behavior] as $dir) {
        if (!is_dir($dir)) {
            fail("not a directory: {$dir}");
        }
    }

    // The eight top-level keys, in the order the contract lists them.
    // Their order in the emitted text does not matter to the harness,
    // which compares parsed documents, but keeping it matches the
    // reference's own struct order and makes the two files diffable by
    // eye. Every family is wrapped so a caseless one renders as `{}`
    // rather than PHP's `[]`.
    $document = [
        'conformance' => asObject(emitConformance($conformance)),
        'rrule' => asObject(emitRRule($conformance . DIRECTORY_SEPARATOR . 'rrule')),
        'validate' => asObject(emitValidate($behavior . DIRECTORY_SEPARATOR . 'validate')),
        'diff' => asObject(emitDiff($behavior . DIRECTORY_SEPARATOR . 'diff')),
        'supersession' => asObject(
            emitSupersession($conformance, $behavior . DIRECTORY_SEPARATOR . 'supersession'),
        ),
        'duration' => asObject(emitDuration($behavior . DIRECTORY_SEPARATOR . 'duration')),
        'ext' => asObject(emitExt($behavior . DIRECTORY_SEPARATOR . 'ext')),
        'time' => asObject(emitTime($conformance, $behavior . DIRECTORY_SEPARATOR . 'time')),
    ];

    $json = json_encode($document, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);

    if ($json === false) {
        fail('encode document: ' . json_last_error_msg());
    }

    echo $json, "\n";
}

main();
