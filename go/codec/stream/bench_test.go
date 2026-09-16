// SPDX-License-Identifier: MIT

package stream_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/codec/rfc6350"
	"hop.top/vstar/codec/stream"
)

// benchSizes is the set of component counts used by every benchmark
// in this file. Choosing a 4-step geometric progression (10, 100,
// 1k, 10k) lets `go test -bench` plot a clean log-scale curve so the
// linear-vs-flat memory contrast is visually obvious in the output.
var benchSizes = []int{10, 100, 1000, 10000}

// buildCalendar produces a wire-form VCALENDAR with n VEVENT
// components. Each VEVENT carries a small fixed property bundle so
// per-component cost is uniform and the benchmark measures iteration
// cost rather than per-component variance.
func buildCalendar(n int) []byte {
	var b bytes.Buffer
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//bench//EN\r\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b,
			"BEGIN:VEVENT\r\n"+
				"UID:e%d\r\n"+
				"DTSTAMP:20260101T000000Z\r\n"+
				"SUMMARY:Bench event %d\r\n"+
				"END:VEVENT\r\n",
			i, i)
	}
	b.WriteString("END:VCALENDAR\r\n")
	return b.Bytes()
}

// buildVCards produces n concatenated BEGIN:VCARD…END:VCARD blocks.
func buildVCards(n int) []byte {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b,
			"BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u%d\r\nFN:Bench %d\r\nEND:VCARD\r\n",
			i, i)
	}
	return []byte(b.String())
}

// BenchmarkVCalendar_Batch_Parse exercises the eager codec on each
// size; allocation per op should grow roughly linearly with size
// because the parser keeps every Component live for the duration of
// the call.
func BenchmarkVCalendar_Batch_Parse(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			data := buildCalendar(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cal, err := rfc5545.Parse(bytes.NewReader(data))
				if err != nil {
					b.Fatalf("Parse: %v", err)
				}
				if len(cal.Components) != n {
					b.Fatalf("got %d components, want %d",
						len(cal.Components), n)
				}
			}
		})
	}
}

// BenchmarkVCalendar_Stream_Parse exercises the iterator codec on
// each size; allocation per op should remain roughly flat because
// only the current component is materialized at a time.
func BenchmarkVCalendar_Stream_Parse(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			data := buildCalendar(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				p := stream.NewVCalendarParser(bytes.NewReader(data))
				count := 0
				for {
					c, err := p.Next()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						b.Fatalf("Next: %v", err)
					}
					if c.Type == "" {
						b.Fatalf("empty component type")
					}
					count++
				}
				if count != n {
					b.Fatalf("count=%d, want %d", count, n)
				}
			}
		})
	}
}

// BenchmarkVCalendar_Batch_Encode pre-builds a Calendar with n
// components and times the eager encoder. Allocation per op grows
// with n because the encoder hands the whole slice off to bufio in
// one call.
func BenchmarkVCalendar_Batch_Encode(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			cal := buildCalendarObj(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var buf bytes.Buffer
				if err := rfc5545.Encode(&buf, cal); err != nil {
					b.Fatalf("Encode: %v", err)
				}
			}
		})
	}
}

// BenchmarkVCalendar_Stream_Encode times the iterator encoder.
// Per-iteration allocation is dominated by the bufio chunk, so it
// should stay roughly flat across n.
func BenchmarkVCalendar_Stream_Encode(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			cal := buildCalendarObj(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var buf bytes.Buffer
				enc := stream.NewVCalendarEncoder(&buf)
				for j := range cal.Components {
					if err := enc.Encode(cal.Components[j]); err != nil {
						b.Fatalf("Encode: %v", err)
					}
				}
				if err := enc.Close(); err != nil {
					b.Fatalf("Close: %v", err)
				}
			}
		})
	}
}

// BenchmarkVCard_Batch_Parse — eager rfc6350 codec across the same
// size grid for an apples-to-apples curve with the streaming side.
func BenchmarkVCard_Batch_Parse(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			data := buildVCards(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cards, err := rfc6350.Default.Parse(bytes.NewReader(data))
				if err != nil {
					b.Fatalf("Parse: %v", err)
				}
				if len(cards) != n {
					b.Fatalf("got %d cards, want %d", len(cards), n)
				}
			}
		})
	}
}

// BenchmarkVCard_Stream_Parse — iterator codec across the same
// size grid; per-op allocation should stay flat.
func BenchmarkVCard_Stream_Parse(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			data := buildVCards(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				p := stream.NewVCardParser(bytes.NewReader(data))
				count := 0
				for {
					_, err := p.Next()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						b.Fatalf("Next: %v", err)
					}
					count++
				}
				if count != n {
					b.Fatalf("count=%d, want %d", count, n)
				}
			}
		})
	}
}

// BenchmarkVCalendar_Batch_HeapInuse measures peak heap residency
// while Parse holds the entire Calendar in memory. The metric is
// reported as a custom "B/peak" counter so go test output makes the
// linear-vs-flat contrast against the streaming counterpart obvious.
func BenchmarkVCalendar_Batch_HeapInuse(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			data := buildCalendar(n)
			var peak uint64
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cal, err := rfc5545.Parse(bytes.NewReader(data))
				if err != nil {
					b.Fatalf("Parse: %v", err)
				}
				// Run GC so HeapInuse reflects actual live working
				// set, not accumulated unreaped allocations from
				// earlier benchmark iterations.
				runtime.GC()
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				if ms.HeapInuse > peak {
					peak = ms.HeapInuse
				}
				// Touch cal so the compiler cannot drop the reference
				// before the MemStats read above.
				if len(cal.Components) != n {
					b.Fatalf("len mismatch")
				}
			}
			b.ReportMetric(float64(peak), "B/peak")
		})
	}
}

// BenchmarkVCalendar_Stream_HeapInuse measures peak heap residency
// while iterating the streaming parser and immediately discarding
// each component. The peak should remain roughly flat across the
// size grid, demonstrating the constant-memory property.
func BenchmarkVCalendar_Stream_HeapInuse(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			data := buildCalendar(n)
			var peak uint64
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				p := stream.NewVCalendarParser(bytes.NewReader(data))
				var lastUID string
				for {
					c, err := p.Next()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						b.Fatalf("Next: %v", err)
					}
					lastUID = c.UID()
				}
				_ = lastUID
				// Run GC so HeapInuse reflects actual live working
				// set, not accumulated unreaped allocations from
				// earlier benchmark iterations.
				runtime.GC()
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				if ms.HeapInuse > peak {
					peak = ms.HeapInuse
				}
			}
			b.ReportMetric(float64(peak), "B/peak")
		})
	}
}

// BenchmarkVCard_Batch_HeapInuse mirrors the calendar batch peak
// benchmark for VCARD inputs.
func BenchmarkVCard_Batch_HeapInuse(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			data := buildVCards(n)
			var peak uint64
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cards, err := rfc6350.Default.Parse(bytes.NewReader(data))
				if err != nil {
					b.Fatalf("Parse: %v", err)
				}
				// Run GC so HeapInuse reflects actual live working
				// set, not accumulated unreaped allocations from
				// earlier benchmark iterations.
				runtime.GC()
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				if ms.HeapInuse > peak {
					peak = ms.HeapInuse
				}
				if len(cards) != n {
					b.Fatalf("len mismatch")
				}
			}
			b.ReportMetric(float64(peak), "B/peak")
		})
	}
}

// BenchmarkVCard_Stream_HeapInuse mirrors the calendar stream peak
// benchmark for VCARD inputs.
func BenchmarkVCard_Stream_HeapInuse(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			data := buildVCards(n)
			var peak uint64
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				p := stream.NewVCardParser(bytes.NewReader(data))
				var lastUID string
				for {
					c, err := p.Next()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						b.Fatalf("Next: %v", err)
					}
					lastUID = c.UID
				}
				_ = lastUID
				// Run GC so HeapInuse reflects actual live working
				// set, not accumulated unreaped allocations from
				// earlier benchmark iterations.
				runtime.GC()
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				if ms.HeapInuse > peak {
					peak = ms.HeapInuse
				}
			}
			b.ReportMetric(float64(peak), "B/peak")
		})
	}
}

// buildCalendarObj returns the in-memory model the encoder benchmarks
// hand to Encode. Identical shape to buildCalendar's wire output so
// the two pairs are comparable apples-to-apples.
func buildCalendarObj(n int) vstar.Calendar {
	cal := vstar.Calendar{ProdID: "-//bench//EN"}
	for i := 0; i < n; i++ {
		cal.Components = append(cal.Components, vstar.Component{
			Type: vstar.CompEvent,
			Props: []vstar.Property{
				{Name: "UID", Value: fmt.Sprintf("e%d", i)},
				{Name: "DTSTAMP", Value: "20260101T000000Z"},
				{Name: "SUMMARY", Value: fmt.Sprintf("Bench event %d", i)},
			},
		})
	}
	return cal
}
