# Parse your first VCALENDAR

Import `hop.top/vstar`, parse an iCalendar document,
and read the results — five minutes from a fresh module.

## Use this when

- You are evaluating vstar for an agentic-system or scheduling project.
- You have an `.ics` file (or VCALENDAR string) and want a typed Go
  representation.
- You want to confirm the library works end-to-end before reading
  deeper docs.

## Result

After completing this guide, you will have a Go program that parses
a VCALENDAR, walks its components, and computes the canonical
`X-VSTAR-HASH`.

## Before you begin

You need:

- Go 1.22 or later.
- A Go module (`go mod init` if you do not have one yet).

## Quick version

```sh
go get hop.top/vstar@latest
```

```go
package main

import (
	"fmt"
	"strings"

	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/hashing"
)

func main() {
	input := strings.Join([]string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//example//demo//EN",
		"BEGIN:VTODO",
		"UID:demo-001",
		"DTSTAMP:20260504T120000Z",
		"SUMMARY:Read the V* quickstart",
		"DUE:20260505T170000Z",
		"END:VTODO",
		"END:VCALENDAR",
		"",
	}, "\r\n")

	cal, err := rfc5545.Parse(strings.NewReader(input))
	if err != nil {
		panic(err)
	}

	todo := cal.Components[0]
	fmt.Printf("Type:  %s\n", todo.Type)
	fmt.Printf("UID:   %s\n", todo.UID())
	fmt.Printf("Hash:  %s\n", hashing.Calendar(cal))
}
```

```text
Type:  VTODO
UID:   demo-001
Hash:  sha256:3994fb0f43da7f37b63d86540641d64a28289c58a2b907a4e174fdf83fe25eab
```

## Steps

### 1. Add the dependency

```sh
go get hop.top/vstar@latest
```

Expected: `go.mod` and `go.sum` are updated; `go list -m
hop.top/vstar` prints the resolved version.

### 2. Parse a VCALENDAR

`codec/rfc5545.Parse` reads a single VCALENDAR from any `io.Reader`
and returns a `vstar.Calendar` value. Property order, parameter
order, and component order are preserved verbatim — no
canonicalization happens at parse.

```go
cal, err := rfc5545.Parse(strings.NewReader(input))
if err != nil {
	// Errors wrap one of vstar.ErrMalformed,
	// vstar.ErrUnclosedBlock, or vstar.ErrUnsupportedVersion.
	// Use errors.Is to dispatch.
	return err
}
```

For files, pass `os.Open(path)`. For network input, wrap with
`bufio.NewReader` if the underlying source is unbuffered.

### 3. Walk components and properties

`Calendar.Components` is the flat list of top-level components
(`VEVENT`, `VTODO`, `VJOURNAL`, `VFREEBUSY`, `VTIMEZONE`). Each
`Component` carries `Type`, `Props`, and nested `Sub` components
(e.g. `VALARM` inside `VEVENT`).

```go
for _, c := range cal.Components {
	switch c.Type {
	case vstar.CompTodo:
		fmt.Println("VTODO uid:", c.UID())
	case vstar.CompEvent:
		fmt.Println("VEVENT uid:", c.UID())
	}
}
```

Property accessors are case-insensitive per RFC 5545 §3.1: `c.Get("UID")`,
`c.Get("uid")`, and `c.Get("Uid")` are equivalent.

### 4. Verify the canonical hash

```go
fmt.Println("hash:", hashing.Calendar(cal))
```

`hashing.Calendar` recomputes the SHA-256 digest of the canonical
byte form (`canonical.Calendar`). Two implementations that agree on
canonical bytes — Go, future TypeScript, AGR-Racket — produce
identical hashes for the same logical content. This is the
foundational invariant for cross-language integrity.

If your input already carries an `X-VSTAR-HASH` and you want to
check it, use `hashing.VerifyXVSTAR(component)` instead — it
returns `(ok, want, got)` so you can surface the mismatch.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `errors.Is(err, vstar.ErrMalformed)` on `Parse` | Input is not RFC 5545 — missing CRLF, malformed BEGIN/END, unknown property. | Check the wrapped error's positional context (line + column). Validate the source bytes with `cat -A`. |
| `errors.Is(err, vstar.ErrUnclosedBlock)` | A `BEGIN:X` has no matching `END:X`. | Ensure your input ends with `END:VCALENDAR\r\n` and every nested block is paired. |
| `errors.Is(err, vstar.ErrUnsupportedVersion)` | The VCARD codec rejected `VERSION:3.0`. | vstar v0.1 supports vCard 4.0 only. Convert your input or use a different library. |
| Hash differs from a peer implementation | Property order, parameter order, NFC normalization, or TZID resolution disagree. | Compare canonical bytes directly: `canonical.Calendar(cal)`. The first divergent byte points at the rule. |

## How it works

`vstar` (root package) ships only the in-memory data model —
`Property`, `Param`, `Component`, `Calendar`, `Card`, plus the
wire-string enums and error sentinels. I/O lives in the codec
packages (`codec/rfc5545`, `codec/rfc6350`, `codec/stream`).
Canonicalization (`canonical`), hashing (`hashing`), validation
(`validate`), and ergonomic helpers (`helpers`) build on top in
their own subpackages. This split keeps the data model small and
import-cycle-free.

The hash you computed in Step 4 is over the *canonical byte form*,
not the wire bytes you parsed. Wire forms can vary in property
order, datetime form, parameter capitalization, and line folding;
the canonical form is the equivalence-class representative.

## Next steps

- [How to validate and hash a Calendar](how-to-validate-and-hash.md) —
  surface and interpret diagnostic codes.
- [How to parse and evaluate RRULE](how-to-recurrence.md) — work
  with VTODO/VEVENT recurrence in v0.2.
- [Diagnostic code catalog](../validate-codes.md) — every
  `validate.Diagnostic.Code` the library emits.
