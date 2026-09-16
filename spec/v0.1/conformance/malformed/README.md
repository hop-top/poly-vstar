# Malformed input fixtures

Per-sentinel negative test fodder for the codec layer. Each fixture
is paired with `<name>.error` containing the expected `vstar.Err*`
sentinel name (one per line). Fixtures here do NOT carry
`.canonical` or `.hash` siblings — most fail to parse, so those
goldens would be undefined.

| File                       | Sentinel              | Mode             |
|----------------------------|-----------------------|------------------|
| `unsupported_version.vcf`  | `ErrUnsupportedVersion` | parse (rfc6350) |
| `malformed.ics`            | `ErrMalformed`        | parse (rfc5545)  |
| `unclosed_block.ics`       | `ErrUnclosedBlock`    | parse (rfc5545)  |
| `missing_uid.vcf`          | `ErrMissingUID`       | encode (rfc6350) |

`ErrMissingUID` is **encoder-only** in v0.1 — the rfc6350 parser
accepts a VCARD without UID. The `missing_uid.vcf` fixture is
therefore tested by parsing successfully, then attempting to
re-encode the parsed Card; the encoder MUST refuse with
`ErrMissingUID`.

The walker tests live at:

- `codec/rfc5545/malformed_test.go`
- `codec/rfc6350/malformed_test.go`

Each walker filters by extension so the dir can mix `.ics` and
`.vcf` (a single corpus folder is easier to navigate).
