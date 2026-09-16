# vstar (go)

Go reference implementation of V\*, the calendar/card-shaped data
convention for agentic systems. It parses and encodes iCalendar
(RFC 5545) and vCard (RFC 6350) components, produces their canonical
byte form, computes `X-VSTAR-HASH`, evaluates RRULEs, validates V\*
invariants, and applies append-only supersession. Every other V\*
implementation cross-validates against this module's output.

> This repository is a read-only language mirror, republished from the
> `hop-top/poly-vstar` monorepo on each release. Open issues and pull
> requests in [`hop-top/poly-vstar`](https://github.com/hop-top/poly-vstar).

## Install

```sh
go get hop.top/vstar
```

Requirements: Go 1.25 or newer. The module depends only on the standard
library and `golang.org/x/text`; it takes no `hop.top/kit` dependency.

## Quick start

```go
package main

import (
	"fmt"
	"strings"

	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/hashing"
)

func main() {
	input := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\n" +
		"BEGIN:VTODO\r\nUID:1\r\nDTSTAMP:20260504T120000Z\r\nEND:VTODO\r\n" +
		"END:VCALENDAR\r\n"

	cal, err := rfc5545.Parse(strings.NewReader(input))
	if err != nil {
		panic(err)
	}
	fmt.Println(hashing.Calendar(cal)) // sha256:<hex> over the canonical bytes
}
```

Full walkthrough (parse + hash + walk components):
[`docs/user/quickstart.md`](https://github.com/hop-top/poly-vstar/blob/main/docs/user/quickstart.md).

## What you get

- `hop.top/vstar` — the in-memory data model: `Calendar`, `Card`,
  components, properties, parameters, typed time values.
- `codec/rfc5545`, `codec/rfc6350` — VCALENDAR and VCARD 4.0 parsers
  and encoders; `codec/stream` — constant-memory iterator codecs for
  large ledgers.
- `canonical` — the deterministic canonical byte form per spec/03.
- `hashing` — `sha256:<hex>` content hashes over the canonical form.
- `validate` — V\* semantic invariants the codecs cannot catch, reported
  as `VSnnn` diagnostics
  ([catalog](https://github.com/hop-top/poly-vstar/blob/main/docs/validate-codes.md)).
- `rrule` — RFC 5545 §3.3.10 RRULE parser, boundary check and forward
  evaluator (`NextOccurrence`).
- `supersession` — the append-only state-change discipline from spec/02.
- `diff` — semantic equality and structural diff via canonical form.
- `ext`, `helpers` — `X-*` extension-namespace predicates and
  convenience constructors above the bare property API.
- `testdata/` — a generated mirror of the conformance corpus
  authored at
  [`spec/v0.1/conformance/`](https://github.com/hop-top/poly-vstar/tree/main/spec/v0.1/conformance),
  committed so this module stays self-contained. `cmd/fixtures-verify`
  regenerates every `.canonical` and `.hash` sibling and fails on
  drift; in a checkout without the monorepo's sibling `spec/` tree it
  verifies `testdata/` in place.

## Where to go next

| You are… | Start here |
|----------|------------|
| **Adopting `hop.top/vstar`** | [`docs/user/quickstart.md`](https://github.com/hop-top/poly-vstar/blob/main/docs/user/quickstart.md) → [`docs/INDEX.md`](https://github.com/hop-top/poly-vstar/blob/main/docs/INDEX.md) for how-tos and reference |
| **Validating and hashing** | [`docs/user/how-to-validate-and-hash.md`](https://github.com/hop-top/poly-vstar/blob/main/docs/user/how-to-validate-and-hash.md) |
| **Working with recurrence** | [`docs/user/how-to-recurrence.md`](https://github.com/hop-top/poly-vstar/blob/main/docs/user/how-to-recurrence.md) |
| **Building a sister implementation** | [`docs/user/how-to-implement-vstar.md`](https://github.com/hop-top/poly-vstar/blob/main/docs/user/how-to-implement-vstar.md) + [spec/05 — conformance](https://github.com/hop-top/poly-vstar/blob/main/spec/v0.1/05-conformance.md) |
| **Changing vstar itself** | [`CONTRIBUTING.md`](https://github.com/hop-top/poly-vstar/blob/main/CONTRIBUTING.md) → [`docs/dev/setup.md`](https://github.com/hop-top/poly-vstar/blob/main/docs/dev/setup.md) |
| **Understanding a canonical-form rule** | [spec/03 — canonicalization](https://github.com/hop-top/poly-vstar/blob/main/spec/v0.1/03-canonicalization.md) |

API reference: `go doc hop.top/vstar` (and `.../codec/rfc5545`,
`.../canonical`, `.../validate`, …), or
[pkg.go.dev/hop.top/vstar](https://pkg.go.dev/hop.top/vstar).

## Related projects

- **AGR** — downstream consumer. Its compiler converts
  Racket-described worlds into V\* component sequences. AGR-internal
  extensions live in `X-AGR-*`.
- **AGNTCY TS SDK** — candidate consumer; agents can emit V\* as
  their canonical action log. Not yet wired.

## License

MIT. See the
[`hop-top/poly-vstar` LICENSE](https://github.com/hop-top/poly-vstar/blob/main/LICENSE).
Spec prose is CC-BY-4.0 and lives at
[`spec/`](https://github.com/hop-top/poly-vstar/tree/main/spec) in the
monorepo; the conformance corpus mirrored into `testdata/` is MIT.
