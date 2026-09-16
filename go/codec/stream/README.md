# `codec/stream` — Constant-memory streaming codecs

`codec/stream` is the iterator-style counterpart to the eager
`codec/rfc5545` and `codec/rfc6350` codecs. It surfaces one
`Component` (or `Card`) at a time, letting callers process arbitrarily
large V\* documents without buffering the full calendar in memory.

The streaming surface targets two AGR consumers that scale
unfavorably with batch parsing:

- **AGR L7** (playthrough fork comparison) iterates many-MB VJOURNAL
  forks; only the current entry needs to be live.
- **AGR L2** (ledger projection) folds events into a running totals
  struct without ever needing the full calendar in memory.

## Surface

```go
type StreamParser interface {
    Next() (vstar.Component, error) // io.EOF when input drained
}

type StreamEncoder interface {
    Encode(c vstar.Component) error
    Close() error
}

type CardStreamParser interface {
    Next() (vstar.Card, error)
}

type CardStreamEncoder interface {
    Encode(c vstar.Card) error
    Close() error
}
```

Concrete constructors:

```go
stream.NewVCalendarParser(r io.Reader)  *VCalendarParser
stream.NewVCalendarEncoder(w io.Writer) *VCalendarEncoder
stream.NewVCardParser(r io.Reader)      *VCardParser
stream.NewVCardEncoder(w io.Writer)     *VCardEncoder
```

`VCalendarParser.Header()` returns the captured calendar-level
properties (`PRODID`, `VERSION`) without consuming any components.

`VCalendarEncoder.SetHeader(c vstar.Calendar)` overrides the default
PRODID before the first `Encode`. After the header is on the wire,
`SetHeader` returns `ErrHeaderLocked`.

Sentinel errors live in `stream.go`:

- `ErrAlreadyClosed` — `Close` called twice or `Encode` after `Close`.
- `ErrHeaderLocked` — `SetHeader` after first `Encode`.

Match with `errors.Is`.

## Usage

### Streaming parse

```go
p := stream.NewVCalendarParser(file)
for {
    c, err := p.Next()
    if errors.Is(err, io.EOF) {
        break
    }
    if err != nil {
        return err
    }
    // process c, then drop the reference
}
```

### Streaming encode

```go
enc := stream.NewVCalendarEncoder(file)
_ = enc.SetHeader(vstar.Calendar{ProdID: "-//acme//Cal//EN"})
for _, c := range stream {
    if err := enc.Encode(c); err != nil {
        return err
    }
}
return enc.Close()
```

## Performance

The package contains four pairs of `Benchmark*` functions that
compare batch (`rfc5545.Parse` / `rfc6350.Default.Parse` and
`rfc5545.Encode`) against streaming (`stream.NewV{Calendar,Card}{Parser,Encoder}`)
across a 4-step geometric grid: 10, 100, 1 000, 10 000 components.

Two metrics are reported per benchmark:

- **`B/op`** — total bytes allocated per iteration. This includes
  short-lived garbage and so grows roughly linearly with `n` for
  both batch and streaming codecs (each component still allocates a
  `Component` struct + property slice).
- **`B/peak`** — peak `runtime.HeapInuse` after a forced GC. This is
  the **live working-set** metric and is the load-bearing data point
  for the constant-memory claim: streaming holds at most one
  component live at a time, while batch keeps the whole calendar.

### Results (Apple M1 Pro, Go 1.26, `-benchtime=5x`)

#### VCALENDAR parse (working set)

| n      | Batch B/peak | Stream B/peak | Ratio |
|--------|-------------:|--------------:|------:|
| 10     |      909 312 |       892 928 |  0.98 |
| 100    |    1 015 808 |       942 080 |  0.93 |
| 1 000  |    1 499 136 |     1 056 768 |  0.70 |
| 10 000 |    6 488 064 |     2 195 456 |  0.34 |

Stream working set grows ~2.5× across a 1 000× input size; batch
grows ~7×. At 10 000 components the streaming parser uses ~3× less
live memory than batch.

#### VCALENDAR parse (allocations per op, for reference)

| n      | Batch B/op | Stream B/op | Batch allocs | Stream allocs |
|--------|-----------:|------------:|-------------:|--------------:|
| 10     |     12 416 |      10 176 |          147 |           142 |
| 100    |     81 872 |      62 736 |        1 320 |         1 312 |
| 1 000  |    741 648 |     595 536 |       13 023 |        13 012 |
| 10 000 |  8 789 420 |   5 923 584 |      130 031 |       130 012 |

Both grow linearly because each yielded Component still carries a
fresh allocation; the difference is that the streaming version's
yielded values are reachable for GC the instant the caller drops
them, while batch keeps the whole slice live.

#### VCALENDAR encode

| n      | Batch B/op | Stream B/op |
|--------|-----------:|------------:|
| 10     |      6 113 |       7 696 |
| 100    |     43 249 |      36 000 |
| 1 000  |    366 225 |     298 816 |
| 10 000 |  3 281 236 |   4 329 012 |

Stream encode B/op is comparable to batch — the encoder buffers
through `bufio` so the per-iteration cost is dominated by the
`Component` flat byte build, not by intermediate allocation.

#### VCARD parse

| n      | Batch B/peak | Stream B/peak |
|--------|-------------:|--------------:|
| 10     |      901 120 |       901 120 |
| 100    |      925 696 |       925 696 |
| 1 000  |      974 848 |       966 656 |
| 10 000 |    1 515 520 |     1 515 520 |

VCARD is naturally a sequence of small flat blocks; both parsers
already operate on roughly one card at a time inside their hot
loop, so the working-set delta is small. The streaming codec is
still preferred for the API ergonomics and for backpressure: the
batch codec materializes a `[]vstar.Card` of length `n` that the
caller has no choice but to keep live until `Parse` returns.

### Re-running the benchmarks

```sh
go test -run "^$" -bench "." -benchmem -benchtime=5x ./codec/stream/
```

Use `-benchtime=2s` for higher-confidence numbers (the
`B/peak` metric only updates when the iteration's GC sees a higher
heap; 5x is enough to expose the curve, 2s smooths run-to-run jitter).

## Constraints

- Streaming codecs are **not safe for concurrent use** across
  goroutines on the same instance. Construct one per goroutine.
- Backpressure / cancellation are the caller's responsibility — wrap
  the underlying `io.Reader` / `io.Writer` with a context-aware
  adapter if you need it.
- VCALENDAR streaming parser does NOT support calendar-level
  properties interleaved with components; all VERSION / PRODID /
  METHOD lines MUST appear before the first `BEGIN:<sub-component>`.
  This matches the AGR producer convention.
