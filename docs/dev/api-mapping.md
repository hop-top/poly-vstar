<!-- SPDX-License-Identifier: MIT -->

# API mapping — Go to TypeScript, Python, Rust, PHP

> **This document is normative and wins.** Where a task description, plan
> or brief spells a symbol differently, follow this document and report the
> conflict rather than choosing silently. Two ports were renamed after the
> fact for this reason (the rrule free functions and the Python exception
> classes); both times the brief was wrong and the document was right.

This document is the naming contract for the four language ports. It
maps every exported symbol of the Go reference implementation to its
name in each target language, so four independently written ports land
on one API shape rather than four.

The mapping is mechanical. Where a rule below decides a name, the rule
wins; where two ports would reasonably disagree, this document decides
for them. Anything not named here is not part of the contract — a port
MAY add private helpers, and MUST NOT add public surface that shadows a
name reserved here.

Read this alongside the [porting guide](porting-guide.md), which gives
the order to build in and the gates each layer must pass.

## Source of truth

The Go packages under [`../../go/`](../../go/) are the reference. When
this document and the Go source disagree, the Go source is right and
this document is a bug — file it.

Symbol counts per package, as enumerated from the Go source
(exported functions, methods, types, constants and variables; the
`go/cmd/...` build tools and the unexported
`go/codec/internal/contentline` package are excluded, since neither is
public API):

| Go package | Exported symbols |
|---|---:|
| root (`hop.top/vstar`) | 65 |
| `canonical` | 4 |
| `codec/rfc5545` | 8 |
| `codec/rfc6350` | 7 |
| `codec/stream` | 22 |
| `duration` | 19 |
| `diff` | 12 |
| `ext` | 6 |
| `hashing` | 7 |
| `helpers` | 40 |
| `rrule` | 30 |
| `supersession` | 5 |
| `validate` | 12 |
| **Total** | **237** |

The root count of 65 is the line-anchored declaration count. It
undercounts the members of grouped `const (...)` / `var (...)` blocks,
whose individual entries are not line-anchored declarations: the four
error sentinels in `go/errors.go`, the seven `CompType` constants, the
three `Kind` constants, the four `TodoStatus` constants, the twelve
`RelType` constants, the three `EventStatus`, three `JournalStatus`,
three `Class` and two `Transp` constants. Every one of those is named
in the tables below; the tables, not the count, are the contract.

## Package and namespace mapping

| Go package | TypeScript `@hop-top/vstar` | Python `vstar` | Rust `hop_top_vstar` | PHP `HopTop\Vstar` |
|---|---|---|---|---|
| root `hop.top/vstar` | `.` (package root) | `vstar` | crate root | `HopTop\Vstar\` |
| `canonical` | `./canonical` | `vstar.canonical` | `canonical` | `HopTop\Vstar\Canonical\` |
| `codec/rfc5545` | `./codec/rfc5545` | `vstar.codec.rfc5545` | `codec::rfc5545` | `HopTop\Vstar\Codec\Rfc5545\` |
| `codec/rfc6350` | `./codec/rfc6350` | `vstar.codec.rfc6350` | `codec::rfc6350` | `HopTop\Vstar\Codec\Rfc6350\` |
| `codec/stream` | `./codec/stream` | `vstar.codec.stream` | `codec::stream` | `HopTop\Vstar\Codec\Stream\` |
| `duration` | `./duration` | `vstar.duration` | `duration` | `HopTop\Vstar\Duration\` |
| `diff` | `./diff` | `vstar.diff` | `diff` | `HopTop\Vstar\Diff\` |
| `ext` | `./ext` | `vstar.ext` | `ext` | `HopTop\Vstar\Ext\` |
| `hashing` | `./hashing` | `vstar.hashing` | `hashing` | `HopTop\Vstar\Hashing\` |
| `helpers` | `./helpers` | `vstar.helpers` | `helpers` | `HopTop\Vstar\Helpers\` |
| `rrule` | `./rrule` | `vstar.rrule` | `rrule` | `HopTop\Vstar\Rrule\` |
| `supersession` | `./supersession` | `vstar.supersession` | `supersession` | `HopTop\Vstar\Supersession\` |
| `validate` | `./validate` | `vstar.validate` | `validate` | `HopTop\Vstar\Validate\` |

TypeScript subpaths are package exports: `import { parse } from
'@hop-top/vstar/codec/rfc5545'`. Every subpath is also re-exported from
the root under a namespace of the same name
(`import { rfc5545 } from '@hop-top/vstar'`), so both spellings work.

PHP has no free functions at namespace scope in the idiomatic style, so
each Go package that exposes package-level functions becomes a final
class with static methods, named for the package: `Canonical`,
`Hashing`, `Diff`, `Ext`, `Helpers`, `Supersession`, `Validate`. The
tables below spell the full `Class::method` for those.

## Naming idiom per language

| Go kind | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| Function | `camelCase` | `snake_case` | `snake_case` | `camelCase` static method |
| Method | `camelCase` | `snake_case` | `snake_case` | `camelCase` |
| Type / struct | `PascalCase` | `PascalCase` class | `PascalCase` | `PascalCase` class |
| Struct field | `camelCase` property | `snake_case` attribute | `snake_case` field | `camelCase` property |
| Constant | `SCREAMING_SNAKE_CASE` | `SCREAMING_SNAKE_CASE` | `SCREAMING_SNAKE_CASE` | `public const SCREAMING_SNAKE_CASE` |
| Enum type | string-literal union or `enum` | `enum.Enum` subclass | `enum` | `enum` (PHP 8.1 backed enum) |

Go acronym runs stay uppercase in Go (`UID`, `DTSTART`, `ParseRRule`).
Ports do **not** keep the run: TypeScript and PHP write `uid`,
`dtstart`, `parseRRule`; Python and Rust write `uid`, `dtstart`,
`parse_rrule`. The only exception is a name that is entirely an RFC
wire token used as an identifier (`DTSTART`, `DTEND`, `DUE`,
`COMPLETED`, `DTSTAMP`) where the Go method has no other word: those
keep their wire spelling in every language so the call site reads like
the property it fetches, lowercased in TS/PHP/Python/Rust per the idiom
column. Each such case is spelled out in the tables, so no port has to
guess.

## Root package — `hop.top/vstar`

### Data model types

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `Property` (struct: `Name`, `Params`, `Value`) | `Property` (`name`, `params`, `value`) | `Property` (`name`, `params`, `value`) | `Property { name, params, value }` | `Property` (`$name`, `$params`, `$value`) |
| `Param` (struct: `Name`, `Value`) | `Param` (`name`, `value`) | `Param` (`name`, `value`) | `Param { name, value }` | `Param` (`$name`, `$value`) |
| `Component` (struct: `Type`, `Props`, `Sub`) | `Component` (`type`, `props`, `sub`) | `Component` (`type`, `props`, `sub`) | `Component { r#type, props, sub }` | `Component` (`$type`, `$props`, `$sub`) |
| `Calendar` (struct: `ProdID`, `Components`) | `Calendar` (`prodId`, `components`) | `Calendar` (`prod_id`, `components`) | `Calendar { prod_id, components }` | `Calendar` (`$prodId`, `$components`) |
| `Card` (struct: `UID`, `Kind`, `Props`) | `Card` (`uid`, `kind`, `props`) | `Card` (`uid`, `kind`, `props`) | `Card { uid, kind, props }` | `Card` (`$uid`, `$kind`, `$props`) |
| `Date` (struct: `Year`, `Month`, `Day`) | `VDate` (`year`, `month`, `day`) | `VDate` (`year`, `month`, `day`) | `Date { year, month, day }` | `VDate` (`$year`, `$month`, `$day`) |

`Date` is renamed to `VDate` in TypeScript, Python and PHP because
`Date` is a built-in in all three; Rust keeps `Date` because the crate
root has no such collision (`chrono::NaiveDate` is imported under its
own path). `Month` is `time.Month` in Go (1-based); ports use a plain
1-based integer, **not** a zero-based month index — a TypeScript port
must not reuse `Date#getMonth()` semantics here.

### Codec interfaces

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `Parser` (interface, `Parse(io.Reader) (Calendar, error)`) | `Parser` (interface) | `Parser` (`Protocol`) | `Parser` (trait) | `Parser` (interface) |
| `Encoder` (interface, `Encode(io.Writer, Calendar) error`) | `Encoder` | `Encoder` | `Encoder` (trait) | `Encoder` |
| `Codec` (interface, `Parser` + `Encoder`) | `Codec` | `Codec` | `Codec` (trait) | `Codec` |

### Value-type constants

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `ValueParam = "VALUE"` | `VALUE_PARAM` | `VALUE_PARAM` | `VALUE_PARAM` | `Vstar::VALUE_PARAM` |
| `ValueDate = "DATE"` | `VALUE_DATE` | `VALUE_DATE` | `VALUE_DATE` | `Vstar::VALUE_DATE` |

### `Calendar` methods

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `(*Calendar) Find(uid string) (Component, bool)` | `find(uid): Component \| undefined` | `find(uid) -> Component \| None` | `find(&self, uid) -> Option<&Component>` | `find(string $uid): ?Component` |
| `(*Calendar) Append(comp Component)` | `append(comp)` | `append(comp)` | `append(&mut self, comp)` | `append(Component $comp): void` |
| `(*Calendar) Filter(t CompType) []Component` | `filter(t): Component[]` | `filter(t) -> list[Component]` | `filter(&self, t) -> Vec<&Component>` | `filter(CompType $t): array` |

### `Card` methods

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `(*Card) Get(name string) (Property, bool)` | `get(name): Property \| undefined` | `get(name) -> Property \| None` | `get(&self, name) -> Option<&Property>` | `get(string $name): ?Property` |
| `(*Card) GetAll(name string) []Property` | `getAll(name): Property[]` | `get_all(name) -> list[Property]` | `get_all(&self, name) -> Vec<&Property>` | `getAll(string $name): array` |
| `(*Card) Set(p Property)` | `set(p)` | `set(p)` | `set(&mut self, p)` | `set(Property $p): void` |
| `(*Card) Add(p Property)` | `add(p)` | `add(p)` | `add(&mut self, p)` | `add(Property $p): void` |
| `(*Card) Remove(name string)` | `remove(name)` | `remove(name)` | `remove(&mut self, name)` | `remove(string $name): void` |

### `Component` methods — property access

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `(*Component) Get(name string) (Property, bool)` | `get(name): Property \| undefined` | `get(name) -> Property \| None` | `get(&self, name) -> Option<&Property>` | `get(string $name): ?Property` |
| `(*Component) GetAll(name string) []Property` | `getAll(name): Property[]` | `get_all(name) -> list[Property]` | `get_all(&self, name) -> Vec<&Property>` | `getAll(string $name): array` |
| `(*Component) Set(p Property)` | `set(p)` | `set(p)` | `set(&mut self, p)` | `set(Property $p): void` |
| `(*Component) Add(p Property)` | `add(p)` | `add(p)` | `add(&mut self, p)` | `add(Property $p): void` |
| `(*Component) Remove(name string)` | `remove(name)` | `remove(name)` | `remove(&mut self, name)` | `remove(string $name): void` |
| `(*Component) UID() string` | `uid(): string` | `uid() -> str` | `uid(&self) -> &str` | `uid(): string` |
| `(*Component) DTSTAMPRaw() string` | `dtstampRaw(): string` | `dtstamp_raw() -> str` | `dtstamp_raw(&self) -> &str` | `dtstampRaw(): string` |
| `(*Component) IsDateOnly(name string) bool` | `isDateOnly(name): boolean` | `is_date_only(name) -> bool` | `is_date_only(&self, name) -> bool` | `isDateOnly(string $name): bool` |

### `Component` methods — date accessors

Each returns `(Date, bool)` in Go; the boolean means "the property is
present and is a DATE value".

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `DTSTARTDate() (Date, bool)` | `dtstartDate(): VDate \| undefined` | `dtstart_date() -> VDate \| None` | `dtstart_date(&self) -> Option<Date>` | `dtstartDate(): ?VDate` |
| `DTENDDate() (Date, bool)` | `dtendDate(): VDate \| undefined` | `dtend_date() -> VDate \| None` | `dtend_date(&self) -> Option<Date>` | `dtendDate(): ?VDate` |
| `DUEDate() (Date, bool)` | `dueDate(): VDate \| undefined` | `due_date() -> VDate \| None` | `due_date(&self) -> Option<Date>` | `dueDate(): ?VDate` |
| `COMPLETEDDate() (Date, bool)` | `completedDate(): VDate \| undefined` | `completed_date() -> VDate \| None` | `completed_date(&self) -> Option<Date>` | `completedDate(): ?VDate` |
| `SetDTSTARTDate(d Date)` | `setDtstartDate(d)` | `set_dtstart_date(d)` | `set_dtstart_date(&mut self, d)` | `setDtstartDate(VDate $d): void` |
| `SetDTENDDate(d Date)` | `setDtendDate(d)` | `set_dtend_date(d)` | `set_dtend_date(&mut self, d)` | `setDtendDate(VDate $d): void` |
| `SetDUEDate(d Date)` | `setDueDate(d)` | `set_due_date(d)` | `set_due_date(&mut self, d)` | `setDueDate(VDate $d): void` |
| `SetCOMPLETEDDate(d Date)` | `setCompletedDate(d)` | `set_completed_date(d)` | `set_completed_date(&mut self, d)` | `setCompletedDate(VDate $d): void` |

### `Component` methods — datetime accessors

The four calendar-context getters take a `Calendar` because resolving a
`TZID`-bearing local time needs the enclosing calendar's VTIMEZONE
registry. `DTSTAMP` does not, because `DTSTAMP` is always UTC.

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `DTSTART(cal Calendar) (time.Time, bool)` | `dtstart(cal): Instant \| undefined` | `dtstart(cal) -> datetime \| None` | `dtstart(&self, cal) -> Option<DateTime<Utc>>` | `dtstart(Calendar $cal): ?DateTimeImmutable` |
| `DTEND(cal Calendar) (time.Time, bool)` | `dtend(cal)` | `dtend(cal)` | `dtend(&self, cal)` | `dtend(Calendar $cal)` |
| `DUE(cal Calendar) (time.Time, bool)` | `due(cal)` | `due(cal)` | `due(&self, cal)` | `due(Calendar $cal)` |
| `COMPLETED(cal Calendar) (time.Time, bool)` | `completed(cal)` | `completed(cal)` | `completed(&self, cal)` | `completed(Calendar $cal)` |
| `DTSTAMP() (time.Time, bool)` | `dtstamp()` | `dtstamp()` | `dtstamp(&self)` | `dtstamp()` |
| `SetDTSTART(t time.Time)` | `setDtstart(t)` | `set_dtstart(t)` | `set_dtstart(&mut self, t)` | `setDtstart(DateTimeImmutable $t): void` |
| `SetDTEND(t time.Time)` | `setDtend(t)` | `set_dtend(t)` | `set_dtend(&mut self, t)` | `setDtend(DateTimeImmutable $t): void` |
| `SetDUE(t time.Time)` | `setDue(t)` | `set_due(t)` | `set_due(&mut self, t)` | `setDue(DateTimeImmutable $t): void` |
| `SetCOMPLETED(t time.Time)` | `setCompleted(t)` | `set_completed(t)` | `set_completed(&mut self, t)` | `setCompleted(DateTimeImmutable $t): void` |

### Date and time free functions

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `DateOf(t time.Time) Date` | `dateOf(t): VDate` | `date_of(t) -> VDate` | `date_of(t) -> Date` | `Vstar::dateOf(...): VDate` |
| `(Date) IsZero() bool` | `isZero(): boolean` | `is_zero() -> bool` | `is_zero(&self) -> bool` | `isZero(): bool` |
| `(Date) Time() time.Time` | `toInstant(): Instant` | `to_datetime() -> datetime` | `to_datetime(&self) -> DateTime<Utc>` | `toDateTime(): DateTimeImmutable` |
| `(Date) String() string` | `toString(): string` | `__str__() -> str` | `Display for Date` | `__toString(): string` |
| `FormatDate(d Date) string` | `formatDate(d): string` | `format_date(d) -> str` | `format_date(d) -> String` | `Vstar::formatDate(VDate $d): string` |
| `ParseDate(s string) (Date, bool)` | `parseDate(s): VDate \| undefined` | `parse_date(s) -> VDate \| None` | `parse_date(s) -> Option<Date>` | `Vstar::parseDate(string $s): ?VDate` |
| `FormatTime(t time.Time) string` | `formatTime(t): string` | `format_time(t) -> str` | `format_time(t) -> String` | `Vstar::formatTime(...): string` |
| `ParseTime(s string) (time.Time, bool)` | `parseTime(s): Instant \| undefined` | `parse_time(s) -> datetime \| None` | `parse_time(s) -> Option<DateTime<Utc>>` | `Vstar::parseTime(string $s): ?DateTimeImmutable` |
| `ParseTimeWithTZID(s, tzid string, cal Calendar) (time.Time, bool)` | `parseTimeWithTzid(s, tzid, cal)` | `parse_time_with_tzid(s, tzid, cal)` | `parse_time_with_tzid(s, tzid, cal)` | `Vstar::parseTimeWithTzid(...)` |
| `Equal(a, b Property) bool` | `propertyEqual(a, b): boolean` | `property_equal(a, b) -> bool` | `property_equal(a, b) -> bool` | `Vstar::propertyEqual(...): bool` |

`Equal` is renamed to `propertyEqual` in every port. In Go the package
qualifier (`vstar.Equal`) carries the meaning; a bare `equal` exported
from a package root does not, and collides with the three
`diff`-package equality functions.

### Wire-string enums

Each Go type below is a `string` with named constants. Ports model them
as string-backed enums (PHP 8.1 `enum: string`, Python `StrEnum`, Rust
enum with `as_str()`/`FromStr`, TypeScript string-literal union plus a
frozen constant object of the same name). The wire value is normative —
a port that changes a wire spelling is broken.

| Go type | Wire values | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|---|
| `CompType` (**open**, see below) | `VCALENDAR`, `VTODO`, `VJOURNAL`, `VEVENT`, `VFREEBUSY`, `VTIMEZONE`, `VALARM` | `CompType` | `CompType` | `CompType` | `CompType` |
| `Kind` | `individual`, `org`, `group` | `Kind` | `Kind` | `Kind` | `Kind` |

**PHP enums cannot declare `__toString()`.** The engine rejects it at
declaration time: `Enum X cannot include magic method __toString`. So a Go
`String()` on a type this document models as a PHP enum maps to a plain
`toString()`, not `__toString()` — that is `Related`, `DiffOp`, `Scope`,
`Freq`, `Weekday`, `RecurrenceRange` and `Severity`. The seven enums Go
gives no `String()` — `CompType`, `Kind`, `VClass`, `Transp`,
`EventStatus`, `TodoStatus`, `JournalStatus` — declare the same
`toString(): string` anyway, returning `->value`, so every PHP enum has
the one accessor. That is a PHP-side uniformity with no Go counterpart,
which is why no row below lists it. A backed enum's wire string also
remains reachable as `->value`. Value classes that are NOT enums —
`VDate`, `VDuration`, `RelType`, `ComponentDiff`, `Rule` — keep
`__toString()` and may implement `\Stringable`.

`CompType` and `Kind` are **open**, exactly like `RelType`: the constants
name the registered vocabulary, they do not bound it. Go declares both as
`type X string`, so any wire token parses.

This is load-bearing, not a nicety. `rfc5545/nested_vtimezone.ics` nests
`BEGIN:STANDARD` and `BEGIN:DAYLIGHT` inside a `VTIMEZONE`, and neither is
one of the seven named constants. A port that models `CompType` as a closed
enum re-emits those sub-components as something else and silently corrupts
the timezone — and a round-trip test does **not** catch it, because both
halves lose them identically. Only encoder-parity against the reference
bytes catches it. Two ports hit this independently.

The same applies to the PHP mapping: `Component::$type` holds the wire
string, with `CompType` as the named view over it. A closed PHP enum on
that property cannot represent a conformant document either.

`Kind` is open for a second reason: its empty value means "no KIND
property", and the encoder omits the line entirely. `rfc6350/minimal.vcf`
has none. Model it as nullable (`Option`/`None`/`null`) rather than
defaulting to a member, or the port invents a `KIND:individual` line.

| `TodoStatus` | `NEEDS-ACTION`, `IN-PROCESS`, `COMPLETED`, `CANCELLED` | `TodoStatus` | `TodoStatus` | `TodoStatus` | `TodoStatus` |
| `EventStatus` | `TENTATIVE`, `CONFIRMED`, `CANCELLED` | `EventStatus` | `EventStatus` | `EventStatus` | `EventStatus` |
| `JournalStatus` | `DRAFT`, `FINAL`, `CANCELLED` | `JournalStatus` | `JournalStatus` | `JournalStatus` | `JournalStatus` |
| `Class` | `PUBLIC`, `PRIVATE`, `CONFIDENTIAL` | `VClass` | `VClass` | `Class` | `VClass` |
| `Transp` | `OPAQUE`, `TRANSPARENT` | `Transp` | `Transp` | `Transp` | `Transp` |
| `RelType` | see below | `RelType` | `RelType` | `RelType` | `RelType` |

`Class` becomes `VClass` in TypeScript, Python and PHP: `class` is a
reserved word in all three, and a type named `Class` reads as the
language's own reflection type in PHP. Rust keeps `Class`.

The three status types are **deliberately distinct types**, not one
type with a shared `CANCELLED`. A port that collapses them into a
single `Status` enum has broken the contract: the RFC scopes each
vocabulary to one component type, and separate types make a cross-type
assignment a compile error rather than a wire-level conformance bug.

Constant names follow the Go spelling, re-cased per language:
`CompCalendar`/`COMP_CALENDAR`, `TodoNeedsAction`/`TODO_NEEDS_ACTION`,
`EventTentative`/`EVENT_TENTATIVE`, `JournalDraft`/`JOURNAL_DRAFT`,
`ClassPublic`/`CLASS_PUBLIC`, `TranspOpaque`/`TRANSP_OPAQUE`,
`KindIndividual`/`KIND_INDIVIDUAL`, and so on for every value listed.

### `RelType` — the open enum

`RelType` is deliberately **open**: any string is a valid `RelType`.
The constants name the registered vocabulary (RFC 5545 §3.2.15 plus RFC
9253 §4 and §5); they do not bound it. A port that models `RelType` as
a closed enum rejecting `X-`-prefixed values is broken.

Registered values: `PARENT`, `CHILD`, `SIBLING`, `FINISHTOSTART`,
`FINISHTOFINISH`, `STARTTOFINISH`, `STARTTOSTART`, `DEPENDS-ON`,
`FIRST`, `NEXT`, `CONCEPT`, `REFID`.

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `DefaultRelType = RelParent` | `DEFAULT_REL_TYPE` | `DEFAULT_REL_TYPE` | `DEFAULT_REL_TYPE` | `RelType::DEFAULT` |
| `ParseRelType(s string) (RelType, bool)` | `parseRelType(s): [RelType, boolean]` | `parse_rel_type(s) -> tuple[RelType, bool]` | `parse_rel_type(s) -> (RelType, bool)` | `RelType::parse(string $s): array` |
| `(RelType) EqualFold(s string) bool` | `relTypeEqualFold(r, s): boolean` | `equal_fold(s) -> bool` | `equal_fold(&self, s) -> bool` | `equalFold(string $s): bool` |

`ParseRelType` is the one place in the API where a `bool` second return
is **not** an optional. It reports "this named a registered value", and
the first return is meaningful in both cases: an empty input yields
`PARENT` with `true`, an unregistered input is returned verbatim with
`false` so a caller accepting extensions can keep the original
spelling. Ports return a genuine pair, not an `Option`. Rust may model
this as a two-field struct or a tuple; either is conformant, a bare
`Option<RelType>` is not.

## `canonical`

Every function returns raw bytes — the canonical wire form — not text.
See the [porting guide](porting-guide.md#byte-comparisons-compare-bytes)
for why that distinction is load-bearing.

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `Component(c vstar.Component) []byte` | `component(c): Uint8Array` | `component(c) -> bytes` | `component(c) -> Vec<u8>` | `Canonical::component(Component $c): string` |
| `ComponentInContext(c vstar.Component, cal vstar.Calendar) []byte` | `componentInContext(c, cal): Uint8Array` | `component_in_context(c, cal) -> bytes` | `component_in_context(c, cal) -> Vec<u8>` | `Canonical::componentInContext(...): string` |
| `Calendar(c vstar.Calendar) []byte` | `calendar(c): Uint8Array` | `calendar(c) -> bytes` | `calendar(c) -> Vec<u8>` | `Canonical::calendar(Calendar $c): string` |
| `Card(c vstar.Card) []byte` | `card(c): Uint8Array` | `card(c) -> bytes` | `card(c) -> Vec<u8>` | `Canonical::card(Card $c): string` |

PHP returns a binary string — PHP's `string` is a byte array and is the
correct type here. A port MUST NOT run these results through any text
transform (trimming, re-encoding, newline normalization) before
comparing them.

## `codec/rfc5545`

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `New() vstar.Codec` | `newCodec(): Codec` | `new_codec() -> Codec` | `new_codec() -> impl Codec` | `Rfc5545::newCodec(): Codec` |
| `Default vstar.Codec` | `DEFAULT` | `DEFAULT` | `default_codec()` | `Rfc5545::default(): Codec` |
| `Parse(r io.Reader) (vstar.Calendar, error)` | `parse(input): Calendar` (throws) | `parse(input) -> Calendar` (raises) | `parse(input) -> Result<Calendar, Error>` | `Rfc5545::parse($input): Calendar` (throws) |
| `Encode(w io.Writer, cal vstar.Calendar) error` | `encode(cal): Uint8Array` | `encode(cal) -> bytes` | `encode(cal) -> Result<Vec<u8>, Error>` | `Rfc5545::encode(Calendar $cal): string` |
| `EncodeComponent(w io.Writer, c vstar.Component) error` | `encodeComponent(c): Uint8Array` | `encode_component(c) -> bytes` | `encode_component(c) -> Result<Vec<u8>, Error>` | `Rfc5545::encodeComponent(...): string` |
| `ParseContentLine(line string) (vstar.Property, error)` | `parseContentLine(line): Property` | `parse_content_line(line) -> Property` | `parse_content_line(line) -> Result<Property, Error>` | `Rfc5545::parseContentLine(string $line): Property` |
| `Scanner` (type alias) | `Scanner` | `Scanner` | `Scanner` | `Scanner` |
| `NewScanner(r io.Reader) *Scanner` | `newScanner(input): Scanner` | `new_scanner(input) -> Scanner` | `Scanner::new(input)` | `Rfc5545::newScanner($input): Scanner` |

Go's encode functions write to an `io.Writer` and return `error`. Ports
that have no idiomatic writer abstraction in the same position return
the bytes instead, as shown. A port MAY *additionally* offer a
writer-taking overload; the byte-returning form is the one the gates
exercise. `Default` is a shared, stateless, concurrency-safe instance
in Go; ports expose it as a module-level constant (TS/Python), a
function returning a `'static` reference or a fresh value (Rust), or a
static method (PHP) — all four are stateless, so any is conformant.

## `codec/rfc6350`

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `Parser` (interface, `Parse(io.Reader) ([]vstar.Card, error)`) | `Parser` | `Parser` | `Parser` (trait) | `Parser` |
| `Encoder` (interface, `Encode(io.Writer, vstar.Card) error`) | `Encoder` | `Encoder` | `Encoder` (trait) | `Encoder` |
| `Codec` (interface) | `Codec` | `Codec` | `Codec` (trait) | `Codec` |
| `New() Codec` | `newCodec(): Codec` | `new_codec() -> Codec` | `new_codec() -> impl Codec` | `Rfc6350::newCodec(): Codec` |
| `Default` | `DEFAULT` | `DEFAULT` | `default_codec()` | `Rfc6350::default(): Codec` |
| `NewEncoder() Encoder` | `newEncoder(): Encoder` | `new_encoder() -> Encoder` | `new_encoder() -> impl Encoder` | `Rfc6350::newEncoder(): Encoder` |
| `NewParser() Parser` | `newParser(): Parser` | `new_parser() -> Parser` | `new_parser() -> impl Parser` | `Rfc6350::newParser(): Parser` |

### `rfc6350.Parse` returns a LIST of cards

`rfc6350.Parser.Parse` returns `[]vstar.Card` — a **list**, not one
card. A vCard stream is a sequence of self-contained
`BEGIN:VCARD…END:VCARD` blocks with no enclosing wrapper, so a file with
three cards parses to three `Card` values.

A port whose `parse` returns a single `Card` is broken, and its
`rfc6350/` corpus round-trip will silently pass on the single-card
fixtures and fail on the rest. Signatures:

- TypeScript: `parse(input: Uint8Array): Card[]`
- Python: `parse(input: bytes) -> list[Card]`
- Rust: `parse(input: &[u8]) -> Result<Vec<Card>, Error>`
- PHP: `parse(string $input): array` (a `list<Card>`)

`Encoder.Encode` is the asymmetric half: it takes exactly **one**
`Card`. Encoding a list means calling it once per card and concatenating
— which is what the stream encoder does.

## `codec/stream`

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `StreamParser` (interface) | `StreamParser` | `StreamParser` | `StreamParser` (trait) | `StreamParser` |
| `StreamEncoder` (interface) | `StreamEncoder` | `StreamEncoder` | `StreamEncoder` (trait) | `StreamEncoder` |
| `CardStreamParser` (interface) | `CardStreamParser` | `CardStreamParser` | `CardStreamParser` (trait) | `CardStreamParser` |
| `CardStreamEncoder` (interface) | `CardStreamEncoder` | `CardStreamEncoder` | `CardStreamEncoder` (trait) | `CardStreamEncoder` |
| `ErrAlreadyClosed` | see [sentinels](#the-twelve-error-sentinels) | " | " | " |
| `ErrHeaderLocked` | see [sentinels](#the-twelve-error-sentinels) | " | " | " |
| `VCalendarParser` (struct) | `VCalendarParser` | `VCalendarParser` | `VCalendarParser` | `VCalendarParser` |
| `NewVCalendarParser(r io.Reader) *VCalendarParser` | `newVCalendarParser(input: LineSource)` | `VCalendarParser(data: Input)` | `VCalendarParser::new(r: R) where R: Read` | `new VCalendarParser($stream)` (a stream resource) |
| `(*VCalendarParser) Header() vstar.Calendar` | `header(): Calendar` | `header() -> Calendar` | `header(&mut self) -> &Calendar` | `header(): Calendar` |
| `(*VCalendarParser) Next() (vstar.Component, error)` | `next(): IteratorResult<Component>` (throws) | `__next__() -> Component` (raises) | `next(&mut self) -> Option<Result<Component, Error>>` | `next(): ?Component` (throws) |
| `VCalendarEncoder` (struct) | `VCalendarEncoder` | `VCalendarEncoder` | `VCalendarEncoder` | `VCalendarEncoder` |
| `NewVCalendarEncoder(w io.Writer) *VCalendarEncoder` | `newVCalendarEncoder(sink)` | `VCalendarEncoder(sink)` | `VCalendarEncoder::new(sink)` | `new VCalendarEncoder($sink)` |
| `(*VCalendarEncoder) SetHeader(h vstar.Calendar) error` | `setHeader(h): void` (throws) | `set_header(h) -> None` (raises) | `set_header(&mut self, h) -> Result<(), Error>` | `setHeader(Calendar $h): void` (throws) |
| `(*VCalendarEncoder) Encode(c vstar.Component) error` | `encode(c): void` (throws) | `encode(c) -> None` (raises) | `encode(&mut self, c) -> Result<(), Error>` | `encode(Component $c): void` (throws) |
| `(*VCalendarEncoder) Close() error` | `close(): void` (throws) | `close() -> None` (raises) | `close(&mut self) -> Result<(), Error>` | `close(): void` (throws) |
| `VCardParser` (struct) | `VCardParser` | `VCardParser` | `VCardParser` | `VCardParser` |
| `NewVCardParser(r io.Reader) *VCardParser` | `newVCardParser(input)` | `VCardParser(input)` | `VCardParser::new(input)` | `new VCardParser($input)` |
| `(*VCardParser) Next() (vstar.Card, error)` | `next(): IteratorResult<Card>` (throws) | `__next__() -> Card` (raises) | `next(&mut self) -> Option<Result<Card, Error>>` | `next(): ?Card` (throws) |
| `VCardEncoder` (struct) | `VCardEncoder` | `VCardEncoder` | `VCardEncoder` | `VCardEncoder` |
| `NewVCardEncoder(w io.Writer) *VCardEncoder` | `newVCardEncoder(sink)` | `VCardEncoder(sink)` | `VCardEncoder::new(sink)` | `new VCardEncoder($sink)` |
| `(*VCardEncoder) Encode(c vstar.Card) error` | `encode(c): void` (throws) | `encode(c) -> None` (raises) | `encode(&mut self, c) -> Result<(), Error>` | `encode(Card $c): void` (throws) |
| `(*VCardEncoder) Close() error` | `close(): void` (throws) | `close() -> None` (raises) | `close(&mut self) -> Result<(), Error>` | `close(): void` (throws) |

### A stream parser MUST accept bytes

Every parser above names its input type, and every one of them can
carry raw octets. That is normative, not incidental: canonicalization
rule 3 folds on a 75-**octet** boundary, so a multi-byte UTF-8 sequence
is split across the fold. A parser whose input is a sequence of decoded
strings cannot represent half a sequence, and silently substitutes
U+FFFD — losing data with no error.

The TypeScript port shipped exactly that defect, accepting only
`Iterable<string>`, and nothing caught it because this table left the
input untyped. Spell the type, and make it one that holds bytes.

### `VCardEncoder` has NO `SetHeader`

Only `VCalendarEncoder` has `SetHeader`. A VCARD stream has no
enclosing wrapper — each `Encode` writes a complete, self-contained
`BEGIN:VCARD…END:VCARD` block — so there is no header to set and no
trailer to emit. `VCardEncoder.Close` exists only for symmetry and to
flush buffered writes.

A port that adds `setHeader` to `VCardEncoder` for API symmetry is
broken: there is nothing for it to do, and its existence invites
callers to emit a wrapper that the parsers will reject.

### Parser exhaustion

Go's stream parsers signal exhaustion by returning `io.EOF`. Ports use
their own idiom, and exhaustion is **never** an error:

- TypeScript: implement the iterator protocol — `next()` returns
  `{done: true}` at exhaustion, throws a `VstarError` on a real parse
  failure. Exposing an async iterator as well is fine.
- Python: implement `__iter__`/`__next__` — raise `StopIteration` at
  exhaustion, raise a `VstarError` subclass on a real failure.
- Rust: implement `Iterator<Item = Result<Component, Error>>` — `None`
  at exhaustion, `Some(Err(_))` on a real failure.
- PHP: implement `Iterator` (or return a `Generator`) — `valid()`
  returns `false` at exhaustion, throws on a real failure.

### Encoder lifecycle errors

`Close` called twice, or `Encode` called after `Close`, yields
`ErrAlreadyClosed`. `SetHeader` called after the first `Encode` yields
`ErrHeaderLocked` — the header is locked at first `Encode` so the
`BEGIN:VCALENDAR` / `VERSION` / `PRODID` / `METHOD` ordering on the wire
is deterministic. Both are in the [sentinel
table](#the-twelve-error-sentinels) and both MUST be reachable by a
port's callers; a port that silently ignores a double `Close` is broken.

## `duration`

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `Duration` (struct) | `VDuration` | `VDuration` | `Duration` | `VDuration` |
| `Parse(s string) (Duration, error)` | `parse(s): VDuration` (throws) | `parse(s) -> VDuration` (raises) | `parse(s) -> Result<Duration, Error>` | `Duration::parse(string $s): VDuration` |
| `Valid(s string) bool` | `valid(s): boolean` | `valid(s) -> bool` | `valid(s) -> bool` | `Duration::valid(string $s): bool` |
| `(Duration) String() string` | `toString(): string` | `__str__() -> str` | `Display for Duration` | `__toString(): string` |
| `(Duration) Signed() time.Duration` | `signed(): number` (ms) | `signed() -> timedelta` | `signed(&self) -> chrono::Duration` | `signed(): DateInterval` |
| `(Duration) IsNegative() bool` | `isNegative(): boolean` | `is_negative() -> bool` | `is_negative(&self) -> bool` | `isNegative(): bool` |
| `(Duration) AddTo(t time.Time) time.Time` | `addTo(t): Instant` | `add_to(t) -> datetime` | `add_to(&self, t) -> DateTime<Utc>` | `addTo(DateTimeImmutable $t): DateTimeImmutable` |
| `FromSigned(td time.Duration) Duration` | `fromSigned(ms): VDuration` | `from_signed(td) -> VDuration` | `from_signed(td) -> Duration` | `Duration::fromSigned(...): VDuration` |
| `ErrNoTrigger` | see [sentinels](#the-twelve-error-sentinels) | " | " | " |
| `ErrNoAnchor` | see [sentinels](#the-twelve-error-sentinels) | " | " | " |
| `Related` (enum: `RelatedStart`, `RelatedEnd`) | `Related` | `Related` | `Related` | `Related` |
| `(Related) String() string` | `toString()` | `__str__()` | `Display for Related` | `toString()` |
| `Trigger` (struct) | `Trigger` | `Trigger` | `Trigger` | `Trigger` |
| `ParseTrigger(p vstar.Property) (Trigger, error)` | `parseTrigger(p): Trigger` | `parse_trigger(p) -> Trigger` | `parse_trigger(p) -> Result<Trigger, Error>` | `Trigger::parse(Property $p): Trigger` |
| `AlarmTrigger(alarm vstar.Component) (Trigger, error)` | `alarmTrigger(alarm): Trigger` | `alarm_trigger(alarm) -> Trigger` | `alarm_trigger(alarm) -> Result<Trigger, Error>` | `Duration::alarmTrigger(...): Trigger` |
| `(Trigger) Property() vstar.Property` | `toProperty(): Property` | `to_property() -> Property` | `to_property(&self) -> Property` | `toProperty(): Property` |
| `(Trigger) Resolve(parent vstar.Component, cal vstar.Calendar) (time.Time, error)` | `resolve(parent, cal): Instant` | `resolve(parent, cal) -> datetime` | `resolve(&self, parent, cal) -> Result<DateTime<Utc>, Error>` | `resolve(Component $parent, Calendar $cal): DateTimeImmutable` |
| `EventEnd(c vstar.Component, cal vstar.Calendar) (time.Time, bool)` | `eventEnd(c, cal): Instant \| undefined` | `event_end(c, cal) -> datetime \| None` | `event_end(c, cal) -> Option<DateTime<Utc>>` | `Duration::eventEnd(...): ?DateTimeImmutable` |
| `AlarmRepeatCycle(alarm vstar.Component) (Duration, int, error)` | `alarmRepeatCycle(alarm): [VDuration, number]` | `alarm_repeat_cycle(alarm) -> tuple[VDuration, int]` | `alarm_repeat_cycle(alarm) -> Result<(Duration, i32), Error>` | `Duration::alarmRepeatCycle(...): array` |

`Duration` becomes `VDuration` in TypeScript, Python and PHP: all three
have a `Duration`-shaped built-in or near-universal library type
(`Temporal.Duration`, `datetime.timedelta`, `DateInterval`), and the V\*
type is not interchangeable with any of them. Rust keeps `Duration`
because the crate path (`duration::Duration`) disambiguates and
`chrono::Duration` is imported under its own path.

`Duration.DayForm` is a real, load-bearing field, not an
implementation detail: it records that the value was authored as `P0D`
rather than `PT0S`. Both are numerically zero and every unit field is
zero in both, so without the flag `String()` cannot reproduce the
authored spelling — and canonical form preserves `DURATION` values
verbatim (spec rule 12). A port that drops `DayForm` will fail the
`behavior/duration` gate. The flag affects formatting only: `signed`,
`isNegative` and `addTo` ignore it.

## `diff`

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `OfComponent(a, b vstar.Component) ComponentDiff` | `ofComponent(a, b): ComponentDiff` | `of_component(a, b) -> ComponentDiff` | `of_component(a, b) -> ComponentDiff` | `Diff::ofComponent(...): ComponentDiff` |
| `OfCard(a, b vstar.Card) ComponentDiff` | `ofCard(a, b)` | `of_card(a, b)` | `of_card(a, b)` | `Diff::ofCard(...)` |
| `OfCalendar(a, b vstar.Calendar) []ComponentDiff` | `ofCalendar(a, b): ComponentDiff[]` | `of_calendar(a, b) -> list[ComponentDiff]` | `of_calendar(a, b) -> Vec<ComponentDiff>` | `Diff::ofCalendar(...): array` |
| `Component(a, b vstar.Component) bool` | `componentEqual(a, b): boolean` | `component_equal(a, b) -> bool` | `component_equal(a, b) -> bool` | `Diff::componentEqual(...): bool` |
| `Card(a, b vstar.Card) bool` | `cardEqual(a, b): boolean` | `card_equal(a, b) -> bool` | `card_equal(a, b) -> bool` | `Diff::cardEqual(...): bool` |
| `Calendar(a, b vstar.Calendar) bool` | `calendarEqual(a, b): boolean` | `calendar_equal(a, b) -> bool` | `calendar_equal(a, b) -> bool` | `Diff::calendarEqual(...): bool` |
| `DiffOp` (enum: `OpAdded`, `OpRemoved`, `OpChanged`) | `DiffOp` | `DiffOp` | `DiffOp` | `DiffOp` |
| `(DiffOp) String() string` | `toString()` | `__str__()` | `Display for DiffOp` | `toString()` |
| `PropertyDiff` (struct: `Op`, `Property`, `Old`) | `PropertyDiff` (`op`, `property`, `old`) | `PropertyDiff` (`op`, `property`, `old`) | `PropertyDiff { op, property, old }` | `PropertyDiff` (`$op`, `$property`, `$old`) |
| `ComponentDiff` (struct: `Path`, `Properties`, `SubDiffs`) | `ComponentDiff` (`path`, `properties`, `subDiffs`) | `ComponentDiff` (`path`, `properties`, `sub_diffs`) | `ComponentDiff { path, properties, sub_diffs }` | `ComponentDiff` (`$path`, `$properties`, `$subDiffs`) |
| `(ComponentDiff) String() string` | `toString()` | `__str__()` | `Display for ComponentDiff` | `__toString()` |
| `(ComponentDiff) Empty() bool` | `isEmpty(): boolean` | `is_empty() -> bool` | `is_empty(&self) -> bool` | `isEmpty(): bool` |

The three boolean functions are renamed with an `Equal` suffix in every
port. In Go, `diff.Component(a, b)` reads correctly because the package
qualifier supplies the verb; without it, a bare `component(a, b)`
returning a boolean is unreadable, and it would collide with
`canonical.component` in a flat import.

`ComponentDiff.Empty()` becomes `isEmpty` — the Go name is a predicate
despite reading as an adjective, and every target language spells
predicates with an `is` prefix.

`DiffOp` numbering starts at 1 (`OpAdded = iota + 1`), so zero is not a
valid `DiffOp`. Ports with a default-constructible enum MUST NOT make
`OpAdded` the zero value.

## `ext`

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `IsExtension(name string) bool` | `isExtension(name): boolean` | `is_extension(name) -> bool` | `is_extension(name) -> bool` | `Ext::isExtension(string $name): bool` |
| `Scope` (enum) | `Scope` | `Scope` | `Scope` | `Scope` |
| `(Scope) String() string` | `toString()` | `__str__()` | `Display for Scope` | `toString()` |
| `ScopeOf(name string) Scope` | `scopeOf(name): Scope` | `scope_of(name) -> Scope` | `scope_of(name) -> Scope` | `Ext::scopeOf(string $name): Scope` |
| `SystemName(name string) (string, bool)` | `systemName(name): string \| undefined` | `system_name(name) -> str \| None` | `system_name(name) -> Option<String>` | `Ext::systemName(string $name): ?string` |
| `ExtensionsByScope(c vstar.Component, scope Scope) []vstar.Property` | `extensionsByScope(c, scope): Property[]` | `extensions_by_scope(c, scope) -> list[Property]` | `extensions_by_scope(c, scope) -> Vec<&Property>` | `Ext::extensionsByScope(...): array` |

`Scope` values: `ScopeNone` (zero value — not an `X-*` extension at
all), `ScopeVStar` (`X-VSTAR-*`), `ScopeSystem` (`X-<SYSTEM>-*`),
`ScopeExperimental` (`X-EXP-*`), `ScopeUnknown` (has the `X-` prefix but
matches no tier). Re-cased per language:
`SCOPE_NONE`/`Scope::None`/`Scope::NONE` as the idiom dictates.
`ScopeNone` is the zero value; a port with a default-constructible enum
MUST keep it there.

The Go function is `ScopeOf`, not `Scope`, because a Go type and
function cannot share a name in one package. Ports have no such
constraint but keep `scopeOf` anyway, so the four ports and the
reference read alike.

## `hashing`

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `XVSTARHashProperty = "X-VSTAR-HASH"` | `X_VSTAR_HASH_PROPERTY` | `X_VSTAR_HASH_PROPERTY` | `X_VSTAR_HASH_PROPERTY` | `Hashing::X_VSTAR_HASH_PROPERTY` |
| `Component(c vstar.Component) string` | `component(c): string` | `component(c) -> str` | `component(c) -> String` | `Hashing::component(Component $c): string` |
| `Calendar(cal vstar.Calendar) string` | `calendar(cal): string` | `calendar(cal) -> str` | `calendar(cal) -> String` | `Hashing::calendar(Calendar $cal): string` |
| `Card(c vstar.Card) string` | `card(c): string` | `card(c) -> str` | `card(c) -> String` | `Hashing::card(Card $c): string` |
| `SetXVSTAR(c *vstar.Component)` | `setXVstar(c): void` | `set_x_vstar(c) -> None` | `set_x_vstar(c: &mut Component)` | `Hashing::setXVstar(Component $c): void` |
| `GetXVSTAR(c vstar.Component) (string, bool)` | `getXVstar(c): string \| undefined` | `get_x_vstar(c) -> str \| None` | `get_x_vstar(c) -> Option<&str>` | `Hashing::getXVstar(Component $c): ?string` |
| `VerifyXVSTAR(c vstar.Component) (ok bool, want, got string)` | `verifyXVstar(c): {ok, want, got}` | `verify_x_vstar(c) -> tuple[bool, str, str]` | `verify_x_vstar(c) -> (bool, String, String)` | `Hashing::verifyXVstar(Component $c): array` |

The three hash functions return the full `sha256:<hex>` string,
prefix included. The prefix is part of the value, not decoration — it
exists so a future algorithm migration is expressible.

`VerifyXVSTAR` returns three values, not an optional. Ports return a
struct/record/tuple of all three (`ok`, `want`, `got`) so a caller can
report *what* differed, not only *that* something did.

### `XVSTARHashProperty` lives in `hashing`

The constant is in the `hashing` package, and stays there. It is not
hoisted to the root package in any port, even though the root package
is where `Property` lives and the constant names a property. The reason
is coupling: the string is meaningful only in company with the hash
functions that write and read it, and the canonical layer's rule-7
exclusion of that property is a hashing concern.

A port that re-exports it from the root as a convenience has diverged;
callers write `hashing.X_VSTAR_HASH_PROPERTY` in every language.

## `helpers`

### Constructors

Each returns `(Component, error)` in Go; the error is a validation
failure on the arguments, not an I/O failure.

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `NewTodo(uid string, due time.Time) (vstar.Component, error)` | `newTodo(uid, due): Component` | `new_todo(uid, due) -> Component` | `new_todo(uid, due) -> Result<Component, Error>` | `Helpers::newTodo(...): Component` |
| `NewJournal(uid string, dtstart time.Time) (vstar.Component, error)` | `newJournal(uid, dtstart)` | `new_journal(uid, dtstart)` | `new_journal(uid, dtstart)` | `Helpers::newJournal(...)` |
| `NewEvent(uid string, dtstart, dtend time.Time) (vstar.Component, error)` | `newEvent(uid, dtstart, dtend)` | `new_event(uid, dtstart, dtend)` | `new_event(uid, dtstart, dtend)` | `Helpers::newEvent(...)` |
| `NewFreeBusy(uid string, dtstart, dtend time.Time) (vstar.Component, error)` | `newFreeBusy(uid, dtstart, dtend)` | `new_free_busy(uid, dtstart, dtend)` | `new_free_busy(uid, dtstart, dtend)` | `Helpers::newFreeBusy(...)` |
| `NewAlarm(uid, action, trigger string) (vstar.Component, error)` | `newAlarm(uid, action, trigger)` | `new_alarm(uid, action, trigger)` | `new_alarm(uid, action, trigger)` | `Helpers::newAlarm(...)` |
| `NewCalendar(prodID string) vstar.Calendar` | `newCalendar(prodId): Calendar` | `new_calendar(prod_id) -> Calendar` | `new_calendar(prod_id) -> Calendar` | `Helpers::newCalendar(string $prodId): Calendar` |
| `NewCard(uid string, kind vstar.Kind) vstar.Card` | `newCard(uid, kind): Card` | `new_card(uid, kind) -> Card` | `new_card(uid, kind) -> Card` | `Helpers::newCard(...): Card` |

`NewCalendar` and `NewCard` return no error — they cannot fail.

### Alarms

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `NewRelativeAlarm(uid, action string, offset duration.Duration, related duration.Related) (vstar.Component, error)` | `newRelativeAlarm(uid, action, offset, related)` | `new_relative_alarm(uid, action, offset, related)` | `new_relative_alarm(...) -> Result<Component, Error>` | `Helpers::newRelativeAlarm(...)` |
| `NewAbsoluteAlarm(uid, action string, at time.Time) (vstar.Component, error)` | `newAbsoluteAlarm(uid, action, at)` | `new_absolute_alarm(uid, action, at)` | `new_absolute_alarm(...)` | `Helpers::newAbsoluteAlarm(...)` |
| `AlarmFiresAt(alarm, parent vstar.Component, cal vstar.Calendar) (time.Time, error)` | `alarmFiresAt(alarm, parent, cal): Instant` | `alarm_fires_at(alarm, parent, cal) -> datetime` | `alarm_fires_at(...) -> Result<DateTime<Utc>, Error>` | `Helpers::alarmFiresAt(...): DateTimeImmutable` |

### Categories

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `Categories(c vstar.Component) []string` | `categories(c): string[]` | `categories(c) -> list[str]` | `categories(c) -> Vec<String>` | `Helpers::categories(Component $c): array` |
| `SetCategories(c *vstar.Component, values []string)` | `setCategories(c, values)` | `set_categories(c, values)` | `set_categories(c: &mut Component, values)` | `Helpers::setCategories(...): void` |
| `AddCategory(c *vstar.Component, value string)` | `addCategory(c, value)` | `add_category(c, value)` | `add_category(c: &mut Component, value)` | `Helpers::addCategory(...): void` |

### Classification and transparency

`Class` and `Transp` each have a getter returning `(T, bool)`, an
`OrDefault` getter applying the RFC default, and a setter. The default
is applied by the `OrDefault` form and by nothing else — the plain
getter reports absence faithfully.

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `Class(c vstar.Component) (vstar.Class, bool)` | `classOf(c): VClass \| undefined` | `class_of(c) -> VClass \| None` | `class_of(c) -> Option<Class>` | `Helpers::classOf(Component $c): ?VClass` |
| `ClassOrDefault(c vstar.Component) vstar.Class` | `classOrDefault(c): VClass` | `class_or_default(c) -> VClass` | `class_or_default(c) -> Class` | `Helpers::classOrDefault(...): VClass` |
| `SetClass(c *vstar.Component, v vstar.Class)` | `setClass(c, v)` | `set_class(c, v)` | `set_class(c: &mut Component, v)` | `Helpers::setClass(...): void` |
| `Transp(c vstar.Component) (vstar.Transp, bool)` | `transp(c): Transp \| undefined` | `transp(c) -> Transp \| None` | `transp(c) -> Option<Transp>` | `Helpers::transp(Component $c): ?Transp` |
| `TranspOrDefault(c vstar.Component) vstar.Transp` | `transpOrDefault(c): Transp` | `transp_or_default(c) -> Transp` | `transp_or_default(c) -> Transp` | `Helpers::transpOrDefault(...): Transp` |
| `SetTransp(c *vstar.Component, v vstar.Transp)` | `setTransp(c, v)` | `set_transp(c, v)` | `set_transp(c: &mut Component, v)` | `Helpers::setTransp(...): void` |

The getter is `classOf`, not `class`, in every port: `class` is a
reserved word in TypeScript, Python and PHP, and would shadow the
`Class` type in Rust. `SetClass` needs no rename.

### Relations

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `RelatedRef` (struct: `UID`, `RelType`) | `RelatedRef` (`uid`, `relType`) | `RelatedRef` (`uid`, `rel_type`) | `RelatedRef { uid, rel_type }` | `RelatedRef` (`$uid`, `$relType`) |
| `RelatedTo(c vstar.Component) []RelatedRef` | `relatedTo(c): RelatedRef[]` | `related_to(c) -> list[RelatedRef]` | `related_to(c) -> Vec<RelatedRef>` | `Helpers::relatedTo(Component $c): array` |
| `AddRelatedTo(c *vstar.Component, uid string, reltype vstar.RelType)` | `addRelatedTo(c, uid, relType)` | `add_related_to(c, uid, rel_type)` | `add_related_to(c: &mut Component, uid, rel_type)` | `Helpers::addRelatedTo(...): void` |

### Integer-valued properties

Each getter returns `(int, bool)`; absence is reported, never defaulted
to zero. `SEQUENCE` has no remover (it is required once present and
`IncrementSequence` is the mutation); `PRIORITY` and `PERCENT-COMPLETE`
each have one.

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `Sequence(c vstar.Component) (int, bool)` | `sequence(c): number \| undefined` | `sequence(c) -> int \| None` | `sequence(c) -> Option<i32>` | `Helpers::sequence(Component $c): ?int` |
| `SetSequence(c *vstar.Component, n int)` | `setSequence(c, n)` | `set_sequence(c, n)` | `set_sequence(c: &mut Component, n)` | `Helpers::setSequence(...): void` |
| `IncrementSequence(c *vstar.Component)` | `incrementSequence(c)` | `increment_sequence(c)` | `increment_sequence(c: &mut Component)` | `Helpers::incrementSequence(...): void` |
| `Priority(c vstar.Component) (int, bool)` | `priority(c): number \| undefined` | `priority(c) -> int \| None` | `priority(c) -> Option<i32>` | `Helpers::priority(Component $c): ?int` |
| `SetPriority(c *vstar.Component, n int)` | `setPriority(c, n)` | `set_priority(c, n)` | `set_priority(c: &mut Component, n)` | `Helpers::setPriority(...): void` |
| `RemovePriority(c *vstar.Component)` | `removePriority(c)` | `remove_priority(c)` | `remove_priority(c: &mut Component)` | `Helpers::removePriority(...): void` |
| `PercentComplete(c vstar.Component) (int, bool)` | `percentComplete(c): number \| undefined` | `percent_complete(c) -> int \| None` | `percent_complete(c) -> Option<i32>` | `Helpers::percentComplete(...): ?int` |
| `SetPercentComplete(c *vstar.Component, n int)` | `setPercentComplete(c, n)` | `set_percent_complete(c, n)` | `set_percent_complete(c: &mut Component, n)` | `Helpers::setPercentComplete(...): void` |
| `RemovePercentComplete(c *vstar.Component)` | `removePercentComplete(c)` | `remove_percent_complete(c)` | `remove_percent_complete(c: &mut Component)` | `Helpers::removePercentComplete(...): void` |

### Status

Three getter/setter pairs, one per component type, mirroring the three
distinct status types.

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `Status(c vstar.Component) (vstar.TodoStatus, bool)` | `todoStatus(c): TodoStatus \| undefined` | `todo_status(c) -> TodoStatus \| None` | `todo_status(c) -> Option<TodoStatus>` | `Helpers::todoStatus(...): ?TodoStatus` |
| `SetStatus(c *vstar.Component, s vstar.TodoStatus)` | `setTodoStatus(c, s)` | `set_todo_status(c, s)` | `set_todo_status(c: &mut Component, s)` | `Helpers::setTodoStatus(...): void` |
| `EventStatus(c vstar.Component) (vstar.EventStatus, bool)` | `eventStatus(c): EventStatus \| undefined` | `event_status(c) -> EventStatus \| None` | `event_status(c) -> Option<EventStatus>` | `Helpers::eventStatus(...): ?EventStatus` |
| `SetEventStatus(c *vstar.Component, s vstar.EventStatus)` | `setEventStatus(c, s)` | `set_event_status(c, s)` | `set_event_status(c: &mut Component, s)` | `Helpers::setEventStatus(...): void` |
| `JournalStatus(c vstar.Component) (vstar.JournalStatus, bool)` | `journalStatus(c): JournalStatus \| undefined` | `journal_status(c) -> JournalStatus \| None` | `journal_status(c) -> Option<JournalStatus>` | `Helpers::journalStatus(...): ?JournalStatus` |
| `SetJournalStatus(c *vstar.Component, s vstar.JournalStatus)` | `setJournalStatus(c, s)` | `set_journal_status(c, s)` | `set_journal_status(c: &mut Component, s)` | `Helpers::setJournalStatus(...): void` |
| `Complete(c *vstar.Component, t time.Time)` | `complete(c, t)` | `complete(c, t)` | `complete(c: &mut Component, t)` | `Helpers::complete(...): void` |

`Status` / `SetStatus` are the VTODO pair. In Go they carry the bare
name because VTODO is the historical default; ports spell them
`todoStatus` / `setTodoStatus` so all three pairs read symmetrically and
no caller reaches for `status` expecting it to work on a VEVENT.

### Due

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `Due(c vstar.Component, cal vstar.Calendar) (time.Time, bool)` | `due(c, cal): Instant \| undefined` | `due(c, cal) -> datetime \| None` | `due(c, cal) -> Option<DateTime<Utc>>` | `Helpers::due(...): ?DateTimeImmutable` |
| `SetDue(c *vstar.Component, t time.Time)` | `setDue(c, t)` | `set_due(c, t)` | `set_due(c: &mut Component, t)` | `Helpers::setDue(...): void` |

## `rrule`

### Parsing, validation and formatting

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `ParseRRule(s string) (Rule, error)` | `parseRRule(s): Rule` (throws) | `parse_rrule(s) -> Rule` (raises) | `parse_rrule(s) -> Result<Rule, Error>` | `Rrule::parse(string $s): Rule` |
| `ValidateRRule(s string) error` | `validateRRule(s): void` (throws) | `validate_rrule(s) -> None` (raises) | `validate_rrule(s) -> Result<(), Error>` | `Rrule::validate(string $s): void` |
| `(Rule) String() string` | `toString(): string` | `__str__() -> str` | `Display for Rule` | `__toString(): string` |
| `(Rule) Property() vstar.Property` | `toProperty(): Property` | `to_property() -> Property` | `to_property(&self) -> Property` | `toProperty(): Property` |
| `ParseDateTimeList(s string) ([]time.Time, error)` | `parseDateTimeList(s): Instant[]` | `parse_date_time_list(s) -> list[datetime]` | `parse_date_time_list(s) -> Result<Vec<DateTime<Utc>>, Error>` | `Rrule::parseDateTimeList(...): array` |
| `FormatDateTimeList(times []time.Time) string` | `formatDateTimeList(times): string` | `format_date_time_list(times) -> str` | `format_date_time_list(times) -> String` | `Rrule::formatDateTimeList(...): string` |

`ValidateRRule` returns only an error — it parses and discards. Ports
with a `Result` type return `Result<(), Error>`; throwing languages
return `void` and throw. A port MUST NOT make it return a boolean: the
*identity* of the failure is the payload, and the `rrule/rejected/`
fixtures assert it.

### Types

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `Rule` (struct) | `Rule` | `Rule` | `Rule` | `Rule` |
| `Set` (struct: `DTStart`, `RRule`, `RDate`, `ExDate`) | `RuleSet` (`dtstart`, `rrule`, `rdate`, `exdate`) | `RuleSet` (`dtstart`, `rrule`, `rdate`, `exdate`) | `RuleSet { dtstart, rrule, rdate, exdate }` | `RuleSet` (`$dtstart`, `$rrule`, `$rdate`, `$exdate`) |
| `ByDay` (struct: `Ordinal`, `Weekday`) | `ByDay` (`ordinal`, `weekday`) | `ByDay` (`ordinal`, `weekday`) | `ByDay { ordinal, weekday }` | `ByDay` (`$ordinal`, `$weekday`) |
| `Freq` (enum) | `Freq` | `Freq` | `Freq` | `Freq` |
| `(Freq) String() string` | `toString()` | `__str__()` | `Display for Freq` | `toString()` |
| `Weekday` (enum) | `Weekday` | `Weekday` | `Weekday` | `Weekday` |
| `(Weekday) String() string` | `toString()` | `__str__()` | `Display for Weekday` | `toString()` |
| `(Weekday) ToTime() time.Weekday` | `toIsoWeekday(): number` | `to_weekday() -> int` | `to_chrono(&self) -> chrono::Weekday` | `toWeekday(): int` |
| `Range` (enum) | `RecurrenceRange` | `RecurrenceRange` | `Range` | `RecurrenceRange` |
| `(Range) String() string` | `toString()` | `__str__()` | `Display for Range` | `toString()` |
| `RecurrenceID` (struct: `Time`, `Range`) | `RecurrenceId` (`time`, `range`) | `RecurrenceId` (`time`, `range`) | `RecurrenceId { time, range }` | `RecurrenceId` (`$time`, `$range`) |
| `ParseRecurrenceID(p vstar.Property) (RecurrenceID, error)` | `parseRecurrenceId(p): RecurrenceId` | `parse_recurrence_id(p) -> RecurrenceId` | `parse_recurrence_id(p) -> Result<RecurrenceId, Error>` | `RecurrenceId::parse(Property $p): RecurrenceId` |
| `(RecurrenceID) Property() vstar.Property` | `toProperty(): Property` | `to_property() -> Property` | `to_property(&self) -> Property` | `toProperty(): Property` |
| `SetFromComponent(c vstar.Component) (Set, error)` | `ruleSetFromComponent(c): RuleSet` | `rule_set_from_component(c) -> RuleSet` | `RuleSet::from_component(c) -> Result<RuleSet, Error>` | `RuleSet::fromComponent(Component $c): RuleSet` |

`Set` becomes `RuleSet` everywhere: a bare `Set` collides with the
JavaScript/TypeScript built-in, Python's `set`, and Rust's
`std::collections::HashSet` conventions, and in PHP reads as a mutator.
`Range` becomes `RecurrenceRange` in TypeScript, Python and PHP for the
same reason (`range` is a builtin in Python, `Range` a DOM type in TS);
Rust keeps `Range` under the module path.

`Rule` fields, in Go spelling, map by the idiom table: `Freq`,
`Interval`, `Until`, `Count`, `ByDay`, `ByMonth`, `ByMonthDay`,
`ByHour`, `ByMinute`, `BySecond`, `ByYearDay`, `ByWeekNo`, `BySetPos`,
`WeekStart` → TS `freq`, `interval`, `until`, `count`, `byDay`,
`byMonth`, `byMonthDay`, `byHour`, `byMinute`, `bySecond`, `byYearDay`,
`byWeekNo`, `bySetPos`, `weekStart`; Python/Rust `freq`, `interval`,
`until`, `count`, `by_day`, `by_month`, `by_month_day`, `by_hour`,
`by_minute`, `by_second`, `by_year_day`, `by_week_no`, `by_set_pos`,
`week_start`; PHP as TS.

`Set.RRule` is a **pointer** in Go (`*Rule`) — it is genuinely optional,
`nil` for an RDATE-only set. Ports model it as nullable
(`Rule | undefined`, `Rule | None`, `Option<Rule>`, `?Rule`).

`Freq` values: `FreqInvalid` (zero value), `FreqMinutely`, `FreqHourly`,
`FreqDaily`, `FreqWeekly`, `FreqMonthly`, `FreqYearly`. `FreqInvalid` MUST be the
zero value and MUST never be produced by a successful parse — a missing
`FREQ` is `ErrMalformed`.

`Weekday` values: `SU`, `MO`, `TU`, `WE`, `TH`, `FR`, `SA`, numbered
0..6 with `SU = 0` per RFC 5545 §3.3.10. That numbering is normative and
happens to match Go's `time.Weekday`; it does **not** match ISO-8601
(`MO = 1`) nor PHP's `DateTime::format('N')`. A port that reuses a
platform weekday number without converting has a one-off bug that the
`rrule/by-clauses/byday` fixtures will catch. `ToTime` is the explicit
conversion to the platform type and is the only place the conversion
should live.

`Range` values: `RangeThisInstance` (zero value, the default when the
`RANGE` parameter is absent) and `RangeThisAndFuture`
(`RANGE=THISANDFUTURE`).

`MaxIterations` in Go is `100000`. Its value is
**implementation-defined** per spec — a port picks its own finite bound
and documents it. What is not optional is that the bound exists, is
finite, and that reaching it produces `ErrIterationCap` rather than an
empty or completed result. The bound counts FREQ periods, so under
`FREQ=MINUTELY` at `INTERVAL=1` the reference's 100 000 spans about 69
days: a rule whose limits admit nothing for longer (for example
`FREQ=MINUTELY;BYMONTH=1` evaluated from February) reports
`ErrIterationCap` rather than the eventual occurrence, and the fixture
`rrule/evaluator/minutely_sparse_limit_caps` assumes a bound below
480 960 minute periods.

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `MaxIterations = 100000` | `MAX_ITERATIONS` | `MAX_ITERATIONS` | `MAX_ITERATIONS` | `Rrule::MAX_ITERATIONS` |

### Evaluation

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `NextOccurrence(rule Rule, dtstart, after time.Time) (time.Time, bool, error)` | `nextOccurrence(rule, dtstart, after): Instant \| undefined` (throws) | `next_occurrence(rule, dtstart, after) -> datetime \| None` (raises) | `next_occurrence(rule, dtstart, after) -> Result<Option<DateTime<Utc>>, Error>` | `Rrule::nextOccurrence(...): ?DateTimeImmutable` (throws) |
| `All(rule Rule, dtstart time.Time) iter.Seq[time.Time]` | `all(rule, dtstart): Generator<Instant>` | `all(rule, dtstart) -> Iterator[datetime]` | `all(rule, dtstart) -> impl Iterator<Item = DateTime<Utc>>` | `Rrule::all(...): Generator` |
| `Occurrences(rule Rule, dtstart time.Time, limit int) ([]time.Time, bool, error)` | `occurrences(rule, dtstart, limit): {times, complete}` (throws) | `occurrences(rule, dtstart, limit) -> tuple[list[datetime], bool]` (raises) | `occurrences(rule, dtstart, limit) -> Result<(Vec<DateTime<Utc>>, bool), Error>` | `Rrule::occurrences(...): array` (throws) |
| `Between(rule Rule, dtstart, start, end time.Time) ([]time.Time, error)` | `between(rule, dtstart, start, end): Instant[]` (throws) | `between(rule, dtstart, start, end) -> list[datetime]` (raises) | `between(rule, dtstart, start, end) -> Result<Vec<DateTime<Utc>>, Error>` | `Rrule::between(...): array` (throws) |
| `(Set) Occurrences(limit int) ([]time.Time, bool, error)` | `occurrences(limit): {times, complete}` | `occurrences(limit) -> tuple[list[datetime], bool]` | `occurrences(&self, limit) -> Result<(Vec<DateTime<Utc>>, bool), Error>` | `occurrences(int $limit): array` |
| `(Set) Between(start, end time.Time) ([]time.Time, error)` | `between(start, end): Instant[]` | `between(start, end) -> list[datetime]` | `between(&self, start, end) -> Result<Vec<DateTime<Utc>>, Error>` | `between(...): array` |

`Occurrences` returns a `complete` flag alongside the list: `true` means
the series terminated within the limit, `false` means the limit
truncated it. Reporting that distinction is required by the spec's
[Expansion](../../spec/v1.0/03-canonicalization.md#expansion) rules. A
port that returns only the list has dropped a required signal;
TypeScript and PHP return a two-field record rather than a positional
pair so the flag cannot be silently ignored.

### `rrule.All` returns a lazy sequence

`All` returns `iter.Seq[time.Time]` — a lazy, potentially infinite
sequence the consumer stops. It is the third of the spec's three
bounding strategies (count, window, laziness), and the only one that
works on an unbounded rule without the caller choosing a bound up
front.

| Language | Shape |
|---|---|
| TypeScript | `function*` generator — `Generator<Instant, void, undefined>` |
| Python | generator function, `Iterator[datetime]` |
| Rust | `impl Iterator<Item = DateTime<Utc>>` |
| PHP | `Generator` (`yield`) |

The sequence MUST be lazy in every port. Materializing it into an array
and returning that is broken: it hangs forever on an unbounded rule,
which is exactly the case the lazy form exists to serve. `All` has no
error channel, which is why the iteration cap is reachable only through
`NextOccurrence`, `Occurrences` and `Between` — an `All` consumer bounds
the work by stopping.

## `supersession`

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `CategoryStatusSupersession = "status-supersession"` | `CATEGORY_STATUS_SUPERSESSION` | `CATEGORY_STATUS_SUPERSESSION` | `CATEGORY_STATUS_SUPERSESSION` | `Supersession::CATEGORY_STATUS_SUPERSESSION` |
| `PropEffectiveStatus = "X-VSTAR-EFFECTIVE-STATUS"` | `PROP_EFFECTIVE_STATUS` | `PROP_EFFECTIVE_STATUS` | `PROP_EFFECTIVE_STATUS` | `Supersession::PROP_EFFECTIVE_STATUS` |
| `ErrTargetCorrupted` | see [sentinels](#the-twelve-error-sentinels) | " | " | " |
| `Supersedes(target vstar.Component, status string, t time.Time) (vstar.Component, error)` | `supersedes(target, status, t): Component` (throws) | `supersedes(target, status, t) -> Component` (raises) | `supersedes(target, status, t) -> Result<Component, Error>` | `Supersession::supersedes(...): Component` |
| `Superseded(c vstar.Component, ledger []vstar.Component) (status string, ok bool)` | `superseded(c, ledger): string \| undefined` | `superseded(c, ledger) -> str \| None` | `superseded(c, ledger) -> Option<String>` | `Supersession::superseded(...): ?string` |

### `PropEffectiveStatus` lives in `supersession`

Like `XVSTARHashProperty`, this constant stays in its own package. It
is not hoisted to the root. `X-VSTAR-EFFECTIVE-STATUS` is meaningful
only inside the supersession pattern — it is the property a
supersession VJOURNAL carries to state the new effective status of the
component its `RELATED-TO` names — and a root-level export invites
callers to write it onto components directly, which is precisely the
mutation the append-only discipline forbids.

### `Supersedes` returns an error, `Superseded` does not

The asymmetry is deliberate and both halves are contract:

- **`Supersedes`** returns `(Component, error)`. It recomputes the
  target's canonical hash and compares it against the target's stored
  `X-VSTAR-HASH`; a mismatch is `ErrTargetCorrupted` and the supersession
  is refused. Writing a supersession record against a component that has
  been mutated since it was hashed would silently attach the new status
  to different content. Ports throw / return `Err`. A port that returns
  a nullable `Component` here has thrown away *which* failure occurred.
- **`Superseded`** returns `(string, bool)` — a `(value, ok)` pair, no
  error. Asking "has this component been superseded by anything in this
  ledger?" has exactly two honest answers, and "no" is not a failure.
  Ports return an optional string.

## `validate`

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `Severity` (enum: `SeverityError`, `SeverityWarning`) | `Severity` | `Severity` | `Severity` | `Severity` |
| `(Severity) String() string` | `toString()` | `__str__()` | `Display for Severity` | `toString()` |
| `Diagnostic` (struct: `Severity`, `Code`, `Message`, `Path`) | `Diagnostic` (`severity`, `code`, `message`, `path`) | `Diagnostic` (`severity`, `code`, `message`, `path`) | `Diagnostic { severity, code, message, path }` | `Diagnostic` (`$severity`, `$code`, `$message`, `$path`) |
| `Validate(cal vstar.Calendar) []Diagnostic` | `validate(cal): Diagnostic[]` | `validate(cal) -> list[Diagnostic]` | `validate(cal) -> Vec<Diagnostic>` | `Validate::validate(Calendar $cal): array` |
| `ValidateComponent(c vstar.Component) []Diagnostic` | `validateComponent(c): Diagnostic[]` | `validate_component(c) -> list[Diagnostic]` | `validate_component(c) -> Vec<Diagnostic>` | `Validate::validateComponent(...): array` |
| `SeverityOf(code string) (Severity, bool)` | `severityOf(code): Severity \| undefined` | `severity_of(code) -> Severity \| None` | `severity_of(code) -> Option<Severity>` | `Validate::severityOf(string $code): ?Severity` |
| `Codes() []string` | `codes(): string[]` | `codes() -> list[str]` | `codes() -> Vec<&'static str>` | `Validate::codes(): array` |
| `StandardPropertyCount() int` | `standardPropertyCount(): number` | `standard_property_count() -> int` | `standard_property_count() -> usize` | `Validate::standardPropertyCount(): int` |

`SeverityError` is the zero value; `SeverityWarning` follows.

`Severity` is spelled per language rather than uniformly. Rust and PHP
carry real enums (PHP's backed by the wire string). TypeScript and
Python use string-literal types — `"error" | "warning"` and the
`Literal` equivalent — which are that language's type-safe enum and
keep one spelling rather than an enum plus a separate wire string that
could drift. TypeScript takes the type straight from its generated
registry module, so the severities and their type have a single source.

Neither `Validate` nor `ValidateComponent` returns an error. Validation
*is* the error channel: a document that fails every check still
validates successfully and returns a list of diagnostics. An empty list
means clean. A port that throws on a diagnostic has inverted the API.

### Generated diagnostic-code constants

The diagnostic codes are **generated** from
[`spec/registry/diagnostic-codes.json`](../../spec/registry/diagnostic-codes.json)
into each port's own generated module, alongside the RFC property
allow-list, the extension scopes and the status vocabularies. The Go
form is
[`go/validate/codes_gen.go`](../../go/validate/codes_gen.go); each port
gets a sibling under the same generator.

Constants are named for their meaning, not their number:
`CodeMissingUID`, `CodeMissingDTSTAMP`, `CodeBadXVSTARHash`,
`CodeUnknownProperty`, and so on — re-cased per language
(`CODE_MISSING_UID` in Python and Rust, `Code.MISSING_UID` or
`CODE_MISSING_UID` in TypeScript and PHP; pick one shape per port and
keep it). The numeric code string lives in the generated value only.

Hand-written port source MUST NOT contain a literal diagnostic-code
string. See the porting guide's
[generated registry constants](porting-guide.md#registry-constants-are-generated)
rule and its `registry-check` requirement — that rule is the reason no
example in this document spells one out.

The full catalog, with meanings and severities, is
[docs/validate-codes.md](../validate-codes.md) — itself generated from
the same registry.

### Generated vocabulary tables

The same generator renders four value-domain tables into every port's
generated module. They are public surface — the ports assert against
them directly — so they carry a naming contract like any other symbol:

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `StatusVocabulary` | `STATUS_VOCABULARY` | `STATUS_VOCABULARY` | `STATUS_VOCABULARY` | `Codes::STATUS_VOCABULARY` |
| `ClassVocabulary` | `CLASS_VOCABULARY` | `CLASS_VOCABULARY` | `CLASS_VOCABULARY` | `Codes::CLASS_VOCABULARY` |
| `TranspVocabulary` | `TRANSP_VOCABULARY` | `TRANSP_VOCABULARY` | `TRANSP_VOCABULARY` | `Codes::TRANSP_VOCABULARY` |
| `RelTypeVocabulary` | `RELTYPE_VOCABULARY` | `RELTYPE_VOCABULARY` | `RELTYPE_VOCABULARY` | `Codes::RELTYPE_VOCABULARY` |

`StatusVocabulary` is keyed by component type — the vocabulary RFC 5545
§3.8.1.11 scopes to `VTODO`, `VEVENT` and `VJOURNAL` differs per type,
which is what the status-scoping diagnostic enforces. The other three
are flat lists. These live in the generated module in every port
(`src/generated/codes.ts`, `vstar._generated.codes`,
`generated::codes`, `HopTop\Vstar\Generated\Codes`), not on the
`validate` surface, so they are imported from there rather than
re-exported.

Entry **order is contractual**: the tables are rendered in the
registry's order and the ports' vocabulary tests pin it, so a
reordering is a user-visible change.

## The twelve error sentinels

V\* names twelve failure classes. Every port MUST surface all twelve,
MUST make them programmatically distinguishable, and MUST use the Go
sentinel name as the stable identifier. The identifier is what the
conformance corpus asserts — `malformed/*.error` files and
`rrule/**/*.expect.json` sidecars name a class by its Go spelling — so
the string `ErrMalformed` is data, not a Go implementation detail.

| Go sentinel | Go package | Meaning |
|---|---|---|
| `ErrMalformed` | root | Structurally invalid input: bad escape, bad parameter syntax, unparseable value, or an RRULE on the parsing scope's hard-error list. |
| `ErrUnclosedBlock` | root | A `BEGIN` line lacks its matching `END` before end of input. |
| `ErrUnsupportedVersion` | root | A `VERSION` property is present but is neither vCard 4.0 nor iCalendar 2.0. |
| `ErrMissingUID` | root | A component that requires `UID` has none. |
| `ErrUnsupportedRRule` | `rrule` | Syntactically valid but outside the RRULE parsing scope: `FREQ=SECONDLY`, `RSCALE`, or a non-UTC `EXDATE`/`RDATE`/`RECURRENCE-ID` value. |
| `ErrIterationCap` | `rrule` | The evaluator reached its iteration bound without finding an occurrence; the rule did not terminate. |
| `ErrUnboundedExpansion` | `rrule` | A request to expand into a list with no bound: a zero or inverted window, or a negative limit. |
| `ErrTargetCorrupted` | `supersession` | The target component's `X-VSTAR-HASH` does not match its recomputed canonical form. |
| `ErrAlreadyClosed` | `codec/stream` | `Close` called twice, or `Encode` called after `Close`. |
| `ErrHeaderLocked` | `codec/stream` | `SetHeader` called after the first `Encode` locked the header. |
| `ErrNoTrigger` | `duration` | A `VALARM` without a `TRIGGER`; the property is mandatory (RFC 5545 §3.6.6), so the alarm cannot be scheduled. |
| `ErrNoAnchor` | `duration` | A relative trigger resolved against a component lacking its anchor — `DTSTART` for `RELATED=START`, or `DTEND` / `DTSTART`+`DURATION` / `DUE` for `RELATED=END`. |

This list is verified against the Go source: the four root sentinels are
in [`go/errors.go`](../../go/errors.go), and the other eight are
package-level `var`s in the packages named. No other exported error
sentinel exists in the library packages.

### How each language surfaces a sentinel

Go wraps sentinels with positional context
(`fmt.Errorf("line %d: %w", n, ErrMalformed)`) and callers dispatch with
`errors.Is`. Each port keeps the wrapping *and* the dispatchability:

**TypeScript** — one error class, `VstarError extends Error`, with a
`code` property carrying the sentinel identifier verbatim:

```ts
class VstarError extends Error {
  readonly code: VstarErrorCode;   // 'ErrMalformed' | 'ErrUnclosedBlock' | …
  readonly line?: number;          // positional context where known
  readonly cause?: unknown;
}

try {
  parse(input);
} catch (e) {
  if (e instanceof VstarError && e.code === 'ErrMalformed') { /* … */ }
}
```

`VstarErrorCode` is a string-literal union of exactly the twelve
identifiers. A port MUST NOT subclass per sentinel in TypeScript:
`instanceof` across module boundaries is unreliable when a bundler
duplicates the module, and a string comparison is not.

**Python** — one exception class per sentinel, all deriving from a
common `VstarError`, each carrying a `sentinel` attribute with the
identifier. Catch by class or by attribute; both work:

```python
class VstarError(Exception):
    sentinel: str

class Malformed(VstarError):
    sentinel = "ErrMalformed"

try:
    parse(data)
except Malformed:
    ...
except VstarError as e:
    if e.sentinel == "ErrIterationCap":
        ...
```

Class names drop the `Err` prefix — `Malformed`, `UnclosedBlock`,
`UnsupportedVersion`, `MissingUid`, `UnsupportedRrule`, `IterationCap`,
`UnboundedExpansion`, `TargetCorrupted`, `AlreadyClosed`,
`HeaderLocked`, `NoTrigger`, `NoAnchor` — since `raise MalformedError`
is not the Python idiom and the class is already an exception. The
`sentinel` attribute keeps the Go identifier verbatim.

**Rust** — one `Error` enum, one variant per sentinel, with a
`sentinel()` method returning the identifier:

```rust
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("malformed: {0}")]
    Malformed(String),
    #[error("unclosed block: {0}")]
    UnclosedBlock(String),
    // … ten more
}

impl Error {
    pub fn sentinel(&self) -> &'static str {
        match self {
            Error::Malformed(_) => "ErrMalformed",
            Error::UnclosedBlock(_) => "ErrUnclosedBlock",
            // …
        }
    }
}
```

Variants carry their positional context as payload. One crate-wide
enum, not a per-module error type: a caller composing parse → canonical
→ validate should match one enum, and the corpus fixtures assert one
identifier space.

**PHP** — one exception class per sentinel, all extending a common
`VstarException`, each with a `sentinel(): string` method:

```php
abstract class VstarException extends \RuntimeException {
    abstract public function sentinel(): string;
}

final class MalformedException extends VstarException {
    public function sentinel(): string { return 'ErrMalformed'; }
}
```

PHP keeps the `Exception` suffix (it is the PSR-ish norm and reads
correctly at the `catch` site) while Python drops it; the `sentinel()`
return value is identical in both.

### The identifier is the contract

All four shapes converge on one requirement: given a caught failure, a
test can recover the exact string in the table above, and compare it to
what a `.error` file or `.expect.json` sidecar says. A port whose
sentinel identifiers are spelled differently — `MALFORMED`,
`malformed`, `E_MALFORMED` — cannot run the shared corpus without a
translation table, and a translation table is a place for the four
ports to diverge. Use the Go spelling.

## The `(value, ok)` versus `error` distinction

Go's reference API uses two distinct shapes, and the difference is
semantic, not stylistic. Collapsing them is the single most likely way
for a port to diverge.

### `(T, bool)` means "optional"

A function returning `(T, bool)` reports **absence**, not failure. The
value is missing; nothing went wrong. Every such function maps to the
language's optional type:

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `(T, bool)` | `T \| undefined` | `T \| None` | `Option<T>` | `?T` |

TypeScript uses `undefined`, not `null`: it is what a missing property
access yields and what an unmatched `Array#find` returns, so it
composes with the language. A port MUST NOT return `null` for some and
`undefined` for others.

Functions in this shape, in full: `Calendar.Find`, `Card.Get`,
`Component.Get`, `Component.DTSTARTDate` and its three siblings,
`Component.DTSTART` / `DTEND` / `DUE` / `COMPLETED` / `DTSTAMP`,
`ParseDate`, `ParseTime`, `ParseTimeWithTZID`, `ext.SystemName`,
`hashing.GetXVSTAR`, `helpers.Class`, `helpers.Transp`,
`helpers.Sequence`, `helpers.Priority`, `helpers.PercentComplete`,
`helpers.Status`, `helpers.EventStatus`, `helpers.JournalStatus`,
`helpers.Due`, `duration.EventEnd`, `supersession.Superseded`,
`validate.SeverityOf`.

`ParseRelType` looks like this shape and is not — see
[the open enum](#reltype--the-open-enum).

### `error` means "failure"

A function returning `error` can fail in a way the caller must handle
and may want to distinguish. Throwing languages throw; `Result`
languages return `Result`:

| Go | TypeScript | Python | Rust | PHP |
|---|---|---|---|---|
| `(T, error)` | `T` + throws `VstarError` | `T` + raises `VstarError` | `Result<T, Error>` | `T` + throws `VstarException` |
| `error` | `void` + throws | `None` + raises | `Result<(), Error>` | `void` + throws |

A port MUST NOT reduce a failing call to a nullable return. Every
failure in this library carries a sentinel identity, and the conformance
corpus asserts that identity. Returning `undefined` for a parse failure
loses it.

### `rrule.NextOccurrence` is `(time.Time, bool, error)`

The one function carrying **both** channels, and the reason this section
exists:

```go
func NextOccurrence(rule Rule, dtstart, after time.Time) (time.Time, bool, error)
```

- The `bool` is exhaustion. The rule terminated — `UNTIL` passed,
  `COUNT` spent — and there is no next occurrence after `after`. This
  is **not an error**. It is the ordinary end of a finite series, and a
  caller walking a rule to its end hits it exactly once.
- The `error` is a rule the evaluator will not or cannot evaluate:
  `ErrUnsupportedRRule` for a rule outside the parsing scope, or
  `ErrIterationCap` when the iteration bound is reached without finding
  an occurrence — which is how an unsatisfiable rule like
  `FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30` surfaces, since the parser
  stays permissive about such combinations.

So a port needs both channels on this one call:

| Language | Signature |
|---|---|
| TypeScript | `nextOccurrence(...): Instant \| undefined` — `undefined` is exhaustion; throws `VstarError` for the error channel |
| Python | `next_occurrence(...) -> datetime \| None` — `None` is exhaustion; raises `VstarError` for the error channel |
| Rust | `next_occurrence(...) -> Result<Option<DateTime<Utc>>, Error>` — **`Result<Option<_>>`, never `Option<Result<_>>`** |
| PHP | `nextOccurrence(...): ?DateTimeImmutable` — `null` is exhaustion; throws `VstarException` for the error channel |

Collapsing the two — treating exhaustion as an error, or swallowing
`ErrIterationCap` as an empty result — fails the
`rrule/evaluator/unsatisfiable_feb_30` fixture, which asserts precisely
that an unsatisfiable rule produces the cap error and not an empty
answer. The spec makes this normative: "Reaching the bound MUST be
distinguished from the rule terminating."

### `supersession.Supersedes` returns `error`

`Supersedes` verifies the target's hash before writing the supersession
record and refuses a corrupted target with `ErrTargetCorrupted`. It is
the error path, not the optional path — see
[the asymmetry](#supersedes-returns-an-error-superseded-does-not).
`Superseded`, its counterpart, is `(value, ok)`.

### Stream encoders return `ErrAlreadyClosed` / `ErrHeaderLocked`

`VCalendarEncoder.Encode`, `VCalendarEncoder.Close`,
`VCardEncoder.Encode` and `VCardEncoder.Close` return
`ErrAlreadyClosed` when used after `Close`; `VCalendarEncoder.SetHeader`
returns `ErrHeaderLocked` after the header is locked. All five are the
error channel — throw or `Result`, never a silently ignored no-op and
never a boolean.

Stream *parser* exhaustion is the opposite case: `io.EOF` in Go, and in
ports the language's own iteration-finished signal, which is never an
error. See [parser exhaustion](#parser-exhaustion).

## Time representation per language

V\* handles instants at second resolution in UTC, and formats them as
RFC 5545 form #2 (`YYYYMMDDTHHMMSSZ`). A port's time type must
round-trip that format exactly and must not introduce a local-timezone
dependency anywhere.

| Language | Instant type | Formatting |
|---|---|---|
| TypeScript | epoch milliseconds (`number`), or `Temporal.Instant` where available | **Own UTC formatter.** Never `Date#toISOString()` semantics. |
| Python | timezone-aware `datetime` with `tzinfo=timezone.utc` | `strftime`-equivalent on the aware value |
| Rust | `chrono::DateTime<Utc>` | `format` with an explicit pattern |
| PHP | `DateTimeImmutable` constructed in UTC | `format('Ymd\THis\Z')` on the UTC value |

### TypeScript: do not reuse `Date#toISOString`

The TypeScript port represents instants as epoch milliseconds and MUST
carry its own UTC formatter and parser. `Date#toISOString()` is close
enough to look usable and is wrong in ways that produce silently
divergent canonical bytes:

- It emits the extended form (`2026-04-01T12:00:00.000Z`), so a port
  that strips separators must also strip the fractional-seconds field —
  and fractional seconds have no place in an RFC 5545 form #2 value.
- It throws on an out-of-range date rather than producing a value.
- Years outside 1000–9999 get expanded-year notation
  (`+012026-…`/`-000001-…`), which no RFC 5545 parser accepts.

Write `formatTime`/`parseTime` against the epoch-ms integer directly and
test them on the `time/` and `rrule/` fixtures. Everything else in the
port funnels through those two functions, so getting them right once is
sufficient.

### Python: aware datetimes only

Every `datetime` crossing the port's API boundary MUST be
timezone-aware with `tzinfo=timezone.utc`. A naive `datetime` compares
and arithmetics differently, and Python will not stop you mixing them —
it raises only on a naive/aware comparison, which means a naive value
can travel a long way before it fails. Reject naive input at the
boundary.

### PHP: immutable, always

Use `DateTimeImmutable`, never `DateTime`. The mutable class makes
`$a->add($i)` modify `$a` in place, which turns an occurrence-expansion
loop into a bug that only shows up with more than one occurrence.

### No local timezone, ever

None of the four ports may read the ambient timezone. Every instant
crossing the API is UTC; local wall-clock times exist only inside a
document, resolved against that document's own VTIMEZONE. See the
porting guide's
[no IANA timezone database](porting-guide.md#no-iana-timezone-database)
rule.

## What a port MUST NOT add

- A `Status` type unifying `TodoStatus`, `EventStatus` and
  `JournalStatus`.
- A `setHeader` on `VCardEncoder`.
- A root-level re-export of `XVSTARHashProperty` or
  `PropEffectiveStatus`.
- A boolean-returning `validateRRule`.
- An eager (array-returning) `all`.
- A closed `RelType` enum.
- A single-`Card`-returning `rfc6350.parse`.
- Any hand-written literal diagnostic-code string.

Each of these is a real divergence one of the four ports would
plausibly reach for, and each has a gate in the
[porting guide](porting-guide.md) that catches it.

## See also

- [Porting guide](porting-guide.md) — build order, layer gates, the
  hard rules, and the README profile for a port.
- [Specification](../../spec/) — the normative text. Canonical form,
  datetime resolution and RRULE scope are in
  [spec/03](../../spec/v1.0/03-canonicalization.md); conformance
  criteria and failure classes are in
  [spec/05](../../spec/v1.0/05-conformance.md).
- [Diagnostic code catalog](../validate-codes.md) — generated from the
  registry.
- [How to build a sister vstar implementation](../user/how-to-implement-vstar.md)
  — the adopter-facing version of the same material.
