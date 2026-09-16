<!-- SPDX-License-Identifier: CC-BY-4.0 -->

# V\* v0.1

**Status:** Draft
**Last updated:** 2026-09-14

First published version of the V\* specification. Extracted from the
`spec/` directory of [`hop-top/vstar`](https://github.com/hop-top/vstar),
where it was drafted alongside the Go reference implementation.

## Files

| File | What |
|------|------|
| [`01-overview.md`](01-overview.md) | Design principles and scope. Read this first. |
| [`02-component-mapping.md`](02-component-mapping.md) | Agentic concept → RFC 5545 / RFC 6350 component. The core mapping table, required common properties, and the supersession ledger. |
| [`03-canonicalization.md`](03-canonicalization.md) | Canonical form, hashing, and equality. Byte-for-byte rules, datetime resolution, the VTIMEZONE subset, and the RRULE parsing scope. |
| [`04-extensions.md`](04-extensions.md) | `X-*` namespace tiers and the promotion discipline. |
| [`05-conformance.md`](05-conformance.md) | What a conformant document and a conformant implementation are; self-certification. |
| [`conformance/`](conformance/) | Cross-language conformance corpus: inputs, canonical bytes, hashes, expected parse errors, fuzz seeds. |

## Conformance

The corpus under [`conformance/`](conformance/) is the canonical
fixture set for v0.1. Every implementation consumes the same
fixtures and MUST produce identical `.canonical` bytes and `.hash`
content. The Go reference implementation carries a mirror of this
corpus as its `testdata/` and verifies it in CI; any drift between
the two is a bug.
