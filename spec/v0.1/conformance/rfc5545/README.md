# RFC 5545 conformance fixtures

Round-trip golden VCALENDARs consumed by the Go codec round-trip
tests and by every port's reference-bytes and canonical gates.

Files use **LF** line terminators on disk for diff-friendliness;
the parser is liberal on input (CRLF or LF) and the encoder always
emits CRLF on output. Round-trip semantic equality is asserted by
`TestRoundTripFixture_*` in `codec/rfc5545/`.

Each fixture's `.notes.md` states what it exercises and, for the
blind-spot fixtures below, which rule every *other* fixture satisfied
by accident. Read the notes before adding a case that looks similar.

## Catalogue

| File | Purpose |
|------|---------|
| `empty.ics` | Calendar with PRODID + VERSION only |
| `one_vtodo.ics` | Single VTODO with UID/DTSTAMP/SUMMARY/PRIORITY |
| `nested_vtimezone.ics` | VTIMEZONE containing STANDARD + DAYLIGHT subs |
| `vevent_valarm.ics` | VEVENT containing one VALARM |
| `vevent_duration_alarm.ics` | VEVENT with DURATION (no DTEND) + VALARM with RELATED=END trigger, REPEAT and DURATION |
| `valarm_absolute_trigger.ics` | VEVENT containing one VALARM with an absolute VALUE=DATE-TIME trigger |
| `vjournal.ics` | Single VJOURNAL component |
| `vfreebusy.ics` | Single VFREEBUSY with two FREEBUSY periods |
| `world.ics` | VCALENDAR with three VTODOs of varying STATUS |
| `all_day_vtodo.ics` | Single all-day VTODO: `DTSTART`/`DUE` carry `VALUE=DATE` |
| `all_day_vtodo_variant.ics` | `all_day_vtodo.ics` with `VALUE=date` + stray TZID on DUE; same `.canonical`/`.hash` |
| `related_reltype.ics` | Mission VTODO + two assignment VTODOs linked by RELATED-TO: RELTYPE omitted (PARENT), DEPENDS-ON, FINISHTOSTART |
| `vevent_status_class_transp.ics` | VEVENT with STATUS:CONFIRMED, CLASS:PRIVATE, TRANSP:TRANSPARENT (spec/05 §7 vocabularies) |
| `vjournal_status.ics` | VJOURNAL with STATUS:FINAL (VJOURNAL-only vocabulary token) |
| `vtodo_sequence_percent.ics` | VTODO with SEQUENCE:2, PRIORITY:0, PERCENT-COMPLETE:40 and DUE (spec/05 §7 integer domains) |
| `fold_split_utf8.ics` | Two-octet UTF-8 sequence placed across the 75-octet fold (rule 3). The only fixture that reaches a fold point with non-ASCII content; without it a port can retreat the cut or fold by characters and stay byte-identical everywhere else |
| `nfc_decomposed.ics` | Decomposed values in a property value *and* a parameter value (rule 9), plus a precomposed value that must be left alone. The only non-NFC input in the corpus |
| `attach_binary.ics` | Both rule 10 shapes: inline `VALUE=BINARY;ENCODING=BASE64` stripped, URI form keeping an unrelated `FMTTYPE` — pins that only the two named parameters go |
| `sort_utf8_uids.ics` | Four UIDs chosen so UTF-8 byte order and UTF-16 code-unit order disagree (rule 6); every other UID in the corpus is ASCII, where the orders coincide |
| `escaping.ics` | Every RFC 5545 §3.3.11 TEXT escape, an escaped calendar-level `PRODID`, and a URI-typed `ATTACH` whose backslash must survive verbatim. The only `.ics` input containing a backslash |

The last five exist because a port skipped the rule they pin and
still reproduced every committed `.canonical` and `.hash` byte for
byte. Each was mutation-tested against the drift gate: disabling
its rule in the reference rewrites exactly that fixture's siblings
and no others.

## Hash goldens

Each fixture has a sibling `<fixture>.hash` file containing the
content hash of its canonical form, in the format

```
sha256:<64 lowercase hex chars>
```

followed by a single LF terminator (file is exactly 72 bytes).

The hash is computed by `hashing.Calendar` (for `.ics`) or
`hashing.Card` (for `.vcf`) over the canonical byte form (see
`spec/v0.1/03-canonicalization.md` and `canonical/`). The
`X-VSTAR-HASH` property — when present in the input — is stripped
before hashing per spec/03 §7.

Every port, and any other implementation that claims V\* conformance,
MUST produce byte-identical `.hash` content for these fixtures. CI enforces this
via `TestHashGoldens` in `hashing/golden_test.go` and, across the
ports, `make test-parity`. To
regenerate after a deliberate canonical change, run a small
generator that walks the fixtures, calls `hashing.Calendar` /
`hashing.Card`, and rewrites the `.hash` siblings.
