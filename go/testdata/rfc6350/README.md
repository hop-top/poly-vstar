# RFC 6350 conformance fixtures

Round-trip golden VCARDs consumed by the Go codec round-trip tests
and by every port's reference-bytes and canonical gates.

Files use **LF** line terminators on disk for diff-friendliness;
the parser is liberal on input (CRLF or LF) and the encoder always
emits CRLF on output. Round-trip semantic equality is asserted in
`codec/rfc6350/`.

This directory is the **only** corpus-level reach into the vCard
encoder's folding: `canonical.Card` folds through
`rfc5545.EncodeComponent`, so no `.canonical` file exercises
`rfc6350`'s own folder. And round-trip equality cannot see an
over-long physical line — unfolding is width-agnostic, so a
76-octet line decodes to the same Card. `TestConformance_RoundTrip`
in `codec/rfc6350/conformance_test.go` therefore measures every
physical line the encoder emits against the 75-octet limit; without
that assertion `fold_long_note.vcf` is inert.

## Catalogue

| File | Purpose |
|------|---------|
| `minimal.vcf` | Single FN-only VCARD |
| `kind_org.vcf` | VCARD with KIND=org |
| `kind_group.vcf` | VCARD with KIND=group + 3 MEMBER properties |
| `escaping.vcf` | VCARD exercising RFC 6350 §3.4 TEXT escaping |
| `fold_long_note.vcf` | NOTE long enough to fold onto **three** continuation lines — one is never enough: an encoder alternating 75/74 payload octets overruns only on the second continuation |
| `grouped.vcf` | VCARD with property grouping |
| `with_extensions.vcf` | One X-* per spec/04 scope (vstar, system, exp) |

## Hash goldens

Each fixture has a sibling `<fixture>.hash` file containing the
content hash of its canonical form, in the format

```
sha256:<64 lowercase hex chars>
```

followed by a single LF terminator (file is exactly 72 bytes).

The hash is computed by `hashing.Card` over the canonical byte
form (see `spec/v0.1/03-canonicalization.md` and `canonical/`).
The `X-VSTAR-HASH` property — when present in the input — is
stripped before hashing per spec/03 §7.

Every port and any future AGR-Racket V\* implementation MUST produce
byte-identical `.hash` content for these fixtures. CI enforces this
via `TestHashGoldens` in `hashing/golden_test.go` and, across the
ports, `make test-parity`.
