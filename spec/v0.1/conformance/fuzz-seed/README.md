# Fuzz seed corpus

Minimal-but-diverse byte sequences that seed the codec fuzz
targets. The seed corpus exists in TWO locations kept in sync by
`make fixtures-verify`:

| Location                                                            | Format                          | Role                |
|---------------------------------------------------------------------|---------------------------------|---------------------|
| `spec/v0.1/conformance/fuzz-seed/<rfc>/seed_*.bytes`                | Raw bytes                       | Canonical source    |
| `codec/<rfc>/testdata/fuzz/FuzzParse_RFC<RFC>/seed_*`            | go-fuzz `v1` wrapper            | Picked up by `go test -fuzz` |

This directory is the source of truth. Update it via
`go -C go run ./cmd/fixtures-gen-fuzz` at the repository root, then run
`go -C go run ./cmd/fixtures-verify` to mirror into the go-fuzz dirs.
The matching go-fuzz files are NOT hand-edited.

## Seed coverage targets

Seeds aim for diversity over depth — exercise distinct RFC paths
rather than fuzz the same shape repeatedly:

- empty input
- minimal-valid input
- LF-only and CRLF EOL variants (encoder always emits CRLF; parser
  is liberal)
- folded long lines (RFC 5545 §3.1)
- escaped TEXT (`\\,`, `\\;`, `\\n`, `\\\\`)
- nested blocks (VTIMEZONE → STANDARD; VEVENT → VALARM)
- params with quotes (TZID="Etc/GMT+0")
- multi-card files (VCARD only)
- VCARD KIND values (individual / org / group)
- VCARD extension scope tiers (`X-VSTAR-*` / `X-AGR-*` / `X-EXP-*`)
- intentionally bad inputs (only-BEGIN; v3 VCARD; mixed EOL)

## Counts

| Codec   | Seed count |
|---------|------------|
| rfc5545 | 12         |
| rfc6350 | 12         |

Both exceed the plan's ≥10 target. Add new seeds to
`cmd/fixtures-gen-fuzz/main.go` (NOT directly to the seed dirs)
so the generator stays the canonical author.

## How fuzz targets consume seeds

`go test -fuzz=FuzzParse_RFC5545 ./codec/rfc5545/` reads the seeds
under `codec/rfc5545/testdata/fuzz/FuzzParse_RFC5545/` per the
go-fuzz convention, plus any inline `f.Add(...)` calls in the test
file. New seeds widen the starting corpus before the engine begins
mutating.

`go test ./codec/rfc5545/` (without `-fuzz`) treats every seed as a
deterministic regression test — if any seed makes Parse panic or
return an undocumented error, the test fails.
