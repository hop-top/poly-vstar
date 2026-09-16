# hop-top/vstar

Canonical calendar and contact interchange for agentic systems —
RFC 5545 and RFC 6350, with byte-stable output and a content hash.

[![Packagist](https://img.shields.io/packagist/v/hop-top/vstar?label=packagist)](https://packagist.org/packages/hop-top/vstar)
[![PHP](https://img.shields.io/packagist/dependency-v/hop-top/vstar/php?label=php)](https://packagist.org/packages/hop-top/vstar)
[![CI](https://img.shields.io/github/actions/workflow/status/hop-top/poly-vstar/ci-php.yml?branch=main&label=ci)](https://github.com/hop-top/poly-vstar/actions/workflows/ci-php.yml?query=branch%3Amain)
[![Types](https://img.shields.io/badge/phpstan-level%209-blue)](https://phpstan.org/)
[![Spec](https://img.shields.io/badge/spec-draft%20v0.1-blue)](https://github.com/hop-top/poly-vstar/tree/main/spec/)
[![License](https://img.shields.io/badge/license-MIT-green)](https://github.com/hop-top/poly-vstar/blob/main/LICENSE)

> **Read-only mirror.** This package is developed in the polyglot
> monorepo [`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar)
> under `php/` and republished on each release to
> [`hop-top/vstar-php`](https://github.com/hop-top/vstar-php), which is
> where Packagist reads it from. The mirror is created by the first
> release. Open issues and pull requests against the monorepo, not the
> mirror.

## Why

A generic iCalendar library will parse your `.ics` and hand you back a
tree. It will not tell you whether two documents mean the same thing —
because the RFCs let the same logical content be written many ways:
properties in any order, parameters in any order, datetimes in local or
UTC form, folding at any column. Serialize the same calendar twice and
you can get different bytes.

V\* pins that down. It defines a **canonical form** — one byte sequence
per logical content — and an `X-VSTAR-HASH` content hash over it, so
"did this change?" is a string comparison rather than a tree walk. On
top of that it adds the things agentic state actually needs: structural
`Diff`, append-only `Supersession` for state transitions, a diagnostic
`Validate` pass with stable codes, and a bounded `Rrule` evaluator.

Reach for this over a generic iCalendar library when you need any of:

- **Byte-identical output across runtimes.** This port is verified
  byte-for-byte against the Go reference over the whole shared
  conformance corpus — not merely self-consistent. See
  [Conformance](#conformance).
- **Content addressing.** A stable hash over canonical bytes, so
  documents can be deduplicated, cached, or compared across services.
- **Append-only state.** Supersession chains rather than mutation, so
  history survives.

**When not to use this:** if you need a full calendaring client —
timezone database management, free/busy scheduling across attendees,
CalDAV sync, or broad support for RFC 5545's long tail — use a general
iCalendar library such as `sabre/vobject`. V\* deliberately implements a
bounded subset chosen for machine-generated state, and its `RRULE` scope
excludes `FREQ=SECONDLY`/`MINUTELY` and `RSCALE`. If you simply want to
read someone else's calendar file, this is more machinery than you need.

## Install

The package ships alpha versions (`0.1.0-alpha.*`) until 1.0, and
Composer will not select an alpha tag unless the consuming project
allows that stability first:

```sh
composer config minimum-stability alpha
composer config prefer-stable true
composer require hop-top/vstar
```

The API may change between alpha tags; pin an exact version until 1.0.

Requirements:

- PHP 8.2 or newer (`"php": ">=8.2"`). CI tests 8.2 and 8.4.
- `ext-mbstring`, for the UTF-8 handling the codec and canonicalizer do.

## Usage

Parse, hash, canonicalize — the most common path:

```php
<?php

require 'vendor/autoload.php';

use HopTop\Vstar\Canonical\Canonical;
use HopTop\Vstar\Codec\Rfc5545\Parser;
use HopTop\Vstar\Hashing\Hashing;

$ics = implode("\r\n", [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//example//EN',
    'BEGIN:VTODO',
    'UID:todo-1',
    'DTSTAMP:20260101T000000Z',
    'DUE:20260102T000000Z',
    'SUMMARY:Ship the port',
    'END:VTODO',
    'END:VCALENDAR',
    '',
]);

$cal = Parser::parse($ics);

// Stable across producers that spell the same instant differently.
echo Hashing::calendar($cal), "\n";

// Canonical form is a BYTE sequence; compare it as one.
$bytes = Canonical::calendar($cal);
echo strlen($bytes), " bytes\n";
```

```text
sha256:e551d17793785d5876edc6e33bca47f2aae73eb9feac49678d76159a78c91b18
175 bytes
```

That hash is the same string the Go, TypeScript, Python and Rust ports
print for the same input — that is the point of the canonical form.

## API

Every class is `final` and every entry point is a static method on an
area class, so a call site names both the area and the operation. The
namespaces are PSR-4 mapped to `src/`; the full Go-to-PHP surface is in
the [API mapping](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/api-mapping.md).

| Namespace | Area |
| --- | --- |
| `HopTop\Vstar` | Data model (`Calendar`, `Card`, `Component`, `Property`, `Param`), enums, `Time` helpers |
| `HopTop\Vstar\Codec\Rfc5545` | iCalendar `Parser` and `Encoder` |
| `HopTop\Vstar\Codec\Rfc6350` | vCard `Parser` and `Encoder` |
| `HopTop\Vstar\Codec\Stream` | Incremental stream decoding and encoding |
| `HopTop\Vstar\Canonical` | Canonical byte form |
| `HopTop\Vstar\Hashing` | `X-VSTAR-HASH` compute and verify |
| `HopTop\Vstar\Validate` | Diagnostics with stable codes |
| `HopTop\Vstar\Rrule` | Recurrence parse and bounded expansion |
| `HopTop\Vstar\Duration` | ISO 8601 durations and alarm triggers |
| `HopTop\Vstar\Ext` | `X-*` extension namespaces |
| `HopTop\Vstar\Diff` | Structural diff |
| `HopTop\Vstar\Supersession` | Append-only state transitions |
| `HopTop\Vstar\Helpers` | Convenience constructors and accessors |
| `HopTop\Vstar\Exception` | The twelve failure classes |

The examples below continue from the `$ics` string in [Usage](#usage).

### Validate

Diagnostics carry a stable `code` and a dotted `path`. Match on the
code; `message` is prose and rewords between versions.

```php
use HopTop\Vstar\Codec\Rfc5545\Parser;
use HopTop\Vstar\Validate\Validate;

// The same VTODO as above, plus an X-VSTAR-HASH that is not its hash.
$cal = Parser::parse(str_replace(
    "END:VTODO",
    "X-VSTAR-HASH:sha256:" . str_repeat('0', 64) . "\r\nEND:VTODO",
    $ics,
));

foreach (Validate::validate($cal) as $d) {
    echo $d->severity->value, ' ', $d->code, ' ', $d->path, "\n";
}
```

```text
error VS010 VCALENDAR.VTODO[uid=todo-1].X-VSTAR-HASH
```

`VS010` is the hash being present and wrong; the same document with no
`X-VSTAR-HASH` at all reports `VS003` instead. Every code's severity is
fixed by the catalog — `Validate::severityOf('VS010')` returns
`Severity::Error`, and `null` for a code that is not in it.

`severity` is a `Severity` enum. Compare the case
(`$d->severity === Severity::Error`) rather than the string; `->value`
is the wire spelling, for output like the line above.

### Recurrence

Expansion is always bounded. `occurrences()` takes a limit and returns a
keyed record — the occurrences plus whether the series ended within the
limit — so an unbounded rule cannot hang a caller and the flag cannot be
silently dropped the way a positional pair invites.

```php
use HopTop\Vstar\Rrule\Rrule;
use HopTop\Vstar\Time;

$rule = Rrule::parse('FREQ=DAILY;COUNT=3');
$dtstart = Time::parseTime('20260401T120000Z');

['occurrences' => $times, 'complete' => $complete] = Rrule::occurrences($rule, $dtstart, 10);

foreach ($times as $t) {
    echo Time::formatTime($t), "\n";
}
var_dump($complete);
```

```text
20260401T120000Z
20260402T120000Z
20260403T120000Z
bool(true)
```

### Helpers

Constructors produce components that already carry the required common
properties — `UID`, a `DTSTAMP` of now, and an `X-VSTAR-HASH` over the
result.

```php
use HopTop\Vstar\Hashing\Hashing;
use HopTop\Vstar\Helpers\Helpers;
use HopTop\Vstar\Property;
use HopTop\Vstar\Time;

$todo = Helpers::newTodo('todo-9', Time::parseTime('20260501T090000Z'));

echo $todo->type, "\n";
echo $todo->uid(), "\n";

// The constructor stamps the current instant, so the hash it computed
// differs on every run. Freeze DTSTAMP and re-hash to get a stable one;
// outside a doc example, the fresh stamp is what you want.
$todo->set(new Property('DTSTAMP', [], '20260501T080000Z'));
Hashing::setXVstar($todo);

echo Hashing::getXVstar($todo), "\n";
var_dump(Hashing::verifyXVstar($todo));
```

```text
VTODO
todo-9
sha256:72cb993c6278718eae7a90d2cc925b0334510c5297bb9d943246eac4f2342ffa
array(3) {
  ["ok"]=>
  bool(true)
  ["want"]=>
  string(71) "sha256:72cb993c6278718eae7a90d2cc925b0334510c5297bb9d943246eac4f2342ffa"
  ["got"]=>
  string(71) "sha256:72cb993c6278718eae7a90d2cc925b0334510c5297bb9d943246eac4f2342ffa"
}
```

### Diff and supersession

When the hashes differ, `Diff` says what changed. When state changes,
`Supersession` records the transition as a new component that points
at the old one instead of mutating it, so the ledger stays append-only.

```php
use HopTop\Vstar\Codec\Rfc5545\Parser;
use HopTop\Vstar\Diff\Diff;
use HopTop\Vstar\Supersession\Supersession;
use HopTop\Vstar\Time;

$before = Parser::parse($ics);
$after = Parser::parse(str_replace(
    'SUMMARY:Ship the port',
    "SUMMARY:Ship the port\r\nPRIORITY:1",
    $ics,
));

var_dump(Diff::calendarEqual($before, $after));
foreach (Diff::ofCalendar($before, $after) as $componentDiff) {
    echo $componentDiff;
}

// Mark the todo COMPLETED without touching it: a new journal entry
// carries the effective status and RELATED-TO the original.
$todo = $before->components[0];
$entry = Supersession::supersedes($todo, 'COMPLETED', Time::parseTime('20260103T000000Z'));

echo $entry->uid(), "\n";
var_dump(Supersession::superseded($todo, [$entry]));
```

```text
bool(false)
--- VCALENDAR.VTODO[uid=todo-1]
+ PRIORITY:1
journal:status:todo-1:20260103T000000Z
string(9) "COMPLETED"
```

`supersedes()` refuses a target whose stored `X-VSTAR-HASH` no longer
matches its canonical form (`TargetCorruptedException`), so a chain
cannot be built on a tampered component.

### Errors

Every failure is a `VstarException` subclass, one per sentinel. Catch
the class when you know which failure you are handling, or catch the
base and switch on `sentinel()` — the identifier every V\*
implementation uses for that failure class — when you are dispatching.

```php
use HopTop\Vstar\Codec\Rfc5545\Parser;
use HopTop\Vstar\Exception\UnclosedBlockException;
use HopTop\Vstar\Exception\VstarException;

try {
    Parser::parse("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//x//EN\r\nBEGIN:VTODO\r\nUID:a\r\n");
} catch (UnclosedBlockException $e) {
    echo "caught by class: ", $e->sentinel(), "\n";
}

try {
    Parser::parse("not a calendar\r\n");
} catch (VstarException $e) {
    echo "dispatched on sentinel: ", $e->sentinel(), "\n";
}
```

```text
caught by class: ErrUnclosedBlock
dispatched on sentinel: ErrMalformed
```

### Enums and `toString()`

Wire-string types are string-backed enums, and the backing `->value` is
the contract — it is the token the corpus compares. Where the reference
also gives a type a display method, the enum spells it `toString()`,
not `__toString()`: PHP rejects the magic method on an enum at
declaration time. That is `Severity`, `Scope`, `DiffOp`, `Related`,
`Freq`, `Weekday`, `ByDay` and `RecurrenceRange`. Non-enum value
classes — `VDuration`, `Rrule\Rule`, `Diff\ComponentDiff`, `VDate`,
`RelType` — keep `__toString()`, so `(string) $rule` works as usual.

## Conformance

This port is a **round-trip** implementation and self-certifies in
[`VSTAR-CONFORMANCE.md`](VSTAR-CONFORMANCE.md) — spec revision, the
deliberate deviations from the Go surface, and which gates are green.

Its output is checked against the Go reference by the cross-language
parity harness: both emitters run the same corpus and must produce a
byte-identical document, so agreement is proven rather than assumed.

```sh
make test-parity   # from the monorepo root
```

## Develop

From the monorepo root:

```sh
make lint-php test-php build-php
```

`src/Generated/` is rendered from `spec/registry/` by `make
registry-gen`. Never hand-edit it; `make registry-check` fails on drift.

See [`CONTRIBUTING.md`](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md)
for repo-wide rules and
[`docs/INDEX.md`](https://github.com/hop-top/poly-vstar/blob/main/docs/INDEX.md#for-developers)
for the development loop.

## Links

- [Packagist](https://packagist.org/packages/hop-top/vstar) — the package
- [Monorepo](https://github.com/hop-top/poly-vstar) — source of truth; [issues](https://github.com/hop-top/poly-vstar/issues) and pull requests go here
- [Specification](https://github.com/hop-top/poly-vstar/tree/main/spec/) — normative text and the conformance corpus
- [API mapping](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/api-mapping.md) — the Go surface and each port's spelling
- [Porting guide](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/porting-guide.md) — writing a sister implementation
- [Diagnostic codes](https://github.com/hop-top/poly-vstar/blob/main/docs/validate-codes.md) — the `VS***` catalog

## License

MIT. See [`LICENSE`](https://github.com/hop-top/poly-vstar/blob/main/LICENSE)
at the monorepo root.
