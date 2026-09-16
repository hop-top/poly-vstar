# attach_binary

`ATTACH` in both of rule 10's shapes.

The first is inline binary: `VALUE=BINARY` and `ENCODING=BASE64` are
both stripped on emit, leaving the value as a bare URI-shaped token.
Canonical form is reference-only — it never re-encodes or inlines the
payload.

The second is already a URI and keeps its unrelated `FMTTYPE`
parameter, so the fixture also pins that rule 10 strips only the two
named parameters rather than every parameter on an `ATTACH`.

This fixture exists because nothing else in the corpus carries a binary
`ATTACH`. Without it a port can leave `VALUE=BINARY` untouched and
still reproduce every other `.canonical` and `.hash` byte for byte.
