# escaping

The `.ics` counterpart to `rfc6350/escaping.vcf`: a VCALENDAR whose
TEXT values carry every RFC 5545 §3.3.11 escape — `\,`, `\;`, `\n`
and `\\` — plus a calendar-level `PRODID` that carries two of them.

Nothing else in the `.ics` corpus contained a backslash, so a parser
could skip TEXT unescaping entirely and still reproduce every other
`.canonical` and `.hash` file byte for byte. Round-tripping this
fixture is what forces the unescape-then-re-escape pair to be
symmetric: a parser that leaves `\,` in the model emits `\\,` on the
way out and fails the canonical comparison.

`PRODID` is included deliberately. It is the one TEXT property that
lives on the calendar rather than inside a component, so an
implementation that unescapes component properties in one code path
and reads the header in another can get the component half right and
still mishandle `PRODID`.

`ATTACH` is the negative case. It is URI-typed, not TEXT, so its
backslash is data and MUST survive verbatim — an implementation that
unescapes every value regardless of type corrupts the address and
fails this fixture.
